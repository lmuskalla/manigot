# Tasks: jdi for opencode

id: foycfl
status: open
analyst: '@analyst'
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Investigate, against the exact `opencode-ai` npm version pinned in
`Dockerfile`, whether OpenCode's CLI has a non-interactive / headless
invocation mode equivalent to `claude --print --output-format json` (see
`scripts/entrypoint.sh`'s claude-code branch): a command/flag that runs one
prompt to completion with no TTY, exits on its own, accepts an `--agent`
selection the same way the existing interactive `--prompt`/`--agent` path
does, and returns its final response in a way a caller can parse
programmatically (plain text? a JSON payload with a distinct result field,
like Claude's `{"type":"result","result":...}`?). This is exactly the
"unresolved" item `docs/backlog.md`'s "OpenCode support for mg-jdi" entry and
`scripts/run.sh`'s current `--print`+opencode rejection are both about — no
code change, investigation only, gates every task below. If no such mode
exists at all, say so explicitly instead of guessing a workaround; that would
shrink this job's real scope to "document why it isn't possible" rather than
"implement it."
- files: none (reference: `Dockerfile` for the pinned `opencode-ai` version;
  `opencode --help` / any headless subcommand's own `--help` inside a built
  image; official OpenCode docs, per the precedent `djss4d_opencode-compatibility`
  set of verifying against docs/npm rather than assuming)
- depends: none
- risk: low — read-only investigation; the risk it's meant to catch (no such
  mode, or a shape `--print` callers can't reliably parse) would otherwise
  land silently in TASK-2/TASK-4.

TASK-2: Extend the `opencode` branch of `scripts/entrypoint.sh` to branch on
`MANIGOT_PRINT` the same way the `claude-code` branch already does: when set,
`exec` OpenCode's confirmed non-interactive command/flags (TASK-1) instead of
the current unconditional `exec opencode "$@"`, translating the existing
`--agent`/`--prompt` passthrough into whatever shape that mode expects. The
interactive path (no `MANIGOT_PRINT`) is untouched.
- files: `scripts/entrypoint.sh`
- depends: TASK-1
- risk: medium — first time this path is exercised for OpenCode at all;
  getting the prompt/agent argument translation wrong would silently break
  every `mg jdi` invocation under an opencode profile rather than erroring
  cleanly.

TASK-3: Relax `scripts/run.sh`'s current unconditional rejection of
`--print` for `TOOL == opencode` (the `if [[ "$PRINT" == "true" && "$TOOL"
== "opencode" ]]` block) now that TASK-2 gives it a real implementation —
allow it for both opencode profiles (`zai`, `opencode-go`), and update the
block's own comment (currently states OpenCode's non-interactive invocation
is "unverified" — no longer true once TASK-1/2 land) and its error message.
- files: `scripts/run.sh`
- depends: TASK-1, TASK-2
- risk: medium — this guard exists specifically because the opencode
  `--print` path was unverified; relaxing it before TASK-2 actually lands
  would let a broken/unhandled invocation through with no clear error.

TASK-4: Extend `tui/internal/orchestrate/signal.go`'s output parsing
(`ResultText`/`DetectSignal`, currently written only for Claude's
`--output-format json` shape) to also recognize OpenCode's non-interactive
output shape confirmed in TASK-1, if it differs — without changing behavior
for the existing Claude-JSON and plain-text fallback paths already covered by
`signal_test.go`.
- files: `tui/internal/orchestrate/signal.go`, `tui/internal/orchestrate/signal_test.go`
- depends: TASK-1
- risk: medium — feeds directly into the `NEEDS-HUMAN-INPUT` stop path; a
  false negative here means `mg jdi` could run past a question meant for a
  human, a false positive could stop a run prematurely.

TASK-5: Add a `--profile` flag to the `mg-jdi` binary (`tui/cmd/jdi/main.go`)
and use its value in `commandAgentRunner.Run` instead of the hardcoded
`config.ProfileClaudePro` pin (see that function's own comment explaining why
it's pinned today), validated against `config.Profiles()` and defaulting to
`claude-pro` when unset so existing callers/behavior are unchanged.
- files: `tui/cmd/jdi/main.go`, `tui/cmd/jdi/main_test.go`
- depends: TASK-3 (no point accepting an opencode profile before `mg
  --print --profile zai|opencode-go` actually works)
- risk: medium — changes the CLI's default-behavior contract at its entry
  point; must not alter output for existing claude-pro callers, and existing
  tests likely assert the old hardcoded pin.

TASK-6: Thread the chosen profile through the TUI's own `mg jdi` launch path:
`tui/internal/launch/launch.go`'s `Jdi` function (currently takes no profile
argument at all, unlike `Agent`/`Quick`, which already do) gains a `profile`
parameter passed as `--profile <profile>` to the `mg-jdi` binary, and its one
call site in `tui/internal/ui/app.go` (the `j` keybinding) passes the
current settings profile (`config.Settings.ProfileValue()`, the same source
`Agent`/`Quick` already use) through to it.
- files: `tui/internal/launch/launch.go`, `tui/internal/ui/app.go`,
  `tui/internal/ui/jdilaunch_test.go` (and any other test asserting `Jdi`'s
  current signature)
- depends: TASK-5
- risk: low-medium — mirrors an existing, working pattern (`Agent`/`Quick`
  already plumb `profile` the same way), but touches a launch path with
  existing async/detached-process tests that assert argv today.

TASK-7: Update every place that currently documents `mg jdi` as Claude-Code-
only, once TASK-1–6 land: `docs/AGENTS.md` (canonical source — both its Job
workflow section and its `scripts/run.sh`'s `--print` flag / `mg jdi`
architecture bullets), `README.md` (the tool-comparison table's
"Non-interactive (`--print` / `mg jdi`)" row, the "Autonomous mode" section's
"v1 is Claude Code only" paragraph), `scripts/mg.sh`'s own help text
("reviewer sequence unattended (Claude Code only)"), and `docs/backlog.md`'s
"OpenCode support for `mg-jdi`" entry (remove or mark resolved, matching how
prior jobs retire a backlog item once it's actually built). Do not edit the
read-only context mounts (`/workspace/AGENTS.md`, `/workspace/.claude/CLAUDE.md`)
if this job is ever run against manigot's own dogfooded checkout — only the
canonical `docs/AGENTS.md` source.
- files: `docs/AGENTS.md`, `README.md`, `scripts/mg.sh`, `docs/backlog.md`
- depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5, TASK-6 (docs should
  describe what was actually built, including if TASK-1 concludes full
  support isn't possible)
- risk: low — documentation only, no behavior change.

TASK-8: Manually verify end to end, after a `make rebuild` (new/changed
`scripts/entrypoint.sh` logic) and `make jdi`: run `mg jdi --job <id>
--profile zai` (or `opencode-go`) against a small real or throwaway test
job and confirm it drives `@analyst` → `@developer` → `@reviewer` the same
way the existing claude-pro path does, that a deliberately-triggered
`NEEDS-HUMAN-INPUT:` case is still detected correctly (TASK-4), and that the
existing claude-pro path is unaffected. Verification only, not a code change.
- files: none (exercises `scripts/run.sh`, `scripts/entrypoint.sh`, and
  `tui/cmd/jdi` end to end)
- depends: TASK-1 through TASK-7
- risk: low as an activity, but it's the task that actually confirms the
  feature works — a failure here means an earlier task needs rework.

## Open questions for review

- TASK-1 is a real gate, not a formality: if the pinned `opencode-ai`
  version turns out to have no viable non-interactive/headless mode at all,
  TASK-2 through TASK-6 cannot proceed as scoped and this job's real
  deliverable becomes documenting that limitation (TASK-7 alone) rather than
  implementing support. Flagging this now rather than assuming TASK-1
  succeeds.
- The brief's `Why` and `Out of scope` sections are blank. Scope above is
  read as "bring `mg jdi` to full parity across all three profiles" (the
  most natural reading of "now it's time to make this work with opencode"),
  not e.g. "just one of the two opencode profiles" — worth confirming during
  review rather than assuming further.
