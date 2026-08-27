# Verdict: git-strictness

id: precisely
status: open
reviewer: @reviewer
date: 2026-08-25

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Third-pass re-review of `git diff main...HEAD` (base branch per
`.manigot/manigot.json`: `main`) against tasks.md, focused on the follow-up
commits addressing the second NEEDS WORK verdict (48a0d33 → bb5689c →
d042884 → 0597389 → f628c52).

The pre-worktree-job blocker is fixed correctly. `SweepJobWorktree`
(`internal/session/sweep.go:55`) now gates on
`root.Job == "" || root.ProjectRoot == root.InvocationRoot`, replacing the
narrower `GitCommonDir == ""` check. Traced against `root.go`:
`InvocationRoot` is set once, before any `--job` reassignment (root.go:82-83),
and `ProjectRoot` is only ever reassigned away from it in the linked-worktree
path (root.go:149, via `git.WorktreeForBranch`) — never in the flat-scan
fallback (root.go:106-119) and never for a pre-worktree job, where
`WorktreeForBranch` matches the main worktree's own porcelain entry and
returns the main root itself. So `ProjectRoot == InvocationRoot` is true in
both main-worktree shapes and false whenever a real linked worktree was
resolved — exactly the gate the task needs, and it no longer depends on
`GitCommonDir`, which was the point of failure in the previous fix.

Verified independently, not just read: `go build ./...`, `go vet ./...`, and
`go test ./...` all pass, including
`TestSweepJobWorktreePreWorktreeJobNoOp` (unit) and
`TestRunSessionDoesNotSweepPreWorktreeJob` (hook-level), both of which
reproduce the exact previous-review scenario (a job branch checked out in the
main worktree, leftover `work.txt` + `.env`) and assert no commit and no
`chore: commit all` diagnostic. The existing "should sweep" unit tests were
correctly updated to set a distinct `InvocationRoot` so the new gate doesn't
false-negative them (`sweep_test.go`, TestSweepJobWorktreeCommitsLeftovers /
CleanTree / NonRepo / FailureWarns). gofmt flags three files
(`internal/git/commitall_test.go`, `internal/session/root_test.go`,
`internal/ui/tig_test.go`) — confirmed pre-existing on `main` and untouched
by this job's diff (`git diff main...HEAD --stat` shows no changes to any of
the three), so not a blocker introduced here.

Scope check: `git diff main...HEAD --stat` shows exactly the five tasks'
files plus the job scaffold (brief/tasks/implementation/verdict) — nothing
unexplained.

TASK-1: PASS
notes: unchanged since last pass — `agents/reviewer.md:85-92`, commit-hygiene
bullets replaced with the squash-at-`mg done` note, verdict-commit
instruction and final commit step kept.

TASK-2: PASS
notes: unchanged since last pass — `agents/developer.md:54-65`, softened
per-task commit language, leave-clean rule added, final commit step kept.

TASK-3: PASS
notes: `internal/session/sweep.go:55` — gate is now
`root.Job == "" || root.ProjectRoot == root.InvocationRoot`, closing both the
flat-scan fallback and the pre-worktree-job shape uniformly, confirmed
against `root.go`'s actual resolution logic (see Review above). The `(int,
bool)` `ran` signal in `docker.go`, the `ErrNothingToCommit`/`ErrNotARepo`
swallow, warn-not-abort, and the wiring in `session.go`/`jdi.go` are
unchanged from the earlier PASS and remain correct.

TASK-4: PASS
notes: `TestSweepJobWorktreePreWorktreeJobNoOp`
(`internal/session/sweep_test.go`) and
`TestRunSessionDoesNotSweepPreWorktreeJob` (`cmd/mg/session_test.go`) close
the gap flagged in the previous verdict — a pre-worktree-job sweep no-op is
now pinned at both unit and hook level, alongside the flat-scan no-op tests
from the prior round. All tests pass (`go test ./...`).

TASK-5: PASS
notes: `docs/AGENTS.md` / `project-template/docs/AGENTS.md` now name both
main-worktree shapes the sweep excludes (0597389), resolving the previous
verdict's doc/gate mismatch note.

## Security

The `.env`-in-a-commit vector is closed for both main-worktree shapes now:
the flat-scan fallback (previous fix) and the pre-worktree job (this fix).
`TestSweepJobWorktreePreWorktreeJobNoOp` and
`TestRunSessionDoesNotSweepPreWorktreeJob` both explicitly assert an
unignored `.env` stays untracked. No other security-relevant change in this
diff.

## Overall

APPROVED

No blockers. All five tasks pass; the reviewer's two prior NEEDS WORK
findings (flat-scan fallback, then pre-worktree job) are both fixed and
pinned by tests at unit and hook level, and independently verified by running
`go build`/`go vet`/`go test ./...` on this branch.
