# Tasks: agents list

id: select
status: open
analyst: mg jdi @analyst
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

Root cause (from brief.md): every surface that lists agents — the `mg agents`
plain listing, the `mg agents` interactive picker, and the TUI's "a" agents
picker — renders the agent's full `description:` frontmatter string on the same
line as the name. Descriptions run 100–200 chars (e.g. `sysadmin`'s ~200), so
the list is a wall of text and the target agent is hard to find.

Proposed approach (conservative): keep one line per agent and cap the
*rendered* description width with an ellipsis, keeping the fixed name column and
the source tag visible; keep the full description in the picker's SearchKey so
type-to-filter still matches on the whole description. This fits the existing
single-line `PickerRow` label model and the single-line `agentsPickerView`
render without reworking the shared `internal/ui.Picker`.

Open questions (flag at review, do not block implementation):
- Exact cap width (proposal below: ~60 chars for the description column) is
  tunable — pick one constant, used by all surfaces.
- Alternative considered and *not* recommended as the minimal change: wrapping
  the description onto a second indented line. It preserves full text but makes
  rows two lines tall and conflicts with the single-line `PickerRow` label model
  in `internal/ui.Picker`. Only pursue if truncation is rejected.
- Shortening the `agents/*.md`/`docs/agents/*.md` descriptions themselves is a
  separate lever and out of scope here unless explicitly requested.

TASK-1: Add a shared description-truncation helper + cap constant used by both the CLI and the TUI, so all three surfaces truncate identically (export the existing `truncate` in `internal/ui/app.go` as e.g. `ui.Truncate`, and add a shared `agentDescriptionWidth`-style constant; `cmd/mg` already imports `internal/ui`, so the helper is reachable from both).
     files: internal/ui/app.go, cmd/mg/agents.go, internal/ui/agentspicker.go
     depends: none
     risk: low — a pure addition; existing callers of `truncate` keep their behavior via a thin wrapper.

TASK-2: Cap the description in the `mg agents` plain (non-TTY) listing (`runAgents`, the `  %2d) %-14s ...` line) so each row stays one line with the name column and the source tag visible.
     files: cmd/mg/agents.go; tests: cmd/mg/agents_test.go
     depends: TASK-1
     risk: medium — the non-TTY listing is asserted by TestAgentsListsWithTags (substring Contains) and documented as byte-identical legacy output; the reformat must keep the asserted name/description/tag substrings present.

TASK-3: Cap the description in the `mg agents` TTY picker row labels and reorder each label to name + source tag + truncated description so the tag is no longer the part the shared Picker's whole-label width truncation cuts off; keep the full description in SearchKey.
     files: cmd/mg/agents.go; tests: cmd/mg/agents_test.go
     depends: TASK-1
     risk: low — TestAgentsPickerGetsAgentRows asserts row labels via substring Contains (name/desc/tag), which a reordered label keeps passing.

TASK-4: Cap the description in the TUI agents picker (`agentsPickerView.render`) to the shared cap and to the remaining view width so rows never spill past the terminal edge; keep the name column, cursor highlight and key hints unchanged.
     files: internal/ui/agentspicker.go; tests: internal/ui/agentspicker_test.go
     depends: TASK-1
     risk: low — a render-only change in a small self-contained view; the existing render test asserts substrings that remain present.

TASK-5: Extend the tests for the new formatting (truncated description present, source tag survives, full description still in SearchKey, ellipsis on long descriptions, no truncation for short ones) and confirm `go test ./...` is green.
     files: cmd/mg/agents_test.go, internal/ui/agentspicker_test.go, internal/ui/app_test.go (helper test)
     depends: TASK-2, TASK-3, TASK-4
     risk: low — tests only, but must cover the width/ellipsis edge cases to keep the suite meaningful.
