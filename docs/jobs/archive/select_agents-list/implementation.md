# Implementation: agents list

id: select
status: open
developer: @developer
date: 2026-08-13

<!-- Produced by @developer after implementation. -->

## Summary

Every surface that lists agents — the `mg agents` plain (non-TTY) listing, the
`mg agents` interactive picker, and the TUI's "a" agents picker — used to
render each agent's full `description:` frontmatter string (100–200 chars,
e.g. `sysadmin`'s ~200) on the same line as the name, producing a wall of
text. This job caps the *rendered* description at a shared width (~60 chars)
with an ellipsis on all three surfaces, keeps one line per agent with the name
column and source tag visible, and keeps the full description in the pickers'
SearchKey so type-to-filter still matches on the whole description.

## Changes

TASK-1: Added the shared truncation building blocks in `internal/ui/app.go`:
`ui.Truncate` (the exported form of the existing `truncate`, with `truncate`
kept as a thin wrapper so existing callers — picker labels, job titles, commit
subjects, action-bar labels — behave identically) and the shared
`ui.AgentDescriptionWidth = 60` cap constant. `cmd/mg` already imports
`internal/ui`, so both are reachable from the CLI.

TASK-2: Capped the description in the `mg agents` plain (non-TTY) listing
(`cmd/mg/agents.go`, `runAgents`): the `  %2d) %-14s ...` row now renders
`ui.Truncate(a.Description, ui.AgentDescriptionWidth)`, keeping the name
column, the number, and the source tag intact.

TASK-3: Capped and reordered the `mg agents` TTY picker row labels
(`cmd/mg/agents.go`): each `ui.PickerRow.Label` is now
`name + source tag + truncated description` instead of
`name + description + tag`, so the shared `Picker`'s whole-label truncation
(which cuts from the end) takes the description and never the source tag.
`SearchKey` still carries `name + full description`, so type-to-filter is
unchanged.

TASK-4: Capped the description in the TUI agents picker
(`internal/ui/agentspicker.go`, `agentsPickerView.render`) to the shared
`AgentDescriptionWidth` *and* to the room left on the row after its fixed
20-char prefix (2-col marker + 16-col name column + 2-col gap), so a long
description can never push a row past the terminal edge. Name column, cursor
highlight, and key hints are unchanged.

TASK-5: Extended the tests:
- `cmd/mg/agents_test.go`: `TestAgentsListingCapsDescription` (plain listing
  truncates with ellipsis, name column intact), `TestAgentsPickerRowsCapDescription`
  (label = name + tag + truncated description, full description preserved in
  SearchKey, ellipsis on long descriptions), plus a "no truncation for short
  descriptions" assertion in `TestAgentsPickerGetsAgentRows`.
- `internal/ui/agentspicker_test.go`: `TestAgentsPickerRenderCapsLongDescription`,
  `TestAgentsPickerRenderKeepsShortDescriptionWhole`,
  `TestAgentsPickerRenderCapsToViewWidth` (narrow width caps to remaining row
  width, not just the shared cap).
- `internal/ui/app_test.go` (new): `TestTruncate` pins the helper's
  at-cap/over-cap/ellipsis/`n <= 0` behavior; `TestTruncateWrapperMatchesExported`
  pins the unexported `truncate` as a thin wrapper.

## Known issues / follow-ups

- `go test ./...` is not fully green in this agent session: every failure is a
  pre-existing environmental one — the session's `git` shim blocks `git init`,
  which repo-setup tests across `internal/ui`, `internal/git`, `internal/job`,
  `internal/session`, and `cmd/mg` (lifecycle/jdi/orphan tests) require. All
  tests covering the changed code pass; the failures were confirmed present
  before this job's first commit.
- The cap width (`AgentDescriptionWidth = 60`) is a single tunable constant
  used by all three surfaces; the analyst flagged it as adjustable at review.
- Truncation is byte-based (the pre-existing `truncate` semantics, preserved
  via the thin wrapper), so a multi-byte character cut at the boundary renders
  as the ellipsis plus the partial final rune; descriptions containing wide
  chars may count differently from display columns. Out of scope per the
  "keep behavior" instruction, but worth noting if the cap is tuned later.
- Shortening the `agents/*.md` / `docs/agents/*.md` descriptions themselves is
  a separate lever, explicitly out of scope.
