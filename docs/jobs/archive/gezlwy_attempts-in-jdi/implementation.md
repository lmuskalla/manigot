# Implementation: attempts in jdi

id: gezlwy
status: open
developer: claude
date: 2026-08-12

## Summary

Changed `mg jdi`'s log attempt numbering from a run-wide counter (the loop
index `i+1`, so a happy path logged analyst 1 → developer 2 → reviewer 3 and
a bounce logged the developer's second call as attempt 4) to a per-agent
counter, so every agent's first call is attempt 1 and a bounced-back agent's
second call is its own attempt 2. The number is presentational only; no
logic branches on it, so the change is confined to `Run`'s counter, the two
log header functions' call sites, their doc comments, the affected tests,
and the README's log-tab description.

## Changes

TASK-1: Introduced a `map[string]int` attempt counter keyed by agent name in
`Run` (`cmd/mg/jdi.go`), seeded at 0 before the loop, incremented once per
invocation for `decision.Agent`, and passed to both `logAgentInvoked` and
`logInvocation` in place of `i+1`. The `i` loop index is untouched — it still
drives the `maxIterations` cap. The invoked/finished headers of one
invocation keep sharing the same number (the invariant
`TestRunLogsAgentInvoked` documents). Updated the doc comments on `Run`
(`cmd/mg/jdi.go`) and on `logAgentInvoked` / `logInvocation`
(`cmd/mg/jdioutput.go`) to state that the attempt counts that agent's
invocations within the run — per agent, not per run.

TASK-2: Updated the three happy-path log assertions in
`cmd/mg/jdi_test.go` that hard-coded the old global numbering —
`TestRunLogsAgentInvoked`, `TestRunFullLogSequenceHappyPath` and
`TestRunFullLogSequenceDedupsMatchingOutput` — from `developer (attempt 2)`
/ `reviewer (attempt 3)` to `attempt 1` for both (analyst was already
attempt 1). The `assertLogOrder` lists are otherwise unchanged. No
assertion in `cmd/mg/jdioutput_test.go` needed changes — those tests pass
explicit attempt numbers straight to the log functions.

TASK-3: Extended `TestRunOneBounceThenApproved` in `cmd/mg/jdi_test.go` with
the bounce-path regression coverage that motivated the brief: it now
captures the log and asserts, in order, `developer invoked/finished
(attempt 1)` → `reviewer invoked/finished (attempt 1)` → `developer
invoked/finished (attempt 2)` → `reviewer invoked/finished (attempt 2)` —
the developer's second call is attempt 2, not attempt 4. The existing
call-sequence assertion (`{"developer","reviewer","developer","reviewer"}`)
is untouched, as is the fake runner's own `call` parameter (test content
only).

TASK-4: Updated the README's log-tab description (the "one section per agent
invocation, with a timestamp/agent/attempt header" sentence) to note that
the attempt number counts that agent's invocations within the run — per
agent, not per run. Verified end to end: `go build ./...`, `go test ./...`
(all packages pass) and `go vet ./...` from the module root.

Also: committed the analyst's task breakdown (`docs/jobs/gezlwy_attempts-in-jdi/tasks.md`),
which was present as uncommitted work in the worktree, as its own commit so
the spec is part of the branch history.

## Known issues / follow-ups

- The per-agent counter resets at the start of every `mg jdi` run, matching
  the old behavior (the loop index restarted at 0). Since `run.log` is
  append-only across runs, a later run of the same job shows attempt 1 again
  for each agent. This was the analyst's scope decision in tasks.md
  ("not persisting counters across runs is the conservative reading of the
  brief"); flagged here for @reviewer in case the brief intended counters to
  persist across runs.
