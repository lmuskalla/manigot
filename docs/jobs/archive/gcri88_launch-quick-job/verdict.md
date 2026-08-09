# Verdict: Launch quick job

id: gcri88
status: open
reviewer: @reviewer
date: 2026-08-09

## Review

TASK-1: PASS
notes: `tui/internal/launch/launch.go` — `Quick(projectRoot, tool string)
(string, error)` (lines 74-81) and `quickShellCommand(manigotPath,
projectRoot, tool string) string` (lines 154-161) added exactly as specified.
Empty `tool` defaults to `config.ToolClaudeCode`; `shellQuote` and
`holdOnFailure` are reused verbatim (not duplicated); the builder is a
separate function from `shellCommand`, which is byte-for-byte unchanged
(verified against `main`). The shared spawn/reap tail was extracted into
`launchDetached` (lines 92-108) and `Agent` (lines 54-61) now resolves, builds
its inner string, and delegates to it — the control flow (buildCmd → stdio
discard → Start → reap goroutine → return desc) is identical to the original,
so `Agent`'s behavior is preserved. All five required tests are present in
`launch_test.go` (`TestQuickShellCommandFormat`,
`TestQuickShellCommandOmitsAgentAndJob`, `TestQuickShellCommandDefaultsEmptyTool`,
`TestQuickShellCommandPassesOpencodeTool`,
`TestQuickShellCommandQuotesPathWithSpaces`) and follow the established
string-level pattern. They pass.
- Observation (non-blocking): the TASK-1 commit `f52db24` also added 98 lines
  to the repo-root `AGENTS.md` (it went from empty to the manigot project
  documentation). This is unrelated to the feature. The developer disclosed it
  in `implementation.md` ("Known issues / follow-ups") — it was a pre-existing
  working-tree edit swept in by `git add -A`. The content is legitimate project
  docs, so reverting would lose useful information; the clean fix would be to
  split it into its own commit, but it does not affect the feature's
  correctness or scope of the launch path.

TASK-2: PASS
notes: `tui/internal/ui/app.go` — a new `case "o":` (lines 242-252) in
`App.updateList` calls `launch.Quick(a.root, a.settings.ToolValue())` and
sets `a.status` to either `"→ quick session in " + desc` or `cmdErrorText(err)`,
mirroring the agent-launch reporting style. `o` was previously unbound in
`updateList` (no conflict with `q/esc/up/k/down/j/home/g/end/G/ctrl+r/enter/l/
right/n/s`). The footer hint (line 499) was updated to include the `o quick`
token and `enter open` was reworded to `enter view`, matching the TASK-2
decision. No model fields, `appState`, or view transitions were added. Status
wording is consistent with the `→ <agent> in <desc>` style.

TASK-3: PASS
notes: `README.md` — a new `o` row was added to the "Keybindings → List view"
table (line 373) and one sentence was added under "Supported platforms"
(lines 337-339) noting the `o` shortcut opens a bare `mg --tool <tool>`.
`AGENTS.md` / `docs/AGENTS.md` were correctly left alone (neither enumerates
TUI keys).

TASK-4: PASS
notes: From `tui/`: `go build ./...` succeeds, `go vet ./...` is clean,
`gofmt -l .` reports nothing, and `go test ./...` passes across all packages
including the five new `launch` tests and the untouched
`TestShellCommandFormat` family.

Commit discipline: PASS
notes: Four task commits + one implementation commit, all in the required
`[gcri88] TASK-N: ...` / `[gcri88] implementation: ...` format, one per task
in order. (The repo-root `AGENTS.md` bundling noted under TASK-1 is a scope
slip inside an otherwise correctly-formatted commit, not a commit-format
problem.)

## Security

none — not run. The change adds a new launch path that reuses the existing
resolve → buildCmd → detached-spawn flow with a strictly smaller command
string (no `--agent`/`--job`), so it introduces no new input handling, no new
shell interpolation (all values still pass through `shellQuote`), and no new
I/O or filesystem surface. No security review requested for this job.

## Overall

APPROVED

The feature is implemented exactly as specified across all four tasks: a clean
`launch.Quick` bare-session path that reuses the established spawn/quoting/
hold-on-failure machinery without touching the agent path, a list-view `o`
key wired with matching status reporting, README documentation, and a green
build + test suite. No bugs, no missing edge cases on the changed paths.

Nothing must change before merge. The one observation (the unrelated
`AGENTS.md` content bundled into the TASK-1 commit) is disclosed, benign
project documentation, and does not block merge; splitting it into its own
commit would be a nice-to-have cleanup, not a requirement.
