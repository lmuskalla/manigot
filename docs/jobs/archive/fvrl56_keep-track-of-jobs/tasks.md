# Tasks: keep track of jobs

id: fvrl56
status: open
analyst: @analyst
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Decisions on the coupled scope ("act on a job from another branch")

The brief locks discovery as cross-branch and git-backed, and explicitly
requires a design answer for acting on a job whose branch isn't the one
currently checked out. The decision below covers all three mutating paths
(launch agent, edit brief, mark done) under one mechanism, rather than the
launch-only framing in the brief — because once discovery is cross-branch the
detail view's read/edit/done paths break for the same root reason, so they need
one coherent answer instead of three ad-hoc ones.

**Decision:** add a single in-TUI "switch to this job's branch" action
(detail-view key `c`) that runs `git checkout <job.Branch>` and refreshes, and
guard all three mutating actions (launch / edit / mark-done) so that on a
branch mismatch they refuse and point the user at `c` instead of silently
operating on the wrong branch's working tree.

**Why this over the brief's other launch option** ("prepend `git checkout &&
sc --agent …` to the spawned command"):

- One branch-switching mechanism, surfaced in the TUI's own status line where
  checkout errors (e.g. uncommitted changes blocking the switch) are actually
  visible. The prepend option runs the checkout in the detached launch window,
  whose output the TUI cannot observe, leaving the TUI's notion of the current
  branch stale until a manual refresh.
- No silent global git mutation triggered as a side effect of "launch an
  agent". With the guard, the working tree only changes when the user
  explicitly asks (`c`), so the TUI's state stays coherent with reality.
- Fully brief-sanctioned for launch: "guard against it with an explicit
  prompt/error" is one of the two options the brief names. We pick it for
  coherence with edit/done, which can't reasonably self-checkout anyway (they
  run in-process via `tea.ExecProcess` against the working tree).
- Honours the TUI's reason for existing (the brief's "Why"): the user never
  has to drop to a shell and remember `git checkout` syntax — `c` does it, and
  the guard tells them when they need to.

The cost is one extra keypress to act on a foreign-branch job (`c`, then the
action key). That trade is judged worth it for coherence and no-silent-mutation.
The "prepend checkout on launch" behaviour remains a possible follow-up if the
extra keypress proves annoying in practice.

`Discover(root)` keeps its signature, so the call sites in `tui/main.go` and
`tui/internal/ui/app.go` (refresh, new-job) are unchanged.

## Task breakdown

TASK-1: Add a small exec-backed git plumbing package `tui/internal/git` so the
job/launch/ui packages can ask git about branches and per-branch file contents
without each shelling out ad-hoc. Expose at minimum: `LocalBranches(root)`
(list `refs/heads/*` short names, via `git for-each-ref` or `git branch`);
`CurrentBranch(root)` (the checked-out branch, returning a sentinel/empty for
detached HEAD); `ListJobDirs(root, branch)` (top-level `docs/jobs/*` directory
names on that branch excluding `archive/`, via `git ls-tree`); and
`ShowFile(root, branch, path)` (file bytes via `git show <branch>:<path>`). All
via `git -C <root>`. Must degrade gracefully — not-a-repo, no commits yet,
detached HEAD, and a missing branch/path return empty/`os.ErrNotExist`-style
errors rather than crashing, mirroring how `job.Discover` already tolerates a
missing `docs/jobs`.
files: tui/internal/git/git.go (new), tui/internal/git/git_test.go (new)
depends: none
risk: medium — foundational; every later task leans on its correctness, and it
must handle the not-a-repo / detached-HEAD / missing-path edge cases the brief
lists. Tests need throwaway git repos (init + commit + branch).

TASK-2: Rewrite `job.Discover` to enumerate jobs from every local branch via
TASK-1 instead of only the working tree. For each branch, list its job dirs,
read each `brief.md` via `git show` (or from the working tree when the job is on
the currently-checked-out branch, so uncommitted brief edits still show — the
brief's last edge case), and build a `Job` with `Branch` set to the branch it
was found on. Add a bytes-based brief parser entry point (or a constructor) so a
brief can be parsed without a disk read, and a snapshot field on `Job`
(e.g. `OnCurrentBranch bool`) so downstream code can pick its read strategy
without re-querying git. Keep the existing date-desc / name-tiebreak sort and
the not-a-repo fallback to today's working-tree behaviour. No dedup yet — a job
on N branches may appear N times until TASK-3.
files: tui/internal/job/discover.go, tui/internal/job/job.go, tui/internal/job/discover_test.go (new), tui/internal/job/job_test.go
depends: TASK-1
risk: high — the heart of the feature; the working-tree-vs-git split and
populating `Branch`/`OnCurrentBranch` correctly are subtle. Tests need a
multi-branch temp git repo. Dedup is deliberately split out (TASK-3) to keep
this reviewable.

TASK-3: Dedup the enumerated list by job ID, keeping the copy whose `Branch`
equals the job's own `branch:` frontmatter field. This handles the brief's
"a job dir appearing on multiple branches (stale branch after a merge)" edge
case: the frontmatter-named branch is authoritative, so the stale copy is
dropped and the job is listed once. Keep the date-desc/name sort after dedup.
files: tui/internal/job/discover.go, tui/internal/job/discover_test.go
depends: TASK-2
risk: medium — the multi-branch-same-job and stale-branch-after-merge cases are
the tricky part; pure data logic, well-suited to table tests.

TASK-4: Make the detail view and stage computation work for jobs that aren't on
the current branch. Today both assume the working tree: `detailView.loadTab`
does `os.ReadFile(j.Dir/<file>)` and `job.Stage()`/`FileIsWritten` stat
`j.Dir/tasks.md` etc. — so a cross-branch job shows all four tabs as "not
written yet" and always reports stage `analyze`. When `Job.OnCurrentBranch` is
false, read the four files via TASK-1 `ShowFile` and have `Stage()` check
per-branch presence via git instead; current-branch jobs keep working-tree
reads so uncommitted edits still appear. The detail view's existing stale/cache
and resize logic must keep behaving.
files: tui/internal/ui/detail.go, tui/internal/job/stage.go, tui/internal/job/stage_test.go, tui/internal/ui/detail_test.go
depends: TASK-1, TASK-2
risk: medium — touches the file-read and stage paths that currently hard-assume
the working tree; the current-branch path must not regress.

TASK-5: Surface each job's branch in the UI so the user understands which jobs
are "elsewhere". Add the branch to the detail view's meta line, and mark
list rows whose job is not on the current branch (a compact tag/dim marker —
not a new column, to keep the existing column layout stable).
files: tui/internal/ui/detail.go (meta line), tui/internal/ui/app.go (renderJobRow), tui/internal/ui/detail_test.go
depends: TASK-2
risk: low — presentational/formatting only.

TASK-6: Add the "switch to this job's branch" action (detail-view key `c`),
the single mechanism the decision above rests on. It runs `git checkout
<job.Branch>` and, on success, re-runs `job.Discover` and rebuilds the open
detail view against the same job (now reading from the working tree, so its
uncommitted edits show). Checkout failures (e.g. uncommitted changes blocking
the switch) must be surfaced in the status line and must not corrupt state.
Capture the current branch fresh at action time (a `launch`/`edit` may have
changed it) rather than trusting a discovery-time snapshot. Run as a `tea.Cmd`
returning a message (not inline in `Update`) so the UI doesn't block on git.
files: tui/internal/git/git.go (a `Checkout(root, branch)` helper, or reuse exec directly here), tui/internal/ui/app.go (updateDetail "c" case + a checkoutMsg), tui/internal/ui/detail.go (status/refresh hook), tui/internal/ui/*_test.go
depends: TASK-1, TASK-2
risk: medium — mutates global git state from a TUI action and triggers a full
re-discover; must handle checkout failure and keep the open job selected.

TASK-7: Guard the three mutating actions — launch agent, edit brief ("e"),
mark done ("D") — against a branch mismatch. When the open job's branch differs
from the current branch (re-checked fresh), each must refuse and set a status
pointing the user at `c` (e.g. "on branch <cur>, this job is on <job> — press c
to switch") instead of launching against / editing / archiving the wrong
branch. Current-branch jobs are unaffected (byte-identical behaviour). This
replaces the earlier "prepend `git checkout` to the launch command" idea — see
the Decisions section for why.
files: tui/internal/ui/app.go (updateDetail agent-key path, "e", "D"), tui/internal/ui/agents_test.go, tui/internal/ui/editordone_test.go, tui/internal/ui/donemsg_test.go
depends: TASK-2, TASK-5, TASK-6
risk: low–medium — additive guard around existing actions; must not block or
alter the current-branch path, and the launch path's test-pinned command string
must not change (this is precisely why the prepend-checkout approach was
rejected).

TASK-8: Build and test verification plus a doc-sync check. Run `go build ./...`
and `go test ./...` under `tui/`. Confirm no doc sync is needed: the job/branch
model is unchanged by this work, so `agents/*.md`, `project-template/docs/AGENTS.md`
and the READMEs should require no edits — verify that assumption rather than
trust it. Also manually confirm the brief's exact symptom: create jobs on
separate branches, `git checkout main`, and verify the TUI now lists all of
them (today it lists zero).
files: none (verification)
depends: TASK-1…TASK-7
risk: low — verification only; surfaces any regressions from the above.

## Explicitly not covered by this breakdown

- Changing the branch-per-job model, index/sidecar files, or anything in the
  brief's "Out of scope" (remote branches, archive handling, rewriting
  `new-job.sh`/`finish-job.sh`). Archived jobs remain excluded via the
  `archive/` filter carried into TASK-1's `ListJobDirs`.
- A list-level "switch to the cursor job's branch" shortcut (without opening
  it). `c` is detail-view-only for v1; a list-level binding is a trivial
  follow-up if wanted.
- Batching the per-job `git show` calls into fewer git invocations. Per-branch
  `ls-tree` plus one `show` per job is acceptable for v1; batching is an
  explicit follow-up if profiling warrants it.
- The "prepend checkout on launch" variant — intentionally rejected (see the
  Decisions section); revisit only if the extra `c` keypress proves annoying.
