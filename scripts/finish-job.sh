#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg-done "go61h7"
# mg-done "go61h7_add-backup-package"
#
# Installed as `mg-done`. See `make install`.
#
# 207bfu_git-worktrees, Decision 3: an open job lives entirely in its own git
# worktree (created by scripts/new-job.sh), never in PROJECT_ROOT's working
# tree. This script does the clean-tree check, the archive move, and the
# archive commit inside that worktree; it switches to the *main* worktree
# (PROJECT_ROOT) only for the squash-merge + branch delete step, then removes
# the job's worktree. In steady state PROJECT_ROOT is just "the main
# worktree, sitting on the base branch" — this script never does individual
# job work there.

# ── Configuration ───────────────────────────────────────────────────────────────
JOBS_DIR="docs/jobs"
ARCHIVE_DIR="docs/jobs/archive"

# ── Parse args ──────────────────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
    echo "Usage: mg-done <job-id-or-slug>"
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

# shellcheck source=lib/worktree.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/worktree.sh"

# ── Resolve job's branch + worktree ─────────────────────────────────────────────
# JOB_ARG is matched against local branch names the same way scripts/run.sh
# resolves --job (a job's branch embeds its id_slug, e.g.
# feature/207bfu_git-worktrees): an exact match on the id_slug segment first,
# then a prefix match, erroring on ambiguity. There is no directory-scan
# fallback under $JOBS_DIR — an open job's files live only in its own
# worktree now, never in PROJECT_ROOT.
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
    echo "Error: branch '$BRANCH' has no git worktree — cannot finish job '$JOB_NAME'." >&2
    echo "A job's worktree is created by 'mg job' and should always exist for an open job; this is an inconsistent state." >&2
    exit 1
fi
WORKTREE_PATH="${WORKTREE_PATH%/}"

JOB_DIR="$WORKTREE_PATH/$JOBS_DIR/$JOB_NAME"

# ── Read job metadata ───────────────────────────────────────────────────────────
BRIEF="$JOB_DIR/brief.md"
VERDICT="$JOB_DIR/verdict.md"

if [[ ! -f "$BRIEF" ]]; then
    echo "Error: brief.md not found in $JOB_DIR"
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
DEFAULT_BRANCH=$(git -C "$PROJECT_ROOT" symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@' || echo "main")

# A worktree always stays on the branch it was created with unless someone
# manually checked out a different one inside it by hand — guard against
# that inconsistent state rather than silently operating on the wrong branch.
WORKTREE_BRANCH=$(git -C "$WORKTREE_PATH" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
if [[ "$WORKTREE_BRANCH" != "$BRANCH" ]]; then
    echo "Error: worktree at $WORKTREE_PATH is on '$WORKTREE_BRANCH', expected '$BRANCH'." >&2
    echo "Someone may have checked out a different branch inside this job's worktree by hand — fix that before finishing." >&2
    exit 1
fi

# Check for uncommitted changes (archiving + merging both need a clean tree).
if ! git -C "$WORKTREE_PATH" diff --quiet || ! git -C "$WORKTREE_PATH" diff --cached --quiet; then
    echo "Error: uncommitted changes in the worktree for branch '$BRANCH'. Commit or stash before finishing." >&2
    exit 1
fi

# ── Info ────────────────────────────────────────────────────────────────────────
JOB_TITLE=$(head -1 "$BRIEF" | sed 's/^# Brief: *//')
echo ""
echo "Finishing job: $JOB_NAME"
echo "  Worktree: $WORKTREE_PATH"
echo "  Branch  : $BRANCH → $DEFAULT_BRANCH"
echo "  Archive : $JOBS_DIR/archive/$JOB_NAME"
echo ""
read -rp "  Proceed? [y/N] " CONFIRM
[[ "$CONFIRM" =~ ^[Yy]$ ]] || exit 0

# ── Archive inside the job's own worktree first ─────────────────────────────────
# The job directory is moved and committed inside its own worktree so the
# squash merge below folds the archive move into a single commit on the
# default branch — one commit per job, no separate "archive:" commit
# afterwards on the default branch.
echo ""
echo "→ Archiving job directory on $BRANCH..."
mkdir -p "$WORKTREE_PATH/$ARCHIVE_DIR"
mv "$JOB_DIR" "$WORKTREE_PATH/$ARCHIVE_DIR/$JOB_NAME"

# ── Update status in brief.md ───────────────────────────────────────────────────
sed -i "s/^status: .*/status: done/" "$WORKTREE_PATH/$ARCHIVE_DIR/$JOB_NAME/brief.md"

git -C "$WORKTREE_PATH" add "$WORKTREE_PATH/$JOBS_DIR"
git -C "$WORKTREE_PATH" commit -m "archive: $JOB_NAME"

# ── Squash-merge into the default branch (one commit for the whole job) ─────────
# From here on, every git operation runs against PROJECT_ROOT (the main
# worktree) rather than the job's own worktree — squash-merging is what
# folds the job branch's history into the default branch, so it must happen
# where that branch is actually checked out.
echo "→ Switching to $DEFAULT_BRANCH in the main worktree..."
git -C "$PROJECT_ROOT" checkout "$DEFAULT_BRANCH"

echo "→ Squash-merging $BRANCH..."
git -C "$PROJECT_ROOT" merge --squash "$BRANCH"
git -C "$PROJECT_ROOT" commit -m "${JOB_TITLE:-$JOB_NAME}

Job: $JOB_NAME"

# ── Remove the job's worktree, then delete its branch ───────────────────────────
# git refuses to delete a branch that is still checked out in another
# worktree, so the worktree must go first. The worktree is clean at this
# point (the archive commit above left it that way), so a plain remove
# (no --force) is expected to succeed; `git worktree prune` afterwards is a
# best-effort cleanup of any stale worktree metadata (Decision 7), not a
# required step for this to have worked.
#
# One exception: when the job's branch is checked out in the *main* worktree
# (a pre-worktree job — e.g. the currently-open job when this change landed),
# worktree_path_for_branch resolves to PROJECT_ROOT itself, and
# `git worktree remove` on the main worktree is always an error
# ("fatal: '<root>' is a main working tree"). In that case the branch was
# just checked out of (line 190's `git checkout "$DEFAULT_BRANCH"`), so the
# removal step is skipped and the branch delete below alone suffices.
MAIN_WORKTREE="$(git -C "$PROJECT_ROOT" rev-parse --show-toplevel 2>/dev/null || echo "$PROJECT_ROOT")"
if [[ "$WORKTREE_PATH" == "$MAIN_WORKTREE" ]]; then
    echo "→ Worktree is the main worktree — skipping worktree removal."
else
    echo "→ Removing worktree $WORKTREE_PATH..."
    git -C "$PROJECT_ROOT" worktree remove "$WORKTREE_PATH"
    git -C "$PROJECT_ROOT" worktree prune >/dev/null 2>&1 || true
fi

echo "→ Deleting branch $BRANCH..."
git -C "$PROJECT_ROOT" branch -D "$BRANCH"

# ── Done ────────────────────────────────────────────────────────────────────────
echo ""
echo "✓ Job finished: $JOB_NAME"
echo "  Merged into : $DEFAULT_BRANCH"
echo "  Archived at : $ARCHIVE_DIR/$JOB_NAME"
