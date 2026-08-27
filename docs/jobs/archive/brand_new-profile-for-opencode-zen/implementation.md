# Implementation: new profile for opencode zen

id: brand
status: open
developer: @developer
date: 2026-08-17

<!-- Produced by @developer after implementation. -->

## Summary

Added a fourth subscription profile `opencode-zen` — OpenCode CLI billed via
the existing `OPENCODE_API_KEY`, defaulting to the free model
`opencode/deepseek-v4-flash`, overridable via a new `OPENCODE_ZEN_MODEL`
.env key. The profile row was appended to the end of the profiles table, so
existing profile indices (1=claude-pro, 2=zai, 3=opencode-go) and the
default profile (claude-pro) are unchanged. All eight code tasks plus the
docs task are implemented and committed; `make mg` builds, `go test ./...`
passes, and the CLI surface (`mg profiles`, `mg setup --check`, `mg
--profile opencode-zen` resolution + auth) was verified live.

## Changes

TASK-1: Added `ProfileOpenCodeZen` const and the profiles-table row
`{ID: "opencode-zen", Label: "OpenCode · Zen", Tool: ToolOpenCode, Auth:
"OPENCODE_API_KEY"}` in `internal/config/config.go`; updated the doc
comments that enumerated the profiles. The TUI and launch package read
`config.Profiles()` dynamically, so they picked the new row up automatically.

TASK-2: Wired `opencode-zen` through `internal/session/session.go`:
the `--profile`/`MANIGOT_PROFILE` validation lists, the profile→tool/key/model
mapping (`OpenCodeKeys=["OPENCODE_API_KEY"]`, `OpenCodeModel =
envDefault("OPENCODE_ZEN_MODEL", "opencode/deepseek-v4-flash")`), the
error-message strings, and the `--print` legacy-tool hint. The key/model
mapping mirrors the opencode-go case exactly (shares `OPENCODE_API_KEY`).

TASK-3: Added `opencode-zen` to `cmd/mg/profiles.go`: `profileAuthKeys`,
`profileModelEnv` (`OPENCODE_ZEN_MODEL`), `profileModelDefaults`
(`opencode/deepseek-v4-flash`), the `profilesHelp` text, and the
`profilesSet` validation switch. The "Valid profiles:" error hint derives
from `profileIDs()` and updated automatically.

TASK-4: Added `opencode-zen` to `cmd/mg/setup.go`: `setupHelp` text, the
arg-validation and per-profile `--check` switches, the wizard dispatch
(single-profile and full walk), and a new `setupOpenCodeZen` wizard
mirroring `setupOpenCodeGo` (reuses `OPENCODE_API_KEY`, prompts
`OPENCODE_ZEN_MODEL` defaulting to `opencode/deepseek-v4-flash`).

TASK-5: Updated the CLI help/enumerations in `cmd/mg/main.go` (help profile
list + `--profile` usage line), `cmd/mg/jdi.go` (`--profile` flag help and
invalid-profile error string), and `cmd/mg/init.go` (`--profile` validation
error string).

TASK-6: Verified `scripts/entrypoint.sh` needs no change — `OPENCODE_API_KEY`
is already in `OPENCODE_KEY_VARS` and the `OPENCODE_MODEL` env plumbing is
profile-agnostic.

TASK-7: Updated pinned tests and added zen coverage: `session_test.go`
(valid-list error string; new `TestResolveProfileOpenCodeZenForwarding`,
`TestResolveProfileOpenCodeZenModelOverride`,
`TestResolveProfileOpenCodeZenMissingKey`; hermetic key list gains
`OPENCODE_ZEN_MODEL`), `docker_test.go` (`TestBuildOpenCodeZenProfile` argv
coverage), `host_test.go` (`TestBuildHostOpenCodeZenProfile`), `profiles_test.go`
(valid-profiles string; new `TestProfilesSetOpenCodeZen`), `setup_test.go`
(`--check-all` line; full-wizard prompt count 5→6; new
`TestSetupWizardOpenCodeZenWritesEnv`), `init_test.go` and `host_test.go`
(error strings), `internal/ui/settings_test.go` (4-profile cycle wrap and
render list).

TASK-8: Updated the user-facing docs: `README.md` (intro, profiles table,
new `opencode-zen` setup section, choosing-a-profile table, usage examples,
`mg jdi` and TUI settings mentions), `docs/AGENTS.md` (canonical — the root
`AGENTS.md` is a read-only mount of it: "four subscription profiles" intro,
.env key list gains `OPENCODE_ZEN_MODEL`, session-launch profile
resolution), and `project-template/docs/AGENTS.md` + `project-template/docs/CLAUDE.md`
(profile example lines). Archived docs/ROADMAP were left untouched as
historical records, per scope.

TASK-9: Verification — `make mg` builds, full `go test ./...` passes, `mg
profiles` lists opencode-zen with its model column and creds state, `mg
setup opencode-zen --check` reports ready/missing correctly, and `mg
--profile opencode-zen` resolves and reaches the auth check (correctly
reporting the missing `OPENCODE_API_KEY` when unset, forwarding it when
set).

## Known issues / follow-ups

- **The analyst's proposed default model id was wrong and was corrected.**
  tasks.md proposed `opencode-zen/deepseek-v4-flash` (assumption flagged for
  confirmation). Against the installed `opencode` (`opencode models`) there
  is no `opencode-zen/` prefix at all; the free DeepSeek V4 Flash model is
  `opencode/deepseek-v4-flash`. Confirmed live: `opencode run --model
  opencode-zen/deepseek-v4-flash` errors (unknown model), while
  `opencode/deepseek-v4-flash` responds with cost 0. All code, tests,
  and docs use the confirmed id; `OPENCODE_ZEN_MODEL` remains the override
  if the platform ever introduces a dedicated zen prefix.
- A real container session under `mg --profile opencode-zen` could not be
  run in this sandbox (no `docker` binary — same limitation recorded in the
  archived jobs). The docker argv shape is pinned by
  `TestBuildOpenCodeZenProfile`, and the model id was verified directly
  against the real `opencode` binary with the real API key.