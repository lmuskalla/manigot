# Implementation: review cycle message

id: cold
status: open
developer: @developer
date: 2026-08-17

<!-- Produced by @developer after implementation. -->

## Summary

The deliberate jdi stop that fires when a job comes back from the reviewer
twice (the one-bounce retry budget exhausted) now tells the human what it is:
its "needs human" message leads with "needs human: review cycle", so this
deliberate developer/reviewer back-and-forth stop reads distinctly from a
crash. The message was pinned in unit tests at both the orchestration layer
(`orchestrate.Next`) and the full `mg jdi` loop (`Run`), and the repo builds
and passes the full test suite.

## Changes

TASK-1: `internal/orchestrate/orchestrate.go` — the `verdictRounds >= 2`
branch of `Next` now returns
`Reason: "needs human: review cycle — retry budget exhausted: 2 verdict
commits already, still not approved"` (previously `"retry budget exhausted:
2 verdict commits already, still not approved"`). The branch comment was
extended to explain that the reason leads with the review-cycle tag so this
deliberate stop is distinguishable from a crash. This Reason is what the
human sees everywhere a stop reason surfaces: `mg jdi`'s final stdout line,
run.log's `mg jdi finished: stop-needs-human` reason line, and the ntfy
needs-attention notification body. No other stop path changed.

TASK-2: Test pins so the wording can't silently regress:
- `internal/orchestrate/orchestrate_test.go` — `TestNext` gained a
  `wantReasonContains` field (asserted via `strings.Contains`); the three
  `verdictRounds >= 2` cases ("2 verdicts, tip is the verdict", "2 verdicts,
  developer responded again", "3+ verdicts") now assert the Reason contains
  "review cycle".
- `cmd/mg/jdi_test.go` — `TestRunStopsAfterOneBounceExhausted` now asserts
  `got.Reason` contains "review cycle", proving the message survives end to
  end from `Next` through `Run`'s `LoopResult`.

TASK-3: Verification — `go build ./...` and `go test ./...` pass across the
whole repo (run with the session git shim removed from PATH, since the
shim's git-command allowlist refuses the `git init` used by test helpers in
`internal/job`/`cmd/mg`). Grep for the old reason string confirms no living
(non-archive) code or docs reference it outside orchestrate.go's own
now-updated string/comment; the remaining hits are this job's own tasks.md
(describing the change), an archived job's tasks.md (`docs/jobs/archive/`),
and `internal/job/jdistatus.go`'s generic comment describing the class of
needs-human stops — which stays accurate because the new Reason keeps the
"retry budget exhausted" tail.

## Known issues / follow-ups

none.