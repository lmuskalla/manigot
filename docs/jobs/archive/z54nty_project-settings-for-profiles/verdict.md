# verdict

id: z54nty
status: open
reviewer: @reviewer
date: 2026-08-11

## Brief

"Where is the TUI profile saved? Make one central saving point shared by CLI
and TUI (switch in TUI -> bare `mg` uses it), and make the CLI able to select
the current profile, not just list."

## Review scope

- Branch: `feature/z54nty_project-settings-for-profiles` — matches the brief's
  `branch:` field.
- Diff reviewed: `git diff 830ba3d...HEAD` (the job's true base, parent
  `13d9037` = merge-base with main). Two commits that show up in
  `main...HEAD` — `fd171db` "Intermediary commit" and `148943c` "opencode auto
  mode" — are main's OWN commits made after this branch was cut; they are not
  changes on this branch and were excluded from the review.
- `go build` / `go vet` / `go test ./...` (tui module): all pass.
- `bash -n` on every touched script: pass.
- Working tree clean; no `.env` or secrets in the diff; no edits to the
  read-only context mounts (`docs/AGENTS.md` is the canonical source and was
  edited there, correctly).

## Per task

TASK-1: PASS
notes: `tui/internal/config/config.go` — `EnvFile()`/`readEnvProfile()`/
`writeEnvProfile()` plus reworked `Load()`/`Save()` make `MANIGOT_PROFILE` in
`manigot/.env` the single shared profile store. Precedence (env → legacy
`profile`/`tool` fields in tui-settings.json → claude-pro) is correct; invalid
env values are ignored rather than propagated; the upsert preserves every
other `.env` line (credentials, comments, model overrides), and handles a
missing file, a missing trailing newline, `export ` prefixes, and quotes.
`Save` no longer persists the profile to `tui-settings.json` (legacy, like
`tool`). 11 new tests cover precedence, migration, upsert preservation and
bash-written-format compatibility; all pass.

TASK-2: PASS
notes: `tui/internal/ui/settings.go` — doc comment and the rendered profile
row now state the profile is saved as `MANIGOT_PROFILE` in `manigot/.env`
shared with the CLI, answering the brief's "where is that saved?" inside the
UI itself. Rendering-only; keybindings and behavior unchanged;
`TestSettingsRender` still passes. Cosmetic note (not a blocker): the added
hint line is ~80 chars and may overflow in a very narrow terminal.

TASK-3: PASS
notes: `scripts/profiles.sh` — set logic factored into
`set_default_profile()`/`confirm_set()` (the positional `mg profiles <name>`
path is behaviorally unchanged), and bare `mg profiles` now prompts
interactively on a TTY (`[1-3, Enter keeps <current>, q quits]`), mirroring
`mg agents`/`mg setup` conventions; piped/non-TTY invocations exit after
listing. Both paths were exercised under a pseudo-TTY (select / keep / quit)
and non-TTY — all correct. All "independent of the TUI" messaging replaced
with shared-default wording in the script, its `-h` help, and `scripts/mg.sh`.

TASK-4: PASS
notes: `scripts/run.sh` — the precedence comment now names the TUI settings
screen as a writer of `$MANIGOT_PROFILE`. Comment-only; no behavior change;
the mechanism (run.sh sources `.env`, `PROFILE="${MANIGOT_PROFILE:-claude-pro}"`)
already picks up a TUI-written value — confirmed end-to-end in TASK-6.

TASK-5: PASS
notes: `docs/AGENTS.md` (config/tui-settings.json bullet, manigot/.env bullet,
profiles.sh bullet, run.sh bullet, Commands `mg profiles` line, Job-workflow
ProfileValue sentence), `README.md` (Profiles quickstart, Usage block, command
table, `j`-keybinding note, Settings section) and `scripts/init.sh`'s comment
all describe the shared store consistently with the implementation. No stale
"independent" claims remain (the one remaining "independent" hit in README is
about the JDI stall backstop, unrelated). `project-template/docs/AGENTS.md`
verified to need no change (it only mentions `--profile` usage).

TASK-6: PASS
notes: verification was real, not just asserted: `go build`/`go vet`/`go test
./...` pass; `bash -n` passes; an end-to-end check confirmed a real
`mg profiles zai` run (bash) is read back by Go `config.Load()` and a Go
`config.Save(opencode-go)` is shown as the active default by `mg profiles`
(bash), with all `.env` credentials preserved. I re-verified the mechanism
independently during this review.

## Overall

APPROVED

The brief is fully satisfied: `MANIGOT_PROFILE` in `manigot/.env` is now the
single profile default shared by CLI and TUI (switching in the TUI settings
screen changes what bare `mg` uses, and vice versa), and `mg profiles` both
lists and interactively selects the current profile. The implementation is
correct, well-tested, and in-scope.

Non-blocking observations (for the record, not blockers):
- `tasks.md` was never filled in by @analyst (still the empty template); the
  implementation was reviewed against the brief directly and matches it.
- This branch still tracks `docs/manigot.json` (contents `{}`, inherited from
  the base commit), but `main` deleted that file in its own later "Intermediary
  commit" (`fd171db`). Merging this job will resurrect the empty file on main
  — harmless (it is the "no conventions" default) but unintended; whoever runs
  `mg done` may want to drop it from the squash.
- `mg jdi --job <id>` without `--profile` still defaults to claude-pro
  (`tui/cmd/jdi/main.go`), ignoring the shared `MANIGOT_PROFILE`. Pre-existing
  behavior, outside this brief's scope (TUI-launched jdi passes the shared
  profile explicitly), noted only for consistency.
