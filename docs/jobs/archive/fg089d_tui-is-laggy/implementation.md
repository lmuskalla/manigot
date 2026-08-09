# Implementation: tui is laggy

id: fg089d
status: open
developer: @developer
date: 2026-08-09

## Summary

Both symptoms in the brief were root-caused and fixed:

1. **Keystroke lag / dropped keys.** `markdown.Render` built a brand-new
   `glamour.TermRenderer` with `glamour.WithAutoStyle()` on every call.
   `WithAutoStyle()` → `termenv.HasDarkBackground()` writes a terminal OSC
   query and then reads raw bytes directly off stdin until it sees the
   response — a read that races with, and can consume, Bubble Tea's own
   raw-mode stdin reader. `detailView.setStatus` (fired on nearly every
   keypress in the detail view) re-rendered all four tabs' markdown via
   `syncViewerSize`, so this raced constantly. Fixed by caching the renderer
   per wrap width (TASK-2) and only eagerly re-rendering the active tab,
   deferring the other three until they're actually switched to (TASK-3).

2. **Agent-launch window flashing closed.** None of `launch.buildCmd`'s five
   spawn paths (tmux new-window, Terminal.app, gnome-terminal,
   x-terminal-emulator, konsole, xterm) kept the window/pane open after the
   inner `mg --agent ... --job ...` command exited, so a fast failure was
   invisible. Fixed by wrapping the inner shell command (TASK-6) so a
   non-zero exit prints the status and waits for Enter before the shell
   exits; a clean exit is left alone since a normal agent session just runs
   until the user ends it.

## Changes

TASK-1: Confirmed the keystroke-lag hypothesis by reading termenv's
`readNextResponse`/`termStatusReport` (reads raw stdin bytes waiting for an
OSC response) and tracing `setStatus` → `syncViewerSize`'s call frequency.
No behavior change; documented in the TASK-2 commit message.
files: none (investigation)

TASK-2: `markdown.Render` now builds/reuses one `glamour.TermRenderer` per
wrap width via a small cache (`rendererFor`), instead of constructing one
(and re-running the stdin probe) on every call.
files: tui/internal/markdown/markdown.go, tui/internal/markdown/markdown_test.go

TASK-3: `detailView.syncViewerSize` now resizes only the active tab's
viewer immediately and marks the other three `stale`; a stale tab is
resized lazily in `render()` the moment it becomes active
(`ensureCurrentSized`).
files: tui/internal/ui/detail.go, tui/internal/ui/detail_test.go

TASK-4: Investigated whether `App.refresh()` (job.Discover +
`detailView.reload`) contributes to the lag. It's plain local-disk I/O over
a handful of small files — not a plausible source of multi-second,
input-dropping lag once TASK-1/3 are fixed. Documented the decision not to
move it to an async `tea.Cmd` inline in `refresh()`'s doc comment.
files: tui/internal/ui/app.go

TASK-5: Confirmed the launch-window hypothesis by reading `buildCmd`'s five
spawn paths: tmux destroys a new window as soon as its command exits (no
`remain-on-exit`), and gnome-terminal/x-terminal-emulator/konsole/xterm all
close on exit by default. Terminal.app's `do script` doesn't have this
problem (the window just returns to a shell prompt), so it wasn't the
mechanism at fault, but the fix is applied uniformly anyway since it's
harmless there. Documented in the TASK-6 commit; no code change of its own.
files: none (investigation)

TASK-6: Added `launch.holdOnFailure`, which wraps the inner shell command so
a non-zero exit prints the failure and blocks on `read` before the shell
exits; `shellCommand` now returns the wrapped form. Applied uniformly to
all five spawn paths via the shared inner-command string, rather than
per-terminal mechanisms (tmux `remain-on-exit`, `xterm -hold`), so the fix
is one change and stays testable with plain string assertions.
files: tui/internal/launch/launch.go, tui/internal/launch/launch_test.go

TASK-7: Reviewed `launch.Agent`'s discarded launcher stdio/exit code beyond
what TASK-6 covers. `cmd.Start()` failures are already surfaced. Decided
not to surface a launcher-binary-itself failure (e.g. gnome-terminal
exiting non-zero with no display): doing so safely would require either
blocking on `cmd.Wait()` (unsafe — xterm's process *is* the window and
won't return until the session ends, which would reintroduce a UI freeze)
or a `tea.Msg` back-channel from the reaping goroutine (an architecture
change out of proportion for this job). Documented inline instead.
files: tui/internal/launch/launch.go

TASK-8: Verification pass.
- `go test ./...` (run from `tui/`) passes, including the new tests added
  for TASK-2/3/6.
- Built `bin/manigot-tui` via `make tui` and drove it over a real pty
  (Python's `pty` module) with keystroke timing: Enter (open a job) and
  Escape (back to list) both round-tripped in ~15ms, quit in <1ms — no
  multi-second stalls or dropped keys, matching the diagnosis and fix.
- Verified `holdOnFailure`'s shell logic directly under `bash -c`: a
  non-zero exit prints `--- manigot: exited with status N ---` and blocks
  on `read` (window stays open); a zero exit runs straight through with no
  extra output (window behavior unchanged for the common case).
- Could not exercise the real GUI-terminal/tmux spawn paths themselves
  (`buildCmd`'s tmux/osascript/gnome-terminal/konsole/xterm branches) since
  this environment has no display server, tmux session, or those binaries
  installed — this matches the tasks.md risk note that these paths are
  "untestable in CI beyond `buildCmd`'s argv construction." The platform
  used to originally report the bug was not specified in the brief (see
  tasks.md's open question); the fix was applied uniformly across all five
  paths so it isn't dependent on knowing which one is in use.
files: none (manual verification + existing test suite)

## Known issues / follow-ups

- The brief's "Why"/"Out of scope" sections and the platform the lag was
  observed on were never filled in (tasks.md's open question). The fixes
  address the mechanism confirmed by static analysis (a stdin-racing
  terminal probe, and none of the five spawn paths holding their window
  open) and were spot-checked with a scripted pty session in this
  environment, but a real interactive confirmation on the user's actual
  terminal/OS is still worth doing.
- TASK-7's launcher-binary-failure gap (e.g. gnome-terminal itself failing
  with no display) is intentionally left unaddressed — see that task's
  entry above for why surfacing it safely would need a bigger change than
  this job's scope.
