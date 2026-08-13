# Tasks: Close the lifecycle hole: orphaned-worktree detection and cleanup

id: nepbxu
status: open
analyst:
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

<!-- TASK-1: Add orphaned-worktree detection in the job package: scan both
     .manigot-worktrees layouts (sibling and nested) for directories whose
     .git file names a gitdir that no longer exists; never report a live
     worktree (existing gitdir), a standalone repo (.git directory), or a
     .git-less stray dir; degrade gracefully on non-repo/no-worktrees.
     files: internal/job/orphan.go, internal/job/orphan_test.go
     depends: none
     risk: low — pure filesystem scan, no git mutation

TASK-2: Add removal with mg delete's confirmation discipline ("This cannot be
     undone." + Proceed?), per-item, declining to ErrCancelled; mirror
     git worktree prune by also pruning stale metadata after removal.
     files: internal/job/orphan.go, internal/job/orphan_test.go
     depends: TASK-1
     risk: low — mirrors DeleteJob's existing confirmation pattern

TASK-3: Surface orphans in mg jobs: list them after the job list, offer
     removal on a TTY, print a mg delete hint otherwise; share one
     bufio.Reader across the confirm and the job selection.
     files: cmd/mg/jobs.go, cmd/mg/jobs_test.go
     depends: TASK-2
     risk: medium — the listing flow already re-execs into a session; must not
     lose buffered input between prompts

TASK-4: Resolve orphans in mg delete: fall back to exact-then-prefix orphan
     matching when DeleteJob reports job-not-found, with a live job always
     winning; distinguish not-found via a sentinel error.
     files: cmd/mg/delete.go, internal/job/finish.go, internal/job/delete.go,
            cmd/mg/lifecycle_test.go
     depends: TASK-2
     risk: low — the not-found error path is the only new branch

TASK-5: Sync docs (docs/AGENTS.md, README.md) with the orphan surfacing.
     files: docs/AGENTS.md, README.md
     depends: TASK-3, TASK-4
     risk: low — doc-only

TASK-6: Verify with go build, go vet, and the full test suite; exercise the
     confirmation workflow on synthetic dead-dir fixtures.
     files: (none — verification)
     depends: TASK-3, TASK-4
     risk: low
-->
