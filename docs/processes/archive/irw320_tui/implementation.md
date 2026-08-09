# Implementation: TUI

id: irw320
status: open
developer: leomuck@posteo.de
date: 2026-08-08

<!-- Produced by @developer after implementation. -->

## Summary

Added an optional, host-side terminal UI for safecode (`safecode-tui`), built in
Go with the Charm stack (Bubble Tea + Lipgloss + Glamour). It browses a
project's jobs under `docs/processes/`, renders their markdown files, surfaces
the right agents for each job's workflow stage, fires agents in a new terminal,
and creates new jobs — all by shelling out to the existing `safecode` /
`new-job` commands. It runs on the user's machine, never inside the container,
and needs no credentials.

The TUI lives in a new `tui/` Go module at the repo root. All 12 tasks are
implemented, each in its own commit. The module builds and `go test ./...` passes
on Go 1.23; `make tui` produces a single static binary.

## Changes

TASK-1 — `docs/processes/irw320_tui/brief.md`: recorded the stack decision
(Go + Bubble Tea/Lipgloss/Glamour) with a comparison table against Textual and
Ink, and resolved the three open scope questions (macOS+Linux target; `finish-job`
deferred; single-binary + symlink distribution).

TASK-2 — `tui/` (new): scaffolded the Go module (`tui/go.mod`, `go.sum`,
`main.go`). Module path `codeberg.org/lmuskalla/safecode/tui`. Minimal but real
Bubble Tea entry point; slimmed in TASK-4 to launch the App.

TASK-3 — `tui/internal/job/` (new): `FindProjectRoot` (mirrors the bash
walk-up-to-`docs/`), `Discover` (enumerates `docs/processes/`, excludes
`archive/`, sorts date-desc), and the loose (non-YAML) `brief.md` frontmatter
parser. Unit tests cover the parser, defaults, archive exclusion, root walk-up,
and parsing against this repo's real jobs.

TASK-4 — `tui/internal/ui/app.go`, `styles.go` (new): the job list view —
header, ID/STATUS/TYPE/DATE/TITLE columns, keyboard nav (↑/↓/j/k/g/G),
status-colour coding (open/done). `main.go` now finds the root and launches the
App.

TASK-5 — `tui/internal/markdown/` (new): Glamour-backed `Render` plus a
dependency-free scrollable `Viewer` (resize re-wraps, clamp-on-scroll,
position reporting). Pinned deps to `go 1.23` (glamour v0.8.0, lipgloss v1.0.0,
bubbletea v1.2.4) to avoid a Go 1.24 toolchain auto-download.

TASK-6 — `tui/internal/ui/detail.go` (new): detail view composing the four job
files as tabbed, scrollable markdown; list↔detail and file↔file navigation;
"not written yet" placeholders for missing files.

TASK-7 — `tui/internal/job/stage.go` (new) + `tui/internal/ui/agents.go` (new):
defined and pinned the "written" rule (real content beyond scaffold comments /
empty headings / frontmatter), derived the job stage (analyze/develop/review),
and rendered a stage→agents action bar with stable trigger keys.

TASK-8 — `tui/internal/launch/` (new): spawns
`cd <root> && safecode --agent <name> --job <id>` in a new terminal, choosing
tmux new-window → macOS Terminal.app → Linux emulator (gnome-terminal /
x-terminal-emulator / konsole / xterm). Arguments are single-quoted with the
standard `'\''` escape (injection-tested). Wired the agent keys into the detail
view, gated by stage.

TASK-9 — `tui/internal/hostcmd/` + `tui/internal/ui/newjob.go` (new): an `n`
shortcut that opens a title/type form (bubbles/textinput) and runs the host
`new-job` command (with `cwd` and `PWD` set to the project root so its
find_project_root resolves), then refreshes the list.

TASK-10 — `tui/internal/ui/app.go`, `detail.go`: refresh (ctrl+r, plus
auto-refresh on returning to the list) re-reads job files edited out-of-band by
agents and clamps the cursor if a job was archived. Status (open/done) already
shown in list + detail from TASK-4/6.

TASK-11 — `Makefile`, `scripts/safecode-tui.sh` (new), `.gitignore`: `make tui`
builds a static `bin/safecode-tui`; the launcher script mirrors the
`scripts/*.sh` symlink-to-PATH install pattern used by `safecode` / `new-job`.

TASK-12 — `README.md`: documented the TUI (what it is, host-side nature,
supported platforms + spawn order, build/install, run, keybindings, and the
stage→agent model) and added the new files to the repo tree. (`finish-job` docs
left for the follow-up chore per tasks.md Conventions §3.)

## Known issues / follow-ups

- **Windows unsupported** (out of scope per TASK-1 decision). A Windows terminal
  launcher (e.g. `wt.exe`) would slot into `launch.buildCmd`.
- **`finish-job` is not surfaced in the TUI** (deferred per TASK-1 decision #2
  and tasks.md Conventions §3) and is still undocumented in the README.
- **Agent-launch key feedback only.** Firing an agent opens the new terminal and
  sets a status line; the TUI does not detect when the agent finishes — the user
  returns and presses ctrl+r (or esc, which auto-refreshes).
- **Terminal focus-report refresh** is not wired: focus events are unreliable
  across terminals, so refresh is manual (ctrl+r) + on view transitions instead.
- **Glamour auto-style** queries the terminal background colour on first render
  (`]11;?`); termenv handles the timeout, but a fixed dark/light style is a
  follow-up if startup latency matters.
- **Pre-existing uncommitted changes** in `AGENTS.md` and `docs/TASKS.md` were
  present in the working tree at session start and were intentionally left
  unstaged (not part of this job).
