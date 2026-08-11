# Implementation: terminal emulator

id: t5oc4j
status: in progress (implementation complete through TASK-7, re-review fixes applied; awaiting @reviewer)
developer: leomuck@posteo.de
date: 2026-08-10

<!-- Produced by @developer after implementation. -->

## Summary

All seven tasks are implemented, including the re-review fixes for the
@reviewer's REJECTED verdict. `buildCmd`'s tmux launch path in
`tui/internal/launch/launch.go` now opens a **split pane** in the TUI's current
tmux window (`tmux split-window -h -p 35`) instead of a brand-new tmux window,
and implements the TASK-1-confirmed **replace** policy for repeated launches:
before each new split, the pane manigot opened last is found and killed (via a
pane-title tag that survives TUI restarts, plus an in-memory pane-id that
survives a running agent retitling the pane), so at most one manigot pane
exists at a time — shared across `launch.Agent` and `launch.Quick`, and
serialized with a mutex against concurrent Bubble Tea command goroutines. Per
the verdict, the pane title tag is applied with `tmux select-pane -t <id> -T
manigot` *after* the split, because `tmux split-window -T` (the original
implementation) does not exist on any released tmux and made every launch fail
outright inside tmux. All other spawn paths (Terminal.app, gnome-terminal,
ptyxis, x-terminal-emulator, konsole, xterm) and `launch.Jdi` are untouched.
`holdOnFailure` was confirmed to behave identically inside a pane, and the
replace interaction was decided as unconditional (a mid-hold pane is killed
like a live session). Full unit-test coverage was added via a stubbed `tmux`
binary on `PATH` (no real tmux server is available in this environment — see
Known issues).

## Changes

TASK-1 (recorded by the prior session): Confirmed the scope decisions in
`docs/jobs/t5oc4j_terminal-emulator/tasks.md` ("TASK-1 scope decision
(confirmed)" section): `tmux split-window -h -p 35` (side-by-side, new pane
gets 35%, TUI keeps 65%), and the **replace** policy for repeated launches —
a single shared tracked pane across `launch.Agent` and `launch.Quick`,
identified via a pane title tag. Adopted from the analyst's own proposals
(grounded in the brief) because no live human confirmation was available;
flagged in Known issues.

TASK-2 (implemented by the prior session, corrected in the re-review): Changed
`buildCmd`'s tmux branch from `tmux new-window` to
`tmux split-window -h -p 35 <inner>` and the returned description from
`"tmux window"` to `"tmux pane"`. The re-review (verdict REJECTED) found the
original construction also passed `-T manigot` to `split-window`, a flag that
exists only in the unreleased 3.8-dev cycle — on every released tmux the
command failed with "unknown option: -T" and no pane was created. `-T` was
removed from `split-window`; the tag is now applied separately with
`tmux select-pane -t <pane_id> -T manigot` (supported by every released tmux)
in `launchTmuxPane` after the split. `TestBuildCmdTmuxUsesSplitWindow` was
updated to the corrected construction.

TASK-3 (implemented by the prior session, corrected in the re-review):
Implemented the replace/reuse policy in `tui/internal/launch/launch.go`:
- `launchDetached` now routes the tmux case ($TMUX set + tmux on PATH) to a new
  `launchTmuxPane`, which runs the split-window client synchronously (it
  returns in milliseconds; the inner command runs in the pane) so the new
  pane's id can be captured.
- `launchTmuxPane` kills the previously manigot-opened pane before splitting
  (best-effort: list/kill failures must not abort a new launch — the
  split-window error below is the accurate one to surface), then runs the
  split, captures the printed pane id, tags the new pane via
  `select-pane -t <id> -T manigot` (best-effort — a tagging failure only
  narrows the restart-surviving lookup and must not abort a launch that
  already succeeded), and records the id. An empty split-window output is
  surfaced as an error rather than silently recorded.
- `killPreviousTmuxPane` identifies the previous pane two ways, both safe
  against killing a pane manigot didn't itself open: the in-memory
  `tmuxLastPaneID` (learned from `split-window -P -F '#{pane_id}'`; reliable
  while this TUI process lives, and covers a running agent retitling the
  pane) and any pane in the current session (`list-panes -s`) whose title is
  `tmuxPaneTag` (set via `select-pane -T`; survives TUI restarts because it
  lives in tmux, not in memory). A pane the user already closed is tolerated
  (kill failure ignored).
- Goroutine safety: a package-level `tmuxMu` serializes the find-kill-split
  sequence and guards `tmuxLastPaneID`, since `Agent`/`Quick` can be invoked
  from Bubble Tea command goroutines.
- `buildCmd`'s tmux branch constructs `tmux split-window -h -p 35 -P -F
  '#{pane_id}' <inner>` (no `-T` — the tag is applied by `launchTmuxPane` via
  select-pane, the single construction site used by both the tests and
  `launchTmuxPane`).

TASK-4: Confirmed `holdOnFailure` behaves identically inside a split pane (tmux
runs the pane's shell-command in a pty and destroys the pane when the command
exits, so the `read -r _ignored` hold keeps a failed pane open with the message
visible exactly as it did a window; the message may wrap in the narrower 35%
pane but stays visible) and decided the replace interaction: **unconditional** —
a pane mid-hold-on-failure is killed by a new launch like a live session
("skip" would break the at-most-one-pane invariant and needs unfeasible
mid-hold detection; "warn" needs new TUI plumbing, out of scope). Recorded in
`holdOnFailure`/`killPreviousTmuxPane` doc comments and in a "TASK-4 decision
(confirmed)" section of `tasks.md`. No code change was needed.

TASK-5 (updated in the re-review): Added a `tmuxStub` test helper (records every
invocation to a log, answers like a minimal tmux using only shell builtins so
it works on a PATH that contains only the stub dir, can simulate
list/split/select-pane/kill failures) plus tests: killPreviousTmuxPane kills
only tracked/tagged panes (never an untagged one), is a no-op with nothing to
replace, tolerates an already-closed pane and a list-panes failure;
launchTmuxPane kills before it splits (order asserted), records the new pane
id, replaces the recorded pane on a second launch, continues after a
list-panes failure, continues after a select-pane tagging failure, and
surfaces a split-window failure; launchDetached routes the tmux case to the
pane path; and `Agent`→`Quick` share one tracked pane (the Quick launch
replaces the pane the Agent launch opened). The re-review corrected the tests
to the fixed command sequence: the split is asserted without `-T`, and
`TestLaunchTmuxPaneReplacesBeforeSplittingAndRecordsPaneID` additionally
asserts the ordering kill-pane → split-window → select-pane and that
select-pane tags only the pane just created (never the previous one).

TASK-6 (updated in the re-review): Updated `launch.go` doc comments that still
described `tmux new-window`/`"tmux window"`: the package doc's spawn-order list
(item 1 now describes the split-pane + replace behavior), `Agent`'s doc comment
(description examples, the synchronous-tmux-path note, and the "inside tmux, a
split pane" opening), `Quick`'s doc comment (cross-referenced to Agent's
behavior), and `buildCmd`'s numbered-list comment. The re-review additionally
corrected the comments that described `split-window -T` tagging
(`tmuxPaneTag`'s const comment, `killPreviousTmuxPane`'s identification list,
`buildCmd`'s tmux-branch comment, `launchTmuxPane`'s doc comment) to the
select-pane mechanism. Historical references to earlier jobs' task numbers
(e.g. "TASK-7 review", "TASK-6's holdOnFailure", "TASK-8/TASK-9 badge/log tab")
were left untouched — they are the file's established authoritative design
record. `tasks.md`'s design-record mentions of `split-window -T` were updated
to `select-pane -t <id> -T` for accuracy.

TASK-7: Updated `README.md`'s "Supported platforms" section — item 1 changed
from "a new **tmux** window, if the TUI is itself running inside tmux" to "a
**tmux** split pane in the TUI's own window, if the TUI is itself running
inside tmux ($TMUX set) — each new launch replaces the pane manigot opened
before, so at most one agent pane exists at a time". Verified `docs/AGENTS.md`
(no tmux/window/pane mentions at all — no change) and `docs/backlog.md` (the
"In-TUI agent terminal (split pane / embedded terminal)" entry describes the
much larger, still-deferred in-TUI PTY rendering feature; its claims that
`launch.go` spawns a terminal per session remain accurate — no change).

## Known issues / follow-ups

- **TASK-1's replace policy was adopted without a live human yes/no.** The
  brief explicitly asked repeated-launch behavior to have "a deliberate answer,
  not a default"; the analyst's proposal (replace) was adopted and recorded
  because it is grounded in the brief's own text, but no interactive user was
  available to confirm. If the user would prefer accumulate-then-manage
  instead, TASK-3's `killPreviousTmuxPane` is the single place to revisit.
- **No real tmux server in this environment** — the unit tests (TASK-5) can
  only verify the commands manigot *runs* are correct (via the stubbed `tmux`
  binary), not their real effect. Runtime verification still needed, per
  TASK-2/TASK-4/TASK-5 risk notes: that `split-window` with no explicit `-t`
  targets the TUI's own pane when run as a subprocess (the $TMUX_PANE
  resolution), that `select-pane -t <id> -T` tags the pane just created, that
  a pane mid-hold-on-failure is actually killed and replaced by a new launch,
  and that the tag/title lookups match real `list-panes` output.
- **Pane-title tag can be overwritten by the running agent** (e.g. Claude Code
  setting the terminal title via OSC sequences). The in-memory `tmuxLastPaneID`
  covers that within one TUI process; the residual gap is a TUI restart *and* a
  retitled pane in the same session, which the tag lookup would then miss
  (accumulation). Rare, documented in `killPreviousTmuxPane`'s comment.
- **`list-panes -s` scopes to the current tmux session** — a manigot pane
  deliberately moved to a *different* tmux session would not be found for
  replacement. Chosen deliberately to avoid killing another session's manigot
  pane; documented in `killPreviousTmuxPane`'s comment.
- **Re-review round (verdict REJECTED → fixed).** The original implementation
  passed `-T` to `tmux split-window`, which no released tmux supports; this
  made every agent launch from inside tmux fail with "unknown option: -T"
  instead of splitting. Fixed by tagging via `select-pane -t <id> -T` after
  the split (TASK-2/TASK-3), updating the stub tests to the corrected
  sequence (TASK-5), and correcting the affected doc comments (TASK-6). The
  `select-pane -T` mechanism is supported by every released tmux version.
- `docs/jobs/t5oc4j_terminal-emulator/tasks.md`'s TASK-2–TASK-7 entries carry
  no `STATUS:` markers (only TASK-1 does); the prior session's pattern, left
  as-is — this file is the record of completion.
