#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# sc-delete "go61h7"
# sc-delete "go61h7_add-backup-package"
#
# Installed as `sc-delete`. See `make install`.
#
# Permanently deletes a job: its directory under docs/jobs/ and, when the job
# has a branch, the branch itself (git branch -D — no merge, unlike sc-done).
# This is destructive and cannot be undone; the confirmation prompt is the
# only safety net.

# ── Configuration ───────────────────────────────────────────────────────────────
JOBS_DIR="docs/jobs"

# ── Parse args ──────────────────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
    echo "Usage: sc-delete <job-id-or-slug>"
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
if [[ ! -f "$BRIEF" ]]; then
    echo "Error: brief.md not found in $JOB_DIR"
    exit 1
fi

JOB_TITLE=$(head -1 "$BRIEF" | sed 's/^# Brief: *//')
BRANCH=$(grep '^branch:' "$BRIEF" | awk '{print $2}' || true)

# ── Confirm ─────────────────────────────────────────────────────────────────────
echo ""
echo "This will permanently delete job: $JOB_NAME"
echo "  Title  : ${JOB_TITLE:-$JOB_NAME}"
echo "  Dir    : $JOBS_DIR/$JOB_NAME"
if [[ -n "$BRANCH" ]]; then
    echo "  Branch : $BRANCH (will be deleted, unmerged)"
fi
echo ""
echo "This cannot be undone."
read -rp "  Proceed? [y/N] " CONFIRM
[[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0

# ── No branch known (non-git project) — just remove the directory ──────────────
if [[ -z "$BRANCH" ]] || ! git -C "$PROJECT_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    echo ""
    echo "→ Removing $JOBS_DIR/$JOB_NAME..."
    rm -rf "$JOB_DIR"
    echo ""
    echo "✓ Job deleted: $JOB_NAME"
    exit 0
fi

# ── Git checks ──────────────────────────────────────────────────────────────────
CURRENT_BRANCH=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD)

if [[ "$CURRENT_BRANCH" != "$BRANCH" ]]; then
    echo "Note: you are on '$CURRENT_BRANCH', job branch is '$BRANCH'"
    echo "→ Switching to job branch $BRANCH..."
    git -C "$PROJECT_ROOT" checkout "$BRANCH"
    CURRENT_BRANCH="$BRANCH"
fi

# Deleting the branch below discards its working tree wholesale; a clean tree
# is required so nothing uncommitted silently leaks onto the branch we switch
# to next (checkout carries forward non-conflicting uncommitted changes).
if ! git -C "$PROJECT_ROOT" diff --quiet || ! git -C "$PROJECT_ROOT" diff --cached --quiet; then
    echo "Error: uncommitted changes on branch '$BRANCH'. Commit or stash before deleting."
    exit 1
fi

DEFAULT_BRANCH=$(git -C "$PROJECT_ROOT" symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@' || echo "main")

echo ""
echo "→ Switching to $DEFAULT_BRANCH..."
git -C "$PROJECT_ROOT" checkout "$DEFAULT_BRANCH"

echo "→ Deleting branch $BRANCH (and everything committed on it, including $JOBS_DIR/$JOB_NAME)..."
git -C "$PROJECT_ROOT" branch -D "$BRANCH"

echo ""
echo "✓ Job deleted: $JOB_NAME"
echo "  Branch removed: $BRANCH"
