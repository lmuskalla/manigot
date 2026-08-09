#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# sc-done "go61h7"
# sc-done "go61h7_add-backup-package"
#
# Installed as `sc-done`. See `make install`.

# ── Configuration ───────────────────────────────────────────────────────────────
JOBS_DIR="docs/jobs"
ARCHIVE_DIR="docs/jobs/archive"

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
if [[ -d "$PROJECT_ROOT/$JOBS_DIR/$JOB_ARG" ]]; then
    JOB_DIR="$PROJECT_ROOT/$JOBS_DIR/$JOB_ARG"
    JOB_NAME="$JOB_ARG"
else
    # Partial match on ID prefix
    MATCH=$(find "$PROJECT_ROOT/$JOBS_DIR" -maxdepth 1 -type d -name "${JOB_ARG}*" 2>/dev/null | grep -v '/archive' | head -1 || true)
    if [[ -n "$MATCH" ]]; then
        JOB_DIR="$MATCH"
        JOB_NAME="$(basename "$MATCH")"
    else
        echo "Error: job '$JOB_ARG' not found under $JOBS_DIR/"
        echo "Active jobs:"
        find "$PROJECT_ROOT/$JOBS_DIR" -maxdepth 1 -type d ! -name "archive" ! -name "jobs" | sort | while read -r d; do
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
        echo "Warning: verdict is '$OVERALL' — job is not approved."
        read -rp "  Continue anyway? [y/N] " CONFIRM
        [[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0
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
    echo "Note: you are on '$CURRENT_BRANCH', job branch is '$BRANCH'"
    echo "→ Switching to job branch $BRANCH..."
    git -C "$PROJECT_ROOT" checkout "$BRANCH"
fi

# Check for uncommitted changes (archiving + merging both need a clean tree)
if ! git -C "$PROJECT_ROOT" diff --quiet || ! git -C "$PROJECT_ROOT" diff --cached --quiet; then
    echo "Error: uncommitted changes on branch '$BRANCH'. Commit or stash before finishing."
    exit 1
fi

# ── Info ────────────────────────────────────────────────────────────────────────
JOB_TITLE=$(head -1 "$BRIEF" | sed 's/^# Brief: *//')
echo ""
echo "Finishing job: $JOB_NAME"
echo "  Branch  : $BRANCH → $DEFAULT_BRANCH"
echo "  Archive : $JOBS_DIR/archive/$JOB_NAME"
echo ""
read -rp "  Proceed? [y/N] " CONFIRM
[[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0

# ── Archive on the job branch first ─────────────────────────────────────────────
# The job directory is moved and committed on the job branch so the squash merge
# below folds the archive move into a single commit on the default branch — one
# commit per job, no separate "archive:" commit afterwards.
echo ""
echo "→ Archiving job directory on $BRANCH..."
mkdir -p "$PROJECT_ROOT/$ARCHIVE_DIR"
mv "$JOB_DIR" "$PROJECT_ROOT/$ARCHIVE_DIR/$JOB_NAME"

# ── Update status in brief.md ───────────────────────────────────────────────────
sed -i "s/^status: .*/status: done/" "$PROJECT_ROOT/$ARCHIVE_DIR/$JOB_NAME/brief.md"

git -C "$PROJECT_ROOT" add "$PROJECT_ROOT/$JOBS_DIR"
git -C "$PROJECT_ROOT" commit -m "archive: $JOB_NAME"

# ── Squash-merge into the default branch (one commit for the whole job) ─────────
echo "→ Switching to $DEFAULT_BRANCH..."
git -C "$PROJECT_ROOT" checkout "$DEFAULT_BRANCH"

echo "→ Squash-merging $BRANCH..."
git -C "$PROJECT_ROOT" merge --squash "$BRANCH"
git -C "$PROJECT_ROOT" commit -m "${JOB_TITLE:-$JOB_NAME}

Job: $JOB_NAME"

echo "→ Deleting branch $BRANCH..."
git -C "$PROJECT_ROOT" branch -D "$BRANCH"

# ── Done ────────────────────────────────────────────────────────────────────────
echo ""
echo "✓ Job finished: $JOB_NAME"
echo "  Merged into : $DEFAULT_BRANCH"
echo "  Archived at : $ARCHIVE_DIR/$JOB_NAME"