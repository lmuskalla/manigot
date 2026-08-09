#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# sc-job "add image gallery block"
# sc-job "fix tenant isolation on media uploads" --type fix
# sc-job "upgrade dependencies" --type chore
#
# Installed as `sc-job`. See `make install`.

# ── Configuration ───────────────────────────────────────────────────────────────
JOBS_DIR="docs/jobs"
DEFAULT_TYPE="feature"

# ── Parse args ──────────────────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
    echo "Usage: sc-job \"title of job\" [--type feature|fix|chore]"
    exit 1
fi

TITLE="$1"
shift

JOB_TYPE="$DEFAULT_TYPE"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --type) JOB_TYPE="$2"; shift 2 ;;
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

# ── Generate job ID and directory ───────────────────────────────────────────────
ID=$(LC_ALL=C tr -dc 'a-z0-9' < /dev/urandom | head -c 6 || true)
SLUG=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/--*/-/g' | sed 's/^-\|-$//g')
JOB_DIR="$PROJECT_ROOT/$JOBS_DIR/${ID}_${SLUG}"
DATE=$(date '+%Y-%m-%d')
AUTHOR=$(git config user.name 2>/dev/null || echo "unknown")

mkdir -p "$JOB_DIR"

# ── Git branch ──────────────────────────────────────────────────────────────────
# Always branch from the base branch (main), regardless of the branch the user is
# currently on. A new job must not inherit work from another in-flight job's branch.
BRANCH="${JOB_TYPE}/${ID}_${SLUG}"
BASE_BRANCH="main"
CURRENT_BRANCH=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

if [[ -n "$CURRENT_BRANCH" ]]; then
    if ! git -C "$PROJECT_ROOT" rev-parse --verify --quiet "refs/heads/${BASE_BRANCH}" >/dev/null; then
        echo "Error: base branch '${BASE_BRANCH}' does not exist; cannot create job branch from it." >&2
        exit 1
    fi
    git -C "$PROJECT_ROOT" checkout -b "$BRANCH" "$BASE_BRANCH"
    echo "  Branch   : $BRANCH (based on $BASE_BRANCH)"
else
    echo "  Warning  : not a git repository — skipping branch creation"
    BRANCH="(no git)"
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

# ── Done ────────────────────────────────────────────────────────────────────────
echo ""
echo "✓ Job created: ${ID}_${SLUG}"
echo "  Dir    : $JOB_DIR"
echo "  Branch : $BRANCH"
echo ""
echo "  Next steps:"
echo "  1. Edit brief.md:"
echo "     $JOB_DIR/brief.md"
echo "  2. Run @product-owner or @analyst inside safecode"
echo "  3. Implement on this branch"
echo "  4. Merge when verdict is APPROVED"