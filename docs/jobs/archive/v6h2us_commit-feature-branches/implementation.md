# Implementation: Commit feature branches

id: v6h2us
status: open
developer: claude
date: 2026-08-10

<!-- Produced by @developer after implementation. -->

## Summary

Added a manual **`P` push** action to the TUI's job detail view, mirroring
the existing `b` (switch branch) shortcut. Pressing `P` runs
`git push -u origin <branch>` for the open job's branch, host-side, giving
the user a quick way to make a feature branch visible on another host via
`git push`/`git pull`. As designed in `tasks.md`, `P` is not gated by the
`branchGuard` check that other mutating actions (`e`/`D`/`j`/`x`) use, since
`git push origin <branch>` operates on the named branch ref directly and
does not require that branch to be checked out.

## Changes

TASK-1: Added `git.Push(root, branch string) error` to
`tui/internal/git/git.go`, following the file's existing conventions
(`ErrNotARepo` classification, `wrapErr`-wrapped errors, doc comment style
matching `Checkout`). It runs `git -C root push -u origin <branch>` with
`GIT_TERMINAL_PROMPT=0` set on the child process so a missing/invalid
credential surfaces as a wrapped error instead of hanging on an interactive
prompt. Since the existing unexported `run()` has no env parameter and is
used by many other functions, added a sibling `runEnv()` helper that accepts
extra env without touching `run()`'s signature or behavior.

TASK-2: Added `pushMsg{branch string; err error}` and `pushCmd(branch
string) tea.Cmd` to `tui/internal/ui/app.go`, mirroring
`checkoutMsg`/`checkoutCmd`. Added `case "P":` to `updateDetail`'s key
switch, explicitly not gated by `a.branchGuard()`. Added a `case pushMsg:`
to the root `Update` message switch that sets the detail view's status to
`"→ pushed <branch> to origin"` on success or `cmdErrorText(err)` on
failure — no job-list/detail rebuild needed, since a push doesn't change
anything about the local working tree or job discovery, unlike a checkout.
Confirmed `"P"` (capital) doesn't collide with any existing binding.

TASK-3: Added `P push to origin` to the detail view's footer hint string in
`renderFooter` (`tui/internal/ui/detail.go`), and extended the
key-collision comment at the top of `tui/internal/ui/agents.go` to list `P`
push alongside the other reserved keys.

TASK-4: Added `tui/internal/git/push_test.go` with unit tests for
`git.Push` against a real local bare-repo remote (not mocked): a successful
push that lands the ref on the remote, a check that `-u` really configures
upstream tracking, the missing-`origin`-remote case (wrapped error, not
misclassified as `ErrNotARepo`), and a rejected non-fast-forward push
(simulated by pushing a divergent commit from a second clone of the bare
remote, then attempting to push a diverging local commit) — asserting the
remote is left untouched rather than force-overwritten.

TASK-5: Added `tui/internal/ui/push_test.go` mirroring
`checkout_test.go`'s pattern: pressing `P` dispatches `pushCmd` and the
resulting `pushMsg` updates `a.detail.status` on success; an error
surfaces via `cmdErrorText`; a job with no known branch is a no-op with a
status message (mirrors `TestCheckoutKeyWithoutBranchIsNoop`); and the key
regression test `TestPushKeyIgnoresBranchGuard` confirms `P` works even
when the currently checked-out branch differs from the job's branch (first
asserting `branchGuard()` really would block a guarded action there, then
confirming `P` still dispatches and succeeds).

TASK-6: Added a `P` row to the README's `### Keybindings` → "Detail view"
table, next to the `b` row it complements.

Also included in this pass: `docs/jobs/v6h2us_commit-feature-branches/tasks.md`
was already written with @analyst's output ("Best option" section + task
breakdown) but left uncommitted in the working tree when this job started —
folded into the commit history here rather than left dangling, since it's
the source this implementation was built from.

## Known issues / follow-ups

None. Out-of-scope items explicitly deferred per `tasks.md`'s notes: force
push, deleting/pruning remote branches, auto-push on commit or job
creation, pushing from inside an agent's container session, an
ahead/behind-origin indicator, and any handling for a project with no
`origin` remote beyond a clear error.
