# Tasks: Launch agents without workflow

id: 8g06st
status: open
analyst: @analyst
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Problem as understood

Two independent asks in the brief:

1. **Agent launch is gated by workflow stage.** In the job detail view, the
   action bar only shows — and `agentForKey` only accepts — the agents
   belonging to the job's *current* `job.Stage()` (analyze →
   product-owner/analyst, develop → developer, review → reviewer/security).
   There is no way to fire e.g. `@developer` on a job that's still in
   "analyze", even though a user may have written `brief.md` and `tasks.md`
   by hand and just wants a developer pass.
2. **There is no "mark done" action in the TUI at all.** `scripts/finish-job.sh`
   (installed as `sc-done`) already does the real work — merges the job
   branch (squash) into the default branch, archives the job directory, sets
   `status: done` — and per commit d40b674 it no longer hard-blocks on an
   unapproved verdict, only warns and asks to continue. But the TUI has never
   called it; `resolve.Done()`'s own doc comment says as much ("The TUI has no
   finish-job action yet").

## Open questions before implementation starts

- **Q1 — where does "mark done" live?** The brief just says "from the TUI".
  This breakdown scopes it to the job **detail** view (TASK-6/7/8), matching
  where every other job-specific action already lives (agent launch, `e`
  edit). A list-view shortcut is not covered — flag if that's also wanted.
- **Q2 — `finish-job.sh`'s exit code is not a reliable success signal.** Every
  one of its `read -rp` confirmation prompts does `exit 0` on decline, not
  just the happy path — so a `0` exit means either "fully finished" or "user
  answered N somewhere along the way". This breakdown (TASK-7) works around
  it by always re-reading the job list from disk after the run instead of
  trying to interpret the exit code as success/failure, but flag if a more
  explicit signal is actually required.
- **Q3 — does the per-stage "recommended agents" concept survive?** The brief
  asks to stop *forcing* the stage's agents, not to remove stage-awareness
  outright. This breakdown (TASK-2/3) assumes the `stage: <name>` label stays
  as an informational hint while the gating itself is dropped, and leaves the
  fate of the now-possibly-unused `Stage.Agents()` as an explicit decision for
  the developer/reviewer rather than guessing.

## Task breakdown

### Feature A — launch any agent regardless of stage

TASK-1: Stop gating which agent keys fire in the job detail view by
`job.Stage()`; accept a key press for any of the five defined agents
(product-owner, analyst, developer, reviewer, security) regardless of the
job's current stage.
     files: tui/internal/ui/app.go (agentForKey)
     depends: none
     risk: low — an isolated function with no dedicated existing test
     coverage; removing the gate is the point of the brief, not a side effect.

TASK-2: Update the detail view's action bar (`renderActionBar`) to always
render all five agent buttons, in a fixed order, instead of only the current
stage's agents. Keep the existing `stage: <name>` label as an
informational-only hint (per Q3) — it no longer restricts anything.
     files: tui/internal/ui/detail.go, tui/internal/ui/agents.go (agentMeta is
     an unordered map today; needs a fixed display-order list now that
     Stage.Agents() no longer supplies the order)
     depends: TASK-1 (same behavioral change; land together)
     risk: low — pure rendering change, no state touched.

TASK-3: Decide the fate of `job.Stage().Agents()` now that no caller gates on
it (per Q3): keep `Stage()` itself (still needed for the label), but decide
whether `Stage.Agents()` and its dedicated test (`TestStageAgents` in
tui/internal/job/stage_test.go) are dead code to remove, or are kept
deliberately (e.g. for a future "highlight the recommended agent" treatment
in the action bar) — do not leave it silently orphaned without a decision
either way.
     files: tui/internal/job/stage.go, tui/internal/job/stage_test.go
     depends: TASK-2
     risk: low — a design/cleanliness judgment call, not a functional change;
     called out explicitly per "when scope is unclear, ask, don't guess."

TASK-4: Add/adjust tests confirming any of the five agent keys fires
regardless of `job.Stage()`, and that the action bar always lists all five
agents.
     files: tui/internal/ui/detail_test.go and/or a new
     tui/internal/ui/agents_test.go
     depends: TASK-1, TASK-2
     risk: low — test-only.

### Feature B — set a job to done from the TUI

TASK-5: Add a host-command wrapper that resolves and builds the `sc-done`
(scripts/finish-job.sh) invocation for a given job, following the same
`resolve.Done()` + cwd/`$PWD`-env pattern `hostcmd.NewJob` already uses for
`sc-job`. Pass the job by its directory name (not an ID prefix) for an exact,
unambiguous match.
     files: tui/internal/hostcmd/hostcmd.go
     depends: none
     risk: medium — must reproduce the cwd/$PWD handling exactly (finish-job.sh
     locates its project root the same way new-job.sh does); a mistake here
     silently breaks project-root detection rather than failing loudly.

TASK-6: Wire a new detail-view key (proposed: capital `D`, currently unused —
verify against the existing key map in tui/internal/ui/app.go and detail.go
before picking it) that runs the TASK-5 command in the **foreground** via
`tea.ExecProcess`, the same suspend-and-resume mechanism the existing `e` edit
shortcut uses. This is necessary — not optional — because `finish-job.sh`'s
several `read -rp` confirmations need a real interactive terminal, unlike
`launch.Agent`'s detached new-window spawn used for agents.
     files: tui/internal/ui/app.go, tui/internal/ui/detail.go
     depends: TASK-5
     risk: medium — this triggers a real, largely irreversible git flow
     (squash-merge into the default branch, branch delete, directory
     move+commit) initiated from inside the TUI; a wiring mistake (wrong job,
     wrong cwd) is a real repository mutation, not just a UI bug.

TASK-7: On return from the foreground finish-job run, always fall back to
"refresh the job list from disk and go back to the list view" (mirroring the
existing esc/backspace path) rather than trying to interpret the exit code as
success/failure — per Q2, a `0` exit does not reliably mean the job was
archived, but re-reading from disk shows the true state either way (job gone
= archived, job still present = declined or failed). A non-zero exit should
still surface through the existing `cmdErrorText` path before falling back.
     files: tui/internal/ui/app.go
     depends: TASK-6
     risk: low — reuses existing `refreshJobs` plumbing; the only risk is
     getting the Q2 exit-code interpretation right.

TASK-8: Update the detail view's footer hint and action-bar area to show the
new "mark done" key, visually separate from the five agent buttons since it
is not an agent action.
     files: tui/internal/ui/detail.go
     depends: TASK-6
     risk: low — user-facing text/layout only.

TASK-9: Add tests for the new done flow: `hostcmd`'s command construction
(cwd/env, and that a resolution failure surfaces `resolve.NotFoundError` the
same way `NewJob`'s does), and the `App`-level key handling for both a clean
and a non-zero return (mirroring `editordone_test.go`'s
`TestEditorDoneMsgSuccess`/`Error` pattern). Use the `SAFECODE_DONE_BIN` env
override so no test invokes the real `finish-job.sh` against a real git repo.
     files: tui/internal/hostcmd/hostcmd_test.go, a new
     tui/internal/ui/donemsg_test.go (or added to an existing app-level test
     file)
     depends: TASK-5, TASK-6, TASK-7
     risk: low — but important: a stub binary via the env override is the
     only safe way to test this without mutating a real repository.

## Explicitly out of scope

- Changing `finish-job.sh`'s own confirmation/exit-code behavior (Q2 notes
  the limitation but does not propose fixing it — that would be a separate
  brief).
- A "mark done" shortcut on the job **list** view (Q1) — scoped to the
  already-open detail view only here.
- Any change to *how* an agent session is launched once triggered
  (`launch.Agent`'s detached-terminal spawning) — Feature A only changes
  *which* keys are allowed to fire, not the launch mechanism itself.
- Any change to the four job files' stage-inference logic (`FileIsWritten`) —
  untouched by either feature; only its *use* as a gate is removed.

## Suggested order

Feature A (TASK-1 → 4) and Feature B (TASK-5 → 9) are independent and can be
done in either order, or in parallel; within each, follow the numbering.
