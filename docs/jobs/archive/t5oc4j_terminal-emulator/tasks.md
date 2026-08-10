# Tasks: terminal emulator

id: t5oc4j
status: open
analyst: leomuck@posteo.de
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Scope note

Only `buildCmd`'s tmux branch in `tui/internal/launch/launch.go` (currently
`tmux new-window`) changes, and only when `$TMUX` is set. Every other spawn
path (Terminal.app, gnome-terminal, ptyxis, x-terminal-emulator, konsole,
xterm) and `launch.Jdi` (no terminal at all) are untouched. `launch.Agent`
and `launch.Quick` both funnel through the same `buildCmd`, so both are
affected identically — the brief doesn't distinguish between them.

## Open questions from the brief's Notes — proposed resolutions

The brief flags these as "resolve before this goes to @analyst" but no
product-owner/decision step happened first, so recording proposed answers
here (with rationale), to be confirmed at/before TASK-1 rather than guessed
silently — following the same pattern used in
`docs/jobs/archive/irw320_tui/tasks.md`.

1. **Split direction/sizing.** Proposed: `tmux split-window -h` (side by
   side, vertical divider) with the new pane sized smaller — e.g.
   `-p 35` (35% of the window) — so the TUI's existing pane keeps the
   majority of the width. This matches the brief's own "probably the TUI
   keeps priority" note. `-v` (stacked top/bottom) is the alternative if a
   narrower terminal window is the more common case — worth a quick check
   with the user before locking TASK-1, since the brief only said
   "probably."

2. **Repeated launches — reuse or accumulate.** Proposed: **replace**. Track
   the most recently manigot-opened split pane (e.g. by tagging it with a
   pane title via `select-pane -t <id> -T <tag>` after the split —
   `split-window` has no `-T` on any released tmux — and finding/killing it
   with `tmux list-panes`/`tmux kill-pane` before creating the next one) and
   kill it before opening a new split, so there is at most one manigot-opened
   pane at a time. A single shared tracked pane across both `launch.Agent`
   and `launch.Quick` (not one tracked pane per call site) — the brief
   introduces them together as one behavior change. This is the option the
   brief's own Why section argues for ("unbounded pane accumulation would
   just reintroduce today's window-sprawl problem in miniature"), so
   treated as the working assumption for TASK-3, but the brief explicitly
   asked for "a deliberate answer, not a default" — flag this specific
   point back to the user for an explicit yes/no before TASK-3 starts.
3. **`holdOnFailure` inside a split pane.** Not a design choice, just needs
   confirming (TASK-4) — `holdOnFailure` wraps the inner shell string
   identically regardless of terminal type, so it should behave the same in
   a pane as in a window, but this needs to be checked against #2's
   replace-on-relaunch logic too: a pane currently holding open on a
   failure must not be silently killed by an unrelated new launch without
   that being a deliberate, called-out choice (see TASK-4).

## TASK-1 scope decision (confirmed)

No further product-owner/human input was available before implementation
started, so the proposed resolutions above are adopted as-is, since they are
each directly grounded in the brief's own text (its "probably the TUI keeps
priority" note for #1, and its explicit "unbounded pane accumulation would
just reintroduce today's window-sprawl problem" argument for #2) rather than
an unguided guess:

1. **Split direction/sizing:** `tmux split-window -h -p 35` — side-by-side,
   new pane gets 35% of the window width, TUI's existing pane keeps the
   majority (65%).
2. **Repeated launches:** **replace**. A single shared tracked pane across
   both `launch.Agent` and `launch.Quick`, identified via a pane title tag
   (`select-pane -t <id> -T <tag>` applied after the split — `split-window`
   has no `-T` on any released tmux) and looked up/killed via
   `tmux list-panes`/`tmux kill-pane` before opening the next one.
3. **`holdOnFailure` interaction:** see TASK-4 — resolved there rather than
   here, since it needs the TASK-3 implementation in hand to reason about
   concretely.

This is flagged in `implementation.md`'s "Known issues / follow-ups" as an
assumption made without an explicit human yes/no, in case it needs revisiting.

## TASK-4 decision (confirmed)

Resolved during TASK-4, recorded here and in `launch.go`'s `holdOnFailure` /
`killPreviousTmuxPane` doc comments (this file's comments double as the
authoritative design record — see TASK-6):

- **`holdOnFailure` inside a split pane:** behaves identically to a window.
  tmux runs the pane's shell-command in a pty and destroys the pane when the
  command exits, so the `read -r _ignored` blocking on a non-zero exit keeps
  the pane open with the failure message visible exactly as it held a new
  window open (message may wrap in the narrower 35% pane, but stays fully
  visible). No code change; the wrap is terminal-agnostic shell, already
  asserted by the existing string-based tests.
- **Interaction with TASK-3's replace:** **unconditional** — a pane
  mid-hold-on-failure is killed by a subsequent launch exactly like a live
  session. "Skip" was rejected (breaks the at-most-one-pane invariant and
  needs unfeasible mid-hold detection from outside the pane); "warn" was
  rejected (needs new TUI plumbing, out of scope for this job). The current
  `killPreviousTmuxPane` already implements this — no additional code.
- Manual verification gap: real tmux pane lifecycle (actual hold + replace
  of a mid-hold pane) can't be exercised by pure unit tests; see TASK-4's
  risk note.

## Task breakdown

TASK-1: Record and confirm the scope decisions above (split direction +
size flags, and explicit confirmation of the "replace" policy for repeated
launches) before other tasks start.
     files: docs/jobs/t5oc4j_terminal-emulator/tasks.md (this file)
     depends: none
     risk: low — decision/documentation only, but every other task below
            depends on getting this right; the brief was explicit that
            repeated-launch behavior in particular needs a deliberate
            answer, not an assumed default.
     STATUS: done — see "TASK-1 scope decision (confirmed)" above.

TASK-2: Replace `buildCmd`'s `tmux new-window` call with `tmux
split-window`, using TASK-1's direction/size flags, and update the returned
description string (currently `"tmux window"`) to reflect a pane instead of
a window.
     files: tui/internal/launch/launch.go
     depends: TASK-1
     risk: medium — must confirm `split-window` (with no explicit `-t`)
            targets the TUI's own currently-active pane by default when run
            as a subprocess of the running TUI, so the new pane actually
            lands next to the TUI rather than in an unrelated window/pane.

TASK-3: Implement the "replace" reuse policy from TASK-1: before opening a
new split pane, find and kill any split pane manigot previously opened
(shared across `launch.Agent` and `launch.Quick`), tolerating the case
where that pane was already closed by the user in the meantime.
     files: tui/internal/launch/launch.go (new tracking state/helpers)
     depends: TASK-2
     risk: high — the trickiest part of this job. Needs a reliable way to
            identify "the pane manigot opened last" (e.g. a pane title tag
            plus `tmux list-panes`/`tmux kill-pane`), safe handling of
            races/edge cases (pane already closed manually, in-memory
            tracking state reset by a TUI restart, concurrent launches),
            and goroutine-safety if the tracked state is package-level
            (Agent/Quick can be invoked from Bubble Tea command
            goroutines) — must never kill a pane it didn't itself open.

TASK-4: Confirm `holdOnFailure` still holds a failed launch's pane open and
visible inside a split pane exactly as it does in a window today, and
decide/implement how it interacts with TASK-3's replace-on-relaunch logic
(should a pane currently mid-hold-on-failure be killed by a new, unrelated
launch, or should replace skip/warn in that case?).
     files: tui/internal/launch/launch.go, tui/internal/launch/launch_test.go
     depends: TASK-2, TASK-3
     risk: medium — an easy-to-miss behavioral edge case; tmux pane
            lifecycle can't be fully exercised by pure unit tests, so this
            likely needs some manual verification alongside any automated
            coverage.

TASK-5: Add/update unit tests in `launch_test.go` for the new
split-window command construction (flags/args per TASK-1) and, as far as
feasible, the replace/reuse tracking logic — e.g. via a stubbed `tmux`
script on `PATH` mirroring the existing `TestJdiStartsResolvedCommandDetached`
stub pattern, rather than requiring a real tmux server.
     files: tui/internal/launch/launch_test.go
     depends: TASK-2, TASK-3
     risk: medium — real tmux session behavior (actual pane creation/
            listing/killing) can't be fully verified without a live tmux
            server; a stub can only confirm the commands manigot *runs* are
            correct, not their real effect — call this limitation out in
            the test comments, matching this file's existing documentation
            style.

TASK-6: Update `launch.go`'s doc comments that currently describe
`tmux new-window` (package doc at the top, `Agent`'s spawn-order comment,
`buildCmd`'s numbered list) to describe the split-pane + replace behavior
instead.
     files: tui/internal/launch/launch.go
     depends: TASK-2, TASK-3
     risk: low — documentation only, but this file's comments double as the
            authoritative design record for past decisions (e.g. the
            existing "TASK-1 scope decision", "TASK-5 investigation",
            "TASK-7 review" references), so keep it accurate and in the
            same style.

TASK-7: Update `README.md`'s "Supported platforms" section (currently
"a new **tmux** window, if the TUI is itself running inside tmux") to
describe the split-pane + replace behavior; verify `docs/AGENTS.md` and
`docs/backlog.md` don't make a now-stale claim (the backlog's "In-TUI agent
terminal" entry describes a different, larger, still-deferred feature, so
it likely needs no change — confirm rather than assume).
     files: README.md; docs/AGENTS.md (verify only); docs/backlog.md (verify only)
     depends: TASK-2, TASK-3
     risk: low — targeted documentation updates only.

## Suggested sequencing

TASK-1 first (hard gate — in particular, get explicit confirmation on the
repeated-launch "replace" policy before TASK-3). Then TASK-2 → TASK-3 →
TASK-4, with TASK-5 written alongside TASK-2/TASK-3 as they land rather than
after. TASK-6 and TASK-7 last, once behavior is final.
