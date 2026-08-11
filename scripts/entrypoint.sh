#!/usr/bin/env bash
set -euo pipefail

# Which agent CLI to start — set by run.sh from the resolved session profile
# (MANIGOT_TOOL: claude-code or opencode).
TOOL="${MANIGOT_TOOL:-claude-code}"

if [[ "$TOOL" == "claude-code" ]]; then
    # Write ~/.claude.json to bypass the onboarding wizard.
    # Without this, Claude Code opens a browser OAuth flow on every container start.
    # Values come from environment variables set by run.sh from your .env file.
    CLAUDE_JSON="$HOME/.claude.json"

    if [[ ! -f "$CLAUDE_JSON" ]]; then
        if [[ -z "${CLAUDE_ACCOUNT_UUID:-}" ]] || [[ -z "${CLAUDE_EMAIL:-}" ]] || [[ -z "${CLAUDE_ORG_UUID:-}" ]]; then
            echo "Error: CLAUDE_ACCOUNT_UUID, CLAUDE_EMAIL, and CLAUDE_ORG_UUID must be set."
            echo "Extract them from your host with:"
            echo "  cat ~/.claude.json | python3 -c \"import json,sys; d=json.load(sys.stdin); print(json.dumps(d.get('oauthAccount'), indent=2))\""
            echo "Then add them to your manigot/.env file."
            exit 1
        fi

        # Get Claude Code version for lastOnboardingVersion
        CLAUDE_VERSION=$(claude --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "2.1.0")

        # `projects["/workspace"].hasTrustDialogAccepted` pre-accepts the
        # folder-trust dialog for the container's fixed WORKDIR, so the "do you
        # trust the files in this folder?" prompt never appears on first start.
        # `bypassPermissionsModeAccepted` pre-accepts the one-time bypass
        # disclaimer so `--dangerously-skip-permissions` (passed to the exec
        # below) actually takes effect in interactive sessions — without it
        # Claude Code downgrades bypass mode to default with "bypass requires
        # accepting the disclaimer interactively first".
        cat > "$CLAUDE_JSON" <<EOF
{
  "hasCompletedOnboarding": true,
  "lastOnboardingVersion": "$CLAUDE_VERSION",
  "bypassPermissionsModeAccepted": true,
  "oauthAccount": {
    "accountUuid": "$CLAUDE_ACCOUNT_UUID",
    "emailAddress": "$CLAUDE_EMAIL",
    "organizationUuid": "$CLAUDE_ORG_UUID"
  },
  "projects": {
    "/workspace": {
      "hasTrustDialogAccepted": true
    }
  }
}
EOF
    fi
else
    # OpenCode has no onboarding wizard to bypass — it reads provider keys from
    # the environment on startup. Just make sure at least one of them is here,
    # otherwise it would start with no usable provider.
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

    HAVE_KEY=0
    for var in "${OPENCODE_KEY_VARS[@]}"; do
        [[ -n "${!var:-}" ]] && HAVE_KEY=1
    done

    if [[ "$HAVE_KEY" -eq 0 ]]; then
        echo "Error: no provider API key found for OpenCode."
        echo "Set at least one of these in your manigot/.env file:"
        printf '  %s\n' "${OPENCODE_KEY_VARS[@]}"
        exit 1
    fi

    # OpenCode doesn't read OPENCODE_MODEL directly — the model must come from a
    # config file. run.sh forwards OPENCODE_MODEL; emit a minimal global config
    # using {env:...} substitution so it actually takes effect. Without this
    # OpenCode boots with its built-in default model (often the wrong provider).
    OPENCODE_CFG="$HOME/.config/opencode/opencode.json"
    if [[ -n "${OPENCODE_MODEL:-}" && ! -f "$OPENCODE_CFG" ]]; then
        mkdir -p "$(dirname "$OPENCODE_CFG")"
        cat > "$OPENCODE_CFG" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "model": "{env:OPENCODE_MODEL}"
}
EOF
    fi
fi

# ── Git config ──────────────────────────────────────────────────────────────────
# GIT_AUTHOR_NAME/EMAIL env vars override gitconfig when set — even when empty.
# Unset them first so gitconfig always wins, then configure gitconfig from
# whatever values we have available.
unset GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL

GIT_NAME="${GIT_AUTHOR_NAME_CFG:-manigot}"
GIT_EMAIL="${GIT_AUTHOR_EMAIL_CFG:-manigot@localhost}"

git config --global user.name  "$GIT_NAME"
git config --global user.email "$GIT_EMAIL"

git config --global --add safe.directory /workspace

# --print mode expects a clean stdout stream (see run.sh and the
# --output-format json branch below) — this quote is purely cosmetic, so
# it's skipped there rather than risking a caller mis-parsing it as part
# of the agent's own output. Printed in italics (raw ANSI, not tput/ncurses
# — not guaranteed present in this slim image) with a trailing blank line.
if [[ "${MANIGOT_PRINT:-false}" != "true" && -n "${MANIGOT_QUOTE:-}" ]]; then
    printf '\033[3m"%s"\033[0m\n\n' "$MANIGOT_QUOTE"
fi

if [[ "$TOOL" == "opencode" ]]; then
    if [[ "${MANIGOT_PRINT:-false}" == "true" ]]; then
        # Non-interactive, one-shot mode (run.sh's --print flag, e.g. for
        # mg-jdi) — mirrors the claude-code branch below. OpenCode's
        # interactive `opencode [project]` command (the "$@" passthrough
        # used otherwise) takes --agent/--prompt as flags, but has no
        # non-interactive equivalent of its own; the headless mode is a
        # separate subcommand, `opencode run [message..]`, which takes the
        # prompt as a positional argument instead and supports its own
        # --agent flag (confirmed via `opencode run --help` against the
        # opencode-ai version installed from the Dockerfile's unpinned `npm
        # install -g opencode-ai`). Translate the incoming --agent/--prompt
        # pair into that shape rather than passing "$@" through unchanged.
        # --format json makes it emit one JSON object per line (JSONL: a
        # stream of step_start/tool_use/text/step_finish events, each
        # reporting the assistant's response in a "text"-typed event's
        # part.text) instead of interactive TUI output — parsed by
        # tui/internal/orchestrate.ResultText/DetectSignal the same way
        # Claude's --output-format json "result" field is.
        OC_AGENT=""
        OC_PROMPT=""
        OC_REST=()
        while [[ $# -gt 0 ]]; do
            case "$1" in
                --agent)  OC_AGENT="$2";  shift 2 ;;
                --prompt) OC_PROMPT="$2"; shift 2 ;;
                *)        OC_REST+=("$1"); shift ;;
            esac
        done

        OC_ARGS=(run)
        [[ -n "$OC_PROMPT" ]] && OC_ARGS+=("$OC_PROMPT")
        [[ -n "$OC_AGENT" ]] && OC_ARGS+=(--agent "$OC_AGENT")
        # --auto makes the headless run explicitly auto-approved: the archived
        # foycfl job verified `opencode run` auto-executes bash/write tool
        # calls even without it, but the flag makes the intent explicit and
        # guards other tools (webfetch, task, lsp, mcp) against an unanswered
        # "ask" prompt stalling an unattended non-TTY run — the headless
        # counterpart of the interactive --auto above and of Claude's
        # --dangerously-skip-permissions on its own --print branch.
        OC_ARGS+=(--auto --format json)
        exec opencode "${OC_ARGS[@]}" "${OC_REST[@]+"${OC_REST[@]}"}"
    fi
    # --auto starts every OpenCode session in full auto mode (no per-tool
    # confirmation, e.g. "can I run this python script?"). Safe in this context
    # because manigot launches an isolated, ephemeral container specifically for
    # this purpose; the brief explicitly wants OpenCode to start in auto mode,
    # mirroring the claude-code branch's --dangerously-skip-permissions below.
    # Placed before "$@" so it composes with the passthrough (--agent <name>,
    # --prompt <text> and/or the positional job prompt).
    exec opencode --auto "$@"
else
    # --dangerously-skip-permissions starts every Claude Code session in full
    # auto mode (no per-tool-call confirmation). Safe in this context because
    # manigot launches an isolated, ephemeral container specifically for this
    # purpose; the brief explicitly wants Claude Code to start in auto mode.
    # Placed before "$@" so it composes with the passthrough (--agent <name>
    # and/or the positional job prompt).
    if [[ "${MANIGOT_PRINT:-false}" == "true" ]]; then
        # Non-interactive, one-shot mode (run.sh's --print flag, e.g. for
        # mg-jdi): no attached terminal, so the caller gets the agent's final
        # response back on stdout instead. --output-format json is used
        # (confirmed supported by the pinned claude version, whose result is
        # a single JSON object with a "result" field carrying the final
        # response text) so callers parse a clean field instead of scanning
        # raw combined stdout, which could false-positive on the marker text
        # (see docs/AGENTS.md) appearing incidentally inside a tool call's
        # own output (e.g. a grep result).
        exec claude --dangerously-skip-permissions --print --output-format json "$@"
    fi
    exec claude --dangerously-skip-permissions "$@"
fi