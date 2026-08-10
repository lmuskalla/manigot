# Tasks: jdi does not work

id: 4i5tcx
status: open
analyst: @analyst
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Root cause (working hypothesis)

`mg jdi`'s loop (`tui/cmd/jdi/main.go`'s `Run`) asks
`tui/internal/orchestrate.Next` what to do first, before ever invoking an
agent. `Next` derives its decision from `job.Job.Stage()`
(`tui/internal/job/stage.go`), which in turn calls `fileWritten("brief.md")`
→ `isWritten`. `isWritten`'s rule requires **at least two** substantive lines
(≥8 chars, not a heading/frontmatter/`TASK-` line) *or* one `TASK-` marker
before it will call a file "written".

A real but terse brief — one filled section (typically "## What") with
"## Why" / "## Out of scope" / "## Notes" left as the scaffold's own
HTML-comment placeholders, exactly this job's own `brief.md` as scaffolded by
`mg job` — only ever produces **one** substantive line. `isWritten` therefore
returns `false`, `Stage()` reports `StageDefine`, and `orchestrate.Next`
immediately returns `StopNeedsHuman` with reason "brief.md is not written
yet" — on the very first loop iteration, before `AgentRunner.Run` is ever
called.

This matches every symptom in the brief:
- **Immediate**: no agent invocation, no container, no LLM call happens at
  all — consistent with "the LLM wasn't even fast enough".
- **`run.log` is empty**: `tui/cmd/jdi/output.go`'s `logInvocation` is only
  called once an agent has actually run; an immediate `Next()` stop never
  reaches it, so the just-opened `run.log` stays at 0 bytes.
- **`status` says `stopped:needs-human`**: `Run`'s `finish` helper calls
  `WriteJDIStatus` unconditionally on any stop, including this
  before-the-first-agent one.

This is a plausible, concretely reproducible-from-static-analysis bug (see
"Open questions" below for its one real caveat), not a docker/auth/network
issue — a genuinely-blank scaffold `brief.md` should still correctly stop
`mg jdi` immediately; the bug is that a real-but-short brief is
misclassified the same way.

## Open questions before implementation starts

This diagnosis comes from reading `isWritten`/`Stage`/`orchestrate.Next`
against the scaffold format and this job's own `brief.md`, not from an actual
`mg jdi` run (no Docker/Claude credentials available in this session) — the
brief doesn't quote the exact reason text `mg jdi` printed, or name which job
it was run against. TASK-5 below asks for a live confirmation before this is
considered closed; if a real run instead shows a *different* immediate-stop
reason (e.g. an auth/docker failure surfacing as a fast `runErr`, which
would populate `run.log` rather than leave it empty — see the brief's own
description), this diagnosis needs revisiting rather than being assumed.

## Task breakdown

TASK-1: Add a regression test to `tui/internal/job/stage_test.go` that
reproduces the bug: a brief with only one filled section (mirroring real
usage — and specifically shaped like this job's own `brief.md`) must be
classified as written by `FileIsWritten`, and a job built from it must report
`StagePlan`, not `StageDefine`. This test should fail against the current
code, confirming the diagnosis before any fix lands.
files: tui/internal/job/stage_test.go
depends: none
risk: low — test-only change, no behavior modified.

TASK-2: Fix `isWritten` in `tui/internal/job/stage.go` so a single genuine
substantive line is sufficient to count a file as "written" (e.g. drop the
"≥2 lines" requirement to "≥1"), while continuing to classify all four of
`new-job.sh`'s untouched scaffold templates (which have zero substantive
lines) as unwritten. Update the function's doc comment to describe and
justify the corrected rule.
files: tui/internal/job/stage.go
depends: TASK-1
risk: medium — this heuristic gates `Stage()` everywhere (the TUI's stage
timeline label and `mg jdi`'s orchestration), so an over-loose fix risks new
false positives; `TestScaffoldTemplatesAreNotWritten` and the rest of
`stage_test.go` must stay green.

TASK-3: Confirm the fix resolves the symptom at the orchestration layer:
add/extend a test asserting that, given a job whose `brief.md` has real-but-
terse content (one filled section, others left as scaffold placeholders) and
no `tasks.md` yet, `orchestrate.Next` returns `RunAgent("analyst")`, not
`StopNeedsHuman`.
files: tui/internal/orchestrate/orchestrate_test.go
depends: TASK-2
risk: low — additive test coverage only.

TASK-4: Make an immediate `mg jdi` stop (`Next()` returning a `Stop*` kind
before any agent has ever run — e.g. a *genuinely* unwritten brief.md, which
correctly should still stop immediately even after TASK-2) also write its
`Reason` to `run.log`, not just to `mg jdi`'s own terminal stdout and the
status sidecar's `Agent` field (which stays empty in this case). Today that
case leaves `run.log` at 0 bytes, which is itself confusing — matching part
of the brief's complaint — independent of whether the stop was correct.
files: tui/cmd/jdi/main.go, tui/cmd/jdi/main_test.go
depends: none (independent of TASK-1–3; can proceed in parallel)
risk: low — additive logging only; does not change any control-flow/decision
logic.

TASK-5: Verification pass: run `go test ./tui/...` for regressions, and — if
a Docker/Claude-credentialed environment is available — a real `mg jdi --job
<id>` run against a job whose brief.md matches the shape described above,
confirming it now proceeds to invoke `@analyst` instead of stopping
immediately. If the live run instead surfaces a different failure than
diagnosed above, stop and report back rather than forcing this task list.
files: none (verification only)
depends: TASK-2, TASK-3, TASK-4
risk: low — verification only; the live end-to-end run needs Docker + Claude
credentials, which may not be available in every environment — note as a
limitation rather than a blocker on the code changes themselves.

## Explicitly not covered by this breakdown

- `scripts/entrypoint.sh`, `scripts/run.sh`'s `--print` handling, or the
  `claude --print --agent ... --output-format json` invocation itself — the
  diagnosis places the bug entirely in host-side Go stage-classification
  logic, not in the container/CLI invocation path.
- `orchestrate.Next`'s retry-budget (`verdictRounds`) logic or the stall
  backstop in `Run` — neither is implicated by this diagnosis; both only run
  after at least one agent invocation has already happened.
- Any change to the scaffold templates in `scripts/new-job.sh` — the fix
  target is how *existing* scaffold/real content is classified, not the
  scaffold text itself.
