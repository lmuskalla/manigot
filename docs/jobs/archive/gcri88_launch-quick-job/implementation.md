# Implementation: Launch quick job

id: gcri88
status: open
developer: @developer
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

Added a "Quick Job" launch path to the TUI so a user can start a bare safecode
session against the current project with no agent and no job, straight from the
list view. This is a TUI-only feature — `scripts/run.sh` already treats
`--agent` and `--job` as optional, so no container-side or scripting changes
were needed. Pressing `o` in the list view spawns a detached new terminal
running `sc --tool <tool>` (same spawn paths as an agent launch), and reports
the outcome in the footer status line, mirroring how agent launches are
reported.

## Changes

TASK-1 — Add `launch.Quick` (bare-session launcher):
- `tui/internal/launch/launch.go`: extracted the shared resolve-independent
  spawn/reap tail (buildCmd → stdio discard → Start → reap goroutine → return
  desc) into a private `launchDetached(inner string) (string, error)` helper.
  `Agent` now resolves, builds its inner string, and delegates to
  `launchDetached`. Added `Quick(projectRoot, tool string) (string, error)`
  which resolves, builds a `--agent`/`--job`-less inner string via the new
  `quickShellCommand`, and delegates to the same `launchDetached`. Added
  `quickShellCommand(safecodePath, projectRoot, tool string) string` — the
  `--agent`/`--job`-less counterpart to `shellCommand`, reusing `shellQuote`
  and `holdOnFailure` verbatim and defaulting empty `tool` to
  `config.ToolClaudeCode` exactly like `shellCommand`. `shellCommand` itself is
  untouched, so its exact-format tests keep passing unchanged.
- `tui/internal/launch/launch_test.go`: added `TestQuickShellCommandFormat`,
  `TestQuickShellCommandOmitsAgentAndJob`, `TestQuickShellCommandDefaultsEmptyTool`,
  `TestQuickShellCommandPassesOpencodeTool`, and
  `TestQuickShellCommandQuotesPathWithSpaces`, mirroring the agent-path tests.

TASK-2 — Wire list-view key `o` to `launch.Quick`:
- `tui/internal/ui/app.go`: added a `case "o":` in `App.updateList` that calls
  `launch.Quick(a.root, a.settings.ToolValue())` and sets `a.status` to either
  `"→ quick session in " + desc` on success or `cmdErrorText(err)` on failure,
  matching how `updateDetail`'s agent-launch branch reports a launch. Updated
  the footer hint string to include the new `o quick` token and reworded
  `enter open` → `enter view` to avoid two "open" tokens (per the TASK-2
  decision). No model fields, appState, or view transitions were added.

TASK-3 — Document the new keybinding:
- `README.md`: added an `o` row to the "Keybindings → List view" table and
  added one sentence under "Supported platforms" noting that the `o` shortcut
  opens a bare `sc --tool <tool>` (same spawn paths, no agent/job). Left
  `AGENTS.md`/`docs/AGENTS.md` alone, as neither enumerates TUI keys.

TASK-4 — Verification (no code changes): from `tui/`, `go build ./...` (and a
full `CGO_ENABLED=0 go build -trimpath` of the TUI binary) succeeds, `go vet
./...` is clean, `gofmt -l .` reports nothing, and `go test ./...` passes —
including the five new `launch` tests and the untouched
`TestShellCommandFormat` family.

## Known issues / follow-ups

- Like `launch.Agent`, the launcher's own later failure (e.g. gnome-terminal
  exiting non-zero because there's no display server) is not surfaced back to
  the TUI after `Quick` has already returned success — this is the existing,
  documented limitation of the detached-spawn design (see `Agent`'s doc
  comment). `holdOnFailure` still covers fast failures of the inner `sc`
  command itself.
- The list-view footer hint now reads `enter view` instead of `enter open`
  (reworded so the two tokens `enter view` / `o quick` aren't both "open").
  This is a docs/hint-string change only; no behavior change.
- A pre-existing working-tree edit that populated the repo-root `AGENTS.md`
  (from empty to the safecode project documentation) was present at job start
  and got bundled into the TASK-1 commit via `git add -A`. It is legitimate
  project documentation and unrelated to this feature's logic; flagged here
  for transparency.
