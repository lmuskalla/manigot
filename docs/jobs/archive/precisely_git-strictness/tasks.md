# Tasks: git-strictness

id: precisely
status: open
analyst: @analyst
date: 2026-08-25

<!-- Produced by @analyst from brief.md. -->

## Decisions this breakdown locks in

**1. Two separate problems, two separate fixes.** (a) `mg done` keeps failing
on the clean-tree check because agent sessions leave uncommitted changes in
the job worktree — including the analyst's `tasks.md`, which the analyst
agent physically cannot commit (`commit: false` → read-only git mount). (b)
The reviewer flags `NEEDS WORK` over git commit hygiene (the "Commit
discipline" section in `agents/reviewer.md`) even though `mg done`
squash-merges the whole branch into one commit, so per-task commit history is
discarded at merge time anyway.

**2. "Every agent always commits" is solved host-side, not by trusting agent
instructions.** Agents demonstrably don't follow commit instructions
reliably (that is the user's complaint), and the analyst is *by design*
unable to commit at all. The robust fix is a host-side sweep: after any
job-worktree session ends, the launcher commits whatever the agent left
uncommitted (`git add -A` + commit, reusing the existing `git.CommitAll`
helper and the TUI "c" key's message convention `[<id>] chore: commit all`).
This guarantees the worktree is clean when the session returns, so
`mg done`'s clean-tree check stops being tripped by agent leftovers — in
interactive sessions, `mg jobs` launches, and every `mg jdi` invocation
alike. The read-only agents' outputs (analyst's `tasks.md`) get committed by
the host after their session, which is exactly the "no agent feels
responsible" gap the user describes.

**3. The sweep is a job-worktree-only behavior.** Gate on `root.Job != ""`
(resolved job name). Plain sessions and `--prompt` sessions (main worktree,
where the user's own uncommitted work lives) are never swept. `mg host`
sessions are out of scope (no isolation by design, the human supervises
directly). The sweep must also never run when the container didn't actually
start (docker-level failure) — an agent that never ran should not trigger a
commit.

**4. The verdict commit convention is load-bearing and stays.** `mg jdi`'s
retry budget and re-review decisions count commits matching
`^[<id>] verdict:` (`git.CountVerdictCommits` / `git.LatestCommitIsVerdict`).
Relaxing commit hygiene must therefore NOT remove the reviewer's
`[ID] verdict: <one-line summary>` commit instruction — only the *task*-commit
format discipline that is pure noise. A sweep commit (`[<id>] chore: commit
all`) deliberately does not match the verdict pattern, so the state machine is
unaffected.

**5. `mg done`'s clean-tree guard stays as a backstop.** With the sweep in
place the guard should rarely fire; keeping it catches non-agent leftovers
(e.g. a human editing files in a job worktree, or a session killed by a
SIGINT that reached the `mg` process itself before the sweep ran). The TUI's
manual "c" commit-all remains the remedy for those residuals. Changing the
guard to auto-commit would mask real problems and is explicitly out of scope.

## Task breakdown

TASK-1: Relax the reviewer's commit-discipline check in `agents/reviewer.md`
— replace the "Commit discipline" bullets (per-task `[ID] TASK-N:` commit
format, implementation.md's own commit) with a note that commit history is
squashed at `mg done`, so message/format hygiene is not a review criterion;
explicitly KEEP the `[ID] verdict: <one-line summary>` commit instruction for
verdict.md (the mg-jdi state machine counts those commits) and the final
"commit verdict.md" step.
  - files: `agents/reviewer.md`
  - depends: none
  - risk: low — doc-only; the one constraint is preserving the verdict-commit
    convention that `git.CountVerdictCommits`/`LatestCommitIsVerdict` rely on.

TASK-2: Soften `agents/developer.md`'s per-task commit strictness — relax
"one commit per task, this is not optional" (and the exact-format example) to
"commit when a task is complete — one commit per task is the recommended
pattern, exact format not required"; add the load-bearing rule that the
developer must leave the worktree clean when finishing (nothing uncommitted,
including the analyst's leftover `tasks.md`); keep the existing final
`implementation.md` summary + commit step.
  - files: `agents/developer.md`
  - depends: none
  - risk: low — doc-only; reworded requirements, no behavior change to any Go
    code.

TASK-3: Add the host-side job-worktree sweep and wire it into both session
paths. New helper (suggest `internal/session.SweepJobWorktree(root Root, diag
io.Writer)`, or the same logic in `internal/git` — implementer's choice): no-op
when `root.Job == ""`; otherwise derive the job id from the job name
(`<id>_<slug>`, split on the first `_`, fall back to the whole name) and call
`git.CommitAll(root.ProjectRoot, fmt.Sprintf("[%s] chore: commit all", id))`,
swallowing `git.ErrNothingToCommit` (clean tree) and `git.ErrNotARepo` (the
non-git job fallback), warning on stderr for any other error, and printing a
short "committed leftover changes" note when it did commit. Call it after the
container run returns in `cmd/mg/session.go` `runSession` (covers bare `mg`,
`mg jobs` launches, and TUI-launched interactive sessions, which re-exec the
session path) and in `cmd/mg/jdi.go` `commandAgentRunner.Run` after `inv.Run`
(covers every `mg jdi` --print invocation, including the analyst whose
`tasks.md` this commits). Sweep only when the session actually ran the
container — no sweep when docker itself failed to launch.
  - files: `internal/session/` (new helper file or addition), `cmd/mg/session.go`,
    `cmd/mg/jdi.go`
  - depends: none (uses the existing `git.CommitAll`, which already handles
    `.opencode/`/`.claude/`/`.manigot/jdi-status/` exclusions and untracked
    leftovers)
  - risk: medium — a behavior change at the core session seam; the gating
    (job-worktree-only, container-actually-ran) and the stderr reporting must
    be exact, and the mg-jdi stall/retry probes must be re-checked against the
    new post-run commit (they read HEAD after `runner.Run`, so the sweep
    commit correctly counts as agent progress).

TASK-4: Unit-test the sweep. New tests following the repo's existing patterns
(`internal/git/commitall_test.go`, `internal/ui/commitall_test.go`): a job
worktree with a leftover modified tracked file (plus a new untracked file and
a deleted tracked file) is swept into one `[<id>] chore: commit all` commit
and left clean; a clean worktree produces no commit and no error
(`ErrNothingToCommit` swallowed); a non-job root (plain session) is a no-op;
a non-repo root is a no-op; the id is derived correctly from an `id_slug`
name; a sweep failure surfaces as a warning, not an abort. Hook-level tests
if practical (runSession/jdi runner with a stubbed container run).
  - files: new test files alongside the sweep helper (e.g. `internal/session/*_test.go`)
  - depends: TASK-3
  - risk: low — test-only, real git against throwaway repos like the rest of
    the suite.

TASK-5: Sync the documentation. Per the hard rule "Keep `agents/*.md` and
`project-template/docs/AGENTS.md` in sync with whatever this file documents":
document the new always-committed job-worktree behavior in `docs/AGENTS.md`
(and the repo-root `/workspace/AGENTS.md` copy, which is byte-identical
today) — a "Job worktrees are kept committed" note in/after the "Read-only
git mount" or "Job lifecycle" section: every job-worktree session ends with a
host-side sweep-commit of leftover changes, so `mg done`'s clean-tree check
isn't tripped by agent leftovers and read-only agents' outputs (analyst's
`tasks.md`) are committed by the host after their session. Mirror the short
version into `project-template/docs/AGENTS.md`, and touch the README's
workflow section (steps 6–7 "committing as it goes") to mention leftovers are
auto-committed at session end.
  - files: `docs/AGENTS.md`, `AGENTS.md`, `project-template/docs/AGENTS.md`, `README.md`
  - depends: TASK-3 (describes its behavior)
  - risk: low — doc-only; keep the two AGENTS.md copies in sync and the
    template in line with the main doc.

## Notes for the developer

- The brief's "less strict git guideline" is fully covered by TASK-1 + TASK-2
  (agent instructions) and TASK-3 (the system no longer *needs* strict agent
  commits to keep `mg done` working).
- The `.opencode/`/`.claude/` mount-target exclusion (`git.ExcludeMountTargets`
  + `.gitignore`) and the `.manigot/jdi-status/` exclusion already protect
  `git add -A` — do not add extra filtering to the sweep.
- `git.WorkingTreeDirty` ignores untracked files; the sweep should therefore
  call `git.CommitAll` unconditionally (it does `git add -A`, catching
  untracked leftovers too) and swallow `ErrNothingToCommit`, rather than
  pre-gating on the dirty check.
- Out of scope (flag back if any turn out to be needed): changing `mg done`'s
  clean-tree guard to auto-commit; sweeping `mg host` sessions; making the
  analyst agent itself commit; changing `git.CountVerdictCommits` /
  `LatestCommitIsVerdict`; new CLI flags.
- Open questions for the implementer (choose conservatively, document the
  choice in implementation.md): the exact home/signature of the sweep helper
  (internal/session vs internal/git); how to detect "the container actually
  ran" from `DockerInvocation.Run`'s exit code (e.g. have Run report whether
  it exec'd docker successfully, or accept that a docker-launch failure
  sweeping pre-existing leftovers is harmless — but the *preferred* reading of
  the brief is: no sweep when no agent ran).