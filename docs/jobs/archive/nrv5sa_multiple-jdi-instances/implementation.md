# Implementation: Multiple jdi-instances

id: nrv5sa
status: open
developer: claude
date: 2026-08-10

<!-- Produced by @developer after implementation. -->

## Summary

Pressing "j" in the job detail view now blocks a second `mg-jdi` launch
against the same job while one is already running, and the detail view
itself shows a live running/stopped indicator next to the `[j] mg-jdi`
button (previously this was only visible on the job-list row).

## Changes

TASK-1: Added `App.jdiAlreadyRunning` (`tui/internal/ui/app.go`), which
combines the on-disk sidecar status (`job.ReadJDIStatus`, the source of
truth once mg-jdi has written it) with the in-session `a.jdiSeen` dedup map
(the fallback for the brief window between `launch.Jdi`'s `Start()`
returning and mg-jdi's own first status write). `updateDetail`'s `"j"` case
now checks this before calling `launch.Jdi`; when it reports the job is
already running, it sets an explanatory footer status naming the running
agent (e.g. "mg-jdi is already running for this job @developer") instead of
starting a second process. The on-disk status always wins over a stale
`jdiSeen` entry, so a job is never permanently blocked once mg-jdi actually
stops.

TASK-2: `detailView.renderActionBar` (`tui/internal/ui/detail.go`) now
appends the same `jdiStatusBadge` formatting the job-list row already uses
(already a package-level function in `app.go`, callable from both files
without further extraction) to the stage line, right after the `[j] mg-jdi`
button — so the running/finished/needs-human state is visible from inside
the detail view itself, not only the list.

TASK-3: Added three tests to `tui/internal/ui/jdilaunch_test.go` using a
marker-writing stub `mg-jdi` binary: an on-disk `JDIRunning` sidecar blocks
a second launch and names the agent in the status text
(`TestJdiKeyBlocksSecondLaunchWhenSidecarSaysRunning`); a press landing
before any sidecar file exists is still caught via the in-session dedup map
(`TestJdiKeyBlocksSecondLaunchViaInSessionDedup`); and an on-disk stopped
status always overrides a stale `jdiSeen` entry, allowing the launch
(`TestJdiKeyAllowsLaunchWhenSidecarStoppedDespiteStaleSession`).

TASK-4: Added four tests to `tui/internal/ui/detail_test.go` asserting
`renderActionBar` shows the running (with agent name), finished, and
needs-human badge variants when a sidecar status exists, and shows no badge
at all (while the plain `[j] mg-jdi` button label stays present) when there
is no sidecar yet.

## Post-verdict fix

The first review pass (`verdict.md`) found that `jdiAlreadyRunning`'s
fallback to `a.jdiSeen` had no expiry of its own: a launch whose mg-jdi
process crashed or was killed before ever writing its first sidecar status
file left a `JDIRunning` entry in `jdiSeen` that blocked "j" forever within
the TUI session, with no badge anywhere (list row or, per TASK-2, the detail
view) showing anything as actually running — recoverable only by restarting
the TUI.

Fixed in `tui/internal/ui/app.go`: `jdiSeen` entries are now timestamped in a
parallel `jdiSeenAt` map, written at both existing seed sites (`pollJDIBell`'s
disk-read dedup and the `"j"` handler's launch-time seed). `jdiAlreadyRunning`
only trusts the `jdiSeen` fallback for `jdiSeenFallbackTTL` (2 minutes — long
enough to bridge the real "just launched, sidecar not written yet" race,
since `cmd/jdi/main.go` writes its first status before invoking any agent,
short enough to recover within the same session from a launch that never got
the chance to write a sidecar at all). Time is read via an indirected
`jdiNow` var (mirroring the existing `ringBell` override pattern) so tests
can simulate the TTL elapsing without an actual sleep.

Added `TestJdiKeyAllowsLaunchAfterInSessionDedupExpiresWithNoSidecar` to
`tui/internal/ui/jdilaunch_test.go`, the regression test the verdict asked
for: an expired `jdiSeen` entry with no sidecar file on disk at all now
allows a fresh launch instead of blocking indefinitely.

## Known issues / follow-ups

- The detail-view indicator, like the existing list-row badge, only
  refreshes on the next Bubble Tea render (any keypress) — there is no
  polling timer, per this project's "no new event-streaming subsystem"
  constraint. This is an existing, accepted limitation carried over from the
  list badge, not something new introduced here.
- Blocking is scoped per job, matching the existing per-job sidecar file:
  running `mg-jdi` for two different jobs at once is unaffected. This
  matches the brief's wording ("press j ... multiple times" against the
  same job) and `tasks.md`'s explicit scoping note; a global (all-jobs)
  limit was not implemented.
- none otherwise.
