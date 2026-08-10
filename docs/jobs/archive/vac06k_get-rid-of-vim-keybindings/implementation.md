# Implementation: Get rid of vim keybindings

id: vac06k
status: open
developer: claude
date: 2026-08-10

<!-- Produced by @developer after implementation. -->

## Summary

Removed the vim-style `h`/`j`/`k`/`l` key aliases from the TUI's job detail
view. Tab switching now only responds to `tab`/`shift+tab`/`left`/`right`,
and scrolling only to `up`/`down` (plus the already-non-vim `pgup`/`pgdown`/
`g`/`G`). The now-free `j` key was repurposed as the new trigger for
"run mg-jdi" (previously `J`). Documentation (README, `docs/AGENTS.md`) and
tests were updated to match.

## Changes

TASK-1: Removed the `l`/`h` aliases from `detailView.update`'s tab-switching
cases and the `j`/`k` aliases from `fileTab.scroll`'s scroll cases in
`tui/internal/ui/detail.go`. `tab`/`shift+tab`/`left`/`right` and
`up`/`down` remain the only ways to do those two things.

TASK-2: Rebound the "run mg-jdi" action from `J` to `j`: the key-dispatch
case in `updateDetail` (`tui/internal/ui/app.go`), the `[j]`/`mg-jdi`
action-bar button, and the footer hint's `j run mg-jdi` text
(`tui/internal/ui/detail.go`).

TASK-3: Deleted the footer's `j/k scroll` hint text entirely
(`tui/internal/ui/detail.go`'s `renderFooter`) and updated the doc comment
above `agentMeta` (`tui/internal/ui/agents.go`) to drop the mentions of
`h`/`l` file nav and `j`/`k` scroll, and to refer to the mg-jdi launch key
as lowercase `j`.

TASK-4: Updated `tui/internal/ui/jdilaunch_test.go`
(`TestJdiKeyLaunchesDetachedAndSeedsBellDedup`,
`TestJdiKeyReportsResolutionFailure`) and
`tui/internal/ui/branchguard_test.go` (`TestBranchGuardBlocksJdi`) to use
`keyMsg("j")` instead of `keyMsg("J")`, including the surrounding doc
comments describing "the J flow" (also caught one stray comment reference
missed on the first pass and fixed it in a small follow-up commit).

TASK-5: Added `TestDetailVimKeysAreInert` to `tui/internal/ui/detail_test.go`
— asserts scroll position and `d.cur` are unchanged after each of `j`, `k`,
`h`, `l`, contrasted against `up`/`down`/`left`/`right`/`tab`/`shift+tab`
still working as expected.

TASK-6: Updated `README.md`: the detail view keybindings table's scroll row
dropped `j`/`k`, its `J` row (run mg-jdi) became `j`, the `b` row's
"needed before e/D/J/x/agent keys" list became "...D/j/x...", the
`make jdi` build-step comment became `"j"`, and both mentions of `J` in the
"mg-jdi status & log" section became `j`.

TASK-7: Updated `docs/AGENTS.md`'s `tui/cmd/jdi` architecture bullet,
changing "a TUI-launched run (`J` in the detail view)" to "(`j` in the
detail view)". Confirmed via search that `agents/*.md` and
`project-template/docs/AGENTS.md` don't mention this detail, so no change
needed there.

## Known issues / follow-ups

The branch already had a pre-existing "Temporary commit" (Dockerfile /
scripts/run.sh changes adding a `--user` flag and `$HOME` handling) present
before this job's work began. It is unrelated to this job's scope (vim
keybindings) and was left untouched, per the instruction not to expand scope
or refactor unrelated code.

## Post-review fix

The reviewer flagged a stray `[J]` reference left in `renderActionBar`'s doc
comment in `tui/internal/ui/detail.go` (line ~534), which described the
`[j]` mg-jdi button using the old uppercase key. Fixed to `[j]` to match the
actual rendered button and the rest of the renamed key references.
`go test ./...` passes.
