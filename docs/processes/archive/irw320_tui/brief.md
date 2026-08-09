# Brief: TUI

status: done
type: feature
id: irw320
branch: feature/irw320_tui
date: 2026-08-08
author: Leander Muskalla

## What

Safecode should get an optional TUI.

A terminal UI for managing jobs and running agents without remembering command syntax.

- [ ] Choose stack — evaluate Bubble Tea (Go) vs Textual (Python) vs Ink (Node) for fit
- [ ] Job list view: show all jobs under `docs/processes/`, display ID, title, type, status
- [ ] Job detail view: render brief.md, tasks.md, result.md, verdict.md as readable markdown
- [ ] Agent action bar: buttons for each agent relevant to current job stage
  - open job → show "Run Product Owner", "Run Analyst"
  - tasks written → show "Run Developer"
  - result written → show "Run Reviewer", "Run Security"
- [ ] Firing an agent opens `safecode --agent <name> --job <id>` in a new terminal window or pane
- [ ] Job status tracking: parse status field from brief.md, update display accordingly
- [ ] New job shortcut: trigger `new-job` from within the TUI
- [ ] Update README

Look at https://github.com/charmbracelet/bubbletea. Would that be a good option for building the TUI?

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

### Decision (TASK-1): Go + Bubble Tea

**Stack: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Go), with
[Lipgloss](https://github.com/charmbracelet/lipgloss) for styling and
[Glamour](https://github.com/charmbracelet/glamour) for markdown rendering.**

Compared against Textual (Python) and Ink (Node) against this job's
constraints (host-side tool, shells out to bash, renders markdown, must spawn
new terminals cross-platform):

| | Bubble Tea (Go) | Textual (Python) | Ink (Node) |
|---|---|---|---|
| Host runtime needed | none — single static binary | Python + deps | Node + npm tree |
| Markdown rendering | Glamour (same family) | rich-textual | manual |
| Distribution fit | one binary, mirrors `safecode`/`new-job` symlink pattern | venv/pyinstaller | npm or bundle |
| New host dependency | no | yes (Python) | yes (Node) |

Go is the right call because the safecode host tooling is currently
bash + Docker with **no Python or Node requirement**. Bubble Tea ships a
self-contained binary that drops into PATH the same way `safecode` and
`new-job` already do, Glamour handles the fiddly markdown rendering
out-of-the-box, and the brief author specifically flagged Bubble Tea. Verified
`go get` of bubbletea/lipgloss/glamour against the public proxy.

### Scope decisions for the open questions (tasks.md §"Open questions")

1. **Target platforms (TASK-8): macOS + Linux.** No Windows for v1 — it's a
   follow-up. Terminal-spawn order of preference: **tmux** split (if a tmux
   server is detected, works everywhere tmux does), then **macOS Terminal.app**
   via `osascript`, then a generic Linux launcher (`x-terminal-emulator` /
   `gnome-terminal`).
2. **`finish-job`: out of scope for this job.** Per tasks.md, surface it in a
   later phase. The TUI wires up `safecode` and `new-job` only.
3. **Distribution: single static binary** via `go build`, installed with the
   same symlink-to-PATH pattern as the other launchers. New `make tui` target
   builds it.
