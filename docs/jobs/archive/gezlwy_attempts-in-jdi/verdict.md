# Verdict: attempts in jdi

id: gezlwy
status: open
reviewer: claude
date: 2026-08-12

## Review

TASK-1: PASS
notes: `cmd/mg/jdi.go` — `attempts := make(map[string]int)` seeded before the
loop (line 355), incremented once per invocation for `decision.Agent` (line
378), and passed to both `logAgentInvoked` (line 379) and `logInvocation`
(line 406) in place of `i+1`. The invoked/finished headers share the same
number because `decision.Agent` is loop-local and never mutated between the
two call sites. The `i` loop index still drives the `maxIterations` cap
(line 357). Doc comments on `Run` (jdi.go:317-322), `logAgentInvoked`
(jdioutput.go:178-181) and `logInvocation` (jdioutput.go:246-249) all
updated to describe the per-agent semantics. No other code branches on the
attempt number (grep confirms the only consumers are the two log headers).

TASK-2: PASS
notes: `cmd/mg/jdi_test.go` — `TestRunLogsAgentInvoked`,
`TestRunFullLogSequenceHappyPath` and
`TestRunFullLogSequenceDedupsMatchingOutput` updated from `developer
(attempt 2)` / `reviewer (attempt 3)` to `attempt 1` for both. Analyst was
already attempt 1. `assertLogOrder` lists otherwise unchanged, and
`cmd/mg/jdioutput_test.go` was correctly left alone (it passes explicit
attempt numbers). All four tests pass.

TASK-3: PASS
notes: `TestRunOneBounceThenApproved` extended to capture the log and
assert, in order via `assertLogOrder`: developer invoked/finished (attempt
1) → reviewer invoked/finished (attempt 1) → developer invoked/finished
(attempt 2) → reviewer invoked/finished (attempt 2). This is a superset of
the task's minimum (it also asserts the finished headers, which additionally
pins the invoked/finished same-number invariant on the bounce path). The
existing call-sequence assertion (`{"developer","reviewer","developer",
"reviewer"}`) and the fake runner's `call` parameter are untouched. Test
passes.

TASK-4: PASS
notes: README.md log-tab description (line ~611-617) now states the attempt
number counts that agent's invocations within the run (per agent, not per
run). Verified `go build ./...`, `go vet ./...`, and `go test ./...` from
the module root — all packages pass.

## Security

None — presentational log-counter change only; no input handling, no
credentials, no file-system exposure introduced.

## Overall

APPROVED

All four tasks implemented as specified, with correct per-agent counting
that also preserves the invoked/finished header pairing. Tests updated and
extended, full build/vet/test run green, commit discipline clean (one
`[ID] TASK-N` commit per task, implementation.md its own commit). The
per-run counter reset matches the analyst's documented scope decision; the
implementation.md follow-up note correctly flags the persist-across-runs
question for the human.
