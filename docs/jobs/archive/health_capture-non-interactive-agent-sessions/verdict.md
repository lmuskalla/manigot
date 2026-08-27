# Verdict: capture non-interactive agent sessions

id: health
status: open
reviewer: '@reviewer'
date: 2026-08-27

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `appendSessionLog` (src/cmd/mg/jdioutput.go:299) is correct: per-section
O_CREATE|O_APPEND|O_WRONLY open, blank-line separator only on non-empty files,
`=== <RFC3339> <agent> (attempt N) ===` header, raw bytes verbatim with a
trailing-newline guarantee, best-effort error path (warns through the log
writer, never aborts). Loop placement (src/cmd/mg/jdi.go:583, right after
`logInvocation`, before the DetectSignal/runErr early returns) appends for
every invocation including failed ones, per spec. The prior BLOCKER is
resolved: the loop-exit sweep in the `finish` closure (jdi.go:491-498) is now
gated — `git.WorktreeForBranch(j.Root, j.Branch)` resolving to the main
worktree (pre-worktree job) skips the sweep entirely
(`filepath.Clean(wt) != filepath.Clean(j.Root)`), mirroring
`session.SweepJobWorktree`'s `ProjectRoot == InvocationRoot` gate; the
stop-before-any-agent path is a swallowed `ErrNothingToCommit`/`ErrNotARepo`
no-op. Sweep-ordering analysis in the comment is accurate and consistent with
`WorkingTreeDirty` reporting tracked modifications but not untracked files
(git.go:811-835) — which is also why the pre-worktree case leaving the final
section uncommitted is harmless for `mg done`. The `append(raw, '\n')`
mutation is safe (out is not reused after the call in Run).

TASK-2: PASS
notes: scripts/entrypoint.sh's claude `--print` branch now execs
`claude --dangerously-skip-permissions --print --verbose --output-format
stream-json` (line 353); the `--verbose` requirement (verified against
2.1.247 per implementation.md, consistent with the real-shape fixture pinned
in signal_test.go) and the removal of the stale "confirmed supported by the
pinned claude version" claim (Dockerfile installs unpinned) are right. All
four stale comment blocks updated (MANIGOT_QUOTE ~268, opencode branch ~294,
claude branch ~337-352, plus the exec line). Interactive branch untouched.
Could not re-verify the flag live (no docker/credentials in this sandbox) —
verification is the pinned real-shape fixture plus full read-through, the
same position as the prior review.

TASK-3: PASS
notes: `parseClaudeStream` (signal.go:147) keys off a type-"result" event so
claude streams can never be mis-detected as opencode JSONL (opencode emits
step_start/tool_use/text/step_finish only — verified against the real
fixture) and vice versa (claude events carry message.content, never
part.text). `ResultText` tries single-result first (defensive fallback),
opencode second, claude-stream third, plain raw last; the stream branch
returns the final result event's text, falling back to joined assistant text
when empty. `DetectSignal` scans only assistant text content blocks — never
tool_use blocks or user tool_result events — via the
`len(stream.assistant) > 0` override, so a degenerate lone-result stream
keeps the result-field scan. Stale comments updated. All-or-nothing parse
failure on a future string-typed message.content degrades to the raw
fallback — the designed, documented version-volatility answer.

TASK-4: PASS
notes: signal_test.go pins against a real capture
(`claude --print --verbose --output-format stream-json "say hi"` on 2.1.247,
auth-failed but full system/assistant/result stream — internally consistent:
claude_code_version 2.1.247, authentication_failed). Coverage matches the
checklist: ResultText extracts the result event's text; DetectSignal matches
a marker in an assistant text event (the JSON-escaped `\n` in the fixture
correctly decodes to a line-start marker post-parse); a marker in a tool_use
block or user tool_result is not matched; the single-result parse still works
as the fallback; a malformed stream falls back to raw; and the two JSONL
shapes are never cross-detected in either direction.

TASK-5: PASS
notes: Helper tests (jdioutput_test.go) cover create/append/header
(agent+attempt+RFC3339 timestamp)/verbatim-preservation/blank-line
separation/trailing-newline guarantee/empty-raw. The prior PARTIAL is
resolved: the two Run-level tests now run against a REAL linked worktree via
`initLinkedWorktreeRepo` (production shape: `git worktree add`), so the
REQUIRED `git status --porcelain`-empty assertion actually exercises the
loop-exit sweep — in `TestRunWritesSessionLogPerInvocation` (one section per
invocation, headers + raw in order, then clean) and in
`TestRunStopsOnNeedsHumanMarkerInClaudeStreamJSON` (marker stop on the FIRST
invocation, where only the loop-exit sweep can have committed the section;
run.log shows extracted prose, never raw JSONL; session.log holds the raw
stream). `TestRunDoesNotSweepMainWorktree` plants an untracked `.env` in the
main worktree on the pre-worktree shape and asserts it stays untracked with
content untouched and no `chore: commit all` on the branch — pins the TASK-1
gate; the session.log section staying uncommitted there is the accepted,
documented trade-off. Test plumbing verified (runGit/mgGit/commit/fakeRunner/
assertLogOrder/sectionHeaderCount all exist; imports complete).

TASK-6: PASS
notes: docs/AGENTS.md (~117) and docs/NAMING.md (~114) name
`--output-format stream-json`; README.md's honesty note (~856-869) now
describes the session.log capture superseding the "final answer only"
limitation; docs/ROADMAP.md item 5 annotated **PARTIALLY addressed** (writer
side landed, reader side in the TUI remains open — matches the brief's
out-of-scope list); jdi.go's AgentRunner doc and jdioutput.go's honesty note
updated. Grepped the tree: the only remaining `--output-format json`
references are intentional (fallback descriptions in signal.go, the
single-result fixture comment, and archived-job docs). agents/*.md and
project-template/docs/AGENTS.md have no --print/output-format references —
verified, no sync needed.

## Security

No new secrets/credentials handling. The raw session.log capture may contain
token/cost fields from the event stream — disclosed in the brief as
acceptable, no UI for it. The prior security blocker (loop-exit sweep
committing the user's unexcluded `.env` onto the job branch on a pre-worktree
job) is resolved by the linked-worktree-only gate (jdi.go:494), pinned by
`TestRunDoesNotSweepMainWorktree`. The session.log capture itself is written
inside the job's own worktree/`docs/jobs/<job>/` and committed there; the
`.manigot/jdi-status/` sidecar is never touched by the sweep (it lives in the
main project root, outside the linked worktree).

## Overall

APPROVED

Non-blocking observations (no change required; recorded for the follow-up):

1. On a non-git project (the discoverWorkingTree fallback), every mg jdi run
   now ends with a `mg jdi: warning: could not resolve worktree for the final
   session-log sweep: not a git repository (or git not installed)` line —
   truthful and harmless (session.log still appends; there is simply no git to
   commit it into), but worth considering a silent skip for `ErrNotARepo` in
   the `WorktreeForBranch` call if it proves noisy.
2. `parseClaudeStream` is all-or-nothing: a future claude shape with
   string-typed `message.content` on any event (user tool_result included)
   fails the whole parse and falls back to raw — the designed fallback,
   pinned against the 2.1.247 capture; this remains the main
   version-volatility surface the brief flags.
3. `DetectSignal` on a claude stream with assistant text present scans only
   that text: a marker appearing solely in the result event's "result" field
   (while assistant text exists) would not match. This is per spec ("scans
   only assistant text events, never tool output") and matches the real
   shape, where a marker response carries the text in both the assistant
   event and the result field.

Review sandbox could not run `go test`/`go vet`/`bash -n` (git-shim
restricted to git read+commit; no docker/credentials); verification is full
diff read-through against the task list plus the pinned real-shape fixtures.