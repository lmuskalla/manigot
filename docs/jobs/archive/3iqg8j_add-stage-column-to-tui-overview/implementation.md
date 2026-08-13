# Implementation: Add stage column to TUI overview

id: 3iqg8j
status: open
developer: opencode
date: 2026-08-13

Produced by @developer after implementation.

## Summary

TASK-1: Added a stage column to the TUI job overview list, rendered between
status and type, populated from the existing `job.Stage()` (no data-model
change). The `columnWidths` struct and `listColumns()` gained a `stage` field
(width 10 — "implement" is the longest stage name at 9 chars), the
`titleColsWidth` math now accounts for the extra column and its separator,
and `renderJobRow` inserts the padded stage cell into each row.

TASK-2: Updated `TestRenderListRecentActivityKeepsFloorWhenListFillsScreen`,
whose "open    feature" adjacency assertion broke once the stage column
separated status and type (rows are now `status + stage + type`; the 
frontmatter-only briefs in that fixture read as unwritten → `define`). Added
`TestRenderListShowsStageColumn`, which exercises every workflow stage via
`mkStageJob` and asserts the padded stage cell appears in both the row and
the full list render.

## Changes

- `internal/ui/list.go` — `columnWidths` + `listColumns()`: added `stage`
  width 10; `render()`: `titleColsWidth` includes the stage column and its
  gap (5 separators now); `renderJobRow`: renders `j.Stage()` between status
  and type.
- `internal/ui/list_test.go` — fixed the floor test's row-count assertion
  and added `TestRenderListShowsStageColumn`.
- `docs/jobs/3iqg8j_add-stage-column-to-tui-overview/tasks.md` — filled in
  the task breakdown (@analyst).

## Known issues / follow-ups

- None. The CLI `mg jobs` listing and the detail view were left untouched per
  the brief's scope; the `columnWidths` doc comment ("used by the list and
  detail headers") is pre-existing and stale, but fixing it is out of scope.
