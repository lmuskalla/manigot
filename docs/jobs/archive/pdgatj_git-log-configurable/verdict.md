# Verdict: git log configurable

id: pdgatj
status: open
reviewer: claude
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Verified on branch `feature/pdgatj_git-log-configurable` via
`git diff main...HEAD` (11 files), per-commit inspection, and a full build +
`go vet` + `go test ./...` (all packages pass).

TASK-1: PASS
notes: `RecentActivityCount int` (`json:"recentActivityCount"`) added to
`config.Settings` (config.go), `DefaultRecentActivityCount = 5` constant,
`RecentActivityCountValue()` getter returning the default when ≤ 0, mirroring
`ProfileValue()`. `Save`'s `legacy := s` copy carries the new field into
`tui-settings.json` automatically (verified at config.go:325-327). Tests cover
custom-value round-trip through Save/Load, default-when-zero (incl. negative),
and a legacy file without the field.

TASK-2: PASS
notes: `recentActivityCeiling` deleted (no references remain anywhere),
`recentActivityFloor` kept. Both uses replaced with
`a.settings.RecentActivityCountValue()`: the fetch in `refreshRecentCommits()`
(app.go:433) and the clamp upper bound in `recentActivityShown()` (app.go:463).
`NewApp` ordering fixed — `config.Load()` (app.go:167) runs before the initial
`a.refreshRecentCommits()` (app.go:176), non-fatal load-error status handling
intact. `a.refreshRecentCommits()` added to the `stSubmit` branch (app.go:769).
Doc comments on the const block and both functions updated. Pre-existing
`refreshRecentCommits` calls (refreshJobs, new-job submit) reuse the same
settings-backed count — consistent.

TASK-3: PASS
notes: 4th focusable field added in settings.go — `stFieldCount` 3→4,
`stFocusCount = 2`, `stFocusProfile` renumbered to 3, placed after base branch
and before profile in both render and tab cycle. Seeded from
`global.RecentActivityCountValue()` with the default as placeholder. `setFocus`
focus/blur wiring, key routing in `update` (profile `left`/`right` still gated
behind `focus == stFocusProfile`), `resize` width, "Recent activity:" render
row + dimmed helper line, and focus-aware `hint()` all present. Validation via
`recentActivityCount()`: trimmed blank → default 5; non-integer or outside
1–100 → error; `updateSettings` surfaces it as form status and persists
nothing (app.go:751-754). `settingsValue()` includes the parsed count only when
valid and never smuggles an invalid value out as 0 into a successful save.

TASK-4: PASS
notes: settings_test.go extends tab/shift+tab cycles to 4 fields, adds
seeding/editing/validation tests (invalid → error + `settingsValue` stays 0,
blank → default, valid parsed into `settingsValue`), App-level test that an
invalid submit keeps the form open with status and unchanged in-memory
settings, and extends `TestSettingsRender` with the new row. list_test.go
drops both `recentActivityCeiling` references; the generous-height test pins
`a.settings.RecentActivityCount = 7`, adds two commits so the repo can reach
it, and re-fetches after pinning (hermetic against on-disk settings); floor
assertions unchanged.

TASK-5: PASS
notes: `config/tui-settings.json` bullet in docs/AGENTS.md now documents
`recentActivityCount` (default 5, valid 1–100, max entries in the
recent-activity strip) and the fallback-defaults list gained "5 for the
recent-activity count". Verified `project-template/docs/AGENTS.md` and
`agents/*.md` contain no settings/strip references — nothing to sync.

Commit discipline: PASS — one commit per task in `[pdgatj] TASK-N: <desc>`
format (TASK-1..5) plus a separate `[pdgatj] implementation: add summary`
commit. TASK-3's commit includes the App-side submit validation, which is the
App half of that task's wiring — coherent.

Scope: PASS — the diff touches only the files named in the tasks plus the job
docs (brief/tasks/implementation); no unrelated refactoring.

## Security

none — no security-sensitive surface touched (local UI preference persisted in
a gitignored personal file; the form never writes outside config/ and the
project's docs/manigot.json).

## Overall

APPROVED

No changes required. Each task matches its spec, the configurable ceiling
preserves the adaptive floor behavior, the `NewApp` ordering fix makes the
first fetch honor the configured count, and all tests (including new coverage
for seeding/editing/validation and the configurable ceiling) pass under
`go build`, `go vet`, and `go test ./...`.
