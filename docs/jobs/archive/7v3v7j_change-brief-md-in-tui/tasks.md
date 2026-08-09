# Tasks: Change brief.md in TUI

id: 7v3v7j
status: open
analyst: architect (Claude)
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Open questions / assumptions

These should be confirmed (or the brief updated) before TASK-2 starts — see
the project rule "when scope is unclear: ask, don't guess".

- **Scope: one file or all four?** RESOLVED: brief.md only. The brief's
  trigger case is exclusively about editing a freshly-created brief.md
  template; tasks.md/implementation.md/verdict.md are agent-written outputs
  and stay read-only from the TUI for now. See brief.md's "Notes" section for
  the recorded decision. TASK-2/TASK-4 were implemented against this
  narrower scope (an `editable` flag on the file table, true only for
  brief.md), with the generic per-tab plumbing left in place so widening the
  scope later is a small follow-up.
- **Editor resolution order.** Assumed `$VISUAL` → `$EDITOR` → `nano` → `vi`
  (mirrors what git and most CLI tools do, and matches the brief's "e.g. nano
  or vim"). Flag if a different order, a config setting, or a hard requirement
  on `$EDITOR` being set is wanted instead.
- **Where it runs.** The TUI is host-side only (never runs in the container —
  see `docs/AGENTS.md`), so the editor process also runs on the host, using
  whatever `nano`/`vim`/etc. the user already has installed there. No
  container/Dockerfile changes are in scope.

## Task breakdown

TASK-1: Add an `internal/editor` package that resolves which editor command to
run — checks `$VISUAL`, then `$EDITOR`, then falls back to `nano`, then `vi`
(first one found via `exec.LookPath`) — and returns an error if none exist.
     files: tui/internal/editor/editor.go (new), tui/internal/editor/editor_test.go (new)
     depends: none
     risk: low — small, self-contained package with no side effects; the
     lookup step should be injectable so tests don't depend on the host's
     actual installed editors.

TASK-2: Wire the edit action into the detail view: on the chosen key, suspend
the Bubble Tea renderer and run the resolved editor against the active tab's
file path via `tea.ExecProcess` (same terminal, unlike `launch.Agent`'s
detached new-window spawn), then re-read that file into its viewer once the
editor process returns.
     files: tui/internal/ui/app.go, tui/internal/ui/detail.go
     depends: TASK-1
     risk: medium — `tea.ExecProcess` drives the terminal's raw-mode
     suspend/resume around an interactive child process; getting the
     completion message or error path wrong risks corrupting the alt-screen
     or leaving the TUI unresponsive to further key presses. bubbletea v1.2.4
     (already vendored) supports `tea.ExecProcess`, so no new dependency.

TASK-3: Surface the outcome in the detail view's footer — a short "edited
<file>" confirmation on success, or an error line (reusing the
`cmdErrorText`-style formatting already used for failed agent launches) if the
editor isn't found or exits non-zero.
     files: tui/internal/ui/app.go, tui/internal/ui/detail.go
     depends: TASK-2
     risk: low — status/formatting logic parallel to the existing agent-launch
     error handling in app.go.

TASK-4: Pick the trigger key, check it against every existing detail-view
binding (tab/h/l/1-4 nav, j/k/pgup/pgdn/g/G scroll, the stage-dependent agent
keys a/p/d/r/s, ctrl+r, esc/backspace, q) to rule out a collision, and add its
hint to the footer/action bar.
     files: tui/internal/ui/detail.go, tui/internal/ui/agents.go (only if the
     hint is rendered next to the agent buttons rather than in the footer)
     depends: TASK-2
     risk: low — presentation change once a non-colliding key is chosen, but
     the agent keys are stage-dependent (vary per job), so the check must
     consider all five, not just whatever the current job happens to show.

TASK-5: Confirm/adjust behaviour for tabs whose file doesn't exist yet
(tasks.md/implementation.md/verdict.md before an agent has run) — opening the
editor on a missing path should let the editor create it on save, and the tab
must flip from its "(label)" placeholder to real content afterward. Add a
regression test alongside the existing detail_test.go coverage.
     files: tui/internal/ui/detail.go, tui/internal/ui/detail_test.go
     depends: TASK-2
     risk: low — `loadTab`/`reload` already re-read from disk unconditionally,
     so this is mainly verification plus a targeted test, not new logic.

TASK-6: Update README.md's TUI "Keybindings" table (detail view section) to
document the new shortcut and the editor-resolution order.
     files: README.md
     depends: TASK-2, TASK-4
     risk: low — documentation only.
