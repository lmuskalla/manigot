# Verdict: Close the lifecycle hole: orphaned-worktree detection and cleanup

id: nepbxu
status: open
reviewer:
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

The implementation was reviewed against the brief and the task breakdown in
tasks.md. Code read: `internal/job/orphan.go`, `internal/job/finish.go`,
`internal/job/delete.go`, `cmd/mg/jobs.go`, `cmd/mg/delete.go`, the new tests
in `internal/job/orphan_test.go` / `cmd/mg/jobs_test.go` /
`cmd/mg/lifecycle_test.go`, plus the doc changes (`docs/AGENTS.md`,
`README.md`). Verified with `go build`, `go vet`, `go test -count=1 ./...`
(15/15 packages pass) and an end-to-end run of the built binary against a
scratch repo with dead worktree dirs in both the sibling and nested layouts
(including a live worktree and a standalone-repo negative case).

TASK-1: PASS
notes: DiscoverOrphans is precise: scans both .manigot-worktrees layouts
(sibling + nested), requires a `.git` pointer file naming a gitdir that no
longer exists, and skips live worktrees (existing gitdir), standalone repos
(.git directory) and .git-less junk. Pure filesystem, degrades gracefully on
non-repo / no-worktrees. Verified end-to-end: only the dead dirs are reported,
the live worktree and standalone repo are not.

TASK-2: PASS
notes: RemoveOrphans mirrors mg delete's confirmation discipline exactly: per-
item listing of what will be removed, "This cannot be undone.", "Proceed?
[y/N]"; a declined answer returns ErrCancelled and stops without touching
further orphans (tested). It also runs git.WorktreePrune after removal, which
closes the reverse direction (registered metadata whose working dir is gone —
git's own "prunable" state) and mirrors `git worktree prune` semantics.

TASK-3: PASS
notes: runJobs surfaces orphans after the job list, offers removal on a TTY
(single "Remove orphaned worktrees? [y/N]" after "This cannot be undone.") and
prints a `mg delete <name>` hint on non-TTY. The orphan confirm and the job
selection share ONE bufio.Reader, avoiding the buffered-input loss the cli
package doc warns about. Existing non-TTY refusal behavior (exit 1 when jobs
exist, exit 0 when none) is preserved and covered by the unchanged passing
tests.

TASK-4: PASS
notes: runDelete tries DeleteJob first, so a live job always wins over an
orphan of the same name; only on ErrJobNotFound does it fall back to
MatchOrphan → RemoveOrphans. The new ErrJobNotFound sentinel (with the
jobNotFoundErr shape keeping the pinned error text byte-identical) cleanly
distinguishes "no such job" from real failures, and is used by both the git
and non-git delete paths. Branch-exists-but-no-worktree remains the hard
"inconsistent state" error, not an orphan. Tested at CLI level: exact match,
prefix match, declined, and the regression cases (mg delete/mg done not-found
still print the original wording).

TASK-5: PASS
notes: docs/AGENTS.md (canonical source) and README.md updated — mg jobs /
mg delete bullets, job-lifecycle section, command table, and a Job-workflow
note. agents/*.md and project-template/docs/AGENTS.md don't document the
lifecycle commands, so no sync was needed there.

TASK-6: PASS
notes: go build + go vet + full suite green; the confirmation workflow was
exercised end-to-end on synthetic dead-dir fixtures identical in shape to the
brief's five dirs (the real dirs were not present in this sandbox — noted in
implementation.md).

## Security

No security findings. The removal target is derived from os.ReadDir of the
project's own .manigot-worktrees parents and gated by the .git-pointer check,
so a reported orphan is always a subdirectory of the tool's own worktree
namespace with a dead gitdir — nothing outside it can be deleted through this
path. Confirmation is per-item / batch-confirmed with destructive wording.

## Overall

NEEDS WORK

The code itself is correct, complete and well-tested — but **nothing is
committed**. The branch `feature/nepbxu_close-the-lifecycle-hole-orphaned-worktree-detection-and-cleanup`
contains only the scaffold commit and the brief commit; all 12 files of
implementation (including `internal/job/orphan.go` + tests and the
`implementation.md` update) are uncommitted working-tree changes.

This must change before the job can proceed:

1. Commit each task with its own commit in the required format:
   `[nepbxu] TASK-1: ...` through `[nepbxu] TASK-6: ...` (or equivalent
   per-task granularity matching the tasks.md breakdown).
2. Commit `docs/jobs/nepbxu_.../implementation.md` in its own commit.
3. Leave the working tree clean afterwards — `mg done`'s clean-tree check
   (`internal/job/finish.go:152`) rejects uncommitted changes, so the job
   cannot be finished until this is done.
