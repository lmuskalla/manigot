# Implementation: tui: git panel

id: diamond
status: open
developer:
date: 2026-08-17

<!-- Produced by @developer after implementation. -->

## Summary

Added a small git panel modal to the TUI detail view, opened with the `g`
key, listing the three git actions the detail view offers: **Commit all**,
**Push to origin**, and the new **Merge default branch** action that brings a
job's worktree up to speed with the project's base branch. The panel is
navigated with ↑/↓/k/j, enter runs the highlighted action, esc/q cancels back
to the detail view, and the outcome (success, conflict, or error) surfaces in
the detail view's status line. The existing `P` (push) and `c` (commit all)
accelerators stay bound — the panel is the discoverable surface, not a
replacement of the old keys.

The merge action resolves the base branch through the same chain the diff tab
and the done-confirmation use (project `baseBranch` from
`.manigot/manigot.json` → `git.SymbolicRefHead` → "main") and runs
`git merge --no-edit <base>` inside the job's own worktree, bounded by the
usual `hostGitTimeout` and `GIT_TERMINAL_PROMPT=0`. No fetch happens first —
the merge targets the same local base ref the diff tab diffs against and
`mg done` merges into.

## Changes

TASK-1: added `git.MergeBranch`/`MergeBranchWithContext` (new
`internal/git/merge.go`): `git merge --no-edit <branch>` with
`GIT_TERMINAL_PROMPT=0`, `ErrNotARepo`/`wrapErr` degradation identical to
Push/CommitAll. One deliberate divergence: a failed merge's error includes
git's **stdout whenever it is non-empty**, appended to the stderr-based
message when both are non-empty — on some git versions a conflicted merge
reports its "CONFLICT (content): ..." lines on stdout (with stderr left
empty), and gating on stderr alone would leave the status line a bare "exit
status 1". New
`internal/git/merge_test.go` covers the diverged-branch happy path (merge
commit with two parents), a real conflict (error mentions CONFLICT, tree left
conflicted), a dirty-worktree refusal, the already-up-to-date no-op, a missing
branch, and the non-repo classification.

TASK-2: added the "merge default branch" action to the detail flow
(`internal/ui/app.go`): `mergeMsg` (carries the resolved base name + error),
a `mergeMsg` case in `App.Update` that mirrors `commitAllMsg` (success →
"→ merged <base> into <branch>" status plus `detail.reload()` +
`refreshCommits` so the diff tab and git-log strip pick up the new commits;
any error → `cmdErrorText` status), and `mergeCmd`, which resolves the base
via the shared `project.BaseBranch` → `git.SymbolicRefHead` chain (read from
`job.Root`, the project root — same as `diffBaseBranch`/`doneConfirmLines`)
and runs `git.MergeBranchWithContext` in the job's own worktree, derived from
the job dir with the same two-`Dir()`-hops-up `commitAllCmd` uses.

TASK-3: added the git panel modal (new `internal/ui/gitpanel.go`): a
non-destructive three-row picker (`gitPanelActions` is the single source of
truth for rendering and dispatch), ↑/↓/k/j navigation with cursor clamping,
enter submits, esc/q cancels. Wired a new `stateGitPanel` into `App`'s
`Update` routing, `View`, and window-resize handling, plus a `gitPanel` field
and an `updateGitPanel` handler that dispatches the selected action to the
existing `commitAllCmd`/`pushCmd` and TASK-2's `mergeCmd`, then returns to the
detail view.

TASK-4: bound the detail view's `g` key to open the git panel (a new `case
"g"` in `updateDetail`, gated on the job having a branch like `P`/`c`/`t`),
replaced the footer hint's "P push to origin · c commit all" with "g git",
removed `g` from the detail file viewer's scroll-to-top binding in
`detail.go` (scroll-to-top stays reachable via `home` — the list view's `g`
jump-to-top is untouched), and refreshed the key-collision comment in
`agents.go`. The deliberate key-repurposing (the analyst's flagged decision)
was implemented with the recommended default: `P`/`c` remain direct
accelerators, the panel is the discoverable entry.

TASK-5: added `internal/ui/gitpanel_test.go` covering: `g` opens the modal
(and is a no-op with a status message for a branch-less job), panel navigation
and action keys, `selected()` mapping, rendering, esc/q cancel back to the
detail view, enter on each row dispatching the right cmd (commit-all lands a
real commit in the job worktree, push lands a real ref on a bare origin), the
end-to-end merge test (open panel → select Merge → base-branch commits land in
the job worktree and the job branch tip becomes a merge commit, with the
success status asserted), a real merge conflict surfacing as a CONFLICT status
error, and synthetic mergeMsg error handling.

## Known issues / follow-ups

- **Fetch-before-merge is out of scope** (analyst's design decision #2): the
  merge targets the locally-resolved base branch with no fetch, so a stale
  local `main`/`origin/HEAD` will not pull in remote state. A fetch-first
  variant is a possible follow-up.
- **Scroll-to-top key repurposed** (analyst's design decision #1): `g` in the
  detail view now opens the git panel instead of scrolling the file viewer to
  the top; `home` still scrolls to top. The list view's `g` (jump-to-top) is
  unaffected.
- A conflicted merge leaves the job worktree in the conflicted state; the TUI
  reports it in the status line and the user resolves it manually (git's own
  behavior — MergeBranch never aborts or resolves). The conflict markers are
  not shown inside the TUI itself; the user resolves them outside the TUI.
- none otherwise.

## Verdict follow-up (NEEDS WORK → fixes)

Two NEEDS WORK rounds were resolved before APPROVED.

### Round 1 — stale README key table, gofmt, RefExists guard

**Blocker 2 — stale README key table (fixed).** Updated README.md's
Detail-view key table: the scroll row dropped `g` and now reads
`pgup`/`pgdn`, `home`/`G` (scroll-to-top is `home`-only after the TASK-4
repurposing; the verdict suggested `G`/`end`, but `home` is the actual
scroll-to-top key and `G`/`end` are the bottom keys, so `home`/`G` is the
accurate pair), a new `g` row documents the git panel, and a new paragraph
after the key table documents the panel and the **Merge default branch**
action including its no-fetch, locally-resolved-base semantics.

**Non-blocking — gofmt (fixed).** `internal/git/merge.go` and
`internal/ui/gitpanel.go` were both missing their trailing newline; ran
`gofmt -w` on both (formatting only, no behavior change).

**Non-blocking — design-decision #2 RefExists guard (left as-is).** The
verdict's note that a missing local base surfaces git's raw "not something we
can merge" message instead of a tailored one is accurate; the guard was
deliberately not added — it is out of the job's scope per the analyst's
decision #2 and the task text ("A fetch-first variant is a possible follow-up,
not part of this job's core scope").

### Round 2 — merge-conflict error content (fixed)

The first NEEDS WORK verdict's blocker claimed that on the image's git
2.47.3 a conflicted `git merge --no-edit` writes the "CONFLICT (content):
..." detail to stdout but "Automatic merge failed; fix conflicts and then
commit the result." to **stderr**, so `MergeBranchWithContext`'s stdout
fallback (gated on empty stderr) would never fire and the two conflict tests
would fail. Round 1 rebutted that with an empirical claim of the opposite
layout but made no code change; the verdict demanded evidence and the
robustness fix.

Re-verified empirically this round against the image's git 2.47.3
(`/usr/bin/git`, hand-built diverged/conflicting fixture, stdout/stderr
captured separately): **a conflicted merge writes everything — "Auto-merging",
"CONFLICT (content): ...", and "Automatic merge failed; fix conflicts and
then commit the result." — to stdout, and stderr is 0 bytes.** So round 1's
claim was factually right for 2.47.3 and the original code did pass the tests
on this version. The verdict's robustness point is nevertheless valid: gating
the fallback on *empty* stderr means any git version that emits even one
stderr line would silently drop all the CONFLICT detail.

Applied the verdict's required fix to `MergeBranchWithContext`
(`internal/git/merge.go`): the failed-merge error now includes git's **stdout
whenever it is non-empty**, appended to the stderr-based message when both
are non-empty, instead of only falling back to stdout when stderr is empty.
Under any stream layout — stdout-only (2.47.3), stderr-only, or both — the
wrapped error now carries the CONFLICT detail. (Also dropped the now-unused
`fmt` import; `go vet` clean.)

Actual test output, run with the real git on PATH (the manigot session git
shim refuses the fixtures' `git init`, so `PATH=/usr/bin:/bin` is required):

```
$ PATH=/usr/bin:/bin go test ./internal/git/ ./internal/ui/
ok  	github.com/lmuskalla/manigot/internal/git	3.291s
ok  	github.com/lmuskalla/manigot/internal/ui	4.071s
$ PATH=/usr/bin:/bin go test ./...
ok  	github.com/lmuskalla/manigot/cmd/mg	3.017s
ok  	github.com/lmuskalla/manigot/internal/git	(cached)
ok  	github.com/lmuskalla/manigot/internal/job	2.260s
ok  	github.com/lmuskalla/manigot/internal/ui	(cached)
… (all packages ok, no failures)
```

Both conflict tests pass (`TestMergeBranchConflict` and
`TestGitPanelMergeConflictSurfacesInStatus`, verbose runs recorded as PASS
alongside the full `TestMergeBranch*` and `TestGitPanel*` suites). The new
error format on a real conflict, confirmed with the fixed code against a
fresh fixture:

```
git merge main: exit status 1: Auto-merging shared.txt
CONFLICT (content): Merge conflict in shared.txt
Automatic merge failed; fix conflicts and then commit the result.
```