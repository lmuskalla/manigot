# implementation

## Summary

The profile default is now stored in exactly one place — `MANIGOT_PROFILE` in
`manigot/.env` — shared by the CLI and the TUI. `scripts/run.sh` already read
that value for bare `mg` runs and `mg profiles` already wrote it; the TUI's
settings screen (`tui/internal/config`) now reads and writes the same key, so
switching the profile in the TUI switches the CLI default and vice versa. The
old `profile` field in `config/tui-settings.json` was demoted to a legacy
migration fallback (handled exactly like the older `tool` field). `mg
profiles` with no arguments now also lets the user pick the default
interactively on a TTY instead of only listing.

## Changes

TASK-1: Make `MANIGOT_PROFILE` in `manigot/.env` the single profile store.
`tui/internal/config/config.go` gained `EnvFile()`, `readEnvProfile()`
(best-effort `.env` scan, tolerating `export ` prefixes and quotes) and
`writeEnvProfile()` (upsert preserving every other line/comment, creating the
file with the standard header when missing). `Load()` now resolves the profile
as: `.env` `MANIGOT_PROFILE` (validated) → legacy `profile`/`tool` fields in
`tui-settings.json` (migration only) → `claude-pro`. `Save()` writes the
editor to `tui-settings.json` and the profile to `.env` only. Added 11 tests
covering precedence, migration, upsert preservation, and the bash-written
`.env` format.

TASK-2: `tui/internal/ui/settings.go` — the settings form's doc comment and
rendered profile row now state that the profile is saved as `MANIGOT_PROFILE`
in `manigot/.env`, shared with the CLI (`bare mg` / `mg profiles`), directly
answering the brief's "where is that saved?". No behavior or keybinding
changes.

TASK-3: `scripts/profiles.sh` — the set logic was factored into
`set_default_profile()` + `confirm_set()`, the bare `mg profiles` invocation
now prompts interactively on a TTY (`[1-3, Enter keeps <current>, q quits]`,
mirroring `mg agents`/`mg setup` conventions; piped invocations still just
list), and the set/list/help messages now describe the default as shared with
the TUI instead of independent of it. `scripts/mg.sh` usage/help updated
accordingly.

TASK-4: `scripts/run.sh` — the profile-precedence comment now notes that
`$MANIGOT_PROFILE` is written by both `mg profiles` and the TUI's settings
screen. No behavior change (`run.sh` already sourced `.env`).

TASK-5: Documentation sync. `docs/AGENTS.md` (the `config/tui-settings.json`
bullet — profile no longer stored there, legacy migration; the `manigot/.env`
bullet — `MANIGOT_PROFILE` is now the ONE value shared between CLI and TUI;
the `scripts/profiles.sh` bullet; the `run.sh` bullet; the Commands
`mg profiles [name]` line; the Job-workflow `ProfileValue` sentence),
`README.md` (Profiles quickstart, Usage block, command table, the `j`
keybinding note, and the Settings section — the profile is now stored in
`manigot/.env` as the shared default), and `scripts/init.sh`'s comment.
`project-template/docs/AGENTS.md` verified to need no change (only mentions
`--profile` usage).

TASK-6: Verification — `go build`/`go vet`/`go test ./...` all pass in
`tui/`; `bash -n` passes on every touched script; an end-to-end check
confirmed a real `mg profiles zai` run (bash) is read back by Go
`config.Load()`, and a Go `config.Save(opencode-go)` is shown as active by
`mg profiles` (bash), with all `.env` credentials preserved.

## Known issues / follow-ups

- Migration is passive: a profile previously saved only in `tui-settings.json`
  is honored on load (shown in the settings screen) and promoted into `.env`
  on the next settings save — the TUI does not write `.env` at startup without
  a user action.
- The TUI still always passes `--profile` explicitly on launches; that is now
  consistent because the value is seeded from the shared store.
- `mg profiles`' interactive prompt only appears on a real TTY; piped/CI
  invocations of bare `mg profiles` just list (deliberate, matching
  `mg setup`/`mg agents`).
