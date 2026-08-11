# Tasks: loading indicator for jdi

id: o1y8oo
status: open
analyst: deepseek-v4-flash
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Context

`mg jdi`'s presence in the TUI is a single static tag: `[running @analyst]`
(plus `[finished]` / `[needs human]`), rendered by the shared
`jdiStatusBadge` helper (`tui/internal/ui/app.go`) in two places — the
job-list row (`renderJobRow`) and the detail-view action bar
(`renderActionBar`, right next to the "[j] just do it" button). The tag
updates only when Bubble Tea happens to re-render (keypress, ctrl+r,
returning to the list): nothing ever *moves*, so a user watching a run can't
tell it's alive. The log tab is there but requires opening the job and the
tab.

This job adds an animated activity indicator — the same idea as the small
spinner opencode shows in its bottom-left corner — rendered next to the
`[running @<agent>]` badge, in both places the badge already appears.

Two facts shape the design:

1. The TUI currently has **no timer-driven redraw at all** (see
   `pollJDIBell`'s doc: "there is no separate timer-driven tick"). An
   animation needs one, so this job introduces the app's first tick — but
   deliberately scoped: it runs only while a JDI run is active, and stops as
   soon as nothing is running, preserving the current idle behavior (no
   redraws, no CPU churn).
2. The animation frame must be visible to both badge call sites, but
   `detailView` is constructed without an App reference
   (`newDetailView(j, width, height)`), so the current frame has to be
   threaded from the App into the open detail view.

## Task breakdown

TASK-1: Build the reusable activity-indicator component.
     Create `tui/internal/ui/activity.go`: a small, pure, Bubble-Tea-free
     spinner helper — a frame set (braille dots, e.g.
     `⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏`, the same family opencode's bottom-left
     indicator uses) plus `activityFrame(step int) string` that cycles
     safely through the frames for any step (zero, negative, huge), and an
     `activityInterval` constant (~100ms per frame). It lives in the ui
     package (its only consumer) as a self-contained file with no imports
     beyond stdlib, so it is trivially unit-testable and re-usable by any
     future UI spot (this is the "re-usable component" the brief asks for —
     a standalone `internal/` package for ~25 lines would be overkill).
     files: tui/internal/ui/activity.go (new)
     depends: none
     risk: low — new, isolated file; nothing else imports it yet.

TASK-2: Drive the animation with a narrowly-scoped Bubble Tea tick.
     Add a `spinnerStep int` field and a `spinnerTickMsg` to `App`;
     `Update` handles `spinnerTickMsg` by advancing `spinnerStep` and
     returning the next tick only while any job is still JDIRunning (via
     `job.ReadJDIStatus` over `a.jobs`, plus the existing
     `jdiSeen`/`jdiSeenAt` fallback `jdiAlreadyRunning` already uses, so a
     just-launched run with no sidecar file yet animates immediately).
     Start the chain from the "j" launch handler in `updateDetail`
     (currently returns `a, nil` on success — return a tick cmd instead),
     and ensure a run discovered any other way (ctrl+r refresh, startup
     when a run is already active) also starts it — a `spinnerTicking bool`
     guard on `App` prevents duplicate concurrent tick chains. The tick
     must terminate when the run ends (sidecar flips to a stopped state:
     return nil, clear the guard). Thread the current step into the open
     detail view: add a `spinnerStep int` field to `detailView`, set from
     the App (tick handler is the natural place), defaulting to 0.
     Also update `pollJDIBell`'s doc comment, which now over-claims "no
     separate timer-driven tick".
     files: tui/internal/ui/app.go, tui/internal/ui/detail.go
     depends: TASK-1 (uses activityFrame/activityInterval)
     risk: medium — the app's first timer-driven redraw; must not tick when
     idle, must not double-schedule ticks, and the running→stopped
     transition must end the chain (otherwise the spinner spins forever
     against a dead run).

TASK-3: Render the spinner next to the running badge.
     Extend the shared `jdiStatusBadge` helper (or restructure it to return
     the label plus a "running" flag and compose at the call sites — either
     is fine, keep one shared path so both places can't drift) so that when
     state == JDIRunning the rendered badge includes the animated frame,
     e.g. `⠋ [running @analyst]`, styled like the badge itself
     (accentStyle). The `[finished]` / `[needs human]` variants stay
     exactly as they are — no spinner, nothing is happening. Call sites:
     `renderJobRow` (pass `a.spinnerStep`) and `renderActionBar` (pass
     `d.spinnerStep`). The existing plain-text contract must survive: tests
     assert on `"running @developer"` — add the glyph *around* the label,
     don't replace it.
     files: tui/internal/ui/app.go, tui/internal/ui/detail.go
     depends: TASK-1, TASK-2 (needs the threaded step)
     risk: low — pure rendering change; existing substring assertions keep
     passing; non-running states untouched.

TASK-4: Tests for the component and the animation.
     New `activity_test.go`: frame cycling (frame 0 ≠ frame 1, wraps at the
     frame count), determinism, any-step safety. Badge tests: a running
     badge renders a spinner frame next to `[running @...]` in both the
     list row and the detail action bar; finished/needs-human badges render
     no spinner frame; the two "omits badge when no status" tests still
     assert full omission. Tick tests: feeding `spinnerTickMsg` advances
     `App.spinnerStep` and returns a next-tick cmd while a job is running;
     after the sidecar flips to a stopped state the handler returns nil
     (chain ends); with no run at all no tick cmd is produced. Existing
     tests to keep green: `TestRenderListShowsJDIRunningBadge`,
     `TestDetailActionBarShowsJDIRunningBadge`, both "omits badge" tests,
     and `refresh_test.go`'s bell test (drives `refreshJobs` directly — the
     new tick must not interfere with it).
     files: tui/internal/ui/activity_test.go (new),
       tui/internal/ui/detail_test.go, tui/internal/ui/list_test.go,
       possibly a new spinner-tick test file
     depends: TASK-2, TASK-3
     risk: low — test-only; the only care point is the bell/refresh tests
     must not start animating spuriously.

TASK-5: Update the README.
     The "mg jdi status & log" section (~line 661) currently says the badge
     and log tab are "polled the same refresh-triggered way as everything
     else in the TUI ... no separate live-streaming subsystem". That is no
     longer fully accurate while a run is active. Update the List-row badge
     bullet to mention the animated indicator next to `[running @<agent>]`,
     and adjust the polling sentence to note the one narrow timer exception:
     a low-frequency redraw only while a run is running.
     files: README.md
     depends: TASK-3 (documents the final behavior)
     risk: low — documentation-only.

## Out of scope

- The `mg jdi` CLI's own output (`tui/cmd/jdi/main.go`, `output.go`): it is
  headless (JSON/JSONL for `--print`, plus a plain-text log fan-out to
  `run.log`), and a spinner there would corrupt the parse stream and the
  log. The brief's "indicates things are happening" is about the TUI.
- Changing the sidecar status format, `tui/internal/job/jdistatus.go`, or
  the job package — the animation is purely client-side.
- Launch mechanics (`launch.Jdi`), the bell/notification logic, or the
  `jdiSeen` dedup.
- Adding any *new information* beyond the existing badge — the spinner is
  purely cosmetic; the badge already names the active agent.
- Archived job docs under `docs/jobs/archive/**`.

## Open questions for @developer

- Frame set: braille (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) matches the opencode reference
  and reads best, but a few terminals/encodings render braille as boxes; the
  ASCII `|/-\` set is the maximally compatible fallback. Keep whichever is
  chosen in one constant so it's a one-line change. Default suggestion:
  braille.
- Spinner position relative to the badge (`⠋ [running @analyst]` vs
  `[running @analyst] ⠋`): purely aesthetic, pick one and keep it consistent
  in both call sites.
- Cadence: ~100ms/frame is suggested; anything ≥ 60ms is fine — slower
  reads calmer, don't go below 60ms or the cycle looks frantic.
