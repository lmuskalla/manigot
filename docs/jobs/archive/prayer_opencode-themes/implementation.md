# Implementation: opencode themes

id: prayer
status: done
developer: claude
date: 2026-08-27

<!-- Produced by @developer after implementation. -->

## Summary

Implemented a global OpenCode theme setting for manigot, per tasks.md
TASK-1 through TASK-5. A new `OPENCODE_THEME` value in `manigot/.env` is
shared across every OpenCode profile (zai/opencode-go/opencode-zen/
opencode-zen-free) — unlike the per-profile `OPENCODE_*_MODEL` keys — and is
forwarded into OpenCode container sessions, which apply it via a generated
`~/.config/opencode/tui.json`. Claude Code needs no change: it already
respects the host terminal's own theme.

Research against https://opencode.ai/docs/themes/ and
https://opencode.ai/docs/config/ (both fetched live — developer sessions run
with network access) resolved the two flags the analyst raised in tasks.md's
scope note:

1. The built-in theme list (`opencode`, `system`, `tokyonight`, `everforest`,
   `ayu`, `catppuccin`, `catppuccin-macchiato`, `gruvbox`, `kanagawa`, `nord`,
   `matrix`, `one-dark` — OpenCode's own docs note more are added over time),
   used only as `mg theme`'s reference/picker list, never to reject a set.
2. Whether `opencode.json` accepts a top-level `"theme"` key: it used to, but
   OpenCode's config docs now say "Legacy theme, keybinds, and tui keys in
   opencode.json are deprecated and automatically migrated when possible" —
   the current, correct home for the theme is a **separate** `tui.json` file
   (`{"$schema": "https://opencode.ai/tui.json", "theme": "..."}`). This is a
   deliberate, researched deviation from TASK-2's literal wording (which
   assumed the theme would join `model` inside one `opencode.json` object) —
   the task's own risk note explicitly asked for this to be verified rather
   than assumed, since a wrong assumption would break config generation for
   every OpenCode container session.

## Changes

TASK-1 (`internal/config/config.go`, `internal/config/config_test.go`):
Added `Settings.Theme` (a plain string, `json:"-"` — never persisted to
`tui-settings.json`) and a `ThemeValue()` accessor (defaults to `""`, meaning
"let OpenCode use its own default/config"). `Load()` reads `OPENCODE_THEME`
from `manigot/.env` into `s.Theme`, mirroring `MANIGOT_PROFILE`; `Save()`
writes it back via `UpsertEnv` unconditionally (an explicit empty value
clears a previously-set theme, matching `EnvValue`'s documented semantics).
Added round-trip, default-value, and env-persistence tests mirroring the
existing Terminal field's coverage.

TASK-2 (`internal/session/session.go`, `internal/session/session_test.go`,
`scripts/entrypoint.sh`): Added `ProfileInfo.OpenCodeTheme`, populated in
`ResolveProfile` for every OpenCode-tool profile (including the legacy
profile-less `--tool opencode` path) independent of which credential key is
in use, since the theme is global rather than per-profile. `CheckAuth`
appends `-e OPENCODE_THEME=<value>` to `KeyEnv` when set. `entrypoint.sh`
gained a second, independent config-generation block (parallel to the
existing model block) that writes `~/.config/opencode/tui.json` via
`{env:OPENCODE_THEME}` substitution, gated on `OPENCODE_THEME` being set and
the file not already existing — the same gating shape the model block uses,
now with a comment explaining why theme and model land in two different
files. Added tests covering theme forwarding across all four OpenCode
profiles, the legacy path, omission when unset, and non-forwarding for
claude-pro.

TASK-3 (`cmd/mg/theme.go`, `cmd/mg/theme_test.go`, `cmd/mg/main.go`): New
`mg theme [name]` command, structurally mirroring `mg profiles`: no args
lists the current value plus the reference theme table (with a TTY picker,
reusing the `ui.Picker`/`pickerRunFunc` seam); a name argument writes
`OPENCODE_THEME` via `config.UpsertEnv` and prints a confirmation. Unlike
`mg profiles`, an unrecognized name is accepted — a one-line note is printed
pointing out it isn't in the reference list, but the value is still written
(OpenCode itself is the source of truth on valid names). Wired into
`main.go`'s dispatch switch and `printHelp`'s command listing.

TASK-4 (`internal/ui/settings.go`, `internal/ui/settings_test.go`): Added a
"Theme" field to the TUI settings screen as a free-text `textinput` (not a
fixed-list cycling selector like Profile, since theme names aren't
validated), appended after Terminal so existing `stFocus*` constants keep
their values — `stFieldCount` went from 6 to 7, and a new `stFocusTheme = 6`
was added. Updated `resize`/`update`/`setFocus`/`settingsValue`/`render`/
`hint` for the new field, and updated the tab-cycle tests (which pinned the
old field count/order) plus added seed/edit/trim/render tests mirroring the
Terminal field's coverage.

TASK-5 (`docs/AGENTS.md`, `project-template/docs/AGENTS.md`, `README.md`):
Documented the theme setting: a new paragraph in the "Session launch"
architecture section explaining the model-vs-theme config-file split and
*why* (the deprecated `opencode.json` theme key), a new `mg theme` bullet in
the `mg init`/`mg profiles`/.../`mg agents` section and the Commands list,
`OPENCODE_THEME` added to the `.env` key list in "Config files", a new
"Theme (OpenCode)" subsection in the README (setup instructions, mirroring
the Profiles section), a `mg theme` row in the installed-commands table, a
usage example, a new "Theme" bullet in the TUI Settings screen writeup, and
one sentence in `project-template/docs/AGENTS.md`'s vendor-neutral intro
noting the theme is a manigot-level setting, not a per-project one.

## Follow-up (post-verdict)

The `## Overall: NEEDS WORK` verdict raised two items; both are addressed:

1. **TASK-2's `mg host` env leak.** Fixed: `internal/session/host.go`'s
   `hostEnv` now excludes `OPENCODE_THEME` from the forwarded child
   environment, mirroring the existing `OPENCODE_MODEL` exclusion, with a new
   test (`TestBuildHostOpenCodeThemeNotForwarded`) asserting it — see commit
   `[prayer] TASK-2: exclude OPENCODE_THEME from mg host's forwarded env`.
2. **TASK-1's missing standalone commit.** Not fixable after the fact: TASK-1's
   `internal/config` changes are already merged into commit `9b43e97`
   ("chore: commit pre-existing analyst tasks.md"), and this session's git
   shim (see `docs/AGENTS.md`'s "Session git shim") disallows history-editing
   commands (`rebase`, `reset`, `commit --amend` is likewise against the
   project's own git discipline rules outside an explicit user request) — so
   the commit cannot be split retroactively without tooling this session
   isn't permitted to use. Recorded here per the verdict's own suggested
   resolution ("a note explaining why it can't be split further at this
   point"). The code itself is correct and covered by tests, per the
   verdict's TASK-1 review.

## Known issues / follow-ups

- `mg theme`'s picker does not offer a way to clear a previously-set theme
  back to "" (unset) — only `mg theme ""` (awkward on most shells) or hand-
  editing `.env`/the TUI's Theme field (which does support clearing, since
  it's free text) can do that. Not explicitly requested by tasks.md; flagged
  rather than added to avoid scope creep.
- The TUI settings screen's README write-up ("### Settings") only documented
  Editor and Profile before this job — Base branch, Job branch prefix,
  Recent activity, and Terminal were already undocumented there. Theme was
  added to keep parity with Profile (the field it's modeled after), but the
  pre-existing gap for the other fields was left alone as out of scope.
- All test runs in this sandboxed session hit a pre-existing, unrelated
  limitation: the session's own git shim (see docs/AGENTS.md's "Session git
  shim") blocks `git init`, which a large number of this repo's existing
  tests (job lifecycle, TUI list/detail, mg-jdi, etc. — none touched by this
  job) rely on for fixture setup. Those failures are present on this branch
  independent of this job's changes; every test actually exercising the new
  theme code (`internal/config`, the new `internal/session` theme tests,
  `cmd/mg/theme_test.go`, and `internal/ui`'s settings tests) passes.
