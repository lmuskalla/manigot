# Implementation: move stuff from docs

id: la222v
status: in-progress
developer: developer (deepseek-v4-flash)
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

Docs are for documents again. `docs/` was holding non-document content:
`quotes.json` (a runtime-flavor asset) and a pile of untracked node artifacts
(`package.json`, `package-lock.json`, 62 MB of `node_modules/`, and the
`docs/.gitignore` that existed only to hide them).

- `docs/quotes.json` → `assets/quotes.json` (`git mv`, 100% rename), with
  `scripts/run.sh` and `docs/NAMING.md` repointed at the new path.
- The untracked node artifacts were deleted from `docs/` — nothing in the
  tracked repo consumed them (no opencode plugin config exists anywhere).

## Changes

TASK-1: Moved `docs/quotes.json` → `assets/quotes.json` (`git mv`, 100%
rename, history preserved). `scripts/run.sh`: `QUOTES_FILE` now
`$MANIGOT_ROOT/assets/quotes.json` and the flavor-quote comment updated.
The quote is still picked on the host (run.sh runs outside the container)
and passed via the `MANIGOT_QUOTE` env var, so no container-side change is
needed. Matches the reference implementation in the unmerged `kwd2ou`
branch (commit `44bbcac`).

TASK-2: `docs/NAMING.md` — all three `docs/quotes.json` references (the
flavor-text section and the rap-sheet entry) updated to
`assets/quotes.json`.

TASK-3: Deleted the untracked node artifacts from `docs/`:
`docs/package.json` (single dep `@opencode-ai/plugin` 1.18.16), `docs/package-lock.json`,
`docs/node_modules/`, and `docs/.gitignore` (which ignored only those files
plus itself). Nothing in the tracked repo referenced or consumed them. A
prior job (`kwd2ou` TASK-6) had recommended keeping them as local
plugin-SDK state for the `docs/` → `/workspace/.opencode` container mount,
but this brief's "docs/ should be for documents only" supersedes that, and
no plugin or config exists that would break. These files were untracked, so
this task produced **no git changes** — there was nothing for git to record,
hence no commit for TASK-3 (an empty commit was deliberately avoided).

TASK-4: Verification — `bash -n scripts/run.sh` passes; the quote-pick logic
(grep/sed, same as run.sh) parses 109 quotes from `assets/quotes.json`
(byte-identical file, so identical to before the move); `git status` shows
exactly the intended change set (rename + two doc/script updates); `docs/`
contains only documents (`AGENTS.md`, `CLAUDE.md`, `NAME.md`, `NAMING.md`,
`backlog.md`, `jobs/`); `git grep` confirms zero remaining
`docs/quotes.json` references in tracked files.

## Known issues / follow-ups

- **`docs/jobs/archive/` untouched.** Historical job records were
  deliberately left alone per the task notes.
- **OpenCode plugin-dev workflow lost its mount trick.** `docs/package.json`
  + `docs/node_modules/` existed so the `@opencode-ai/plugin` SDK landed in
  the `/workspace/.opencode` container mount for in-session plugin imports.
  Nothing tracked consumed it and no plugin exists, so it was deleted rather
  than relocated. If plugin development ever becomes real, the clean fix is a
  root-level `package.json`/`node_modules` plus an opencode plugin config —
  explicitly out of this brief's scope.
- **Unmerged `kwd2ou` branch.** `feature/kwd2ou_move-settings-file` (archive
  commit `0c70ef1`) also contains this quotes move plus broader moves
  (`.manigot/manigot.json`, `.manigot/jdi-status/`) that were never merged
  into `main`. If any of those broader moves get picked up later, the
  quotes move here and that branch's TASK-4 overlap — resolve then.
