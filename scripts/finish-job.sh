#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# sc-done "go61h7"
# sc-done "go61h7_add-backup-package"
#
# Installed as `sc-done`. See `make install`.

# ── Configuration ───────────────────────────────────────────────────────────────
PROCESSES_DIR="docs/processes"
ARCHIVE_DIR="docs/processes/archive"

# ── Parse args ──────────────────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
    echo "Usage: sc-done <job-id-or-slug>"
    exit 1
fi

JOB_ARG="$1"

# ── Resolve project root ────────────────────────────────────────────────────────
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
PROJECT_ROOT="${PROJECT_ROOT%/}"

if [[ -z "$PROJECT_ROOT" ]]; then
    echo "Error: could not find project root (no docs/ directory found)."
    exit 1
fi

# ── Resolve job directory ───────────────────────────────────────────────────────
JOB_DIR=""
JOB_NAME=""

# Exact match first
if [[ -d "$PROJECT_ROOT/$PROCESSES_DIR/$JOB_ARG" ]]; then
    JOB_DIR="$PROJECT_ROOT/$PROCESSES_DIR/$JOB_ARG"
    JOB_NAME="$JOB_ARG"
else
    # Partial match on ID prefix
    MATCH=$(find "$PROJECT_ROOT/$PROCESSES_DIR" -maxdepth 1 -type d -name "${JOB_ARG}*" 2>/dev/null | grep -v '/archive' | head -1 || true)
    if [[ -n "$MATCH" ]]; then
        JOB_DIR="$MATCH"
        JOB_NAME="$(basename "$MATCH")"
    else
        echo "Error: job '$JOB_ARG' not found under $PROCESSES_DIR/"
        echo "Active jobs:"
        find "$PROJECT_ROOT/$PROCESSES_DIR" -maxdepth 1 -type d ! -name "archive" ! -name "processes" | sort | while read -r d; do
            echo "  $(basename "$d")"
        done
        exit 1
    fi
fi

# ── Read job metadata ───────────────────────────────────────────────────────────
BRIEF="$JOB_DIR/brief.md"
VERDICT="$JOB_DIR/verdict.md"

if [[ ! -f "$BRIEF" ]]; then
    echo "Error: brief.md not found in $JOB_DIR"
    exit 1
fi

# Extract branch from brief.md
BRANCH=$(grep '^branch:' "$BRIEF" | awk '{print $2}' || true)
if [[ -z "$BRANCH" ]]; then
    echo "Error: could not read branch from brief.md"
    exit 1
fi

# ── Check verdict ───────────────────────────────────────────────────────────────
if [[ -f "$VERDICT" ]]; then
    OVERALL=$(grep -i '^## Overall' -A5 "$VERDICT" | grep -iE 'APPROVED|REJECTED|NEEDS WORK' | head -1 || true)
    if [[ -z "$OVERALL" ]]; then
        echo "Warning: could not determine verdict status from verdict.md"
        read -rp "  Continue anyway? [y/N] " CONFIRM
        [[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0
    elif echo "$OVERALL" | grep -iqE 'REJECTED|NEEDS WORK'; then
        echo "Error: verdict is '$OVERALL' — job is not approved."
        echo "Resolve all issues and re-run @reviewer before finishing."
        exit 1
    fi
else
    echo "Warning: no verdict.md found — job has not been reviewed."
    read -rp "  Continue anyway? [y/N] " CONFIRM
    [[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0
fi

# ── Git checks ──────────────────────────────────────────────────────────────────
CURRENT_BRANCH=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD)
DEFAULT_BRANCH=$(git -C "$PROJECT_ROOT" symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@' || echo "main")

if [[ "$CURRENT_BRANCH" != "$BRANCH" ]]; then
    echo "Warning: you are on '$CURRENT_BRANCH', job branch is '$BRANCH'"
    read -rp "  Switch to job branch before merging? [Y/n] " CONFIRM
    if [[ ! "$CONFIRM" =~ ^[Nn]$ ]]; then
        git -C "$PROJECT_ROOT" checkout "$BRANCH"
    fi
fi

# Check for uncommitted changes
if ! git -C "$PROJECT_ROOT" diff --quiet || ! git -C "$PROJECT_ROOT" diff --cached --quiet; then
    echo "Error: uncommitted changes on branch '$BRANCH'. Commit or stash before finishing."
    exit 1
fi

# ── Info ────────────────────────────────────────────────────────────────────────
echo ""
echo "Finishing job: $JOB_NAME"
echo "  Branch  : $BRANCH → $DEFAULT_BRANCH"
echo "  Archive : $PROCESSES_DIR/archive/$JOB_NAME"
echo ""
read -rp "  Proceed? [y/N] " CONFIRM
[[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0

# ── Merge ───────────────────────────────────────────────────────────────────────
echo ""
echo "→ Switching to $DEFAULT_BRANCH..."
git -C "$PROJECT_ROOT" checkout "$DEFAULT_BRANCH"

echo "→ Merging $BRANCH..."
git -C "$PROJECT_ROOT" merge --no-ff "$BRANCH" -m "Merge $BRANCH

Job: $JOB_NAME"

echo "→ Deleting branch $BRANCH..."
git -C "$PROJECT_ROOT" branch -d "$BRANCH"

# ── Archive job directory ────────────────────────────────────────────────────────
echo "→ Archiving job directory..."
mkdir -p "$PROJECT_ROOT/$ARCHIVE_DIR"
mv "$JOB_DIR" "$PROJECT_ROOT/$ARCHIVE_DIR/$JOB_NAME"

# ── Update status in brief.md ───────────────────────────────────────────────────
sed -i "s/^status: .*/status: done/" "$PROJECT_ROOT/$ARCHIVE_DIR/$JOB_NAME/brief.md"

# ── Commit the archive move ──────────────────────────────────────────────────────
git -C "$PROJECT_ROOT" add "$PROJECT_ROOT/$ARCHIVE_DIR/$JOB_NAME"
git -C "$PROJECT_ROOT" add "$PROJECT_ROOT/$PROCESSES_DIR" 2>/dev/null || true
git -C "$PROJECT_ROOT" commit -m "archive: $JOB_NAME"

# ── Done ────────────────────────────────────────────────────────────────────────
echo ""
echo "✓ Job finished: $JOB_NAME"
echo "  Merged into : $DEFAULT_BRANCH"
echo "  Archived at : $ARCHIVE_DIR/$JOB_NAME"