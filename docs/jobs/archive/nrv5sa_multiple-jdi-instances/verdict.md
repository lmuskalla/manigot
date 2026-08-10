# Verdict: Multiple jdi-instances

id: nrv5sa
status: open
reviewer: claude (correctness re-review, post-fix)
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

This is a re-review after the developer's post-verdict fix for the single
blocker raised in the first pass (`jdiAlreadyRunning`'s unbounded `jdiSeen`
fallback). Commits `9dbcc68` (TASK-1 fix) and `7e4aeed` (implementation.md
update) since then.

TASK-1: PASS
notes: `tui/internal/ui/app.go`. The fix adds a parallel `jdiSeenAt map[string]time.Time`
(~L98), written at both existing `jdiSeen` seed sites — `pollJDIBell`
(~L499, only reached when a sidecar actually exists) and the `"j"` handler's
launch-time seed (~L756). `jdiAlreadyRunning` (~L528) now only trusts the
`jdiSeen` fallback for `jdiSeenFallbackTTL` (2 minutes, ~L520) via an
indirected `jdiNow` var, mirroring the existing `ringBell` override pattern
for testability. Traced the exact scenario the first verdict reproduced —
`jdiSeen[job] = JDIRunning` with no sidecar file at all (simulating a
crashed/killed mg-jdi that never got to write its first status) — and
confirmed it now self-heals after 2 minutes instead of blocking for the rest
of the TUI session. Checked for regressions in the surrounding logic:
- `pollJDIBell` only reaches the `jdiSeenAt` write when `job.ReadJDIStatus`
  returns `ok=true` (a sidecar exists), so it can never mask the crash case —
  `jdiAlreadyRunning`'s fallback branch is only ever consulted when no
  sidecar exists, and in that branch `jdiSeenAt` can only have been set by
  the launch-time seed, which is exactly what the TTL is meant to bound.
- The TTL doesn't interact badly with `job.ReadJDIStatus`'s own 30-minute
  sidecar staleness window (`jdiRunningStaleAfter`): by the time a sidecar
  degrades to `ok=false` on staleness grounds, the corresponding `jdiSeenAt`
  (set no later than launch time) would already be far past the 2-minute TTL
  too, so there's no window where the sidecar is "trusted but stale" and the
  fallback disagrees.
- The 2-minute value is justified in the comment (`cmd/jdi/main.go` writes
  its first status before invoking any agent — normally a git checkout,
  seconds not minutes) and is plausible; not independently re-verified
  against `cmd/jdi/main.go`'s actual startup sequence, but the reasoning
  holds and the number is conservative in the right direction (short enough
  to recover quickly, long enough not to reintroduce the original race).

TASK-3: PASS
notes: `tui/internal/ui/jdilaunch_test.go`, new
`TestJdiKeyAllowsLaunchAfterInSessionDedupExpiresWithNoSidecar` (~L258) is
exactly the regression test the first verdict asked for: seeds `jdiSeen` =
`JDIRunning` with `jdiSeenAt` set to `now - TTL - 1s` and no sidecar file on
disk, freezes `jdiNow` for determinism (no real sleep), presses "j", and
asserts both that the status text no longer says "already running" and that
the stub `mg-jdi` binary actually ran once. Correctly distinguishes this
from the pre-existing "on-disk stopped status overrides stale jdiSeen" test
— this one has no sidecar at all, which is the specific gap that was found.
Ran the full suite (`go build ./...`, `go vet ./...`, `gofmt -l .`,
`go test ./...`) from `tui/`: all seven `Jdi`-prefixed tests in
`internal/ui` pass, along with the rest of the module; `go vet` and
`gofmt -l` are clean.

TASK-2 / TASK-4: unchanged since the first pass — still PASS, not touched by
this fix (verified via `git diff 079b0ad..HEAD --stat`: only
`tui/internal/ui/app.go`, `tui/internal/ui/jdilaunch_test.go`, and this job's
`implementation.md` changed).

Commit discipline: `9dbcc68 [nrv5sa] TASK-1: expire stale jdiSeen fallback
entries with no sidecar` and `7e4aeed [nrv5sa] implementation: document
post-verdict jdiSeen expiry fix` — correct format, each with its own commit,
matching the pattern established in the first pass. No unrelated files
touched; scope stayed exactly to the one blocker raised.

## Security

none — no security review requested; no new attack surface (the fix only
adds an in-memory timestamp map and a time-indirection var, no new external
input or persisted state).

## Overall

<!-- APPROVED / REJECTED / NEEDS WORK -->
APPROVED

The one blocker from the first pass — the unbounded `jdiSeen` fallback that
could permanently block a relaunch after a crashed/killed mg-jdi process
with no sidecar and no visible indicator anywhere — is fixed with a bounded,
tested TTL. All four tasks now PASS. No other issues found in this pass.
