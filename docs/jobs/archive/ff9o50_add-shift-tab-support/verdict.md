# Verdict: Add shift-tab support

id: ff9o50
status: open
reviewer: claude
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `tui/internal/ui/settings.go` — `case "tab", "shift+tab":` added exactly
as specified, reusing the existing `v.focus = 1 - v.focus` toggle. Both
footer hint strings updated to `tab/shift+tab`. The match happens in the
top-level key switch before any fallthrough to `v.editor.Update(msg)`, so
shift+tab is correctly intercepted even while the text input has focus (the
"stuck" symptom from the brief is fixed).

TASK-2: PASS
notes: `tui/internal/ui/newjob.go` — identical fix applied to `newJobView.update`
(`case "tab", "shift+tab":`), same toggle logic, both footer hints updated.
Matches TASK-1 pattern exactly as required.

TASK-3: PASS
notes: `tui/internal/ui/newjob_test.go` gets `TestNewJobShiftTabTogglesFocusBackward`
asserting 0→1→0 via `keyMsg("shift+tab")`. New file
`tui/internal/ui/settings_test.go` adds a full test suite for `settingsView`
(none existed before) including `TestSettingsShiftTabTogglesFocusBackward`,
following the `newjob_test.go`/`detail_test.go` patterns as directed. Reuses
the existing `keyMsg` helper from `checkout_test.go` — no duplicate helpers
introduced. `go build ./...` and `go test ./internal/ui/...` both pass
(`ok`), `go vet ./...` is clean.

## Security

None — no security-relevant surface touched (pure key-handling in a local
TUI form, no input parsing/injection risk, no new dependencies).

## Overall

APPROVED

Scope is tight and matches brief.md/tasks.md exactly: only
`tui/internal/ui/{settings,newjob}.go` and their tests changed, plus the
job's own docs files. Each task has its own correctly-formatted commit
(`[ff9o50] TASK-N: ...`), and `implementation.md` has its own commit.
`implementation.md` accurately describes the diff. No unrelated refactors,
no scope creep. Build, tests, and vet all pass. Nothing to change before
merge.
