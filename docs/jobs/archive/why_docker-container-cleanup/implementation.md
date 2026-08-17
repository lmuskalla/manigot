# Implementation: docker container cleanup

id: why
status: open
developer:
date: 2026-08-17

## Summary

Implemented the brief's resolution to the orphaned-docker-containers problem:
every session runs `docker run --rm --name manigot-<project>-<pid>`, so
containers self-remove on normal exit and orphans are purely the residue of
abnormal ends (killed client, killed pane/window, host/daemon restart, hung
CLI). Cleanup removes only EXITED containers whose name matches the manigot
prefix — running manigot containers (an unattended agent may still be
working) and foreign containers are never touched, mirroring `docker
container prune` semantics scoped to manigot's own containers.

Cleanup happens at two points: automatically before every container launch
(the fail-soft self-healing path), and via an explicit `mg prune` command for
on-demand or cron use.

## Changes

TASK-1: `internal/session/prune.go` (new) + `internal/session/prune_test.go`
(new) — `PruneOrphans(diag)`, which lists exited manigot-* containers
(`docker ps -aq --filter name=^/manigot- --filter status=exited`), removes
them (`docker rm`), counts the still-running manigot-* containers (`docker ps
-q --filter name=^/manigot-`), and returns both counts. The docker exec is
behind a package-level `dockerCommand` var (the cmd/mg/diff.go `tigLookPath`
seam pattern) so tests stub it without a daemon; a pruning line goes to diag
only when something was actually removed. docker missing or the daemon down
is an error the caller decides how to surface.

TASK-2: `cmd/mg/session.go` and `cmd/mg/jdi.go` — wired the prune into every
container-launch path before the run: `runSession` (bare mg, mg init's
@prompter re-exec, mg agents/mg jobs re-exec, TUI-launched sessions — all
funnel through runSession) and `commandAgentRunner.Run` (every mg jdi
invocation). Both call a `pruneOrphans` package var (split out so cmd/mg
tests can stub it); a prune failure only warns on stderr and never aborts the
launch. mg host is unaffected — it never creates a container
(BuildHostInvocation). Tests: `cmd/mg/session_test.go` (new —
`TestRunSessionPruneCalledBeforeRun`, `TestRunSessionPruneFailureIsFailSoft`,
plus the `pathWithRealGitOnly` helper that makes the docker launch fail
deterministically on any machine) and `cmd/mg/jdi_test.go`
(`TestCommandAgentRunnerPrunesBeforeRun`,
`TestCommandAgentRunnerPruneFailureDoesNotAbort`, `pruneTestJob`).

TASK-3: `cmd/mg/prune.go` (new) + `cmd/mg/prune_test.go` (new) +
`cmd/mg/main.go` — the explicit `mg prune` subcommand: dispatcher case and
help entry in main.go; `runPrune` runs PruneOrphans, prints "Removed N
orphaned manigot container(s)." (or "Nothing to prune." when empty) plus the
running-manigot count, and exits 1 with a clear "Error: mg prune: ..." when
docker is missing or the daemon is down.

TASK-4: `docs/AGENTS.md` and `README.md` — added `mg prune` to the Commands
list / command table, a usage line in the README's Usage section, and a short
container-cleanup note in docs/AGENTS.md's session-launch section.
`project-template/docs/AGENTS.md` was not touched — its Commands section is
per-project placeholders, not a mirror of mg's command list.

TASK-5: Verified `go build ./...`, `go vet ./...`, and the full
`go test ./...` suite (all packages green, run uncached). The live
docker-daemon filter sanity check could not be run — docker is not installed
in this environment — so the filter semantics are covered by the stubbed
tests in TASK-1 instead (exited removed, running/foreign never touched).

TASK-6: Investigated the stray `/workspace/implementation.md` at the worktree
root — it is tracked and committed on the base branch (`main`, commit
`c728a17` "implementation: add summary", the prior mg-done-dirty-worktree
job's summary), the worktree copy is clean, and it is not in .gitignore.
Per the task's rule ("if committed on the base branch, leave it and note it
as out of scope") it was left in place; it does not trip mg done's clean-tree
check since it is committed, not a dirty change.

TASK-3 (verdict fix): the reviewer flagged that `internal/session/prune.go`
swallowed docker's captured output (`CombinedOutput`) when wrapping errors,
so a down daemon surfaced only as "docker ps: exit status 1" — not the
"clear error when docker is missing or the daemon is down" TASK-3 requires.
Fixed with a `wrapDockerErr` helper that appends docker's own trimmed output
to the wrapped error at all three failure sites (exited `ps`, `rm`, running
`ps`); `internal/session/prune_test.go` gains
`TestPruneOrphansDaemonDownIncludesOutput` pinning the new message. Also ran
`gofmt -w` on the five new Go files (the reviewer noted they lacked a final
newline). `go build ./...`, `go vet ./...`, and the full `go test ./...`
suite re-verified green with the real git on PATH.

## Known issues / follow-ups

- The live docker-daemon sanity check from TASK-5 could not be executed in
  this environment (no docker binary). The filter semantics are pinned by the
  stubbed unit tests; a one-off `docker ps -aq --filter name=^/manigot-`
  against a real daemon is worth a glance before release.
- The stray root `implementation.md` (committed on the base branch by the
  prior job) remains — it is out of scope for this job; deleting it would be
  a separate housekeeping change on the base branch.
- The test suite must run with the real git first on PATH (e.g.
  `PATH=/usr/bin:$PATH go test ./...`) in environments where the manigot git
  shim is on PATH — the shim refuses the test helpers' `git init`/`git
  worktree` calls. This is an environment quirk of agent sessions, not a
  code issue.