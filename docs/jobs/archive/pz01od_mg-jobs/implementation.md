# Implementation: mg jobs

id: pz01od
status: open
developer: @developer (opencode-go)
date: 2026-08-13

## Summary

Added a new `mg jobs` CLI subcommand to manigot: it lists every open job with
its state — mirroring the TUI's list row (ID, status, type, date, title, plus
a plain-text mg-jdi activity badge when a `.manigot/jdi-status/` sidecar
exists) — and lets the user pick one interactively on a TTY, then re-execs
`mg --job <id> <passthrough>` in the foreground so the session launcher mounts
the chosen job's worktree and prompts with its brief.md. Done jobs
(`docs/jobs/archive/`) are excluded by `job.Discover`, same as the TUI. On a
non-TTY stdin it prints the list and refuses to pick (exit 1); with no
project root it errors like `mg job`; with no jobs it prints an invite (exit 0).

## Changes

TASK-1: dispatch `mg jobs` subcommand — `cmd/mg/main.go`
  - Added `case "jobs":` to the dispatcher switch, calling
    `runJobs(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin))`
    (same shape as `runAgents`).
  - Added `mg jobs  List jobs and pick one to start a session in` to the
    `mg -h` Commands block, directly under `mg job`.
  - Updated the package doc comment's command enumeration.
  - The switch case references `runJobs`, which TASK-2 implements; to keep the
    tree buildable at every commit (the verification gate), TASK-1's commit
    also introduces `cmd/mg/jobs.go` with a minimal compiling stub of
    `runJobs`, which TASK-2's commit replaces wholesale.

TASK-2: implement the command — `cmd/mg/jobs.go`
  - `runJobs(passthrough, r, stdout, stderr, tty)`: resolves the project root
    via `job.FindProjectRoot` (errors and the no-`docs/` message mirror
    `cmd/mg/job.go`), lists via `job.Discover`, prints "Jobs:" + numbered rows
    in date-desc order (TUI column widths 8/6/8/12, plain spaces, no styling),
    refuses to pick on non-TTY, prompts with `cli.Select("Select a job
    [1-N]: ")` and re-execs `mg --job <id> <passthrough>` via the existing
    `reexec` helper (the `mg agents` pattern), printing
    `→ Starting a session in <id>...`.
  - `jobsBadge`: plain-text port of the TUI's `jdiStatusBadge` —
    `[running @<agent>]` / `[finished]` / `[needs human]` — gated by
    `job.ReadJDIStatus(root, j.Name)`, empty when nothing live to report.
  - Empty list prints `No jobs yet — run 'mg job "<title>"' to create one.`
    and exits 0.

TASK-3: tests — `cmd/mg/jobs_test.go` (new)
  - Hermetic fixtures on non-git temp dirs with `docs/jobs/<name>/brief.md`,
    exercising `job.Discover`'s working-tree fallback (the git-worktree-backed
    path is covered by the job package's own tests). Status sidecars are
    written with `job.WriteJDIStatus` so the on-disk format is the real one.
  - Coverage: list rendering (columns, date-desc order, `[running @developer]`
    badge), non-TTY refusal wording + exit 1, TTY selection → launch line +
    re-exec rejection (the go-test-binary pattern from
    `TestAgentsSelectWritesChosenAndLaunches`), empty-list message + exit 0,
    missing-project-root error, and a `jobsBadge` unit test for all three
    states plus the no-sidecar case.

TASK-4: doc sync — `docs/AGENTS.md`, `README.md`
  - `docs/AGENTS.md`: added `jobs` to the dispatcher blurb's command
    enumeration and a `mg jobs` bullet to the Commands list (after `mg job`).
  - `README.md`: added a `mg jobs` row to "The installed commands" table
    (after `mg job`).
  - Wording kept in sync with the `mg -h` help text from TASK-1.
  - Verified `agents/*.md` and `project-template/docs/AGENTS.md` need no
    change (neither enumerates the command surface beyond a passing `mg done`
    mention).

## Verification results

- `go vet ./...` — clean.
- `go test ./...` — full suite green, including `go test ./cmd/mg -run TestJobs -v`.
- `make mg` — builds `bin/mg`.
- End-to-end smoke with a scratch git repo + real worktree (`mg jobs < /dev/null`
  from the job worktree): lists rows with columns (ID/status/type/date/title,
  date-desc), then `Error: mg jobs needs an interactive terminal to select a
  job.` with exit 1. TTY selection path covered by the tests.

## Known issues / follow-ups

- **In-container smoke-test caveat**: the verification's
  `./bin/mg jobs < /dev/null` from this repo's root lists *nothing* inside the
  agent container, because this environment only mounts the current job's
  worktree at `/workspace` plus the shared git metadata — every other
  worktree (including this job's own) is registered at host paths
  (`/home/lmuskalla/code/...`) that don't exist here, so `job.Discover`'s
  git-worktree scan finds nothing (it degrades to an empty list, and the TUI
  behaves identically in-container). On the host, where the registered paths
  resolve, the repo's own job appears — this is an environment artifact, not a
  command bug. The full behavior was verified here with a hermetic scratch
  repo (real worktrees) and the unit tests.
- **Mid-session git accident, repaired**: a botched smoke-test command
  (an empty scratch variable caused `git init`/`git add`/`git commit` to run
  against the workspace repo itself) briefly created two junk commits on this
  branch (`init`, `job alpha`) and overwrote the shared repo's
  `user.email`/`user.name`. Both were repaired: the branch was reset back to
  the TASK-1 commit (junk commits and a stray `feature/def02_beta` ref
  deleted) and the identity config restored to the repo's author values. The
  final history on `feature/pz01od_mg-jobs` is clean: brief → TASK-1 → TASK-2
  → TASK-3 → TASK-4 → this summary.
- **TASK-1/TASK-2 boundary**: TASK-1's commit contains a minimal `runJobs`
  stub in `cmd/mg/jobs.go` so the dispatcher wiring compiles standalone (the
  switch case must reference a callable `runJobs`, per TASK-1's own notes);
  TASK-2's commit replaces it with the full implementation. No behavioral
  residue — the stub is gone in the final tree.
- `docs/NAMING.md` and `docs/CODE_QUALITY.md` mention the command surface
  but were explicitly out of TASK-4's scope (files: `docs/AGENTS.md`,
  `README.md`); they remain accurate as-is.
