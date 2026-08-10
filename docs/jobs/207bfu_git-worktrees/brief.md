# Brief: git worktrees

status: open
type: feature
id: 207bfu
branch: feature/207bfu_git-worktrees
date: 2026-08-10
author: Leander Muskalla

## What

  Give each job its own git worktree instead of sharing the single host
  working tree that `scripts/run.sh` mounts today. When a job has a worktree,
  `--job <id>` sessions (interactive `mg` and `mg jdi`) should operate against
  that job's own checked-out directory, not whatever branch happens to be
  checked out in the main working tree.

  ## Why

  `scripts/run.sh` currently mounts `PROJECT_ROOT` directly, and
  `new-job.sh`/`finish-job.sh`/`delete-job.sh` all `git checkout` that same
  directory. Only one job's branch can be live at a time. The TUI's
  `branchGuard` (`tui/internal/ui/app.go`) exists specifically to police this —
  it blocks edit/done/delete/jdi actions when the open job isn't the currently
  checked-out branch, and its own comments call this out as a known friction
  point. Worse than friction: it's a correctness risk. `mg jdi` does its own
  checkout when it runs, so a `mg jdi` run on job A racing against an
  interactive `mg` session on job B (or two concurrent `mg jdi` runs) can
  checkout out from under a container that already has the directory mounted
  read-write mid-session. Worktrees remove the race by giving every job its
  own directory to check out into, so multiple jobs — interactive or
  autonomous — can genuinely run in parallel.

  ## Out of scope

  - Any TUI/terminal launch UX changes (tmux, split panes, etc.) — separate job.
  - Auto-merging or any change to how a job's branch gets merged into main.
  - Changing the job workflow stages or file structure (brief/tasks/implementation/verdict).
  - Windows support, if not already a target.

  ## Notes

  Open questions to resolve before this goes to @analyst:
  - Where do worktrees live (e.g. a sibling/hidden dir per job vs. under
    `docs/jobs/<id>_<slug>/`) and how does `run.sh` resolve which path to mount
    when `--job` is passed, instead of always using `PROJECT_ROOT`?
  - Lifecycle: created by `new-job.sh` alongside the branch, removed by
    `finish-job.sh` (on merge) and `delete-job.sh` (on delete). What happens to
    a worktree with uncommitted changes when a job is deleted?
  - Does the TUI's `branchGuard`/`checkoutCmd`/"b" and "m" key flow
    (`tui/internal/ui/app.go`) become unnecessary once every job-workflow
    action targets its own worktree, or does it stay as a fallback for
    non-worktree cases?
  - Disk usage: every job now duplicates a full checkout. Is any cleanup/GC
    needed, or is this the user's problem to manage manually?
  - Does `mg jdi`'s orchestration (`tui/internal/orchestrate`) or
    `launch.Jdi`/`launch.Agent` need to change how they invoke `run.sh`/`mg`
    now that the target directory isn't always `PROJECT_ROOT`?
  - Backward compatibility: projects without `docs/` (no job workflow) keep
    working exactly as today — worktrees only apply where `--job` identifies a
    specific job.
