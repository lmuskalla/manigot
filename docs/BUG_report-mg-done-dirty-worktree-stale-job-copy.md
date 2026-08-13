# Bug report: `mg done` keeps failing on "uncommitted changes in the worktree"

Status: fixed (2026-08-13 — see "Resolution" below)
Date: 2026-08-13
Reporter: Leander (via gemeinsam-in-bremen / php8 project)
Severity: high — blocks closing every job whose agent session touched the
mounted docs through the colliding path. Has happened multiple times.

## Symptom

`mg done <job>` aborts immediately with:

```
Error: uncommitted changes in the worktree for branch 'feature/<job>'. Commit or stash before finishing.
```

The error is the clean-tree check in `FinishJob`
(`internal/job/finish.go:149-153`). Running `git status` inside the job's
worktree shows unstaged deletions of a **stale duplicate copy of the job**
under `.opencode/jobs/<job>/…`:

```
 D .opencode/jobs/<job>/brief.md
 D .opencode/jobs/<job>/implementation.md
 D .opencode/jobs/<job>/tasks.md
 D .opencode/jobs/<job>/verdict.md
```

The real, up-to-date job files live (correctly) at `docs/jobs/<job>/`. The
`.opencode/` copies are byte-identical leftovers that were committed on the
job branch by an agent and later deleted from the working tree without a
commit. The unstaged deletion is what trips the dirty check.

## Root cause

A **path collision between the container's docs mount target and the repo's
own `.opencode/` (or `.claude/`) path**.

For OpenCode profiles the session launcher mounts the project's `docs/`
directory into the container at `/workspace/.opencode`
(`internal/session/docker.go:68-72, 97-100`):

```
-v <worktree>/docs:/workspace/.opencode:z
-v <worktree>:/workspace:z
```

Inside the container, `/workspace` is the job worktree and
`/workspace/.opencode` is a bind mount of `docs/`. The job is therefore
reachable in the container at two paths that look like two different copies
but are the same content:

- `/workspace/docs/jobs/<job>/` (the canonical path, given in the job prompt)
- `/workspace/.opencode/jobs/<job>/` (the mounted docs, at the repo-relative
  path `.opencode/jobs/<job>/`)

Git has no notion of mount points: a `git add` (or `git add -A` / `git add .`)
run from `/workspace` on anything under `/workspace/.opencode/…` addresses the
**repo path** `.opencode/…` — and since `.opencode/` is not ignored anywhere,
git happily tracks it. So when an agent stages the job's files via the
`.opencode/…` path it is actually looking at, the files land in the repository
under `.opencode/jobs/<job>/` — a second, stale copy of the job. This is
exactly what the agent commits show ("sync opencode job copy of summary").

The same collision exists for the Claude-Code profile, whose docs mount target
is `/workspace/.claude` (`internal/session/docker.go:69`) → repo path
`.claude/`.

Later, when the duplicate is cleaned up by hand (`rm` the files) without a
`git rm`/commit, the worktree becomes dirty and `mg done` refuses. The empty
`.opencode/` directory left behind is root-owned — created by docker as the
mount point inside the `/workspace` bind mount on every OpenCode session —
which is further confirmation the copies were produced inside a container.

## Evidence (job `sign_validate-php8-update`, repo `gemeinsam-in-bremen`)

- `git log --diff-filter=A -- .opencode/jobs/` on the job branch → the files
  were added by the agent's commit `24e764d "[sign] TASK-1: bump Docker base
  images from PHP 8.2 to PHP 8.4"`.
- That same commit also modified `docs/jobs/<job>/tasks.md` — the agent was
  working the job through both paths at once.
- `git show HEAD:.opencode/jobs/<job>/<f>.md | diff - docs/jobs/<job>/<f>.md`
  → identical for brief/implementation/tasks/verdict, confirming the
  `.opencode/` tree is a pure duplicate produced through the docs mount.
- The worktree's `.opencode/` directory is empty and root-owned (docker
  mountpoint), while everything else in the worktree is user-owned.
- The untracked root-owned `.opencode/` also appears in the **main** worktree
  after any OpenCode session (observed at the repo root), so the collision
  affects non-job sessions too.

## Reproduction

1. Start an OpenCode-profile session on a job (`mg --profile opencode-go
   --job <id>` or `mg jdi --profile opencode-go --job <id>`).
2. Let the agent stage/commit the job's docs — it will reach them at
   `/workspace/.opencode/jobs/<job>/`; `git add .` or an explicit
   `git add .opencode/jobs/...` tracks them under `.opencode/`.
3. Delete the `.opencode/jobs/` files by hand without committing.
4. `mg done <job>` → fails with the uncommitted-changes error.

## Proposed fix

Keep the docs mount (agents must be able to write `tasks.md`,
`implementation.md`, `verdict.md`), but stop git from ever tracking the
mounted docs under the colliding repo path:

1. **Exclude the colliding paths from git in every manigot-managed worktree.**
   Add `.opencode/` and `.claude/` to the worktree's `.git/info/exclude` via
   the existing `git.ExcludePath` helper (`internal/git/git.go:842`,
   currently only used for `.manigot-worktrees/` at
   `internal/job/create.go:166`). Then `git add .` / `git add -A` never pick
   up the mounted docs, and an explicit `git add .opencode/jobs/...` fails
   loudly ("paths are ignored") instead of silently creating the duplicate.
   Do it:
   - at job-worktree creation (`internal/job/create.go`), and
   - at session launch for the resolved project root
     (`internal/session/root.go`), so non-job sessions in the main worktree
     are covered too.
2. Optionally harden the git shim (`scripts/entrypoint.sh`) to refuse
   `git add` on paths starting with `.opencode/` or `.claude/` as a
   belt-and-braces second layer.

## Notes

- The empty root-owned `.opencode/`/`.claude/` mountpoint dirs docker leaves
  in worktrees are cosmetic (untracked, empty) but worth cleaning up at
  worktree removal (`git worktree remove` does not delete them).
- The fix must not break the normal flow where agents write job files through
  the mount: writes to `/workspace/.opencode/jobs/…` still land in
  `docs/jobs/…` and are tracked there — only the git *tracking* of the
  `.opencode/` path is what needs to be prevented.

## Resolution

Implemented 2026-08-13, following the "Proposed fix" above:

1. `internal/git`: new `git.ExcludeMountTargets` helper appends `.opencode/`
   and `.claude/` to the repo's `.git/info/exclude` (via the existing
   `ExcludePath`), idempotently. `info/exclude` lives in the repository's
   common git dir, shared by every worktree, so one call protects the main
   worktree and all job worktrees alike.
2. `internal/job/create.go`: every new job worktree is excluded at creation.
3. `internal/session/root.go`: every session launch excludes the resolved
   root (`ResolveRoot`/`ResolveRootFrom`), covering the main worktree for
   non-job sessions and pre-existing job worktrees whose next session
   re-resolves them.
4. `scripts/entrypoint.sh`: the git shim now refuses `git add` of a pathspec
   starting with `.opencode/` or `.claude/` (belt-and-braces second layer for
   worktrees the host-side exclusion hasn't reached).

Effect: `git add .` / `git add -A` never pick up the mounted docs, and an
explicit `git add .opencode/...` fails loudly ("paths are ignored") instead of
silently creating the stale duplicate. Already-tracked `.opencode/` entries
from before the fix are unaffected by the exclusion (git ignore rules don't
untrack files) — those jobs still need the deletion committed once, by hand.

