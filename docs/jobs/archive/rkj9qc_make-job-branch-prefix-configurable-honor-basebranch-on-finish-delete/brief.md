# Brief: make job branch prefix configurable; honor baseBranch on finish/delete

status: open
type: feature
id: rkj9qc
branch: feature/rkj9qc_make-job-branch-prefix-configurable-honor-basebranch-on-finish-delete
date: 2026-08-12
author: Leander Muskalla

## What

Two related problems, one job.

**Problem 1 — branch name collision with pre-existing namespaces.** A project
may legitimately have a long-lived branch named exactly `feature`, `fix`, or
`chore` (no slash). Git stores refs as filesystem paths, so a plain branch
`feature` occupies the file `refs/heads/feature`, which makes it impossible to
also create `refs/heads/feature/<anything>` (that path would need `feature` to
be a *directory*). `mg job` currently hardcodes the job branch as
`${JOB_TYPE}/${ID}_${SLUG}` (e.g. `feature/ati6um_jtl-typ-selektor`), so such a
project cannot create any feature-type job — git fails with the cryptic
"cannot lock ref 'refs/heads/feature/…': 'refs/heads/feature' exists".

The job branch prefix must become project-configurable, so a project that
cannot rename its pre-existing `feature` branch can still use the job
workflow. Everything else in the toolchain (mg done / mg delete / mg jdi /
TUI discovery / run.sh --job) resolves a job by the `<id>_<slug>` tail segment
(`${b##*/}`), never by the prefix, so changing the prefix does not break any
resolver. The type stays recorded in brief.md (`type: feature`), so nothing
depends on it being in the branch name.

**Problem 2 — finish/delete ignore baseBranch.** `finish-job.sh` and
`delete-job.sh` resolve their merge/switch target from
`git symbolic-ref refs/remotes/origin/HEAD` (fallback `main`), ignoring the
`baseBranch` key in `.manigot/manigot.json` that `mg job` and the TUI's `m`
shortcut already honor. A project configured with `baseBranch: development`
therefore has its finished jobs squash-merged into `main` — work lands on the
wrong branch and vanishes from where the team integrates.

## Why

- Problem 1: a 20-year-old project with many developers has a branch named
  `feature` that cannot be renamed. The tool must adapt to the project, not
  force the project to adapt to the tool's naming convention.
- Problem 2: `baseBranch` is the documented project convention for where work
  integrates; finish/delete must use the same convention, not a remote-default
  guess that happens to be `main`.

## Out of scope

- Renaming any branch in any project.
- Auto-detecting collisions and silently choosing a different prefix — the
  prefix must be explicit configuration, never magic.
- Per-type prefixes (e.g. separate `featurePrefix`/`fixPrefix` settings) — one
  namespace setting covers all three job types.
- Changing how the reviewer agent computes diffs (`git diff main...HEAD` in
  agents/reviewer.md) — tracked separately, not part of this job.

## Notes

- The `baseBranch` settings plumbing (job 6kbt43) is the architectural
  template: `project.Settings` field, TUI settings form row, guarded single-key
  `sed` read in the bash script, seeded template file, docs sync.
- `mg jdi` (tui/cmd/jdi) never merges and never checks out in the main
  worktree, so it is unaffected by Problem 2; it resolves jobs the same way
  the scripts do (id_slug matching), so it is unaffected by Problem 1.
