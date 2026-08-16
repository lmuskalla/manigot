# Verdict: commit all

id: technology
status: open
reviewer: @reviewer
date: 2026-08-16

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `internal/git/git.go` — `CommitAll`/`CommitAllWithContext` run `git add -A`
followed by `git commit -m <message>` via the existing `runCtx`, exactly the
`CommitFile` structure. The new exported sentinel `ErrNothingToCommit` is
returned when git commit's empty-index exit-1 "nothing to commit" case is
detected on stdout — verified empirically in this environment that git writes
that message to stdout (not stderr), so the stdout scan is correct and matches
the pre-existing `CommitFile` pattern. `ErrNotARepo` is returned for non-repo /
missing-git (checked on both the add and commit steps), other failures wrap
with stderr via `wrapErr`. Error-order is correct (notARepo checked before the
"nothing to commit" scan).

TASK-2: PASS
notes: `internal/ui/app.go` — `commitAllMsg{err}` type, `commitAllCmd()` bound
by the existing `hostGitTimeout` (30s), running against the job's own worktree
root derived exactly as `commitBriefCmd` does (`filepath.Dir(filepath.Dir(a.detail.job.Dir))`
— a job at `<worktree>/docs/jobs/<id>_<slug>` resolves to the worktree root,
so the commit lands on the job branch, never `a.root`). Message is
`[<id>] chore: commit all` per decision 3. `case "c":` in `updateDetail`
guards the branchless (non-repo fallback) job with "no branch known for this
job" before dispatching, mirroring `P`. `commitAllMsg` handling in `Update`
is correct: success → "→ committed all changes" plus `reload()` (recomputes
the diff tab) and `refreshCommits(...)` (re-reads the git-log strip) — the
exact refresh pair `refresh()` uses for ctrl+r; `ErrNothingToCommit` →
"nothing to commit" status; other errors → `cmdErrorText`. `errors` import
added; no behavior changed for existing switch cases. Key collision check
confirmed: agent keys are a/o/d/r/s, detail keys are tab/1-6/e/D/j/x/del/P/
ctrl+r/esc/backspace/q, detail-view scroll keys are arrows/pgup/pgdn/g/G —
`c` is free.

TASK-3: PASS
notes: `internal/ui/detail.go` renderFooter gains "· c commit all" in the same
hint list as `P push to origin`; `internal/ui/agents.go` key-collision comment
lists `c commit all`; `README.md` Detail-view table gains the `c` row right
after the `P` row. All three are exactly as specified; footer tests assert
substrings ("q quit") rather than the exact hint, so the hint change breaks no
existing assertions. `docs/AGENTS.md` indeed does not enumerate individual TUI
keybindings (only the `j` mg-jdi and diff-tab `6` mentions), so the no-change
decision matches the v6h2us precedent.

TASK-4: PASS
notes: `internal/git/commitall_test.go` (new) — covers all four required
cases using the package's real-git helpers: one call sweeps modified + new +
deleted files into one commit (worktree clean, subject correct); clean repo →
`ErrNothingToCommit` via `errors.Is` and explicitly not `ErrNotARepo`; non-repo
→ `ErrNotARepo`; a prior partial `git add` is swept in (`git add -A`
semantics). Tests mirror the existing package test style (initRepo/writeFile/
runGit/commitAll).

TASK-5: PASS
notes: `internal/ui/commitall_test.go` (new) — mirrors `push_test.go`'s
pattern with real worktrees: pressing `c` on a dirty worktree dispatches a
tea.Cmd whose result is a `commitAllMsg`; feeding it back through `Update` sets
"→ committed all changes", the commit really landed on the job branch
(`git -C <worktree> log -1 --format=%s` = `[cmt01] chore: commit all`), the
worktree is clean afterwards, and the main worktree's history is untouched. The
`ErrNothingToCommit` msg surfaces as "nothing to commit"; a real error surfaces
via `cmdErrorText`; a branchless (non-repo fallback) job reports "no branch
known for this job" without dispatching a cmd. The 80×24 detail view and
`reload()`/`refreshCommits` calls all work against the temp repo fixture.

Scope: PASS — the diff touches only the files named in the tasks (git.go,
app.go, detail.go, agents.go, README.md, the two new test files, and the job's
own docs). No new `mg` subcommand, no bash/agent/shim changes, no refactors
beyond the task. Commit discipline is correct: one commit per task in
`[technology] TASK-N: <desc>` format, implementation.md has its own commit.

## Security

none — a host-side, human-initiated `git add -A` + `git commit` in the job's
own worktree, gated like the existing `P` push. No new exec surface beyond the
already-reviewed `git` package wrapper; no agent/shims touched; `.opencode/`/
`.claude/` mount targets stay excluded via `.git/info/exclude` which `git add
-A` honours. The commit message is a fixed format string (no shell
interpolation) passed as a single `-m` argument.

## Overall

APPROVED

Informational note for the human at `mg done` time (not a blocker, not caused
by this job's implementation): `main` advanced past this branch's cut point
with the `tig in tui` commit (f274d98) after this job's TASK-3 landed, and it
edited the same hunks this job was tasked to edit — the `agents.go`
key-collision comment, the `app.go` `updateDetail` switch (tig added `case
"t":` right where this job added `case "c":`), the `detail.go` footer hint,
and the README keybindings table row. The squash merge onto current `main`
will therefore report conflicts in those files; resolve by keeping both
features (keys `c` and `t` are distinct and coexist). This is the same
situation the repo has handled before ("Resolve merge conflicts on main").
The job itself is complete and correct.

Note: `go build`/`go vet`/`go test ./...` could not be executed from this
session — the container git shim allowlists only git read/commit commands, and
the test suite requires `git init`/`git worktree` (which the shim refuses).
The new code and tests were verified by static review against the existing
patterns and helpers.
