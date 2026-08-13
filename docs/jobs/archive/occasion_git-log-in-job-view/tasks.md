# Tasks: git log in job view

id: occasion
status: open
analyst:
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

The list view already shows a "recent activity" git-log strip at the bottom of
the screen (`internal/ui/list.go`): one dim `activityStyle` line per commit —
`shortHash  subject  relTime  branch` — fed by `git.RecentCommits(root, n)`
(commits across *all* local branches, capped by the settings'
`RecentActivityCount`). This job brings the same log into the job (detail)
view — same visuals, same position (bottom of the view, below the footer) —
scoped to just that job's branch (`job.Job.Branch`, populated authoritatively
by `job.Discover` from the worktree).

TASK-1: Add a single-branch recent-commits function to the git package.
     Add `git.BranchCommits(root, branch string, n int) ([]Commit, error)` —
     `git log -n <n> --source --format=<same format> <branch>` reusing
     RecentCommits' field separator/parsing (ideally via a shared logCommits
     helper) — with the package's degrade rules: n <= 0 → nil, missing branch →
     empty/nil, non-repo → ErrNotARepo.
     files: internal/git/git.go, internal/git/recentcommits_test.go (tests)
     depends: none
     risk: low — a straight mirror of the existing RecentCommits shape, same
     format string and parsing.

TASK-2: Give the detail view a per-job commits cache and wire its refresh from
     the App.
     Add `recentCommits []git.Commit` to detailView plus a refreshCommits
     (maxRecent int) method calling git.BranchCommits(d.job.Root, d.job.Branch,
     maxRecent) (degrading to nil on empty branch/error); call it from app.go
     after newDetailView(...) in updateList's "enter" case and from
     App.refresh() (alongside detail.reload()), using
     a.settings.RecentActivityCountValue().
     files: internal/ui/detail.go, internal/ui/app.go
     depends: TASK-1
     risk: low-medium — additive field + two call-site additions; existing
     tests construct detailView directly so the strip simply stays empty there,
     but the refresh wiring must be consistent with the existing refresh/reload
     paths.

TASK-3: Render the job-branch git-log strip in the detail view with identical
     visuals, and fit it into the vertical budget.
     Extract the per-commit line formatting from listView.renderRecentActivity
     into a shared free function (e.g. renderActivityLines(commits []git.Commit,
     w int) string) used by both views so the visuals cannot drift; add a
     detail-view strip renderer (scoped to the new cache, empty → render
     nothing) drawn below renderFooter() in detailView.render(); extend
     bodyHeight()'s chrome budget by the strip's row count plus its spacer so
     the total render stays ≤ viewport.
     files: internal/ui/detail.go, internal/ui/list.go
     depends: TASK-2
     risk: medium — the vertical-budget math can overflow the alt-screen
     viewport if under-counted (the exact bug
     TestDetailBodyHeightShrinksForMultiLineStatus guards), and the
     shared-helper extraction must not alter the list view's output.

TASK-4: Tests for the new function and the detail-view strip.
     git.BranchCommits unit tests (branch-scoped results, cap at n, missing
     branch, empty repo, non-repo) in internal/git; detail-view tests (only the
     job branch's commits appear, no strip when Branch == "" / non-repo, render
     still fits the viewport height, strip coexists with footer) in
     internal/ui/detail_test.go; a list-view regression asserting the
     extraction left the recent-activity strip byte-identical in
     internal/ui/list_test.go.
     files: internal/git/recentcommits_test.go, internal/ui/detail_test.go,
     internal/ui/list_test.go
     depends: TASK-1, TASK-2, TASK-3
     risk: low — test additions following existing patterns (commitAllAt,
     gitInitRepo, addJobWorktree helpers already exist).

TASK-5: Document the detail view's git-log strip.
     Add a short paragraph to the README's TUI section (near the log-tab /
     detail-view keybindings text) describing the bottom git-log strip in the
     job view, scoped to the job's branch.
     files: README.md
     depends: TASK-3
     risk: low — documentation only.

## Open questions (decide before TASK-3 / TASK-4 sizing)

- "Same position" is read as: a bottom strip below the footer, mirroring the
  list view. If the intent was instead a new tab, that is a materially
  different design — confirm before implementing.
- Line format: keep the branch column byte-for-byte identical to the list
  ("same visuals"), even though it is redundant when all commits belong to one
  branch.
- Strip sizing: mirror the list's adaptive approach (configured max, floor of
  1, clamped to available commits and spare vertical room) rather than a fixed
  count, for consistency with listView.recentActivityShown.
