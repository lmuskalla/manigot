# Verdict: part 2 of web ui tui path

id: farmer
status: open
reviewer: deepseek-v4-flash
date: 2026-08-28

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Independent full re-review of the current branch state (`feature/farmer_
part-2-of-web-ui-tui-path`, base `main`), after the two commits
(`fa45e2df`, `e996eed0`) that followed the previous NEEDS WORK verdict.
The TASK-7 blocker from that verdict is now addressed; I re-verified the
whole surface from the diff rather than trusting the prior notes.

TASK-1: PASS
notes: Pure relocation verified: `src/internal/job/sessionlog.go` is the
  removed `openSessionLog`/`sessionLog`/`ensureSessionLogFile` from
  `src/cmd/mg/jdioutput.go` byte-for-byte modulo the exported renames
  (`SessionLog`/`OpenSessionLog`/`EnsureSessionLogFile`); `cmd/mg/jdi.go`
  call sites (jdi.go:591, 150) use `job.OpenSessionLog`/
  `job.EnsureSessionLogFile` with identical semantics and `sess.Close()`
  still runs in the loop (jdi.go:601-605); the moved coverage lives in
  `src/internal/job/sessionlog_test.go` with the same test names and
  assertions (same on-disk format: blank-line separator, "=== <RFC3339>
  <agent> (attempt N) ===" header, trailing-newline guarantee). Only the
  session-log tests were removed from `jdioutput_test.go` (its remaining
  tests + the now-unused `time` import are gone cleanly); no dangling
  references to the old private names remain anywhere in cmd/mg.

TASK-2: PASS
notes: `src/internal/session/oneshot.go` — `CommandAgentRunner.Run` mirrors
  cmd/mg/jdi.go's `commandAgentRunner.Run` (jdi.go:366-415) exactly:
  ResolveProfile → ResolveRootFrom → CheckAuth → BuildDockerInvocation →
  fail-soft container prune → Run → SweepJobWorktree on ran; the
  invocation options are identical (`Options{Print: true, Profile, Agent,
  Job: j.Name}` — jdi.go:348-350), so the one-shot invocation also carries
  the NEEDS-HUMAN-INPUT marker (injected in BuildDockerInvocation when
  Print is set, docker.go:337-341). `RunOneShot` opens one section at
  attempt 1, tees output live, writes runner errors into the same section
  (the only reporting channel once the HTTP request has returned), closes
  with the trailing-newline guarantee. The concurrent-mg-jdi limitation is
  documented. Tested with a fake runner (section format, trailing newline,
  error-into-log, multiple appends). Home in internal/session avoids the
  import cycle (serve→session→job).

TASK-3: PASS
notes: `handleCreateJob` (mutate.go:118) — lock taken (mutate.go:137),
  `type` pre-validated against CreateJob's accepted set via
  `validJobTypes` (clean 400, mutate.go:132-135), title required (400),
  201 with `jobRowFromJob(res.Job)` + branch + worktreePath, `out`
  discarded via io.Discard. Signatures verified against create.go:75.
  Tested: git (real branch+worktree, discoverable after), non-git
  fallback, validation (mutate_test.go:54-153).

TASK-4: PASS
notes: `handleEditBrief` (mutate.go:181) — raw-body replace of brief.md
  only (the PUT route is a literal path, so tasks/implementation/verdict
  cannot be written; a PUT to those paths is a 405), empty and >1 MiB
  bodies are 400s, immediate `SweepJobWorktree` commit in the job's own
  worktree (mutate.go:215-227), lock taken (mutate.go:205 — this task's
  reasoned addition, correct: the sweep commits). The worktree-resolution
  gate handles git / non-git / no-registered-worktree states (warning, not
  failure). Git + non-git + validation tested. `j.Dir` from
  `job.Discover` is the job's own worktree path, so the write lands where
  agents see it and the sweep commits the right checkout.

TASK-5: PASS
notes: `handleLaunchAgent` (mutate.go:245) — agent segment validated by
  validSegment + matched against `agentlist.Discover` first (fast 404 on
  unknown, mutate.go:255-275), optional `{"profile": "..."}` validated via
  `resolveProfile` defaulting to `config.ProfileClaudePro` (400 on
  unknown), 202 returned, `go session.RunOneShot(...)` in a background
  goroutine, no lock (per the brief's Notes). Tested (202 + session.log
  section appears; unknown agent 404; bad profile 400).

TASK-6: PASS
notes: `handleLaunchJDI` (mutate.go:304) — 409 when `job.ReadJDIStatus`
  reports a live running state (staleness correctly degrades to ok=false
  via jdistatus.go:100, so a killed run does not block relaunch),
  `launch.Jdi` is a detached subprocess (launch.go:338), no lock, 202.
  Tested with JdiExe stubbed (mutate_test.go:295-323).

TASK-7: PASS
notes: The blocker from the previous verdict is FIXED. `handleDoneJob`
  (mutate.go:401) now appends the explanatory `out` buffer to the 409
  error body on `ErrSquashMergeConflict` (mutate.go:435-438) — the body
  explicitly states the job is already archived-and-committed and cannot
  be undone, and `TestHandleDoneJobConflictIsStructured409` now asserts
  the "already archived" wording (mutate_test.go:411-413). The additive
  `FinishJobWithOptions` change is complete: zero-value options behave
  byte-for-byte like FinishJob (finish_test.go:238), `NoConflictRecovery`
  returns the distinguishable `ErrSquashMergeConflict` with no prompt, no
  rollback, no @git-solver handoff (finish.go:265-279 — the no-rollback
  and no-git-solver properties are each pinned by tests), existing
  CLI/TUI callers untouched, and the "verdict not approved"/"no
  verdict.md" warnings require `{"force": true}` with the CLI's own
  warning text in the 409 (mutate.go:416-419, doneVerdictWarning:364 —
  text verified against finish.go's own checks, same helpers).

TASK-8: PASS
notes: `handleDeleteJob` (mutate.go:462) — lock, pre-approved confirm
  (`approvedConfirm`, the HTTP call is the confirmation — same precedent
  as the TUI's yesConfirm), DeleteResult returned. Tested for git and
  non-git projects and unknown jobs.

TASK-9: PASS
notes: `handlePushJob` (mutate.go:497) — lock, `git.PushWithContext`
  bounded by a 30s timeout (pushTimeout:490, matching the TUI's
  hostGitTimeout), git's own message in the structured error. Tested with
  no remote (500 with the origin error) and a real bare remote (200).

TASK-10: PASS
notes: `handlePrune` (mutate.go:533) — top-level `POST /prune`
  (containers are not project-partitioned), wraps `session.PruneOrphans`,
  reports removed + running counts, no lock. Tested (docker absent →
  structured 500 with the docker error — the contract being a JSON
  envelope, never a silent no-op).

TASK-11: PASS
notes: GET /projects/{p}/orphans (DiscoverOrphans, read-only, no lock)
  and POST /projects/{p}/orphans/{name}/delete (validSegment, MatchOrphan,
  one named orphan via RemoveOrphansConfirmed, under the lock — the
  reasoned addition, since RemoveOrphansConfirmed calls git.WorktreePrune).
  Signatures verified (orphan.go:59/130/195). Tested: list, delete-one,
  untouched-other, 404.

TASK-12: PASS
notes: `stream.go` — SSE over plain net/http (no new dependency), starts
  at EOF / byte 0 for a missing file / `?from=` offset (validated before
  any SSE header is written — bad from is a clean 400), truncation resets
  to byte 0 with a fresh `start` event carrying `{"offset":N}`, per-line
  `data:` frames, keepalive on idle, stops on client disconnect AND on
  daemon shutdown via the server-scoped shutdownCtx cancelled at the top
  of `Server.Shutdown` (server.go:111-114) — the brief's explicitly
  called-out graceful-shutdown requirement, wired to the drain in
  cmd/mg/serve.go:154-163. The audit middleware's `statusRecorder.Flush`
  delegation (audit.go:43-54) is correct and necessary (without it the
  `w.(http.Flusher)` assertion fails through the middleware chain — the
  live-growth test exercises the full wrapped Handler and would fail
  otherwise). Tests cover live growth, from-offset resume,
  file-appears-after-start, bad from, disconnect, and a prompt shutdown
  drain.

TASK-13: PASS
notes: security_test.go extends hostile/encoded traversal segments to
  every new URL position ({project} on create, {job} on all six mutating
  routes, {agent}, orphan {name}) with a no-leak assertion; all paths flow
  through the same resolveProject/resolveJob/validSegment choke points.
  credentials_test.go re-runs the known-credential grep across every
  mutating endpoint's success and error envelopes (create/edit-brief/
  launch-agent/launch-jdi/done/push/prune/orphans list+delete/stream bad-
  from). By inspection no mutating handler touches .env or echoes
  credential material.

TASK-14: PASS
notes: concurrency_test.go pins the boundary end to end: create/edit-brief/
  done/delete/push/orphan-delete block behind a held project lock and
  proceed once released; launch-agent/launch-jdi/prune/reads complete
  immediately; two different projects proceed independently; a serialized
  create's critical section actually runs to completion. Matches the
  shipped handler code exactly (locks at mutate.go:137/205/421/472/507/603;
  no locks at 288-291/330/534/558/stream).

TASK-15: PASS
notes: docs/AGENTS.md (+ project-template mirror) document the mutating
  API, the 202-then-poll/stream contract, the SSE endpoint, and the
  shipped ProjectLocks boundary (which ops lock, which don't, and why);
  README.md's command table + Listener section updated the same way;
  docs/listener.md marks job two DONE with a short annotation including
  the recorded edit-brief+relaunch decision on NEEDS-HUMAN-INPUT; the
  `mg serve` help text no longer claims read-only (serve.go:59-65). All
  accurately describe what shipped.

## Security

No new findings beyond the review notes. The two job-one invariants (zero
path inputs; credentials never returned) are re-pinned across the whole
mutating surface by TASK-13's tests. The auth/audit middleware chain is
unchanged in structure; `statusRecorder.Flush` is the only modification
and is required for SSE to work at all. The stream handler's `start`/
`data` frames carry only log content and byte offsets; `?from=` is
validated before any SSE headers are written.

Out-of-band note (unchanged from the previous verdict, still on `main`,
outside this branch's diff): `main` carries the stray commit 66d2524
"[farmer] implementation: add summary" adding a root-level
`implementation.md` — an artifact of an early session committed to the
main worktree. It does not conflict with this job's squash merge, but the
human should clean it up when merging.

## Non-blocking observations (no action required for merge)

1. TASK-4: `editBriefResponse.Warning` is also populated on a *successful*
   sweep commit ("mg: committed leftover changes ...", mutate.go:219-221),
   contradicting its own doc comment ("set only when the post-write commit
   sweep could not be attempted or failed"). Cosmetic naming mismatch.
2. TASK-7: the conflict explanation's phrasing "already archived on
   <branch>" (finish.go:272-273) is loose — the archive lives at
   docs/jobs/archive/ — but it does state the required fact (already
   archived, cannot be undone). Cosmetic.
3. TASK-12: blank lines in session.log are emitted as `: keepalive`
   comments rather than empty `data:` frames, so blank section separators
   are invisible to EventSource consumers. Cosmetic protocol note.
4. TASK-2: a session.log *open* failure in the detached goroutine path is
   silent (the invocation still runs with output discarded; the open error
   is only in RunOneShot's return value, which the goroutine drops). Rare
   edge case, matches the documented best-effort open stance.

## Overall

APPROVED

No blockers. All fifteen tasks are implemented as specified, in scope, and
correct by inspection; the previous verdict's single blocker (TASK-7's 409
body not stating that the job is already archived-and-committed) is fixed
in the handler and pinned by the test. The non-blocking observations above
are cosmetic and may be addressed in a later job if desired.