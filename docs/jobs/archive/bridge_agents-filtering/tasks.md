# Tasks: agents filtering

id: bridge
status: open
analyst: Leander Muskalla
date: 2026-08-14

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Add type-to-filter to the TUI's "Launch an agent" picker (`agentsPickerView`), mirroring the shared `ui.Picker` the CLI `mg agents` path uses, so both menus behave identically.
     files: internal/ui/agentspicker.go
     depends: none
     risk: medium — changes the key-handling semantics of an existing input component (j/k/g/G/q shift roles while a filter is active, q becomes cancel when no filter, enter with zero matches is a no-op), but every change deliberately mirrors the already-shipped shared Picker; one existing test assertion on the footer wording must be updated in TASK-2.

TASK-2: Update and extend the agents-picker tests to cover the new filtering behavior, mirroring picker_test.go's existing type-to-filter coverage.
     files: internal/ui/agentspicker_test.go
     depends: TASK-1
     risk: low — test-only; follows the shared Picker's already-established test patterns (narrowing + cursor clamp, two-stage esc, backspace, nav/input interplay, filtered submit, filtered render).

TASK-3: Document the TUI agent-picker filtering in the canonical docs/AGENTS.md ("TUI and mg jdi" section) so the two launch paths' picker behavior is described consistently.
     files: docs/AGENTS.md
     depends: TASK-1 (for accurate key wording)
     risk: low — docs only; project-template/docs/AGENTS.md and agents/*.md are intentionally untouched because neither documents TUI picker keys today, so no sync divergence is introduced.

## Scope notes / decisions

- Search key matches the CLI exactly: case-insensitive substring over `Name + " " + Description`.
- Key semantics mirror the shared `ui.Picker` (CLI path) for parity: esc clears the filter first, then cancels; up/down/home/end always navigate the filtered list; backspace edits the filter; while a filter is active every printable key extends it (j/k/g/G/q type); with no filter j/k navigate, g/G jump, q cancels, and any other printable key starts a filter. This adds q-cancel to the TUI picker (previously a no-op) — a deliberate, documented decision for consistency with `mg agents`.
- Enter with a filter that matches nothing is a no-op (stays in the picker) instead of closing it, matching the shared Picker.
- No scrolling/viewport is added — the picker already renders the whole list; filtering only narrows it. Out of scope unless a review finds it necessary.
- The brief's Why/Out-of-scope sections are empty; scope was inferred from the What ("the TUI agents menu should have the filtering the CLI has") — if that reading is wrong, stop and ask.
