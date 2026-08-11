#!/usr/bin/env bash
# scripts/lib/worktree.sh
#
# Shared bash helper for locating a job's git worktree by branch name
# (207bfu_git-worktrees, Decision 2). Sourced — never executed directly — by
# scripts/new-job.sh, scripts/run.sh, scripts/finish-job.sh and
# scripts/delete-job.sh, so all four agree on exactly one way to answer
# "does branch X have a worktree, and where". The Go side has its own
# equivalent: tui/internal/git.WorktreeForBranch.
#
# Decision 1's naming convention
# (<dirname(PROJECT_ROOT)>/.manigot-worktrees/<basename(PROJECT_ROOT)>/<id>_<slug>)
# is used exactly once, at creation time in new-job.sh — and only when
# PROJECT_ROOT's parent is on the same filesystem. A PROJECT_ROOT that is
# itself a mount point (parent on a different device) nests its worktrees at
# <PROJECT_ROOT>/.manigot-worktrees/<id>_<slug> instead, so they never land
# outside the project's persistent storage. Every other touch
# point — including this file — asks git directly rather than recomputing
# that path, so a worktree moved or relocated by hand is still found.
#
# set -euo pipefail is intentionally NOT set here: this file is sourced into
# callers that already set their own shell options, and worktree_path_for_branch
# below relies on a `while read` loop over a command substitution that must
# not abort the sourcing script on a non-matching branch.

# worktree_path_for_branch <root> <branch>
#
# Prints the absolute path of the worktree currently checked out to <branch>
# in the repository at <root>, or prints nothing (empty stdout) if no
# worktree has that branch checked out — including when <root> is not a git
# repository, or has no worktrees at all beyond the main one. Never treats
# "no worktree found" as an error: callers decide whether that's fatal.
#
# Parses `git -C <root> worktree list --porcelain`, whose output is a series
# of blank-line-separated blocks. Each block starts with a "worktree <path>"
# line and, for an ordinary (non-bare, non-detached) checkout, also has a
# "branch refs/heads/<name>" line. Matching is done against the full ref
# (refs/heads/<branch>), not a bare-name substring, so a branch name that is
# a prefix of another (e.g. "feature/x" vs. "feature/x-y") can never
# cross-match. A bare or detached-HEAD worktree entry has no "branch" line at
# all and is simply never matched.
worktree_path_for_branch() {
    local root="$1" branch="$2"
    local path="" match_ref="refs/heads/${branch}"
    local line

    while IFS= read -r line; do
        case "$line" in
            "worktree "*)
                path="${line#worktree }"
                ;;
            "branch "*)
                if [[ "${line#branch }" == "$match_ref" ]]; then
                    printf '%s\n' "$path"
                    return 0
                fi
                ;;
            "")
                # Blank line: end of this worktree's block.
                path=""
                ;;
        esac
    done < <(git -C "$root" worktree list --porcelain 2>/dev/null || true)

    return 0
}
