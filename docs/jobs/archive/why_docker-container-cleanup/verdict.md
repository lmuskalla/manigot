# Verdict: docker container cleanup

id: why
status: open
reviewer:
date: 2026-08-17

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Re-review of the TASK-3 fix (commits `bbb1085` + `355cd80`): the sole blocker
from the previous verdict — a down docker daemon surfacing only as "docker ps:
exit status 1" because `internal/session/prune.go` discarded `CombinedOutput`'s
captured bytes when wrapping errors — is resolved.

TASK-1: PASS
notes: `internal/session/prune.go` matches the spec: lists only EXITED manigot-prefixed
containers (`docker ps -aq --filter name=^/manigot- --filter status=exited`, matching
the `manigot-<project>-<pid>` name set in docker.go), removes them with `docker rm`,
counts running manigot containers via `docker ps -q --filter name=^/manigot-` (no `-a`,
so running only), returns both counts, and shells out through the `dockerCommand` var
(tigLookPath-style seam) so tests stub it without a daemon. Tests pin the contract:
exited removed / running + foreign never touched, no `rm` when nothing to prune,
docker-missing and rm-failure both surface as errors. The daemon-down diagnostic is
now also pinned by `TestPruneOrphansDaemonDownIncludesOutput` (see TASK-3).

TASK-2: PASS
notes: prune wired before the run in both container-launch paths: `runSession`
(cmd/mg/session.go) and `commandAgentRunner.Run` (cmd/mg/jdi.go, before
`inv.Run`), both fail-soft (warning, never abort). Every launch path funnels
through one of these two — bare mg, mg init's @prompter re-exec, mg agents/mg
jobs re-exec, TUI-launched sessions, and mg jdi CLI/TUI. mg host untouched
(BuildHostInvocation). Tests stub the `pruneOrphans` package var and prove the
prune runs before the docker launch and that a prune failure does not abort.

TASK-3: PASS (was PARTIAL — blocker fixed)
notes: dispatch case + help entry in cmd/mg/main.go, `runPrune` in cmd/mg/prune.go
(unknown-arg rejection, "Removed N ..." / "Nothing to prune." / running-count
reporting, exit 1 on docker failure) all correct, and cmd/mg/prune_test.go covers
each. The previous miss — `internal/session/prune.go` wrapping only the
`*exec.ExitError` and discarding `CombinedOutput`'s bytes, so a down daemon showed
only "docker ps: exit status 1" — is fixed by the new `wrapDockerErr` helper
(prune.go:30-35), which appends docker's own trimmed output to the wrapped error
at all three failure sites (exited `ps` at :58, `rm` at :65, running `ps` at :73).
`TestPruneOrphansDaemonDownIncludesOutput` (prune_test.go:131-148) pins the message:
a stubbed daemon-down invocation must produce an error containing both "docker ps"
and docker's "Cannot connect to the Docker daemon" diagnostic. The existing
docker-missing (`TestPruneOrphansDockerMissing`) and rm-failure
(`TestPruneOrphansRmFailure`) tests still pass against the new wrapper (empty
output falls back to the plain wrap). The task's "clear error when docker is
missing or the daemon is down" requirement is now met for both cases.

TASK-4: PASS
notes: docs/AGENTS.md gets the session-launch container-cleanup note and an `mg prune`
Commands entry; README.md gets the command-table row and a usage example. Both mirror
the actual behavior, including the new daemon-down error semantics. project-template
docs correctly untouched.

TASK-5: PASS (as far as verifiable in this environment)
notes: `go build ./...`, `go vet ./...`, and the full `go test ./...` suite are claimed
green (uncached), re-verified after the TASK-3 fix commit. The fix is small and purely
additive (one helper + one test) and does not touch any other package's API, so the
re-verification is credible; this reviewer cannot re-run it (no Go toolchain in the
review sandbox). The live docker-daemon filter sanity check could not be run — docker
is genuinely absent here — and is honestly flagged as a follow-up in implementation.md;
the filter semantics are pinned by the stubbed TASK-1 tests.

TASK-6: PASS
notes: verified `/workspace/implementation.md` is tracked and committed on the base
branch `main` (commit c728a17 "implementation: add summary"), is clean in this worktree,
and is not in .gitignore. Per the task's own rule ("if committed on the base branch,
leave it and note it as out of scope") it was correctly left in place; it does not trip
mg done's clean-tree check.

## Security

Not run — no security-specific task in this job. The change is host-side docker CLI
shell-out, scoped strictly to containers whose name matches the manigot prefix; it never
touches foreign containers and never runs containers, so the exposure surface is a
narrowed `docker ps`/`docker rm` on the host. No new container-side code.

## Overall

APPROVED

The single blocker is fixed: `wrapDockerErr` now carries docker's own diagnostic
through all three failure paths of `PruneOrphans`, so `mg prune` (and the launch-path
warning) produce a clear message when the daemon is down, satisfying TASK-3's explicit
requirement. The fix is covered by a new pinned test, and the cosmetic gofmt note from
the previous verdict (missing final newlines) was also addressed.

Non-blocking notes (no change required for merge):
- `mg prune` prints two different phrasings of the same fact when containers were removed:
  "Pruned N ..." on stderr (PruneOrphans' diag line, since runPrune passes stderr as
  diag) and "Removed N ..." on stdout. Cosmetic redundancy.
- In the mg jdi path the prune warning goes into the diag buffer, which only surfaces on
  error, so a prune failure is invisible on a successful run. This matches TASK-2's
  "call PruneOrphans with their diag writer" instruction and is documented in
  implementation.md.
- The live docker-daemon filter sanity check from TASK-5 still cannot be executed in this
  environment (no docker binary); a one-off `docker ps -aq --filter name=^/manigot-`
  against a real daemon is worth a glance before release.