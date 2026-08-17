# Tasks: review cycle message

id: cold
status: open
analyst: @analyst
date: 2026-08-17

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Change the deliberate review-cycle stop reason in
`orchestrate.Next`'s `verdictRounds >= 2` branch (the "job came back twice
from the reviewer / one-bounce retry budget exhausted" case) so the "needs
human" message it produces says the stop is the review cycle, not a crash or
anything else. Today the branch returns
`Decision{Kind: StopNeedsHuman, Reason: "retry budget exhausted: 2 verdict
commits already, still not approved"}` (internal/orchestrate/orchestrate.go,
`case verdictRounds >= 2`). The new Reason must lead with the brief's
requested phrase — e.g. `"needs human: review cycle — retry budget
exhausted: 2 verdict commits already, still not approved"` — while keeping
the explanatory context (2 verdict commits / retry budget). This Reason is
what the human sees as the message: `mg jdi`'s final stdout line
(`mg jdi: stop-needs-human — <reason>` in cmd/mg/jdi.go), the run.log's
`mg jdi finished: stop-needs-human` reason line (logJobFinished), and the
ntfy needs-attention notification body (notifyStop). No other stop path
(marker, runner error, stall backstop, maxIterations, brief-not-written,
unknown stage) changes — the brief only asks to tag the deliberate
developer/reviewer back-and-forth stop, and crashes keep their own distinct
signal (a stale `running` sidecar).
files: internal/orchestrate/orchestrate.go
depends: none
risk: low — a string-only change to a pure function's Reason; every existing
test asserts Kind (and reason non-empty), never the exact reason text, so
nothing else in the codebase can break.

TASK-2: Pin the new message in tests so the wording can't silently regress:
extend `internal/orchestrate/orchestrate_test.go`'s TestNext cases for
`verdictRounds >= 2` (the "2 verdicts, tip is the verdict", "2 verdicts,
developer responded again", and "3+ verdicts" subtests) to also assert the
Reason contains "review cycle", and extend `cmd/mg/jdi_test.go`'s
TestRunStopsAfterOneBounceExhausted (which drives the full loop into this
exact stop) to assert `got.Reason` contains "review cycle" — proving the
message survives end to end from Next through Run's LoopResult.
files: internal/orchestrate/orchestrate_test.go, cmd/mg/jdi_test.go
depends: TASK-1
risk: low — additive test assertions only, mirroring the existing
non-empty-Reason checks already in TestNext.

TASK-3: Verification pass: confirm the repo still builds and all tests pass
(`go build ./...` and `go test ./...`, or at minimum
`go test ./internal/orchestrate/... ./cmd/mg/... ./internal/job/...`), and
confirm no living (non-archive) code or docs still reference the old reason
string "retry budget exhausted" outside orchestrate.go's own now-updated
comment/string. No docker/credentialed end-to-end run is required — the
change is fully covered by the unit tests.
files: none (verification only)
depends: TASK-1, TASK-2
risk: low — verification only; no new runtime dependency.

## Explicitly not covered by this breakdown

- No new jdi sidecar state or status field (e.g. no
  `stopped:needs-human-review-cycle` state and no Reason field added to the
  status.json schema) — the brief asks for a message, and the sidecar's
  state/agent/updated shape is consumed by the TUI badges and the
  ntfy crash-dedup logic, so extending it would be scope creep. The message
  lives where humans already read it: the final stdout line, run.log, and
  the ntfy notification body.
- No TUI badge text change: the `[needs human]` badge is deliberately
  generic (derived from the sidecar state); the run.log tab already shows
  the reason line under `mg jdi finished: stop-needs-human`.
- No change to the other StopNeedsHuman paths (NEEDS-HUMAN-INPUT marker,
  runner error, stall backstop, maxIterations, brief-not-written, unknown
  stage) — each already has its own distinct reason, and the brief targets
  only the deliberate review-cycle stop.