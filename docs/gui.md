# GUI: translating the TUI to a native desktop application

Exploration notes (2026-08-27). Verdict: **REVISIT** — the direction is plausible,
but the question as phrased ("translate our TUI to a nice GUI") hides a fork that
decides the size of everything, and the foundations the GUI would need to be
*nice* on top of don't exist yet. This document records the fork, what already
works, the genuinely new pieces, and the recommended path.

## Context: same week as the web-interface exploration

A sibling question was explored the same day — `docs/web-interface.md`: running
manigot on a server with a browser control plane (verdict REVISIT, sequenced
behind roadmap items #4 and #5). A native GUI is the *same bet on a different
surface*: it answers the same underlying question ("watch and control jobs
without sitting in a terminal") locally instead of remotely. The two share the
same fork and the same missing foundations. The web doc's own caution applies
here verbatim: don't build the same bet on a second surface at the same time.
The roadmap (item 6, in-TUI embedded terminal) is the same embedded-terminal bet
on a *third* surface. Only one of these should be built, on the surface the
product actually commits to.

## The idea

Replace the terminal-bound `mg tui` with a native desktop GUI on Linux (no other
platforms for now — consistent with the product's existing Linux-first posture
and its total lack of a Windows story). A window showing projects, their jobs,
and job state — with the ability to create jobs, view/edit the four files,
review diffs, launch agents, watch runs live, and run the lifecycle
(done / delete / push / merge).

## What already works (the good news)

Most of the heavy machinery is already UI-independent — the TUI calls it
in-process, and a GUI process can call the same functions in-process:

- **`internal/job`** — `CreateJob`, `FinishJob`, `DeleteJob`, `Discover`,
  `ReadJDIStatus` are pure functions over a project root. No TTY involved.
- **`internal/git`** — worktree create/remove, squash merge, push, diff,
  commits. All host-side, all callable.
- **`mg jdi`** — already runs fully unattended via the `--print` path, writes a
  pollable status sidecar (`.manigot/jdi-status/`), handles
  `NEEDS-HUMAN-INPUT:` markers, and pushes ntfy notifications. Exactly the state
  model a GUI would render.
- **The state model of the TUI itself** — `mg tui` is already a long-lived
  process with refresh/polling/timer ticks (ctrl+r, the jdi badge, the spinner).
  A GUI process is the same shape. This is the 70%-there part.
- **Linux-only scoping** — fine as a product decision; no Windows story exists
  at all, and the terminal-candidate table already proves Linux-first.

## The fork that decides everything

**Is the GUI a control plane for background runs, or a home for interactive
agent sessions?**

- **Control plane (recommended for v1):** browse jobs, view/edit the four
  files, launch `mg jdi` / one-shot `--print` agent runs in the background,
  watch them live, review the diff, answer `NEEDS-HUMAN-INPUT:` markers, and run
  the lifecycle (done / delete / push / merge). Reuses everything above.
- **Interactive agent sessions in the GUI:** embedded terminals so you can chat
  with an agent from the window. This is the roadmap's item 6 bet (in-TUI
  embedded terminal — "biggest bet, last, smallest slice first"), transplanted
  onto a second surface. This is the expensive one, and it is already being
  sized in the roadmap for the TUI.

The trap is in the word "translate". Today every interactive agent launch spawns
a tmux pane or a terminal window (`internal/launch`) — the TUI's replace-policy
pane, the gnome-terminal/ptyxis/x-terminal-emulator/konsole/xterm chain, tig.
A GUI cannot do tmux panes. So a GUI that *replaces* the TUI inherits the
interactive question whether it wants it or not. Either it is a control plane
that *complements* the TUI (interactive sessions stay in the terminal), or it is
the PTY bet. There is no middle, and the choice decides the size of the whole
thing.

## User perspective

The TUI's user is a developer who already lives in the terminal. The honest
candidates for what a GUI buys them:

- A glanceable, always-open surface for run status without being inside tmux.
- Rich rendering the terminal can't do well: side-by-side diffs, markdown, real
  text selection/copy.
- Native notifications instead of a terminal bell / ntfy on the phone.
- Discoverability (menus instead of keybindings).

But the product's own trajectory cuts against the premise: the autonomy story
(`mg jdi`, headless, the away digest) exists precisely so the human's job is
*review and decide*, not drive. The roadmap's whole arc makes the terminal matter
*less*, not more. The cheap version of the "check on runs without being in a
terminal" need already exists — `mg jdi` + ntfy from an always-on box, answered
from a phone (the web doc's "validate the cheap version first"). A GUI is a
justified answer only if the need that survives that test is *local* and
*glanceable*; if it is *remote*, the web exploration from the same day is the
better path. Don't build both.

## Scope assessment — the genuinely new pieces

The UI itself — list/detail/tabs/diff/forms — is a port of existing information
design. The real new work:

1. **Run supervision with streamed logs.** Today's interactive launches spawn
   windows; a GUI-launched agent must be a detached process with captured output
   the UI watches live. The status sidecars give you *stage*, not what the agent
   is *doing*. This is roadmap item 5 (event-streaming) — currently not built.
   This is the same gap the web doc names, and the biggest single piece.
2. **Live updates.** "Nice" in a GUI means live. Polling the coarse sidecars
   (what the TUI does on ctrl+r today) is exactly the non-dynamic experience to
   avoid. "Dynamic" should mean the event stream made visible — not 40 widgets.
3. **Editing.** The TUI suspends into `$EDITOR`; a GUI can't. Embedded markdown
   edit, or "open in editor and reload on return" — small either way, but a
   decision.
4. **Diff rendering.** The TUI renders `git diff --stat` as plain text. A
   side-by-side diff is the single most legitimate "nice GUI" win — and a real
   component of its own.
5. **Packaging / environment.** A GUI adds a display-server dependency
   (X11/Wayland) to a product that is otherwise headless-capable — fine for the
   target user, but a new environment assumption. Desktop entry, icon, updates:
   new surface the product has never had. Whatever the toolkit, the "one Go
   binary" ethos should be preserved as much as possible.

## What is NOT the blocker

- **The state model.** `mg tui` is already a long-lived process with
  refresh/polling/timers calling `internal/job` and `internal/git` in-process.
  A GUI process does the same thing.
- **Linux-only.** Consistent with the product's existing posture. The one caveat
  is the display-server dependency above, not the OS choice.

## Concerns

1. **Three surfaces, one bet.** The web doc says don't build the interactive
   terminal slice on a second surface while it is the roadmap's item 6. A GUI
   risks the same collapse: the embedded-terminal slice is the identical bet and
   should be built once, on the surface the product commits to.
2. **Port, don't re-imagine.** "Boring in the right places" is a product
   property, not an accident. A GUI must port the TUI's information design — id /
   status / stage / type / date / title, the four files, the diff tab, the same
   actions — not reinvent what a dashboard should be. This is exactly where
   "nice" goes wrong.
3. **Foundation sequencing.** A GUI built on today's polling is a worse TUI with
   more pixels. Built on items #4/#5 it is a genuinely better surface. The
   difference is the sequence, not the toolkit.
4. **One-binary ethos.** The web doc calls embedding the UI into the Go binary
   "very on-brand" for a product that reduced itself to one binary on purpose.
   Packaging cost should be counted up front, not discovered.

## Recommendation

1. **Decide the fork and the relationship to the TUI** — control plane vs.
   interactive, complement vs. replace. Stake the answer before any code.
2. **Validate the cheap version first** (same as the web doc): the away digest —
   `mg jdi` + ntfy on an always-on box, answered from a phone — may cover a large
   share of the "check on runs without being in a terminal" need at near-zero
   cost. If the need that survives is *local* and *glanceable*, a GUI is
   justified; if *remote*, the web exploration is the better path. Not both.
3. **Land roadmap items #4 (headless/queue) and #5 (event-streaming) once**,
   against the run.log consumer the roadmap names, and let the GUI attach to the
   event stream afterward.
4. **Only then** scope the surface job: persistent process, streamed-run
   supervision, info-design port, editor story, native notifications, diff view.
   Each is cleanly separable.

The short version: translating the TUI is the easy 20%. The run-supervision +
event-stream foundation is the real work, it is shared with the web direction
explored the same day, and the fork — control plane or embedded terminals —
decides whether this is three clean jobs or one unbounded bet. Decide the fork
first, and don't build two of these surfaces at once.