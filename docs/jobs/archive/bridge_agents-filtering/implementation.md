# Implementation: agents filtering

id: bridge
status: open
developer: Leander Muskalla
date: 2026-08-14

<!-- Produced by @developer after implementation. -->

## Summary

The TUI's "Launch an agent" picker (`agentsPickerView`, opened by "a" from
the list view) now supports type-to-filter, mirroring the shared `ui.Picker`
the CLI `mg agents` path uses, so both menus behave identically: typing
narrows the list against a case-insensitive substring of each agent's name
and description, esc clears the filter before cancelling, backspace edits it,
and while a filter is active every printable key extends it (j/k/g/G/q type
instead of navigating). q now cancels the TUI picker when no filter is
active (previously a no-op), and enter with a filter that matches nothing is
a no-op instead of closing the picker. Tests and docs updated accordingly.

## Changes

TASK-1: `internal/ui/agentspicker.go` — added a `filter` field to
`agentsPickerView`, a `filtered()` helper (case-insensitive substring over
`Name + " " + Description`, the same SearchKey the CLI builds), a
`clampCursor()` helper, reworked `update()` to mirror the shared Picker's
key semantics, made `selected()` resolve against the filtered list, and
extended `render()` with the filter line, "no matches" hint, and a footer
that changes while a filter is active.

TASK-2: `internal/ui/agentspicker_test.go` — updated the one existing footer
assertion in `TestAgentsPickerRender` (the no-filter footer now reads
"… enter launch · type to filter · esc/q cancel"), and added six tests
mirroring picker_test.go's type-to-filter coverage: narrowing + cursor
clamp, two-stage esc, backspace editing, nav/input interplay (j/q roles
flip with an active filter), filtered submit (including the no-matches
no-op), and filtered render.

TASK-3: `docs/AGENTS.md` — documented the TUI's "a" agent picker in the
"TUI and mg jdi" section, describing its filtering as behaving like the
CLI's `mg agents` picker, so the two launch paths' picker behavior is
described consistently.

## Known issues / follow-ups

- The pre-existing `tasks.md` fill-in by the analyst (uncommitted in the
  working tree at job start) was swept into the TASK-1 commit by
  `git add -A`; content is the job's task list and belongs in the repo.
- The ui-package test suite has pre-existing failures from the session git
  shim refusing `git init` in tests that create temp repos (e.g.
  TestRenderList* / TestPushKey*); unrelated to this change — they fail the
  same way on the base branch in this environment.
