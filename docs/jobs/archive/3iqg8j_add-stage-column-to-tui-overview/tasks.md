# Tasks: Add stage column to TUI overview

id: 3iqg8j
status: open
analyst: opencode
date: 2026-08-13

Produced by @analyst from brief.md.

## Task breakdown

TASK-1: Add the stage column to the TUI job list rendering.
       files: internal/ui/list.go
       depends: none
       risk: low — a self-contained column in renderJobRow/listColumns; job.Stage() already exists and is read-only

TASK-2: Update and extend the list-view tests for the new column.
       files: internal/ui/list_test.go
       depends: TASK-1
       risk: low — one existing assertion ("open    feature" adjacency in
       TestRenderListRecentActivityKeepsFloorWhenListFillsScreen) breaks when
       the stage column slots between status and type; a new test pins the
       stage for every workflow stage.

## Notes

- Rendering change only — no data model change. Reuse job.Stage().
- Column order: id / status / stage / type / date / title — stage sits next to
  status so the two state columns read together (the brief's at-a-glance goal).
- Stage names are up to 9 chars ("implement"), so the stage column is wider
  than the type column (8); title column absorbs the cost via the existing
  titleColsWidth math. Only the CLI `mg jobs` listing and detail view are out
  of scope per the brief.
