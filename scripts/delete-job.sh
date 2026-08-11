#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg-delete "go61h7"
# mg-delete "go61h7_add-backup-package"
#
# Installed as `mg-delete`. See `make install`.
#
# Permanently deletes a job: its worktree (207bfu_git-worktrees, Decision 3)
# and, when the job has a branch, the branch itself (git branch -D — no
# merge, unlike mg-done). This is destructive and cannot be undone; the
# confirmation prompt is the only safety net. A worktree with uncommitted
# changes is force-removed (git worktree remove --force) after an explicit
# warning in that same confirmation — there is no separate "commit or stash
# first" error path the way mg-done has, since deleting a job is already an
# unconditional discard.

# ── Configuration ───────────────────────────────────────────────────────────────
JOBS_DIR="docs/jobs"

# ── Parse args ──────────────────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
    echo "Usage: mg-delete <job-id-or-slug>"
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

# ── Non-git project: no branches, no worktrees ──────────────────────────────────
# Mirrors new-job.sh's own non-git fallback: the scaffold was written
# straight into PROJECT_ROOT/docs/jobs/<name>, so that's where it's removed
# from too — a plain directory delete, no git involved at all.
if ! git -C "$PROJECT_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    if [[ -d "$PROJECT_ROOT/$JOBS_DIR/$JOB_ARG" ]]; then
        JOB_DIR="$PROJECT_ROOT/$JOBS_DIR/$JOB_ARG"
        JOB_NAME="$JOB_ARG"
    else
        MATCH=$(find "$PROJECT_ROOT/$JOBS_DIR" -maxdepth 1 -type d -name "${JOB_ARG}*" 2>/dev/null | grep -v '/archive' | head -1 || true)
        if [[ -z "$MATCH" ]]; then
            echo "Error: job '$JOB_ARG' not found under $JOBS_DIR/"
            exit 1
        fi
        JOB_DIR="$MATCH"
        JOB_NAME="$(basename "$MATCH")"
    fi
    JOB_TITLE=$(head -1 "$JOB_DIR/brief.md" 2>/dev/null | sed 's/^# Brief: *//' || true)
    echo ""
    echo "This will permanently delete job: $JOB_NAME"
    echo "  Title  : ${JOB_TITLE:-$JOB_NAME}"
    echo "  Dir    : $JOBS_DIR/$JOB_NAME"
    echo ""
    echo "This cannot be undone."
    read -rp "  Proceed? [y/N] " CONFIRM
    [[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0
    echo ""
    echo "→ Removing $JOBS_DIR/$JOB_NAME..."
    rm -rf "$JOB_DIR"
    echo ""
    echo "✓ Job deleted: $JOB_NAME"
    exit 0
fi

# shellcheck source=lib/worktree.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/worktree.sh"

# ── Resolve job's branch + worktree ─────────────────────────────────────────────
# Same branch-matching convention scripts/run.sh and scripts/finish-job.sh
# use: an exact match on the id_slug segment first, then a prefix match,
# erroring on ambiguity. No directory-scan fallback — an open job's files
# live only in its own worktree now.
BRANCHES=$(git -C "$PROJECT_ROOT" for-each-ref --format='%(refname:short)' refs/heads/ 2>/dev/null || true)

MATCHED_BRANCH=""
while IFS= read -r b; do
    [[ -z "$b" ]] && continue
    if [[ "${b##*/}" == "$JOB_ARG" ]]; then
        MATCHED_BRANCH="$b"
        break
    fi
done <<< "$BRANCHES"

if [[ -z "$MATCHED_BRANCH" ]]; then
    PREFIX_MATCHES=()
    while IFS= read -r b; do
        [[ -z "$b" ]] && continue
        if [[ "${b##*/}" == "$JOB_ARG"* ]]; then
            PREFIX_MATCHES+=("$b")
        fi
    done <<< "$BRANCHES"
    case "${#PREFIX_MATCHES[@]}" in
        0)
            echo "Error: job '$JOB_ARG' not found among local branches."
            echo "Active job branches:"
            git -C "$PROJECT_ROOT" for-each-ref --format='  %(refname:short)' refs/heads/ 2>/dev/null || true
            exit 1
            ;;
        1)
            MATCHED_BRANCH="${PREFIX_MATCHES[0]}"
            ;;
        *)
            echo "Error: job '$JOB_ARG' is ambiguous — matches branches: ${PREFIX_MATCHES[*]}"
            exit 1
            ;;
    esac
fi

BRANCH="$MATCHED_BRANCH"
JOB_NAME="${BRANCH##*/}"

WORKTREE_PATH="$(worktree_path_for_branch "$PROJECT_ROOT" "$BRANCH")"
if [[ -z "$WORKTREE_PATH" ]]; then
    echo "Error: branch '$BRANCH' has no git worktree — cannot delete job '$JOB_NAME'." >&2
    echo "A job's worktree is created by 'mg job' and should always exist for an open job; this is an inconsistent state." >&2
    exit 1
fi
WORKTREE_PATH="${WORKTREE_PATH%/}"

JOB_DIR="$WORKTREE_PATH/$JOBS_DIR/$JOB_NAME"
JOB_TITLE=$(head -1 "$JOB_DIR/brief.md" 2>/dev/null | sed 's/^# Brief: *//' || true)

# Deleting the worktree discards its working tree wholesale — surface that
# explicitly in the confirmation below rather than erroring out (unlike
# mg-done, there is no "commit first" requirement for a delete).
DIRTY="false"
if ! git -C "$WORKTREE_PATH" diff --quiet || ! git -C "$WORKTREE_PATH" diff --cached --quiet; then
    DIRTY="true"
fi

# ── Confirm ─────────────────────────────────────────────────────────────────────
echo ""
echo "This will permanently delete job: $JOB_NAME"
echo "  Title    : ${JOB_TITLE:-$JOB_NAME}"
echo "  Worktree : $WORKTREE_PATH"
echo "  Branch   : $BRANCH (will be deleted, unmerged)"
if [[ "$DIRTY" == "true" ]]; then
    echo "  Warning  : this worktree has uncommitted changes — they will be discarded."
fi
echo ""
echo "This cannot be undone."
read -rp "  Proceed? [y/N] " CONFIRM
[[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0

# ── Remove the worktree, then delete its branch ─────────────────────────────────
# git refuses to delete a branch that is still checked out in another
# worktree, so the worktree must go first — --force so a dirty worktree
# (warned about above) is discarded rather than blocking the delete.
# `git worktree prune` afterwards is a best-effort cleanup of any stale
# worktree metadata (Decision 7), not a required step for this to have
# worked.
DEFAULT_BRANCH=$(git -C "$PROJECT_ROOT" symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@' || echo "main")

# The main worktree must not itself be sitting on the branch being deleted —
# shouldn't normally happen (PROJECT_ROOT stays on the base branch in
# steady state), but guard against a hand-checked-out state all the same.
# This is also exactly the pre-worktree-job case (a job whose branch is
# checked out in the main worktree): switching the main worktree off the
# branch is what makes the branch deletable, since there is no separate job
# worktree to remove.
CURRENT_MAIN_BRANCH=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
if [[ "$CURRENT_MAIN_BRANCH" == "$BRANCH" ]]; then
    echo ""
    echo "→ Switching the main worktree off $BRANCH..."
    git -C "$PROJECT_ROOT" checkout "$DEFAULT_BRANCH"
fi

# When the resolved worktree is the main worktree itself (the pre-worktree-job
# case above), `git worktree remove --force` always fails ("fatal: '<root>' is
# a main working tree") — the main working tree cannot be removed. The branch
# delete alone suffices there: switching the main worktree off the branch (just
# done) is all the cleanup a main-worktree job needs.
MAIN_WORKTREE="$(git -C "$PROJECT_ROOT" rev-parse --show-toplevel 2>/dev/null || echo "$PROJECT_ROOT")"
if [[ "$WORKTREE_PATH" == "$MAIN_WORKTREE" ]]; then
    echo ""
    echo "→ Job's worktree is the main worktree — skipping worktree removal."
else
    echo ""
    echo "→ Removing worktree $WORKTREE_PATH (and everything committed on $BRANCH)..."
    git -C "$PROJECT_ROOT" worktree remove --force "$WORKTREE_PATH"
    git -C "$PROJECT_ROOT" worktree prune >/dev/null 2>&1 || true
fi

echo "→ Deleting branch $BRANCH..."
git -C "$PROJECT_ROOT" branch -D "$BRANCH"

echo ""
echo "✓ Job deleted: $JOB_NAME"
echo "  Branch removed: $BRANCH"
