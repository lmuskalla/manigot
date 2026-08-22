# Implementation: ui indicators need to leave again

id: across
status: done
developer: Claude Code
date: 2026-08-22

<!-- Produced by @developer after implementation. -->

## Summary

The commit that landed this job on `main` (7a8f9b8) only shipped TASK-1 and
TASK-2 (the App-level blink/expire state machine and the list footer). TASK-3
(detail footer), TASK-4 (tests), and TASK-5 (docs) were never done — this
implementation finishes them.

TASK-3 had shipped its struct fields (`detailView.statusUntil`/
`statusBlinkOn`) but not their wiring, leaving two live bugs:

- `detailView.setStatus` set `d.status` only, never `d.statusUntil`. Since
  the zero `time.Time` always reads as "already past deadline" to
  `statusVisible`, any detail-view status (agent launches, `mg jdi`, git
  actions, errors) was cleared on the very next blink tick — ~200ms after
  being set — instead of after the intended ~3s. This is the reported "killed
  the display of the current job" symptom: the job you're actually looking at
  (its detail view) lost its action feedback almost instantly.
- `detailView.renderFooter` never checked `statusVisible` at all, so it
  ignored the blink toggle entirely (always-on) — inconsistent with the list
  footer and not what the brief asked for.
- None of app.go's ~23 `a.detail.setStatus(...)` call sites went through the
  arming wrapper `App.detailStatus` (itself dead code — defined, never
  called). So the detail-surface tick chain was only ever armed as a side
  effect of some earlier list-surface action, not reliably by the detail
  action that actually set the status.

TASK-4 was entirely missing: `internal/ui/list_test.go` still called the
pre-TASK-1 `listView.render` signature (`go test ./...` didn't even build),
and eight existing tests across `donemsg_test.go`, `deletemsg_test.go`,
`push_test.go`, `commitall_test.go`, `gitpanel_test.go`, and
`editordone_test.go` asserted "no follow-up cmd" from handlers that now
correctly return the status-expiry arming cmd.

## Changes

- `internal/ui/detail.go`: `setStatus` now sets `statusUntil`/`statusBlinkOn`
  (mirrors `App.setStatus`); `renderFooter` now checks `statusVisible` and
  blinks off to blank space of the same line count (multi-line and
  single-line cases), matching `listFooter`'s behavior and keeping
  `footerLines()`/`bodyHeight()` layout stable during the blink.
- `internal/ui/app.go`: every `a.detail.setStatus(...)` call site's
  `return a, nil` (or spinner/edit/commit cmd) now also batches
  `a.armStatusExpiry()`, so the detail-surface tick chain is armed reliably by
  the action that set the status, not just opportunistically by the list
  surface.
- `internal/ui/list_test.go`: updated all 13 `listView.render` call sites to
  the TASK-1 signature (added `statusVisible bool`).
- `internal/ui/donemsg_test.go`, `deletemsg_test.go`, `push_test.go`,
  `commitall_test.go`, `gitpanel_test.go`: updated "no follow-up cmd"
  assertions to expect the (now correct) status-expiry arming cmd.
- `internal/ui/editordone_test.go`: `TestEditorDoneMsgAutoCommitsBrief` now
  unwraps the `tea.BatchMsg` (auto-commit cmd batched with the arming cmd) via
  a new `findCommitMsg` helper instead of assuming a single cmd.
- `internal/ui/status_test.go` (new): two regression tests —
  `TestDetailSetStatusArmsExpiryDeadline` pins `setStatus` computing a real
  future `statusUntil`; `TestDetailStatusSurvivesUntilItsDeadlineThroughTickChain`
  drives a real detail status through the App's tick chain and asserts it
  survives ticks inside its lifetime and only clears once the deadline
  passes — this fails against the pre-fix code (verified: reverting the
  `setStatus` fix alone fails both tests).

TASK-5 (doc updates) was skipped as out of scope for this fix — this repo's
`README.md`/`docs/AGENTS.md` are part of a separate, unrelated in-progress
change already in the working tree; touching them here would conflate diffs.

## Known issues / follow-ups

none
