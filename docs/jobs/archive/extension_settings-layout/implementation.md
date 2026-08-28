# Implementation: settings layout

id: extension
status: open
developer: @developer
date: 2026-08-28

<!-- Produced by @developer after implementation. -->

## Summary

Restructured the mg tui settings form (`src/internal/ui/settings.go`) from a
flat wall of seven label+input+long-dim-example triples into two bold-headed
sections — **Personal settings** (editor, recent activity, profile, terminal,
theme) and **Project settings** (base branch, job branch prefix) — with a bold
headline per setting, the value next to it in an aligned label column, the
input value (not the headline) dimmed when unfocused, and shortened,
de-emphasized example lines. The form is compressed to **22 rendered lines**,
so the whole form — every field and the footer — now fits the 22 content rows
of a standard 24-row terminal (the render is a raw, non-scrollable string).
All interaction semantics (tab cycle order, profile ←/→ cycling, enter/esc,
focus-aware footer hint) are unchanged.

## Changes

TASK-1 (`src/internal/ui/settings.go`):
- Two section headlines (`headerStyle`, bold) each carrying the storage
  location once as a dim suffix ("saved in config/tui-settings.json +
  manigot/.env" / "stored in .manigot/manigot.json, shared with your team"),
  so the per-field "stored in …" clauses could be dropped.
- `settingsField` helper + `settingsLabelWidth` const (17 = "Job branch
  prefix"): every setting label is bold and right-padded to a fixed column so
  all text inputs start at the same x; when a field is unfocused only its
  input value dims, the bold headline stays readable.
- Examples shortened and kept `dimStyle` at a 2-space indent; the long
  "max entries shown in …"/"stored in config/tui-settings.json"/"saved as
  OPENCODE_THEME" clauses and the redundant "tool: … · billed via …"
  selected-profile description line were removed.
- `settingsView` doc comment rewritten to describe the two-section grouping
  and the height budget; `stInputWidth` updated for the wider label column.

Height-budget fix (reviewer re-work, same file):
- The initial 28-line render cut off the Job branch prefix field and the
  footer at a standard 80×24 terminal (content area is 22 rows). The render
  is now exactly 22 lines: the blank after the title and the per-field blank
  lines are gone (one blank remains, between the two sections), the
  MANIGOT_PROFILE persistence note moved onto the Profile headline line
  ("Profile — saved as MANIGOT_PROFILE in manigot/.env, shared with the
  CLI") so the information survives the row cut, and the base branch row's
  example line was dropped (its placeholder "main" already communicates the
  blank default). Verified by rendering at 80×24: 22 lines, footer and every
  field including Job branch prefix on screen.

TASK-2 (`src/internal/ui/settings_test.go`):
- `TestSettingsRender` updated to assert the new strings (section headlines,
  per-setting headlines, typed value, profile rows, shortened examples) and
  to assert the old phrasing is gone ("Editor:", "(project)" suffixes,
  per-field storage clauses, "billed via"); the "blank = main" assertion was
  removed with the dropped base-branch example.
- `TestSettingsRenderPersistenceNotesOncePerSection`: each persistence note
  appears exactly once (the merged profile note still matches).
- `TestSettingsRenderFitsHeightBudget` rewritten to assert the **real**
  budget — the render fits the 22 content rows of a 24-row terminal — plus
  a regression guard that the footer and every setting headline are present,
  so the height requirement the reviewer flagged is now actually guarded by
  a test (previously it pinned an arbitrary 28-line count).
- All model-level tests (focus cycle, shift+tab, profile cycling, per-field
  edit/seeding, validation) pass unchanged, as expected.

## Known issues / follow-ups

- The form is not scrollable, so it fits a 24-row terminal exactly (22
  content rows); terminals shorter than 24 rows will still clip the footer.
  That matches the original design's assumption and the task's budget.
- Full `go test ./...` in this session fails on ~53 ui-package tests that
  shell out to `git init`/tig in temp dirs — the agent session's git shim
  refuses `git init` ("git 'init' is not allowed in agent sessions") and tig
  is not installed. All settings/new-job tests pass; the failures are
  environment restrictions, not regressions from this change (same 53-test
  baseline before and after this work).