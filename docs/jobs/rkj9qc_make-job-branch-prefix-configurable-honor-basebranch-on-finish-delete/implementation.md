# Implementation: make job branch prefix configurable; honor baseBranch on finish/delete

id: rkj9qc
status: open
developer: @developer
date: 2026-08-12

## Summary

Two related product gaps, both about manigot assuming a project's git layout
instead of adapting to it:

1. **Job branch prefix is now project-configurable.** `mg job` previously
   hardcoded job branches as `feature|fix|chore/<id>_<slug>`. A project whose
   git history contains a plain branch named exactly `feature` (or `fix`/
   `chore`) cannot create any job of that type — git stores refs as
   filesystem paths, so `refs/heads/feature` (a file) blocks the whole
   `refs/heads/feature/...` namespace (reproduced: `fatal: cannot lock ref
   'refs/heads/feature/ati6um_...': 'refs/heads/feature' exists`). A new
   `jobBranchPrefix` key in `.manigot/manigot.json` (default empty = today's
   naming) prepends a namespace to the branch: with `"jobs"` a feature job's
   branch is `jobs/feature/<id>_<slug>`. Every resolver (run.sh,
   finish-job.sh, delete-job.sh, TUI discovery, mg jdi) matches jobs by the
   `<id>_<slug>` tail segment, so the prefix needed no resolver changes.

2. **finish/delete now honor `baseBranch`.** `mg done` and `mg delete`
   resolved their merge/switch target from `git symbolic-ref
   refs/remotes/origin/HEAD` (fallback `main`), ignoring the `baseBranch`
   setting that `mg job` and the TUI already honored. Both scripts now read
   `baseBranch` from `.manigot/manigot.json` first, falling back to the old
   `origin/HEAD` → `main` detection when the key is absent. A project that
   integrates on `development` now has finished jobs squash-merged into
   `development`, not `main`.

Plus a pre-flight namespace-collision check in `new-job.sh`: before touching
git it verifies no ancestor path segment of the composed branch name exists
as a plain ref, and fails with a human-readable error (pointing at the
`jobBranchPrefix` setting) instead of git's cryptic `cannot lock ref` fatal.

Verified end-to-end in scratch repos: (a) a project with `main`,
`development` and a plain `feature` branch, `baseBranch: development`,
`jobBranchPrefix: jobs` → `mg job` creates `jobs/feature/<id>_<slug>` off
`development`; `mg done` merges it into `development`, not `main`; the plain
`feature` branch is untouched. (b) Without a prefix, `mg job` in a repo with
a plain `feature` branch fails with the new clear error (exit 1, no worktree
created).

## Changes

TASK-1: Add `JobBranchPrefix` to `project.Settings`
- `tui/internal/project/settings.go` — new `JobBranchPrefix` field
  (`json:"jobBranchPrefix,omitempty"`); package doc updated for two
  conventions. No defaulting method — empty is a meaningful value ("no
  prefix"), unlike BaseBranch.
- `tui/internal/project/settings_test.go` — round-trip test now carries the
  new field.

TASK-2: TUI settings form gains a "Job branch prefix" field
- `tui/internal/ui/settings.go` — new `jobBranchPrefix` text input after the
  base-branch row (both project-scoped); `stFieldCount` 5 → 6 with a new
  `stFocusJobPrefix` constant inserted between `stFocusBranch` and
  `stFocusCount` (count/profile/terminal shifted); wired through
  `newSettingsView`, `resize`, `setFocus`, `update`'s text-input routing,
  `projectValue()`, `render` (project-scoped label + hint), and `hint()`.
- `tui/internal/ui/settings_test.go` — tab/shift+tab cycle tests updated for
  the 6-field cycle; profile/recent-count/terminal tests adjusted for the
  extra tab; new `TestSettingsJobPrefixEdits` and
  `TestSettingsJobPrefixSeededFromProjectSettings`; render string list gains
  the new label.

TASK-3: `new-job.sh` composes prefixed branches
- `scripts/new-job.sh` — reads `jobBranchPrefix` from `.manigot/manigot.json`
  with the same guarded single-key `sed` used for `baseBranch`; branch is now
  `${PREFIX:+${PREFIX}/}${JOB_TYPE}/${ID}_${SLUG}`; header/usage comments
  updated. Empty prefix preserves today's exact naming.

TASK-4: finish/delete honor baseBranch
- `scripts/finish-job.sh` — `DEFAULT_BRANCH` (the squash-merge target) now
  reads `baseBranch` from `.manigot/manigot.json` first, falling back to the
  previous `origin/HEAD` → `main` detection. Comment updated.
- `scripts/delete-job.sh` — same for the "switch the main worktree off the
  deleted branch" target. Comment updated.

TASK-5: Pre-flight namespace-collision check in `new-job.sh`
- `scripts/new-job.sh` — after the base-branch existence check and before
  worktree creation, walks every ancestor path segment of the composed
  branch name (all segments except the `<id>_<slug>` leaf) and fails with a
  clear error if any exists as a plain ref. Only runs in the git case.

TASK-6: Docs sync
- `docs/AGENTS.md` — `new-job.sh` bullet documents `[<prefix>/]<type>/<id>_<slug>`
  naming, the collision rationale, and the pre-check; `finish-job.sh` and
  `delete-job.sh` bullets document the `baseBranch`-first merge target; the
  `.manigot/manigot.json` bullet documents `jobBranchPrefix` alongside
  `baseBranch`; `mg job`/`mg done` command lines and the Job workflow section
  updated for prefixed branches and the baseBranch-aware merge.
- `README.md` — "Branch naming" line documents the optional prefix.
- `project-template/docs/AGENTS.md` deliberately untouched (generic
  per-project placeholder, per the 6kbt43 precedent). `agents/*.md` verified
  to need no change.

## Known issues / follow-ups

- `agents/reviewer.md` still hardcodes `git diff main...HEAD` — a real
  follow-up for a base-branch-neutral reviewer, explicitly out of scope for
  this job (noted in brief.md). A project whose baseBranch is `development`
  gets reviewer diffs against the wrong ref.
- `new-job.sh`'s `jobBranchPrefix` and `baseBranch` reads are two guarded
  single-key `sed` extractions; a third project key would justify switching
  to `jq` (comment already notes this).
- The TUI settings form seeds `jobBranchPrefix` from `proj.JobBranchPrefix`,
  and `mg job`/`finish-job.sh`/`delete-job.sh` read it from disk directly —
  consistent with how `baseBranch` already works (TUI writes the file, the
  scripts re-read it at call time).
