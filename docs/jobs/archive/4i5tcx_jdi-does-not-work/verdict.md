# Verdict: jdi does not work

id: 4i5tcx
status: open
reviewer: @reviewer
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `tui/internal/job/stage_test.go` adds `terseRealBrief` (shaped exactly
after this job's own `brief.md`) plus `TestTerseRealBriefIsWritten` and
`TestTerseRealBriefJobStageIsPlan`. Verified these fail against the pre-fix
`isWritten` (≥2-line rule) by inspection of the diff and confirmed they pass
post-fix (`go test ./...` from `tui/`, all green).

TASK-2: PASS
notes: `tui/internal/job/stage.go` — threshold dropped from `substantive >= 2`
to `substantive >= 1`. To avoid the false-positive this alone would introduce
(scaffold `analyst:`/`developer:`/`reviewer:` attribution lines are exactly 8
chars and were previously tolerated only because 1 alone wasn't enough), the
frontmatter skip was generalized from a fixed-key allowlist to "any bare
`key:` line with an empty value", which correctly keeps `TestScaffoldTemplatesAreNotWritten`
green while accepting genuine one-line content. Doc comment updated to
explain and justify both changes. Verified by reading `stage.go` line-by-line
against all four `new-job.sh` scaffold templates in the test file, and by
`go test ./tui/internal/job/...` (green).

TASK-3: PASS
notes: `tui/internal/orchestrate/orchestrate_test.go`'s
`TestNextRealButTerseBriefRunsAnalyst` builds a real `job.Job` from the same
terse-but-real brief shape, asserts `Stage()` is `StagePlan`, and asserts
`Next(stage, 0, false)` returns `{RunAgent, "analyst"}` — confirming the fix
holds at the exact layer `mg jdi`'s `Run` loop calls, not just in
`FileIsWritten` directly. Matches `Next`'s actual `StagePlan` branch in
`orchestrate.go`.

TASK-4: PASS
notes: `tui/cmd/jdi/main.go`'s `Run` now tracks `agentEverRan`, gating a new
`logImmediateStop(log, decision.Reason)` call (added in `output.go`) so it
fires only on a `Stop*` decision reached before any agent invocation — the
`return finish(...)` right after it means this can only trigger once, and
only for the pre-first-agent case, not for stops after agents have already
run (which already get logged via `logInvocation`). `TestRunLogsImmediateStopReason`
in `main_test.go` exercises this against a genuinely-unwritten `brief.md`
scaffold and asserts `run.log` is non-empty and contains the stop reason,
with no agent invoked. Verified logic by reading the loop in full; the
`agentEverRan` flag is set immediately after `runner.Run` is called, before
any of the loop's other early-return paths, so it can't leak false.

TASK-5: PASS
notes: `go test ./...` run from `tui/` (module root) by this reviewer:
all packages green, including every new test above (`go build ./...` and
`gofmt -l .` also clean). The live end-to-end `mg jdi --job <id>` run was
correctly not attempted — no Docker/Claude credentials in this sandbox
either — and `tasks.md` explicitly allows this as a noted limitation rather
than a blocker.

## Security

None — pure Go logic and test changes to host-side stage classification and
logging; no new I/O, network, secrets, or shell-out behavior introduced.

## Overall

APPROVED

Diagnosis, fix, and regression coverage all check out: `isWritten`'s ≥2-line
threshold is confirmed as the root cause of `mg jdi` stopping immediately on
a real-but-terse `brief.md`, the fix (≥1 line + generalized empty-key-value
skip) closes the gap without reopening the scaffold-false-positive case, and
the fix is verified at both the `job` and `orchestrate` layers plus the new
`run.log` immediate-stop logging. `go build`/`go vet`/`go test ./...` (from
`tui/`) and `gofmt -l .` are all clean. Every task has its own commit in the
correct `[4i5tcx] TASK-N: ...` format, plus separate `implementation:` and
`tasks:` commits; no changes outside the scope of `tasks.md`. Only
limitation is the unavoidable lack of a live Docker/Claude-credentialed
`mg jdi` run in this environment, which both `tasks.md` and
`implementation.md` correctly flag as a known, allowed limitation rather
than glossing over it.
