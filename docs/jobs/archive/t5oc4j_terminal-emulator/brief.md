# Brief: terminal emulator

status: done
type: feature
id: t5oc4j
branch: feature/t5oc4j_terminal-emulator
date: 2026-08-10
author: Leander Muskalla

## What

  When the TUI launches a non-jdi agent session (`launch.Agent`/`launch.Quick`
  in `tui/internal/launch/launch.go`) and the TUI process is itself already
  running inside a tmux session (`$TMUX` set), open the session as a split
  pane in the current tmux window (`tmux split-window`) instead of a brand-new
  tmux window (`tmux new-window`, the current behavior). No other spawn path
  changes — Terminal.app, gnome-terminal, ptyxis, x-terminal-emulator,
  konsole, xterm all stay exactly as they are today.

  ## Why

  Every agent launch from the TUI currently opens a separate window the user
  has to individually track and switch to. Inside tmux this is already a new
  *window*, not a pane, so it still requires switching (next/prev window or
  picking it off the window list) rather than appearing alongside the TUI.
  Splitting into a pane keeps the launched session visible next to the TUI
  without hunting for it. This is the smallest version of the "In-TUI agent
  terminal" idea already logged in `docs/backlog.md` — validate this first
  before considering that heavier, dedicated feature.

  ## Out of scope

  - Bundling or auto-installing tmux. It stays a fully optional, detected
    dependency exactly like every other terminal in `buildCmd` — never a
    required install for any manigot user.
  - Auto-wrapping/launching the TUI inside a tmux session when it isn't
    already running in one. If the user isn't in tmux, behavior is unchanged
    from today (new OS terminal window via the existing fallback chain).
  - Embedding a PTY/terminal renderer inside the Go TUI itself — that's the
    full `docs/backlog.md` "in-TUI agent terminal" item, a separate and much
    larger piece of work, not part of this job.
  - Changing `launch.Jdi` — `mg jdi` runs already have no terminal at all
    (fully detached, visibility via the status badge/log tab), unaffected by
    this.

  ## Notes

  Open questions to resolve before this goes to @analyst:
  - Split direction/sizing (`split-window -h` vs `-v`) and which pane keeps
    the larger share — probably the TUI keeps priority since it's the
    always-open anchor.
  - Repeated launches: does each new agent launch reuse/replace the existing
    split pane, or does every launch add a new one? Unbounded pane
    accumulation would just reintroduce today's window-sprawl problem in
    miniature — needs a deliberate answer, not a default.
  - Confirm the existing `holdOnFailure` wrap (`launch.go`) still behaves
    correctly inside a split pane (fast-failure messages should stay visible
    the same way they do in a new window today).
