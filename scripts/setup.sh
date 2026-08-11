#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg setup                    # interactive wizard for all three profiles
# mg setup <name>             # interactive wizard for one profile (claude-pro|zai|opencode-go)
# mg setup --check            # non-interactive: which profiles are ready to use
# mg setup --help             # this help
#
# Walks you through getting each subscription's credentials into manigot/.env,
# auto-applying whatever it can read off this host (e.g. your Claude account
# details from ~/.claude.json) and pasting the rest. Values are written to the
# same .env scripts/run.sh sources; nothing is sent anywhere.
#
# Keep the profile table below in sync with tui/internal/config/config.go and
# the one in scripts/run.sh.

# ── Resolve repo ────────────────────────────────────────────────────────────────
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

# ── Load existing .env so we know what's already set ────────────────────────────
if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck source=/dev/null
    source "$ENV_FILE"
    set +a
fi

# ── Helpers ─────────────────────────────────────────────────────────────────────
# have NAME — true if the env var NAME is set to a non-empty value.
have() { [[ -n "${!1:-}" ]]; }

# mask — a short, safe display form of a secret.
mask() {
    local v="$1"
    if [[ ${#v} -le 8 ]]; then
        echo "****"
    else
        echo "${v:0:4}…${v: -4}"
    fi
}

# set_env_var NAME VALUE — upsert NAME=VALUE into .env, preserving every other
# line (comments, other credentials). Refuses to write an empty value.
set_env_var() {
    local name="$1" value="$2"
    [[ -z "$value" ]] && return 1
    if [[ ! -f "$ENV_FILE" ]]; then
        echo "# manigot configuration — credentials and defaults (never commit this file)" > "$ENV_FILE"
    fi
    if grep -q "^${name}=" "$ENV_FILE"; then
        awk -v n="$name" -v v="$value" '
            BEGIN { done = 0 }
            !done && $0 ~ "^" n "=" { print n "=" v; done = 1; next }
            { print }
            END { if (!done) print n "=" v }
        ' "$ENV_FILE" > "$ENV_FILE.tmp" && mv "$ENV_FILE.tmp" "$ENV_FILE"
    else
        echo "${name}=${value}" >> "$ENV_FILE"
    fi
}

# prompt_secret LABEL NAME — ask for a secret, keep the current value on Enter.
prompt_secret() {
    local label="$1" name="$2" val=""
    if have "$name"; then
        read -r -p "  $label [currently $(mask "${!name}") — Enter keeps it]: " val || val=""
        [[ -n "$val" ]] && set_env_var "$name" "$val"
    else
        read -r -p "  $label: " val || val=""
        if [[ -n "$val" ]]; then
            set_env_var "$name" "$val"
        else
            echo "  (skipped — $name not set)"
        fi
    fi
}

# prompt_value LABEL NAME [DEFAULT] — like prompt_secret but for non-secrets,
# showing a default the user can accept with Enter.
prompt_value() {
    local label="$1" name="$2" default="${3:-}" val="" shown=""
    shown="${!name:-$default}"
    read -r -p "  $label [${shown:-empty}] — Enter keeps it: " val || val=""
    if [[ -n "$val" ]]; then
        set_env_var "$name" "$val"
    elif [[ -n "${!name:-}" ]]; then
        :
    elif [[ -n "$default" ]]; then
        set_env_var "$name" "$default"
    fi
}

# extract_claude_account — reads accountUuid/emailAddress/organizationUuid from
# ~/.claude.json (the host's Claude Code config) when possible, printed as
# three lines. Returns non-zero when it can't.
extract_claude_account() {
    local cfg="$HOME/.claude.json"
    [[ -f "$cfg" ]] || return 1
    command -v python3 >/dev/null 2>&1 || return 1
    python3 - "$cfg" <<'PY' 2>/dev/null || return 1
import json, sys
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(1)
a = d.get("oauthAccount") or {}
if not a:
    sys.exit(1)
print(a.get("accountUuid", ""))
print(a.get("emailAddress", ""))
print(a.get("organizationUuid", ""))
PY
}

# ── Per-profile setup ───────────────────────────────────────────────────────────

setup_claude_pro() {
    echo "────────────────────────────────────────────────────────────────────"
    echo "  claude-pro — Claude Code, billed to your Claude Pro/Max subscription"
    echo "────────────────────────────────────────────────────────────────────"
    if have CLAUDE_CODE_OAUTH_TOKEN && have CLAUDE_ACCOUNT_UUID && have CLAUDE_EMAIL && have CLAUDE_ORG_UUID; then
        echo "  ✓ Already configured (token $(mask "$CLAUDE_CODE_OAUTH_TOKEN"), $CLAUDE_EMAIL)."
        return 0
    fi
    echo "  Claude Code runs with your subscription's OAuth credentials. You need"
    echo "  a token plus three account details."

    if ! have CLAUDE_CODE_OAUTH_TOKEN; then
        echo ""
        echo "  Step 1 — the OAuth token. On your HOST, with Claude Code installed"
        echo "  locally, run:"
        echo "      claude setup-token"
        echo "  Paste the 'sk-ant-oat01-…' token it prints below."
        prompt_secret "  CLAUDE_CODE_OAUTH_TOKEN" CLAUDE_CODE_OAUTH_TOKEN
    else
        echo "  ✓ CLAUDE_CODE_OAUTH_TOKEN already set ($(mask "$CLAUDE_CODE_OAUTH_TOKEN"))."
    fi

    if ! ( have CLAUDE_ACCOUNT_UUID && have CLAUDE_EMAIL && have CLAUDE_ORG_UUID ); then
        echo ""
        echo "  Step 2 — account details (CLAUDE_ACCOUNT_UUID, CLAUDE_EMAIL,"
        echo "  CLAUDE_ORG_UUID)."
        local acct uuid email org
        if acct="$(extract_claude_account)" && [[ -n "$acct" ]]; then
            uuid="$(sed -n '1p' <<<"$acct")"
            email="$(sed -n '2p' <<<"$acct")"
            org="$(sed -n '3p' <<<"$acct")"
            if [[ -n "$uuid" && -n "$email" && -n "$org" ]]; then
                echo "  Found them in ~/.claude.json on this host — applying automatically."
                set_env_var CLAUDE_ACCOUNT_UUID "$uuid"
                set_env_var CLAUDE_EMAIL "$email"
                set_env_var CLAUDE_ORG_UUID "$org"
                return 0
            fi
        fi
        echo "  Could not read them from ~/.claude.json here. On the host where"
        echo "  Claude Code is logged in, extract them with:"
        echo "      cat ~/.claude.json | python3 -c \"import json,sys; d=json.load(sys.stdin); print(json.dumps(d.get('oauthAccount'), indent=2))\""
        echo ""
        prompt_value "  CLAUDE_ACCOUNT_UUID" CLAUDE_ACCOUNT_UUID
        prompt_value "  CLAUDE_EMAIL" CLAUDE_EMAIL
        prompt_value "  CLAUDE_ORG_UUID" CLAUDE_ORG_UUID
    fi
}

setup_zai() {
    echo "────────────────────────────────────────────────────────────────────"
    echo "  zai — OpenCode, billed to your Z.AI Coding Plan"
    echo "────────────────────────────────────────────────────────────────────"
    if have ZHIPU_API_KEY; then
        echo "  ✓ Already configured (ZHIPU_API_KEY $(mask "$ZHIPU_API_KEY"))."
    else
        echo "  OpenCode authenticates to Z.AI with an API key from your Z.AI"
        echo "  Coding Plan. Get one from the Z.AI / BigModel console"
        echo "  (https://www.bigmodel.cn), then paste it below."
        prompt_secret "  ZHIPU_API_KEY" ZHIPU_API_KEY
    fi
    echo ""
    echo "  Optional — the model this profile defaults to, as provider/model."
    prompt_value "  OPENCODE_ZAI_MODEL" OPENCODE_ZAI_MODEL "zai-coding-plan/glm-5.2"
}

setup_opencode_go() {
    echo "────────────────────────────────────────────────────────────────────"
    echo "  opencode-go — OpenCode, billed to the OpenCode Go subscription"
    echo "────────────────────────────────────────────────────────────────────"
    if have OPENCODE_API_KEY; then
        echo "  ✓ Already configured (OPENCODE_API_KEY $(mask "$OPENCODE_API_KEY"))."
    else
        echo "  OpenCode Go uses your OpenCode API key — the same key you get at"
        echo "  https://opencode.ai/auth, billed against your Go subscription."
        echo ""
        echo "  1. Open https://opencode.ai/auth and sign in"
        echo "  2. Subscribe to OpenCode Go (if you haven't already)"
        echo "  3. Copy your API key and paste it below"
        prompt_secret "  OPENCODE_API_KEY" OPENCODE_API_KEY
    fi
    echo ""
    echo "  Optional — the model this profile defaults to, as provider/model."
    echo "  Go model ids use the opencode-go/ prefix (e.g. opencode-go/deepseek-v4-flash)."
    prompt_value "  OPENCODE_GO_MODEL" OPENCODE_GO_MODEL "opencode-go/glm-5.2"
}

# ── --check mode ────────────────────────────────────────────────────────────────
check_profile() {
    local p="$1" missing=() k
    case "$p" in
        claude-pro)
            for k in CLAUDE_CODE_OAUTH_TOKEN CLAUDE_ACCOUNT_UUID CLAUDE_EMAIL CLAUDE_ORG_UUID; do
                have "$k" || missing+=("$k")
            done
            ;;
        zai)        have ZHIPU_API_KEY || missing+=(ZHIPU_API_KEY) ;;
        opencode-go) have OPENCODE_API_KEY || missing+=(OPENCODE_API_KEY) ;;
    esac
    if [[ ${#missing[@]} -eq 0 ]]; then
        printf "  \u2713 %-12s ready\n" "$p"
    else
        printf "  \u2717 %-12s missing: %s   (fix with: mg setup %s)\n" "$p" "${missing[*]}" "$p"
    fi
}

# ── Main ────────────────────────────────────────────────────────────────────────
usage() {
    cat <<'EOF'
mg setup [profile] [--check]

Configures credentials for manigot's subscription profiles into manigot/.env:
  claude-pro    Claude Code, billed to your Claude Pro/Max subscription
  zai           OpenCode, billed to your Z.AI Coding Plan
  opencode-go   OpenCode, billed to the OpenCode Go subscription

With no profile, the wizard walks through all three. --check reports which
profiles are ready without prompting. Values are written to the same .env
scripts/run.sh sources; nothing is sent anywhere.
EOF
}

CHECK="false"
TARGET=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --check) CHECK="true"; shift ;;
        -h|--help|help) usage; exit 0 ;;
        claude-pro|zai|opencode-go)
            if [[ -n "$TARGET" ]]; then
                echo "Error: give a single profile, not several." >&2
                exit 1
            fi
            TARGET="$1"; shift ;;
        *)
            echo "Error: unknown argument '$1'." >&2
            echo "Usage: mg setup [claude-pro|zai|opencode-go] [--check]" >&2
            exit 1
            ;;
    esac
done

if [[ "$CHECK" == "true" ]]; then
    if [[ -n "$TARGET" ]]; then
        check_profile "$TARGET"
    else
        check_profile claude-pro
        check_profile zai
        check_profile opencode-go
    fi
    exit 0
fi

if [[ ! -t 0 ]]; then
    echo "mg setup: interactive setup needs a terminal." >&2
    echo "Use 'mg setup --check' for a non-interactive status report." >&2
    exit 1
fi

if [[ -n "$TARGET" ]]; then
    setup_one="$TARGET"
    case "$setup_one" in claude-pro) setup_claude_pro ;; zai) setup_zai ;; opencode-go) setup_opencode_go ;; esac
else
    setup_claude_pro
    setup_zai
    setup_opencode_go
fi

echo ""
echo "  Done. Switch the default with: mg profiles <name>"
echo "  or start a one-off session with:  mg --profile <name>"
