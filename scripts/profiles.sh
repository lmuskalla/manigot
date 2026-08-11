#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg profiles                 # list the profiles, then pick the default (TTY)
# mg profiles <name>          # set the default profile for bare `mg` runs
#
# The default is stored as MANIGOT_PROFILE in manigot/.env — the same file
# scripts/run.sh sources, so a bare `mg` (no --profile/--tool flag) resolves to
# it. It is the one shared default: the TUI's settings screen reads and writes
# the same value, so a profile switched here is what TUI-launched sessions use,
# and a profile switched in the TUI is what bare `mg` uses.
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

# ── Load existing .env so we know the active profile and key status ─────────────
if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck source=/dev/null
    source "$ENV_FILE"
    set +a
fi

# ── Profile table ───────────────────────────────────────────────────────────────
declare -A PROF_LABEL PROF_TOOL PROF_MODEL PROF_AUTH
PROF_LABEL[claude-pro]="Claude Code · Claude Pro"
PROF_TOOL[claude-pro]="claude-code"
PROF_MODEL[claude-pro]="(Claude Code default)"
PROF_AUTH[claude-pro]="CLAUDE_CODE_OAUTH_TOKEN"

PROF_LABEL[zai]="OpenCode · Z.AI Coding Plan"
PROF_TOOL[zai]="opencode"
PROF_MODEL[zai]="${OPENCODE_ZAI_MODEL:-zai-coding-plan/glm-5.2}"
PROF_AUTH[zai]="ZHIPU_API_KEY"

PROF_LABEL[opencode-go]="OpenCode · Go"
PROF_TOOL[opencode-go]="opencode"
PROF_MODEL[opencode-go]="${OPENCODE_GO_MODEL:-opencode-go/glm-5.2}"
PROF_AUTH[opencode-go]="OPENCODE_API_KEY"

ORDER=(claude-pro zai opencode-go)
ACTIVE="${MANIGOT_PROFILE:-claude-pro}"

# ── Help ────────────────────────────────────────────────────────────────────────
case "${1:-}" in
    -h|--help|help)
        cat <<'EOF'
mg profiles [name]

Lists manigot's subscription profiles — claude-pro, zai, opencode-go — showing
which are configured and which is the default used by bare `mg`. With a name,
sets that profile as the default (written as MANIGOT_PROFILE in manigot/.env);
with no name and an interactive terminal, prompts to select the default after
listing.

The default is shared: the TUI's settings screen reads and writes the same
MANIGOT_PROFILE, so TUI-launched sessions use whatever this command sets, and
vice versa.
EOF
        exit 0
        ;;
esac

# ── set_default_profile NAME — write MANIGOT_PROFILE=<name> into .env ───────────
# Upserts the value, preserving every other line (.env may hold credentials and
# comments the user wrote). NAME must already be validated by the caller; it is
# a known profile id, so it is safe both as an awk regex and as literal output.
set_default_profile() {
    local target="$1"
    if [[ ! -f "$ENV_FILE" ]]; then
        echo "# manigot configuration — credentials and defaults (never commit this file)" > "$ENV_FILE"
    fi
    if grep -q '^MANIGOT_PROFILE=' "$ENV_FILE"; then
        awk -v v="$target" '
            BEGIN { done = 0 }
            !done && $0 ~ /^MANIGOT_PROFILE=/ { print "MANIGOT_PROFILE=" v; done = 1; next }
            { print }
            END { if (!done) print "MANIGOT_PROFILE=" v }
        ' "$ENV_FILE" > "$ENV_FILE.tmp" && mv "$ENV_FILE.tmp" "$ENV_FILE"
    else
        echo "MANIGOT_PROFILE=$target" >> "$ENV_FILE"
    fi
}

# confirm_set NAME — echo the set confirmation + missing-credentials warning,
# shared by the positional and interactive set paths.
confirm_set() {
    local target="$1"
    echo "Default profile set to $target (MANIGOT_PROFILE in $ENV_FILE)."
    echo "Bare \`mg\` sessions and TUI-launched sessions share this default."

    KEY="${PROF_AUTH[$target]}"
    if [[ -z "${!KEY:-}" ]]; then
        echo "Warning: $KEY is not set in $ENV_FILE — run 'mg setup $target' first, or sessions will fail at launch."
    fi
}

# ── Set mode (positional) ───────────────────────────────────────────────────────
if [[ $# -gt 0 ]]; then
    TARGET="$1"
    shift
    if [[ $# -gt 0 ]]; then
        echo "Error: too many arguments." >&2
        echo "Usage: mg profiles [name]" >&2
        exit 1
    fi

    case "$TARGET" in
        claude-pro|zai|opencode-go) ;;
        *)
            echo "Error: unknown profile '$TARGET'." >&2
            echo "Valid profiles: ${ORDER[*]}" >&2
            exit 1
            ;;
    esac

    set_default_profile "$TARGET"
    confirm_set "$TARGET"
    exit 0
fi

# ── List mode ───────────────────────────────────────────────────────────────────
echo "Active default: $ACTIVE   (shared with the TUI; switch with: mg profiles <name>, or pick one below)"
echo ""

printf "  %-13s %-28s %-10s %-26s %s\n" "profile" "label" "tool" "model" "creds"
printf "  %-13s %-28s %-10s %-26s %s\n" "---------" "----------------------------" "----------" "--------------------------" "-----"
for p in "${ORDER[@]}"; do
    mark=" "
    [[ "$p" == "$ACTIVE" ]] && mark="*"

    creds="✓ ready"
    KEY="${PROF_AUTH[$p]}"
    case "$p" in
        claude-pro)
            for k in CLAUDE_CODE_OAUTH_TOKEN CLAUDE_ACCOUNT_UUID CLAUDE_EMAIL CLAUDE_ORG_UUID; do
                [[ -z "${!k:-}" ]] && { creds="✗ missing $k"; break; }
            done
            ;;
        *)
            [[ -z "${!KEY:-}" ]] && creds="✗ missing $KEY"
            ;;
    esac

    printf "  %s%-12s %-28s %-10s %-26s %s\n" \
        "$mark" "$p" "${PROF_LABEL[$p]}" "${PROF_TOOL[$p]}" "${PROF_MODEL[$p]}" "$creds"
done

echo ""
echo "  * = default. Configure credentials with: mg setup [name]"

# ── Interactive selection (TTY only) ───────────────────────────────────────────
# Bare `mg profiles` on an interactive terminal lets the user pick the default
# right there; piped/non-interactive invocations just get the listing above.
if [[ ! -t 0 ]]; then
    exit 0
fi

echo ""
echo "Select the default profile (shared with the TUI):"
while true; do
    read -rp "  [1-${#ORDER[@]}, Enter keeps $ACTIVE, q quits]: " SELECTION
    if [[ -z "$SELECTION" ]]; then
        echo "Keeping $ACTIVE."
        exit 0
    fi
    case "$SELECTION" in
        q|Q|quit|exit)
            exit 0
            ;;
    esac
    if [[ "$SELECTION" =~ ^[0-9]+$ ]] && (( SELECTION >= 1 && SELECTION <= ${#ORDER[@]} )); then
        TARGET="${ORDER[$((SELECTION - 1))]}"
        break
    fi
    echo "Enter a number between 1 and ${#ORDER[@]}, or q to quit."
done

echo ""
set_default_profile "$TARGET"
confirm_set "$TARGET"
