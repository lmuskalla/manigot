# Verdict: mg jobs

id: pz01od
status: open
reviewer: @reviewer
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `cmd/mg/main.go` adds `case "jobs":` calling `runJobs(args[1:], os.Stdin,
os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin))` — the same shape as `runAgents`
— adds `mg jobs  List jobs and pick one to start a session in` to the `mg -h`
Commands block directly under `mg job`, and adds `jobs` to the package doc's
command enumeration. Deviation noted: TASK-1's commit also introduces a minimal
compiling `runJobs` stub in `cmd/mg/jobs.go` (TASK-1's files list named
`cmd/mg/main.go` only) so the tree stays buildable at every commit; TASK-2
replaces it wholesale and no residue remains in the final tree (verified).
Non-blocking.

TASK-2: PASS
notes: `cmd/mg/jobs.go` implements every locked decision; verified line by line.
Root resolution + missing-project error mirror `cmd/mg/job.go`
("Error: could not find project root (no docs/ directory found)."); listing via
`job.Discover` in date-desc order with the TUI's column widths (8/6/8/12 +
title); `jobsBadge` renders `[running @<agent>]` / `[finished]` / `[needs
human]` gated by `job.ReadJDIStatus(root, j.Name)`; non-TTY prints the list then
"Error: mg jobs needs an interactive terminal to select a job." (exit 1); TTY
selection via `cli.Select`; foreground re-exec of `mg --job <id> <passthrough>`
via the existing `reexec` helper with "→ Starting a session in <id>..."; empty
list prints the invite and exits 0.

TASK-3: PASS
notes: `cmd/mg/jobs_test.go` covers list rendering + badge + date-desc ordering,
non-TTY refusal wording and exit code, TTY selection → launch line with
passthrough, empty-list exit 0, missing-project-root error, and a `jobsBadge`
unit test for all three states plus the no-sidecar case. Fixtures write sidecars
with `job.WriteJDIStatus` (real on-disk format) and use non-git temp dirs
exercising `job.Discover`'s working-tree fallback — the git-worktree-backed
discovery path is covered by the job package's own tests, which is acceptable.
Full suite re-run fresh (`go clean -testcache && go test ./...`): green.

TASK-4: PASS
notes: `docs/AGENTS.md` dispatcher enumeration and Commands list gain `mg jobs`;
`README.md`'s installed-commands table gains the `mg jobs` row after `mg job`.
Wording consistent with the `mg -h` text. Sync rule verified: `agents/*.md` and
`project-template/docs/AGENTS.md` do not enumerate the command surface, so no
change was needed there.

## Commit discipline

Each task has its own commit in the correct format: `79cfbcf` `[pz01od]
TASK-1: dispatch mg jobs subcommand`, `c6492de` `[pz01od] TASK-2: implement mg
jobs command`, `75582d4` `[pz01od] TASK-3: add mg jobs tests`, `ad1a9fa`
`[pz01od] TASK-4: sync mg jobs docs`, `a36e379` `[pz01od] implementation: add
summary`. Per-commit file scopes match the tasks (the TASK-1 stub exception is
documented in implementation.md and leaves no residue). `tasks.md` (the analyst
deliverable, uncommitted in the working tree at handoff) was picked up into
TASK-1's commit — reasonable workflow handling. No amends, no history rewrite
beyond the developer's own documented repair of its mid-session accident, which
I verified: final history is `Scaffold → brief → TASK-1..4 → summary`, tree
clean, git identity config restored to the repo's author values.

## Security

None run (no @security pass requested). Reviewed informally: no credentials or
secrets involved; the new code only reads existing job files and the mg-jdi
status sidecar (read-only); the only exec is the pre-existing `reexec` pattern
(`mg --job <id>`). No new file writes, no shell interpolation of untrusted
input.

## Overall

APPROVED

All four tasks are individually correct and match their descriptions in
`tasks.md`. `go vet ./...` clean, full `go test ./...` suite green (re-run
fresh, not cached), `make mg` builds, and the interactive flow was verified
end-to-end by the developer on a hermetic scratch repo with real git worktrees.
The in-container smoke-test limitation (worktrees registered at host paths that
don't resolve inside the agent container, so `job.Discover` finds nothing) is an
environment artifact that affects the TUI identically, not a command bug — it is
documented in implementation.md. No out-of-scope changes. Two non-blocking
observations for the backlog: (1) `jobsBadge` duplicates the TUI's
`jdiStatusBadge` formatting logic — a future shared plain-text helper could
unify them; (2) the TASK-1 buildable-stub deviation, already documented in
implementation.md. No further changes required before merge.
