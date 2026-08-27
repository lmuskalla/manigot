# Tasks: opencode themes

id: prayer
status: open
analyst: claude
date: 2026-08-27

<!-- Produced by @analyst from brief.md. -->

## Scope note

The brief asks for "a global setting for mg to set the theme from the
available ones" for OpenCode (https://opencode.ai/docs/themes/); Claude Code
already respects the host's terminal theme, so it needs no change. This is a
**global** setting — one value shared across every OpenCode profile
(zai/opencode-go/opencode-zen/opencode-zen-free), unlike the per-profile
`OPENCODE_*_MODEL` keys, matching the brief's own wording ("a global
setting", not "a per-profile setting").

Two things below are flagged rather than guessed, per "ask, don't guess":

1. **The exact list of OpenCode's built-in theme names** — manigot has no
   network access at analysis time to confirm it against
   https://opencode.ai/docs/themes/. TASK-3 must fetch/verify the current
   list (developer sessions run with `--network=bridge`) before hardcoding
   anything, and the chosen value should not be strictly validated against
   that list (OpenCode may add themes manigot doesn't know about yet — let
   OpenCode itself reject an invalid name at runtime).
2. **Whether OpenCode's `opencode.json` genuinely accepts a top-level
   `"theme"` key** the same way it accepts `"model"` — TASK-2 must confirm
   this against the docs link (or OpenCode's own schema) before changing
   `scripts/entrypoint.sh`, since a wrong assumption there breaks config
   generation for every OpenCode container session (theme AND model).

`mg host` is explicitly out of scope: host-mode OpenCode runs the user's own
host installation with its own real `~/.config/opencode/opencode.json`,
which already reflects whatever theme the user configured there — the same
reason Claude Code needs no change. No host.go edits are expected.

## Task breakdown

TASK-1: Add a global `Theme` setting to `internal/config`: an `OPENCODE_THEME` key in manigot's `.env` (read/written exactly like `Settings.Profile`/`MANIGOT_PROFILE` — via `config.EnvValue`/`config.UpsertEnv`, not `tui-settings.json`), plus a `Settings.Theme` field and a `ThemeValue()` accessor that defaults to `""` (meaning "let OpenCode use its own default/config").
     files: internal/config/config.go, internal/config/config_test.go
     depends: none
     risk: low — additive field mirroring the existing Profile persistence pattern closely; no behavior changes for anyone who doesn't set it.

TASK-2: Forward the configured theme into OpenCode container sessions and apply it. Extend `ProfileInfo`/`CheckAuth` in `internal/session/session.go` to append `-e OPENCODE_THEME=<value>` to `KeyEnv` when the resolved tool is OpenCode and a theme is configured (independent of which profile/API key is in use). Update `scripts/entrypoint.sh`'s `opencode.json` generation so it composes a config object from *whichever* of `OPENCODE_MODEL`/`OPENCODE_THEME` are set (today it only ever writes `model`, and only when the file doesn't already exist and `OPENCODE_MODEL` is set — that gating condition must become "either is set"), each substituted via `{env:...}` exactly like `model` is today.
     files: internal/session/session.go, internal/session/session_test.go, internal/session/docker_test.go, scripts/entrypoint.sh
     depends: TASK-1
     risk: high — scripts/entrypoint.sh is the project's only bash file and must stay self-contained; its `opencode.json` write path is shared by every OpenCode session (all four profiles), so a mistake here breaks model selection for everyone, not just theme users. Also carries the "confirm the theme key's real schema shape" flag from the scope note above.

TASK-3: Add a way for the user to set the theme. Introduce `mg theme [name]` (new `cmd/mg/theme.go`, dispatched from `cmd/mg/main.go`'s switch) mirroring `mg profiles`' shape: no args lists the setting (current value, plus a reference list of OpenCode's known built-in theme names fetched/verified from https://opencode.ai/docs/themes/ at implementation time — do not guess the list), with a TTY interactive picker (reusing the existing `ui.Picker`/`pickerRunFunc` seam `mg profiles` uses); a name argument writes `OPENCODE_THEME` via `config.UpsertEnv` and prints a confirmation, matching `mg profiles`' `confirmSet` style. Unlike profile IDs, an unrecognized name is still accepted (see scope note) — only used for the picker's row list, never for rejecting a set.
     files: cmd/mg/theme.go, cmd/mg/theme_test.go, cmd/mg/main.go, cmd/mg/main help text (printHelp)
     depends: TASK-1
     risk: medium — new user-facing command surface plus the unverified theme-list research called out in the scope note.

TASK-4: Add a "Theme" field to the TUI settings screen (`internal/ui/settings.go`), as a free-text input alongside Editor/Terminal (not a fixed-list cycling selector like Profile, consistent with TASK-3 not hard-validating the name), wired to `config.Save`/`config.Load` via the new `Settings.Theme` field from TASK-1. Requires extending `stFieldCount` and the `stFocus*` tab-cycle constants/indices without disturbing the existing field order, and updating the corresponding `_test.go` coverage.
     files: internal/ui/settings.go, internal/ui/settings_test.go
     depends: TASK-1
     risk: medium — touches the settings form's tab-cycle indices, which several existing tests pin exactly; an off-by-one breaks focus navigation for every existing field, not just the new one.

TASK-5: Update documentation to describe the new setting: the "Session launch" architecture section's `opencode.json` description (model → model+theme), the "Config files" section's `.env` key list (add `OPENCODE_THEME`), and `mg theme`'s entry in the Commands list — in both `docs/AGENTS.md` (this project's own copy, which `.claude/CLAUDE.md`/`.opencode/AGENTS.md` mount read-only from) and `project-template/docs/AGENTS.md` (per the existing "keep in sync" rule for docs describing this system), plus `README.md`'s corresponding `.env`/config section.
     files: docs/AGENTS.md, project-template/docs/AGENTS.md, README.md
     depends: TASK-2, TASK-3, TASK-4 (documents their outcome, including whatever the TASK-2/TASK-3 research settles on)
     risk: low — documentation only, but must stay accurate to whatever TASK-2/3 actually implement (e.g. don't document a `"theme"` opencode.json key if TASK-2's research finds OpenCode expects something else).
