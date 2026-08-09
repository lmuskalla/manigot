#!/usr/bin/env bash
set -euo pipefail

# Which agent CLI to start — set by run.sh from its --tool flag.
TOOL="${SAFECODE_TOOL:-claude-code}"

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
            echo "Then add them to your safecode/.env file."
            exit 1
        fi

        # Get Claude Code version for lastOnboardingVersion
        CLAUDE_VERSION=$(claude --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "2.1.0")

        cat > "$CLAUDE_JSON" <<EOF
{
  "hasCompletedOnboarding": true,
  "lastOnboardingVersion": "$CLAUDE_VERSION",
  "oauthAccount": {
    "accountUuid": "$CLAUDE_ACCOUNT_UUID",
    "emailAddress": "$CLAUDE_EMAIL",
    "organizationUuid": "$CLAUDE_ORG_UUID"
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
        echo "Set at least one of these in your safecode/.env file:"
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

GIT_NAME="${GIT_AUTHOR_NAME_CFG:-${CLAUDE_EMAIL:-safecode}}"
GIT_EMAIL="${GIT_AUTHOR_EMAIL_CFG:-${CLAUDE_EMAIL:-safecode@localhost}}"

git config --global user.name  "$GIT_NAME"
git config --global user.email "$GIT_EMAIL"

git config --global --add safe.directory /workspace

if [[ "$TOOL" == "opencode" ]]; then
    exec opencode "$@"
else
    exec claude "$@"
fi