# Implementation: more log for jdi

id: rj4prf
status: open
developer: claude
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

Implemented all six tasks from `tasks.md`. `mg jdi`'s run.log/stdout now
reports four new timestamped events in addition to the pre-existing
per-invocation output — "started", "agent invoked", "agent finished"
(reworded from the old ambiguous header), and "job finished" — plus a dedup
step so an agent's response text isn't printed twice when it's just an echo
of the job file it already wrote.

TASK-1: Added `logStarted(w, jobName, profile string)` in `output.go` and
call it in `main()` right after `logDest` is finalized, before `Run` is
invoked — the very first line written to both stdout and run.log for a run.

TASK-2: Added `logAgentInvoked(w, agent string, attempt int)` in
`output.go`, called from `Run` immediately before `runner.Run(...)`, reusing
the same `i+1` attempt number the post-run header already used.

TASK-3: Reworded `logInvocation`'s header from
`=== <timestamp> <agent> (attempt N) ===` to
`=== <timestamp> <agent> finished (attempt N) ===`, so it now reads as the
"finished" half of an invoked/finished pair. Updated the existing
header-substring assertions in `output_test.go` to match.

TASK-4: Added `logJobFinished(w, kind orchestrate.Kind, reason string,
includeReason bool)` in `output.go`, called once at every exit point of
`Run`'s `finish` closure. `includeReason` is `false` only for the
stop-before-any-agent-ran case (Stage() == StageDefine, an unwritten
brief.md) — `logImmediateStop` has already printed `reason` immediately
above in that case, so `logJobFinished` still fires (for a consistent "job
finished" marker either way) but doesn't repeat the same reason text a
second time in a row.

TASK-5: `logInvocation`'s signature grew two parameters, `targetFile` and
`targetContent` — the just-run agent's expected job file
(`agentTargetFile`: analyst → tasks.md, developer → implementation.md,
reviewer → verdict.md) and its content, read fresh off disk via `j.Dir` in
`Run` right after the invocation returns (safe per `ensureOnBranch`'s own
guarantee that `mg jdi` always operates on the job's own checked-out
branch). A new `isDuplicateOutput` helper does a trimmed,
whitespace-normalized *substring* comparison (not exact equality, since a
real response typically echoes the file content plus its own surrounding
commentary) — when it matches, the "finished" header is unchanged but the
body is replaced with `(output matches <file>, omitted)`. An empty
`targetContent` (file doesn't exist yet, or unreadable) never counts as a
duplicate, so the dedup check degrades to a no-op rather than a false
positive.

TASK-6: Added two end-to-end tests in `main_test.go`
(`TestRunFullLogSequenceHappyPath`, `TestRunFullLogSequenceDedupsMatchingOutput`)
asserting the complete started → invoked → finished → ... → job finished
sequence appears in order in the log, for both a normal run and the TASK-5
dedup case, via a new `assertLogOrder` test helper.

## Changes

- `tui/cmd/jdi/output.go`:
  - New `logStarted`, `logAgentInvoked`, `logJobFinished` helpers.
  - `logInvocation`'s header reworded to "finished"; signature extended
    with `targetFile, targetContent string`; new `agentTargetFile` map and
    `isDuplicateOutput`/`normalizeWhitespace` helpers for the dedup check.
- `tui/cmd/jdi/main.go`:
  - `main()`: calls `logStarted` before `Run`.
  - `Run`: calls `logAgentInvoked` before each invocation; `finish` closure
    now takes an `includeReason bool` and calls `logJobFinished`; reads the
    just-run agent's target file content and passes it to `logInvocation`.
  - New `path/filepath` import for the target-file read.
- `tui/cmd/jdi/output_test.go`: updated existing `logInvocation` call sites
  for the new signature/wording; added tests for `logStarted`,
  `logJobFinished`, the dedup path (`isDuplicateOutput` and
  `logInvocation`'s omission behavior).
- `tui/cmd/jdi/main_test.go`: added `TestRunLogsAgentInvoked`,
  `TestRunLogsJobFinishedOnNormalStop`,
  `TestRunImmediateStopDoesNotDuplicateReason`,
  `TestRunFullLogSequenceHappyPath`,
  `TestRunFullLogSequenceDedupsMatchingOutput`, and the `assertLogOrder`
  helper.

No TUI changes were needed — `tui/internal/ui/detail.go`'s log tab just
displays `run.log`'s raw tail, with no header-specific parsing to update.

## Follow-up: process blocker from verdict.md

`verdict.md` (NEEDS WORK) flagged that this job's `tasks.md` — the real
analyst content (Context, TASK-1..6, open question) — was never committed to
this branch, only sitting uncommitted in the working tree, risking being
lost on merge. Fixed: committed `docs/jobs/rj4prf_more-log-for-jdi/tasks.md`
on its own (`8713ee5`), verified via `git status` that it's now clean for
this job's files. `go build ./...`, `go vet ./...`, and `go test ./...` in
`tui/` still all pass unchanged — no code was touched, only the missing
commit.

The verdict's second point — an uncommitted, unrelated `docs/NAMING.md`
"Parking lot" change also sitting in the working tree — was deliberately
left untouched. It predates this job, isn't part of `brief.md`'s scope (log
verbosity for `mg jdi`), and doesn't exist on any other local or remote
branch, so committing or discarding it isn't this job's call to make; it
was excluded from the `tasks.md` commit above by staging that file
specifically rather than `git add -A`. It's still sitting uncommitted in the
working tree and needs a human decision (commit on its own unrelated
branch/commit, or discard).

## Known issues / follow-ups

- `docs/NAMING.md`'s uncommitted "Parking lot" section (see above) is still
  unresolved and out of scope for this job — needs a human to commit it
  elsewhere or discard it.
- TASK-5's substring-containment heuristic is exactly what `tasks.md`'s open
  question flagged as negotiable; it wasn't revisited here since nothing in
  implementation surfaced a case where it doesn't work as intended, but real
  agent output in practice may still warrant a different comparison
  strategy later.
