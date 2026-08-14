# Implementation: show mg diff in tui

id: pick
status: open
developer: @developer
date: 2026-08-14

<!-- Produced by @developer after implementation. -->

## Summary

Added a computed **diff** tab to the TUI's job detail view — the 6th tab
(index 5, after `log`, key `6`), mirroring `mg diff`'s default "quick
eyeball" (`git log --oneline <base>...<branch>` + `git diff --stat
<base>...<branch>` via the existing internal/git helpers), so a job's
changes can be reviewed in the TUI before `D` done. The base branch resolves
exactly as `mg diff` does (project.Load BaseBranch → git.SymbolicRefHead),
and the tab degrades to plain-text placeholders for no-branch / no-changes /
git-error instead of crashing. Recomputes on every `ctrl+r`. Docs synced
(README.md + docs/AGENTS.md).

## Changes

TASK-1: Added the diff tab to the detail view.
- `internal/ui/detail.go` — new `isDiff` flag on `fileTab`; the 6th tab
  appended in `newDetailView` (label "diff", never editable, own viewer);
  `loadTab` handles the `isDiff` case via the new `loadDiff` helper, which
  computes the quick eyeball from `git.LogOneline` + `git.DiffStat` over
  `<base>...<branch>`, and the new `diffBaseBranch` helper resolving the base
  exactly like cmd/mg/diff.go (`project.Load(root).BaseBranch` → fallback
  `git.SymbolicRefHead(root)` — deliberately not `BaseBranchValue()`).
  Placeholders: no branch → "_this job has no branch to diff (not a git
  worktree job)._" (exists=false, dimmed "(diff)" tab like the log tab's
  no-run state); undiverged branch → "No changes on <branch> relative to
  <base>." (exists=true); git error → "_could not compute the diff: <err>_"
  (exists=false).
- `internal/ui/detail_test.go` — `TestDetailViewHasFiveTabsIncludingLog`
  became `TestDetailViewHasSixTabsIncludingLogAndDiff` (6 tabs,
  tabs[5].label == "diff", not editable).

TASK-2: Wired navigation/chrome for the 6th tab.
- `internal/ui/detail.go` — `6` key → `d.cur = 5` in `detailView.update`;
  footer hint "tab/1-5 files" → "tab/1-6 files".
- `internal/ui/agents.go` — stale comment "1-5 file/log select" →
  "1-6 file/log/diff select".
- `internal/ui/detail_test.go` — new `TestDetailViewDiffTabKeyBindingSwitchesTab`
  (press "6" → cur 5).

TASK-3: Content tests for the diff tab on real scratch repos (existing
gitInitRepo / addJobWorktree helpers).
- `internal/ui/detail_test.go` — five new tests:
  `TestDetailDiffTabShowsLogAndStatForDivergedBranch` (log + stat rendered),
  `TestDetailDiffTabNoChangesPlaceholderForUndivergedBranch`,
  `TestDetailDiffTabNoBranchPlaceholderForWorkingTreeFallback`,
  `TestDetailDiffTabRespectsConfiguredBaseBranch` (configured `trunk` base
  is used, proven by the diff failing against it), and
  `TestDetailDiffTabRefreshPicksUpNewCommits` (reload → re-computed diff).
- The ui copy of the `gitInitRepo` helper now inits with `-b main`: a
  scratch repo's default branch would otherwise be "master", which the diff
  tab's SymbolicRefHead fallback (no origin/HEAD → "main") would never
  resolve — every happy-path test would have hit the git-error placeholder.
  The helper still reads the branch back, so its contract is unchanged.

TASK-4 (optional, dropped): the full-patch toggle inside the diff tab was
explicitly optional and droppable ("confirm before implementing"), and the
brief only asks to "see the changes" — the quick eyeball satisfies that
minimally. No human was available to confirm, so per the scope assumption it
was dropped rather than guessed at.

TASK-5: Documentation sync.
- `README.md` — keybindings table: `tab` / `1`-`5` → `tab` / `1`-`6` with
  the tab list gaining `diff`; the `e`-edit row's parenthetical now includes
  diff as TUI-computed; new paragraph describing the diff tab after the log
  tab paragraph.
- `docs/AGENTS.md` — the `mg diff` Commands entry notes the TUI's computed
  `diff` tab (key `6`, same base resolution); the "TUI and mg jdi" section
  now says "opens each job's four files plus a computed `diff` tab".

## Known issues / follow-ups

- TASK-4 (full-patch toggle in the diff tab) was dropped as optional — the
  natural follow-up if the full patch is ever wanted in the TUI.
- The ui test copy of `gitInitRepo` diverged from the job package's copy
  (`git init -q -b main` vs `git init -q`); kept deliberately self-contained
  per the helper's own comment, and only the ui copy needed the pin.
- `mg diff` (CLI and now the diff tab) resolves the base to "main" via
  SymbolicRefHead when neither .manigot/manigot.json nor origin/HEAD is set;
  a repo whose default branch is "master" with no origin therefore shows the
  git-error placeholder in the diff tab. This is pre-existing `mg diff`
  behavior, mirrored intentionally.
