# Implementation: mg consolidation

id: gegb1q
status: open
developer:
date: 2026-08-12

<!-- Produced by @developer after implementation. -->

## Summary

Completed the strangler migration of manigot's host side into its end state:
**one Go binary, `mg`** (`cmd/mg`), which implements every host-side command
in-process — session, profiles, setup, agents, job, done, delete, init, tui,
jdi — with bash reduced to exactly one file, `scripts/entrypoint.sh` (the
container-side agent-environment script, deliberately kept bash).

All five phases from the brief are done: module relocation (Phase 1, TASK-1–3
were completed before this session; the remaining tasks were implemented
here), the four Phase-2 script ports, the Phase-3 session launcher port, the
Phase-4 job-lifecycle port, and the Phase-5 removal of every script and of
the `resolve`/`hostcmd` machinery, with the TUI and jdi folded in-process.
`go vet ./...`, `go test ./...`, and `make check` are green (14 packages),
and a manual acceptance smoke of the git-only paths passed: `mg init` on a
scratch directory, a full `mg job` → work → `mg done` roundtrip (worktree
layout, squash merge, branch delete, archive with `status: done`), `mg
delete` (with the "This cannot be undone." confirmation), `mg profiles`,
`mg setup --check`, the no-`docs/` fallbacks, the `--job` worktree
resolution incl. the hard error on a branch without a worktree, and the
`--print` stdout/stderr contract (0 bytes on stdout, diagnostics on stderr).

## Changes

- TASK-15: extended `internal/git` with the lifecycle operations —
  `WorktreeAdd/Remove/RemoveForce/Prune`, `BranchDelete`, `SquashMerge` +
  `CommitStaged`, `Stage`, `WorkingTreeDirty`, `SymbolicRefHead`, `RefExists`,
  `GitCommonDir`, `GitPath`, `ExcludePath`, `RevParseToplevel`,
  `ConfigUserName/Email` — all exec-backed with the package's ErrNotARepo
  degrade rules, tested against scratch repos (`internal/git/lifecycle_test.go`).
  Also made `notARepo` case-insensitive (`git diff` reports "Not a git
  repository" with a capital N) and unified the session package's private
  `gitCommonDir` onto `git.GitCommonDir`.
- TASK-16: `job.CreateJob` — byte-exact port of `new-job.sh`: arg validation,
  `crypto/rand` id, slug pipeline, base-branch + `jobBranchPrefix` from
  `project.Load`, the ancestor namespace-collision pre-check with the exact
  error + fix hint, the sibling-vs-nested worktree layout decision (device
  check injectable for tests, `.git/info/exclude` append in the nested case),
  the four scaffold files byte-identical to the script's heredocs, and the
  "Scaffold job <id>_<slug>" commit inside the worktree; non-git fallback
  writes straight into `<root>/docs/jobs/`. The git/non-git decision is made
  on the no-branches signal (`git.LocalBranches` empty → plain-directory
  fallback), matching new-job.sh's `git rev-parse --abbrev-ref HEAD` probe,
  which fails on an unborn HEAD — so a freshly `git init`'d repo (zero
  commits) creates the job in the non-git fallback with `branch: (no git)`
  instead of erroring. Tests cover the full create, the nested/mount-point
  case, the non-git case, the fresh-repo/zero-commits case (added after the
  review), base-branch-missing, namespace collision, invalid type, and the
  slug pipeline.
- TASK-17: `job.FinishJob` — port of `finish-job.sh`: exact→prefix branch
  resolution with ambiguity error, worktree hard-error, worktree-branch
  guard, clean-tree check, verdict checks reusing the exact grep semantics
  (`verdictOverallMatch`), base-branch chain (settings → `origin/HEAD` →
  `main`), archive move + `status: done` rewrite + archive commit inside the
  job's worktree, squash merge onto the base branch, worktree removal
  (skipped for the main-worktree case) + prune, branch `-D`, identical info
  lines, with prompts via `cli.Confirm` (declined → `ErrCancelled`). Tests
  cover the full roundtrip, the main-worktree-job case, verdict warnings,
  dirty-tree rejection, and the not-found/ambiguous resolutions.
- TASK-18: `job.DeleteJob` — port of `delete-job.sh`: non-git plain
  directory delete (exact→prefix, excluding archive/), the git path's branch
  + worktree resolution, dirty-worktree warning wording, main-worktree-case
  handling (switch off the branch, skip removal), `worktree remove --force`
  + prune, branch `-D`, identical confirmations incl. "This cannot be
  undone."
- TASK-19: wired `mg job`/`mg done`/`mg delete` subcommands to the Go
  lifecycle with the scripts' exact wording (usage errors, prompts, unknown
  args); the TUI's `updateNewJob` now calls `job.CreateJob` in-process, and
  the done/delete flows run the lifecycle in-process with a new Bubble Tea
  confirmation view (`internal/ui/confirm.go`) showing the same summary and
  warning lines the scripts showed; `internal/hostcmd` deleted.
- TASK-20: deleted `internal/resolve`; added `internal/home` for
  checkout-root resolution (binary at `<root>/bin/mg`, symlinked install,
  `go run`, or `$MANIGOT_HOME`), keyed off `scripts/entrypoint.sh` (the one
  surviving script); `config`, `agentlist`, `session`, `cmd/mg`, `cmd/tui`
  reworked onto it; `ui.cmdErrorText` dropped the `NotFoundError` branch.
- TASK-21: folded `cmd/tui` and `cmd/jdi` into `cmd/mg` as in-process
  `runTUI`/`runJDI` entrypoints (with `tuiVersion`/`jdiVersion` ldflags);
  deleted `cmd/tui`, `cmd/jdi`, `bin/manigot-tui`, `bin/manigot-jdi`, every
  script except `scripts/entrypoint.sh`, `scripts/lib/`, and the stale
  `Makefile.txt`; `Makefile`'s `tui`/`jdi` targets became aliases of `mg`.
- TASK-22: finalized the `Makefile` (one `bin/mg`, one symlink, `tui`/`jdi`/
  `run` collapse), rewrote `README.md` (repo tree, installed-commands table,
  installing-without-symlinks, TUI build/run, keybindings, rebuild section)
  and `docs/AGENTS.md` (the canonical project context — one-binary
  architecture, `cmd/mg` subcommands, `internal/*` packages,
  `scripts/entrypoint.sh` as the only bash) for the new architecture; synced
  `docs/NAMING.md` and `docs/backlog.md` stale script references;
  `project-template/docs/AGENTS.md` needed no change (its manigot claims —
  mounting, profiles, agents — are unchanged).
- TASK-23: added `make check` (go vet, go test, shellcheck on
  `scripts/entrypoint.sh` when installed); ran the acceptance smoke —
  docker-dependent smokes (real session launch, `mg jdi` end-to-end, the
  interactive TUI) could not run because no docker daemon or TTY exists in
  this environment; everything git-only was exercised with the real
  `bin/mg` binary and verified (see Summary).

## Known issues / follow-ups

- **Review roundtrip:** the verdict (NEEDS WORK) flagged one blocker — TASK-16
  regressed the fresh-repo (no commits yet) case (`git init` → `mg init` →
  `mg job` refused with "base branch 'main' does not exist"). Fixed by basing
  the git/non-git decision on the no-branches signal instead of
  `git.CurrentBranch` (which succeeds on an unborn HEAD), plus a
  zero-commits create test; verified by hand against the reviewer's repro.
  The verdict's non-blocking `gofmt -l` findings (5 files) were also cleaned
  up with a `gofmt -w` pass. Its other observations — the `verdictOverallMatch`
  end-anchored regex, the non-git `mg done`/`mg delete` wording, and
  provenance comments referencing the deleted scripts — were accepted as-is
  per the verdict ("no change required for merge").
- **CI vehicle left undecided (TASK-23 ambiguity, flagged per the task):** no
  `.github/` exists and the task said to flag rather than guess, so only a
  `make check` target was added. If GitHub Actions is the intended vehicle, a
  minimal workflow running `make check` is the natural next step.
- **Docker-dependent acceptance smokes are a manual-verification gap:** this
  environment has no docker daemon, so the real container session launch
  (with and without `docs/`, and `--print` runs that actually invoke an
  agent) and a full `mg jdi` end-to-end run could not be executed. The
  `--print` stdout/stderr split was verified up to the docker exec boundary
  (stdout stays empty), and the session/docker argv construction is pinned
  by `internal/session/docker_test.go`, but a human should run the brief's
  docker-dependent smoke list once on a machine with docker.
- **`bin/mg` in this tree is a build artifact** (rebuilt by `make mg`); it is
  gitignored and not part of the commit.
- The docs in `docs/jobs/archive/` and `CONSOLIDATION-BRIEF.md` still
  describe the old bash architecture — they are historical records of past
  jobs and the migration brief itself and were deliberately left untouched.
