# Implementation: Add shift-tab support

id: ff9o50
status: open
developer: claude
date: 2026-08-10

<!-- Produced by @developer after implementation. -->

## Summary

Implemented shift+tab (reverse tab) support in the two TUI forms named in the
brief — "Settings" and "New job" — so users can tab backward through their
focusable fields instead of getting stuck when shift+tab was previously
swallowed by the focused text input. Both forms have exactly two focusable
fields, so the existing `v.focus = 1 - v.focus` toggle used for `"tab"`
already produces the correct backward result; the fix was to also match
`"shift+tab"` on that same case. Added unit test coverage for both forms.

## Changes

TASK-1: `tui/internal/ui/settings.go` — `settingsView.update` now matches
`case "tab", "shift+tab":` (previously `case "tab":` only) so shift+tab moves
focus backward between the editor field and the tool selector using the same
toggle logic as forward tab. Updated the two footer hint strings (editor-
focused and tool-focused) to read `tab/shift+tab` instead of `tab`.

TASK-2: `tui/internal/ui/newjob.go` — same fix applied to `newJobView.update`:
`case "tab", "shift+tab":` moves focus backward between the title field and
the type selector, and both footer hint strings updated to `tab/shift+tab`.

TASK-3: `tui/internal/ui/newjob_test.go` — added
`TestNewJobShiftTabTogglesFocusBackward`, mirroring the existing
`TestNewJobTabTogglesFocus`, asserting shift+tab toggles focus 0→1→0.
`tui/internal/ui/settings_test.go` (new file) — added a full test suite for
`settingsView` (there was none before), following the pattern of
`newjob_test.go`: initial focus, esc/enter actions, forward tab toggling,
`TestSettingsShiftTabTogglesFocusBackward` (the new coverage this task
requires), tool cycling scoped to tool-focus, text editing, and render
output. Uses the existing `keyMsg` helper (from `checkout_test.go`, already
used the same way in `detail_test.go`) to construct the `"shift+tab"`
`tea.KeyMsg`.

## Known issues / follow-ups

None — scope was limited to the settings and new job forms as specified in
the brief. `tui/internal/ui/detail.go` already handled shift+tab before this
job and was not touched.
