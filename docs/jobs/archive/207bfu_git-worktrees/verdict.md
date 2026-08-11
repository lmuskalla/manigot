# Verdict: git worktrees

id: 207bfu
status: open
reviewer:
date: 2026-08-11

## Review

Verified: git branch `feature/207bfu_git-worktrees`, full diff `main...HEAD`
(24 commits: analyst, 13 TASK commits, implementation, round-1 verdict, 5
review-fix commits, implementation addendum). `go build ./...`, `go vet ./...`,
and `go test ./...` all green under `tui/`. Live bash verification of the
worktree-creation path (TASK-2) re-run in this round against a throwaway repo:
worktree lands at Decision 1's path on the job branch, scaffold commit lives
in the worktree's history, `PROJECT_ROOT` stays on `main` with a clean tree,
all four files present. The remaining bash paths (run.sh `--job` resolution,
`mg done`, `mg delete`) were verified live by the round-1 reviewer and
re-verified statically here; the three round-1 blockers are fixed in code.

This is the round-2 verdict. The round-1 verdict (NEEDS WORK, three blockers)
was re-examined against the five `review-fix` commits; all three blockers and
both documentation gaps are resolved as described below.

TASK-1: PASS
notes: scripts/lib/worktree.sh — porcelain blocks matched on the full
`refs/heads/<branch>` ref (prefix branches like `feature/x` vs
`feature/x-y` cannot cross-match), blank-line block termination resets the
path, locked/prunable/detached entries handled (detached has no branch line
and is never matched), non-repo/no-worktree cases print nothing and return 0.
Sourced (not executed) by all four consumers; `set -euo pipefail` correctly
left to the callers. Correct.

TASK-2: PASS
notes: scripts/new-job.sh — `git worktree add <path> -b <branch> <base-branch>`
at `<dirname(PROJECT_ROOT)>/.manigot-worktrees/<basename>/<id>_<slug>`,
JOB_DIR pointed into the worktree, scaffold commit via `git -C "$JOB_DIR" add
.` + commit (stages exactly the four files, lands on the job branch — verified
live this round). `PROJECT_ROOT` is never switched; base-branch verification
(`rev-parse --verify refs/heads/<base>`) kept; non-git fallback (write
scaffold into `PROJECT_ROOT/docs/jobs/`) kept and correct.

TASK-3: PASS
notes: scripts/run.sh — `--job` resolves the job by exact id_slug-segment
branch match, then prefix match with explicit ambiguity error, then
`worktree_path_for_branch`; a matched branch with no worktree hard-errors
("has no git worktree" / "Refusing to fall back to mounting ... instead",
exit 1) with no silent fallback to the wrong job's content. `PROJECT_ROOT` is
reassigned to the worktree so PROJECT_DOCS_DIR, CONTEXT_MOUNT, the `.env`-
shadow scan, the primary `-v ...:/workspace:z` mount, and the banner all key
off the same resolved root; `INVOCATION_ROOT` keeps the original project name
for the banner's Project line. Round-1 blocker 3 fixed: a project with no
local branches at all (non-git, or a fresh repo before its first commit)
falls back to the pre-worktree directory-scan resolution and leaves
PROJECT_ROOT untouched, mirroring `job.discoverWorkingTree`'s trigger
condition. The JOB_PROMPT consistency check is now a hard error (defensive),
not a fallback. Correct.

TASK-4: PASS
notes: scripts/finish-job.sh — clean-tree check, worktree-branch guard, archive
move, `status: done` edit, and archive commit all run inside the job's own
worktree; `PROJECT_ROOT` is used only for `checkout $DEFAULT_BRANCH` +
squash-merge + commit; then the worktree is removed (`git worktree remove`,
plain, since the archive commit left it clean) with best-effort prune, and
finally `branch -D`. Round-1 blocker 1 fixed: when the resolved worktree
equals the main worktree (`git rev-parse --show-toplevel` comparison) the
removal step is skipped — the branch was already switched out of (the earlier
`checkout $DEFAULT_BRANCH`), so the branch delete alone suffices. No
half-removed-worktree path: the branch delete happens only after removal
succeeds (or is correctly skipped). "One clean commit per job" property
preserved (squash merge folds scaffold + archive move into one commit on the
default branch).

TASK-5: PASS
notes: scripts/delete-job.sh — non-git project path (plain `rm -rf`, no git)
checked first via `rev-parse --git-dir` and preserved. Git path resolves the
branch + worktree the same way run.sh/finish-job.sh do; DIRTY check against
the worktree; confirmation explicitly warns "this worktree has uncommitted
changes — they will be discarded" when dirty; `git worktree remove --force` +
best-effort prune, then `branch -D`. Round-1 blocker 2 fixed: when the
worktree resolves to the main worktree the removal is skipped and the main
worktree is first switched off the job branch (`git checkout $DEFAULT_BRANCH`
when `CURRENT_MAIN_BRANCH == BRANCH`), making the branch deletable. Decline
path leaves the job fully intact. One narrow nuance (non-blocking, noted in
round-1's own minor-gap spirit): in the transitional main-worktree case with a
*dirty* main tree, `git checkout $DEFAULT_BRANCH` may carry non-conflicting
uncommitted changes onto the default branch rather than discarding them (or
abort on a conflict, deleting nothing) — the confirmation's "will be
discarded" wording is then not literally what happens. No data loss either
way, and this state is transitional-only (this job itself). Acceptable.

TASK-6: PASS
notes: tui/internal/git WorktreeForBranch — same exact-ref match as the bash
helper, ErrNotARepo degrade, ""/false for no-worktree/missing-branch,
porcelain parsing covered by tests including the prefix cross-match guard
(TestWorktreeForBranchNoCrossMatchOnPrefix) and the not-a-repo case. Solid.

TASK-7: PASS
notes: tui/internal/job — Discover enumerates open jobs from `git worktree
list` via WorktreeForBranch, reading each worktree's `docs/jobs/` straight off
disk; a directory only counts as a job if it has a brief.md (keeps
`.jdi-status` and stray dirs out). Round-1 gap 2 (the big one) fixed:
`TestDiscoverListsTransitionalMainWorktreeJob` confirms the main worktree is
scanned like any other, so the current pre-worktree job stays visible to the
TUI and resolvable by `mg jdi`; `TestDiscoverIgnoresNonJobDirsInMainWorktree`
covers the sidecar/stray-dir exclusion; `TestDiscoverFreshRepoFallsBack` and
`TestDiscoverIsReadOnly` (Discover never switches the main worktree's branch)
round out the coverage. `OnCurrentBranch`/`briefBranch`/`dedupByID`/the
`git show` read path deleted; `stage.go`'s `fileWritten`/`readFile` collapsed
to a single unconditional disk read via `Job.Dir`; `discoverWorkingTree`
(non-repo/no-branches fallback) kept as-is. Round-1 gap 1 (the archive-claim
mismatch) fixed in documentation: Discover is worktree-only, archives are
intentionally never listed (unchanged from before this job), and
implementation.md's "Review round 2" section now states exactly that. Note:
`stage.go` lines ~161/171 still carry two stale doc comments referencing the
removed "off-branch ... git show" path — cosmetic only, the code is a pure
disk read. Two `_test.go` files (job_test.go, stage_test.go) updated
consistently; all pass.

TASK-8: PASS
notes: tui/internal/ui — branchGuard/checkoutCmd/blockedByBranchCmd/
checkoutMsg/branchFlash/branchFlashGen/branchFlashDoneMsg, both "b" keys
(detail-view switch and list-view base-branch quick checkout), the off-branch
meta-line styling, the branch tag on list rows, both footers' stale "b" hints,
and jobByID/indexOfJob all deleted; `e`/`D`/`x`/`j`/agent launch run
unconditionally. branchguard_test.go and checkout_test.go deleted; all other
ui tests converted to real worktree fixtures (`addJobWorktree`, mirroring
new-job.sh's shape) and pass. `commitBriefCmd` is worktree-aware: it derives
the worktree root from `job.Dir` (two hops up from
`<wt>/docs/jobs/<id>_<slug>`) and runs `git -C <worktree> add/commit`, so a
brief edit commits on the job branch, never the main worktree's —
TestEditorDoneMsgAutoCommitsBrief asserts the commit lands in the worktree and
does not leak into the main worktree's history. `P` (push) untouched. The
two-hop derivation is safe for every Discover-produced job (all are exactly
one level under `docs/jobs/`; archives are never listed). Correct.

TASK-9: PASS
notes: tui/internal/hostcmd — no change needed (empty diff), confirmed: the
underlying scripts resolve the job's own worktree internally and new-job.sh's
stdout format is unchanged in the ways the tests assert on.

TASK-10: PASS
notes: tui/internal/launch — no code change; doc comments on Agent/Jdi capture
the reasoning correctly (launch from projectRoot is required because run.sh
re-derives the effective worktree root from `--job` per invocation).

TASK-11: PASS
notes: tui/cmd/jdi — `ensureOnBranch` and its `git.Checkout` call deleted
outright (the exact checkout race the brief's "Why" exists for); output.go's
`agentTargetFile` comment updated to the unconditional-`j.Dir` rationale;
main_test.go's fixture cleaned. Confirmed every agent invocation still lands
in the right worktree: `commandAgentRunner.Run` passes `--job <j.Name>` with
cmd.Dir = projectRoot, and run.sh's `--job` resolution (TASK-3) re-derives the
worktree per invocation — for a job in its own worktree and for the
transitional main-worktree job alike. git.CountVerdictCommits/HeadCommit key
off `root`+branch, which is worktree-agnostic. `ensureSidecarIgnored` kept and
still needed (jobless `--agent`-only sessions still mount PROJECT_ROOT
directly). Correct.

TASK-12: PASS
notes: tasks.md addendum is thorough and its round-2 corrections match the
code exactly: the non-git `run.sh --job` fallback, the main-worktree
`mg done`/`mg delete` skip, and the main-worktree Discover listing are all
implemented as described. I re-ran the worktree-creation leg live this round
(see header); the round-1 reviewer reproduced the rest live. Docker remains
unavailable here, so the container run itself is still stubbed — the
resolution logic and all git operations are real.

TASK-13: PASS
notes: `go build ./...` / `go vet ./...` / `go test ./...` all pass under
`tui/` (re-run this round). docs/AGENTS.md updated: run.sh's job-worktree
resolution, the worktree lifecycle in new-job/finish-job/delete-job, the
base-branch quick-checkout bullet removed (not just renamed), the Job workflow
section rewritten, mg-jdi's per-invocation worktree resolution documented.
README.md updated: worktree-per-job model with no branch-guard concept, both
keybinding tables' "b" rows removed, command table and mg-jdi section
rewritten. scripts/mg.sh usage lines mention worktrees. project-template/docs/
AGENTS.md correctly untouched. One non-blocking note: `agents/developer.md`
and `agents/security.md` still tell agents to `git checkout <branch>` if not
on the correct branch — harmless under the worktree model (the mounted
worktree is always on the job branch, so the instruction no-ops) and not in
the breakdown's file list, but the AGENTS.md hard rule ("keep agents/*.md in
sync") would justify a follow-up wording tweak.

Commit discipline: PASS — one `[207bfu] TASK-N:` commit per task (TASK-9
correctly absent as a no-op), implementation.md has its own commit, the 5
review-fix commits map one-to-one onto the round-1 blockers, and the
intermediate broken-build state between TASK-7 and TASK-8 is acknowledged in
implementation.md. Scope: clean — every changed file maps to the task list or
to this job's own four workflow files.

## Security

No security findings. The worktree path and all git operations are host-side
and parameterized, matching existing conventions; the `.env`-shadow scan now
also covers job worktrees (the correct extension, so a job worktree's own
.env files are shadowed too); the `.jdi-status` sidecar remains outside any
job's worktree and excluded from git.

## Overall

APPROVED

No blockers. The round-1 NEEDS WORK verdict's three blockers are all fixed and
verified: finish-job.sh skips worktree removal for a main-worktree job (with
the branch switched out first), delete-job.sh does the same after switching
the main worktree off the job branch, and run.sh's `--job` restores the
no-branches directory-scan fallback for non-git/fresh-repo projects. The two
documentation gaps (archive-claim mismatch, main-worktree Discover visibility)
are also fixed, with regression tests for each.

Non-blocking notes for a future follow-up (not merge-gating):
- `stage.go` lines ~161/171: stale "off-branch ... git show" doc comments.
- `agents/developer.md`/`agents/security.md`: `git checkout <branch>` advice
  is now a no-op under the worktree model; reword to match.
- `git.Checkout` in tui/internal/git is dead code (kept deliberately per the
  breakdown's cleanup list; documented in implementation.md).
- delete-job.sh's dirty-main-worktree confirmation wording (see TASK-5 notes).
- Fresh-repo (git init, no commits) `mg done`/`mg delete` still fail with a
  clear "job not found among local branches" error — pre-existing failure
  mode (the old code died on the same path with a raw git error), and a
  corner of the explicitly out-of-scope no-backward-compat decision.
