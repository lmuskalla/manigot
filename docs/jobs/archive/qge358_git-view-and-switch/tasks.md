# Tasks: git view and switch

id: qge358
status: open
analyst: claude
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Add a `git.RecentCommits(root string, n int) ([]Commit, error)` helper
(new `Commit{Hash, ShortHash, Subject, RelTime, Branch string}` type) to
`tui/internal/git/git.go` that returns the last `n` commits across all local
branches, deduped by commit hash, most-recent first.
  - Design for the "dedup across branch tips" open question: call
    `git log <all local branch names> --source -n <n> --format=%H%x1f%h%x1f%s%x1f%cr%x1f%S`
    — passing every local branch as a positional ref to a single `git log`
    union-traverses their histories and visits each commit exactly once no
    matter how many of the given branches can reach it, so dedup falls out of
    git's own traversal instead of needing a manual hash-set merge.
    `--source` (surfaced per-line via `%S`) reports which of the given refs
    each commit was reached through, giving the "which branch it's on" label
    for free, including for shared/undiverged commits. Order the branch
    arguments with a clear, documented tie-break (e.g. current branch first,
    then the rest sorted by `LocalBranches`' order) so which branch gets
    credited for a shared commit is deterministic.
  - Reuse `LocalBranches` to enumerate branches and `run`/`notARepo` to shell
    out, following the file's existing degrade-gracefully pattern:
    `ErrNotARepo` for a missing git binary / non-repo, empty slice + nil error
    for a repo with zero commits or zero local branches (unborn HEAD).
  - files: `tui/internal/git/git.go`
  - depends: none
  - risk: medium — the multi-ref `git log --source` dedup/attribution
    strategy is new to this codebase and needs to be verified against real
    edge cases (a job branch that hasn't diverged from `main`, an empty repo,
    a single-branch repo, fewer than `n` commits total) rather than assumed
    correct from documentation.

TASK-2: Unit-test `git.RecentCommits` in the `git` package, covering: commits
deduped when two branches share a tip (the brief's explicit open question),
correct most-recent-first ordering, `n` truncation when history is longer
than `n`, fewer-than-`n` results when history is shorter, an empty repo
(no commits yet), and a non-repo directory (`ErrNotARepo`).
  - files: `tui/internal/git/git_test.go` (or a new `recentcommits_test.go`
    in the same package, following that package's existing `initRepo`/
    `runGit`/`commitAll` test-helper pattern)
  - depends: TASK-1
  - risk: low — test-only, but the shared-tip dedup case is the crux of the
    whole feature and needs to actually assert single-appearance-by-hash, not
    just "no crash".

TASK-3: Show the current branch in the list header (additive only, no new
interaction). Add a `currentBranch string` field to `App`, populate it in
`NewApp` via `git.CurrentBranch`, and refresh it at every point the job list
itself is refreshed (`refreshJobs`, the `checkoutMsg` success path, and
`updateNewJob`'s post-`sc-job` refresh) so it can't go stale relative to the
`· <branch>` tags `renderJobRow` already prints. Render it in `renderList`'s
header line, next to the existing "jobs in `<root>`" text. A `""` result
(detached HEAD, or `job.Discover`'s non-repo fallback) should render nothing
rather than an empty/awkward label.
  - files: `tui/internal/ui/app.go`
  - depends: none (uses the existing `git.CurrentBranch`)
  - risk: low — purely additive display logic, small blast radius.

TASK-4: Add a single "back to main" quick-checkout action to the list view,
bound to one new key not already used at list scope (list currently uses
`q/esc`, `up/k`, `down/j`, `home/g`, `end/G`, `ctrl+r`, `enter/l/right`, `n`,
`s`, `o` — suggest `m`). On press, dispatch the existing `App.checkoutCmd`
(currently only reachable from the detail view's `c` key) with the hardcoded
branch name `"main"`, per the brief's explicit rejection of any
default-branch-detection logic. The existing `checkoutMsg` handler in
`Update` already has an `a.detail == nil` branch that reports success/failure
to `a.status` — confirm/adjust that path covers the list-only case (e.g. a
"already on main"/no-op checkout should still surface a clear status, not
just silence). Update the footer hint string to mention the new key.
  - files: `tui/internal/ui/app.go` (`updateList`, `footer`)
  - depends: none (reuses `checkoutCmd`/`checkoutMsg` as-is)
  - risk: low-medium — mechanism is proven from the detail view, but the
    list-view entry point (checkout with no open detail view, or while
    already on `main`) is a code path that currently has no test coverage
    and needs one.

TASK-5: Render the read-only "recent activity" strip in the list header:
call `git.RecentCommits(a.root, 5)` at the same refresh points as TASK-3
(store as `a.recentCommits []git.Commit`), and render each entry as one
compact dimmed line (short hash + truncated subject + relative time +
branch) beneath the branch line added in TASK-3. Must degrade to rendering
nothing when the repo has no commits or `RecentCommits` errors (e.g.
non-repo), matching the rest of the header's optional-content handling.
Truncate the subject the same way `renderJobRow`/`truncate` already truncate
titles, so a long commit message can't wrap the header. This is explicitly
a fixed, non-interactive strip — no scrolling, no new keybinding to open it,
and it must not push the job rows further down than the existing header
already does (watch-out called out directly in the brief).
  - files: `tui/internal/ui/app.go` (`renderList`, likely a new
    `renderRecentActivity` helper), possibly `tui/internal/ui/styles.go` if
    `dimStyle` isn't sufficient for a visually distinct-but-quiet strip
  - depends: TASK-1, TASK-3 (shares the branch-line placement and refresh
    plumbing)
  - risk: medium — the only real layout risk in this job; needs manual
    verification at a few terminal widths that the strip stays glanceable and
    doesn't crowd out the job list, per the brief's watch-out.

TASK-6: Test coverage for TASK-3/4/5's UI-layer changes, mirroring the
existing patterns in `checkout_test.go` / `detail_test.go` (`gitInitRepo`,
`gitRun`, `gitCommitJob` helpers already defined in package `ui`):
  - `renderList` includes the current branch when on one, and omits it
    cleanly on detached HEAD / non-repo.
  - the new list-view key checks out `main` via a real `git.Checkout` call
    (same style as `TestCheckoutKeySwitchesBranchAndRebuildsDetail`) and the
    resulting `checkoutMsg` (with `a.detail == nil`) updates `a.status` and
    `a.currentBranch`/`a.jobs` correctly, including the already-on-`main`
    no-op case and a refused-checkout (uncommitted changes) case.
  - the recent-activity strip renders deduped, most-recent-first entries and
    disappears gracefully on a fresh empty repo.
  - files: `tui/internal/ui/app.go`-adjacent new/extended test files, e.g.
    `tui/internal/ui/checkout_test.go` (extend) and a new
    `tui/internal/ui/list_test.go`
  - depends: TASK-3, TASK-4, TASK-5
  - risk: low — test-only, but non-trivial fixture setup (multiple branches
    with a shared tip) to actually exercise the dedup path end-to-end through
    the UI layer, not just the `git` package unit tests from TASK-2.

## Notes for the developer

- Do not introduce a generic branch picker or any new view/state — every
  task above is scoped to additions inside the existing list view
  (`renderList`) and its header, per the brief's locked product decision.
- `main` is a hardcoded literal (matching `scripts/new-job.sh`'s convention),
  not a detected default branch — no new "find the default branch" logic
  anywhere in this job.
- If TASK-5's 5-entry strip proves noisy once implemented (e.g. generic agent
  commit messages), the brief's specified fallback is shrinking the count
  (down to a single most-recent entry), not adding filtering or interaction.
