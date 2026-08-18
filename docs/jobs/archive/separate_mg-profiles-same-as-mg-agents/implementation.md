# Implementation: mg profiles same as mg agents

id: separate
status: open
developer: developer
date: 2026-08-18

<!-- Produced by @developer after implementation. -->

## Summary

`mg profiles`' interactive default-profile selection now uses the same
interactive picker as `mg jobs` and `mg agents` instead of the old numbered
`cli.Select` prompt. The shared Picker gained an initial-cursor setter
(`StartAt`) so the picker opens on the active default profile (a bare enter
keeps it — the old "Enter keeps X" affordance), and the injected
`pickerRunFunc` seam in cmd/mg now carries a start index. `cli.Select` and
`cli.ErrQuit` — orphaned by the migration — were removed from internal/cli.

## Changes

TASK-1: Added `StartAt(i int)` to the shared Picker in internal/ui/picker.go
— clamps i into [0, len(rows)-1], sets the cursor and runs clampView, a no-op
on an empty row list — plus `TestPickerStartAt` covering the clamp at both
ends and the empty-list no-op. Existing callers keep NewPicker's cursor-0
default.

TASK-2: Widened the injected picker seam in cmd/mg from
`func(title, rows) (id, ok, err)` to `func(title, rows, start) (id, ok, err)`:
`pickerRunFunc` (cmd/mg/picker.go) carries the new start param, `ttyPicker`
builds the picker, calls `StartAt(start)` and then `ui.RunPicker`;
`runAgents`/`runJobs` pass 0. The shared test helpers (`pickerStub`,
`pickerChoice`) and the closure fakes in jobs_test.go/agents_test.go were
updated to the new signature in the same commit so the tree stays compiling.

TASK-3: Migrated `mg profiles`' interactive selection to the ui.Picker
(cmd/mg/profiles.go): `runProfiles` gained the `pick pickerRunFunc`
parameter, wired with `ttyPicker` in main.go's profiles dispatch. On a TTY it
now builds one PickerRow per profile (ID = profile id; SearchKey =
id + label + tool + model + creds; Label = the same padded columns as the
plain listing, minus the leading indent the picker's own cursor prefix
provides, keeping the `*` active mark) and runs the picker titled
"Select the default profile" with the cursor starting on the active default's
index. Submit with the already-active profile prints "Keeping X."; any other
choice writes MANIGOT_PROFILE via UpsertEnv + confirmSet; cancel exits 0
quietly. The non-TTY path is byte-identical (listing + exit 0). The three
interactive tests were reworked onto pickerChoice/pickerStub and
`TestProfilesPickerGetsProfileRows` pins the title, the 4 rows
(id/label/search-key contents) and the start index == the active profile's
index.

TASK-4: Updated the help texts: `profilesHelp` in cmd/mg/profiles.go and the
profiles entry in cmd/mg/main.go's printHelp now describe the interactive
picker ("type to filter, enter to choose, esc/q cancel"), aligned with the
mg agents/mg jobs wording.

TASK-5: Removed the now-unused `cli.Select` and `cli.ErrQuit` from
internal/cli/cli.go (their last caller was the old profiles prompt) and the
seven Select tests from cli_test.go (dropping the now-unused `errors`/`io`
imports). Grep-verified zero remaining references: only archived job docs and
this job's own tasks.md mention them historically. docs/CODE_QUALITY.md's
sentinel-error example list was updated to drop the removed `ErrQuit`.

TASK-6: Verified `go build ./...` and `go vet ./...` green; `go test ./...`
green for every package except the git-fixture tests, which fail only because
this agent session's git shim refuses `git init` (environmental, pre-existing
— every failure message is the shim's "git 'init' is not allowed in agent
sessions"). Manual smoke: `mg profiles` off a TTY (unchanged listing, exit 0),
`mg profiles <name>` set path unchanged, and a PTY-driven `mg profiles` TTY
run showing the picker opening on the active default (`▶ *claude-pro`),
type-to-filter ("zen" → one row), enter setting the new default with the
confirmation, and exit 0.

## Known issues / follow-ups

- A real interactive-terminal smoke (arrow-key navigation and esc/q cancel by
  a human) was not possible in this unattended, non-TTY session; the PTY
  smoke above plus the picker's existing unit tests (navigation, cancel,
  scroll, filter) cover the paths.
- `r io.Reader` in `runProfiles`' signature is now unused on the TTY path
  (the picker reads the terminal itself) — kept deliberately for signature
  uniformity with `runAgents`, per tasks.md's decisions.
- The git-fixture test failures in this environment (`git init` refused by the
  session git shim) predate this job and are unrelated to it.
