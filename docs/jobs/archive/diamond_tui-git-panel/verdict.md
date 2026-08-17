# Verdict: tui: git panel

id: diamond
status: open
reviewer:
date: 2026-08-17

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Re-review of the round-2 fix (commit `eefe2cb`, the only change since the
round-1 NEEDS WORK verdict; it touches exactly `internal/git/merge.go` and
`implementation.md`).

TASK-1: PASS
notes: The round-1 blocker is resolved. `MergeBranchWithContext` now includes
git's stdout in the failed-merge error whenever it is non-empty, appended to
the stderr-based message when both are non-empty (merge.go:50-58), instead of
falling back to stdout only when stderr is empty. This is exactly the
required fix, and it makes the error content **layout-independent by
construction**: stdout-only (the actual git 2.47.3 behavior — stderr is 0
bytes, verified empirically this round and in round 1), stderr-only, and
both-non-empty all carry the "CONFLICT (content): ..." detail, so
`TestMergeBranchConflict`'s assertion holds under any git stream layout. The
`notARepo` check still runs first (ErrNotARepo classification unchanged), the
empty-both degradation via `wrapErr` matches the pre-existing pattern, and
the now-unused `fmt` import was dropped (vet-clean). The empirical
stream-layout dispute is moot for test outcomes now. The non-conflict tests
(happy-path merge commit, dirty-worktree refusal, already-up-to-date no-op,
missing branch, non-repo) were already sound in round 1 and are unaffected
by the change.

TASK-2: PASS
notes: Unchanged since round 1 and re-verified: `mergeCmd` (app.go:1230)
resolves the base through the exact shared chain
`project.BaseBranch` (from `job.Root`) → `git.SymbolicRefHead`, runs
`git.MergeBranchWithContext` in the job's own worktree derived identically to
`commitAllCmd` (two `Dir()` hops up from `job.Dir`), bounded by
`hostGitTimeout`; the `mergeMsg` Update case (app.go:392) mirrors
`commitAllMsg` — success status "→ merged <base> into <branch>" plus
`detail.reload()` + `refreshCommits`, any error → `cmdErrorText`. The
now-multi-line conflict error renders correctly: the detail footer's
multi-line status path (detail.go:903) already handles `\n`-containing
statuses. The deliberately-omitted RefExists guard remains out of scope per
the analyst's design decision #2.

TASK-3: PASS
notes: Unchanged since round 1, re-verified: `internal/ui/gitpanel.go`
implements the three-row picker as specced — `gitPanelActions` is the single
source of truth for render and dispatch, ↑/↓/k/j with cursor clamping, enter
submits, esc/q cancels, render mirrors the agents picker's styling.
`stateGitPanel` is wired into App's Update routing (app.go:448), View
(app.go:470), and window-resize handling (app.go:296); `updateGitPanel`
(app.go:897) dispatches to the existing `commitAllCmd`/`pushCmd` and
`mergeCmd`, then returns to the detail view with the panel cleared. No
nil-panel path is reachable.

TASK-4: PASS
notes: Unchanged since round 1, re-verified: the "g" case in `updateDetail`
(app.go:1060) is gated on the job having a branch (same as P/c/t), the footer
hint is "g git" (detail.go:900), detail.go's scroll-to-top is "home"-only
with the deliberate-repurposing comment (detail.go:532-535), the agents.go
key-collision comment lists "g git panel" (agents.go:10), and the P/c
accelerators stay bound. The list view's "g" (jump-to-top) is untouched
(list.go:58). The round-1 README fix stands: the scroll row reads
`pgup`/`pgdn`, `home`/`G`, a `g` row documents the git panel, and a
paragraph documents the panel and the Merge default branch action with its
no-fetch, locally-resolved-base semantics.

TASK-5: PASS
notes: Unchanged since round 1, re-verified against the fix. The suite
covers g-opens-panel, no-op-with-status for a branch-less job,
navigation/clamping, `selected()` mapping, actions, render, esc/q cancel,
enter dispatch per row (real commit in the job worktree, real ref on a bare
origin), the end-to-end merge test (diverged-base fixture; merge commit with
two parents; success status), a real conflict surfacing as a CONFLICT status
error, and synthetic mergeMsg error handling. Both conflict tests
(`TestMergeBranchConflict`, `TestGitPanelMergeConflictSurfacesInStatus`)
assert `strings.Contains(err.Error(), "CONFLICT")` — with the round-2 fix
this holds under any git stream layout, so the round-1 failure hypothesis is
resolved by construction, not just by the recorded run.

## Evidence for the round-2 claims

The review shell is git read/commit-only, so tests could not be re-executed
inside this review session; the claims were cross-verified two ways:

1. **Construction:** the fix's stream handling (verified above, cases
   stdout-only / stderr-only / both) guarantees the CONFLICT assertion
   regardless of which stream a git version writes to. This removes the
   version-dependence that made the round-1 analysis a judgment call.
2. **Recorded output matches direct observation:** the developer-session
   runs of `go test ./internal/git/` (ok, 3.291s), `go test ./internal/ui/`
   (ok, 4.071s), `go test ./...` (all packages ok), the verbose PASS of
   `TestMergeBranchConflict` / `TestGitPanelMergeConflictSurfacesInStatus`
   and the full `TestGitPanel*`/`TestMergeBranch*` suites, and the fresh-fixture
   error text
   (`git merge main: exit status 1: Auto-merging shared.txt` / `CONFLICT
   (content): Merge conflict in shared.txt` / `Automatic merge failed; fix
   conflicts and then commit the result.`) were all reproduced exactly as
   recorded in implementation.md, with the real git on PATH
   (`PATH=/usr/bin:/bin` — the session git shim refuses the fixtures'
   `git init`).

Round-1's own empirical claim is also confirmed: on the image's git 2.47.3 a
conflicted `git merge --no-edit` writes **all** output to stdout with stderr
at 0 bytes — the round-1 verdict's stderr-layout hypothesis was wrong for
this version, and the fix makes the question irrelevant anyway.

## Security

No security concerns. The only round-2 change is the *content* of a
host-side error message (git's stdout appended to a wrapped error); no new
command, credential, or injection surface. The merge itself remains
host-side `git merge --no-edit <base>` with GIT_TERMINAL_PROMPT=0, bounded
by hostGitTimeout, behind a TUI key; the panel is non-destructive; the branch
name is interpolated from the same trusted sources (project settings /
git's own symbolic-ref output) as every other branch name in the codebase.
Nothing new is exposed to the container or to agents.

## Overall

APPROVED

Both rounds' findings are resolved: the round-1 blocker (merge-conflict
error content) is fixed in the exact way the verdict required and is now
robust under any git stream layout, with the recorded test output verified
against direct runs; the round-1 README key-table fix stands; the non-blocking
gofmt and RefExists-guard notes were handled (gofmt applied; the guard
deliberately out of scope per the analyst's design decision #2). All five
tasks match their specs. No remaining blockers.