# Tasks: mg profiles same as mg agents

id: separate
status: open
analyst: analyst
date: 2026-08-18

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Add an initial-cursor setter to the shared Picker so a picker can open with the cursor on a chosen row instead of always row 0 — e.g. `func (p *Picker) StartAt(i int)` that clamps i into [0, len(rows)-1], sets p.cursor and runs clampView (no-op on an empty row list) — with a test covering the clamp at both ends; existing callers keep NewPicker's cursor-0 default.
     files: internal/ui/picker.go, internal/ui/picker_test.go
     depends: none
     risk: low — an additive method on a self-contained component; no existing caller changes behavior.

TASK-2: Extend the injected picker seam in cmd/mg to carry a start index: change `pickerRunFunc` to `func(title string, rows []ui.PickerRow, start int) (id string, ok bool, err error)`, make `ttyPicker` build the picker, call StartAt(start), then ui.RunPicker; update `runAgents`/`runJobs` to pass 0 and update the shared test helpers (`pickerStub`, `pickerChoice`) plus the closure fakes in jobs_test.go/agents_test.go to the new signature — all call sites land in this one task so the build stays green.
     files: cmd/mg/picker.go, cmd/mg/agents.go, cmd/mg/jobs.go, cmd/mg/jobs_test.go, cmd/mg/agents_test.go
     depends: TASK-1
     risk: medium — touches the seam shared by two commands and their wiring tests; mechanical but must be atomic to keep the tree compiling.

TASK-3: Migrate `mg profiles`' interactive default-profile selection from the numbered `cli.Select` prompt to the ui.Picker on a TTY: add a `pick pickerRunFunc` parameter to `runProfiles` (wiring `ttyPicker` in main.go's profiles dispatch), build one PickerRow per profile (ID = profile id; SearchKey = id + label + tool + model + creds; Label = the same padded columns as the plain listing minus the row number, keeping the `*` active mark), run the picker titled "Select the default profile" with the cursor starting on the active default's index; on submit, chosen == active → print "Keeping <active>.", else UpsertEnv MANIGOT_PROFILE + confirmSet; on cancel exit 0 quietly. Keep the non-TTY path byte-identical (list + exit 0 — unlike jobs/agents there is no refusal, listing is the command's documented purpose). Rework the three interactive profiles tests (numbered-input "2\n"/"\n"/"q\n") onto pickerChoice/pickerStub and add a row-wiring test asserting title, the 4 rows (id/label/search-key contents) and the start index == active profile's index.
     files: cmd/mg/profiles.go, cmd/mg/main.go, cmd/mg/profiles_test.go
     depends: TASK-2
     risk: medium — the "Enter keeps the current default" affordance must survive (hence StartAt on the active profile); non-TTY output must stay byte-identical; choosing the already-active profile now prints "Keeping X." instead of re-writing .env (intended, minor wording change).

TASK-4: Update the `mg profiles` help text to describe the picker, aligned with the mg agents/mg jobs wording: `profilesHelp` in cmd/mg/profiles.go and the profiles entry in cmd/mg/main.go's printHelp ("interactive picker on a TTY — type to filter, enter to choose, esc/q cancel").
     files: cmd/mg/profiles.go, cmd/mg/main.go
     depends: TASK-3
     risk: low — documentation wording only.

TASK-5: Remove the now-unused `cli.Select` and `cli.ErrQuit` from internal/cli (their last caller was the old profiles prompt), along with the Select tests in cli_test.go — verify via grep that nothing references them afterwards; this is cleanup directly orphaned by TASK-3, mirroring how the archived examine_cli-menus-with-go job's verdict noted the same removal for jobs/agents.
     files: internal/cli/cli.go, internal/cli/cli_test.go
     depends: TASK-3
     risk: low-medium — dead-code removal in a shared prompt package; must confirm zero remaining callers and that no docs describe the numbered prompt as current behavior.

TASK-6: Verify the result: `go build ./...`, `go vet ./...`, `go test ./...` all green; manual smoke — `mg profiles` on a TTY (cursor opens on the active default, up/down/k/j, type-to-filter, enter keeps/sets, esc/q cancel), `mg profiles` off a TTY (unchanged listing), `mg profiles <name>` set path unchanged, and `mg jobs`/`mg agents` still behave identically through the widened seam.
     files: none (verification only)
     depends: TASK-3, TASK-5
     risk: low — verification; the real risk is a picker that misbehaves on a real terminal, which the manual smoke covers.

## Out of scope / decisions

- Non-TTY `mg profiles` keeps its current list-and-exit-0 behavior — a TTY-only enhancement, unlike `mg jobs`/`mg agents` which refuse off a TTY because selection is their only purpose.
- The TUI settings screen's profile selector (←/→ cycling in internal/ui/settings.go) is untouched — the brief names only the `mg profiles` CLI command.
- Picking the already-active profile prints "Keeping X." (the old bare-enter path) rather than re-writing MANIGOT_PROFILE — a deliberate, minor wording change.
- `r io.Reader` stays in runProfiles' signature even though the picker reads the terminal itself — runAgents already carries an unused `r`, keeping the command signatures uniform.
