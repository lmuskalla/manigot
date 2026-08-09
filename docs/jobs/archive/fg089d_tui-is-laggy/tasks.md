# Tasks: tui is laggy

id: fg089d
status: open
analyst: @analyst
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Open questions before implementation starts

The brief's "Why" and "Out of scope" sections are empty, and neither states
which platform/terminal the lag was observed on (macOS Terminal.app, a Linux
emulator, or inside tmux) — the fix for the second symptom differs by launch
path (see TASK-6). Per the project's hard rules ("when scope is unclear: ask,
don't guess"), this should be confirmed before TASK-6/TASK-7 land; the
investigation tasks (TASK-1, TASK-5) below are scoped so they can proceed
without that answer, but the fix tasks may need to be narrowed or widened once
it's known.

The brief describes two distinct, seemingly-unrelated symptoms. This
breakdown treats them as two independent bugs with two independent root-cause
hypotheses (below), rather than guessing at a single shared cause.

## Task breakdown

TASK-1: Confirm the root cause of the keystroke lag ("takes several seconds
to register Enter/Escape or you have to hit it several times"). Leading
hypothesis: `markdown.Render` (tui/internal/markdown/markdown.go) constructs a
brand-new `glamour.NewTermRenderer(glamour.WithAutoStyle(), ...)` on every
call, and `glamour.WithAutoStyle()` performs a terminal background-color probe
that reads from stdin — this can race with, and steal input from, Bubble
Tea's own stdin reader (which owns the terminal in raw/alt-screen mode),
delaying or swallowing real keypresses. This path is hit far more often than
"open a file": `detailView.setStatus` → `syncViewerSize` re-renders *all four*
tabs' markdown on every status change (i.e. after almost every keypress in
the detail view, including agent-launch attempts), not just the visible tab.
files: tui/internal/markdown/markdown.go, tui/internal/ui/detail.go
depends: none
risk: low — investigation only, no behavior change; requires interactive
terminal testing to confirm (not reproducible from static analysis alone).

TASK-2: Stop constructing a new `glamour.TermRenderer` (and re-triggering its
`WithAutoStyle()` terminal probe) on every markdown render; build/reuse a
renderer instead, keyed by wrap width since `WithWordWrap` is width-dependent.
files: tui/internal/markdown/markdown.go, tui/internal/markdown/markdown_test.go
depends: TASK-1 (only worth doing once the hypothesis is confirmed)
risk: medium — changes a shared rendering path used by every tab; must not
introduce stale-width rendering when the terminal is resized.

TASK-3: Stop eagerly re-rendering all four detail-view tabs on every status
change. `detailView.syncViewerSize` (called from `setStatus`, which itself
fires on nearly every keypress) resizes/re-renders all four `markdown.Viewer`s
even though only one tab is visible at a time. Re-render the active tab
immediately; defer the other three until they are actually switched to.
files: tui/internal/ui/detail.go, tui/internal/ui/detail_test.go
depends: TASK-2
risk: medium — changes viewer lifecycle to lazy re-render; must verify tab
switching and window-resize still produce correctly wrapped content for tabs
that were skipped while hidden.

TASK-4: Check whether `App.refresh()` — invoked synchronously inside
`Update()` on every "esc"/"backspace" (back to list) and "ctrl+r" — is itself
a source of perceived lag, since it re-walks the filesystem
(`job.Discover`) and re-reads/re-renders every job file
(`detailView.reload`) before the key handler returns. If TASK-1/TASK-3 do not
fully explain the reported lag, consider moving this work into an async
`tea.Cmd` instead of running it inline on the UI goroutine.
files: tui/internal/ui/app.go, tui/internal/ui/detail.go
depends: TASK-1
risk: medium — moving synchronous work to a `tea.Cmd` changes ordering
guarantees other code relies on (status text, cursor clamping); only pursue
if TASK-1/3 don't fully resolve the symptom.

TASK-5: Confirm the root cause of "a window appears and it immediately closes
again" when opening an agent. `launch.buildCmd` (tui/internal/launch/launch.go)
has five spawn paths (tmux new-window, macOS Terminal.app via osascript,
gnome-terminal, x-terminal-emulator, konsole, xterm) and none of them keep the
window/pane open after the inner `cd ... && mg --agent ... --job ...` command
exits — success or failure. tmux in particular closes ("destroys") a new
window as soon as its command exits unless `remain-on-exit` is set, which
matches the brief's "window" terminology. Any fast failure (e.g. `docker` not
running, resolve failure inside the container, auth error) is therefore
invisible: the window flashes and disappears before it can be read.
files: tui/internal/launch/launch.go
depends: none
risk: low — investigation only; needs confirmation of which of the 5 paths
the user is actually hitting (see Open questions).

TASK-6: Make the spawned terminal window/pane stay open after the inner
command exits so its output (including any error) is readable — e.g. wrap the
inner shell command so it pauses before the shell exits (only doing so
unconditionally is a possible route; only doing it on non-zero exit is
another and needs a decision), set tmux's `remain-on-exit` (and clear it) for
the new window, and use `xterm -hold` where available. gnome-terminal,
x-terminal-emulator and konsole have no built-in hold flag, so they need the
shell-wrapping approach.
files: tui/internal/launch/launch.go, tui/internal/launch/launch_test.go
depends: TASK-5
risk: medium — touches all five spawn paths' shell-quoting; a mistake in the
wrapper risks breaking a path that currently works, and behavior necessarily
differs per platform/emulator (untestable in CI beyond `buildCmd`'s argv
construction — no CI machine has a real GUI terminal or tmux session to
launch into).

TASK-7: Re-review `launch.Agent`'s error handling: today the *launcher*
process's own stdout/stderr are discarded (`io.Discard`), so if e.g.
`gnome-terminal` itself fails to start (as opposed to the inner command it
was asked to run), that failure is silent beyond `cmd.Start()`'s error. Decide
whether to surface anything here beyond what TASK-6 already exposes via the
kept-open window.
files: tui/internal/launch/launch.go
depends: TASK-6
risk: low — additive error surfacing only.

TASK-8: Manual verification pass confirming both fixes against the symptoms
in the brief: keystroke responsiveness while navigating the list and detail
views and firing agents (TASK-2/3/4), and that an agent-launch window stays
open long enough to read its outcome, including a deliberately-forced failure
case (TASK-6), on whichever platform(s) are confirmed per the Open questions
above. Also run the existing suite (`go test ./tui/...`) to check for
regressions.
files: none (manual verification + existing test suite)
depends: TASK-2, TASK-3, TASK-6
risk: low — verification only.

## Explicitly not covered by this breakdown

- Any change to `scripts/entrypoint.sh`, the `Dockerfile`, or the container-side
  agent CLIs — the brief describes host-side TUI symptoms only (input
  handling and window spawning), not agent behavior once a session starts.
- Redesigning the launch mechanism itself (e.g. switching to `tea.ExecProcess`
  for agents the way the "e" edit shortcut already does for editors) — out of
  scope unless TASK-5/6 conclude the current detached-window architecture
  cannot be fixed in place; not assumed here.
