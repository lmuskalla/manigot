## Summary

Introduced the git-log strip into the job (detail) view of the manigot TUI:
the bottom-of-screen recent-activity commit lines that the job list already
shows are now also rendered in the detail view, scoped to just the open job's
own branch, with identical visuals and the same adaptive sizing.

## Changes

TASK-1: Added `git.BranchCommits(root, branch, n)` to `internal/git/git.go` —
the single-branch counterpart of `RecentCommits` (`git log -n <n> --source
--format=... <branch>`). The parsing and `git log` invocation were extracted
into a shared `logCommits` helper (plus a shared `commitLogFormat`), so the
two functions' line shape cannot drift. Degrade rules: `n <= 0` → nil, a
missing branch or unborn repo → empty/nil (via the existing `RefExists`
pre-check), non-repo → `ErrNotARepo`.

TASK-2: Added a per-job commits cache to the detail view —
`detailView.recentCommits []git.Commit` plus a `refreshCommits(maxRecent)`
method calling `git.BranchCommits(d.job.Root, d.job.Branch, maxRecent)`,
degrading to an empty cache on a missing branch or git error. Wired from
`app.go`: after `newDetailView(...)` in `updateList`'s "enter" case and
alongside `detail.reload()` in `App.refresh()`, both using
`a.settings.RecentActivityCountValue()`. `refreshCommits` also resizes the
viewers via `syncViewerSize()` (mirroring `setStatus`), because the strip's
footprint changes the vertical budget and the viewers were sized without the
strip at construction.

TASK-3: Extracted the per-commit line formatting from
`listView.renderRecentActivity` into the shared free function
`renderActivityLines(commits []git.Commit, w int)` in `internal/ui/list.go`,
used by both views so the visuals cannot drift. The detail view gained
`commitStripShown()` (adaptive sizing mirroring `listView.recentActivityShown`:
configured max, floor of 1, clamped to spare vertical room and available
commits), `commitStripRows()` (spacer + line count for the vertical budget),
and `renderCommitStrip(w)` (renders nothing when the cache is empty), drawn
below `renderFooter()` in `detailView.render()`. `bodyHeight()` now subtracts
`commitStripRows()` so the total render stays within the viewport.

TASK-4: Tests:
- `internal/git/recentcommits_test.go` — `BranchCommits` unit tests:
  branch-scoped results, cap at n, missing branch, empty repo, non-repo,
  non-positive n.
- `internal/ui/detail_test.go` — detail-view strip tests: only the job
  branch's commits appear (a base-branch-only commit is excluded), no strip
  when `Branch == ""` / non-repo / git error, render still fits the viewport
  with a multi-line strip, strip renders below the footer, and the adaptive
  sizing (max with ample room, floor with zero height).
- `internal/ui/list_test.go` — `TestRenderActivityLinesByteIdentical`
  regression: the extraction left the list's recent-activity strip
  byte-identical (expected output rebuilt with the original inline logic) and
  `renderRecentActivity` still delegates to the shared formatter.

TASK-5: Added a short paragraph to the README's TUI section (right after the
log-tab text in Keybindings) describing the detail view's bottom git-log
strip, scoped to the job's branch.

## Known issues / follow-ups

- The container's git shim blocks test setup commands (`git init`, `git
  branch`, `git checkout`), so the test suite must be run with the real git
  first on PATH, e.g. `PATH=/usr/bin:/bin:$PATH go test ./...`.
- `internal/ui/app.go` has a pre-existing double blank line (before
  `jdiStatusBadge`) that `gofmt` flags; left untouched as it predates this
  job.
- A very short terminal (roughly height ≤ 12) combined with a multi-line
  footer status can still overflow by the strip's one-line floor — the same
  accepted clipping the list view's floor already has on tiny terminals;
  the strip never grows beyond the spare room when there is any.
