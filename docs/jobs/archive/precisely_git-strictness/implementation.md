# Implementation: git-strictness

id: precisely
status: open
developer: @developer
date: 2026-08-25

<!-- Produced by @developer after implementation. -->

## Summary

Two problems, two fixes, plus the system change that makes them stick:

1. **`mg done` kept failing on uncommitted worktree leftovers.** Agents don't
   reliably commit, and the read-only agents (the analyst above all) *cannot*
   commit at all (`commit: false` → read-only git mount). Fixed host-side:
   after every job-worktree session — bare `mg --job`, `mg jobs` launches,
   TUI launches, and every `mg jdi` --print invocation — the launcher now
   sweeps whatever the agent left uncommitted into a `[<id>] chore: commit
   all` commit (reusing the existing `git.CommitAll` and the TUI "c" key's
   message convention). The worktree is clean when the session returns, so
   `mg done`'s clean-tree guard stops being tripped by agent leftovers — and
   the analyst's `tasks.md` gets committed by the host after its session,
   closing the "no agent feels responsible" gap.
2. **The reviewer flagged `NEEDS WORK` over commit hygiene that is discarded
   at merge time.** The "Commit discipline" check in `agents/reviewer.md`
   (per-task `[ID] TASK-N:` format, `implementation.md`'s own commit) is
   relaxed: commit history is squashed into one commit at `mg done`, so
   message/format hygiene is not a review criterion. The load-bearing
   verdict-commit convention (`[ID] verdict: <one-line summary>`, counted by
   `git.CountVerdictCommits` / `LatestCommitIsVerdict`) is explicitly kept.
3. **`agents/developer.md`'s per-task commit strictness is softened** the same
   way ("one commit per task is the recommended pattern, exact format not
   required"), and the developer is now told to leave the worktree clean when
   finishing — including files earlier agents left behind.

`mg done`'s clean-tree guard stays as a backstop (it catches non-agent
leftovers, e.g. a human editing a job worktree); changing it to auto-commit
was out of scope. `mg host` sessions are never swept (no isolation by
design), and plain sessions (the user's own uncommitted work) are never
swept.

Rework after the reviewer's NEEDS WORK (one blocker): the sweep's `root.Job
!= ""` gate was not actually worktree-only — the `--job` flat-scan fallback
(a git repo with no local branches, or a non-git project, where the job's
files live directly in the main project root) set `Job` while `ProjectRoot`
stayed the main root, so `git.CommitAll` would have created the repo's FIRST
commit from the user's entire main-worktree contents, `.env` included. The
sweep now additionally requires `root.GitCommonDir != ""` — set only when
`--job` resolved a real worktree — closing that vector with unit + hook-level
tests pinning a fresh-repo flat-scan job as a no-op.

## Changes

TASK-1: `agents/reviewer.md` — replaced the "Commit discipline" bullets
(per-task `[ID] TASK-N:` format + `implementation.md`'s own commit) with a
note that commit history is squashed at `mg done`, so message/format hygiene
is not a review criterion; explicitly kept the `[ID] verdict: <one-line
summary>` commit instruction for `verdict.md` (the mg-jdi state machine
counts those commits) and the final "commit verdict.md" step.

TASK-2: `agents/developer.md` — softened the per-task commit strictness:
"Step 3 — Commit" is now "one commit per task is the recommended pattern, but
the exact message format is not required" (the exact-format example and "this
is not optional" are gone); added the load-bearing rule that the developer
must leave the worktree clean when finishing, including the analyst's leftover
`tasks.md`; kept the final `implementation.md` summary + commit step; softened
the frontmatter description and the matching hard-rules line the same way.

TASK-3: host-side sweep, wired into both session paths.
- `internal/session/sweep.go` (new) — `SweepJobWorktree(root Root, diag
  io.Writer)`: no-op when `root.Job == ""` **or `root.GitCommonDir == ""`**
  (the worktree gate — `GitCommonDir` is set only when `--job` resolved a
  real worktree, so the `--job` flat-scan fallback, where the job's files
  live directly in the main project root, is never swept); derives the job
  id from the `<id>_<slug>` name (split on the first underscore, whole name
  fallback); calls `git.CommitAll` with `[<id>] chore: commit all`; swallows
  `git.ErrNothingToCommit` (clean tree) and `git.ErrNotARepo` (a broken
  worktree whose gitdir vanished); warns on stderr for any other failure
  (never aborts); prints a short "committed leftover changes" note when it
  did commit.
- `internal/session/docker.go` — `DockerInvocation.Run` now returns
  `(exitCode int, ran bool)`: `ran` is false only when docker could not be
  exec'd at all (missing on PATH, permission denied) — the "did the container
  session happen" signal the sweep keys off. An `ExitError` (docker ran,
  exited non-zero) still counts as ran=true: the agent session happened.
  Chosen over "sweep on any non-zero exit" ambiguity because a docker-daemon
  failure exits non-zero via the docker CLI; the accepted trade-off (per the
  task's open question) is that a daemon-level launch failure sweeps
  pre-existing leftovers, which is harmless — the preferred reading, "no
  sweep when no agent ran", is honored for the launch-failure case that
  actually means "no agent ran".
- `cmd/mg/session.go` — `runSession` sweeps after `inv.Run` when `ran` is
  true. Covers bare `mg --job`, `mg jobs` launches, and TUI-launched
  interactive sessions (they re-exec the mg binary through
  `internal/launch`, landing in `runSession`).
- `cmd/mg/jdi.go` — `commandAgentRunner.Run` sweeps after `inv.Run` when
  `ran` is true, covering every `mg jdi` --print invocation (including the
  analyst). The sweep runs before the loop's post-run stall probe reads HEAD,
  so the sweep commit correctly counts as agent progress (a stuck agent that
  writes nothing still trips the stall backstop, since `ErrNothingToCommit`
  is swallowed and HEAD stays put). The sweep commit does not match the
  verdict pattern, so `CountVerdictCommits`/`LatestCommitIsVerdict` (the
  retry budget and re-review decisions) are unaffected.

TASK-4: tests.
- `internal/session/sweep_test.go` (new) — real-git tests: a job worktree
  with a modified tracked file + a new untracked file + a deleted tracked
  file is swept into one `[<id>] chore: commit all` commit and left clean; a
  clean worktree produces no commit and no error; a non-job root is a no-op;
  a non-repo root (with the gate passing) warns nothing; the id is derived
  correctly from `id_slug` names (including multi-underscore slugs and
  no-underscore fallback); a sweep failure (stale `index.lock`) surfaces as a
  stderr warning, not an abort; **a fresh-repo flat-scan fallback (no local
  branches, `GitCommonDir == ""`) is a no-op — the repo keeps its unborn HEAD
  and the user's `.env`/work files stay untracked** (`TestSweepJobWorktreeFlatScanFallbackNoOp`).
- `cmd/mg/session_test.go` — hook-level: `TestRunSessionSweepsJobWorktree`
  (fake exit-0 docker on PATH, real worktree job, leftover tasks.md → swept,
  worktree clean, project root untouched),
  `TestRunSessionDoesNotSweepWhenDockerFailed` (git-only PATH, docker never
  exec'd → no sweep commit, leftover stays uncommitted), and
  `TestRunSessionDoesNotSweepFlatScanFallback` (fresh git init, no local
  branches, fake exit-0 docker, user's `.env`/work in the main root → no
  sweep, unborn HEAD preserved).
- `cmd/mg/jdi_test.go` — hook-level: `TestCommandAgentRunnerSweepsJobWorktree`
  (fake docker, real worktree job, analyst's leftover tasks.md → swept by the
  host after `commandAgentRunner.Run`).

TASK-5: docs sync (per the hard rule keeping `agents/*.md`,
`project-template/docs/AGENTS.md` and `docs/AGENTS.md` describing the same
system).
- `docs/AGENTS.md` — new "Job worktrees are kept committed" subsection after
  "Read-only git mount for non-committing agents": every job-worktree session
  ends with a host-side sweep-commit; gating (job-worktree-only,
  container-actually-ran); the flat-scan `--job` fallback is never swept
  (there is no job worktree, and sweeping the main root would commit the
  user's own uncommitted work); the verdict-pattern non-interference. The
  repo root `/workspace/AGENTS.md` is a read-only mount of `docs/AGENTS.md`
  (same inode, byte-identical), so the single edit covers both paths.
- `project-template/docs/AGENTS.md` — short version mirrored into the context
  comment block, including the flat-scan fallback exclusion.
- `README.md` — workflow flow steps 6–7 now read "commits as it goes" (the
  strict `[ID] TASK-N:` example is gone), with a new paragraph on the
  relaxed commit hygiene + the end-of-session auto-commit; "How to get a job
  done" step 6 mentions leftovers are auto-committed at session end.

## Rework after the second NEEDS WORK (pre-worktree job blocker)

The reviewer found the `root.GitCommonDir != ""` gate closed the `--job`
flat-scan fallback but not a second, still-reachable main-worktree shape: a
**pre-worktree job** — the job's branch checked out in the main worktree
itself, an explicitly supported transitional state (`internal/job/discover.go`
keeps listing it so the TUI and `mg-jdi` keep working on it). For such a job,
`git.WorktreeForBranch` resolves to the main worktree's own porcelain entry,
so `--job` resolution sets `ProjectRoot` to the main project root while
`GitCommonDir` still resolves non-empty (the repo's own `.git`) — passing the
old gate and sweeping the user's own uncommitted work (`.env` included) onto
the job branch.

Fixed by replacing the gate with `root.ProjectRoot == root.InvocationRoot`
(`internal/session/sweep.go`): a linked job worktree's `ProjectRoot` always
differs from `InvocationRoot`, so this one comparison closes both main-worktree
shapes — the flat-scan fallback (which never reassigns `ProjectRoot` away from
`InvocationRoot`) and the pre-worktree job (whose `WorktreeForBranch` result
equals the main root) — uniformly, without needing to special-case
`GitCommonDir` at all.

- `internal/session/sweep.go` — gate changed from `root.GitCommonDir == ""`
  to `root.ProjectRoot == root.InvocationRoot`; doc comment updated to
  describe both main-worktree shapes it now excludes.
- `internal/session/sweep_test.go` — existing "should sweep" unit tests now
  set a distinct `InvocationRoot` so the new gate doesn't false-negative them;
  the flat-scan no-op test now sets `InvocationRoot: dir` (matching the real
  resolution, where the flat-scan branch never reassigns `ProjectRoot`); new
  `TestSweepJobWorktreePreWorktreeJobNoOp` pins the pre-worktree-job shape as
  a no-op even with a non-empty `GitCommonDir`.
- `cmd/mg/session_test.go` — new `TestRunSessionDoesNotSweepPreWorktreeJob`:
  a real repo with a job branch checked out in the main worktree (no linked
  worktree created), leftover `.env`/work file, fake exit-0 docker — asserts
  no sweep commit and the leftovers stay untracked.
- `docs/AGENTS.md` / `project-template/docs/AGENTS.md` — the "Job worktrees
  are kept committed" section now names both main-worktree shapes the sweep
  excludes, not just the flat-scan fallback.

`go build ./...`, `go vet ./...`, and `go test ./internal/... ./cmd/...` all
pass; `gofmt -l` still only flags the pre-existing, unrelated
`internal/session/root_test.go`.

## Known issues / follow-ups

- The analyst's `tasks.md` for this very job was sitting uncommitted in the
  worktree when implementation began (the analyst cannot commit, and the host
  sweep did not exist yet). `git add -A` on the TASK-1 commit swept it in, so
  the TASK-1 commit also carries `tasks.md` — the exact failure mode this job
  fixes, and now impossible going forward.
- Reviewer blocker (fixed): the first implementation's sweep gate
  (`root.Job != ""`) would have swept the `--job` flat-scan fallback's main
  project root — committing the user's own uncommitted work, `.env` included,
  as the repo's first commit. Fixed by additionally requiring
  `root.GitCommonDir != ""` (the worktree-resolved path), with the fix pinned
  by `TestSweepJobWorktreeFlatScanFallbackNoOp` (unit) and
  `TestRunSessionDoesNotSweepFlatScanFallback` (hook-level).
- `internal/session/root_test.go` is flagged by `gofmt -l` — pre-existing
  (untouched by this job), left alone per the no-unrelated-refactor rule.
- Test runs in a manigot container session need real git first on PATH
  (`PATH=/usr/bin:/bin:$PATH go test ./...`), because the session's git shim
  refuses the `git init`/`worktree`/`branch -D` the test helpers use. This is
  an environment artifact of running tests inside a manigot session, not a
  code change.
- Open question resolved: the sweep helper lives in `internal/session`
  (it takes `session.Root` and writes to the session's diag writer), and the
  "did the container actually run" signal is `DockerInvocation.Run`'s new
  `ran` return value (docker was exec'd successfully). A docker-daemon-down
  launch failure sweeps pre-existing leftovers — harmless, documented in the
  code.