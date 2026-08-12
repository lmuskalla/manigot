# Tasks: error on solyto

id: yz0vfz
status: open
analyst: deepseek-v4-flash
date: 2026-08-12

<!-- Produced by @analyst from brief.md. -->

## Summary

`mg tui` panics with `runtime error: slice bounds out of range [:1] with capacity 0` at
`internal/ui/app.go:1236` (`renderRecentActivity`) right after `mg init` on a fresh project.

Root cause (confirmed by reproduction, same panic message and stack site):

- On a repo with no commits yet (unborn HEAD — exactly the state after `mg init` on a brand-new
  project), `git.RecentCommits` returns an empty slice with a nil error, so `a.recentCommits` is
  empty.
- Bubble Tea renders the very first `View()` before the first `tea.WindowSizeMsg` arrives, so
  `a.height == 0` on that first render.
- `recentActivityShown()` has an early-return for `a.height == 0` that returns `recentActivityFloor`
  (1) *without* applying the "fewer real commits than the computed count" clamp that the
  `height > 0` path applies (app.go:474-479). `renderRecentActivity` then slices
  `a.recentCommits[:1]` against an empty slice → panic.

Fix: clamp the returned count to `len(a.recentCommits)` on every path, and harden the slicing site
(defense in depth). No behavior change for repos with commits: the height==0 path still renders the
floor (1) entry when commits exist; a commit-less repo simply renders no strip, which is the
established empty-strip degrade path (`renderList` already handles `activity == ""`).

## Task breakdown

TASK-1: Fix `recentActivityShown()` so the count is clamped to `len(a.recentCommits)` on the
a.height == 0 path too (hoist the existing clamp out of the height > 0 branch so it runs for both),
and update the function's doc comment to mention the empty-commits degrade.
     files: internal/ui/app.go
     depends: none
     risk: low — two-line restructure of one pure function; existing tests pin all non-panic
            behaviors (floor at height 0 with commits, scaling with spare room, floor when full).

TASK-2: Harden `renderRecentActivity()` by clamping `n` to `len(a.recentCommits)` immediately
before the `a.recentCommits[:n]` slice, so the slicing site can never panic even if a future caller
computes an oversized count.
     files: internal/ui/app.go
     depends: none (independent edit; do alongside TASK-1)
     risk: low — 2-line guard at the panic site, no behavior change for in-range counts.

TASK-3: Add a regression test in internal/ui/list_test.go reproducing the exact crash combination —
a fresh repo with no commits and an App that never received a WindowSizeMsg (a.height == 0) — and
assert renderList() does not panic. Existing coverage misses this: TestRenderListZeroHeightDoesNotPanic
uses a repo *with* an init commit; TestRenderListRecentActivityEmptyOnFreshRepo uses height 24.
     files: internal/ui/list_test.go
     depends: TASK-1 (fails against the current code)
     risk: low — pure addition mirroring the two existing neighbors; delete the repro file's pattern
            is already proven (identical panic reproduced before the fix).

TASK-4: Verify the fix — `go test ./internal/ui/` (full package) and `go build ./...` must pass.
     files: none (verification only)
     depends: TASK-1, TASK-2, TASK-3
     risk: low — confirmation step.
