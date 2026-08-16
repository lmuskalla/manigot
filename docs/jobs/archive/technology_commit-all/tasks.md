# Tasks: commit all

id: technology
status: open
analyst: @analyst
date: 2026-08-16

<!-- Produced by @analyst from brief.md. -->

## Decisions this breakdown locks in

**1. Scope — the TUI detail view only, no new CLI subcommand, no agent-file
changes.** The brief asks for "an option to 'commit all'" inside the detail
job view, with the `c` key free. The detail view already owns every other
per-job git action (`P` push, `e` edit+auto-commit, `D` done, `x` delete,
`j` mg-jdi) — adding `c` there keeps all job/branch actions in one place,
the same reasoning v6h2us (push) and 7v3v7j (edit+auto-commit) used. A new
`mg commit` CLI verb would just wrap `git add -A` + `git commit`, which works
fine typed directly; the gap the brief describes is a *quick* action while
already sitting in the TUI. No changes to `agents/*.md`,
`scripts/entrypoint.sh` or the git shim: this is a host-side, human-initiated
action (like `P` push), not something agents need.

**2. What "commit all" does — stage everything in the job's own worktree
and commit it there.** The commit must run inside the job's worktree root,
not the project root: a job's files live in its own worktree (one worktree
per job), so a pathspec relative to the project root would escape the main
worktree ("outside repository"), and the commit must land on the *job
branch*, not the base branch — exactly the same `git -C <worktree>` reasoning
`commitBriefCmd` already documents (`internal/ui/app.go`, derived from
`a.detail.job.Dir` via two `Dir()` hops). "All" = `git add -A` (new,
modified *and deleted* files — the deleted-stale-copy scenario in
docs/BUG_report-mg-done-dirty-worktree-stale-job-copy.md is precisely a
deletion that trips `mg done`'s clean-tree check). The `.opencode/` and
`.claude/` mount-target paths are already excluded via `.git/info/exclude`
(`git.ExcludeMountTargets`), and `git add -A` honours exclude rules, so the
mounted docs can never be re-staged by this action.

**3. Commit message follows the shared `[ID] <type>: <summary>` convention:
`[<id>] chore: commit all`.** Every agent commit and the existing brief
auto-commit use this shape (`[ab0001] brief: edit via TUI`, `[ID] verdict:
...`, `[ID] TASK-N: ...`). A catch-all sweep commit is a chore-type action,
so `chore` is the type segment regardless of the job's own `feature`/`fix`
type — it labels *what happened* (a generic cleanup commit), matching how the
brief auto-commit uses a fixed `brief:` label rather than the job type.

**4. "Nothing to commit" is a distinct, non-error outcome.** Pressing `c` on
an already-clean worktree should tell the user the tree is clean, not pretend
it committed. `git commit` on an empty index exits 1 with "nothing to
commit"; `CommitFile` currently swallows that as a silent no-op. For a
*manual* action the feedback is the point, so the new git helper reports it
via a package-level sentinel error `ErrNothingToCommit` (mirroring the
existing `ErrNotARepo` sentinel pattern), and the UI shows "nothing to
commit" instead of an error or a false success.

**5. After a successful commit-all, refresh the open detail view.** Unlike
`pushMsg` (a push changes nothing locally visible), a commit *does* change
what the detail view shows: the bottom git-log strip (`refreshCommits`) and
the computed diff tab (recomputed on `reload`). The handler refreshes both so
the new commit appears immediately instead of on the next `ctrl+r`.

## Task breakdown

TASK-1: Add a `git.CommitAll(root, message string) error` helper (plus the
`CommitAllWithContext(ctx, root, message)` variant the TUI's background cmd
will use, mirroring `CommitFileWithContext`) to `internal/git/git.go`: `git
add -A` followed by `git commit -m <message>`, run against `root` via the
package's existing `run`/`runCtx`. "Nothing to commit" (the `git commit`
exit-1/empty-index case `CommitFile` already detects by scanning stdout for
"nothing to commit") returns a new exported sentinel `var ErrNothingToCommit
= errors.New("nothing to commit")` — a *distinct, non-failure* outcome, so
callers can `errors.Is` it apart from a real error. A non-repo / missing git
binary returns the package's existing `ErrNotARepo`; any other failure
returns the wrapped error including git's stderr (existing `wrapErr`).
  - files: `internal/git/git.go`
  - depends: none
  - risk: low — follows the exact structure of the existing
    `CommitFile`/`CommitFileWithContext` pair; the only new surface is `git
    add -A` (all changes incl. deletions) and the new sentinel.

TASK-2: Wire the `c` key into the TUI detail view. Add a `commitAllMsg{err
error}` type and a `commitAllCmd() tea.Cmd` to `internal/ui/app.go`,
mirroring `pushCmd`/`commitBriefCmd`: a plain git call off the UI goroutine,
bounded by the existing `hostGitTimeout` context, running
`git.CommitAllWithContext` against the job's own worktree root
(`filepath.Dir(filepath.Dir(a.detail.job.Dir))` — the same derivation
`commitBriefCmd` documents) with message `fmt.Sprintf("[%s] chore: commit
all", a.detail.job.ID)`. Add a `case "c":` to `updateDetail` that guards a
branchless job (`a.detail.job.Branch == ""` — the non-repo working-tree
fallback, same guard `P` push uses) with the status "no branch known for
this job" and otherwise dispatches `commitAllCmd`. Handle `commitAllMsg` in
`Update` (alongside `pushMsg`): on `errors.Is(err, git.ErrNothingToCommit)`
set the detail status to "nothing to commit"; on any other error use
`cmdErrorText(err)`; on success set "→ committed all changes" and refresh
the open detail view so the new commit shows — `a.detail.reload()` (re-runs
`loadTabs`, which recomputes the diff tab) plus
`a.detail.refreshCommits(a.settings.RecentActivityCountValue())` (re-reads
the git-log strip). Confirm `"c"` collides with no existing binding —
agents use `a`/`o`/`d`/`r`/`s`, the detail keys are `tab`/`1`-`6`/`e`/`D`/
`j`/`x`/`del`/`P`/`ctrl+r`/`esc`/`backspace`/`q` — `c` is free.
  - files: `internal/ui/app.go`
  - depends: TASK-1
  - risk: medium — touches the shared `updateDetail` key-dispatch switch and
    the root `Update` message switch, both hit by every other detail-view
    key; needs care not to change behavior for any existing case (the same
    caution the push/auto-commit jobs applied).

TASK-3: Cosmetic follow-ups once TASK-2 lands: add `c commit all` to the
detail view's footer hint string (`renderFooter` in
`internal/ui/detail.go`, in the same "· " list as `e edit` and
`P push to origin`); extend the key-collision comment at the top of
`internal/ui/agents.go` ("chosen so they never collide with the detail
view's other bindings…") to list `c commit all` alongside the other reserved
keys; and add a `c` row to the README's `### Keybindings` → "Detail view"
table describing the action (next to the `P` row it complements).
  - files: `internal/ui/detail.go`, `internal/ui/agents.go`, `README.md`
  - depends: TASK-2
  - risk: low — comment/string/docs-only changes, no behavior.

TASK-4: Unit-test `git.CommitAll` in the git package (new
`internal/git/commitall_test.go`, using the package's existing
`initRepo`/`writeFile`/`runGit` helpers): a scratch repo with a modified
tracked file, a new untracked file, and a deleted tracked file — one
`CommitAll` call commits all three (assert `git status --porcelain` is empty
after and the commit subject is the given message); an already-clean repo
returns `ErrNothingToCommit` (assert via `errors.Is`, and assert it is *not*
`ErrNotARepo`); a non-repo returns `ErrNotARepo`; and a modified file left
staged-but-uncommitted by a prior partial `git add` is swept into the commit
(`git add -A` semantics).
  - files: `internal/git/commitall_test.go` (new)
  - depends: TASK-1
  - risk: low — test-only, real git against throwaway repos like the rest of
    the package's tests.

TASK-5: Unit-test the `c` wiring in package `ui` (new
`internal/ui/commitall_test.go`, mirroring `push_test.go`'s pattern — the
existing `gitInitRepo`/`addJobWorktree` helpers, a real `detailView` against
a real temp worktree): pressing `c` on a dirty worktree dispatches a tea.Cmd
whose result is a `commitAllMsg`; feeding it back through `Update` sets the
detail status to "→ committed all changes" and the commit really landed on
the job branch (`git -C <worktree> log -1 --format=%s` = `[<id>] chore:
commit all`) with a clean worktree afterwards; an `ErrNothingToCommit` msg
surfaces as the "nothing to commit" status; a real error surfaces via
`cmdErrorText`; and a branchless job (non-repo fallback) is a no-op that
reports "no branch known for this job" without dispatching a cmd.
  - files: `internal/ui/commitall_test.go` (new)
  - depends: TASK-2
  - risk: low — test-only, same local-worktree fixture approach as
    `push_test.go`/`editordone_test.go`.

## Notes for the developer

- No new bash script, no new `mg` subcommand, no changes to `scripts/*.sh`
  or `agents/*.md` — scoped entirely to the host-side TUI plus the git
  helper it calls (see "Decisions" 1).
- The commit must run against the job's *worktree root*, never `a.root` —
  `git -C <worktree>` is what makes the commit land on the job branch and
  keeps the pathspec in-repo (see `commitBriefCmd`'s doc comment in
  `internal/ui/app.go` for the full reasoning; reuse its exact derivation).
- The `.opencode/`/`.claude/` mount-target exclusion (`git.ExcludeMountTargets`)
  already protects `git add -A` from re-staging the mounted docs — do not
  add extra filtering; the tests in TASK-4/5 can assert a stale tracked
  `.opencode/`-style deletion is swept in if desired, but that is already
  covered by `git add -A` semantics.
- Out of scope (flag back if any turn out to be needed): committing from
  inside an agent's container session (the git shim already allows
  add/commit there), force-push or any remote interaction, an
  "uncommitted changes" indicator in the list or detail view, and handling
  for a project with no git at all beyond the branchless-job no-op status.
- `docs/AGENTS.md` does not enumerate individual TUI keybindings outside the
  README's `### Keybindings` table TASK-3 updates — no change needed there,
  mirroring the v6h2us (push) precedent.
