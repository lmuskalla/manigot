# Implementation: part 2 of web ui tui path

id: farmer
status: open
developer: farmer
date: 2026-08-28

<!-- Produced by @developer after implementation. -->

## Summary

Job two of the control-plane sequence: the `mg serve` daemon grew from
read-only (job one) into an actual control plane. It can now drive a job's
full lifecycle over HTTP — create, edit brief, launch agents and `mg jdi`
detached, done, delete, push, prune, and orphaned-worktree cleanup — and
stream a live agent run's `session.log` over Server-Sent Events, the network
twin of the TUI's `l` tail.

## Changes

TASK-1: Relocated the session.log section-writer machinery out of `cmd/mg`
  into `internal/job/sessionlog.go` (exported `SessionLog`/`OpenSessionLog`/
  `EnsureSessionLogFile`), so both the mg-jdi loop and the new one-shot
  agent-run primitive can write a live, tailable session.log. Pure relocation:
  on-disk format unchanged, `cmd/mg/jdi.go` updated to call the relocated
  functions, and `jdioutput_test.go`'s coverage moved (not rewritten) to
  `internal/job/sessionlog_test.go`. Previously nothing wrote a job's
  session.log for a bare `mg --print` invocation outside the mg-jdi loop.

TASK-2: Added the one-shot detached agent-run primitive in
  `internal/session/oneshot.go` (`AgentRunner`, `CommandAgentRunner`,
  `RunOneShot`): a single `--print` invocation via the launcher's own
  ResolveProfile → ResolveRootFrom → CheckAuth → BuildDockerInvocation → Run
  path, ending with the same post-run `SweepJobWorktree` mg-jdi performs,
  opening one session.log section at attempt 1 and writing any runner error
  into the same section (the only place left to report once the triggering
  HTTP request has returned). Lives in internal/session so `internal/serve`
  can reuse it without importing cmd/mg. Known documented limitation: a
  concurrently-running mg-jdi loop against the same job would write to the
  same session.log — safe against corruption, but two processes touching the
  same git worktree is not detected or prevented.

TASK-3: `POST /projects/{project}/jobs` create-job endpoint
  (`handleCreateJob` in `internal/serve/mutate.go`) — wraps `job.CreateJob`
  under `s.locks.Lock(root)`, validates `type` against CreateJob's accepted
  set up front (clean 400 instead of a 500), returns 201 with the created
  job's row plus branch/worktree path.

TASK-4: `PUT /projects/{project}/jobs/{job}/files/brief` edit-brief endpoint
  (`handleEditBrief`) — raw-body replace of brief.md (no `$EDITOR`), then
  immediately commits it in the job's own worktree via
  `session.SweepJobWorktree` so the edit never sits uncommitted. Takes the
  project lock (this task's reasoned addition to the brief's git-mutating
  set: the commit touches git). Empty/oversized bodies are clean 400s; a
  non-git project or a branch with no registered worktree reports a warning
  (the write still succeeds).

TASK-5: `POST /projects/{project}/jobs/{job}/agents/{agent}` launch-agent
  endpoint (`handleLaunchAgent`) — validates the agent against
  `agentlist.Discover` first (fast 404 on unknown names), accepts an optional
  `{"profile": "..."}` (default `claude-pro`), then starts TASK-2's
  `RunOneShot` in a background goroutine and returns 202. No lock taken. The
  caller watches the run via the session-log stream or by polling
  `GET .../jdi`'s sessionLog tail.

TASK-6: `POST /projects/{project}/jobs/{job}/jdi` launch-jdi endpoint
  (`handleLaunchJDI`) — wraps `launch.Jdi` (already a detached subprocess),
  refusing 409 when the job's status sidecar says running (best-effort
  double-launch guard, documented as not airtight). No lock. Returns 202.

TASK-7: `POST /projects/{project}/jobs/{job}/done` done endpoint
  (`handleDoneJob`) plus the additive `FinishJobWithOptions` change in
  `internal/job/finish.go`. A squash-merge conflict under the endpoint is a
  structured 409 (`job.ErrSquashMergeConflict` via
  `FinishOptions.NoConflictRecovery`) — neither of the two interactive CLI
  behaviors (git-solver handoff, auto-rollback) is taken; the job is left
  archived-and-committed in its own worktree with the main worktree in its
  conflicted state, for an explicit follow-up call. The 409 body explicitly
  states the job is already archived-and-committed and cannot be undone (it
  appends FinishJobWithOptions' own explanation written to its `out` buffer —
  `err.Error()` alone would imply "nothing happened" and a retry of done would
  then fail confusingly with "brief.md not found"). Existing callers (CLI mg
  done, TUI) keep their confirm-based behavior unchanged. The earlier
  "verdict not approved"/"no verdict.md" warnings require `{"force": true}`
  (409 echoing the CLI's warning text otherwise).

TASK-8: `POST /projects/{project}/jobs/{job}/delete` delete endpoint
  (`handleDeleteJob`) — wraps `job.DeleteJob` under the lock with a
  pre-approved confirm (the HTTP call itself is the confirmation, the same
  precedent the TUI's yesConfirm established). Returns the DeleteResult.

TASK-9: `POST /projects/{project}/jobs/{job}/push` push endpoint
  (`handlePushJob`) — `git.PushWithContext` under the lock, bounded by the
  TUI's own 30s timeout; failures surface as structured errors with git's own
  message.

TASK-10: `POST /prune` prune endpoint (`handlePrune`) — wraps
  `session.PruneOrphans`, deliberately top-level (containers are not
  partitioned by project). Reports removed + running counts.

TASK-11: Orphaned-worktree endpoints — `GET /projects/{project}/orphans`
  (read-only list via `job.DiscoverOrphans`) and
  `POST /projects/{project}/orphans/{name}/delete` (one named orphan via
  `job.MatchOrphan` + `job.RemoveOrphansConfirmed`, under the lock — this
  task's reasoned addition, since RemoveOrphansConfirmed calls
  `git.WorktreePrune`).

TASK-12: `GET /projects/{project}/jobs/{job}/session-log/stream` SSE endpoint
  (`internal/serve/stream.go`) — Server-Sent Events over the plain net/http
  server (no WebSocket, no new dependency), tailing the same session.log the
  TUI's `l` key tails. Starts at EOF (tail -f), byte 0 when the file doesn't
  exist yet, or `?from=<byte offset>` for reconnects; a truncated file resets
  to byte 0 with a fresh `start` event. The `Server` struct gained a
  shutdown context cancelled at the top of `Shutdown`, so an open stream
  unblocks and the drain returns promptly instead of waiting out its full
  timeout. Two fixes were needed to the swept-in implementation: the handler
  was missing its `bytes`/`io` imports and an undefined `readAll`, and the
  `start` event payload was written as a bare number despite the documented
  `{"offset":N}` protocol. Also fixed the audit middleware's `statusRecorder`,
  which wrapped the ResponseWriter in a way that made the stream handler's
  `w.(http.Flusher)` assertion fail — every stream would have returned
  "streaming unsupported" in the real daemon; the recorder now delegates
  Flush.

TASK-13: Extended `security_test.go` (hostile/encoded traversal segments at
  every new URL position: {project} on create, {job} on all mutating routes,
  {agent}, and orphan {name}) and `credentials_test.go` (every mutating
  endpoint's success and error bodies grepped for the known credential values
  planted in `.env` + env — the same technique job one used, re-run across
  the new surface).

TASK-14: `internal/serve/concurrency_test.go` — pins the ProjectLocks
  boundary across the handler surface: create, edit-brief, done, delete,
  push and orphan-delete block behind a held project lock (they serialize);
  launch-agent, launch-jdi, prune and reads complete immediately (they
  don't); two different projects' mutating calls proceed independently.

TASK-15: Docs sync — `docs/AGENTS.md` and its mirror
  `project-template/docs/AGENTS.md` now document the mutating API, the async
  202-then-poll/stream contract, the SSE endpoint, and the shipped
  ProjectLocks boundary; `README.md`'s Listener section and command table
  row updated the same way; `docs/listener.md`'s "Sequencing after this job"
  marks job two as DONE with a short annotation (not a rewrite), including
  the recorded decision that "answering" a `NEEDS-HUMAN-INPUT:` marker is the
  composition of edit-brief + relaunch, not its own endpoint. The `mg serve`
  help text no longer claims the API is read-only.

## Known issues / follow-ups

- **Test-environment note:** `internal/ui` and `cmd/mg` tests that shell out
  to `git init` fail when the manigot session git shim is on PATH (it refuses
  `git init`); they pass with a real-git-only PATH. This is a pre-existing
  session-environment artifact, not a regression from this job.
- The launch-jdi double-launch guard (409 on a running status sidecar) is
  best-effort by design: two launches issued back-to-back before the first
  process writes its status could both proceed. Documented, not solved —
  the brief does not ask for real locking here.
- A concurrently-running mg-jdi loop against the same job as a detached
  agent launch writes to the same session.log (safe: append-only,
  single-writer-per-section) but means two processes touch the same git
  worktree — not detected or prevented (documented limitation of TASK-2).
- The done endpoint's conflict path leaves the main worktree in its
  conflicted-merge state by design; resolving it requires an explicit
  follow-up (e.g. launching @git-solver via the agent endpoint, or `mg host`
  from a terminal). The response says so explicitly rather than implying
  nothing happened.