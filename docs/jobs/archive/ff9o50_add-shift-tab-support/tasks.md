# Tasks: Add shift-tab support

id: ff9o50
status: open
analyst: claude
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: In the "Settings" form, handle `shift+tab` in `settingsView.update` so it moves focus backward between the editor field and the tool selector (today only `"tab"` is matched; `shift+tab` falls through and, while the editor field has focus, is swallowed by the text input instead of moving focus — this is the "stuck" symptom from the brief). Since the form only has two focusable fields, backward and forward land on the same toggle, so this can reuse the existing `v.focus = 1 - v.focus` logic; also update the footer hint strings to mention `shift+tab`.
files: tui/internal/ui/settings.go
depends: none
risk: low — small, self-contained change to a key-switch statement in a two-state focus toggle; mirrors the existing `"tab"` case exactly.

TASK-2: Same fix as TASK-1 for the "New job" form: handle `shift+tab` in `newJobView.update` to move focus backward between the title field and the type selector, and update its footer hint strings to mention `shift+tab`.
files: tui/internal/ui/newjob.go
depends: none (independent of TASK-1, same pattern applied to a different file)
risk: low — same reasoning as TASK-1.

TASK-3: Add unit test coverage asserting `shift+tab` moves focus backward in both forms — a new test in tui/internal/ui/newjob_test.go alongside the existing `TestNewJobTabTogglesFocus`, and a new tui/internal/ui/settings_test.go (no such file exists yet) covering the same for `settingsView`, following the `shift+tab` assertion pattern already used in tui/internal/ui/detail_test.go.
files: tui/internal/ui/newjob_test.go, tui/internal/ui/settings_test.go (new)
depends: TASK-1, TASK-2
risk: low — test-only addition, no production code paths beyond what TASK-1/TASK-2 already touch.

## Notes

- Both forms have exactly two focusable fields, so a backward toggle is behaviorally identical to the existing forward toggle (`v.focus = 1 - v.focus` works for both directions). No new field-ordering logic is needed — this is scoped to making `shift+tab` reach that existing toggle instead of being swallowed by the focused text input.
- tui/internal/ui/detail.go already handles `shift+tab` (for its 5-tab bar, `case "shift+tab", "left":`) and its test (`detail_test.go`) is a useful precedent for TASK-3's test style.
- Out of scope per brief: any other views/forms not named in the brief (settings and new job pages only).
