# Implementation: new agent: git solver

id: baby
status: open
developer: claude
date: 2026-08-25

<!-- Produced by @developer after implementation. -->

## Summary

Added a new global agent, `@git-solver`, an expert at diagnosing and fixing
tricky git/git-worktree states (broken worktrees, conflicted merges/rebases,
detached HEADs, stray branches), resolving merge conflicts, and advising safe
cleanup. It gets read/write/git access like `@developer`, commits its own
work, and is subject to the exact same platform-wide git-shim denylist as
every other committing agent — no special exemption. Documentation (README's
agent table, ROADMAP's agent count) updated to match.

## Changes

TASK-1: Created `agents/git-solver.md` — frontmatter matches `@developer`'s
shape (`name: git-solver`, one-line `description:`, `tools: Read, Write,
Edit, Bash, Grep, Glob`, `commit: true`, and the same OpenCode `permission:`
block with the standard destructive-git-command denylist). Body covers what
it diagnoses/resolves/advises, includes the "Branch" check section (same
wording as `devops.md`/`sysadmin.md`), and has an explicit "Container
limitation" section spelling out that inside a job/container session it can
diagnose, resolve conflicts via edit+commit, and inspect history, but cannot
run any shim-refused command (worktree fixes, force-remove, hard reset,
branch delete, etc.) — for that class of fix it tells the user to re-run it
via `mg host`. Hard rules mirror the other committing agents (no push, no
merge, no touching other branches, no routing around the shim).

TASK-2: Updated `README.md`'s Agents section — "Thirteen agents" →
"Fourteen agents", and appended a `@git-solver` row to the agent table
(role summary, "read + write" tools) at the end, matching the existing
append-only ordering.

TASK-3: Updated `docs/ROADMAP.md`'s current-state paragraph — "thirteen
agents" → "fourteen agents".

TASK-4: Verified end to end:
- `go test ./...` — `internal/agentlist`, `internal/cli`, `internal/config`,
  `internal/editor`, `internal/home`, `internal/launch`, `internal/markdown`,
  `internal/notify`, `internal/orchestrate`, `internal/project` all pass.
  `cmd/mg`, `internal/git`, `internal/job`, `internal/session`, `internal/ui`
  fail, but only because their hermetic fixtures need `git init`/`worktree`/
  `merge`/`push` to set up temp repos, and this agent session's own git shim
  (the one documented in `docs/AGENTS.md`) refuses those commands — this is
  a pre-existing limitation of running the test suite from inside a manigot
  agent session, confirmed unrelated to this change (same failures, same
  shim-refusal messages, on packages this job never touched).
- Confirmed `agentlist.Discover` picks up the new agent: an ad-hoc test run
  with `MANIGOT_HOME=/workspace` against `internal/agentlist` found
  `git-solver` in the discovered list (temp test file, removed after
  verifying — not part of the diff).
- Confirmed no changes to `internal/ui/agents.go`, `internal/agents/
  agents.go`, `internal/orchestrate/`, `Dockerfile`, or
  `scripts/entrypoint.sh` (`git status`/`git diff --stat` on those paths is
  empty) — the new agent is a standalone utility agent outside the TUI
  action bar and the `mg jdi` sequence, and gets no shim exception, per the
  task's explicit constraints.

## Known issues / follow-ups

- The container image must be rebuilt (`make rebuild`) before `@git-solver`
  actually exists inside running containers — an ops step, not a code
  change.
