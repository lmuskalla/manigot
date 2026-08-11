# Verdict: run agent from tui

id: 1hynhm
status: reviewed
reviewer: claude
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `tui/internal/agentlist/agentlist.go` discovers global `agents/*.md`
(sorted), applies `docs/agents/<name>.md` overrides, and appends
project-only additions — order/precedence matches `scripts/agents.sh`.
`resolve.Home()` is reused for checkout-root resolution (no second lookup
strategy introduced). Missing checkout / missing global dir / empty result
all return errors rather than panicking or silently returning an empty list.
Description parsing mirrors the shell script's `sed`-based `describe()`,
including the "(no description)" fallback. Tests cover global-only, project
override, project-only addition, no-description fallback, empty
projectRoot, unresolvable checkout, and missing global dir — all pass
(`go test ./...` green).

TASK-2: PASS
notes: `launch.AgentQuick` in `tui/internal/launch/launch.go` follows the
exact `Agent`/`Quick` pattern (same `resolve.Resolve`/`launchDetached`
flow), with its own `agentQuickShellCommand` builder (not overloading
`shellCommand`/`quickShellCommand`), per the file's existing convention.
Empty profile defaults to `claude-pro`, matches `--agent <agent>` with no
`--job`. Tests mirror the existing `shellCommand`/`quickShellCommand` string
tests (format, --job omission, empty-profile default, non-default profile,
path-with-spaces quoting).

TASK-3: PASS
notes: New `stateAgents` state and `agentsPickerView`
(`tui/internal/ui/agentspicker.go`) wired into `App`'s `Update`
(`WindowSizeMsg` resize + `KeyMsg` routing) and `View` dispatch, plus a new
`a` case in `updateList` and `updateAgentsPicker` handler in `app.go`. `a`
does not collide with any existing keybinding. A discovery error or empty
agent list degrades to a footer status line without opening the picker,
matching the file's `cmdErrorText` convention. Submit reports "→ <agent> in
<desc>" the same way `o` does; cancel/submit both return to `stateList` and
clear `agentsPicker`. Footer hint updated. Tests cover navigation
(including boundary clamps, home/end), cancel/submit actions, `selected()`,
rendering, the `a`-key wiring (success + discovery-failure paths), esc
cancel, submit resolution-failure reporting, and the footer hint text — all
pass.

TASK-4: PASS
notes: `README.md` Keybindings table gains an `a` row and the prose above it
gains a parallel note describing what `a` opens, alongside the existing `o`
note. Documentation-only, matches the described behavior.

## Verification performed

- `go build ./...` — clean
- `go test ./...` (all `tui` packages) — all pass, including the new
  `agentlist`, `launch`, and `ui` tests
- `go vet ./...` — clean
- `gofmt -l .` — only flags the pre-existing, unrelated
  `tui/internal/ui/settings.go` issue called out (and left untouched) in
  `implementation.md`; nothing in this job's own changed files is
  unformatted
- Read `scripts/agents.sh` directly and compared its discovery
  order/precedence and `description:` extraction against
  `agentlist.Discover`/`readDescription` — they match
- `git diff main...HEAD --stat` reviewed in full; no changes outside the
  four tasks' stated files plus the job's own `docs/jobs/1hynhm_.../` files
- Commits: one commit per task in `[1hynhm] TASK-N: ...` format, plus a
  separate `[1hynhm] implementation: add summary` commit — matches the
  required commit discipline

## Security

None — no new external input parsing beyond local filesystem reads under
paths the TUI already trusts (manigot checkout, project `docs/agents/`), no
new subprocess argument comes from unsanitized user input beyond the agent
name, which is drawn from a filename already used the same way by the
existing `Agent`/`Quick` launch paths and shell-quoted identically.

## Overall

APPROVED

No blockers found. All four tasks are implemented as specified in
tasks.md, match the existing codebase conventions they were asked to
mirror (`scripts/agents.sh`'s discovery/precedence, `Agent`/`Quick`'s launch
pattern, `newJobView`/`settingsView`'s picker-view style), are covered by
tests that pass, and the diff contains no out-of-scope changes.
