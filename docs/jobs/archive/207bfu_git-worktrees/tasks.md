# Tasks: git worktrees

id: 207bfu
status: open
analyst: claude (architect pass, revised after scoping conversation)
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Revision note

This replaces the first pass of this breakdown after two scoping decisions
made directly with the author, both incorporated below rather than left as
open questions:

1. **No backward compatibility.** manigot has exactly one user and one
   currently-open job (this one) — there is no installed base to keep
   working for. Every job gets a worktree, unconditionally, with no legacy
   fallback path anywhere. This removes an entire axis of complexity every
   task below would otherwise have needed (a worktree-vs-legacy branch in
   `run.sh`, `finish-job.sh`, `delete-job.sh`, `mg-jdi`, and the TUI).
2. **Full discovery modernization.** The cross-branch, `git show`-based job
   discovery built for `fvrl56_keep-track-of-jobs` — and the TUI's
   `branchGuard`/"b" switch-branch/"m"→"b" base-branch-checkout machinery it
   motivated — existed specifically to cope with only one shared working
   tree existing at a time. Worktrees remove that constraint entirely, so
   this job now also removes that machinery rather than adapting it. This is
   a bigger cut than the original brief scoped on its own, called out here
   explicitly since it deletes a working, previously-shipped subsystem.

Also folded in: this branch's merge of `main` renamed the list view's base-
branch quick-checkout key from "m" to "b" (`08b20fc`/`0213dc6`/`15d0caa`/
`ffe6d86`, merged in `a1306ff`) — moot now, since Decision 5 below removes
that action entirely rather than keeping it under either name.

## Decisions this breakdown locks in

**1. Worktree location — sibling to `PROJECT_ROOT`, not under
`docs/jobs/<id>_<slug>/` or anywhere else inside it.** Path:
`<dirname(PROJECT_ROOT)>/.manigot-worktrees/<basename(PROJECT_ROOT)>/<id>_<slug>`.
Nesting a second full checkout of the whole repo inside a subdirectory that
is itself tracked content of the repo is structurally awkward and would need
its own gitignore handling on every branch. Keeping worktrees outside
`PROJECT_ROOT` entirely means the main worktree's own `git status`/
`git add -A` never has to know or care that worktrees exist alongside it.
Matches the ordinary git convention of `git worktree add ../repo-feature-x`.

**2. Worktree lookup — dynamic via `git worktree list`, matched by branch,
not a path independently recomputed by every touch point.** Five places need
to agree on "does job X have a worktree, and where": `new-job.sh` (create),
`finish-job.sh`/`delete-job.sh` (locate + remove), `run.sh` (locate + mount),
and the TUI (`job.Discover`, launch, hostcmd). Decision 1's naming
convention is used exactly once, at creation time; every other touch point
asks git directly (`git worktree list --porcelain`, matched against the
job's own branch). Because this parsing is genuinely fiddly to get right and
a mistake risks mounting or deleting the wrong directory, it gets one shared
bash helper (`scripts/lib/worktree.sh`, sourced by the four bash scripts
that need it) rather than four independent copies — a deliberate, narrow
exception to this codebase's existing per-script duplication convention. The
Go side gets its own equivalent, `tui/internal/git.WorktreeForBranch`.

**3. Lifecycle — worktrees are the only mechanism, no fallback.**
`new-job.sh` creates the worktree (`git worktree add <path> -b <branch>
<base-branch>`) instead of `git checkout -b` in `PROJECT_ROOT`; every
subsequent step in that script (scaffold files, scaffold commit) operates
against the worktree, and `PROJECT_ROOT` is never switched at all.
`finish-job.sh` does the archive-move + archive-commit inside the job's
worktree, switches to the *main* worktree (`PROJECT_ROOT`) only for the
squash-merge + branch delete (unchanged from today otherwise), then removes
the job's worktree. `delete-job.sh` does its uncommitted-changes check
against the worktree and, if dirty, extends its existing "This cannot be
undone" confirmation to say so explicitly, then force-removes the worktree
(`git worktree remove --force`) and deletes the branch from the main
worktree. In steady state `PROJECT_ROOT` is now just "the main worktree,
sitting on the base branch" — individual job work never happens there.

**4. `run.sh`'s mount-root resolution — single path, hard error if a job has
no worktree.** Because `PROJECT_ROOT` stays on the base branch in steady
state (Decision 3), an open job's `docs/jobs/<id>_<slug>/` never exists in
`PROJECT_ROOT`'s working tree at all — it only ever exists inside its own
worktree. So `--job` resolution changes shape, not just gains an override:
resolve `<JOB>` by matching it against local branch names (branches embed
the `id_slug`, e.g. `feature/207bfu_git-worktrees`), look up that branch's
worktree via Decision 2's helper, and mount it. There is no directory-scan
fallback for *git* projects (Decision 1 of the "no backward compatibility"
revision) — a branch match with no worktree is a hard error with a clear
message (an inconsistent state that should only happen if worktree creation
itself partially failed; falling back to silently mounting `PROJECT_ROOT`
instead would be actively misleading, since that's the *wrong* job's
content). The one exception is a project with **no local branches at all**
(not a git repo, or a fresh repo before its first commit): there no worktrees
are even possible, and new-job.sh's kept non-git fallback wrote the job
straight into `PROJECT_ROOT/docs/jobs/`, so `--job` falls back to the
pre-worktree directory-scan resolution and `PROJECT_ROOT` is left untouched
— mirroring `job.discoverWorkingTree`'s trigger condition. (This exception
was added in review round 2 after the reviewer caught the regression: the
kept non-git fallback made jobs that could be created but not launched.)
`PROJECT_DOCS_DIR`/`CONTEXT_MOUNT`/the `.env`-shadow scan/the primary
`-v ...:/workspace:z` mount all key off the same resolved root.

**5. TUI/job-package modernization — worktree-native discovery, branch-
switching machinery deleted outright.** `job.Discover` no longer walks every
local branch and reads other branches' files via `git show`
(`fvrl56_keep-track-of-jobs`'s mechanism) — that existed to handle a job
living on a branch that isn't checked out anywhere, which can no longer
happen once every open job has its own worktree. Instead: open jobs are
enumerated from `git worktree list` (each worktree *is* a job, read straight
off its own disk), including the main worktree itself when it happens to hold
a job (the transitional pre-worktree case — see the review-round-2 note
below). Closed/archived jobs are **not** enumerated at all: Discover is
worktree-only and archives were never listed, before this job or after it
(the detail view reads an archive dir only when explicitly opened). This
deletes `OnCurrentBranch`, `briefBranch`, `dedupByID`, and the whole
`git show` read path from `job.go`/`discover.go`/`stage.go` — a `Job.Dir` is
now unconditionally the live, correct place to read its four files from, no
branch check needed anywhere. Consequently
`tui/internal/ui/app.go`'s `branchGuard`, `checkoutCmd`, `blockedByBranchCmd`,
`checkoutMsg`, the branch-flash machinery, and *both* "b" keys (the detail
view's switch-to-job-branch and the list view's base-branch quick checkout,
née "m") are deleted, not adapted — there is no "wrong branch checked out"
state left to guard against or switch away from. Every previously-guarded
action (`e` edit, `D` done, `x`/delete, `j` mg-jdi, agent launch) now runs
unconditionally. `P` (push) is unaffected — it never depended on any of this.
The non-repo/no-branches fallback (`discoverWorkingTree`) is untouched: a
project that isn't a git repo at all can't have worktrees, so it keeps
working exactly as it does today (a single flat `docs/jobs/` directory, no
branch concept at all).

**6. `mg-jdi` drops its own branch checkout entirely.** `ensureOnBranch`
(`tui/cmd/jdi/main.go`) exists today specifically because `run.sh` bind-
mounted `PROJECT_ROOT` as-is, so `mg-jdi` had to `git checkout` the job's
branch there itself before invoking any agent — the exact concurrency race
the brief's "Why" section names as this whole job's reason to exist. Once
Decision 4 makes every `mg --print` invocation resolve its own correct
worktree per call, that checkout is not just unnecessary but actively wrong
to keep: it would still mutate `PROJECT_ROOT`'s checked-out branch for no
reason. Deleted outright, not made conditional.

**7. Disk usage / GC — no new cleanup command.** The lifecycle in Decision 3
(create on `mg job`, remove on `mg done`/`mg delete`) is the only management
mechanism this job builds. `git worktree prune` is folded in, best-effort,
wherever `finish-job.sh`/`delete-job.sh` already remove a worktree, but
nothing proactively scans for or reports orphaned worktrees (e.g. a job
directory deleted by hand outside `mg delete`). Documented as the user's own
responsibility.

## Task breakdown

TASK-1: `scripts/lib/worktree.sh` — a new shared bash helper implementing
Decision 2's lookup: given a project root and a branch name, print that
branch's worktree path (via `git worktree list --porcelain`) or nothing if
none exists. Sourced by TASK-2/3/4/5 below.
files: scripts/lib/worktree.sh (new)
depends: none
risk: medium — the porcelain-parsing correctness (branch names containing
`/`, a locked/prunable worktree entry, no worktrees at all) every other
bash-side task in this breakdown relies on.

TASK-2: `scripts/new-job.sh` — create the job's worktree instead of checking
out the new branch in `PROJECT_ROOT` (Decision 1 for the path, Decision 3
for the sequencing): `git worktree add <path> -b <branch> <base-branch>`,
then point `JOB_DIR` and every file-write/`git add`+commit step at the
worktree, not `PROJECT_ROOT`. `PROJECT_ROOT` itself is never switched. Keep
the existing non-git-repo fallback (skip branch/worktree creation entirely,
warn, write the scaffold into `PROJECT_ROOT` as today) unchanged.
files: scripts/new-job.sh
depends: TASK-1
risk: medium-high — the job-creation path; every downstream script and the
TUI's discovery assume a job's worktree was created correctly here.

TASK-3: `scripts/run.sh` — rewrite `--job` resolution per Decision 4: resolve
the job by branch-name match plus TASK-1's worktree lookup, mount that
worktree, and hard-error with a clear message if the matched branch has no
worktree. For a project with no local branches at all (non-git, or a fresh
repo before its first commit — new-job.sh's kept non-git fallback), fall back
to the pre-worktree directory-scan resolution instead (see Decision 4's
exception note; added in review round 2). Thread the resolved root through the
`docs/` mount, context-file mount, `.env`-shadow scan, and the primary
`-v ...:/workspace:z` mount. Update the diagnostic banner (`Root`/`Docs`
lines) to show the resolved worktree path.
files: scripts/run.sh
depends: TASK-1, TASK-2
risk: high — rewires the one invocation path every interactive session,
agent launch, and `mg jdi` run goes through.

TASK-4: `scripts/finish-job.sh` — operate the clean-tree check, archive
move, and archive commit against the job's worktree; switch to
`PROJECT_ROOT` only for the squash-merge + branch delete step (unchanged
otherwise); remove the worktree (`git worktree remove`) after a successful
merge, folding in a best-effort `git worktree prune` (Decision 7).
files: scripts/finish-job.sh
depends: TASK-1, TASK-3
risk: medium — this script already runs the trickiest multi-step git
sequence in the codebase; adding worktree removal without breaking the "one
clean commit per job" property, or leaving a half-removed worktree behind on
a mid-sequence failure, needs care.

TASK-5: `scripts/delete-job.sh` — same worktree-aware clean-tree check
(Decision 3); extend the existing confirmation prompt to explicitly warn
when the worktree has uncommitted changes that will be discarded; remove the
worktree with `git worktree remove --force` (folding in a best-effort
`git worktree prune`) before deleting the branch.
files: scripts/delete-job.sh
depends: TASK-1
risk: medium — destructive path; must not partially complete (branch
deleted but worktree registration left behind, or vice versa).

TASK-6: `tui/internal/git.WorktreeForBranch(root, branch) (path string, ok
bool, err error)` — the Go-side equivalent of TASK-1's bash lookup (Decision
2), following this package's existing degrade conventions.
files: tui/internal/git/git.go, tui/internal/git/git_test.go
depends: none
risk: low-medium — read-only git plumbing following an established package
pattern; main risk is the same porcelain-parsing edge cases TASK-1 has to
get right, solved independently here.

TASK-7: `tui/internal/job` — full discovery/model modernization (Decision
5), the largest single code change in this breakdown: rewrite `Discover` to
enumerate open jobs from `git worktree list` (via TASK-6) reading each
straight off its own worktree disk — including the main worktree when it
holds a job (see the review-round-2 note below). Closed/archived jobs are
intentionally **not** enumerated (Discover never scans
`PROJECT_ROOT/docs/jobs/archive/`; archives are not listed, exactly as before
this job). Delete `OnCurrentBranch`,
`briefBranch`, `dedupByID`, and the `readJobOnBranch`/`git show` machinery
from `job.go`/`discover.go`; collapse `stage.go`'s `fileWritten`/`readFile`
to a single unconditional disk read via `Job.Dir`. Remove `git.ShowFile` and
`git.ListJobDirs` from `tui/internal/git/git.go` once confirmed unused
elsewhere (`go build`/`go vet` after the rewrite is the check). Keep
`discoverWorkingTree` (the non-repo fallback) as-is. Update every
`_test.go` file this touches (`job_test.go`, `discover_test.go`,
`stage_test.go`, `git_test.go`).
files: tui/internal/job/job.go, tui/internal/job/discover.go,
tui/internal/job/stage.go, tui/internal/git/git.go, plus the corresponding
_test.go files
depends: TASK-6
risk: high — rewrites the job-discovery model this codebase has used since
`fvrl56_keep-track-of-jobs`; nearly every other TUI file that reads a `Job`
depends on the shape this task gives it.

TASK-8: `tui/internal/ui` — delete `branchGuard`, `checkoutCmd`,
`blockedByBranchCmd`, `checkoutMsg`, `branchFlash`/`branchFlashGen`/
`branchFlashDoneMsg`, the detail view's "b" (switch-to-job-branch) case, the
list view's "b" (base-branch quick checkout) case, the "other branch" meta-
line styling in `detail.go`, and both footers' now-stale hint text. Every
previously-guarded action (`e`, `D`, `x`/delete, `j`, agent launch) drops its
`branchGuard` check and runs unconditionally. Remove `jobByID`/`indexOfJob`
too if they become unused once `checkoutMsg`'s handler (their only caller)
is gone — verify via `go build`. `P` (push) and everything else in
`renderJobRow` besides the branch tag are untouched.
files: tui/internal/ui/app.go, tui/internal/ui/detail.go — deletes
tui/internal/ui/branchguard_test.go and tui/internal/ui/checkout_test.go
outright, updates tui/internal/ui/detail_test.go and
tui/internal/ui/list_test.go
depends: TASK-7
risk: high — this is the actual "remove the friction point" deliverable the
brief exists for; must confirm nothing else in the action-dispatch chain
still implicitly assumes a branch check happened.

TASK-9: `tui/internal/hostcmd` — confirm `NewJob`/`DoneCommand`/
`DeleteCommand`'s `cmd.Dir`/`PWD` handling needs no change (still
`PROJECT_ROOT`; TASK-2/4/5 make the underlying scripts resolve a job's own
worktree internally). Adjust only if TASK-2's `new-job.sh` stdout format
changes in a way `NewJob`'s existing tests assert on.
files: tui/internal/hostcmd/hostcmd.go (change only if needed),
tui/internal/hostcmd/hostcmd_test.go
depends: TASK-2, TASK-4, TASK-5
risk: low — expected to be a no-op or test-only change.

TASK-10: `tui/internal/launch` — confirm `Agent`/`Quick`/`AgentQuick`/`Jdi`'s
`cd projectRoot && mg ...` / `cmd.Dir = projectRoot` pattern needs no change
(TASK-3's `run.sh` re-derives the effective worktree root itself from
`--job`); add a doc-comment note capturing this reasoning.
files: tui/internal/launch/launch.go (comments only, expected)
depends: TASK-3
risk: low — expected to be a no-op change.

TASK-11: `tui/cmd/jdi/main.go` — delete `ensureOnBranch` and its
`git.Checkout` call outright (Decision 6); update the doc comments that
justified it (including `output.go`'s `agentTargetFile` comment, which cites
`ensureOnBranch` as the reason reading `j.Dir` is safe — it's safe
unconditionally now, per TASK-7). Separately confirm (no code change
expected) that `output.go`'s `ensureSidecarIgnored` is still needed: it is —
a jobless `--agent`-only session (`launch.AgentQuick`/`Quick`) still mounts
`PROJECT_ROOT` directly and could still sweep the sidecar into a commit via
`git add -A`, independent of the job-branch scenario this task removes.
files: tui/cmd/jdi/main.go, tui/cmd/jdi/main_test.go, tui/cmd/jdi/output.go
(comment only)
depends: TASK-3, TASK-7
risk: medium — this is the concrete fix for the race described in the
brief; must confirm every agent invocation really does land in the right
worktree per TASK-3 before removing the checkout that used to guarantee it.

TASK-12: Bash-side end-to-end verification (no automated test harness exists
for the shell scripts in this repo). Against a real throwaway project:
create a job and confirm the worktree exists at Decision 1's path and
`PROJECT_ROOT` stays on the base branch; run an interactive `--job` session
and confirm it operates inside the worktree; `mg done` it and confirm the
squash-merge and worktree removal both succeed; repeat `mg job` +
`mg delete` with an uncommitted change left in the worktree to confirm the
force-remove and its warning; also confirm the non-repo fallback in
`new-job.sh`/`job.Discover` (Decision 5's `discoverWorkingTree`) still works
unchanged. Findings written up as an addendum to this file or to
`implementation.md`'s "Known issues."
files: none (manual verification)
depends: TASK-2, TASK-3, TASK-4, TASK-5
risk: low — verification only, but it's the only thing that proves the
bash-side lifecycle end to end.

TASK-13: Go-side build/test verification and documentation — `go build
./...` / `go test ./...` under `tui/` for TASK-6–TASK-11; update
`docs/AGENTS.md` (Architecture: `run.sh`'s job-worktree resolution, the
worktree lifecycle in `new-job.sh`/`finish-job.sh`/`delete-job.sh`, and —
this bullet is already stale independent of this job, since it still
describes the pre-merge "m" key — remove the base-branch-quick-checkout
bullet entirely rather than just re-naming it, per Decision 5) and
`README.md` (the "discovered across every local branch"/"press c to switch"
paragraph, which is also already stale on the key name — rewrite to
describe the worktree-per-job model with no branch-guard concept at all, and
the keybindings table's now-removed "b"/"m" rows). `project-template/docs/AGENTS.md`
deliberately not touched — it describes a project's own context, not
manigot's internals.
files: docs/AGENTS.md, README.md
depends: TASK-1 through TASK-12
risk: low — docs + verification only.

## Notes for the developer

- The one currently-open job (this one, `207bfu_git-worktrees`) needs **one
  piece of** special handling, and it is mandatory, not optional: its branch
  is checked out in the *main* worktree (`/workspace` is the only worktree),
  so the code must handle a job whose worktree resolution returns the main
  worktree itself. Concretely: `finish-job.sh`/`delete-job.sh` must skip the
  `git worktree remove` step when `worktree_path_for_branch` resolves to the
  main worktree (`git worktree remove` on the main worktree is always an
  error — "is a main working tree"), and `job.Discover` must scan the main
  worktree like any other (see TASK-7) so the TUI and `mg jdi` keep seeing
  and launching the job. The reviewer caught this the first time around
  (blockers 1/2 and TASK-7 note 2); the round-2 fixes are recorded in
  implementation.md's "Review round 2" section.
- `git.LocalBranches` stays — `git.RecentCommits` still uses it independent
  of job discovery. Only `git.ShowFile`/`git.ListJobDirs` are expected to
  become dead code (TASK-7).

## Explicitly not covered by this breakdown

- Any TUI/terminal launch UX change (tmux, split panes, etc.) — out of scope
  per the brief; TASK-10 confirms `launch.go`'s existing spawn mechanism
  needs no change.
- Auto-merging or any change to how a job's branch gets merged —
  `finish-job.sh`'s squash-merge step is relocated (main worktree only) but
  its actual merge/commit behavior is untouched.
- Any change to the job workflow's four-file structure — untouched.
- Windows support — out of scope per the brief; this breakdown makes the
  same POSIX-path assumptions the existing bash scripts already make
  everywhere else.
- Any new GC/orphan-detection command beyond the best-effort `git worktree
  prune` folded into TASK-4/TASK-5 (Decision 7).

## TASK-12 verification addendum

Manual bash-side end-to-end verification (TASK-12), run against a throwaway
project (`git init`, base branch `main`, `docs/manigot.json` with
`baseBranch: main`) on this branch. Docker is not available in this
environment, so `run.sh`'s `--job` resolution was verified through the
diagnostic banner and a stub `docker` binary that recorded the invocation's
`-v` mount arguments instead of actually running the container — everything
before `docker run` is real.

All findings PASS:

- **`mg job` creates the worktree at Decision 1's path.** After
  `new-job.sh "test worktree feature"`, `git worktree list` shows the new
  worktree at `<dirname(PROJECT_ROOT)>/.manigot-worktrees/<basename>/<id>_<slug>`
  on `feature/<id>_<slug>`, and the scaffold commit lives in the worktree's
  history. `PROJECT_ROOT` stays on `main` (never switched), its working tree
  is untouched, and `git status` there is clean.
- **`run.sh --job` operates inside the job's own worktree.** Resolved via
  `--job <id>` prefix, the banner's `Root`/`Docs` lines and the primary
  `-v <worktree>:/workspace:z` mount (plus the `docs/` mount) all point at the
  worktree, not `PROJECT_ROOT`.
- **Hard error on a worktree-less branch.** Creating a branch by hand
  (`git checkout -b feature/ghost_job`, no worktree) and running
  `run.sh --job ghost_job` fails with "branch ... has no git worktree",
  "Refusing to fall back to mounting ... instead", and exit 1 — no silent
  fallback to the wrong job's content.
- **`mg done` succeeds end to end.** With an APPROVED verdict committed in the
  worktree: the archive move + `status: done` edit + archive commit all happen
  inside the worktree; the squash-merge lands on `main` in `PROJECT_ROOT`;
  the worktree is removed (`git worktree remove` + best-effort prune) and the
  branch deleted. Post-state: only the main worktree remains, the branch is
  gone, and the job is archived at `docs/jobs/archive/<id>_<slug>` on `main`.
- **`mg delete` warns on a dirty worktree and force-removes.** With an
  uncommitted edit in the job worktree, the confirmation explicitly says
  "this worktree has uncommitted changes — they will be discarded", then
  `git worktree remove --force` + prune removes the worktree and `branch -D`
  deletes the branch. Declining the prompt (answering `n`) leaves the job
  fully intact.
- **Non-repo fallback unchanged.** In a directory with `docs/` but no git
  repo, `new-job.sh` prints "not a git repository — skipping branch/worktree
  creation" and writes the scaffold into
  `PROJECT_ROOT/docs/jobs/<id>_<slug>`; `delete-job.sh` deletes that
  directory with no git involved. The Go-side equivalent
  (`job.discoverWorkingTree`) is exercised by the existing
  `job.Discover`/ui test suites, which pass.
- **Jobless `run.sh` unchanged.** Without `--job`, the banner and mounts point
  at `PROJECT_ROOT` exactly as before this job.

### Review-round-2 corrections

The reviewer re-ran this verification live and found two cases the addendum
above missed; both were fixed in review round 2 (see implementation.md's
"Review round 2" section) and the corrected behavior is now:

- **Non-git `run.sh --job` works again.** A job created via new-job.sh's kept
  non-git fallback — no branches, no worktrees, flat `docs/jobs/` — was
  unlaunchable after TASK-3 (run.sh died with "job not found among local
  branches"). With Decision 4's no-branches exception, `run.sh --job <id>`
  falls back to the pre-worktree directory-scan resolution and mounts
  `PROJECT_ROOT`, exactly as before this job.
- **`mg done` / `mg delete` on a main-worktree job (the currently-open
  pre-worktree job) succeed.** When the job's branch is checked out in the
  *main* worktree, `git worktree remove` / `remove --force` on the main
  worktree is always an error ("is a main working tree"). finish-job.sh and
  delete-job.sh now skip the worktree-removal step when
  `worktree_path_for_branch` resolves to the main worktree — the branch
  delete alone suffices, after switching the main worktree off the job branch.
  The addendum's `mg done`/`mg delete` PASSes above exercised the
  job-in-its-own-worktree path; the main-worktree case is what the reviewer
  caught failing.
- **Transitional main-worktree job is visible to the TUI and mg-jdi.** The
  reviewer also noted job.Discover's main-worktree skip made the current job
  invisible to the TUI and unresolvable by `mg jdi --job`. Discover now scans
  the main worktree too (a directory under docs/jobs/ counts as a job only if
  it has a brief.md, so the .jdi-status sidecar and stray dirs are excluded),
  so the current job is listed and launchable until it is finished or
  migrated.
