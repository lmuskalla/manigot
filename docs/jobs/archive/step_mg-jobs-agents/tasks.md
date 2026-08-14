# Tasks: mg jobs agents

id: step
status: open
analyst: @analyst
date: 2026-08-14

<!-- Produced by @analyst from brief.md. -->

## Problem as understood

`mg jobs` on a TTY lists open jobs and lets the user pick one, then re-execs
`mg --job <id> <passthrough>` — a session with **no agent**, prompting with
the job's `brief.md` (cmd/mg/jobs.go). The brief wants that launch to be
stage-aware: check the job's current workflow stage and start the session in
the appropriate agent (e.g. analyst, developer or reviewer) instead of a bare
session.

`job.Job.Stage()` already derives the stage from the job's files
(internal/job/stage.go): define → plan → implement → review → finished, with
a written-but-not-approved verdict bouncing back to implement. The
stage→agent mapping already exists in spirit in `orchestrate.Next`
(internal/orchestrate/orchestrate.go), but it is mg-jdi-specific (depends on
verdict-commit history via `git.CountVerdictCommits` /
`git.LatestCommitIsVerdict` and returns stop kinds, not just agents), so it is
not directly reusable for a single interactive launch.

`mg --agent <name> --job <id>` is already a supported, tested launch
combination (the TUI launches exactly this via `launch.Agent`), so the change
is confined to `cmd/mg/jobs.go`'s launch-argument construction — the session
launcher needs no changes.

## Scope boundary (important)

The previous job `8g06st_launch-agents-without-workflow` deliberately removed
the stage→agent **gate** (`job.Stage().Agents()`) so the TUI always allows all
five agents from any stage, and `internal/job/stage.go`'s doc comment now
explicitly says: *"do not reintroduce one as a gate."* This brief is **not** a
reversal of that: it only asks the CLI `mg jobs` selection flow to launch in
the *appropriate* agent. Do not re-gate the TUI's action bar, do not re-add a
gating `Stage.Agents()` method, and do not restrict which agents a user can
explicitly request via `mg jobs --agent <name>` / passthrough.

## Open questions before implementation starts

- **Q1 — the define/finished edge stages have no fitting agent.** StageDefine
  means brief.md isn't written yet (writing it is a human task, per
  `orchestrate.Next`'s own reasoning); StageFinished means the verdict is
  APPROVED and the job is ready for `mg done`. Proposal (TASK-2): keep the
  current agent-less launch for both, but print a short heads-up line naming
  the situation (e.g. "brief.md is not written yet" / "verdict is APPROVED —
  merge with mg done"). Flag if either stage should instead refuse to launch.
- **Q2 — where does the stage→agent mapping live?** Not in `stage.go` (the
  doc comment forbids reintroducing an `Agents()`-style method). Proposal:
  a small unexported helper in `cmd/mg/jobs.go` (e.g.
  `stageAgent(stage job.Stage) string`), keeping the change local to the CLI.
  Flag if a shared, exported, non-gating "recommended agent" helper in the
  `job` package is preferred instead.
- **Q3 — StageImplement nuance.** A REJECTED/NEEDS WORK verdict bounces the
  stage back to implement; strictly, after the developer has committed a fix
  since the rejection, the appropriate next agent is the reviewer (re-review),
  not the developer (orchestrate distinguishes these via verdict-commit
  history). Proposal: keep it simple for v1 — StageImplement always suggests
  `developer` — and note the re-review refinement as out of scope. Flag if the
  orchestrate-style refinement is wanted now.

## Task breakdown

TASK-1: In `runJobs`' TTY selection path, look up the picked job in the
discovered `jobs` slice, compute its `Stage()`, and include `--agent <name>`
in the re-exec launch args (`mg --job <id> --agent <name> <passthrough>`) for
the stages that have a fitting agent: plan → analyst, implement → developer,
review → reviewer. Update the "→ Starting a session in ..." launch line to
name the agent (e.g. "→ Starting a session in @analyst for <id>...").
     files: cmd/mg/jobs.go
     depends: none
     risk: medium — touches the launch path whose wording is pinned by
     cmd/mg/jobs_test.go (TestJobsSelectWritesChosenAndLaunches); must not
     break the passthrough order or the no-agent default for non-mapped
     stages.

TASK-2: Handle the two edge stages explicitly: StageDefine (brief.md not
written) and StageFinished (verdict APPROVED) launch without an agent but
print a short guidance line naming the situation (per Q1 — e.g. "brief.md is
not written yet — write it first" / "verdict is APPROVED — run mg done to
merge"). Keep the agent-less session launch itself unchanged for both.
     files: cmd/mg/jobs.go
     depends: TASK-1
     risk: low — output-only addition; behavior (session still launches) is
     unchanged for these stages.

TASK-3: Preserve explicit-user-agent precedence: when the caller already
passed `--agent`/`-a` in passthrough, do not derive/override it with the
stage agent — the user's explicit choice wins (the derived agent only fills
the default). Verify against session.ParseArgs's last-wins flag semantics
("--job <id> --agent <stage> --agent <user>" would silently override; prefer
skipping the derived flag entirely when an explicit one is present).
     files: cmd/mg/jobs.go
     depends: TASK-1
     risk: low-medium — a precedence bug would silently launch the wrong
     agent; needs an explicit test pinning that an explicit --agent beats the
     stage-derived one.

TASK-4: Add/adjust tests in cmd/mg/jobs_test.go: per-stage launch-arg
assertion (plan→analyst, implement→developer, review→reviewer, define and
finished→no agent + guidance line), explicit passthrough --agent precedence,
and the updated launch-line wording. Reuse the existing jobsCheckout fixtures
(they already exercise job.Discover's working-tree fallback) and extend them
to write tasks.md/implementation.md/verdict.md as needed to land on each
stage — mirror the filled-* fixtures in internal/job/stage_test.go.
     files: cmd/mg/jobs_test.go
     depends: TASK-1, TASK-2, TASK-3
     risk: low — test-only; the fixture staging must produce the exact stage
     via job.Stage()'s isWritten rules (scaffold files count as unwritten).

TASK-5: Doc sync for the changed `mg jobs` behavior: `mg -h` help text
(cmd/mg/main.go), README.md's command table + job-workflow section, and
docs/AGENTS.md's Commands list — all currently describe `mg jobs` as "pick
one to start a session in"; add a brief mention that the session launches in
the agent appropriate to the job's stage (e.g. analyst/developer/reviewer).
Verify project-template/docs/AGENTS.md and agents/*.md need no change.
     files: cmd/mg/main.go, README.md, docs/AGENTS.md
     depends: TASK-1
     risk: low — documentation wording only, but must stay in sync across the
     three surfaces per the repo's doc-sync convention.

## Explicitly out of scope

- Re-gating the TUI's action bar or reintroducing a gating
  `job.Stage().Agents()` method (explicitly forbidden by stage.go's doc
  comment; the "all agents always launchable" TUI behavior from
  8g06st_launch-agents-without-workflow stays).
- Changing the session launcher or agent definitions — `--agent` + `--job`
  already works end to end.
- The StageImplement re-review refinement (Q3) — always developer for v1.
- Adding a stage column to the `mg jobs` listing (not requested by the brief;
  the TUI list already shows it).
- `mg host --job` / bare `mg --job`: direct launches, not a selection flow —
  unchanged.
- `mg jdi` / `orchestrate.Next`: untouched.

## Suggested order

TASK-1 → TASK-2 → TASK-3 (all in cmd/mg/jobs.go; land together to keep the
launch path coherent), then TASK-4 (tests), then TASK-5 (docs). Each commit
must keep `go test ./...` green.
