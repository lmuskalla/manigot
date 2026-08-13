# Implementation: make color of git log a big brighter

id: wide
status: open
developer:
date: 2026-08-13

## Summary

The "git log" in this brief is the TUI dashboard's read-only recent-activity
strip (see `list.go`'s `renderRecentActivity`), styled by `activityStyle` in
`internal/ui/styles.go`. Its foreground `#585858` combined with `Faint(true)`
rendered nearly invisible on low-contrast screens. TASK-1 lightened the gray
from `#585858` to `#808080`, keeping the faint + subdued-background-information
treatment so the strip stays the least prominent text on the dashboard.

Note: `tasks.md` was still the empty `@analyst` scaffold when this job was
worked — no task breakdown had been produced. The brief is self-contained and
unambiguous, so the single change was derived directly from it (see
"Known issues / follow-ups").

## Changes

TASK-1: lighten the recent-activity (git log) strip color in the TUI
- `internal/ui/styles.go` — `activityStyle`: `#585858` → `#808080` (kept
  `Faint(true)`), with the comment updated to record the lightening and why.
  `#808080` is the only change site — `list.go`'s `renderRecentActivity` is
  the sole user of the style, and no test pins the color value.
- Verified: `go build ./...` and `go vet ./internal/ui/` pass. The full
  `go test ./internal/ui/` run fails only in tests whose helpers call
  `git init` — blocked by the session's read-only git shim (environment
  limitation, pre-existing, unrelated to this change); all non-git UI tests
  pass.

## Known issues / follow-ups

- `tasks.md` was never produced by `@analyst` (still the empty scaffold). If
  a fuller audit of the strip's contrast is wanted, it should be driven from
  a proper task breakdown.
- `#808080` is a moderate lightening chosen to be legible on low-contrast
  screens while still reading as background info. If the user's screen still
  swallows it, the follow-up knob is dropping `Faint(true)` or raising the
  gray further — deliberately not done here to keep the change "a bit
  lighter" as the brief asked.
