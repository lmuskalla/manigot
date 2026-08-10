#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg agents                       # list agents, pick one, start a session in it
# mg agents --profile zai         # ...anything else is passed through to run.sh
#
# Reached via `mg agents`. Lists every agent available to the current project —
# manigot's global `agents/*.md`, with any `docs/agents/` overrides or
# project-only additions swapped in when the project is initialized — lets you
# pick one interactively, then hands off to run.sh with `--agent <name>` (all
# other args, e.g. `--profile`, passed through unchanged).

# ── Resolve repo ────────────────────────────────────────────────────────────────
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
GLOBAL_AGENTS_DIR="$MANIGOT_ROOT/agents"

if [[ ! -d "$GLOBAL_AGENTS_DIR" ]]; then
    echo "Error: no agents/ directory found at $GLOBAL_AGENTS_DIR" >&2
    exit 1
fi

# ── Resolve project root ────────────────────────────────────────────────────────
# Same walk-up-to-docs/ used by run.sh. Optional — an uninitialized project
# (no docs/) still gets the global agent list, just no docs/agents/ overrides.
find_project_root() {
    local dir="$PWD"
    while [[ "$dir" != "/" ]]; do
        if [[ -d "$dir/docs" ]]; then
            echo "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo ""
}
PROJECT_ROOT="$(find_project_root)"
PROJECT_AGENTS_DIR=""
[[ -n "$PROJECT_ROOT" ]] && PROJECT_AGENTS_DIR="${PROJECT_ROOT%/}/docs/agents"

# ── Build the agent list ────────────────────────────────────────────────────────
# Global agents first, in the order `agents/*.md` sorts to, each swapped for its
# docs/agents/ override (same filename) when one exists; then any project-only
# agents that don't shadow a global name, appended after. Mirrors the
# override precedence documented in docs/AGENTS.md.
NAMES=()
FILES=()
SOURCES=()

for f in "$GLOBAL_AGENTS_DIR"/*.md; do
    [[ -e "$f" ]] || continue
    name="$(basename "$f" .md)"
    if [[ -n "$PROJECT_AGENTS_DIR" && -f "$PROJECT_AGENTS_DIR/$name.md" ]]; then
        NAMES+=("$name")
        FILES+=("$PROJECT_AGENTS_DIR/$name.md")
        SOURCES+=("project override")
    else
        NAMES+=("$name")
        FILES+=("$f")
        SOURCES+=("global")
    fi
done

if [[ -n "$PROJECT_AGENTS_DIR" && -d "$PROJECT_AGENTS_DIR" ]]; then
    for f in "$PROJECT_AGENTS_DIR"/*.md; do
        [[ -e "$f" ]] || continue
        name="$(basename "$f" .md)"
        if [[ ! -f "$GLOBAL_AGENTS_DIR/$name.md" ]]; then
            NAMES+=("$name")
            FILES+=("$f")
            SOURCES+=("project")
        fi
    done
fi

if [[ ${#NAMES[@]} -eq 0 ]]; then
    echo "Error: no agents found under $GLOBAL_AGENTS_DIR" >&2
    exit 1
fi

# ── Describe each ────────────────────────────────────────────────────────────────
describe() {
    local file="$1" desc
    desc="$(sed -n '/^description:/s/^description: *//p' "$file" | head -1)"
    echo "${desc:-(no description)}"
}

# ── Present the menu ─────────────────────────────────────────────────────────────
echo "Available agents:"
echo ""
for i in "${!NAMES[@]}"; do
    n=$((i + 1))
    tag=""
    [[ "${SOURCES[$i]}" != "global" ]] && tag=" (${SOURCES[$i]})"
    printf "  %2d) %-14s %s%s\n" "$n" "${NAMES[$i]}" "$(describe "${FILES[$i]}")" "$tag"
done
echo ""

if [[ ! -t 0 ]]; then
    echo "Error: mg agents needs an interactive terminal to select an agent." >&2
    exit 1
fi

SELECTION=""
while true; do
    read -rp "Select an agent [1-${#NAMES[@]}]: " SELECTION
    if [[ "$SELECTION" =~ ^[0-9]+$ ]] && (( SELECTION >= 1 && SELECTION <= ${#NAMES[@]} )); then
        break
    fi
    echo "Enter a number between 1 and ${#NAMES[@]}."
done

CHOSEN="${NAMES[$((SELECTION - 1))]}"
echo ""
echo "→ Starting a session in @$CHOSEN..."
echo ""

exec "$SCRIPT_DIR/run.sh" --agent "$CHOSEN" "$@"
