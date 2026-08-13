# Verdict: git log in job view

id: occasion
status: open
reviewer: @reviewer
date: 2026-08-13

## Review

The diff `main...HEAD` was reviewed in full (12 files, +742/−5): the five task
commits plus the implementation summary. Cross-referenced against tasks.md.

TASK-1: PASS
notes: `git.BranchCommits` (internal/git/git.go:284) runs exactly the
specified `git log -n <n> --source --format=<same format> <branch>` and
parses through the shared `logCommits` helper (git.go:241) with the shared
`commitLogFormat` (git.go:231), so the line shape cannot drift from
RecentCommits. Degrade rules hold: `n <= 0` → nil (git.go:286), missing
branch → empty/nil via the `RefExists` pre-check (git.go:290–294), non-repo →
ErrNotARepo. Unit tests cover branch scoping, cap at n, missing branch,
empty repo, non-repo, and n ≤ 0 — all green.

TASK-2: PASS
notes: `detailView.recentCommits`/`recentMax` fields (detail.go:83–96) and
`refreshCommits(maxRecent)` (detail.go:295–317) call
`git.BranchCommits(d.job.Root, d.job.Branch, maxRecent)` and degrade to nil
on empty branch or error. Both specified call sites are wired with
`a.settings.RecentActivityCountValue()`: after `newDetailView` in
updateList's "enter" case (app.go:632) and alongside `detail.reload()` in
`App.refresh()` (app.go:606). The `syncViewerSize()` call added inside
refreshCommits is a necessary correctness addition — without it the TASK-3
strip would overflow the alt-screen viewport on open (newDetailView sizes the
viewers before the strip's footprint is known) — and is consistent with the
existing setStatus budget pattern. Verified `Job.Branch` is authoritative
from the worktree (job/discover.go:120); the fallback paths degrade safely.

TASK-3: PASS
notes: per-commit formatting extracted into the shared `renderActivityLines`
(list.go:247), used by both `listView.renderRecentActivity` and
`detailView.renderCommitStrip` (detail.go:794), so the visuals cannot drift.
Strip renders below `renderFooter()` (detail.go:574–578) and renders nothing
(not even a spacer) when the cache is empty. Vertical budget verified by
hand: with `commitStripRows()` = 1 spacer + n lines subtracted in
`bodyHeight()` (detail.go:450), the total render equals the viewport height
exactly when the body floor isn't hit; `commitStripShown()` (detail.go:499)
mirrors `listView.recentActivityShown`'s adaptive sizing (configured max,
floor of 1, clamped to spare room and available commits) per the open-question
resolution. The extraction is byte-identical to the previous list output
(regression-tested).

TASK-4: PASS
notes: all specified tests present and passing — git.BranchCommits unit tests
(branch-scoped, cap at n, missing branch, empty repo, non-repo; plus n ≤ 0)
in internal/git/recentcommits_test.go; detail-view tests (only the job
branch's commits appear, no strip when Branch == "" / non-repo / git error,
render still fits the viewport with a multi-line strip, strip coexists below
the footer, adaptive sizing) in internal/ui/detail_test.go; byte-identical
list-strip regression + delegation check in internal/ui/list_test.go. Full
suite (`go test ./...`) green.

TASK-5: PASS
notes: README paragraph added immediately after the log-tab text
(README.md:692–699) describing the job view's bottom git-log strip, its
branch scoping, refresh behavior, and adaptive sizing.

Scope: the diff touches only the files named in tasks.md plus the job's own
directory files — no unrelated refactors, no changes outside the task list.

Commit discipline: one commit per task in `[occasion] TASK-N: ...` format,
plus a separate `[occasion] implementation: add summary` commit.

## Security

none — this feature adds a read-only `git log` strip to the detail view; no
new privilege surface, no writes, no input handling beyond a branch name
already derived from `job.Discover`. The git-shim environment's test-run
caveat (`PATH=/usr/bin:/bin:$PATH go test ./...`) and the pre-existing
gofmt nit in app.go are documented in implementation.md and predate this job.

## Overall

APPROVED

Nothing must change before this can be merged.
