# Tasks: run agent from tui

id: 1hynhm
status: open
analyst: claude
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Context

The TUI's list view (dashboard) already has a jobless "quick session" launch
(`o` key → `launch.Quick`, no `--agent`/`--job`, opened in a new terminal/tmux
pane — see `tui/internal/ui/app.go`'s `updateList` and
`tui/internal/launch/launch.go`'s `Quick`). The host CLI already has an
equivalent agent-picker, `mg agents` (`scripts/agents.sh`): it lists every
agent available to the project (global `agents/*.md`, each optionally
shadowed by a same-named `docs/agents/` override, plus any project-only
`docs/agents/` additions), prompts for a numbered selection, then execs
`run.sh --agent <name>` with no `--job`.

The brief asks for that same "pick any agent, launch it as a quick session"
capability surfaced natively inside the TUI's dashboard, rather than only
being reachable by dropping to a shell and running `mg agents`. This mirrors
the existing native-form pattern the TUI already uses for other host actions
(`newJobView`/`stateNewJob` for `mg job`, `settingsView`/`stateSettings` for
settings) rather than shelling out to the interactive `mg agents` script
itself.

The brief's "Why" section is empty and out-of-scope isn't filled in either;
nothing here turns on that gap (the "What" is concrete enough to break into
tasks), but @developer/@product-owner should flag it if a hidden constraint
turns up during implementation.

## Task breakdown

TASK-1: Add a new Go package that discovers every agent available to the
current project — the global `agents/*.md` shipped in the manigot checkout,
each swapped for its `docs/agents/<name>.md` override when one exists, plus
any project-only `docs/agents/` additions appended after — returning each
agent's name and one-line `description:` frontmatter value, in the same
order `scripts/agents.sh` presents them. Needs to resolve the manigot
checkout root (the global `agents/` dir lives there, not in the target
project) — reuse `tui/internal/resolve`'s existing checkout-root resolution
(`resolve.Home()`/its `$MANIGOT_HOME`-then-executable-location logic) rather
than inventing a second one.
     files: new `tui/internal/agentlist/agentlist.go`, new
       `tui/internal/agentlist/agentlist_test.go`
     depends: none
     risk: medium — the global agents directory lives outside the mounted
       project root, in the manigot checkout the TUI binary was built/run
       from; getting that resolution wrong (or not degrading gracefully when
       it can't be found — e.g. a binary copied somewhere `resolve.Home()`
       can't place) breaks the picker for some install layouts even though
       it looks fine in a dev checkout.

TASK-2: Add `launch.AgentQuick(agent, projectRoot, profile string) (string,
error)` alongside the existing `Agent`/`Quick` functions: same spawn
mechanism (tmux pane / Terminal.app / Linux terminal emulator, same
hold-on-failure wrap) as `Quick`, but with `--agent <agent>` added to the
invocation and no `--job`. Add its own shell-command builder (mirroring
`shellCommand`/`quickShellCommand`) rather than overloading either existing
one, per the file's existing "deliberately a separate function" convention
for exact-format test stability.
     files: `tui/internal/launch/launch.go`, `tui/internal/launch/launch_test.go`
     depends: none
     risk: low — closely follows the existing `Agent`/`Quick` pattern and
       their existing string-based tests; `scripts/run.sh` already accepts
       `--agent` without `--job` (confirmed by reading it — no container- or
       script-side change needed).

TASK-3: Add a new app state (`stateAgents`) and picker view (agent name +
description list, `↑`/`↓`/`k`/`j` to move, `enter` to launch, `esc` to
cancel) wired into `App`: `Update`'s `tea.WindowSizeMsg` resize handling and
`tea.KeyMsg` state routing, `View`'s render dispatch, and a new `a` key in
`updateList` (dashboard) that builds the picker from TASK-1's discovery
against `a.root` and opens it. Selecting an agent calls TASK-2's
`launch.AgentQuick` with `a.settings.ProfileValue()` and reports the outcome
in the footer status the same way `o` (quick session) already does
(`"→ " + agent + " in " + desc`), then returns to the list. A discovery
failure (TASK-1 error) or an empty agent list surfaces as a status message
instead of opening the picker, the same "degrade to a status line, never
crash" convention every other host-command error already follows in this
file (`cmdErrorText`).
     files: new `tui/internal/ui/agentspicker.go`, new
       `tui/internal/ui/agentspicker_test.go`, `tui/internal/ui/app.go`
       (new `appState`, `Update`/`View`/resize wiring, `updateList`'s `a`
       case, footer hint text)
     depends: TASK-1, TASK-2
     risk: medium — touches `App`'s central state machine
       (`Update`/`View`/`WindowSizeMsg`), which every other view
       (list/detail/newJob/settings) also routes through; must not regress
       their existing key handling or resize behavior.

TASK-4: Update `README.md`'s TUI section — the list view's Keybindings
table (new `a` row) and the "Supported platforms" prose describing what `o`
opens (parallel note for `a`) — to document the new picker.
     files: `README.md`
     depends: TASK-3
     risk: low — documentation only, no behavior change.
