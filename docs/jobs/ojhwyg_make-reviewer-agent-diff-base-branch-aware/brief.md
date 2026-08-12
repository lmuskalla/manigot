# Brief: make reviewer agent diff base-branch aware

status: open
type: fix
id: ojhwyg
branch: fix/ojhwyg_make-reviewer-agent-diff-base-branch-aware
date: 2026-08-12
author: Leander Muskalla

## What

`agents/reviewer.md` instructs the reviewer to run `git diff main...HEAD`
(line 17) to see a job's changes. For any project whose base branch is not
`main` (e.g. `baseBranch: development` in `.manigot/manigot.json`), this
diffs against the wrong ref — the review surface is wrong or empty, and the
reviewer's verdict is meaningless.

Make the reviewer agent resolve the base branch the same way the tooling
does: read `baseBranch` from `.manigot/manigot.json` (the same guarded
single-key extraction the scripts use), falling back to `main` when the key
is absent. The diff command becomes `git diff <base>...HEAD`.

Also audit the other agents (`agents/*.md`) for any similar hardcoded base
branch / `main` assumptions in git commands and fix any found. Known result
of the audit: only `reviewer.md` has one; the others use branch-agnostic
`git branch --show-current` verification only.

## Why

- `git diff main...HEAD` is wrong for any project with a non-main base
  branch. The reviewer must review the same surface the job was cut from.
- The fix job rkj9qc (job branch prefix configurable) verified the
  end-to-end `mg done` flow against a `development` base — the reviewer
  would still have compared against `main`, making its review wrong in the
  exact scenario that job enabled.

## Out of scope

- Changing historical archived verdicts / run logs that mention
  `git diff main...HEAD` — those are records, not instructions.
- Renaming `main` anywhere else in the tooling (scripts, TUI, docs) — they
  already honor `baseBranch` or default to `main` correctly.

## Notes

- `.manigot/manigot.json` is tracked and present inside job worktrees, so
  the reviewer can read it directly.
- The reviewer runs inside the job's own worktree, on the job branch — the
  same `find_project_root`-less direct read the scripts use relative to the
  project root applies; the file is at `.manigot/manigot.json` in the
  worktree.
- Keep the change minimal: one step in reviewer.md's "How to start", plus a
  short note explaining the resolution. The other agents need no change but
  the audit result should be recorded in implementation.md.
