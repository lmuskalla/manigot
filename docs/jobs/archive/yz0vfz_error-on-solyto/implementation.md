# Implementation: error on solyto

id: yz0vfz
status: open
developer: deepseek-v4-flash
date: 2026-08-12

<!-- Produced by @developer after implementation. -->

## Summary

Fixed the `mg tui` panic `runtime error: slice bounds out of range [:1] with capacity 0` that
occurred right after `mg init` on a fresh project (reproduced locally, identical message and stack
site at `internal/ui/app.go:1236`). Root cause: on a repo with no commits yet (unborn HEAD),
`git.RecentCommits` returns an empty slice with a nil error; Bubble Tea renders the first `View()`
before the first `WindowSizeMsg` arrives, so `a.height == 0`; and `recentActivityShown()`'s
`a.height == 0` early-return gave the floor count (1) without the "fewer real commits than the
computed count" clamp the `height > 0` path applies — so `renderRecentActivity` sliced
`a.recentCommits[:1]` against an empty cache.

The count is now clamped to `len(a.recentCommits)` on every path, the slicing site is hardened as
defense in depth, and a regression test pins the exact crash combination. Behavior is unchanged for
repos with commits; a commit-less repo renders no strip (the established empty-strip degrade path).

## Changes

TASK-1: `internal/ui/app.go` — hoisted the `n > len(a.recentCommits)` clamp out of the
`height > 0` branch of `recentActivityShown()` so it runs on the `a.height == 0` path too (the
zero-height fallback now degrades to 0 instead of returning the floor of 1 against an empty cache).
Updated the function's doc comment to document the empty-commits degrade.

TASK-2: `internal/ui/app.go` — hardened `renderRecentActivity()` by clamping `n` to
`len(a.recentCommits)` immediately before the `a.recentCommits[:n]` slice, so the slicing site can
never panic even if a future caller computes an oversized count.

TASK-3: `internal/ui/list_test.go` — added `TestRenderListZeroHeightNoCommitsDoesNotPanic`,
combining the two halves the existing tests each covered separately: a fresh repo with no commits
(like `TestRenderListRecentActivityEmptyOnFreshRepo`) and an App that never received a
WindowSizeMsg, `a.height == 0` (like `TestRenderListZeroHeightDoesNotPanic`, which used a repo
*with* an init commit). It asserts `renderList()` renders sane output without panicking; it fails
against the pre-fix code (confirmed during reproduction).

TASK-4: verification — `go test ./internal/ui/` and `go build ./...` pass; full-repo
`go test ./...` also passes.

## Known issues / follow-ups

none
