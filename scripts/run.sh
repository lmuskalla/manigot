#!/usr/bin/env bash
set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────
IMAGE_NAME="safecode"
CLAUDE_DIR_NAME="docs"

# ── Parse args ──────────────────────────────────────────────────────────────────
AGENT=""
JOB=""
TOOL="claude-code"
PASSTHROUGH=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --agent) AGENT="$2"; shift 2 ;;
        --job)   JOB="$2";   shift 2 ;;
        --tool)  TOOL="$2";  shift 2 ;;
        *)       PASSTHROUGH+=("$1"); shift ;;
    esac
done

case "$TOOL" in
    claude-code|opencode) ;;
    *)
        echo "Error: --tool must be 'claude-code' or 'opencode' (got '$TOOL')."
        exit 1
        ;;
esac

# ── Load .env from safecode dir ─────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SAFECODE_ROOT="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$SAFECODE_ROOT/.env"

if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck source=/dev/null
    source "$ENV_FILE"
    set +a
else
    echo "Warning: no .env found at $ENV_FILE"
fi

# ── Resolve project root ────────────────────────────────────────────────────────
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

PROJECT_ROOT="$(find_project_root)"

if [[ -z "$PROJECT_ROOT" ]]; then
    echo "Error: could not find a '$CLAUDE_DIR_NAME/' directory in this or any parent directory."
    echo "Add a docs/ directory to your project root — see the safecode README."
    exit 1
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
else
    echo "Warning: no $CLAUDE_DIR_NAME/AGENTS.md found — the agent will start without project context."
fi

# ── Resolve job directory ───────────────────────────────────────────────────────
AGENT_FLAG=()
JOB_PROMPT=""

if [[ -n "$JOB" ]]; then
    JOB_DIR="$PROJECT_ROOT/docs/processes/$JOB"
    if [[ ! -d "$JOB_DIR" ]]; then
        MATCH=$(find "$PROJECT_ROOT/docs/processes" -maxdepth 1 -type d -name "${JOB}*" 2>/dev/null | head -1 || true)
        if [[ -n "$MATCH" ]]; then
            JOB_DIR="$MATCH"
            JOB="$(basename "$MATCH")"
        else
            echo "Error: job '$JOB' not found under docs/processes/"
            exit 1
        fi
    fi
    CONTAINER_JOB_DIR="/workspace/docs/processes/$JOB"
    JOB_PROMPT="Please work on the job at ${CONTAINER_JOB_DIR} — start by reading brief.md"
fi

# --agent is passed as a proper CLI flag to claude, not as prompt text.
# This ensures Claude Code actually starts in agent mode rather than
# interpreting @agentname as a loose mention in a regular session.
if [[ -n "$AGENT" ]]; then
    AGENT_FLAG=(--agent "$AGENT")
fi

# Claude Code takes the initial prompt as a positional argument, OpenCode as --prompt.
PROMPT_ARGS=()
if [[ -n "$JOB_PROMPT" ]]; then
    if [[ "$TOOL" == "opencode" ]]; then
        PROMPT_ARGS=(--prompt "$JOB_PROMPT")
    else
        PROMPT_ARGS=("$JOB_PROMPT")
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
        echo "Remove it from your environment before running sc with --tool claude-code."
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
    echo "  Shadowing: $envfile → /dev/null inside container"
    ENV_MOUNTS+=(--mount "type=bind,source=/dev/null,target=$container_path,readonly")
done < <(find "$PROJECT_ROOT" -type f \( -name ".env" -o -name ".env.*" \) -print0)

if [[ ${#ENV_MOUNTS[@]} -eq 0 ]]; then
    echo "  Shadowed : none (no .env files found)"
fi

# ── Info ────────────────────────────────────────────────────────────────────────
echo "╔══════════════════════════════════════╗"
echo "║           safecode                   ║"
echo "╠══════════════════════════════════════╣"
echo "║  Project : $(basename "$PROJECT_ROOT")"
echo "║  Root    : $PROJECT_ROOT"
echo "║  Docs    : $PROJECT_DOCS_DIR"
[[ -n "$CONTEXT_FILE" ]] && echo "║  Context : $CONTEXT_FILE → $CONTEXT_TARGET"
[[ -n "$TOOL"  ]] && echo "║  Tool    : $TOOL"
[[ -n "$AGENT" ]] && echo "║  Agent   : $AGENT"
[[ -n "$JOB"   ]] && echo "║  Job     : $JOB"
echo "╚══════════════════════════════════════╝"
echo ""

# ── Run ─────────────────────────────────────────────────────────────────────────
docker run -it --rm \
    --name "safecode-$(basename "$PROJECT_ROOT")-$$" \
    -v "$PROJECT_ROOT:/workspace:z" \
    -v "$PROJECT_DOCS_DIR:$DOCS_MOUNT_TARGET:z" \
    "${CONTEXT_MOUNT[@]+"${CONTEXT_MOUNT[@]}"}" \
    "${ENV_MOUNTS[@]+"${ENV_MOUNTS[@]}"}" \
    "${KEY_ENV_ARGS[@]+"${KEY_ENV_ARGS[@]}"}" \
    -e CLAUDE_CODE_OAUTH_TOKEN="${CLAUDE_CODE_OAUTH_TOKEN:-}" \
    -e CLAUDE_ACCOUNT_UUID="${CLAUDE_ACCOUNT_UUID:-}" \
    -e CLAUDE_EMAIL="${CLAUDE_EMAIL:-}" \
    -e CLAUDE_ORG_UUID="${CLAUDE_ORG_UUID:-}" \
    -e GIT_AUTHOR_NAME_CFG="${GIT_AUTHOR_NAME:-}" \
    -e GIT_AUTHOR_EMAIL_CFG="${GIT_AUTHOR_EMAIL:-}" \
    -e SAFECODE_TOOL="$TOOL" \
    --network=bridge \
    --memory=2g \
    --security-opt=no-new-privileges \
    "$IMAGE_NAME" \
    "${AGENT_FLAG[@]+"${AGENT_FLAG[@]}"}" \
    "${PROMPT_ARGS[@]+"${PROMPT_ARGS[@]}"}" \
    "${PASSTHROUGH[@]+"${PASSTHROUGH[@]}"}"