# Tasks: Launch quick job

id: gcri88
status: open
analyst: @analyst
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Problem as understood

The TUI's list view lets you open a job's detail view (`enter`), create a new
job (`n`), and edit settings (`s`) — but every actual *session launch* today is
tied to a specific job **and** a specific agent: `launch.Agent` always builds
`cd <root> && sc --tool <tool> --agent <agent> --job <jobID>` (see
`tui/internal/launch/launch.go:shellCommand`). There is no way from the TUI to
just start a plain safecode session against the current project with no agent
and no job, for a quick ad-hoc change.

The good news: the container launcher **already supports this**. In
`scripts/run.sh`, both `--agent` and `--job` are optional (AGENT/JOB default to
`""`, `AGENT_FLAG` and `PROMPT_ARGS` stay empty, and the bare `sc --tool <tool>`
runs claude/opencode with no agent flag and no job prompt). So this job is a
**TUI-only** feature — no `scripts/`, `Dockerfile`, or container-side changes
are needed. We just need a new launch path that omits `--agent`/`--job`, and a
key in the list view to trigger it.

## Open questions / decisions before implementation

- **DECISION (TASK-2) — which key triggers it?** Recommended default: **`o`**
  ("open a session"), footer token `o quick`. Alternatives that are also free in
  the list view: `x`, `b`, `space`. Note the footer already contains the token
  `enter open` (for opening the detail view), so if `o` is chosen the footer may
  want a tiny reword (`enter open` → `enter view`) to avoid two "open" tokens —
  that reword is folded into TASK-2 and is optional. If a different key is
  chosen, only the footer/README strings change, not the code shape. Flag if
  @product-owner wants to weigh in.
- **Scope: list view only.** This breakdown puts the shortcut in the list view
  (where `n`/`s` already live). A detail-view shortcut is **out of scope** — the
  detail view is job-specific by definition, which is the opposite of "no job".
  Flag if a detail-view entry is also wanted.
- **No confirmation prompt.** Like `launch.Agent`, this spawns a detached new
  terminal and returns immediately; it does not ask "are you sure?". This
  matches every other launch action in the TUI. Flag if a confirm step is wanted.

## Task breakdown

<!-- TASK-1: Add launch.Quick — a bare-session launcher that reuses the existing
     resolve → buildCmd → detached-spawn path but omits --agent/--job.
     files: tui/internal/launch/launch.go, tui/internal/launch/launch_test.go
     depends: none
     risk: low — additive; the existing shellCommand and its tests are untouched.

     TASK-2: Wire a list-view key (default `o`) in App.updateList to call
     launch.Quick, surface the result/error in the footer status, and update the
     footer hint string.
     files: tui/internal/ui/app.go
     depends: TASK-1
     risk: low — one new case in an existing switch plus a string edit; no model
     or state changes.

     TASK-3: Document the new keybinding in the README's keybindings table (and
     the intro sentence about what firing a session opens, if it mentions the
     agent/job command specifically).
     files: README.md
     depends: TASK-2 (so the documented key matches the shipped one)
     risk: low — docs only.

     TASK-4: Build and run the Go test suite to confirm nothing regressed.
     files: none (verification only)
     depends: TASK-1, TASK-2
     risk: low — `go build ./... && go test ./...` from the repo root.
-->

### TASK-1 — Add `launch.Quick` (bare-session launcher)

**What.** Add a new exported `Quick(projectRoot, tool string) (string, error)`
to `tui/internal/launch/launch.go` that opens a new terminal running `sc --tool
<tool>` (no `--agent`, no `--job`) from `projectRoot`, and returns the same
short "where it opened" description (`launch.Agent` does today).

**How.** `Agent`'s body is: resolve `resolve.Safecode()` → build the inner shell
string → `buildCmd(inner)` → discard stdio → `cmd.Start()` → reap goroutine →
return desc. Only the "build the inner shell string" step differs for a bare
session. Recommended factoring to avoid duplicating ~15 lines of spawn/reap
logic:

- Extract the resolve-independent tail (`buildCmd` → stdio discard → `Start` →
  reap → return desc) into a private helper, e.g. `launchDetached(inner string)
  (string, error)`.
- `Agent` becomes: resolve → `inner := shellCommand(...)` → `launchDetached(inner)`.
- `Quick` becomes: resolve → `inner := quickShellCommand(...)` →
  `launchDetached(inner)`.
- Add `quickShellCommand(safecodePath, projectRoot, tool string) string` that
  builds `cd '<root>' && '<safecode>' --tool '<tool>'` (single-quoted via the
  existing `shellQuote`, empty `tool` defaulting to `config.ToolClaudeCode`
  exactly like `shellCommand` does) and wraps it with the existing
  `holdOnFailure`. Reuse `shellQuote`/`holdOnFailure` verbatim — do **not**
  duplicate the quoting logic.

Do **not** touch the existing `shellCommand` or its signature — its exact-output
tests (`TestShellCommandFormat` etc.) must keep passing unchanged, which is why
the bare-session builder is a separate function rather than a generalization.

**Tests** (`launch_test.go`, following the established string-level pattern —
there is no App-level spawn test for `Agent` either, deliberately):

- `TestQuickShellCommandFormat`: `quickShellCommand("/usr/local/bin/safecode",
  "/home/me/proj", "claude-code")` has prefix `cd '/home/me/proj' &&
  '/usr/local/bin/safecode' --tool 'claude-code'` and equals
  `holdOnFailure(<that prefix>)`.
- `TestQuickShellCommandOmitsAgentAndJob`: the result contains neither
  `--agent` nor `--job`.
- `TestQuickShellCommandDefaultsEmptyTool`: empty `tool` → contains
  `--tool 'claude-code'` (mirror of `TestShellCommandDefaultsEmptyTool`).
- `TestQuickShellCommandPassesOpencodeTool`: `opencode` → `--tool 'opencode'`.
- `TestQuickShellCommandQuotesPathWithSpaces`: a safecode path/root with spaces
  stays single-quoted (mirror of the agent equivalent).

**Files likely affected:**
- `tui/internal/launch/launch.go` (new `Quick`, `quickShellCommand`, shared
  `launchDetached`; `Agent` refactored to call `launchDetached`)
- `tui/internal/launch/launch_test.go` (new tests above)

**Depends on:** none.
**Risk:** low — purely additive on the launch path; the agent path's behavior
and tests are unchanged.

---

### TASK-2 — Wire a list-view key to launch a quick session

**What.** In `App.updateList` (`tui/internal/ui/app.go`, the `switch msg.String()`
around line 209), add a new case for the chosen key (default `"o"`) that calls
`launch.Quick(a.root, a.settings.ToolValue())`, then sets `a.status` to either
`"→ quick session in " + desc` on success or `cmdErrorText(err)` on failure —
mirroring exactly how `updateDetail`'s `agentForKey` branch reports an agent
launch (lines 339-347). No new `appState`, no new view, no state transition:
like the agent launch, this fires a detached terminal and stays on the list.

**Footer hint** (`app.go` line 488): add the new token to the hint string, e.g.
`"↑/↓ navigate · enter open · o quick · n new · s settings · ctrl+r refresh · q quit"`
(adjust the key/label to whatever TASK-2 actually binds; if `o` is chosen,
consider `enter open` → `enter view` to avoid two "open" tokens).

**Why no App-level unit test for the dispatch:** the existing agent-launch
dispatch in `updateDetail` is not exercised at the App level either, because it
really spawns a terminal — coverage lives at the `launch` package's string level
(TASK-1). Follow that same boundary here rather than introducing test-only
launch seams.

**Files likely affected:**
- `tui/internal/ui/app.go` (one new `case` in `updateList`, footer hint string)

**Depends on:** TASK-1 (`launch.Quick` must exist).
**Risk:** low — one new case in an existing switch and a string edit; touches no
model fields, no state machine, no rendering of job rows.

---

### TASK-3 — Document the new keybinding in the README

**What.** Update the README's "Keybindings → List view" table
(`README.md`, ~lines 365-372) with a row for the new key, e.g.:

`| \`o\` | launch a quick safecode session (no agent, no job) |`

Also check the "Supported platforms" intro (~line 328): it currently says firing
an agent opens `sc --tool <tool> --agent <name> --job <id>`. Add one sentence
noting that the quick-session shortcut opens a bare `sc --tool <tool>` instead.
Leave `AGENTS.md` / `docs/AGENTS.md` alone — neither enumerates TUI keys.

**Files likely affected:**
- `README.md`

**Depends on:** TASK-2 (document the key that actually shipped).
**Risk:** low — documentation only.

---

### TASK-4 — Build and run the test suite

**What.** From the repo root, run `go build ./... && go test ./...` (the TUI is
the only Go code; this is also what `make tui`'s build step exercises). Confirm
the new `launch` tests pass and no existing test regressed (in particular the
untouched `TestShellCommandFormat` family).

**Files likely affected:** none (verification only).
**Depends on:** TASK-1, TASK-2.
**Risk:** low.

## Notes for the developer

- `scripts/run.sh` needs **no** changes — `--agent` and `--job` are already
  optional. Do not edit it; that would be out of scope.
- Reuse `shellQuote`, `holdOnFailure`, `resolve.Resolve(resolve.Safecode())`,
  `buildCmd`, and `config.ToolClaudeCode` exactly as `Agent` does. The only
  genuinely new logic is the inner command string with no `--agent`/`--job`.
- Keep the bare-session builder a **separate** function from `shellCommand`
  rather than generalizing `shellCommand` to take optional agent/job — the
  latter would risk the existing exact-format tests and broaden the diff for no
  benefit.
- Match the status-line wording style of the agent launch (`"→ <agent> in <desc>"`)
  so the two launch paths feel consistent in the footer.
