# Brief: Close the lifecycle hole: orphaned-worktree detection and cleanup

status: open
type: feature
id: nepbxu
branch: feature/nepbxu_close-the-lifecycle-hole-orphaned-worktree-detection-and-cleanup
date: 2026-08-13
author: Leander Muskalla

## What

Make `mg jobs` or `mg delete` surface orphaned worktrees and offer to remove them, mirroring `git worktree prune` semantics but with the confirmation discipline `mg delete` already has.

An orphan is a registered worktree whose metadata is gone, or vice versa. Concretely, the five dead directories in `.manigot-worktrees/` (`o3kk3n_jdi-is-broken`, `a75hdc_opencode-jdi-issues`, `6ro7eg_add-stage-to-overview`, `sd62w9_add-jdi-in-overview`, `7431d6_different-configurable-docker-images`) — each ~3.5 MB with `.git` files pointing at gitdirs that no longer exist — are the proof the current lifecycle leaves no tool path to clean up after itself.

## Why

A job scaffolded and then abandoned leaves no branch, no worktree registration, no entry in `mg jobs` — and no way to clean it up through the tool. The user is left `rm -rf`-ing directories by hand and wondering whether they're safe to delete. This quietly erodes trust in the lifecycle; the tool should clean up after itself.

## Out of scope

- Anything else in the `mg jdi` loop or the TUI overview (separate jobs `63quv2`, `3iqg8j`, `ru97hg`).
- Deleting the five existing orphan dirs by hand in this job's code — the job should make the tool able to do it, and the confirmation workflow should be exercised on them as the acceptance test.

## Notes

- Mirror `git worktree prune` semantics, but keep `mg delete`'s confirmation discipline and "This cannot be undone." wording.
- The orphaned dirs are the acceptance-test fixture.
