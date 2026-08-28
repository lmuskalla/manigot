# Implementation: Fix tailing

id: guest
status: open
developer: developer
date: 2026-08-28

<!-- Produced by @developer after implementation. -->

## Summary

Fixed the job detail view's tail feature ("l" key) so it actually shows a
live stream instead of duplicating the log tab. Root cause, confirmed by
reading the code: the tail was a `tail -f` of the mg-jdi sidecar `run.log` —
the exact same file the log tab (key 5) renders — and `run.log` only grows at
invocation boundaries (agent invoked / result / next agent invoked), so there
was never anything live to follow. The raw step-level output *was* being
captured (full `--print` stdout) but only persisted post-hoc per invocation.

The fix makes the log-vs-tail distinction real: the log tab stays on
`run.log` (the event summary); the "l" tail now follows the job's own
`session.log` (`docs/jobs/<id>_<slug>/session.log`), which mg-jdi now streams
into **live during each invocation** — section header (agent, attempt,
timestamp) written before the invocation, raw bytes tee'd in as they arrive,
trailing newline guaranteed on close. `session.log` stays in the job's folder
(unchanged location), so the sweep-commit, `mg done` archive-carry,
`mg delete`, and the serve `/jdi` endpoint all keep working untouched.

## Changes

TASK-1: Verified the invocation-output capture end to end and pinned it.
  Reading `DockerInvocation.Run` (internal/session/docker.go): `cmd.Stdout =
  stdout` — when the writer is a non-`*os.File` (mg-jdi's `bytes.Buffer`),
  exec.Cmd pipes the child's stdout and forwards every chunk to the writer:
  no bytes dropped, order preserved; the granularity a tail sees is whatever
  the in-container CLI flushes per write (docker forwards as it arrives;
  opencode's Go CLI writes JSONL events straight to the unbuffered pipe).
  Added `TestDockerInvocationRunCapturesFullStdout`,
  `TestDockerInvocationRunPreservesMissingTrailingNewline` and
  `TestDockerInvocationRunCapturesStdoutOnNonZeroExit` in
  src/internal/session/docker_test.go (fake `docker` emitting an opencode
  JSONL stream; byte-exact capture, including on a non-zero exit — failed
  invocations' output survives too), and extended
  `TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL` in src/cmd/mg/jdi_test.go
  to assert the raw JSONL lands verbatim in the job's `session.log`.

TASK-2: Made mg-jdi stream each invocation's raw output live into the job's
  `session.log`, replacing the post-hoc `appendSessionLog` call.
  - src/cmd/mg/jdioutput.go: `appendSessionLog` replaced by `openSessionLog`
    (opens the file, writes the blank-line separator + section header BEFORE
    the invocation) + a `sessionLog` writer type (tracks the last byte
    written; `Close` guarantees the section ends with a newline) +
    `ensureSessionLogFile` (creates the file at run start so the TUI tail
    gate is stable from the moment a run begins, mirroring `run.log`).
  - src/cmd/mg/jdi.go: `AgentRunner.Run` now takes a `live io.Writer`;
    `commandAgentRunner.Run` tees the container's stdout via
    `io.MultiWriter(&stdout, live)`; Run's loop opens the session-log
    section before each invocation, passes it as the live writer, and closes
    it after (degrading to `io.Discard` + a warning when the open fails).
    The loop-exit sweep comment was updated: the per-invocation sweep now
    runs *after* the live session.log writes and commits each section as it
    goes; the loop-exit sweep remains the backstop for a container that
    never started or a test fake that sweeps nothing.
  - Tests adapted: fake/err runners take the live writer (the fake tees its
    output into it, mirroring the real tee);
    `TestRunWritesSessionLogPerInvocation` updated, new
    `TestRunWritesSessionLogHeaderBeforeInvocation` pins the
    header-before-invocation contract; the `appendSessionLog` unit tests
    became `openSessionLog`/`sessionLog` tests (jdioutput_test.go).

TASK-3: Pointed the detail view's "l" tail at the job's `session.log`.
  - src/internal/ui/app.go: the "l" case now gates on
    `detail.sessionLogExists()` and launches
    `tail -f <job.Dir>/session.log` (was `job.JDIRunLogPath(...)`); status
    text "→ tailing session.log in ..."; comments updated to the
    log-vs-tail distinction.
  - src/internal/ui/detail.go: `runLogExists` renamed to `sessionLogExists`
    (stat of `<job.Dir>/session.log`); the renderFooter "l tail" hint uses
    the new gate.
  - src/internal/ui/tail_test.go: all three tests updated to the
    session.log gate/path/status.
  - The log tab (key 5) stays on `run.log` unchanged — this is what makes
    the log-vs-tail difference real.

TASK-4: Updated the documentation to the new log-vs-tail description:
  docs/AGENTS.md (the "l" key paragraph), README.md ("Watching a run", the
  `l` keybinding row, the log-tab description, and the "Live tail pane"
  section), project-template/docs/AGENTS.md (kept in sync per the hard
  rules), and docs/ROADMAP.md (item 5's "reader side remains open" note:
  the tail reader side has landed; the richer in-TUI structured step-level
  view remains open). Also synced the stale `appendSessionLog` reference in
  scripts/entrypoint.sh's comment (now `openSessionLog`).

## Known issues / follow-ups

- Intermediate agent commits inside the container may capture a
  partially-written session.log section (the live tee means the file grows
  during the run, and the in-container agent's own `git add -A` can stage
  it mid-write). The final file content is always complete — the host-side
  per-invocation sweep commits the rest — so this is cosmetic only (a
  partial section snapshot in an intermediate commit), not data loss.
- The observed streaming granularity depends on the in-container CLI's
  flushing (per-event for opencode's JSONL writes to a pipe; any
  CLI-internal buffering would deliver bursts rather than drops). A live
  opencode run was not available in this environment to measure actual
  per-event flush timing; the capture path is pinned byte-exact by tests.
- The serve `/jdi` endpoint and the ROADMAP's richer in-TUI step-level view
  remain as documented — out of scope here.