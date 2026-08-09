# Verdict: keep track of jobs

id: fvrl56
status: open
reviewer: @reviewer
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Methodology: read brief.md/tasks.md/implementation.md, reviewed `git diff
main...HEAD` file by file (git.go, discover.go, job.go, stage.go, detail.go,
app.go, README.md, and every *_test.go), ran `go build ./...`, `go vet ./...`,
`go test ./...` (all pass) and `gofmt -l` under `tui/`, and spot-checked test
coverage against each task's edge cases.

TASK-1: PASS
notes: `tui/internal/git/git.go` — `LocalBranches`, `CurrentBranch`,
`ListJobDirs`, `ShowFile` all implemented as specified, all `git -C <root>`,
all degrade gracefully (`ErrNotARepo` / `os.ErrNotExist`-wrapped) rather than
panicking. `Checkout` was added ahead of schedule (needed by TASK-6) — clearly
disclosed in implementation.md, same file, not a scope concern.
`git_test.go` covers not-a-repo, no-commits, detached HEAD, missing
path/branch, and checkout success/failure with real throwaway repos.

TASK-2: PASS
notes: `job.Discover` enumerates every local branch via TASK-1, builds a `Job`
per (branch, job) with `Branch`/`OnCurrentBranch` set, reads the
current-branch job's brief from disk and every other job's brief via `git
show` (`ReadJobFromBytes` in job.go). Not-a-repo / no-branches-yet correctly
falls back to `discoverWorkingTree`, byte-identical to the pre-feature path.
`Discover(root)`'s signature is unchanged, so `main.go`/`app.go` call sites
needed no edits (verified: no diff to `launch.go` or `main.go`). The brief's
exact symptom (3 jobs on 3 branches, `git checkout main` still lists all 3) is
covered by `TestDiscoverListsJobsAcrossBranches`.

TASK-3: PASS
notes: `dedupByID` collapses duplicate job IDs, preferring the copy whose
`Branch` matches the brief's own `branch:` field (`briefBranch`), falling back
to first-seen when no copy matches. Covered by both a real multi-branch-merge
git test and a direct table test over the tie-break logic
(`TestDedupByIDTable`).

TASK-4: PASS
notes: `Stage()`/`fileWritten` and `detailView.loadTab`/`readFile` branch on
`OnCurrentBranch`, reading off-branch files via `git.ShowFile` and reusing the
exact same "written" classification (`isWritten`) so behaviour matches the
working-tree path bit-for-bit. Current-branch path is untouched (still
`os.ReadFile`/`FileIsWritten`). Tested for both branches in
`stage_test.go`/`detail_test.go`.

TASK-5: PASS
notes: Detail view meta line shows `· branch: <name>` or a `(other branch —
press c to switch)` variant; list rows for off-branch jobs get a dimmed
trailing `· <branch>` tag, no new column. Matches the task exactly.
Minor (non-blocking) observation: the trailing tag is appended after the
title column is already sized to fill the terminal width, so on a narrow
terminal a branch-tagged row can exceed `w` and wrap. Task explicitly scoped
this as "presentational/formatting only," so not treated as a blocker.

TASK-6: PASS
notes: `c` dispatches `git.Checkout` as a `tea.Cmd` (`checkoutCmd`), handled
via `checkoutMsg` in `Update` — re-discovers, rebuilds the detail view against
the same job id (rebuild-not-patch matches the task's own wording), keeps the
list cursor in sync (`indexOfJob`), and surfaces checkout failure in the
status line without mutating job list/open job. Current branch is re-fetched
fresh at action time per the design decision. Well covered in
`checkout_test.go` including the failure path and the no-known-branch no-op.

TASK-7: PASS
notes: `branchGuard` re-checks `git.CurrentBranch` fresh (not the
discovery-time snapshot) and gates `e`, `D`, and the agent-key dispatch;
jobs with no known `Branch` (non-git fallback) are never guarded, preserving
byte-identical behaviour there. `branchguard_test.go` covers all three guarded
actions plus the current-branch-unblocked case.

TASK-8: PASS
notes: `go build ./...`, `go vet ./...`, and `go test ./...` all pass clean
under `tui/`, confirmed independently. `gofmt -l tui/` now reports nothing
(re-verified). README doc-sync (cross-branch discovery blurb + `c` keybinding
row) is accurate and matches the shipped behaviour.

Follow-up commit `19f6fcf` ("TASK-8: gofmt discover_test.go, correct
implementation.md claim") addresses the one finding from the first review
pass: it gofmt's the misaligned comment in `discover_test.go` and rewrites
the implementation.md note to correctly attribute the formatting issue to
this job's own changes rather than an inherited one. Re-verified both the
diff and a full build/vet/test/gofmt pass — clean.

## Security

None run — no attack surface introduced (local git shell-outs against
developer-controlled branch names/paths already present in the environment,
no network/user-input parsing added beyond existing brief.md frontmatter
parsing, which is unchanged in shape).

## Overall

APPROVED

All 8 tasks pass. Cross-branch discovery, dedup, off-branch reads for
detail/stage, branch surfacing in the UI, the `c` checkout action, and the
launch/edit/done guards all match tasks.md, are well tested (including the
brief's exact reproduction scenario), build/vet/test/gofmt clean, and stay
within the brief's locked scope (no index/sidecar file, no remote branches,
no branch-per-job model change, no `new-job.sh`/`finish-job.sh` rewrite).
Commit discipline is correct: one commit per task in `[fvrl56] TASK-N: ...`
format, a separate `implementation` commit, and the one review finding
(gofmt + inaccurate implementation.md claim) was addressed in its own
follow-up commit.
