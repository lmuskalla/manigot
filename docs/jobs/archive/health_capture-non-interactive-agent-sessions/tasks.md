# Tasks: capture non-interactive agent sessions

id: health
status: open
analyst: health
date: 2026-08-27

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

<!-- TASK-1: Append each invocation's raw captured output to the job's own session.log (docs/jobs/<id>_<slug>/session.log, i.e. filepath.Join(j.Dir, "session.log")) from inside the mg jdi Run loop, with a per-invocation section header carrying agent + attempt + timestamp in the same "=== ... ===" sectioned shape run.log uses, writing the section for every invocation including failed ones (right after logInvocation, before the DetectSignal/runErr early returns), best-effort (warn on failure through the log writer — Run has no stderr — never abort).
     **Required, corrected from the earlier draft: the run must leave the worktree clean.** Because the per-invocation sweep (session.SweepJobWorktree inside commandAgentRunner.Run) runs BEFORE the loop's append, the final section would otherwise stay an uncommitted tracked modification: session.log is untracked when iteration 1 appends it, iteration 2's sweep `git add -A` commits it (now TRACKED), every later append is a tracked modification, and the final section is never swept — git.WorkingTreeDirty (git.go:816) DOES report tracked modifications, so mg done's clean-tree check (finish.go:151-155) refuses to finish the job. Add a loop-exit sweep that commits the final section on every exit path: resolve the worktree via git.WorktreeForBranch(j.Root, j.Branch) — resolves the main worktree too, so tests without a linked worktree work — and git.CommitAll with the sweep's own "[<id>] chore: commit all" message, wired into the finish closure (which every return path, including the DetectSignal/runErr early returns and the maxIterations exit, already goes through; the stop-before-any-agent path is a harmless no-op via swallowed ErrNothingToCommit). The per-invocation sweeps keep doing their existing job (agent leftovers + prior sections); the loop-exit sweep covers only the final section.
     files: src/cmd/mg/jdi.go (the Run loop, after logInvocation, + the loop-exit sweep), src/cmd/mg/jdioutput.go (a small appendSessionLog-style helper: O_CREATE|O_APPEND|O_WRONLY open per section, blank-line separator + header + raw bytes, ensure a trailing newline so the next section's header never glues to raw output), src/cmd/mg/jdi_test.go, src/cmd/mg/jdioutput_test.go
     depends: none
     risk: medium — central loop change; the sweep-ordering requirement (final section committed before mg done) is the fiddly part and must be pinned by a test (see TASK-5)

TASK-2: Switch the claude --print invocation in scripts/entrypoint.sh (line 344) from --output-format json to --output-format stream-json, and update the four stale comment blocks in that file that describe the json format (the MANIGOT_QUOTE comment at ~line 268, the opencode --print comment's "Claude's --output-format json result field" reference at ~line 294, the claude --print branch's own comment at ~lines 337-343 — which also claims "confirmed supported by the pinned claude version", stale since the Dockerfile line 49 installs @anthropic-ai/claude-code unpinned). Verified: agents/*.md and project-template/docs/AGENTS.md have no --print/output-format references, so no sync needed there.
     files: scripts/entrypoint.sh
     depends: none
     risk: high — claude-code is installed unpinned in the Dockerfile; a version that rejects --output-format stream-json would break every claude-pro --print run, so the developer must verify the flag against the installed version (claude --help / a smoke invocation in the container) and keep the change limited to the --print branch so interactive sessions are untouched

TASK-3: Add a third parser branch to internal/orchestrate/signal.go for Claude's stream-json shape (JSONL of typed events: system/assistant/user/result; assistant events carry message.content blocks of type "text" or "tool_use"; the final event is type "result" with a "result" field): ResultText returns the final result event's text; DetectSignal scans only assistant events' text content blocks, never tool output; keep the existing single-result parse first as the defensive fallback and plain-raw last; the claude-stream branch must key off a type "result" event so it cannot be mis-detected as opencode JSONL (opencodeResultText requires every line to parse and at least one part.type=="text" event, which claude streams never satisfy — text lives in message.content, not part — verified against signal.go:96-121); update the file's stale --output-format json comments (lines ~23, 50, 55, 59, 70).
     files: src/internal/orchestrate/signal.go
     depends: none (parser can land before or after TASK-2; both are needed for the full feature)
     risk: medium — stream-json is version-volatile; must not regress the existing single-result or opencode JSONL paths (the single-result parse already handles a lone {"type":"result","result":...} line, so a stream with no newlines is covered by the fallback)

TASK-4: Add signal_test.go coverage for the stream-json branch: a shape fixture (documented-shape; a real claude-pro capture may not be obtainable without credentials, mirroring how the opencode fixture was pinned); ResultText extracts the final result event's text; DetectSignal matches a marker in an assistant text event; DetectSignal does not match a marker inside a tool_use content block or a user tool_result; the single-result parse still works as the fallback; a malformed/partial stream falls back to raw; opencode JSONL is not mis-detected as claude stream and a claude stream is not mis-detected as opencode JSONL.
     files: src/internal/orchestrate/signal_test.go
     depends: TASK-3
     risk: low-medium — the fixture is the only uncertain piece; a documented-shape fixture is the acceptable fallback if no real capture is possible

TASK-5: Add loop-level and helper tests for the session.log persistence: jdioutput_test.go for the append helper (creates on first use, appends across sections, header carries agent/attempt/timestamp, raw bytes preserved verbatim, sections separated from each other, trailing newline guarantee); jdi_test.go for a Run-level test with the fake runner (session.log exists in j.Dir with exactly one section per invocation, headers + raw content in order) and a claude stream-json marker-in-assistant-text-event loop test mirroring TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL (loop stops with StopNeedsHuman, run.log shows the extracted prose, never the raw JSONL); REQUIRED (not optional — pins TASK-1's loop-exit sweep): a git-level assertion after Run returns that the worktree is clean — `git status --porcelain` empty (or `git diff --quiet` passes), i.e. the final section was committed by the loop-exit sweep, so mg done's clean-tree check cannot refuse a just-finished run.
     files: src/cmd/mg/jdioutput_test.go, src/cmd/mg/jdi_test.go
     depends: TASK-1, TASK-3
     risk: low-medium — test plumbing only; the sweep-ordering and section-separation assertions are the fiddly parts

TASK-6: Sync the user-facing docs and internal comment drift for the format switch: docs/AGENTS.md (~line 117) and docs/NAMING.md (~line 114) name --output-format json; README.md's honesty note (~lines 859-862) claims claude --print returns only the final response text, which the new session.log capture supersedes; docs/ROADMAP.md's "Event-streaming subsystem" item (~lines 99-108, "replacing the README's honest final answer only limitation") is this job's writer side but should be annotated PARTIALLY addressed, not marked done — the item's "reader side in the TUI" remains open per the brief's out-of-scope list (TUI log tab stays on run.log); and the internal comments describing the format in src/cmd/mg/jdi.go (~line 299, the AgentRunner doc) and src/cmd/mg/jdioutput.go (~lines 238 and 249-254, the honesty note) need updating to mention stream-json + session.log. All cited lines verified against the current files.
     files: docs/AGENTS.md, docs/NAMING.md, README.md, docs/ROADMAP.md, src/cmd/mg/jdi.go, src/cmd/mg/jdioutput.go
     depends: TASK-2 (doc content follows the switch)
     risk: low — pure documentation/comments; keep each edit to the single stale sentence or block

-->