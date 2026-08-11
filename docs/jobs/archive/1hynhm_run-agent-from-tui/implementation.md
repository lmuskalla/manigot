# Implementation: run agent from tui

id: 1hynhm
status: open
developer: claude
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

Added a jobless "pick any agent, launch it as a quick session" picker
natively inside the TUI's dashboard, reached with a new `a` key next to the
existing `o` (bare quick session). It mirrors `mg agents`
(`scripts/agents.sh`)'s discovery/precedence rules but as a native Bubble Tea
view rather than shelling out to the interactive script.

## Changes

TASK-1: Added `tui/internal/agentlist` (`agentlist.go` +
`agentlist_test.go`). `Discover(projectRoot string) ([]Agent, error)` lists
every agent available to the project: the manigot checkout's global
`agents/*.md` (sorted by name), each swapped for its
`docs/agents/<name>.md` override in `projectRoot` when one exists, followed
by any project-only `docs/agents/` additions that don't shadow a global
name — the same order/precedence `scripts/agents.sh` uses. The checkout root
is resolved via `resolve.Home()` (the existing
`$MANIGOT_HOME`-then-executable-location logic), so no second resolution
strategy was introduced; a checkout that can't be found is a returned error,
not a panic or a silent empty list. `Agent.Description` mirrors
`scripts/agents.sh`'s `describe()` (first `description:` frontmatter line,
falling back to `"(no description)"`).

TASK-2: Added `launch.AgentQuick(agent, projectRoot, profile string)
(string, error)` to `tui/internal/launch/launch.go`, plus its own
`agentQuickShellCommand` builder (kept separate from `shellCommand`/
`quickShellCommand` per the file's existing "deliberately a separate
function" convention for exact-format test stability) and matching tests in
`launch_test.go`. Same spawn mechanism (`launchDetached`/tmux-pane path,
`holdOnFailure` wrap) as `Agent`/`Quick`, invoking `--profile <profile>
--agent <agent>` with no `--job`. No container- or `scripts/run.sh`-side
change was needed — it already treats `--agent` and `--job` as independent
optional flags.

TASK-3: Added `tui/internal/ui/agentspicker.go` (+ `agentspicker_test.go`):
a new `agentsPickerView` — a pure input component (name/description rows,
`↑`/`↓`/`k`/`j`/`home`/`end` to move, `enter` to submit, `esc` to cancel),
following the same style as `newJobView`/`settingsView`. Wired into
`tui/internal/ui/app.go`: a new `stateAgents` app state, `agentsPicker`
field, `Update`'s `tea.WindowSizeMsg` resize and `tea.KeyMsg` state routing,
`View`'s render dispatch, a new `updateAgentsPicker` handler (cancel returns
to the list; submit calls `launch.AgentQuick` with
`a.settings.ProfileValue()` and reports the outcome in the footer status the
same way `o` already does — `"→ " + agent + " in " + desc`), a new `a` case
in `updateList` that calls TASK-1's `agentlist.Discover` against `a.root`
and either opens the picker or degrades a discovery error / empty list to a
status line (never opens a picker with nothing to show), and an updated
footer key hint.

TASK-4: Updated `README.md`'s TUI section — the list view Keybindings table
(new `a` row) and the "Supported platforms" prose (parallel note describing
what `a` opens, alongside the existing `o` note) — to document the new
picker.

## Known issues / follow-ups

- The brief's "Why" and "Out of scope" sections were left empty by the user;
  nothing encountered during implementation depended on filling them in, but
  they're still worth a follow-up pass if the reasoning ever needs to be on
  record.
- `gofmt -l` flags a pre-existing formatting issue in
  `tui/internal/ui/settings.go` (unrelated to this job — not touched by any
  of the four tasks); left alone rather than fixed as an out-of-scope drive-by.
- `launch.AgentQuick`'s success path (an actual spawned terminal/tmux pane)
  is exercised only indirectly, the same way `Quick`/`Agent` already are
  (string-format tests on the shell-command builder, plus the shared
  `launchDetached`/tmux-pane tests) — no UI-level test spawns a real
  terminal end to end, matching this codebase's existing test coverage for
  the "o" key and agent-launch keys already in the detail view.
