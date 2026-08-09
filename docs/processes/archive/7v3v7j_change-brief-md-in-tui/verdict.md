# Verdict: Change brief.md in TUI

id: 7v3v7j
status: open
reviewer: reviewer (Claude)
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `tui/internal/editor/editor.go` resolves `$VISUAL` → `$EDITOR` →
`nano` → `vi` via an injectable `LookPath`, returns `ErrNotFound` otherwise,
and `Command()` shells out through `sh -c '<editor> "$1"'` so args in
`$EDITOR` (e.g. `code --wait`) and spaces/special chars in the path both
work. `editor_test.go` covers priority order, both fallbacks, not-found, and
argument handling without touching the host's real editors. `go build`,
`go vet`, `gofmt -l`, and `go test ./...` all pass.

TASK-2: PASS
notes: `tea.ExecProcess` wiring in `app.go` (`editCmd`, `editorDoneMsg`) and
`detail.go` (`editable` flag, `reloadCurrent`) is correct and works as
described — no raw-mode/terminal issues found, reload-on-return is wired
through `loadTab`. Follow-up commit `c18d48e` closes the previously-flagged
gap: `brief.md`'s "Notes" section now records the brief.md-only scope
decision and its rationale, and `tasks.md`'s open question is updated to
"RESOLVED: brief.md only" with a cross-reference to `brief.md`'s Notes —
this job's own docs now carry the audit trail that `implementation.md`
asserted. No code changed in this follow-up (confirmed via `git diff
main...HEAD -- tui/ README.md`, unchanged since the prior review); this was
a docs-only fix, appropriately scoped to the one blocker raised.

TASK-3: PASS
notes: success (`"edited " + filepath.Base(path)`) and error (`cmdErrorText`)
paths both wired through `editorDoneMsg` in `app.go`, matching the existing
agent-launch error formatting. `editordone_test.go` pins both paths plus the
reload-after-save behavior.

TASK-4: PASS
notes: `"e"` checked against every existing detail-view binding
(tab/h/l/1-4, j/k/pgup/pgdn/g/G, agent keys a/p/d/r/s via `agentMeta`,
ctrl+r, esc/backspace, q) — confirmed no collision by reading `detail.go`
and `agents.go` directly. Footer hint in `renderFooter` is correctly scoped
to editable tabs only, with a regression test
(`TestDetailFooterEditHintOnlyOnEditableTab`).

TASK-5: PASS
notes: `TestEditorDoneMsgCreatesMissingFile` verifies a job with no
`brief.md` yet starts on the "(brief)" placeholder and flips to real content
once the `editorDoneMsg` handler reloads after the simulated editor write.
Matches tasks.md's expectation that `loadTab`/`reload` already handle this
unconditionally.

TASK-6: PASS
notes: README's detail-view keybindings table documents `e` (scoped to
brief.md, with the rationale) and the `$VISUAL`/`$EDITOR`/`nano`/`vi`
resolution order.

## Security

None found. Editor invocation uses `exec.Command`/`sh -c` with the target
path passed positionally as `"$1"` (no string interpolation of the path into
the shell command), so paths with spaces or shell metacharacters can't break
out. `$VISUAL`/`$EDITOR` values are trusted the same way git/most CLI tools
trust them (an attacker would already need control of the user's own
environment). No container/Dockerfile changes, consistent with tasks.md's
note that the TUI and its child editor process are host-side only.

## Overall

APPROVED

Previously flagged blocker (TASK-2's undocumented brief.md-only scope
decision) is resolved in commit `c18d48e` — `brief.md`'s Notes section and
`tasks.md`'s open question now both record the decision and reference each
other. No code changed alongside the docs fix. Commit discipline holds
(`[7v3v7j] TASK-N: ...` per task, a separate implementation commit, and a
correctly-scoped `[7v3v7j] docs: ...` follow-up commit for the blocker
fix). Nothing further required before merge.
