# Implementation: git log configurable

id: pdgatj
status: open
developer: claude
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

Made the number of entries shown in the TUI dashboard's recent-activity strip
configurable. A new personal preference, `recentActivityCount`, lives in
`config/tui-settings.json` (default 5, valid 1–100) and is editable as a 4th
field in the settings form (editor → base branch → recent activity → profile).
The strip's adaptive floor of 1 is unchanged; only the ceiling is now
configurable. The initial fetch at app startup now honors the configured count
(settings are loaded before the first refresh), and submitting the settings
form refreshes the strip immediately.

## Changes

TASK-1: Added `RecentActivityCount int` (`json:"recentActivityCount"`) to
`config.Settings`, a `DefaultRecentActivityCount = 5` constant, and a
`RecentActivityCountValue()` getter that returns the default when the value is
≤ 0 (the JSON zero value means "unset"), mirroring `ProfileValue()`. `Save`'s
existing `legacy` copy carries the new field into `tui-settings.json`
automatically. Extended `config_test.go` with a custom-value round-trip through
Save/Load, the default-when-zero getter test (including negative values), and a
missing-field legacy-file test.

TASK-2: Deleted the `recentActivityCeiling` constant from `app.go` (kept
`recentActivityFloor` = 1) and replaced both uses with
`a.settings.RecentActivityCountValue()`: the fetch count in
`refreshRecentCommits()` and the clamp upper bound in `recentActivityShown()`.
Fixed `NewApp` init ordering so `config.Load()` runs before the initial
`a.refreshRecentCommits()`, and added a `a.refreshRecentCommits()` call in
`updateSettings`' submit branch so the strip reflects a new count immediately on
return to the list. Updated the stale doc comments on the const block and both
functions.

TASK-3: Added the 4th settings-form field in `settings.go`: a numeric
`textinput` for the recent-activity count, placed after base branch and before
profile. `stFieldCount` 3 → 4; added `stFocusCount` (2) and renumbered
`stFocusProfile` to 3. Wired seeding (resolved number, with the default as
placeholder), `setFocus`, key routing in `update`, `resize`, a "Recent
activity:" render row with a dimmed helper line, and focus-aware `hint()`.
Validation: trimmed blank → default 5; otherwise must parse as an integer in
1–100, else the submit path sets the form's status and persists nothing. The
parsed value is exposed via `recentActivityCount()` so neither the App nor
`settingsValue()` reaches into the raw textinput; `settingsValue()` includes the
parsed count when valid. `app.go`'s `updateSettings` validates before any
`config.Save`/`project.Save`.

TASK-4: Extended `settings_test.go` for the 4-field tab/shift+tab cycle, added
seeding/editing/validation tests (`recentActivityCount()` errors on non-integer
and out-of-range values, blank → default, valid value parsed into
`settingsValue()`), an App-level test that an invalid count keeps the form open
with a status and changes nothing in memory, and extended `TestSettingsRender`
for the new row. Updated `list_test.go`'s generous-height test to pin
`a.settings.RecentActivityCount` to a custom value (7, with two extra commits so
the strip can actually reach it) and re-fetch after pinning — keeping it hermetic
against a developer's local `tui-settings.json` — instead of the deleted
`recentActivityCeiling` constant; floor assertions unchanged.

TASK-5: Updated the `config/tui-settings.json` bullet in `docs/AGENTS.md` to
mention the new `recentActivityCount` preference (default 5, range 1–100, max
entries in the dashboard's recent-activity strip) and its default in the
fallbacks list. `project-template/docs/AGENTS.md` and `agents/*.md` contain no
settings/strip references, so nothing to sync there.

## Known issues / follow-ups

none.
