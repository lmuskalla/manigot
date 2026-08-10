#!/usr/bin/env bash
set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────
IMAGE_NAME="manigot"
CLAUDE_DIR_NAME="docs"

# ── Parse args ──────────────────────────────────────────────────────────────────
AGENT=""
JOB=""
INITIAL_PROMPT=""
TOOL="claude-code"
PRINT="false"
PASSTHROUGH=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --agent)  AGENT="$2";  shift 2 ;;
        --job)    JOB="$2";    shift 2 ;;
        --prompt) INITIAL_PROMPT="$2"; shift 2 ;;
        --tool)   TOOL="$2";   shift 2 ;;
        --print)  PRINT="true"; shift ;;
        *)        PASSTHROUGH+=("$1"); shift ;;
    esac
done

case "$TOOL" in
    claude-code|opencode) ;;
    *)
        echo "Error: --tool must be 'claude-code' or 'opencode' (got '$TOOL')."
        exit 1
        ;;
esac

# --print is a non-interactive, one-shot invocation (see scripts/entrypoint.sh)
# built for automated/unattended callers like mg-jdi, not for a human's own
# session. It is Claude Code only for v1 — OpenCode's non-interactive
# equivalent is unverified (see docs/backlog.md) — so reject the combination
# with a clear error rather than silently falling back to an interactive run.
if [[ "$PRINT" == "true" && "$TOOL" == "opencode" ]]; then
    echo "Error: --print is only supported with --tool claude-code."
    echo "OpenCode's non-interactive invocation is unverified for v1 — see docs/backlog.md."
    exit 1
fi

# ── Diagnostic output redirection ────────────────────────────────────────────
# --print callers expect a clean stdout carrying only the agent's own output
# (see the "Run" section below) — run.sh's own info/warning/banner lines below
# go through fd 3 instead of a bare echo, which points at stderr in --print
# mode and at stdout otherwise. Interactive behavior is unchanged (both fds
# render to the same terminal there).
if [[ "$PRINT" == "true" ]]; then
    exec 3>&2
else
    exec 3>&1
fi

# ── Load .env from manigot dir ─────────────────────────────────────────────────
# Follow symlinks to the real script location — this is installed as a
# PATH symlink (make install), and `dirname "${BASH_SOURCE[0]}"` alone would
# resolve to the symlink's directory (e.g. /usr/local/bin), not the checkout.
resolve_script_dir() {
    local src="${BASH_SOURCE[0]}" dir
    while [[ -h "$src" ]]; do
        dir="$(cd -P "$(dirname "$src")" && pwd)"
        src="$(readlink "$src")"
        [[ "$src" != /* ]] && src="$dir/$src"
    done
    cd -P "$(dirname "$src")" && pwd
}
SCRIPT_DIR="$(resolve_script_dir)"
MANIGOT_ROOT="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$MANIGOT_ROOT/.env"

if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck source=/dev/null
    source "$ENV_FILE"
    set +a
else
    echo "Warning: no .env found at $ENV_FILE" >&3
fi

# ── Resolve project root ────────────────────────────────────────────────────────
# A docs/ directory (walking up from $PWD) marks an initialized project — the
# job workflow, project context, and project-level agent overrides only exist
# there. Its absence is not an error: mg still starts a plain, isolated agent
# session, so it can replace calling `claude`/`opencode` directly in any
# project. The container boundary then falls back to the git root, if any,
# else $PWD.
find_project_root() {
    local dir="$PWD"
    while [[ "$dir" != "/" ]]; do
        if [[ -d "$dir/$CLAUDE_DIR_NAME" ]]; then
            echo "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo ""
}

DOCS_INITIALIZED="true"
PROJECT_ROOT="$(find_project_root)"

if [[ -z "$PROJECT_ROOT" ]]; then
    DOCS_INITIALIZED="false"
    PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
    [[ -z "$PROJECT_ROOT" ]] && PROJECT_ROOT="$PWD"
fi

PROJECT_ROOT="${PROJECT_ROOT%/}"
PROJECT_DOCS_DIR="$PROJECT_ROOT/$CLAUDE_DIR_NAME"

# Each tool looks for its project-level config (agents, etc.) in its own directory.
if [[ "$TOOL" == "opencode" ]]; then
    DOCS_MOUNT_TARGET="/workspace/.opencode"
else
    DOCS_MOUNT_TARGET="/workspace/.claude"
fi

# ── Project context file ────────────────────────────────────────────────────────
# docs/AGENTS.md is the canonical, tool-neutral project context. Neither tool
# reads it from where docs/ is mounted, so it gets a second read-only mount at
# the path each one actually looks in:
#   OpenCode    — AGENTS.md found by traversing up from the working directory
#   Claude Code — .claude/CLAUDE.md in the project
# docs/CLAUDE.md is still accepted as a fallback so older projects keep working.
CONTEXT_FILE=""
if [[ -f "$PROJECT_DOCS_DIR/AGENTS.md" ]]; then
    CONTEXT_FILE="$PROJECT_DOCS_DIR/AGENTS.md"
elif [[ -f "$PROJECT_DOCS_DIR/CLAUDE.md" ]]; then
    CONTEXT_FILE="$PROJECT_DOCS_DIR/CLAUDE.md"
fi

CONTEXT_MOUNT=()
if [[ -n "$CONTEXT_FILE" ]]; then
    if [[ "$TOOL" == "opencode" ]]; then
        CONTEXT_TARGET="/workspace/AGENTS.md"
    else
        CONTEXT_TARGET="/workspace/.claude/CLAUDE.md"
    fi
    CONTEXT_MOUNT=(-v "$CONTEXT_FILE:$CONTEXT_TARGET:ro")
elif [[ "$DOCS_INITIALIZED" == "true" ]]; then
    echo "Warning: no $CLAUDE_DIR_NAME/AGENTS.md found — the agent will start without project context." >&3
else
    echo "Note: no $CLAUDE_DIR_NAME/ directory — starting a plain session with no project context or job workflow." >&3
    echo "      See 'Per-project setup' in the manigot README to enable them." >&3
fi

# docs/ itself is only mounted into the container when it exists — an absent
# host path would otherwise make docker silently create an empty directory
# there.
DOCS_MOUNT=()
if [[ -d "$PROJECT_DOCS_DIR" ]]; then
    DOCS_MOUNT=(-v "$PROJECT_DOCS_DIR:$DOCS_MOUNT_TARGET:z")
fi

# ── Resolve job directory ───────────────────────────────────────────────────────
AGENT_FLAG=()
JOB_PROMPT=""

if [[ -n "$JOB" ]]; then
    if [[ "$DOCS_INITIALIZED" != "true" ]]; then
        echo "Error: --job requires an initialized project (no docs/ found)."
        echo "See 'Per-project setup' in the manigot README, then 'mg job' to create one."
        exit 1
    fi
    JOB_DIR="$PROJECT_ROOT/docs/jobs/$JOB"
    if [[ ! -d "$JOB_DIR" ]]; then
        MATCH=$(find "$PROJECT_ROOT/docs/jobs" -maxdepth 1 -type d -name "${JOB}*" 2>/dev/null | head -1 || true)
        if [[ -n "$MATCH" ]]; then
            JOB_DIR="$MATCH"
            JOB="$(basename "$MATCH")"
        else
            echo "Error: job '$JOB' not found under docs/jobs/"
            exit 1
        fi
    fi
    CONTAINER_JOB_DIR="/workspace/docs/jobs/$JOB"
    JOB_PROMPT="Please work on the job at ${CONTAINER_JOB_DIR} — start by reading brief.md"
    # --print sessions are non-interactive and unattended (no human can answer
    # a follow-up question), so they get one extra sentence defining the
    # NEEDS-HUMAN-INPUT marker (see docs/AGENTS.md). Interactive sessions
    # never see this — a human is there to just answer the question instead.
    if [[ "$PRINT" == "true" ]]; then
        JOB_PROMPT="${JOB_PROMPT}"$'\n\n'"You are running non-interactively and unattended — no human is watching this session or able to answer questions. If you cannot proceed without a human decision, stop immediately and print a line starting with exactly \`NEEDS-HUMAN-INPUT:\` followed by a one-sentence reason, and make no further changes."
    fi
fi

# --agent is passed as a proper CLI flag to claude, not as prompt text.
# This ensures Claude Code actually starts in agent mode rather than
# interpreting @agentname as a loose mention in a regular session.
if [[ -n "$AGENT" ]]; then
    AGENT_FLAG=(--agent "$AGENT")
fi

# Claude Code takes the initial prompt as a positional argument, OpenCode as --prompt.
# Two sources can seed it: --job (JOB_PROMPT, the job-workflow path) and --prompt
# (INITIAL_PROMPT, for ad-hoc non-job sessions like `mg init`'s prompter hand-off).
# When both are given, the job prompt wins (job-only is simplest and the prompter
# hand-off is only used when there's no job in play).
PROMPT_ARGS=()
INITIAL_PROMPT_TEXT="$JOB_PROMPT"
if [[ -z "$INITIAL_PROMPT_TEXT" && -n "$INITIAL_PROMPT" ]]; then
    INITIAL_PROMPT_TEXT="$INITIAL_PROMPT"
fi
if [[ -n "$INITIAL_PROMPT_TEXT" ]]; then
    if [[ "$TOOL" == "opencode" ]]; then
        PROMPT_ARGS=(--prompt "$INITIAL_PROMPT_TEXT")
    else
        PROMPT_ARGS=("$INITIAL_PROMPT_TEXT")
    fi
fi

# ── Auth checks ─────────────────────────────────────────────────────────────────
# OpenCode is multi-provider: any one of these keys is enough to start it.
# Keep this list in sync with the one in scripts/entrypoint.sh.
OPENCODE_KEY_VARS=(
    ANTHROPIC_API_KEY
    OPENAI_API_KEY
    OPENROUTER_API_KEY
    GOOGLE_GENERATIVE_AI_API_KEY
    GROQ_API_KEY
    XAI_API_KEY
    DEEPSEEK_API_KEY
    OPENCODE_API_KEY
    ZHIPU_API_KEY
)

KEY_ENV_ARGS=()

if [[ "$TOOL" == "claude-code" ]]; then
    if [[ -z "${CLAUDE_CODE_OAUTH_TOKEN:-}" ]]; then
        echo "Error: CLAUDE_CODE_OAUTH_TOKEN is not set."
        echo "Add it to $ENV_FILE:"
        echo "  CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-..."
        exit 1
    fi

    # Subscription protection — only relevant to Claude Code, where an API key
    # would override the mounted OAuth credentials and bill per token.
    if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
        echo "Error: ANTHROPIC_API_KEY is set — this overrides your subscription and bills per token."
        echo "Remove it from your environment before running mg with --tool claude-code."
        exit 1
    fi
else
    for var in "${OPENCODE_KEY_VARS[@]}"; do
        [[ -n "${!var:-}" ]] && KEY_ENV_ARGS+=(-e "$var=${!var}")
    done

    if [[ ${#KEY_ENV_ARGS[@]} -eq 0 ]]; then
        echo "Error: --tool opencode needs at least one provider API key."
        echo "Add one of these to $ENV_FILE:"
        printf '  %s\n' "${OPENCODE_KEY_VARS[@]}"
        exit 1
    fi

    # Optional: which model OpenCode should start with, as provider/model.
    [[ -n "${OPENCODE_MODEL:-}" ]] && KEY_ENV_ARGS+=(-e "OPENCODE_MODEL=$OPENCODE_MODEL")
fi

# ── Shadow .env files with /dev/null mounts ─────────────────────────────────────
ENV_MOUNTS=()
while IFS= read -r -d '' envfile; do
    [[ "$envfile" == *.example ]] && continue
    [[ "$envfile" == *.sample ]] && continue
    container_path="/workspace${envfile#"$PROJECT_ROOT"}"
    echo "  Shadowing: $envfile → /dev/null inside container" >&3
    ENV_MOUNTS+=(--mount "type=bind,source=/dev/null,target=$container_path,readonly")
done < <(find "$PROJECT_ROOT" -type f \( -name ".env" -o -name ".env.*" \) -print0)

if [[ ${#ENV_MOUNTS[@]} -eq 0 ]]; then
    echo "  Shadowed : none (no .env files found)" >&3
fi

# ── Info ────────────────────────────────────────────────────────────────────────
echo "╔══════════════════════════════════════╗" >&3
echo "║           manigot                   ║" >&3
echo "╠══════════════════════════════════════╣" >&3
echo "║  Project : $(basename "$PROJECT_ROOT")" >&3
echo "║  Root    : $PROJECT_ROOT" >&3
if [[ "$DOCS_INITIALIZED" == "true" ]]; then
    echo "║  Docs    : $PROJECT_DOCS_DIR" >&3
else
    echo "║  Docs    : (none — job workflow unavailable)" >&3
fi
[[ -n "$CONTEXT_FILE" ]] && echo "║  Context : $CONTEXT_FILE → $CONTEXT_TARGET" >&3
[[ -n "$TOOL"  ]] && echo "║  Tool    : $TOOL" >&3
[[ -n "$AGENT" ]] && echo "║  Agent   : $AGENT" >&3
[[ -n "$JOB"   ]] && echo "║  Job     : $JOB" >&3
[[ "$PRINT" == "true" ]] && echo "║  Print   : yes (non-interactive)" >&3
echo "╚══════════════════════════════════════╝" >&3
echo "" >&3

# ── Run ─────────────────────────────────────────────────────────────────────────
# --print drops -it in favor of a plain foregrounded run: no pseudo-tty is
# allocated, so the container's stdout (the agent's own output — see
# scripts/entrypoint.sh) streams straight through as this script's stdout,
# uncluttered by the diagnostics above (redirected to fd 3/stderr).
DOCKER_TTY_FLAGS=(-it)
if [[ "$PRINT" == "true" ]]; then
    DOCKER_TTY_FLAGS=()
fi

docker run "${DOCKER_TTY_FLAGS[@]+"${DOCKER_TTY_FLAGS[@]}"}" --rm \
    --name "manigot-$(basename "$PROJECT_ROOT")-$$" \
    --user "$(id -u):$(id -g)" \
    -v "$PROJECT_ROOT:/workspace:z" \
    "${DOCS_MOUNT[@]+"${DOCS_MOUNT[@]}"}" \
    "${CONTEXT_MOUNT[@]+"${CONTEXT_MOUNT[@]}"}" \
    "${ENV_MOUNTS[@]+"${ENV_MOUNTS[@]}"}" \
    "${KEY_ENV_ARGS[@]+"${KEY_ENV_ARGS[@]}"}" \
    -e CLAUDE_CODE_OAUTH_TOKEN="${CLAUDE_CODE_OAUTH_TOKEN:-}" \
    -e CLAUDE_ACCOUNT_UUID="${CLAUDE_ACCOUNT_UUID:-}" \
    -e CLAUDE_EMAIL="${CLAUDE_EMAIL:-}" \
    -e CLAUDE_ORG_UUID="${CLAUDE_ORG_UUID:-}" \
    -e GIT_AUTHOR_NAME_CFG="${GIT_AUTHOR_NAME:-}" \
    -e GIT_AUTHOR_EMAIL_CFG="${GIT_AUTHOR_EMAIL:-}" \
    -e MANIGOT_TOOL="$TOOL" \
    -e MANIGOT_PRINT="$PRINT" \
    --network=bridge \
    --memory=2g \
    --security-opt=no-new-privileges \
    "$IMAGE_NAME" \
    "${AGENT_FLAG[@]+"${AGENT_FLAG[@]}"}" \
    "${PROMPT_ARGS[@]+"${PROMPT_ARGS[@]}"}" \
    "${PASSTHROUGH[@]+"${PASSTHROUGH[@]}"}"