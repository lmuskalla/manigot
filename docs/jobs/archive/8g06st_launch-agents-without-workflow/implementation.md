# Implementation: Launch agents without workflow

id: 8g06st
status: open
developer: claude
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

Both asks from the brief are implemented, following `tasks.md`'s breakdown
task by task.

**Feature A — launch any agent regardless of stage (TASK-1 → 4):**
`App.agentForKey` no longer looks at `job.Stage()`; it matches a pressed key
against a new fixed-order list (`agentOrder` in `agents.go`) covering all five
agents, so any of them fires from any stage. The detail view's action bar
(`renderActionBar`) always renders all five buttons in that same order.
`Stage()` itself (and the `stage: <name>` label) stays as an informational
hint — only its former use as a gate is gone. `Stage.Agents()`, now called by
nothing, was removed as dead code along with its dedicated test. Added
`agents_test.go` covering all five agent keys across all three stages, plus
the always-shows-all-five action-bar rendering.

**Feature B — mark a job done from the TUI (TASK-5 → 9):** added
`hostcmd.DoneCommand`, mirroring `NewJob`'s resolve + cwd/`$PWD` pattern for
`sc-done` (`scripts/finish-job.sh`), keyed on the job's exact directory name.
Wired a new capital-`D` key in the detail view that runs it via
`tea.ExecProcess` (the same foreground suspend/resume `e` edit already uses),
since `finish-job.sh`'s `read -rp` confirmations need a real terminal. Per Q2
in `tasks.md` (`finish-job.sh`'s exit code doesn't reliably mean "archived" —
every confirmation's decline path also exits 0), the `doneMsg` handler always
refreshes the job list from disk and returns to it regardless of outcome; a
non-zero exit still surfaces through the existing `cmdErrorText` path first.
The action bar and footer were updated to show the new key, visually
separated from the agent buttons. Added tests for `DoneCommand`'s resolution
and constructed command, and for the `App`-level `doneMsg` handling (clean
success, "declined" nil-error-but-job-still-present, and non-zero error).

README.md's "Stage → agent model" section, which documented the now-removed
gating, was updated to match the new behavior and to document the `D` key.

## Changes

- `tui/internal/ui/app.go` — `agentForKey` no longer gates on `job.Stage()`;
  added `doneMsg`, `doneCmd`, the `"D"` key case in `updateDetail`, and the
  `doneMsg` handling in `Update` (TASK-1, TASK-6, TASK-7).
- `tui/internal/ui/agents.go` — added `agentOrder`, the fixed display-order
  list the action bar and `agentForKey` now both use (TASK-2).
- `tui/internal/ui/detail.go` — `renderActionBar` always renders all five
  agent buttons plus a separated `[D] Done` button; footer hint mentions `D`
  (TASK-2, TASK-8).
- `tui/internal/job/stage.go` / `stage_test.go` — removed `Stage.Agents()`
  and `TestStageAgents`; `Stage()`/the stage constants are kept for the
  informational label (TASK-3).
- `tui/internal/ui/agents_test.go` (new) — tests for TASK-4.
- `tui/internal/hostcmd/hostcmd.go` — added `DoneCommand` (TASK-5).
- `tui/internal/resolve/commands.go` — updated `Done()`'s doc comment now
  that it has a caller.
- `tui/internal/hostcmd/hostcmd_test.go` — tests for `DoneCommand` (TASK-9).
- `tui/internal/ui/donemsg_test.go` (new) — tests for the `doneMsg`/`D`-key
  flow (TASK-9).
- `README.md` — updated the keybindings table and the former
  "Stage → agent model" section to match the new behavior.

## Known issues / follow-ups

- Q1 in `tasks.md` (no list-view "mark done" shortcut) and Q2 (not fixing
  `finish-job.sh`'s exit-code ambiguity itself) were both explicitly flagged
  as out of scope by the analyst breakdown and left as-is.
- Q3 was resolved by removing `Stage.Agents()` rather than keeping it for a
  future "highlight the recommended agent" treatment — nothing in the brief
  asked for that, and an unused exported method with no caller is worse than
  re-adding it later if it's actually wanted.
