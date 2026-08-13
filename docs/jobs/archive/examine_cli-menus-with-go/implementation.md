## Summary

Made the `mg jobs` and `mg agents` CLI selections pleasing to use by giving
them an interactive Bubble Tea picker on a TTY, replacing the old numbered
`cli.Select` prompt. A new reusable single-select picker component
(`internal/ui/picker.go`) renders a title plus a scrollable,
cursor-highlighted list of pre-rendered rows, supports ↑/↓/k/j/home/end
navigation, type-to-filter, enter to submit and esc/q to cancel, and runs on
the terminal's alternate screen. Non-TTY behavior of both commands is
byte-for-byte unchanged (plain listing + the existing "needs an interactive
terminal" refusal); cancelling the picker now exits 0 quietly instead of the
old `mg jobs: quit` error path.

## Changes

TASK-1: Added the reusable picker model — `internal/ui/picker.go` (+
`picker_test.go`): `PickerRow{ID, SearchKey, Label}`, a `Picker` tea.Model
(title + scrollable rows with cursor/offset windowing, up/down/k/j/home/end
+ g/G navigation, enter submit, esc/q/ctrl+c cancel, tea.WindowSizeMsg
resize, 80×24 default before the first resize) and `RunPicker` (runs it on
the alt screen). `PickerResult` reports the chosen row's ID or a cancel.
Tested directly through Update/View.

TASK-2: Added type-to-filter to the picker (same files): typing narrows the
list against each row's SearchKey (case-insensitive substring), backspace
edits the filter, esc clears it and only falls through to cancel once it is
empty, and the cursor is clamped into the filtered list. The navigation-vs-
input resolution: with no filter, j/k/q/g/G keep their TASK-1 roles; once a
filter is active every printable key (including j/k/q) extends it and
navigation moves to the arrow/home/end keys. A matching-nothing filter
renders "no matches" and enter does not submit.

TASK-3: Replaced `mg jobs`' `cli.Select` with the picker on a TTY —
`cmd/mg/jobs.go`: job rows are built from the same ID/status/type/date/
title + jdi-badge columns as the plain listing (search key id + title), the
picker runs with the alt screen via a new injected seam
(`pickerRunFunc`, `cmd/mg/picker.go`, following the confirm-func injection
pattern), submit prints "→ Starting a session in <id>..." and re-execs
`mg --job <id> <passthrough>`, cancel exits 0. The orphaned-worktree
surfacing + removal offer still runs before the picker (the bufio.Reader is
now created only for that confirm, so no shared reader sits on stdin ahead
of the picker). Non-TTY listing + refusal byte-identical. Tests reworked
(`jobs_test.go`): non-TTY tests pass a picker stub, the TTY launch test uses
an injected fake, and new wiring tests pin the rows fed to the picker and
the quiet cancel.

TASK-4: Mirrored TASK-3 for `mg agents` — `cmd/mg/agents.go`: rows show
name, description and the "(project)"/"(project override)" source tag
(search key name + description), submit re-execs
`mg --agent <name> <passthrough>`, cancel exits 0. Same seam, same
non-TTY preservation. Tests reworked the same way (`agents_test.go`).

TASK-5: Synced the docs — `docs/AGENTS.md` (the `mg agents` section's
stale "numbered selection" wording, the Commands list entries for both
commands, and the `internal/ui` architecture bullet now mentioning the
picker), `README.md` (installed-commands table rows and the Agents section
sentence), and the `mg --help` text in `cmd/mg/main.go`. `agents/*.md` were
scanned — no stale numbered-selection mention exists there.
`project-template/docs/AGENTS.md` has no `mg jobs`/`mg agents` content at
all (it is a blank per-project context template, only mentioning
`mg --profile` for vendor-neutrality), so there was nothing stale to update
in it; the sync rule (docs/AGENTS.md ↔ agents/*.md ↔ template) is
satisfied.

TASK-6: Verification — `go build ./...`, `go vet ./...`, and
`go test ./...` all green. Manual smoke under a real PTY (via a small
python pty driver) confirmed: the TTY picker renders (title, highlighted
cursor row, footer), ↑/↓ navigation, type-to-filter narrowing, enter
submit → "→ Starting a session in <id>/@<name>..." → re-exec, esc clears
the filter before cancelling, and esc/q cancel exits 0 quietly; the non-TTY
listing + refusal output is byte-identical to before. The re-exec launch
itself stops at the expected "CLAUDE_CODE_OAUTH_TOKEN is not set" error in
this sandbox (no credentials), which is the pre-existing session-launcher
behavior, not a regression.

## Known issues / follow-ups

- While the filter is empty, the vi keys j/k/q/g/G keep their navigation/
  cancel roles, so a filter cannot *start* with one of those letters (e.g.
  a job id or agent name beginning with "g"/"j"/"k"/"q"). Once any other
  character starts the filter, those letters type normally. This is the
  documented navigation-vs-input trade-off of the modeless design; the
  picker docstring and footer hint describe it. A dedicated filter-mode key
  (e.g. "/") would resolve it if it ever becomes a real annoyance.
- `mg profiles`' interactive default-profile selection still uses
  `cli.Select` — explicitly out of scope per tasks.md.
- The TTY-path tests exercise the picker through the injected seam rather
  than a real Bubble Tea program (that is the point of the seam); the
  real-terminal behavior is covered by the manual smoke.
