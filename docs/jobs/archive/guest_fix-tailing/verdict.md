# Verdict: Fix tailing

id: guest
status: open
reviewer: reviewer
date: 2026-08-28

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed `git diff main...HEAD` on `fix/guest_fix-tailing` (branch matches
brief's `branch:` field; base per `.manigot/manigot.json` = `main`), plus the
full current state of every changed file and the job's own production
artifacts (`session.log`).

TASK-1: PASS
  notes: The "are we actually grabbing the output" question is answered
  with evidence, not just claims:
  - `DockerInvocation.Run` (src/internal/session/docker.go:534-550) wires
    `cmd.Stdout = stdout`; with a non-`*os.File` writer (mg-jdi's
    `bytes.Buffer`) exec.Cmd pipes the child's stdout and forwards every
    chunk — no drops, order preserved. Correct, no production change needed.
  - Pinned by three new tests in src/internal/session/docker_test.go
    (byte-exact capture of an opencode JSONL stream, missing-trailing-newline
    preservation, non-zero-exit capture), and `TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL`
    (src/cmd/mg/jdi_test.go) now asserts the raw JSONL lands verbatim in the
    job's `session.log`.
  - Production artifact confirms it: the job's own
    `docs/jobs/guest_fix-tailing/session.log` holds the full raw opencode
    `--format json` JSONL stream verbatim (step_start/tool_use/step_finish
    events with per-event millisecond timestamps) — the capture path works
    end to end.

TASK-2: PASS
  notes: Live streaming is implemented as specified:
  - src/cmd/mg/jdioutput.go: `appendSessionLog` replaced by `openSessionLog`
    (blank-line separator + section header written BEFORE the invocation),
    a `sessionLog` writer (lastByte tracking; `Close` guarantees the section
    ends with a newline; O_APPEND so concurrent appends can't clobber), and
    `ensureSessionLogFile` (creates the file at run start so the TUI tail
    gate is stable from the moment a run begins).
  - src/cmd/mg/jdi.go: `AgentRunner.Run` takes a `live io.Writer`;
    `commandAgentRunner.Run` tees via `io.MultiWriter(&stdout, live)`; the
    Run loop opens the section before each invocation, closes it after
    (degrading to `io.Discard` + warning when the open fails); every exit
    path goes through `finish()`, whose loop-exit sweep backstops the
    trailing-newline write (which lands after the per-invocation sweep) and
    the container-never-started case. Sweep ordering is documented correctly.
  - Tests: `TestRunWritesSessionLogPerInvocation` (REAL linked worktree +
    clean-tree assertion — the loop-exit sweep must commit the final
    section), `TestRunWritesSessionLogHeaderBeforeInvocation`, adapted
    fake/err runners, and converted `openSessionLog`/`sessionLog` unit tests
    in jdioutput_test.go. The full suite was run by the developer
    (`go test ./...` — all 16 packages ok, recorded in the job's
    session.log) and the changed files compile-clean on inspection (imports
    present, no stale `appendSessionLog`/`runLogExists` references remain).

TASK-3: PASS
  notes: The log-vs-tail distinction is now real:
  - src/internal/ui/app.go "l" case gates on `detail.sessionLogExists()` and
    launches `tail -f <job.Dir>/session.log` (was `job.JDIRunLogPath(...)`),
    status "→ tailing session.log in ...".
  - src/internal/ui/detail.go: `runLogExists` → `sessionLogExists` (stat of
    `<job.Dir>/session.log`); footer hint uses the new gate. Log tab (key 5)
    stays on run.log unchanged.
  - src/internal/ui/tail_test.go: all three tests updated to the session.log
    gate/path/status; `discoverOneJob` creates the job dir so the writes are
    valid.

TASK-4: PASS
  notes: docs/AGENTS.md, README.md (keybinding row, log-tab description,
  "Live tail pane" section), project-template/docs/AGENTS.md (sync rule
  honored), docs/ROADMAP.md (reader side partly landed note) all updated to
  the new log-vs-tail description; the stale `appendSessionLog` reference in
  scripts/entrypoint.sh's comment was synced to `openSessionLog`. No
  out-of-scope changes: the diff touches only the task-listed files plus
  that one-line entrypoint.sh comment sync (same rename, defensible).

## Security

No security findings. The serve `/jdi` endpoint already serves session.log
from the job dir and is untouched by this change (verified:
src/internal/serve/api.go reads `<j.Dir>/session.log` for the tail, and the
file endpoint correctly refuses session.log — behavior unchanged).

## Observations (non-blocking)

1. Mid-run session.log write failure is not fully best-effort: a live-write
   error (e.g. ENOSPC) propagates through `io.MultiWriter` back into
   exec.Cmd's pipe-copy goroutine, which stops copying and can kill the
   container via SIGPIPE — aborting the invocation, contrary to the
   documented "never aborts the loop" intent (which only covers the open
   failure). Extremely low probability (host filesystem failure mid-run) and
   the partial output survives in both session.log and run.log's summary, so
   not a blocker; worth a comment if the tee is ever touched again.
2. A legacy job that only ever ran pre-fix mg-jdi has run.log but no
   session.log, so "l" reports "no mg jdi run has happened for this job yet"
   — slightly misleading for that case, harmless.
3. The job's own working tree currently has an uncommitted developer section
   in session.log (the mg-jdi run that drove the developer did not complete
   its sweep before ending; the reviewer was launched manually). This is an
   interrupted-run artifact, not a defect in the changed code — the pinned
   tests prove a normal run leaves the worktree clean — and the host-side
   session-end sweep commits the remainder.

## Overall

APPROVED

All four tasks are implemented as specified, pinned by tests, and scoped to
the task list. The root cause (tail followed run.log, which only grows at
invocation boundaries) is fixed: the "l" tail now follows the job's
session.log, which mg-jdi streams into live during each invocation, and the
log tab's run.log event summary is untouched. Nothing must change before
merge.