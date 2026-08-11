# Implementation: jdi for opencode

id: foycfl
status: open
developer: '@developer'
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

Brought `mg jdi` to parity across all three subscription profiles
(`claude-pro`, `zai`, `opencode-go`) by giving OpenCode a real non-interactive
invocation, wiring it through `scripts/entrypoint.sh`/`scripts/run.sh`,
teaching `tui/internal/orchestrate` to parse its output shape, and exposing a
`--profile` choice at both `mg jdi`'s CLI and the TUI's `j` keybinding.

## Changes

TASK-1 (investigation, no code change): Confirmed OpenCode (`opencode-ai`,
installed unpinned by the Dockerfile — `1.18.16` at investigation time) has a
real headless mode: `opencode run [message..] --agent <agent> --format json`.
It takes the prompt as a positional argument (not a flag, unlike the
interactive `opencode [project]` command's own `--prompt`), runs to
completion with no TTY, honors `--agent` the same way the interactive path
does, auto-executes tool calls with no permission prompt (verified live —
`bash`/`write` tool calls completed without `--auto`), and exits 0 on
success. `--format json` output is **not** a single JSON object like Claude's
`--output-format json` — it's JSONL, one JSON object per line, a stream of
typed events (`step_start`, `tool_use`, `text`, `step_finish`, ...). Only
`"text"`-typed events carry the assistant's actual response prose, in
`part.text`; warnings (e.g. "agent not found") go to stderr, keeping stdout
clean. All of this was verified with live invocations against the real
`opencode` binary in this environment, not just documentation.

TASK-2: `scripts/entrypoint.sh`'s `opencode` branch now branches on
`MANIGOT_PRINT` like the `claude-code` branch already did. When set, it
parses the incoming `--agent`/`--prompt` pair out of `"$@"` and re-emits them
as `opencode run <prompt> --agent <agent> --format json`. The interactive
path (no `MANIGOT_PRINT`) is untouched.

TASK-3: `scripts/run.sh`'s `--print`+opencode rejection now only fires for
the legacy, profile-less `--tool opencode` path (never in scope here);
`--print` is allowed for the `zai` and `opencode-go` profiles. Updated the
block's comment and error message.

TASK-4: `tui/internal/orchestrate/signal.go` gained `opencodeResultText`,
tried after the existing Claude-JSON-object parse and before the plain-text
fallback: it requires every non-blank line to parse as a JSON object with a
`type` field (JSONL), and concatenates every `"text"`-event's `part.text` as
the response. Falls straight through to the untouched plain-text path if any
line doesn't fit that shape. New tests cover the OpenCode-JSONL match,
no-match, non-text-event, and malformed-line-fallback cases; all existing
Claude-JSON and plain-text tests pass unchanged.

TASK-5: `tui/cmd/jdi/main.go` gained a `--profile` flag (validated against
`config.Profiles()`, default `claude-pro`), threaded through
`newCommandAgentRunner`/`commandAgentRunner.Run` in place of the previous
hardcoded `config.ProfileClaudePro` pin.

TASK-6: `tui/internal/launch/launch.go`'s `Jdi` gained a `profile` parameter
(empty defaults to `claude-pro`, matching `Agent`/`Quick`), passed as
`mg-jdi`'s own `--profile` flag. Its one call site in `tui/internal/ui/app.go`
now passes `a.settings.ProfileValue()`. Updated `launch_test.go`'s existing
`Jdi` calls for the new signature and added coverage for both the default and
an explicit non-default profile being forwarded.

TASK-7: Updated every place documenting `mg jdi` as Claude-Code-only:
`docs/AGENTS.md` (the `entrypoint.sh` bullet, the `--print` flag bullet, the
`mg jdi` command bullet, and the Job workflow section), `README.md` (the
tool-comparison table's Non-interactive row, and the Autonomous mode
section), `scripts/mg.sh`'s help text, and `docs/backlog.md` (removed the now
-resolved "OpenCode support for `mg-jdi`" entry). The mounted read-only
`.claude/CLAUDE.md` overlay was left untouched, as required.

TASK-8 (verification, no code change): `make jdi` builds cleanly. `go build
./...`, `go vet ./...`, and `go test ./...` all pass. A full `mg jdi --job
<id> --profile zai` run through the real container could not be exercised in
this sandbox — no `docker` binary is available here (see Known issues).
Instead, verified the actual pieces that make that path work, live, against
the real `opencode` CLI already installed in this environment (same
`opencode-ai` version the Dockerfile installs): reproduced
`entrypoint.sh`'s exact `--agent`/`--prompt` → `opencode run ... --format
json` translation by hand and ran it for real; fed the captured raw output
through the actual (not fabricated) `orchestrate.DetectSignal`/`ResultText`
functions for both a deliberately-triggered `NEEDS-HUMAN-INPUT:` case (a live
prompt instructing the agent to print the marker) and a normal response —
both parsed correctly (marker detected with the right reason text; the
normal response produced no false positive). The claude-pro path's own
`entrypoint.sh`/`run.sh` lines are untouched by this job (confirmed by diff
against the pre-job commit), so it's unaffected.

## Known issues / follow-ups

- TASK-8's end-to-end verification could not include an actual `mg jdi --job
  <id> --profile zai` run through the real Docker container — this sandbox
  has no `docker` binary. What *could* be verified (the exact command
  translation and the real OpenCode output being parsed correctly by the
  real Go code) was verified live instead; a human with a working `mg`
  install should still do one real `mg jdi --job <id> --profile zai|opencode-go`
  run before relying on this in production, per TASK-8's own scope.
- The README's caveat that OpenCode agents aren't tool-restricted the way
  Claude Code's are (the `tools:`-stripping caveat under Agents) is
  unrelated to this job and was left as-is.
