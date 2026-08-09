# Implementation: git view and switch

id: qge358
status: open
developer: claude
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

Implemented the scoped-down "git view and switch" feature: the job list's
header now shows the currently checked-out branch, a new `m` key does a
one-key "back to main" checkout from the list, and a read-only recent-
activity strip shows the last 5 commits across all local branches, deduped
and most-recent-first. No new views, no generic branch picker, no
interactive git log — matches the brief's locked product decision.

## Changes

TASK-1: Added `git.RecentCommits(root string, n int) ([]Commit, error)` and
the `Commit{Hash, ShortHash, Subject, RelTime, Branch}` type to
`tui/internal/git/git.go`. It enumerates local branches via `LocalBranches`,
orders them with the current branch first (deterministic tie-break for
shared commits), then runs a single `git log --source -n <n> <branches...>`
so git itself union-traverses and dedups history across every branch in one
pass, with `--source`/`%S` supplying the "which branch" attribution for free.
Degrades the same way the rest of the package does: `ErrNotARepo` for a
missing git binary/non-repo, empty slice + nil error for an unborn HEAD or a
repo with no local branches.

TASK-2: Added `tui/internal/git/recentcommits_test.go` covering dedup of a
commit shared by two undiverged branches, most-recent-first ordering across
branches (using a `commitAllAt` helper with explicit `GIT_AUTHOR_DATE`/
`GIT_COMMITTER_DATE` so same-second wall-clock commits can't tie and produce
a flaky test), `n` truncation, fewer-than-`n` results, an empty repo, and a
non-repo directory.

TASK-3: Added `App.currentBranch string` in `tui/internal/ui/app.go`,
populated in `NewApp` and re-populated in every place the job list itself
refreshes (`refreshJobs`, `checkoutMsg`'s success path via `refreshJobs`, and
`updateNewJob`'s post-`sc-job` refresh) so it can't go stale relative to
`renderJobRow`'s existing `· <branch>` tags. Rendered in `renderList`'s
header line next to "jobs in `<root>`"; renders nothing when empty (detached
HEAD or a non-repo project).

TASK-4: Added the `m` key to `updateList` (list scope only), dispatching the
existing `App.checkoutCmd("main")` — the exact mechanism the detail view's
`c` key already uses, just with a hardcoded target instead of the open job's
branch, per the brief's explicit rejection of a generic branch picker. Added
an `else` branch to the `checkoutMsg` handler's success path for the
`a.detail == nil` case (previously it refreshed the job list silently and
returned with no status), so both a real switch and an already-on-main no-op
now report "switched to main" in the footer instead of doing nothing
visible. Updated the footer hint string to include "m main".

TASK-5: Added `App.recentCommits []git.Commit` and `refreshRecentCommits`
(errors degrade to an empty strip, matching `currentBranch`'s treatment),
called at the same refresh points as TASK-3. Added `renderRecentActivity` in
`tui/internal/ui/app.go`, rendering up to `recentActivityCount` (5) dimmed
lines — short hash, truncated subject (via the existing `truncate` helper),
relative time, branch — beneath the branch header line. Renders nothing when
there's no history. Verified manually at several terminal widths (60–120
cols) that long subjects truncate cleanly and columns stay aligned.

TASK-6: Added list-view checkout tests to `tui/internal/ui/checkout_test.go`
(`TestMainKeySwitchesToMainFromList`, `TestMainKeyAlreadyOnMainStillReportsStatus`,
`TestMainKeyRefusedCheckoutSurfacesStatusInList`), including a
`gitEnsureMain` helper so the tests work regardless of this git
installation's `init.defaultBranch` (renames the init default to a branch
literally named `main` before exercising the hardcoded `m` target). Added
`tui/internal/ui/list_test.go` covering the branch header line's presence/
absence and the recent-activity strip's dedup, ordering, and empty-repo
behaviour end-to-end through `renderList`.

## Post-review fix (TASK-5)

The reviewer flagged the originally-shipped 5-entry recent-activity strip as
a blocking regression: it pushed the job rows down by up to 5 lines versus
the pre-existing header, contradicting the brief's explicit "must not push
the job rows down" constraint. Fixed by applying the brief's own named
fallback — `recentActivityCount` is now `1` ("last commit only") — and by
having the strip take over the header's existing blank spacer line instead
of appending after it, so the header is exactly 2 lines before the column
header (the pre-TASK-5 baseline) whether or not there's a commit to show.
Verified by direct line-count inspection of `renderList`'s output. The
UI-layer recent-activity test (`list_test.go`) was updated for the new
1-entry behavior; the exhaustive multi-entry dedup/ordering coverage from
TASK-2 (`recentcommits_test.go`, at the `git` package level) is unaffected
and untouched.

## Known issues / follow-ups

none.
