# Implementation: keep track of jobs

id: fvrl56
status: open
developer: @developer
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

Job discovery is now cross-branch and git-backed: the TUI enumerates jobs
from every local branch instead of only the working tree of whichever branch
is currently checked out, so `git checkout main` no longer hides every
in-flight job. Each `Job` now carries the branch it lives on and whether that
branch is the one currently checked out; the detail view and `Stage()` read
off-branch jobs via `git show` while continuing to read the current-branch
job from disk (so uncommitted edits still show). The UI surfaces which jobs
are "elsewhere" (branch tag in the list, meta line + hint in the detail
view), and a new `c` action in the detail view checks out a job's branch
on demand. The three mutating actions (launch agent, `e` edit, `D` mark
done) refuse with a pointer to `c` when the open job's branch doesn't match
the branch actually checked out right now.

## Changes

TASK-1: Added `tui/internal/git` (`git.go`, `git_test.go`) — the only place
in the TUI that shells out to git. Exposes `LocalBranches`, `CurrentBranch`,
`ListJobDirs`, `ShowFile`, and (added ahead of TASK-6, since the checkout
action needed it) `Checkout`. All degrade gracefully: not-a-repo/missing git
binary → `ErrNotARepo`; missing path/branch → `os.ErrNotExist`-wrapped error
or an empty result, matching how `job.Discover` already tolerated a missing
`docs/jobs`.

TASK-2: Rewrote `job.Discover` (`discover.go`) to enumerate every local
branch's job dirs via the git package and build a `Job` per (branch, job)
pair, reading the brief from the working tree when the branch is the
checked-out one and via `git show` otherwise. Added `Job.Branch`,
`Job.OnCurrentBranch`, and the bytes-based `ReadJobFromBytes` constructor
(`job.go`) so a brief can be parsed without touching disk. A non-repo or a
repo with no branches yet falls back to the pre-feature working-tree-only
enumeration (`discoverWorkingTree`), so non-git projects are unaffected.

TASK-3: Added `dedupByID` (`discover.go`) to collapse a job that appears on
more than one branch (e.g. a stale branch left after a merge) to a single
entry, preferring the copy whose discovery branch matches the brief's own
`branch:` frontmatter field (kept in the unexported `Job.briefBranch`).

TASK-4: Made the detail view (`detail.go`) and `Stage()`/`stage.go` branch on
`Job.OnCurrentBranch`: a current-branch job keeps reading its four files via
`os.ReadFile`/file stats; an off-branch job reads them via `git.ShowFile` and
checks per-branch presence via `git ls-tree`/`git show` instead of `os.Stat`,
so its tabs and stage no longer collapse to "not written yet" / `analyze`.

TASK-5: Surfaced the branch in the UI — the detail view's meta line now
shows `· branch: <name>`, flagged as `(other branch — press c to switch)`
when off-branch (`detail.go`); list rows for a job not on the current branch
get a trailing dimmed `· <branch>` tag rather than a new column
(`app.go`'s `renderJobRow`).

TASK-6: Added the `c` detail-view key (`app.go`): it dispatches
`git.Checkout` as a `tea.Cmd` (`checkoutCmd`) rather than running it inline,
so the UI doesn't block on git. The result comes back as a `checkoutMsg`
handled in `Update`: on success it re-runs `job.Discover`, looks the same
job id back up in the refreshed list (`jobByID`/`indexOfJob`), and rebuilds
the open `detailView` against it (now reading from the working tree); on
failure it surfaces `cmdErrorText` in the detail status line without
touching the job list or the open job. `detail.go`'s footer hint gained a
`c switch branch` entry, scoped to off-branch jobs. Tests in
`checkout_test.go`.

TASK-7: Added `App.branchGuard` (`app.go`), which re-checks the current
branch fresh via `git.CurrentBranch` (not the discovery-time
`OnCurrentBranch` snapshot, since `c` or an external checkout may have moved
it since) and compares it to the open job's `Branch`. Wired into all three
mutating call sites in `updateDetail` — the `e` edit case, the `D` done
case, and the agent-key dispatch — each of which now refuses with a
`"on branch <cur>, this job is on <job> — press c to switch"` status instead
of running against the wrong branch's working tree. A job with no known
`Branch` (the non-git working-tree fallback) is never guarded, so that path
is byte-for-byte unchanged. Tests in `branchguard_test.go`.

TASK-8: `go build ./...` and `go test ./...` pass clean under `tui/`
(all packages, `-v` run to confirm every new test executes). Manually
reproduced the brief's exact symptom in a throwaway repo: created 3 jobs each
on its own `feature/<id>` branch, ran `git checkout main` (working tree's
`docs/jobs/` genuinely empty at that point), and confirmed `job.Discover`
still returns all 3 with their correct branches. Checked `agents/*.md` and
`project-template/docs/AGENTS.md` for doc drift — neither describes job
listing/branch scope in enough detail to need a change (the branch-per-job
model itself is unchanged). `README.md`'s TUI section does document exact
keybindings and job-list behavior, and was stale: added a paragraph on
cross-branch discovery/the off-branch row tag, and a `c` row to the detail
view's keybinding table.

## Known issues / follow-ups

- Per the brief/tasks' explicit scope: no index/sidecar file, no remote
  branch enumeration, and no list-level (as opposed to detail-view) `c`
  shortcut — all noted as intentional v1 cuts in `tasks.md`'s "Explicitly
  not covered" section.
- `gofmt -l .` flagged a formatting issue in
  `tui/internal/job/discover_test.go` that was actually introduced by this
  job's own changes (not pre-existing on `main`); fixed with `gofmt -w`.
