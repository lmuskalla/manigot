# Tasks: tui: git panel

id: diamond
status: open
analyst:
date: 2026-08-17

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

<!-- TASK-1: Add a context-bounded `git.MergeBranch` helper (host-side, `git merge --no-edit <branch>` with GIT_TERMINAL_PROMPT=0, ErrNotARepo/wrapErr degrade like Push/CommitAll) plus a unit test with a diverged-branch fixture.
     files: internal/git/git.go (or new internal/git/merge.go), internal/git/merge_test.go
     depends: none
     risk: low — a new isolated function following the existing Push/CommitAllWithContext pattern; the only sharp edge is `--no-edit` so a diverged merge that needs a merge-commit message never hangs on an editor prompt.

TASK-2: Add the "merge default branch" action to the detail flow: resolve the base branch via the shared chain (project.BaseBranch → git.SymbolicRefHead, same as diffBaseBranch/doneConfirmLines), add mergeMsg + mergeCmd that run git.MergeBranch in the job's own worktree (worktree root derived exactly like commitAllCmd: two Dir() hops up from the job dir), and Update handling that surfaces success/conflict/error in the detail status line and refreshes the detail (reload + refreshCommits) like commitAllMsg.
     files: internal/ui/app.go
     depends: TASK-1
     risk: medium — needs the worktree-root derivation and base-branch resolution chain to be right; a dirty worktree or merge conflict must surface as a status error without crashing the app (a dirty tree makes `git merge` refuse; conflicts leave the tree in a conflicted state the user resolves manually).

TASK-3: Add a small git panel modal view (new gitpanel.go): a non-destructive picker listing the three actions (Commit all / Push to origin / Merge default branch), navigated with ↑/↓/k/j, enter to select, esc/q to cancel, rendered like the agents picker — plus a new stateGitPanel wired into App's Update routing, View, resize, and an updateGitPanel handler that dispatches to the existing commitAllCmd/pushCmd and TASK-2's mergeCmd, then returns to the detail view.
     files: internal/ui/gitpanel.go (new), internal/ui/app.go
     depends: TASK-2 (merge dispatch target)
     risk: medium — a new appState touches the core Update/View/resize routing, but it mirrors the existing agentsPicker/confirm overlay patterns closely and adds no new git behavior itself.

TASK-4: Bind the detail view's "g" key to open the git panel (a new "g" case in updateDetail before the detail-view fall-through), update the footer hint to advertise the panel (replace "P push to origin · c commit all" with "g git"), and refresh the key-collision comment in agents.go. NOTE: "g" is currently the detail file viewer's scroll-to-top key (shared with "home") — the panel takes it over; scroll-to-top stays reachable via "home", and the list view's "g" (jump-to-top) is untouched. Decision flagged for the developer/reviewer: keeping P/c as direct accelerators is recommended (backwards-compatible, existing tests keep passing) while the panel becomes the discoverable entry.
     files: internal/ui/app.go, internal/ui/detail.go, internal/ui/agents.go
     depends: TASK-3
     risk: medium — a deliberate key-binding change (repurposing "g" in the detail view) that a user could notice; must be called out in implementation.md and verified against the existing detail-key tests.

TASK-5: Add UI tests for the panel: "g" opens the modal, esc/q cancel back to the detail view, enter on each row dispatches the right cmd, mergeMsg success/conflict/error status handling, and an end-to-end merge test (open panel → select Merge → base branch commits land in the job worktree), modeled on push_test.go/commitall_test.go with the gitInitRepo/addJobWorktree helpers.
     files: internal/ui/gitpanel_test.go (new)
     depends: TASK-3, TASK-4
     risk: low — test-only, following the existing key-flow test patterns; the end-to-end merge test needs a diverged-base fixture (a commit on the base branch after the job worktree was created), same spirit as push_test's bare-origin setup.
-->

## Design decisions & open questions

1. **Key conflict (needs human confirmation if not acceptable):** the brief names `g` for the git panel, but `g` is currently the detail file viewer's scroll-to-top key (`detail.go`'s `case "g", "home"`). The default in TASK-4 is to repurpose `g` for the panel (scroll-to-top remains on `home`). If the user wants scroll-to-top preserved, the panel needs a different key — that is a product decision outside the analyst's remit to override.
2. **Fetch before merge?** "Bring a worktree up to speed" could imply fetching origin first so the merge pulls in remote state. The conservative default in TASK-2 is to merge the locally-resolved base branch (project BaseBranch → origin/HEAD → main) with no fetch — the same local base the diff tab diffs against and `mg done` merges into — and to guard with RefExists so a missing local base surfaces a clear status message instead of git's raw error. A fetch-first variant is a possible follow-up, not part of this job's core scope.
3. **Scope of the modal:** the panel is a detail-view feature (that is where commit-all and push already live, and "bring a worktree up to speed" is per-job). The list view's `g` (jump-to-top) is untouched.
4. **Merge mechanics:** run inside the job's own worktree with `git merge --no-edit <base>`, bounded by hostGitTimeout, GIT_TERMINAL_PROMPT=0, and refuse (status message) when the job has no branch — mirroring the existing "no branch known for this job" guard on P/c.