# Verdict: agents list

id: select
status: open
reviewer: @reviewer
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: internal/ui/app.go:1157 defines the shared `AgentDescriptionWidth = 60` constant; app.go:1164-1169 exports `Truncate` with the exact semantics of the old helper; app.go:1171-1173 keeps `truncate` as a thin wrapper, so every pre-existing caller (picker.go:253, list.go:254/256/275, detail.go:629) is byte-for-byte unchanged. `cmd/mg/agents.go` already imports `internal/ui`, so both are reachable from the CLI as required.

TASK-2: PASS
notes: cmd/mg/agents.go:57 caps the plain (non-TTY) listing description with `ui.Truncate(a.Description, ui.AgentDescriptionWidth)`; the `%2d` number, `%-14s` name column and source tag are untouched. The at-risk test TestAgentsListsWithTags uses only short descriptions (< 60 chars), so its substring assertions are unaffected. Non-blocking observation: the runAgents doc comment (agents.go:24-25) still claims the off-TTY listing is "byte-identical to before", which is no longer true for descriptions over 60 chars — cosmetic, comment only.

TASK-3: PASS
notes: cmd/mg/agents.go:76 reorders the picker label to name + source tag + truncated description, and SearchKey (agents.go:70) still carries the full description; the shared Picker filters on SearchKey (picker.go:185) and truncates the whole label from the end (picker.go:253), so the tag now survives that truncation — exactly the intent. Existing TestAgentsPickerGetsAgentRows (substring name/desc/tag) still passes with the reordered labels.

TASK-4: PASS
notes: internal/ui/agentspicker.go:102 caps the description to `clamp(v.width-20, 1, AgentDescriptionWidth)`; the row prefix is exactly 20 display columns (2-col marker + 16-col `pad`-padded name + 2-col gap), so a row can never exceed the terminal width. Name column, cursor highlight and key hints are unchanged. Degenerate widths ≤ 20 render just the ellipsis rather than the full description — no crash, acceptable.

TASK-5: PASS
notes: New tests cover every claimed behavior: listing cap with ellipsis (TestAgentsListingCapsDescription), picker rows cap + full-description SearchKey + no-truncation-for-short (TestAgentsPickerRowsCapDescription and the extension to TestAgentsPickerGetsAgentRows), TUI render cap for long/short/narrow-width (TestAgentsPickerRenderCapsLongDescription, TestAgentsPickerRenderKeepsShortDescriptionWhole, TestAgentsPickerRenderCapsToViewWidth), and Truncate edge cases incl. n <= 0 (TestTruncate, TestTruncateWrapperMatchesExported). None of the changed or new tests require `git init`, so the environmental failures noted in implementation.md (session git shim blocks `git init` for unrelated repo-setup tests) do not touch this job's coverage. Note: `go test ./...` could not be re-run in this review session (bash is restricted to git read/commit here); static analysis of the changed code and all affected pre-existing tests (TestAgentsListsWithTags, TestAgentsPickerGetsAgentRows, TestAgentsPickerRender, TestUpdateListAKeyOpensPicker) shows consistency — they all use short descriptions or the same truncation semantics.

Commit discipline: one commit per task in `[ID] TASK-N: description` format, and `implementation.md` has its own commit (`[select] implementation: add summary`). Non-blocking hygiene note: the analyst's tasks.md fill-in was committed inside the TASK-1 commit (44778be) rather than separately, and the gofmt whitespace removal shares the TASK-5 label — cosmetic history issues only, no functional impact.

Scope: only the files listed in tasks.md were changed, plus the job's own docs files and a one-line blank-line removal in app.go. No unrelated refactoring.

## Security

No security surface is touched — render-only changes to agent listing output, no new inputs, no privilege changes. `sysadmin`-class concerns (execution, mounts, git) are unaffected.

## Overall

APPROVED

No blockers. The three agent-listing surfaces (mg agents plain listing, mg agents TTY picker, TUI agents picker) now truncate descriptions identically via the shared `ui.Truncate`/`ui.AgentDescriptionWidth`, keeping one line per agent with the name column, source tag and type-to-filter (full description in SearchKey) intact — exactly what brief.md asked for. Two optional, non-blocking follow-ups if a cleanup pass is ever done: refresh the stale "byte-identical" doc comment in cmd/mg/agents.go, and consider a separate commit for analyst-produced tasks.md content.
