## Summary

Captured the full output of every non-interactive agent invocation (`mg jdi`
/ `--print` runs) and persisted it as a session log in the job's own folder,
so what an agent *did* during an unattended run survives instead of only its
final answer. The claude `--print` path now emits the full step-level
stream-json event stream (which also required `--verbose`), a third parser
branch handles that shape, and the raw output is appended per invocation to
`docs/jobs/<id>_<slug>/session.log`, committed by the normal host-side
sweeps and carried into the archive by `mg done`. Follow-up review fix: the
loop-exit sweep is gated to linked worktrees only, so a pre-worktree job
never sweeps the user's own main-worktree changes onto the job branch, and
the session.log tests run against a real linked worktree plus a new
main-worktree-untouched test.

## Changes

TASK-1: `src/cmd/mg/jdioutput.go` gained `appendSessionLog` (per-section
O_CREATE|O_APPEND|O_WRONLY open, blank-line separator between sections,
`=== <timestamp> <agent> (attempt N) ===` header, raw bytes verbatim,
trailing-newline guarantee). `src/cmd/mg/jdi.go`'s `Run` loop appends every
invocation's raw output — failed ones included — right after `logInvocation`
(best-effort: warns through the log writer, never aborts), and the `finish`
closure now runs a loop-exit sweep (`git.WorktreeForBranch` +
`git.CommitAll` with the `[<id>] chore: commit all` message) so the final
section is committed before `mg done`'s clean-tree check runs; the
stop-before-any-agent path is a swallowed `ErrNothingToCommit` no-op. The
sweep is gated to **linked worktrees only** (reviewer-driven fix): when
`WorktreeForBranch` resolves to the main worktree itself (a pre-worktree job,
branch checked out in the main worktree — an explicitly supported state) the
sweep is skipped, mirroring `session.SweepJobWorktree`'s
`ProjectRoot == InvocationRoot` gate — otherwise it would commit the user's
own uncommitted main-worktree changes, an unexcluded `.env` included, onto
the job branch.

TASK-2: `scripts/entrypoint.sh`'s claude `--print` branch switched from
`--output-format json` to `--verbose --output-format stream-json`. The
`--verbose` flag is required — verified against the installed claude
2.1.247 (`--print --output-format stream-json` without it fails with
"requires --verbose") — and the four stale comment blocks describing the
json format (the `MANIGOT_QUOTE` comment, the opencode branch's
"Claude's ... json 'result' field" reference, and the claude branch's own
comment incl. the stale "confirmed supported by the pinned claude version"
claim) were updated; the version claim was removed since the Dockerfile
installs claude-code unpinned.

TASK-3: `src/internal/orchestrate/signal.go` gained a third parser branch for
Claude's stream-json shape (JSONL of system/assistant/user/result events):
`parseClaudeStream` keys off a type `"result"` event so it can never be
mis-detected as opencode JSONL (and vice versa — opencode events carry
`part.text`, never `message.content`). `ResultText` returns the final result
event's "result" text (falling back to joined assistant text blocks when
empty); `DetectSignal` scans only assistant events' text content blocks,
never tool output (a degenerate lone-result stream keeps the existing
single-result parse, which remains the first, defensive fallback — a stream
with no newlines is covered by it; plain raw stays last). Stale comments
updated.

TASK-4: `src/internal/orchestrate/signal_test.go` gained coverage pinned
against a **real** capture (`claude --print --verbose --output-format
stream-json "say hi"` against claude 2.1.247 — auth fails without
credentials but the full system/assistant/result event stream is emitted,
mirroring how the opencode fixture was pinned): `ResultText` extracts the
result event's text; `DetectSignal` matches a marker in an assistant text
event, scans assistant text rather than the result field, ignores markers in
`tool_use` blocks / user `tool_result` events; the single-result parse still
works as the fallback; a malformed/partial stream falls back to raw; and the
two JSONL shapes are never cross-detected.

TASK-5: `src/cmd/mg/jdioutput_test.go` gained `appendSessionLog` tests
(creates on first use, appends across sections, header carries
agent/attempt/timestamp, raw preserved verbatim, sections separated,
trailing-newline guarantee, empty raw still writes the header).
`src/cmd/mg/jdi_test.go` gained a Run-level test (session.log in `j.Dir`
with exactly one section per invocation, headers + raw in order) plus a
claude stream-json marker-in-assistant-text-event loop test mirroring
`TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL` (stops with StopNeedsHuman,
run.log shows the extracted prose never the raw JSONL, session.log holds the
raw stream). Both include the REQUIRED git-level assertion that
`git status --porcelain` is empty after `Run` — the loop-exit sweep has
committed the final section, so `mg done`'s clean-tree check cannot refuse a
just-finished run. (Reviewer-driven rework: both tests now run against a
REAL linked worktree via the new `initLinkedWorktreeRepo` helper, so the
clean-tree assertion actually exercises the sweep — previously they used the
pre-worktree shape where the sweep is gated off — and a new
`TestRunDoesNotSweepMainWorktree` pins that a pre-worktree job leaves the
main worktree untouched, verified to fail without the TASK-1 gate.)

TASK-6: Synced the format-switch drift: `docs/AGENTS.md` (~line 117) and
`docs/NAMING.md` (~line 114) now name `--output-format stream-json`;
README.md's honesty note (~lines 859-862) now says the full step-level
output is captured to the job's `session.log`, superseding the "final answer
only" limitation; `docs/ROADMAP.md`'s "Event-streaming subsystem" item is
annotated **PARTIALLY addressed** (writer side landed, reader side in the
TUI remains open per the brief's out-of-scope list); and the internal
comments in `src/cmd/mg/jdi.go` (AgentRunner doc) and `src/cmd/mg/jdioutput.go`
(honesty note) now describe stream-json + session.log.

## Known issues / follow-ups

- The claude stream-json event shape is version-volatile; the single-result
  parse is kept as the defensive fallback, and the parser is pinned against a
  capture from the currently installed claude 2.1.247. If a future claude
  version changes or drops stream-json, the fallback covers extraction and
  the `--print` plain-text path covers the rest (per the brief's notes).
- `--verbose` is a hard requirement of `--print --output-format stream-json`
  on 2.1.247; if a future version drops that requirement, the extra flag is
  harmless, but the entrypoint comment's version reference may drift.
- For a pre-worktree job (branch checked out in the main worktree), the final
  `session.log` section is deliberately left uncommitted — the loop-exit
  sweep (and the per-invocation sweep) never touch the main worktree, by
  design; the same trade-off the rest of the host-side sweep machinery
  already makes for that transitional state.
- Rendering/tailing `session.log` anywhere (TUI log tab stays on `run.log`)
  is deliberately out of scope — a separate, later job.