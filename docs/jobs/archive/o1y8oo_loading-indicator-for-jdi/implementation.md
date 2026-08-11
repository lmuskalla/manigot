# Implementation: loading indicator for jdi

id: o1y8oo
status: open
developer: deepseek-v4-flash
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

Added an animated activity indicator (a braille-dot spinner, the same idea as
the one opencode shows in its bottom-left corner) to the TUI's mg-jdi status
badge. While an `mg jdi` run is active, the `[running @<agent>]` badge now
renders as `⠋ [running @<agent>]` in both places it appears — the job-list
row and the detail-view action bar — with the frame advancing roughly every
100ms. This is the TUI's first timer-driven redraw, deliberately scoped: the
tick chain starts only when a run is active (launched via "j", discovered on
ctrl+r/refresh, or already running at startup) and self-terminates the moment
the run's sidecar flips to a stopped state, so idle behaviour (no redraws,
no CPU churn) is unchanged.

## Changes

TASK-1: Created `tui/internal/ui/activity.go` — a self-contained, pure
spinner helper in the ui package (its only consumer): the braille frame set
(`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) kept in one place for a one-line swap, `activityFrame(step)`
that cycles safely for any step (zero, negative, huge, deterministic), and an
`activityInterval` constant (~100ms).

TASK-2: Wired up the timer in `tui/internal/ui/app.go` and
`tui/internal/ui/detail.go`. Added `spinnerStep`, `spinnerTicking`, and the
`spinnerTickMsg` message to `App`; the `Update` handler advances the step,
threads it into the open detail view (`detailView.spinnerStep`), returns the
next tick only while `anyJDIRunning()` (the same sidecar-first, jdiSeen-
fallback liveness check `jdiAlreadyRunning` already uses, so a just-launched
run with no sidecar yet animates immediately), and returns nil + clears the
guard when the run ends. The chain is started by `startSpinnerIfRunning()`
from three paths: the "j" launch handler (now returns a tick cmd instead of
nil), `refreshJobs`/`refresh` (ctrl+r, returning to list, checkout — the
"run started outside this session" discovery path), and `Init()` (startup
with a run already active). The `spinnerTicking` guard prevents duplicate
concurrent chains. Updated `pollJDIBell`'s doc comment, which previously
claimed there was "no separate timer-driven tick".

TASK-3: Rendered the frame in `jdiStatusBadge` (both call sites — `renderJobRow`
passes `a.spinnerStep`, `renderActionBar` passes `d.spinnerStep`), styled with
the badge's own accentStyle and placed *around* the label (`⠋ [running
@developer]`) so the existing plain-text substring assertions keep passing.
The `[finished]` / `[needs human]` variants render no frame.

TASK-4: Added `activity_test.go` (frame cycling, determinism, any-step
safety), list + detail badge tests (running badge shows a frame that changes
with the step; stopped badges show none; no-status omission unchanged), and
`spinner_test.go` (tick advances the step and continues while running, ends
the chain when the sidecar flips to stopped, produces no cmd with no run, and
the start guard prevents double-scheduling). Updated
`TestJdiKeyLaunchesDetachedAndSeedsBellDedup`, whose "j" handler now
legitimately returns the tick cmd. All pre-existing badge/bell/refresh tests
stay green.

TASK-5: Updated the README's "mg jdi status & log" section — the List-row
badge bullet now mentions the animated indicator next to `[running
@<agent>]`, and the polling sentence notes the one narrow timer exception: a
low-frequency redraw only while a run is running.

## Review follow-up (NEEDS WORK bounce-back)

The reviewer's verdict flagged one regression introduced by TASK-2's return-
value refactor: in `updateList`'s "ctrl+r" case the status line
`fmt.Sprintf("refreshed · %d job(s)", len(a.jobs))` was evaluated *before*
`a.refresh()` re-discovered the jobs, so the footer could show a stale job
count whenever the refresh changed the job list. Fixed by restoring the
original ordering — capture the returned tick cmd, refresh, then read
`len(a.jobs)`:

```go
case "ctrl+r":
    spinnerCmd := a.refresh()
    a.status = fmt.Sprintf("refreshed · %d job(s)", len(a.jobs))
    return a, spinnerCmd
```

Added `TestListCtrlRStatusShowsRefreshedJobCount` in `list_test.go` as a
regression test (a second job created out-of-band must appear in the status
count after ctrl+r). Full suite green (`go test ./...`, `go vet`, `go build`).

## Known issues / follow-ups

none
