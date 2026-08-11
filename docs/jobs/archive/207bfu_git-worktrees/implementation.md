# Implementation: git worktrees

id: 207bfu
status: open
developer:
date:

## Summary

Gave every job its own git worktree instead of sharing the single host
working tree that `scripts/run.sh` mounts. `new-job.sh` now creates the job's
branch as its own worktree at
`<dirname(PROJECT_ROOT)>/.manigot-worktrees/<basename(PROJECT_ROOT)>/<id>_<slug>`
(a sibling of the project root, never nested inside it), and
`run.sh`/`finish-job.sh`/`delete-job.sh`/`mg jdi`/the TUI all resolve and
operate against that worktree dynamically via `git worktree list` matched by
branch — never against whatever branch happens to be checked out in the main
working tree. This removes the checkout race the brief's "Why" section named
(an interactive `mg` session on job A racing a `mg jdi` run on job B), and
lets multiple jobs run in parallel. The TUI's entire branch-guard subsystem
(`branchGuard`, the "b"/"m" checkout keys, the off-branch row/detail styling)
became unnecessary and was deleted outright rather than adapted.

## Changes

TASK-1: added `scripts/lib/worktree.sh`, a shared bash helper
(`worktree_path_for_branch <root> <branch>`) that prints a branch's worktree
path via `git worktree list --porcelain`, matched against the full ref. Sourced
by the four bash scripts that need it (new-job/run/finish-job/delete-job) so
all agree on one lookup.

TASK-2: `scripts/new-job.sh` now creates the job's worktree
(`git worktree add <path> -b <branch> <base-branch>`) instead of checking out
the new branch in `PROJECT_ROOT`; every file-write and the scaffold commit
operate against the worktree, and `PROJECT_ROOT` itself is never switched. The
non-git-repo fallback (skip branch/worktree, write scaffold into
`PROJECT_ROOT`) is unchanged.

TASK-3: `scripts/run.sh`'s `--job` resolution rewritten: the job is resolved
by matching its id_slug against local branch names, the matched branch's
worktree is looked up via `scripts/lib/worktree.sh`, and `PROJECT_ROOT` is
reassigned to that worktree so the `docs/` mount, context-file mount,
`.env`-shadow scan, and primary `-v ...:/workspace:z` mount all key off the
same resolved root. A branch match with no worktree is a hard error (no
directory-scan fallback, which would silently mount the wrong job's content).
The diagnostic banner shows the resolved worktree path.

TASK-4: `scripts/finish-job.sh` runs the clean-tree check, archive move, and
archive commit inside the job's own worktree; switches to `PROJECT_ROOT` (the
main worktree) only for the squash-merge + branch delete; then removes the
worktree (`git worktree remove`) with a best-effort `git worktree prune`.

TASK-5: `scripts/delete-job.sh` does its clean-tree check against the job's
worktree, extends the confirmation to explicitly warn when the worktree has
uncommitted changes that will be discarded, and removes it with
`git worktree remove --force` (plus best-effort prune) before deleting the
branch. The non-git project path (plain directory delete) is unchanged.

TASK-6: added `git.WorktreeForBranch(root, branch) (path string, ok bool, err
error)` to `tui/internal/git` — the Go-side equivalent of TASK-1's bash lookup
— with the package's existing degrade conventions and porcelain-parsing test
coverage (including the prefix-branch cross-match guard).

TASK-7: rewrote `tui/internal/job.Discover` to enumerate open jobs from
`git worktree list` (each worktree is a job, read straight off its own disk);
deleted `OnCurrentBranch`, `briefBranch`, `dedupByID`, and the `git show` read
path from `job.go`/`discover.go`/`stage.go`. `Job.Dir` is now unconditionally
the live, correct place to read a job's four files from. `discoverWorkingTree`
(the non-repo fallback) kept as-is. Note: Discover enumerates **worktrees
only** — it does not scan `PROJECT_ROOT/docs/jobs/archive/`; closed jobs are
intentionally never listed, exactly as before this job (archives are read via
the filesystem only when explicitly opened, e.g. by the detail view's archive
tab). (Note: TASK-7's commit left the TUI and `cmd/jdi` build broken until
TASK-8/11 removed the last `OnCurrentBranch` references — an expected
intermediate state across the dependency chain.)

TASK-8: deleted the TUI's branch-guard machinery from `tui/internal/ui`:
`branchGuard`, `checkoutCmd`, `blockedByBranchCmd`, `checkoutMsg`,
`branchFlash`/`branchFlashGen`/`branchFlashDoneMsg`, the detail view's "b"
(switch-to-job-branch) case, the list view's "b" (base-branch quick checkout)
case, the "other branch" meta-line styling in `detail.go`, both footers'
now-stale "b" hints, `jobByID`/`indexOfJob`, and the branch tag on list rows.
Every previously-guarded action (`e`, `D`, `x`/delete, `j`, agent launch) now
runs unconditionally. `detail.go`'s `readFile` is a single unconditional disk
read. `commitBriefCmd` was made worktree-aware (verified against real git: a
`..` pathspec from the main worktree is rejected with "outside repository"
and would commit against the wrong branch/index): it now derives the job
worktree root from `job.Dir` and runs `git -C <worktree> add/commit` with the
path relative to that root. `branchguard_test.go` and `checkout_test.go`
deleted; all other ui tests converted from the old commit-into-main-tree
pattern to real worktree fixtures (`addJobWorktree`). Also removed the now-dead
`git.ShowFile`/`git.ListJobDirs` (TASK-7's deferred cleanup, confirmed unused
after `detail.go` dropped its git-show read) plus their tests.

TASK-9: confirmed `tui/internal/hostcmd`'s `NewJob`/`DoneCommand`/
`DeleteCommand` `cmd.Dir`/`$PWD` handling needs no change — the underlying
scripts resolve the job's own worktree internally, and `new-job.sh`'s stdout
format didn't change in a way the tests assert on. No code change.

TASK-10: `tui/internal/launch` — no code change needed (`cd projectRoot && mg
...` is correct because `run.sh` re-derives the effective worktree root from
`--job` itself); added doc-comment notes to `Agent`/`Jdi` capturing that
reasoning.

TASK-11: deleted `ensureOnBranch` and its `git.Checkout` call from
`tui/cmd/jdi/main.go` — the exact checkout race the brief's "Why" section
exists for. Updated the `output.go` `agentTargetFile` comment that justified
reading `j.Dir` via `ensureOnBranch` (it's safe unconditionally now), and
removed the stale `OnCurrentBranch` field from `main_test.go`'s fixture.

TASK-12: manual bash-side end-to-end verification against a throwaway
project. Verified: `mg job` creates the worktree at Decision 1's path with
`PROJECT_ROOT` staying on the base branch; `run.sh --job` mounts the job's own
worktree (via the diagnostic banner and a stub docker recording the mount
args); the worktree-less-branch hard error fires with no fallback; `mg done`
squash-merges into `main`, removes the worktree, and deletes the branch;
`mg delete` warns on a dirty worktree and force-removes it; the non-repo
fallback and jobless `run.sh` behave exactly as before. Full findings recorded
as an addendum to `tasks.md`.

TASK-13: `go build ./...` / `go vet ./...` / `go test ./...` all pass under
`tui/`. Updated `docs/AGENTS.md` (run.sh's job-worktree resolution, the
worktree lifecycle in new-job/finish-job/delete-job, the removed base-branch
quick-checkout bullet, the Job workflow section) and `README.md` (the
"discovered across every local branch"/"press c to switch" paragraph, the
keybindings table's removed "b" rows, the command table, and the mg-jdi
autonomous-mode description). `scripts/mg.sh`'s usage lines updated to mention
worktrees. `project-template/docs/AGENTS.md` deliberately not touched.

## Known issues / follow-ups

- `git.Checkout` in `tui/internal/git` is now dead code (its only two callers,
  the TUI's `checkoutCmd` and `mg-jdi`'s `ensureOnBranch`, were deleted by
  TASK-8/11). Left in place with its tests because the breakdown's cleanup
  list named only `git.ShowFile`/`git.ListJobDirs`; a follow-up could remove
  it.
- No docker is available in this environment, so TASK-12's `run.sh` checks
  were verified through the resolution logic and a stub docker rather than a
  real container launch. The worktree resolution itself is real (real git,
  real worktrees); only the container run was stubbed.
- `tasks.md`'s revision note predicted the list view's "b" (née "m") base
  branch quick-checkout would be removed "rather than just re-naming it" —
  confirmed and done: both "b" keys are gone entirely (TASK-8).
- Disk usage / GC: no new cleanup command was built (per Decision 7); orphaned
  worktrees remain the user's responsibility to manage, with best-effort
  `git worktree prune` folded into `finish-job.sh`/`delete-job.sh`.

## Review round 2 (verdict rework)

The reviewer found three blockers and two documentation gaps in the first
pass; all fixed:

- **`mg done` on a pre-worktree job (branch checked out in the main worktree)
  died at `git worktree remove`** with "fatal: '<root>' is a main working
  tree". `finish-job.sh` now compares the resolved worktree against
  `git rev-parse --show-toplevel` and skips the removal step (the branch
  delete alone suffices) when they match.
- **`mg delete` had the same main-worktree failure** at
  `git worktree remove --force`. `delete-job.sh` skips it the same way, after
  switching the main worktree off the job branch.
- **Non-git `run.sh --job` regression**: a job created via new-job.sh's kept
  non-git fallback (flat `docs/jobs/`, no branches, no worktrees) could no
  longer be launched. `run.sh` now falls back to the pre-worktree
  directory-scan resolution when the project has no local branches at all,
  mirroring `job.discoverWorkingTree`'s trigger condition.
- **TUI Discover skipped the main worktree**, so a transitional pre-worktree
  job (the current one) was invisible to the TUI and to `mg jdi`'s
  `resolveJob`. `Discover` now scans the main worktree like any other
  worktree; in steady state it contributes nothing (its `docs/jobs/` holds
  only `archive/`, excluded), and a directory under `docs/jobs/` only counts
  as a job if it has a `brief.md`, so the `.jdi-status` sidecar and stray
  dirs are not mislisted.
- implementation.md's TASK-7 claim that Discover reads closed jobs from
  `PROJECT_ROOT/docs/jobs/archive/` was wrong — corrected above: Discover is
  worktree-only; archives are never listed (unchanged from before this job).

Net effect: a pre-worktree job (branch checked out in the main worktree) is
fully operable — the TUI lists it, `mg --job`/`mg jdi` resolve it, and
`mg done`/`mg delete` finish/delete it — which is what tasks.md's "needs no
special handling" claim was really asserting.
