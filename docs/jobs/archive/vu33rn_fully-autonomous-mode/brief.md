# Brief: fully autonomous mode

status: done
type: feature
id: vu33rn
branch: feature/vu33rn_fully-autonomous-mode
date: 2026-08-09
author: Leander Muskalla

## What

A new mode, `mg-jdi` ("just do it"), available via both TUI and CLI: given a
job, it orchestrates a fixed agent sequence — `@analyst` → `@developer` per
task → `@reviewer` (bouncing back to `@developer` once if needed, see (2)) —
end to end without a human manually triggering each stage.

Scope decisions (product-owner + author, 2026-08-09):

1. **Completion criteria** — stops at `verdict.md`'s "## Overall" saying
   APPROVED (i.e. `Stage() == StageFinished`). It does not auto-merge; the
   human still checks out the branch and merges themselves, same as today.
2. **Failure/loop bound** — if `@reviewer` comes back
   REJECTED/NEEDS WORK, autonomous mode sends it back to `@developer`
   exactly **once**. If the re-review still isn't APPROVED after that one
   cycle, it stops and hands control back to the human — no further retries.
3. **"Needs human input" trigger set** — kept deliberately narrow for v1,
   exactly two conditions:
   - the retry budget in (2) is exhausted (one bounce-back didn't fix it)
   - the running agent asks the human a question / indicates it's blocked
     (needs a defined signal to detect this from an agent's output/transcript
     — TBD in tasks.md)
4. **Notification** — a status surfaced in the TUI's job list (derived from
   polling job files / `Stage()`, no new event-streaming subsystem for v1),
   plus a ping/notification sound when autonomous mode stops for any reason
   (finished, or needs human input).
5. **CLI entry point** — `mg-jdi`.
6. **Agent sequence, fixed and uniform** — `@analyst` → `@developer` →
   `@reviewer` for every job, regardless of job `type` (feature/fix/chore).
   `@product-owner` and `@security` are not part of the sequence `mg-jdi`
   drives in v1; starting simple. Both remain available as ordinary manual
   agents via the TUI/CLI, unaffected by this job.
7. **Visibility while running** — a human must be able to see what each
   agent is doing, from both the CLI and the TUI, not just be told when the
   whole run stops (that's (4), which is silent about the run itself). A
   direct CLI run streams each agent's output live to the terminal it was
   started from. A TUI-launched run has no terminal of its own at all —
   `mg-jdi` runs detached in the background there, not in a spawned window
   (there is nothing in it that needs a human, or a subprocess, to interact
   with one) — so the TUI instead gets a persisted, pollable log it reads
   from inside itself. Same polling/no-event-streaming constraint as (4).

## Why

If manigot works good, it'll be so isolated and well-defined that you can just let it run. You can set it on a task, it'll handle everything and when it's done, you have a full feature branch you can check out and probably just merge it.

## Out of scope

- Auto-merging the finished branch — the human still merges manually.
- Retrying more than once per review cycle — after one bounce-back to
  `@developer`, it stops and waits for a human regardless of outcome.
- Any event-streaming subsystem for tracking agent progress — v1 notification
  is polling-based, not a live stream.
- Running agents inside the TUI itself (split pane / embedded terminal) — see
  `docs/backlog.md`, deferred to a future job.
- Headless/cron execution — see `docs/backlog.md`, deferred to a future job;
  the current architecture assumes an attended, spawned terminal, which
  running via cron would not have.
- `@product-owner` and `@security` in the automated sequence — v1 is
  `@analyst → @developer → @reviewer` only, uniformly for every job type.
  Folding either back in is a future extension, not v1.

## Notes

- It needs the agent-sequence orchestration logic (which agent runs next,
  given a job's current `Stage()` and verdict history) — this doesn't exist
  yet; today stage-advancement is entirely a human clicking the next agent
  button in the TUI.
- The "agent asks a question / is blocked" detection signal in scope item 3
  needs a concrete definition before `@developer` can build it — flag this
  for `@analyst` to pin down as part of task breakdown, not to be guessed.
