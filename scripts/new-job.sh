#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg-job "add image gallery block"
# mg-job "fix tenant isolation on media uploads" --type fix
# mg-job "add gallery" --base-branch develop
# mg-job "upgrade dependencies" --type chore
#
# The base branch new jobs are cut from is read from .manigot/manigot.json
# (defaulting to "main"); --base-branch overrides it for one invocation.
#
# Every job gets its own git worktree (207bfu_git-worktrees, Decision 1/3):
# created alongside the branch at
# <dirname(PROJECT_ROOT)>/.manigot-worktrees/<basename(PROJECT_ROOT)>/<id>_<slug>,
# sibling to PROJECT_ROOT rather than nested inside it. PROJECT_ROOT itself is
# never switched — it stays on whatever branch it was already on. A
# non-git-repo project keeps the pre-worktree behavior: no branch, no
# worktree, the scaffold is written straight into PROJECT_ROOT.
#
# Installed as `mg-job`. See `make install`.

# ── Configuration ───────────────────────────────────────────────────────────────
JOBS_DIR="docs/jobs"
DEFAULT_TYPE="feature"
DEFAULT_BASE_BRANCH="main"

# ── Parse args ──────────────────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
    echo "Usage: mg-job \"title of job\" [--type feature|fix|chore] [--base-branch <name>]"
    exit 1
fi

TITLE="$1"
shift

JOB_TYPE="$DEFAULT_TYPE"
BASE_BRANCH_OVERRIDE=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --type) JOB_TYPE="$2"; shift 2 ;;
        --base-branch) BASE_BRANCH_OVERRIDE="$2"; shift 2 ;;
        *) echo "Unknown argument: $1"; exit 1 ;;
    esac
done

# Validate type
case "$JOB_TYPE" in
    feature|fix|chore) ;;
    *) echo "Invalid type '$JOB_TYPE'. Use: feature, fix, or chore."; exit 1 ;;
esac

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
if [[ -z "$PROJECT_ROOT" ]]; then
    echo "Error: could not find project root (no docs/ directory found)."
    exit 1
fi

# ── Resolve base branch ─────────────────────────────────────────────────────────
# The base branch new jobs are cut from is configured per-project in
# .manigot/manigot.json (baseBranch key), defaulting to "main". This reads the
# same file the TUI writes, so `mg job` and the TUI's "m" checkout agree on
# what "base branch" means for this project. The --base-branch flag overrides
# the file for a single invocation (an ad-hoc one-off, e.g. spiking a branch
# off a release tag while the project default stays on main).
#
# The JSON extraction is a guarded sed regex against the single known key —
# revisit (add jq) only if more project keys appear.
BASE_BRANCH="$DEFAULT_BASE_BRANCH"
SETTINGS_FILE="$PROJECT_ROOT/.manigot/manigot.json"
if [[ -f "$SETTINGS_FILE" ]]; then
    # Match "baseBranch": "value" anywhere on a line, so both the
    # pretty-printed JSON project.Save writes and a hand-written one-liner
    # are accepted. Single known key with sane ref-name values, so a greedy
    # leading .* is safe here.
    VALUE=$(sed -n 's/^.*"baseBranch"[[:space:]]*:[[:space:]]*"\([^"]*\)".*$/\1/p' "$SETTINGS_FILE" | head -n1)
    [[ -n "$VALUE" ]] && BASE_BRANCH="$VALUE"
fi
[[ -n "$BASE_BRANCH_OVERRIDE" ]] && BASE_BRANCH="$BASE_BRANCH_OVERRIDE"

# ── Generate job ID ─────────────────────────────────────────────────────────────
ID=$(LC_ALL=C tr -dc 'a-z0-9' < /dev/urandom | head -c 6 || true)
SLUG=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/--*/-/g' | sed 's/^-\|-$//g')
DATE=$(date '+%Y-%m-%d')
AUTHOR=$(git config user.name 2>/dev/null || echo "unknown")

# ── Git branch + worktree ────────────────────────────────────────────────────────
# Always branch from the configured base branch, regardless of the branch the
# user is currently on. A new job must not inherit work from another in-flight
# job's branch. The job gets its own worktree (207bfu_git-worktrees, Decision
# 1/3) rather than being checked out in PROJECT_ROOT: PROJECT_ROOT itself is
# never switched, so it stays free for other jobs' worktrees to branch from
# and multiple jobs can be worked on in parallel.
BRANCH="${JOB_TYPE}/${ID}_${SLUG}"
CURRENT_BRANCH=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

if [[ -n "$CURRENT_BRANCH" ]]; then
    if ! git -C "$PROJECT_ROOT" rev-parse --verify --quiet "refs/heads/${BASE_BRANCH}" >/dev/null; then
        echo "Error: base branch '${BASE_BRANCH}' does not exist; cannot create job branch from it." >&2
        exit 1
    fi
    # Sibling to PROJECT_ROOT, not nested inside it — see scripts/lib/worktree.sh's
    # doc for why. This naming convention is only ever computed here, at
    # creation time; every other touch point looks the worktree up dynamically
    # via worktree_path_for_branch instead of recomputing this path.
    WORKTREE_PARENT="$(dirname "$PROJECT_ROOT")/.manigot-worktrees/$(basename "$PROJECT_ROOT")"
    WORKTREE_PATH="$WORKTREE_PARENT/${ID}_${SLUG}"
    mkdir -p "$WORKTREE_PARENT"
    git -C "$PROJECT_ROOT" worktree add "$WORKTREE_PATH" -b "$BRANCH" "$BASE_BRANCH"
    echo "  Branch   : $BRANCH (based on $BASE_BRANCH)"
    echo "  Worktree : $WORKTREE_PATH"
    # The job's own docs/jobs/<id>_<slug> directory, inside the worktree —
    # not the worktree's own checkout root. The worktree just cloned the base
    # branch's docs/jobs/ (without this job's subdirectory yet), so it needs
    # creating same as the non-git fallback below.
    JOB_DIR="$WORKTREE_PATH/$JOBS_DIR/${ID}_${SLUG}"
    mkdir -p "$JOB_DIR"
else
    echo "  Warning  : not a git repository — skipping branch/worktree creation"
    BRANCH="(no git)"
    JOB_DIR="$PROJECT_ROOT/$JOBS_DIR/${ID}_${SLUG}"
    mkdir -p "$JOB_DIR"
fi

# ── Write brief.md ──────────────────────────────────────────────────────────────
cat > "$JOB_DIR/brief.md" << EOF
# Brief: $TITLE

status: open
type: $JOB_TYPE
id: $ID
branch: $BRANCH
date: $DATE
author: $AUTHOR

## What

<!-- What needs to be done? Be specific. -->

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
EOF

# ── Write tasks.md ──────────────────────────────────────────────────────────────
cat > "$JOB_DIR/tasks.md" << EOF
# Tasks: $TITLE

id: $ID
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

<!-- TASK-1: description
     files: list of files likely affected
     depends: none
     risk: low / medium / high — reason

TASK-2: ...
-->
EOF

# ── Write implementation.md ─────────────────────────────────────────────────────────────
cat > "$JOB_DIR/implementation.md" << EOF
# Implementation: $TITLE

id: $ID
status: open
developer:
date:

<!-- Produced by @developer after implementation. -->

## Summary

<!-- What was implemented, task by task. Reference task IDs. -->

## Changes

<!-- List of files changed and what changed in each. -->

## Known issues / follow-ups

<!-- Anything that came up during implementation that wasn't in scope but should be tracked. -->
EOF

# ── Write verdict.md ─────────────────────────────────────────────────────────────
cat > "$JOB_DIR/verdict.md" << EOF
# Verdict: $TITLE

id: $ID
status: open
reviewer:
date:

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

<!-- TASK-1: PASS / FAIL / PARTIAL
     notes: ...

TASK-2: ...
-->

## Security

<!-- Any security findings from @security, or "none" if not run. -->

## Overall

<!-- APPROVED / REJECTED / NEEDS WORK -->
<!-- Summary of what needs to change before this can be approved, if anything. -->
EOF

# ── Commit scaffolding ──────────────────────────────────────────────────────────
# Commit the template docs immediately so the new branch doesn't start with
# uncommitted changes — they'd otherwise follow the user around until work
# begins. Run with cwd at JOB_DIR (inside the job's own worktree, not
# PROJECT_ROOT — PROJECT_ROOT is never switched onto the job's branch at
# all), so `git add .` stages exactly the four new files.
if [[ -n "$CURRENT_BRANCH" ]]; then
    git -C "$JOB_DIR" add .
    git -C "$JOB_DIR" commit -m "Scaffold job ${ID}_${SLUG}" >/dev/null
fi

# ── Done ────────────────────────────────────────────────────────────────────────
echo ""
echo "✓ Job created: ${ID}_${SLUG}"
echo "  Dir    : $JOB_DIR"
echo "  Branch : $BRANCH"
echo ""
echo "  Next steps:"
echo "  1. Edit brief.md:"
echo "     $JOB_DIR/brief.md"
echo "  2. Run @product-owner or @analyst inside manigot"
echo "  3. Implement on this branch"
echo "  4. Merge when verdict is APPROVED"