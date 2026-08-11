# Implementation: Rename jdi running indicator

id: fv3j7q
status: open
developer: claude
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

- TASK-1 (previously committed): dropped the `mg jdi: ` prefix from the
  `jdiStatusBadge` helper's three label variants in `tui/internal/ui/app.go`,
  and updated its doc comment.
- TASK-2: updated the two "omits badge" tests
  (`TestRenderListOmitsJDIBadgeWhenNoStatus`,
  `TestDetailActionBarOmitsJDIBadgeWhenNoStatus`) to assert the absence of
  the badge-only substrings (`[running`, `[finished]`, `[needs human]`)
  instead of the now-gone `"mg jdi:"` substring, keeping them meaningful.
  The positive badge assertions in both test files already used the
  prefix-free wording, so they needed no change.
- TASK-3: updated the README's "List-row badge" bullet under "mg jdi status &
  log" to the prefix-free wording.

## Changes

- `tui/internal/ui/list_test.go` — "omits badge" test now asserts absence of
  the three badge substrings instead of `"mg jdi:"`.
- `tui/internal/ui/detail_test.go` — same rewrite for the action-bar badge
  test, and its doc comment updated.
- `README.md` — "List-row badge" bullet now reads `[running @<agent>]` /
  `[finished]` / `[needs human]`.

## Known issues / follow-ups

None.
