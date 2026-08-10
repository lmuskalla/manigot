# Tasks: Multiple jdi-instances

id: nrv5sa
status: open
analyst: claude (architect pass)
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Context

The "j" key in the job detail view (`tui/internal/ui/app.go`, `updateDetail`'s
`"j"` case) calls `launch.Jdi` unconditionally on every press. `launch.Jdi`
starts a detached `mg-jdi --job <id>` process with no window and returns
immediately, so nothing currently stops a second (third, ...) press from
starting another concurrent `mg-jdi` against the very same job while the
first is still running — the brief's "spawn several processes" / "you have
no idea you're doing that".

The status-reporting side of this already exists from a prior job (`docs/AGENTS.md`'s Decision 4/4a/7a,
`tui/internal/job/jdistatus.go`, `jdiStatusBadge` in `app.go`): mg-jdi writes
a `running` / `stopped:finished` / `stopped:needs-human` sidecar status file
per job, and the **job list row** already renders a `[mg-jdi: running @agent]`
style badge from it. Two gaps remain, matching the brief's two asks:

1. That badge only renders on the list row — the detail view (where "j" is
   actually pressed) shows nothing ongoing beyond the one-time, easily-missed
   "→ mg-jdi started in the background" footer status set at launch, which
   then just sits there unchanged whether the run is still going or long
   finished.
2. Nothing reads that status back before launching — the guard the brief
   asks for ("further invocations need to be blocked") doesn't exist at all.

## Task breakdown

TASK-1: Block a second `mg-jdi` launch for a job that's already running one —
in `updateDetail`'s `"j"` case (`tui/internal/ui/app.go`), read the job's
current status before calling `launch.Jdi` and, when it reports `JDIRunning`,
skip the launch and set an explanatory footer status instead (e.g. naming the
agent currently running, per the existing badge's wording) rather than
starting a second process. The on-disk sidecar (`job.ReadJDIStatus`) is the
source of truth, but it is only written by mg-jdi itself at each
agent-invocation boundary, not the instant the process starts — so a press
immediately following a just-launched run (before that process has written
its first `running` status file) must also be caught, most likely by also
consulting the in-memory `a.jdiSeen` dedup map this same handler already
seeds on launch. Getting the combination of "on-disk say running" OR
"we just launched it ourselves and haven't seen it stop yet" right is the
crux of this task.
files: tui/internal/ui/app.go
depends: none
risk: medium — a real, easy-to-get-wrong race between "just launched,
sidecar not written yet" and "genuinely finished/stale"; too loose still
allows the double-launch this job exists to fix, too strict permanently
blocks re-running a job after mg-jdi actually stops.

TASK-2: Show a live running/stopped indicator inside the detail view itself,
not only the job-list row — e.g. alongside the existing `[j] mg-jdi` button
in the action bar (`renderActionBar`, `tui/internal/ui/detail.go`) — reusing
the same `job.ReadJDIStatus` data and wording the list row's `jdiStatusBadge`
(`app.go`) already renders, so a user sitting in the detail view (where "j"
is pressed and TASK-1's block message appears) can see at a glance whether
mg-jdi is still going, rather than only the job list. Likely wants
`jdiStatusBadge` extracted somewhere both `app.go`'s list rendering and
`detail.go` can call, to avoid two copies of the same formatting drifting
apart. Note there is no polling timer in this TUI (`docs/AGENTS.md`: "no new
event-streaming subsystem" constraint) — the indicator reads the sidecar
fresh on every `render()` call the same way the list badge already does, so
it updates whenever Bubble Tea next re-renders (any keypress), not
continuously; that's an existing, accepted limitation of the list badge too,
not a new one.
files: tui/internal/ui/detail.go, tui/internal/ui/app.go (to share/extract
jdiStatusBadge's formatting)
depends: none (independent of TASK-1, though both touch the same "j" area)
risk: low — purely additive rendering on an already-tested read path; main
risk is fitting it into the existing narrow-terminal action-bar truncation
logic (`renderActionBar`'s width budget) without breaking that layout.

TASK-3: Test coverage for the new block: pressing "j" while the sidecar (or
the in-session dedup) reports the job already running does not invoke
`launch.Jdi` again and instead sets an explanatory status; a stopped or
absent status still allows launching as before.
files: tui/internal/ui/jdilaunch_test.go
depends: TASK-1
risk: low — test-only, but must deterministically exercise both the
on-disk-status and just-launched-in-session race branches from TASK-1 (via
`job.WriteJDIStatus`, matching the pattern `list_test.go` already uses for
the badge tests).

TASK-4: Test coverage for the new detail-view indicator: renders the running
badge when the sidecar says `JDIRunning` (including the agent name), the
finished/needs-human variants after a stop, and nothing when there's no
sidecar for the job yet.
files: tui/internal/ui/detail_test.go
depends: TASK-2
risk: low — test-only, mirrors the existing list-badge tests in
`list_test.go` (`TestRenderListShowsJDIRunningBadge` /
`TestRenderListShowsJDINeedsHumanBadge`).

## Out of scope (per brief, and flagging for confirmation)

- Blocking is scoped per job (matching the existing per-job sidecar file) —
  running mg-jdi for two *different* jobs at once is unaffected. The brief's
  wording ("press j ... multiple times", "several processes") reads as
  repeated presses against the same job; if a global (all-jobs) limit was
  actually intended instead, that changes TASK-1's design and should be
  confirmed before implementation.
- The brief's "Why" and "Out of scope" sections in brief.md are still blank —
  nothing above depends on them, but worth the requester filling in before
  `@developer` picks this up.
