# Implementation: job view: p -> o

id: 02ckl4
status: open
developer: @developer
date: 2026-08-13

<!-- Produced by @developer after implementation. -->

## Summary

Changed the Owner agent's keyboard shortcut in the TUI's job detail view from
`p` (a mnemonic left over from when the role was still called "product owner")
to `o`, since the role is now plain "owner". The binding lives in the single
`agentMeta` table in `internal/ui/agents.go`, from which both the action-bar
rendering (`renderActionBar`) and the key dispatch (`agentForKey`) derive, so
one edit propagated everywhere. `o` collides with nothing on the detail view
(the list view's `o` quick-session key is a separate view state), and the
change is covered by updated and new tests. Build and the full test suite pass.

## Changes

TASK-1: Changed `agents.Owner`'s action-bar key from `"p"` to `"o"` in the
`agentMeta` table.
     files: internal/ui/agents.go

TASK-2: Updated the three existing test assertions that expected the owner
key `"p"`: the `wantByKey` map in `TestAgentForKeyIgnoresStage`, the
`"[p] Owner"` unified-format button in
`TestRenderActionBarUnifiedFormat`, and the `"[p]"` key in
`TestRenderActionBarTruncatesLabelsAt80ColsKeysStayIntact`.
     files: internal/ui/agents_test.go

TASK-3: Added `TestAgentForKeyPNoLongerResolves`, mirroring the existing
`TestAgentForKeyVNoLongerResolves` precedent (Developer's old `"v"` key),
asserting `"p"` no longer resolves to any agent.
     files: internal/ui/agents_test.go

TASK-4: Verified with `go build ./...` and `go test ./...` — both pass,
including the updated/added ui tests.

## Known issues / follow-ups

- **Git worktree registration for this job is missing** (pre-existing,
  out of scope): the container's `/workspace` is the job's worktree
  (`/home/leo/code/.manigot-worktrees/manigot/02ckl4_job-view-p-o`), and its
  `.git` file points at `/home/leo/code/manigot/.git/worktrees/02ckl4_job-view-p-o`,
  which does not exist — while the main repo's `worktrees/` currently
  registers `/workspace` against the *different* job
  `otv2d2_make-6-string-char-more-writable`. As a result, ordinary `git`
  commands from `/workspace` fail. Commits were made on
  `feature/02ckl4_job-view-p-o` via an ephemeral gitdir (sharing the main
  repo through `commondir`) that writes nothing to the shared
  `.git/worktrees/` registrations. The host-side `mg` tooling should
  re-register this job's worktree (and resolve the `/workspace` conflict
  with the `otv2d2` job) before `mg done` / `mg delete` are run on this job.
- Nothing else was found to be in scope but undone: the list view's `o`
  quick-session binding is intentionally unchanged (different view state),
  and the stale "b switch branch" mention in `agents.go`'s comment is a
  pre-existing, unrelated comment issue.
