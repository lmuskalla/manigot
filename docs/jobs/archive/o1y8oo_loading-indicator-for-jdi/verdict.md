# Verdict: loading indicator for jdi

id: o1y8oo
status: open
reviewer: deepseek-v4-flash
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `tui/internal/ui/activity.go` is a self-contained, pure, stdlib-only
spinner helper exactly per spec: braille frame set in one `activityFrames`
var for a one-line swap, `activityFrame(step)` cycles safely for any step
(zero, negative, huge — modulo, with negative wrap), `activityInterval` =
100ms. `activity_test.go` covers cycling, determinism, and any-step safety.

TASK-2: PASS
notes: `App` gained `spinnerStep`, `spinnerTicking`, and `spinnerTickMsg`
(app.go ~line 392). The `Update` handler advances the step, threads it into
the open detail view, continues the chain only while `anyJDIRunning()`
(sidecar-first via `job.ReadJDIStatus`, with the `jdiSeen`/`jdiSeenAt`
fallback `jdiAlreadyRunning` already uses, so a just-launched run with no
sidecar animates immediately), and returns nil + clears the guard when the
run ends. The chain starts from the "j" handler (`updateDetail`), from every
refresh/discovery path (`refreshJobs`/`refresh` — ctrl+r, esc, done, delete,
checkout, and the return-to-list transitions), and from `Init()` for a run
already active at startup; `spinnerTicking` prevents duplicate chains, and
the guard is re-asserted on the continuing tick path so "guard set" stays
equivalent to "chain alive". `pollJDIBell`'s doc no longer over-claims "no
separate timer-driven tick". The single regression this introduced in the
first pass — `updateList`'s "ctrl+r" status line reading `len(a.jobs)`
*before* `a.refresh()` (stale job count) — is fixed in becf7a4: the cmd is
captured, refresh runs first, the count is read after, and
`TestListCtrlRStatusShowsRefreshedJobCount` (list_test.go) locks it in.

TASK-3: PASS
notes: `jdiStatusBadge(root, j, spinnerStep)` renders `⠋ [running @<agent>]`
styled with `accentStyle` for the running state, glyph *around* the label so
the existing `"running @developer"` substring assertions keep passing.
`renderJobRow` passes `a.spinnerStep`; `renderActionBar` passes
`d.spinnerStep` (detail.go). The `[finished]` / `[needs human]` variants
render no frame. Both call sites verified by the new list/detail badge tests.

TASK-4: PASS
notes: `activity_test.go`, `spinner_test.go`, and the additions to
`detail_test.go` / `list_test.go` cover frame cycling, running-badge-with-
frame in both call sites (frame changes with the step), no-frame for the
stopped states, full badge omission when there is no status, and the tick
chain: advance + continue while running (with detail threading), termination
when the sidecar flips to stopped, no cmd with no run, and the
no-double-start guard. `jdilaunch_test.go`'s updated expectation (the "j"
handler now returns the tick cmd) is a legitimate consequence of TASK-2. The
bell/refresh tests in `refresh_test.go` call the now-`tea.Cmd`-returning
`refreshJobs`/`refresh` as statements (legal; the discarded tick cmd means no
spurious chain in those tests). Verified myself: `go build ./...`,
`go vet ./...`, and `go test ./...` all clean; the ui package passes 3
repeated runs (no flake).

TASK-5: PASS
notes: README "mg jdi status & log" section updated — the List-row badge
bullet mentions the animated indicator (`⠋ [running @<agent>]`) and the
polling sentence notes the one narrow timer exception (a low-frequency redraw
only while a run is active). Accurate w.r.t. the implementation.

## Security

None. The change is client-side rendering plus a Bubble Tea timer; no new
process, network, or file I/O surface beyond reading the existing sidecar via
the pre-existing `job.ReadJDIStatus`. No secrets, no new files written.

## Overall

APPROVED

All five tasks are implemented as specified, the previous review's single
blocker (the ctrl+r status-line regression) is fixed and regression-tested,
the full suite is green and stable, and there is no out-of-scope change. The
only minor note, not a blocker: the TASK-4 commit bundles a 3-line guard-
assertion addition to the tick handler alongside its tests — but that logic
is squarely part of TASK-2's guard semantics and is documented in the code,
so it is not a scope violation.
