# Brief: capture non-interactive agent sessions

status: done
type: feature
id: health
branch: feature/health_capture-non-interactive-agent-sessions
date: 2026-08-27
author: Leander Muskalla

## What

Capture the full output of every non-interactive agent invocation (`mg jdi` /
`--print` runs) and persist it as a session log in the job's own folder, so
what an agent *did* during an unattended run survives — instead of only its
final answer.

Concretely:

- In the `mg jdi` run loop (`src/cmd/mg/jdi.go`, where the invocation's full
  captured stdout already sits in memory), append that raw output to a session
  log in the job's folder: `docs/jobs/<id>_<slug>/session.log`, one section
  header per invocation (agent, attempt, timestamp) — the same sectioned shape
  `run.log` already uses.
- The session log is committed by the normal host-side sweep-commit, so it
  rides the job's branch and is carried into the archive automatically by
  `mg done`'s archive move. `mg delete` removes it with the job folder.
- Switch the claude `--print` path (`scripts/entrypoint.sh`, the claude-code
  branch) from `--output-format json` to `--output-format stream-json`, so
  claude-pro runs emit step-level events (tool calls, intermediate messages)
  instead of only the final result object.
- Add a third parser branch in `internal/orchestrate/signal.go` for Claude's
  stream-json shape: `ResultText` takes the final result event's text;
  `DetectSignal` scans only assistant text events, never tool output. Keep the
  existing single-result parse as a defensive fallback (stream-json has been
  more version-volatile than the stable result shape).

## Why

In interactive mode the human sees everything the agent does. In unattended
mode (`mg jdi`) that information is thrown away — the run log keeps only each
invocation's final answer, not the blow-by-blow. There is no way to look back
into a session. The goal: capture the whole run now, worry about rendering it
somewhere later.

## Out of scope

- Rendering/tailing the session log anywhere (TUI log tab stays on `run.log`;
  a tailer is a separate, later job).
- Normalizing the raw event stream into a structured format.
- Per-step cost surfacing (the raw capture may contain tokens/cost — that is
  fine, but no UI for it).
- Any change to `mg done` / `mg delete` behavior — the archive move carries
  the session log for free.
- Streaming/incremental reads during a run — buffered post-hoc persistence is
  the v1 shape.

## Notes

- The raw output is already fully captured in memory today at `jdi.go` (the
  `out` buffer from `runner.Run`), then discarded after `logInvocation` /
  `DetectSignal`. This job is mostly a persistence change plus the claude
  format switch.
- OpenCode profiles already emit the full step-level event stream
  (`step_start` / `tool_use` / `text` / `step_finish` JSONL); claude-pro does
  not until the format switch lands. After this job, both capture the session.
- `run.log` (the sidecar summary, human-readable final prose) stays as-is —
  it is the summary; `session.log` is the raw truth. They do not replace each
  other.
- Stream-json version volatility is the main reliability risk: the fallback to
  the existing result-parse must cover the case where a future claude version
  changes or drops the format.
