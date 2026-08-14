# Verdict: agents filtering

id: bridge
status: open
reviewer:
date:

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: internal/ui/agentspicker.go — type-to-filter added to the TUI "Launch an agent" picker, mirroring the shared ui.Picker (CLI mg agents path) key-for-key: esc clears the filter before cancelling, up/down/home/end always navigate the filtered list, backspace edits the filter, every printable key extends an active filter (j/k/g/G/q type), with no filter j/k navigate, g/G jump, q cancels, and any other printable key starts a filter. `filtered()` matches the CLI's SearchKey construction exactly (case-insensitive substring over Name + " " + Description — cmd/mg/agents.go:70), `selected()` resolves against the filtered list, `clampCursor()` keeps the cursor in bounds on every filter change, and `render()` adds the filter line, "no matches" hint, and the filter-aware footer. Enter with zero matches is a no-op (apNone), matching the shared Picker. Verified the App wiring (app.go updateAgentsPicker / updateList "a" case) needs no change: a fresh agentsPickerView is constructed on every open, so the filter resets between openings, and apSubmit/selected() can only be reached with a non-empty filtered list and an in-bounds cursor. Deliberate, pre-existing divergences from the shared Picker are documented in tasks.md and are not blockers: ctrl+c quits the whole TUI via the global handler (not picker cancel), and the TUI picker adds no scrolling/viewport. Diff is confined to the four intended files plus job docs — no out-of-scope refactors.

TASK-2: PASS
notes: internal/ui/agentspicker_test.go — the one existing footer assertion in TestAgentsPickerRender was updated to the new no-filter footer ("type to filter", "esc/q cancel"), and six tests were added mirroring picker_test.go's type-to-filter coverage (narrowing + cursor clamp, two-stage esc, backspace editing incl. empty-filter no-op, nav/input interplay with the j/q role flip, filtered submit incl. no-matches no-op, filtered render incl. "no matches"). I traced every assertion against the implementation by hand (could not execute `go test` — the review session's git shim restricts bash to git commands); all are consistent. One observation, not a defect: TestAgentsPickerFilterRender's "impl" filter actually matches both developer and reviewer ("Review implementations."), but the test only asserts developer presence and analyst absence, so it passes either way.

TASK-3: PASS
notes: docs/AGENTS.md — the "TUI and mg jdi" section now documents the "a" launch-an-agent picker's filtering with the same wording structure as the CLI picker description ("↑/↓/k/j navigate, type to filter (case-insensitive substring over the agent's name and description, narrowing the list until esc clears it), enter launches the highlighted agent, esc clears the filter before cancelling, and q cancels"), which accurately matches the implemented behavior. README.md and agents/*.md were correctly left untouched (they do not document TUI picker keys).

Commit discipline: PASS — each task has its own `[bridge] TASK-N: …` commit plus a separate `[bridge] implementation: add summary` commit; the scaffold and brief commits are normal job-lifecycle commits. The analyst's tasks.md fill-in was swept into the TASK-1 commit by `git add -A`; this is disclosed in implementation.md's Known issues and is expected here (the analyst agent is read-only and cannot commit), and the content is the job's own task list — acceptable.

## Security

none — no security surface touched: pure UI input-component change (in-memory filter state, no new IO, no new subprocesses, no file access).

## Overall

APPROVED

No blockers. The implementation fulfils the brief ("the TUI agents menu should have the filtering the CLI has") with behavior that mirrors the shared Picker exactly for the documented key set, tests mirror the shared Picker's established coverage, and the docs describe both launch paths consistently. Note for the record: the ui-package suite's pre-existing TestRenderList*/TestPushKey* failures (git shim refusing `git init` in temp-repo tests) are unrelated to this change, as disclosed in implementation.md; they fail identically on the base branch in this environment.
