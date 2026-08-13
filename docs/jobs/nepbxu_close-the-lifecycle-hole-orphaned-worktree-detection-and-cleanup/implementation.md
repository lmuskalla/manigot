# Implementation: Close the lifecycle hole: orphaned-worktree detection and cleanup

id: nepbxu
status: open
developer:
date: 2026-08-13

<!-- Produced by @developer after implementation. -->

## Summary

Implemented orphaned-worktree detection and cleanup across the CLI, closing
the lifecycle hole where a job scaffolded and then abandoned leaves a dead
`.manigot-worktrees/` directory (a `.git` file pointing at a gitdir that no
longer exists) with no branch, no worktree registration, no entry in `mg jobs`,
and no tool path to remove it.

The implementation has two surfaces, sharing one detection + removal core:

- **`mg jobs`** surfaces orphaned worktrees after the job list and, on a TTY,
  offers to remove them ("Remove orphaned worktrees? [y/N]", then the removal
  with `mg delete`'s "This cannot be undone." discipline). On non-TTY it prints
  the listing plus a `mg delete <name>` hint.
- **`mg delete <id>`** resolves an orphan by name the way it resolves job ids
  (exact, then prefix) when no live job branch matches — a live job always wins
  over an orphan of the same name.

Detection mirrors `git worktree prune` semantics in both directions but where
prune only removes stale *metadata*, this removes the leftover *directories*
prune can't reach (and also runs `git.WorktreePrune` after removal to close the
metadata side). Confirmation discipline is `mg delete`'s own: what-will-be-
removed listing, "This cannot be undone.", and a Proceed? prompt; a decline is
`ErrCancelled` (the scripts' `exit 0`).

## Changes

- `internal/job/orphan.go` (new) — `Orphan` type, `DiscoverOrphans` (scans both
  the sibling and nested `.manigot-worktrees` layouts; a dir counts only when
  its `.git` file names a gitdir that no longer exists — live worktrees,
  standalone repos with a `.git` *directory*, and `.git`-less junk are all
  skipped), `MatchOrphan` (exact-then-prefix by name), `RemoveOrphans`
  (per-item confirmation + removal + prune), and `RemoveOrphansConfirmed`
  (batch removal for `mg jobs`' already-confirmed offer).
- `internal/job/finish.go` — added `ErrJobNotFound` sentinel and the
  `jobNotFoundError` shape (Unwrap → `ErrJobNotFound` while keeping the pinned
  error text byte-identical), so the CLI can distinguish "no such job" from a
  real failure.
- `internal/job/delete.go` — the non-git not-found path now returns the same
  sentinel-wrapped shape.
- `cmd/mg/jobs.go` — orphan surfacing + TTY removal offer in `runJobs`; one
  shared `bufio.Reader` across the orphan confirm and the job selection so no
  buffered input is lost.
- `cmd/mg/delete.go` — `runDelete` falls back to `job.MatchOrphan` →
  `job.RemoveOrphans` when `DeleteJob` returns `ErrJobNotFound`.
- `internal/job/orphan_test.go` (new) — detection across both layouts, live-
  worktree/standalone-repo exclusion, exact/prefix matching, confirmed and
  declined removal, stop-on-decline.
- `cmd/mg/jobs_test.go`, `cmd/mg/lifecycle_test.go` — CLI-level orphan listing,
  TTY removal offer (accept/decline), and `mg delete` orphan fallback
  (exact/prefix/declined).
- `docs/AGENTS.md`, `README.md` — documented the orphan surfacing and removal
  (`mg jobs` / `mg delete` bullets, the job-lifecycle section, the command
  table, and a short note in the Job workflow section).

## Known issues / follow-ups

- The acceptance fixture — the five dead dirs (`o3kk3n_jdi-is-broken`,
  `a75hdc_opencode-jdi-issues`, `6ro7eg_add-stage-to-overview`,
  `sd62w9_add-jdi-in-overview`, `7431d6_different-configurable-docker-images`)
  — was not present in this environment (the sandbox's `.manigot-worktrees/`
  was stripped), so the confirmation workflow was verified against synthetic
  fixtures built the same way (a `git worktree add` followed by deletion of the
  gitdir metadata and branch). The tool's behavior on the real five dirs should
  be exercised as the acceptance test.
- The TUI is deliberately untouched (out of scope per the brief — separate
  jobs `63quv2`, `3iqg8j`, `ru97hg`). `mg tui` still only lists jobs.

