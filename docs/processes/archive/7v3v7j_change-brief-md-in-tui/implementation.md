# Implementation: Change brief.md in TUI

id: 7v3v7j
status: open
developer: Claude (developer)
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

Added an "e" keyboard shortcut to the TUI's job detail view that opens the
active tab's file in the user's terminal editor ($VISUAL → $EDITOR → nano →
vi), suspending the TUI in place (`tea.ExecProcess`) and reloading the file
once the editor exits. Per the scope decision confirmed with the user
before implementation, the shortcut is currently wired up for `brief.md`
only — the other three job files (`tasks.md`, `implementation.md`,
`verdict.md`) are meant to stay agent-written — but the underlying
plumbing (editor resolution, `tea.ExecProcess` wiring, reload-on-return) is
generic per-tab, gated by a single `editable` flag per file, so opening it
up to the other tabs later is a one-line change rather than new code.

## Changes

TASK-1: Added `tui/internal/editor/editor.go` — `Resolve()` returns the
editor command ($VISUAL, then $EDITOR, then the first of `nano`/`vi` found
via an injectable `LookPath` var), and `Command(path)` wraps it in a
`sh -c '<editor> "$1"'` invocation so editor values with arguments (e.g.
`code --wait`) work and the target path never needs shell-quoting. Covered
by `editor_test.go` (priority order, both fallbacks, not-found error,
argument handling) with no dependency on what's actually installed on the
host running the tests.

TASK-2: Wired the shortcut into `tui/internal/ui/detail.go` (added an
`editable` flag to the `jobFiles` table and `fileTab`, true only for
`brief.md`; added `reloadCurrent()`) and `tui/internal/ui/app.go` (new
`editorDoneMsg`, `App.editCmd()` builds the `tea.ExecProcess` command via
`editor.Command`, and an `"e"` case in `updateDetail` that only fires on
editable tabs — non-editable tabs fall through to normal no-op key
handling).

TASK-3: The success ("edited <file>") and error (`cmdErrorText`, same
formatting as failed agent launches) footer messages were already produced
by the TASK-2 `editorDoneMsg` handler; added
`tui/internal/ui/editordone_test.go` to pin both message paths and the
reload-after-save behaviour so they don't silently regress.

TASK-4: Checked `"e"` against every existing detail-view binding (tab/h/l
nav, 1-4 file select, j/k/pgup/pgdn/g/G scroll, the stage-dependent agent
keys a/p/d/r/s, ctrl+r, esc/backspace, q) — no collision. Added an
`"e edit"` footer hint in `renderFooter` (`detail.go`), shown only when the
active tab is editable, plus a regression test
(`TestDetailFooterEditHintOnlyOnEditableTab` in `detail_test.go`).

TASK-5: Added `TestEditorDoneMsgCreatesMissingFile` in
`editordone_test.go` — opening the editor on a job whose `brief.md` doesn't
exist yet (shown as the "(brief)" placeholder) and having the editor create
it on save correctly flips the tab to real content, since `loadTab` already
re-reads unconditionally.

TASK-6: Updated `README.md`'s detail-view keybindings table with the `e`
row and the `$VISUAL`/`$EDITOR`/`nano`/`vi` resolution order.

## Known issues / follow-ups

- The brief's original open question about scope (one tab vs. all four) was
  resolved with the user before implementation: brief.md only for now, kept
  modular for later. No further action needed unless that decision changes.
- `tea.ExecProcess`/editor behaviour (raw-mode suspend/resume around an
  interactive child process) isn't practically unit-testable end-to-end;
  coverage stops at the `editorDoneMsg` handler and the `editor` package's
  own resolution logic, which is the same boundary the codebase's other
  host-process code (`launch.Agent`) is tested to.
