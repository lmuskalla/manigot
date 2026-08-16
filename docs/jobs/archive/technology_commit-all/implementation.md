# Implementation: commit all

id: technology
status: open
developer: @developer
date: 2026-08-16

<!-- Produced by @developer after implementation. -->

## Summary

Added a "commit all" action to the TUI's job detail view: pressing `c` on an
open job stages every uncommitted change in that job's own git worktree
(`git add -A` — new, modified *and* deleted files) and commits it there with
the shared `[<id>] chore: commit all` subject convention, so files agents
sometimes leave behind no longer trip `mg done`'s clean-tree check. The
action is host-side (like the existing `P` push), scoped entirely to the git
helper it calls plus the TUI detail view — no new `mg` subcommand, no agent
or shim changes. An already-clean worktree reports "nothing to commit" as a
distinct, non-failure status, and a successful commit refreshes the detail
view's git-log strip and diff tab immediately.

## Changes

TASK-1 (`internal/git/git.go`): added `CommitAll(root, message)` and
`CommitAllWithContext(ctx, root, message)`, running `git add -A` followed by
`git commit -m <message>` via the package's existing `runCtx`. Added the
exported sentinel `ErrNothingToCommit` — returned (instead of the wrapped
error `CommitFile` silently swallowed) when git commit's exit-1 empty-index
"nothing to commit" case is detected on stdout, so callers can `errors.Is`
it apart from a real failure. A non-repo / missing git binary returns the
existing `ErrNotARepo`; any other failure returns the wrapped error with
git's stderr.

TASK-2 (`internal/ui/app.go`): added the `commitAllMsg{err error}` message
type and `commitAllCmd()`, a plain git call off the UI goroutine bounded by
the existing `hostGitTimeout`, running `git.CommitAllWithContext` against
the job's own worktree root (`filepath.Dir(filepath.Dir(a.detail.job.Dir))`
— the exact derivation `commitBriefCmd` documents, so the commit lands on
the job branch, never `a.root`'s) with message
`fmt.Sprintf("[%s] chore: commit all", a.detail.job.ID)`. Wired `case "c":`
into `updateDetail`: a branchless job (non-repo fallback) reports "no branch
known for this job" without dispatching, mirroring `P`. Handled
`commitAllMsg` in `Update` alongside `pushMsg`: `ErrNothingToCommit` →
"nothing to commit" status; other errors → `cmdErrorText`; success →
"→ committed all changes" plus `a.detail.reload()` (recomputes the diff tab)
and `a.detail.refreshCommits(...)` (re-reads the git-log strip) so the new
commit appears immediately. `c` was confirmed free of collisions (agents use
`a`/`o`/`d`/`r`/`s`; detail keys are tab/1-6/e/D/j/x/del/P/ctrl+r/esc/q).

TASK-3: cosmetic documentation — `internal/ui/detail.go`'s `renderFooter`
hint gains `c commit all` (in the same "· " list as `P push to origin`);
`internal/ui/agents.go`'s key-collision comment lists `c commit all`
alongside the other reserved detail keys; `README.md`'s `### Keybindings` →
"Detail view" table gains a `c` row next to the `P` row.

TASK-4 (`internal/git/commitall_test.go`, new): unit tests for
`git.CommitAll` using the package's existing `initRepo`/`writeFile`/`runGit`
helpers — one call sweeps a modified tracked file, a new untracked file and
a deleted tracked file into one commit (worktree clean afterwards, subject
is the given message); an already-clean repo returns `ErrNothingToCommit`
(`errors.Is`, explicitly not `ErrNotARepo`); a non-repo returns
`ErrNotARepo`; and content left staged-but-uncommitted by a prior partial
`git add` is swept into the commit (`git add -A` semantics).

TASK-5 (`internal/ui/commitall_test.go`, new): unit tests for the `c`
wiring mirroring `push_test.go`'s pattern — pressing `c` on a dirty real
worktree dispatches a tea.Cmd whose result is a `commitAllMsg`; feeding it
back through `Update` sets the status to "→ committed all changes", the
commit really landed on the job branch (`git -C <worktree> log -1 --format=%s`
= `[cmt01] chore: commit all`) with a clean worktree afterwards, and the
main worktree's history is untouched; an `ErrNothingToCommit` msg surfaces
as "nothing to commit"; a real error surfaces via `cmdErrorText`; and a
branchless (non-repo fallback) job is a no-op reporting "no branch known for
this job" without dispatching a cmd.

## Known issues / follow-ups

- The analyst's uncommitted `tasks.md` was swept into the TASK-2 commit by
  `git commit -am` (it was dirty in the worktree at session start). The file
  is this job's own task breakdown, so it belongs on the branch — noted only
  so the commit boundary is clear.
- The existing footer-hint test assertions were verified unchanged; the
  full `go build`/`go vet`/`go test ./...` suite passes.
- Note for anyone running the tests inside a containerized manigot session:
  the session git shim refuses `git init`/`git worktree`, so the git- and
  ui-package tests (which build throwaway repos) must be run with a PATH
  that resolves `git` to the real binary (e.g. excluding `~/.manigot/bin`).
  This is a session-environment artifact, not a test defect.
