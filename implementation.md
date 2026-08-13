## Summary

Fixed the `mg done` "uncommitted changes in the worktree" bug
(`docs/BUG_report-mg-done-dirty-worktree-stale-job-copy.md`): agents were
committing a stale duplicate of the job under `.opencode/jobs/<job>/` (or
`.claude/...`) because the container mounts the project's `docs/` at those
repo-relative paths, and git — having no notion of mount points — tracked
whatever the agent staged through them. The duplicate's later hand-deletion
without a commit left the worktree dirty and `mg done` refused.

The fix implements the report's proposed fix: git is now prevented from ever
tracking the colliding paths, in every manigot-managed worktree, plus a
belt-and-braces shim rule.

## Changes

TASK-1: `internal/git/git.go` — added `ExcludeMountTargets(root)`, which
appends `.opencode/` and `.claude/` to the repository's `.git/info/exclude`
via the existing idempotent `ExcludePath` helper (a non-repo is not an
error). `info/exclude` lives in the repository's common git dir and is shared
by every worktree, so one call protects the main worktree and all job
worktrees of the repo. Verified empirically that `git add .opencode/...`
then fails loudly ("paths are ignored") while `git add .` / `git add -A`
skip the paths. Tests: `TestExcludeMountTargets` (both patterns present,
idempotent), `TestExcludeMountTargetsNotARepo` in
`internal/git/lifecycle_test.go`.

TASK-2: `internal/job/create.go` — every new job worktree is excluded at
creation, right after `git.WorktreeAdd`. Test: `TestCreateJobFullRoundtrip`
asserts the repo's `info/exclude` carries both patterns.

TASK-3: `internal/session/root.go` — every session launch excludes the
resolved root (`ResolveRoot`/`ResolveRootFrom`), covering the main worktree
for non-job sessions and pre-existing job worktrees (created before this fix)
whose next session re-resolves them. The flat-scan `--job` fallback was
restructured slightly (exact-match and prefix-match now share one exit with
the exclusion); wording of all errors unchanged. Tests:
`TestResolveRootExcludesMountTargets`, `TestResolveJobWorktreeExcludesMountTargets`.

TASK-4: `scripts/entrypoint.sh` — the session git shim now refuses `git add`
of a pathspec starting with `.opencode/` or `.claude/` (covering `./`
variants), the belt-and-braces second layer for a worktree the host-side
exclusion hasn't reached. Verified by generating the shim exactly as the
entrypoint does: explicit `.opencode/`/`.claude/` adds are denied, `add .`,
`add -A`, `add docs/...` and commit/log remain allowed.

Docs: `docs/BUG_report-...md` status flipped open → fixed with a Resolution
section; `docs/AGENTS.md`'s "Session git shim" section documents the add
denial and the host-side exclusion.

## Known issues / follow-ups

- Already-tracked `.opencode/jobs/<job>/` entries committed before this fix
  are not untracked by the exclusion (git ignore rules never untrack files);
  those jobs still need the stale deletion committed once, by hand, before
  `mg done` will pass. Out of scope for the preventive fix.
- The empty root-owned `.opencode/`/`.claude/` mountpoint dirs docker leaves
  in worktrees remain (cosmetic; untracked and ignored now). Cleaning them up
  at worktree removal would be a separate change.
