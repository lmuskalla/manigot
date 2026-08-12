# Tasks: attempts in jdi

id: gezlwy
status: open
analyst: claude
date: 2026-08-12

<!-- Produced by @analyst from brief.md. -->

## Context

mg-jdi's log output (both its stdout for a direct CLI run and the sidecar
`.manigot/jdi-status/<job>/run.log`, which share one writer — see
`cmd/mg/jdi.go`'s `io.MultiWriter` and the `sectionWriter` wrapper in
`cmd/mg/jdioutput.go`) labels every agent invocation with an attempt number:

```
=== <timestamp> <agent> invoked (attempt N) ===      (logAgentInvoked, jdioutput.go:179)
=== <timestamp> <agent> finished (attempt N) ===     (logInvocation, jdioutput.go:271)
```

The number is computed in `Run` (`cmd/mg/jdi.go`) as the loop index `i+1`
(jdi.go:365 for the invoked header, jdi.go:392 for the finished header) — a
*global* counter across every agent invocation in the run. So a happy path
logs analyst (attempt 1) → developer (attempt 2) → reviewer (attempt 3), and
a review bounce logs the developer's *second* call as attempt 4.

The brief: the attempt number should be PER agent. The analyst's first call
is attempt 1, the developer's first call is attempt 1, the reviewer's first
call is attempt 1 — and a bounce back to the developer is that developer's
attempt 2, not attempt 4. The number is presentational only (a log header a
human reads); no logic branches on it (the retry budget, stall backstop and
`maxIterations` cap are all driven by `orchestrate.Next` + git state, not by
the counter). The TUI log tab renders run.log's raw content
(`job.ReadJDIRunLogTail`, `internal/ui/detail.go:146`) and never parses
attempt numbers, so no TUI change is needed.

Scope decision: the per-agent counter resets at the start of every `mg jdi`
run, matching today's behavior (the loop index restarts at 0). run.log is
append-only across runs, so a later run will show attempt 1 again for each
agent — acceptable: each run is a fresh drive of the job. Not persisting
counters across runs is the conservative reading of the brief; flag to
@reviewer if the brief intended otherwise.

## Task breakdown

TASK-1: Change `Run` in `cmd/mg/jdi.go` to count attempts per agent instead
of using the loop index: introduce a per-agent counter (e.g. a
`map[string]int` keyed by agent name, seeded with 0 before the loop),
increment it for `decision.Agent` once per invocation, and pass that
per-agent count to both `logAgentInvoked` (jdi.go:365) and `logInvocation`
(jdi.go:392) in place of `i+1`. The `invoked` and `finished` headers for the
same invocation must keep sharing the same number (that invariant is what
`TestRunLogsAgentInvoked` documents today, and it must hold with per-agent
counting too). The `i` loop index stays as-is for the `maxIterations` cap.
Update the doc comments on `Run` (jdi.go:311-322), `logAgentInvoked`
(jdioutput.go:174-178) and `logInvocation` (jdioutput.go:237-270) to say the
attempt is per agent, not per run, if they describe it as a run-wide count.
     files: cmd/mg/jdi.go
     depends: none
     risk: low — a localized change in the single function that owns the
       counter; both headers for one invocation move together, so the
       invoked/finished pairing cannot drift.

TASK-2: Update the existing happy-path assertions in `cmd/mg/jdi_test.go`
that hard-code the global counter: `TestRunLogsAgentInvoked` (lines 157-161),
`TestRunFullLogSequenceHappyPath` (lines 493-505) and
`TestRunFullLogSequenceDedupsMatchingOutput` (lines 542-552). Each expects
`developer invoked (attempt 2)` / `developer finished (attempt 2)` /
`reviewer invoked (attempt 3)` / `reviewer finished (attempt 3)` — these
become attempt 1 for developer and attempt 1 for reviewer (analyst is already
attempt 1 in all three). The `assertLogOrder` lists stay otherwise
unchanged.
     files: cmd/mg/jdi_test.go
     depends: TASK-1
     risk: low — three tests, each a mechanical substitution of expected
       substrings; no assertion in `cmd/mg/jdioutput_test.go` is affected
       (its `logAgentInvoked`/`logInvocation` tests pass explicit attempt
       numbers, e.g. `TestLogAgentInvoked` passes 2 and `TestLogInvocation*`
       passes 1).

TASK-3: Add regression coverage for the per-agent semantics on the bounce
path — the case that actually motivated the brief. Extend
`TestRunOneBounceThenApproved` (or add a sibling test next to it) to capture
the log and assert, in order: `developer invoked (attempt 1)` /
`reviewer invoked (attempt 1)` / `developer invoked (attempt 2)` /
`reviewer invoked (attempt 2)` — i.e. the developer's second call is attempt
2, NOT attempt 4 as the old `i+1` counter produced. The fake runner's own
`call` parameter (written into `implementation.md` content as "attempt %d")
is unrelated to the log headers and needs no change — it is only test
content. Use `assertLogOrder` (jdi_test.go:450) for the ordering check.
     files: cmd/mg/jdi_test.go
     depends: TASK-1, TASK-2
     risk: low — a new/expanded test asserting only the log headers; the
       existing bounce call-sequence assertion
       (`{"developer","reviewer","developer","reviewer"}`) is untouched.

TASK-4: Update the README's log-tab description (README.md:613-616) — "one
section per agent invocation, with a timestamp/agent/attempt header" — to note
that the attempt number counts that agent's invocations within the run (per
agent, not per run). Verify end to end: `go build ./...` and `go test ./...`
from the module root (`/workspace`, where `go.mod` lives), and `go vet
./...`.
     files: README.md
     depends: TASK-1
     risk: low — a documentation sentence plus a full test run; the only
       plausible surprise would be an unseen test asserting the old global
       numbering, which the greps in TASK-2/TASK-3 already rule out.
