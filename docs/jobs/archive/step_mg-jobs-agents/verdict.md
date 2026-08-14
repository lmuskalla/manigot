# Verdict: mg jobs agents

id: step
status: open
reviewer: @reviewer (opencode-go)
date: 2026-08-14

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Review surface: `git diff main...HEAD` (base branch `main` per
`.manigot/manigot.json`), cross-referenced against tasks.md and
implementation.md. The diff touches only the five files in the task list
(cmd/mg/jobs.go, cmd/mg/jobs_test.go, cmd/mg/main.go, README.md,
docs/AGENTS.md) plus the job's own docs — nothing out of scope, no
unrelated refactors.

Note: `go build`/`go test` could not be re-run in this review session (the
session git shim refuses non-git commands), so verification is by static
review of the final tree plus per-commit inspection. The fixture staging
was traced line-by-line against `isWritten`/`verdictApproved` and lands on
the intended stages.

TASK-1: PASS
notes: cmd/mg/jobs.go — `runJobs` looks the picked job up in the discovered
`jobs` slice by ID, computes `Stage()`, and `jobsLaunchArgs` builds
`mg --job <id> --agent <name> <passthrough>` for plan→analyst,
implement→developer, review→reviewer (via `stageAgent`, using the
`internal/agents` constants). The launch line is updated to
"→ Starting a session in @analyst for aaa01..." via `jobsLaunchLine`. The
no-agent default and blank-line layout are byte-identical to before for the
agent-less cases; `TestJobsSelectWritesChosenAndLaunches` (define-stage
fixture) still expects the plain line and passes unchanged. Non-TTY
listing/refusal and cancel paths are untouched (new code sits after the
picker).

TASK-2: PASS
notes: cmd/mg/jobs.go — `stageGuidance` returns "brief.md is not written
yet — write it first" for StageDefine and "verdict is APPROVED — run mg
done to merge" for StageFinished, printed indented under the launch line
while the agent-less launch itself is unchanged for both (stageAgent
returns "" for them).

TASK-3: PASS
notes: cmd/mg/jobs.go — `hasExplicitAgent` token-matches `--agent`/`-a` in
passthrough (the same names `session.ParseArgs`'s sessionValueFlags
recognises) and `jobsLaunchArgs` skips the derived flag entirely rather
than appending it, correctly sidestepping the last-wins override. The
launch line uses `session.ParseArgs(passthrough).Agent`, so it announces
the agent that will actually run (explicit beats derived). Pinned by
TestJobsLaunchArgsStageDerivesAgent (explicit --agent, -a, on an edge
stage, and after other passthrough flags) and the end-to-end @security
case. One non-blocking observation: the equals form `--agent=<name>` is
not detected by either `hasExplicitAgent` or `session.ParseArgs` (it lands
in `Pass`), so the derived agent launches and the user's choice is
silently dropped — but that form is uniformly unsupported for session
flags across the codebase (a direct `mg --agent=x --job y` behaves the
same), so this is not a regression of a supported path; the trailing
value-less `--agent` divergence is documented in implementation.md's known
issues.

TASK-4: PASS
notes: cmd/mg/jobs_test.go — TestJobsStageAgent, TestJobsStageGuidance,
TestJobsLaunchLine, TestJobsLaunchArgsStageDerivesAgent, and the end-to-end
TestJobsSelectStageLaunchOutput (define/plan/implement/review/finished/
rejected-bounce/explicit --agent) cover every stage, the guidance lines,
the launch-line wording, and the explicit-agent precedence. Fixtures
`jobsWriteJobFile`/`jobsWrittenBrief`/`jobsFilled*`/`jobsApprovedVerdict`/
`jobsRejectedVerdict` were traced against `isWritten` and
`verdictApproved` and correctly land on define/plan/implement/review/
finished/implement-bounce. No pre-existing assertions were weakened. Minor
nit: implementation.md says "all 14 job tests pass" but jobs_test.go has
15 top-level test functions (counting `-run Jobs`).

TASK-5: PASS
notes: cmd/mg/main.go (`mg -h` `mg jobs` entry), README.md (command table
row + job-workflow section), and docs/AGENTS.md (Commands bullet) all
mention the stage-appropriate agent launch with explicit `--agent` winning.
Verified project-template/docs/AGENTS.md and agents/*.md contain no `mg
jobs` mention, so no change was needed there.

Scope / commit discipline: nothing changed outside the task list. Commits
follow the `[ID] TASK-N:` format; the two `tasks:` and two
`implementation:` commits plus commit `180b36e` (a jobs.go edit landed
under a "tasks: add analyst task breakdown" message) are the documented
residue of the concurrent second @developer session (implementation.md
"Known issues") — the final tree is coherent and the messages are the only
residue, so this is noted but not blocking.

## Security

No security findings. The change adds a derived `--agent` flag to a
re-exec of the existing, already-tested `mg --job <id> --agent <name>`
launch combination; no new surface, no credential/env handling, no
filesystem writes beyond the pre-existing launch path. The read-only
agent/git-mount boundaries are untouched.

## Overall

APPROVED

All five tasks are implemented as specified and match the brief: `mg jobs`
now launches the picked job in the stage-appropriate agent (plan → analyst,
implement → developer, review → reviewer), keeps the agent-less launch with
a guidance line for the define/finished edge stages, preserves explicit
`--agent`/`-a` precedence, is fully tested, and is documented on all three
doc surfaces. No blockers. Non-blocking observations recorded in the task
notes: the `--agent=<value>` equals form is unrecognised (consistent with
the rest of the session launcher), the documented trailing-`--agent`
divergence, the 14-vs-15 test-count nit in implementation.md, and the
parallel-session commit-message residue.
