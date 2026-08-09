# Brief: keep track of jobs

status: done
type: feature
id: fvrl56
branch: feature/fvrl56_keep-track-of-jobs
date: 2026-08-09
author: Leander Muskalla

## What

The TUI must list **all** in-flight jobs across every local branch, not only
those present on the currently-checked-out branch.

Today `tui/internal/job/discover.go` reads the working tree of the current
branch only. The concrete symptom: create 3 jobs (each on its own
`feature|fix|chore/<id>_<slug>` branch), then `git checkout main` — the TUI
shows **zero** jobs. From any single job's branch you see only that one job.
The user is forced to `git checkout` each branch just to remember what's open,
which defeats the TUI's purpose (managing jobs / launching agents without
remembering command syntax).

### Product decision (locked)

Solve this with **cross-branch, git-backed discovery**. A job's docs stay on
its branch (model unchanged); discovery enumerates job dirs from every local
branch's tree instead of only the working tree.

The original idea floated for this job — "keep job folders outside of git
until merge" — is **rejected**. A job's docs (brief → tasks → implementation →
verdict) are the spec and record for the code on that branch; they must travel
with it. Decoupling them would make a branch's history code-without-rationale,
break collaboration (a teammate checking out a job branch would get no brief),
and force a rewrite of `new-job.sh` / `finish-job.sh` and the documented
branch-per-job model — all to fix what is really just a listing problem with a
cheaper fix.

## Why

- The TUI is the front door for "what am I working on?" If it can't show
  in-flight jobs across branches, it fails its core job the moment more than
  one piece of work is open at a time.
- Radically keeping the user oriented — without making them hop branches by
  hand — is the whole point of having a TUI over the raw `sc` scripts.

## Out of scope

- **Changing the branch-per-job model.** Job docs stay on their branch. No
  untracked/out-of-git job docs.
- **Index / sidecar files** (e.g. `docs/jobs/index.json` on main). They add a
  write surface, concurrent-creation merge conflicts, and drift. Do not
  introduce one.
- **Remote branches.** Start with local branches only. Remote enumeration can
  be a follow-up.
- **Archive changes.** Archived jobs are already on the default branch via
  squash-merge and correctly excluded by `Discover`; leave that alone.
- **Rewriting `new-job.sh` or `finish-job.sh`** unless a minimal, clearly
  justified touch is required.

## Notes

### Coupled scope the analyst must design: acting on a job from another branch

The moment discovery is cross-branch, the user can select a job that lives on
branch B while currently on branch A. Launching an agent today
(`tui/internal/launch/launch.go`) does `cd <root> && sc --agent … --job <id>`
with **no branch switch** — so it would run the agent against the wrong branch's
working tree. This feature must pair cross-branch discovery with a clear
answer for "act on a job not on the current branch": either check out the job's
branch as part of launch, or guard against it with an explicit prompt/error.
The interaction needs design, not just a code change.

### Pointers

- Read path (the thing to change): `tui/internal/job/discover.go`,
  `tui/internal/job/job.go`.
- Launch path (the coupled gap): `tui/internal/launch/launch.go` — note it
  builds a `cd <root> && sc …` command with no branch awareness.
- The `Job` struct already carries a `Branch` field parsed from brief.md, so
  discovery can populate which branch each job lives on without new file
  formats.
- Branch → job mapping is available via `git ls-tree --name-only <branch>
  docs/jobs` (top level, exclude `archive/`) and `git show
  <branch>:docs/jobs/<job>/brief.md`.

### Edge cases for the analyst to cover

- A job dir appearing on multiple branches (e.g. a stale branch after a merge):
  dedup by job ID, preferring the branch named in the job's own `branch:`
  frontmatter field.
- Branches that aren't job branches (`main`, manual branches): only consider
  branches that actually contain a non-archive job dir.
- No branches / fresh repo / detached HEAD / empty `docs/jobs`: degrade
  gracefully (already handled for the working-tree path).
- The currently-checked-out job should still reflect the working tree (so
  uncommitted edits to its brief show up), while all other jobs come from git.
