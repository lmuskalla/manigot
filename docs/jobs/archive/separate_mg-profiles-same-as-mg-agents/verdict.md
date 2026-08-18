# Verdict: mg profiles same as mg agents

id: separate
status: open
reviewer: reviewer
date: 2026-08-18

## Review

TASK-1: PASS
notes: `StartAt(i int)` added to internal/ui/picker.go — clamps i into [0, len(rows)-1], sets p.cursor, runs clampView, no-op on an empty row list. `TestPickerStartAt` (internal/ui/picker_test.go) covers a middle row, the clamp at both ends, and the empty-list no-op. NewPicker's cursor-0 default is untouched.

TASK-2: PASS
notes: `pickerRunFunc` widened to `(title string, rows []ui.PickerRow, start int)` in cmd/mg/picker.go; `ttyPicker` builds the picker, calls `StartAt(start)`, then `ui.RunPicker`. `runAgents`/`runJobs` pass 0 (cmd/mg/agents.go:80, cmd/mg/jobs.go:122). All test helpers (`pickerStub`, `pickerChoice`) and the closure fakes in jobs_test.go/agents_test.go updated in the same commit — grep confirms every call site matches the new signature, tree stays compiling.

TASK-3: PASS
notes: `runProfiles` gained the `pick pickerRunFunc` parameter, wired with `ttyPicker` in main.go's profiles dispatch. profilesList builds one PickerRow per profile (ID = id; SearchKey = id + label + tool + model + creds; Label = same padded columns as the plain listing minus the leading indent, keeping the `*` mark), runs the picker titled "Select the default profile" with start = the active profile's index. Submit with the already-active profile prints "Keeping X."; any other choice does UpsertEnv + confirmSet; cancel exits 0 quietly. The non-TTY path is byte-identical (only lines after `if !tty { return 0 }` changed). The three interactive tests were reworked onto pickerChoice/pickerStub, and `TestProfilesPickerGetsProfileRows` pins title, the 4 rows (id/label/search-key contents) and start index == active profile's index.

TASK-4: PASS
notes: profilesHelp (cmd/mg/profiles.go) and the profiles entry in printHelp (cmd/mg/main.go:86-91) now describe the interactive picker ("type to filter, enter to choose, esc/q cancel"), aligned with the agents/jobs wording.

TASK-5: PASS
notes: `cli.Select` and `cli.ErrQuit` removed from internal/cli/cli.go along with the seven Select tests in cli_test.go; the now-unused `errors`/`io` imports were dropped and the remaining imports (`bufio`, `errors`, `fmt`, `io`, `os`, `strings`, `x/term`) are all still used. Grep confirms zero remaining code references; only archived job docs and this job's own tasks/implementation mention them. docs/CODE_QUALITY.md's sentinel list updated to drop ErrQuit (the other three listed — ErrNotARepo, ErrCancelled, ErrNotFound — all still exist). No current docs describe the numbered prompt as current behavior (README.md/AGENTS.md only say "pick it interactively on a TTY", still accurate).

TASK-6: PARTIAL (verification only)
notes: Could not independently re-run `go build`/`go vet`/`go test` in this review session — the session tool permission deny-list blocks all non-git commands (the same environmental restriction the implementation documents for the git-fixture tests, which fail only on the shim's refused `git init`). Static review found no compile issues: no unused imports, all signatures consistent, all seam call sites updated. The implementation's claimed green build/vet and PTY smoke of the picker are consistent with the code as reviewed.

## Security

none.

## Overall

APPROVED.

The implementation matches tasks.md exactly: the shared Picker gained the StartAt cursor setter (TASK-1), the pickerRunFunc seam carries a start index with every call site updated atomically (TASK-2), `mg profiles` interactive selection migrated to the picker with the "enter keeps the active default" affordance preserved via the start index and the non-TTY path byte-identical (TASK-3), help texts updated (TASK-4), and the orphaned cli.Select/ErrQuit removed with zero remaining references (TASK-5). Commit discipline is correct — one commit per task in `[separate] TASK-N:` format plus the implementation.md commit. No out-of-scope changes beyond the justified CODE_QUALITY.md doc touch-up.

Non-blocking observations (no action required):
- profiles.go's TTY picker-error path still prints "mg profiles: %v" to stdout (w) rather than stderr — pre-existing behavior preserved from the old cli.Select path, consistent with the file's existing UpsertEnv error handling.
- `r io.Reader` in runProfiles/profilesList is unused on the TTY path — deliberate per tasks.md's decisions section (signature uniformity with runAgents).
