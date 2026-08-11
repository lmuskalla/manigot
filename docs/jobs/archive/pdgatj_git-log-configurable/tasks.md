# Tasks: git log configurable

id: pdgatj
status: open
analyst: claude
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Scope confirmation

The "git log on the tui dashboard" is the read-only **recent-activity strip**
at the bottom of the list view, backed by `git.RecentCommits(root, n)`
(`tui/internal/git/git.go`). Its count is bounded by two constants in
`tui/internal/ui/app.go`:

- `recentActivityCeiling = 5` — fetched in `refreshRecentCommits()` (line 430)
  and used as the clamp upper bound in `recentActivityShown()` (line 461).
- `recentActivityFloor = 1` — a layout-minimum so the strip never pushes job
  rows down (`recentActivityShown()` clamps between floor and ceiling based on
  spare terminal height; `dashboardFixedChrome = 7`).

Settings plumbing: personal TUI prefs live in `config/tui-settings.json` via
`config.Settings` (`tui/internal/config/config.go`) — only `Editor` is stored
there today (Profile lives in `manigot/.env`). The settings form
(`tui/internal/ui/settings.go`) has 3 tab-cycled fields (editor → base branch
→ profile; `stFieldCount = 3`). On submit, `updateSettings` (app.go line 739)
persists via `config.Save`/`project.Save` and updates `a.settings`. Note:
`NewApp` calls `a.refreshRecentCommits()` (line 168) **before**
`config.Load()` (line 169) — ordering must be fixed so the initial fetch uses
the configured count.

## Design decisions (confirmed with the requester)

1. The new setting is a **personal** preference, stored in
   `config/tui-settings.json` (same file as `Editor`), not in the project's
   `docs/manigot.json`.
2. The setting is the **maximum** number of entries the strip may show —
   `recentActivityShown()` still scales down to the floor of 1 when the
   terminal is too short (it must never exceed available room / push job rows
   down). The adaptive behavior from 78fgoq is preserved; only the ceiling
   becomes configurable.
3. Default when unset/blank: **5** (current behavior). Valid range: **1–100**
   (cap confirmed). `recentActivityFloor` stays a hard constant of 1.

## Task breakdown

---

TASK-1: Add `RecentActivityCount` to `config.Settings` with default + tests.

Add `RecentActivityCount int` (`json:"recentActivityCount"`) to
`config.Settings`, a `DefaultRecentActivityCount = 5` constant, and a getter
(e.g. `RecentActivityCountValue()`) returning the default when the value is
≤ 0 — mirroring the existing `ProfileValue()` pattern. Zero (the JSON zero
value) means "unset → default", so `Save` persists it naturally with no extra
work (the `legacy`-copy pattern in `Save` already carries any new fields into
`tui-settings.json`). Extend `config_test.go`: round-trip of a custom value
through `Save`/`Load`, and default-when-zero.

files: tui/internal/config/config.go, tui/internal/config/config_test.go
depends: none
risk: low — additive struct field + pure getter; no call sites change yet.

---

TASK-2: Wire the configured count into the recent-activity strip in app.go.

Replace `recentActivityCeiling` (delete the constant; keep
`recentActivityFloor`) with `a.settings.RecentActivityCountValue()` in both
`refreshRecentCommits()` (fetch count, line 430) and `recentActivityShown()`
(clamp upper bound, line 461). Fix `NewApp` ordering: load settings via
`config.Load()` **before** the initial `a.refreshRecentCommits()` call, so the
first fetch honors the configured count (keep the existing non-fatal
load-error handling — status line — intact). In `updateSettings`' `stSubmit`
branch, call `a.refreshRecentCommits()` after `a.settings = s` so the strip
reflects the new count immediately on return to the list. Update the stale doc
comments (the `recentActivityFloor/recentActivityCeiling` const block,
`refreshRecentCommits`, `recentActivityShown`).

Note for tests: existing list tests construct `NewApp` which calls the real
`config.Load()` — tests that assert strip counts must pin
`a.settings.RecentActivityCount` explicitly (or assert against
`config.DefaultRecentActivityCount`) to stay hermetic against a developer's
local `tui-settings.json`.

files: tui/internal/ui/app.go
depends: TASK-1
risk: medium — touches `NewApp` init ordering and the settings-submit path;
`list_test.go` references to the deleted constant break compilation until
TASK-4 updates them.

---

TASK-3: Add the count field to the settings form.

Add a 4th focusable field to `settingsView`: a numeric `textinput` for the
recent-activity count, placed after base branch and before profile (profile
stays last in the cycle so wrap-around semantics stay intuitive). Update
`stFieldCount` (3 → 4) and the focus constants (add `stFocusCount`, renumber
`stFocusProfile` to 3). Wire through:
- `newSettingsView` — seed from `global.RecentActivityCountValue()` (empty
  input when default? or the resolved number? — either is fine; the blank =
  default rule below is what matters).
- `setFocus` — focus/blur the new input alongside the existing two textinputs.
- `update` — route key input to it when focused; `left`/`right` must still
  only cycle the profile when the profile field is focused.
- `resize` — apply `stInputWidth` to the new input too.
- `render` — new "Recent activity:" row + a dimmed helper line (e.g.
  "blank = 5 · max entries shown in the dashboard's recent activity strip,
  stored in config/tui-settings.json").
- `hint()` — extend the focus-aware footer hints for the new field.

Validation (confirmed): trimmed empty → default 5; otherwise must parse as an
integer in 1–100; anything else sets `status` (form stays open, nothing
persisted). Expose the parsed value so `settingsValue()`/`updateSettings` get
it without the App reaching into the view's raw textinput.

files: tui/internal/ui/settings.go
depends: TASK-1
risk: medium — the focus-cycle constant renumber and render/hint changes
ripple through all settings tests until TASK-4 updates them.

---

TASK-4: Update UI tests for the new field and the configurable ceiling.

In `settings_test.go`: extend the tab and shift+tab cycle tests from 3 to 4
fields; add tests for seeding, editing, and validation (invalid text → status
set + value unchanged, blank → default, valid number parsed into
`settingsValue()`); extend `TestSettingsRender` to expect the new row.

In `list_test.go`: replace the two `recentActivityCeiling` references (lines
153–154) — pin `a.settings.RecentActivityCount` in the test (or assert against
`config.DefaultRecentActivityCount`) and keep the generous-height test
asserting the strip reaches the configured max, not the deleted constant. The
floor assertions (lines 185–186) stay as-is.

files: tui/internal/ui/settings_test.go, tui/internal/ui/list_test.go
depends: TASK-2, TASK-3
risk: medium — must pin the exact new semantics (ceiling configurable, floor
fixed) and keep tests hermetic against real on-disk settings.

---

TASK-5: Sync documentation.

Update the `config/tui-settings.json` bullet in `docs/AGENTS.md` (currently
says it holds only "which editor opens brief.md") to mention the new
`recentActivityCount` preference (default 5, range 1–100, max shown in the
dashboard's recent-activity strip). Leave archive-job docs untouched
(historical). `project-template/docs/AGENTS.md` and `agents/*.md` were checked
and contain no settings/strip references, so nothing to sync there.

files: docs/AGENTS.md
depends: none (can proceed in parallel)
risk: low — documentation-only.

---

## Out of scope

- No change to `recentActivityFloor` (1) — it stays a hard layout constraint.
- No interactive git log viewer, drill-down, or filtering (per qge358's
  locked decision).
- No project-scoped (team-shared) variant of the setting — this is a personal
  pref, per confirmed decision 1.
