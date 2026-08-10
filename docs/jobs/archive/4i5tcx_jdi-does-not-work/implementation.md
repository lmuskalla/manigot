## Summary

Confirmed and fixed the root cause diagnosed in `tasks.md`: `isWritten`
(`tui/internal/job/stage.go`) required at least two substantive lines before
classifying a job file as "written". A real-but-terse `brief.md` — one
filled section (typically "## What"), with every other scaffold section left
as its HTML-comment placeholder, exactly how `mg job` scaffolds a brief and
exactly how this job's own `brief.md` looked — only ever produces one
substantive line, so it was misclassified as unwritten. That made
`job.Stage()` report `StageDefine` and `orchestrate.Next` return
`StopNeedsHuman` ("brief.md is not written yet") on the very first loop
iteration, before any agent was ever invoked — matching every symptom in the
brief (immediate stop, empty `run.log`, `stopped:needs-human`).

Also fixed a related, previously-invisible confusion: even a genuinely
correct immediate stop (e.g. a truly blank `brief.md`) left `run.log` at 0
bytes, since `logInvocation` is only ever called once an agent has actually
run. That's now logged too, independent of whether the stop itself was the
right call.

## Changes

TASK-1: Added `tui/internal/job/stage_test.go` regression tests
(`TestTerseRealBriefIsWritten`, `TestTerseRealBriefJobStageIsPlan`) using a
brief shaped exactly like this job's own `brief.md` — one filled "## What"
section, the rest left as scaffold placeholders. Confirmed these failed
against the pre-fix code (`FileIsWritten` returned false, `Stage()` reported
`StageDefine` instead of `StagePlan`), pinning down the diagnosis before any
fix landed.

TASK-2: Fixed `isWritten` in `tui/internal/job/stage.go`: dropped the "≥2
substantive lines" requirement to "≥1". This alone caused a regression in
`TestScaffoldTemplatesAreNotWritten` — `tasks.md`/`implementation.md`/
`verdict.md`'s scaffold templates each have one never-recognized,
empty-valued attribution line (`analyst:`, `developer:`, `reviewer:` — not in
`frontmatterKeys`, but exactly 8+ chars long), which the old ≥2 rule
tolerated as a false-but-harmless single count but the new ≥1 rule alone
would wrongly promote to "written". Generalized the frontmatter-skip check
so any bare `key:` line with an empty value (not just the fixed
`frontmatterKeys` allowlist) is treated as a non-substantive placeholder,
regardless of key — a `key:` field only starts counting once it's actually
filled in with a value. Updated the function's doc comment to describe and
justify both the ≥1 threshold and the empty-value-key exemption.
`TestScaffoldTemplatesAreNotWritten` and the rest of `stage_test.go` (plus
every other `tui/...` package) stay green.

TASK-3: Added `TestNextRealButTerseBriefRunsAnalyst` to
`tui/internal/orchestrate/orchestrate_test.go`: builds a real `job.Job` from
a terse-but-real `brief.md` (same shape as TASK-1's), computes its
`Stage()`, and asserts `orchestrate.Next` returns `RunAgent("analyst")` —
confirming the fix holds at the orchestration layer `mg jdi` actually calls,
not just in `job.FileIsWritten` directly.

TASK-4: `tui/cmd/jdi/main.go`'s `Run` now tracks whether any agent has been
invoked yet (`agentEverRan`); an immediate `Stop*` decision before that ever
happens now calls a new `logImmediateStop` helper (`tui/cmd/jdi/output.go`)
that writes a timestamped header plus the stop `Reason` to `log` (the same
fan-out target `logInvocation` writes to: `mg jdi`'s stdout + the sidecar's
`run.log`). Added `TestRunLogsImmediateStopReason` to
`tui/cmd/jdi/main_test.go` asserting `run.log` is non-empty and contains the
stop reason for this case, and that no agent was invoked.

TASK-5: Verification pass. `go test ./...` (run from `tui/`, the module
root) passes cleanly across every package, including all of the above new
tests. No Docker/Claude-credentialed environment is available in this
sandbox, so the live end-to-end `mg jdi --job <id>` confirmation described in
`tasks.md` could not be performed here — noted as a limitation per the
task's own allowance, not a blocker on the code changes themselves.

## Known issues / follow-ups

- The live end-to-end `mg jdi` run against a real job (TASK-5's optional
  second half) was not performed — no Docker/Claude credentials available in
  this environment. The unit/integration-level fix and its regression tests
  are otherwise complete and green.
