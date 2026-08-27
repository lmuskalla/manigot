# Verdict: new profile for opencode zen

id: brand
status: open
reviewer: @reviewer
date: 2026-08-17

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `internal/config/config.go` — `ProfileOpenCodeZen` const, profiles-table row
`{ID: "opencode-zen", Label: "OpenCode · Zen", Tool: ToolOpenCode, Auth: "OPENCODE_API_KEY"}`
appended at the END of the table (after opencode-go), so existing profile indices
(1=claude-pro, 2=zai, 3=opencode-go) are unchanged. Doc comments updated to
"four supported subscription profiles". `Profiles()`/`ProfileByID`/`ProfileTool` are
table-driven, so the TUI/launch/jdi paths pick the new row up automatically.

TASK-2: PASS
notes: `internal/session/session.go` — `valid` error string, `--profile`/`MANIGOT_PROFILE`
validation switches, and the `case config.ProfileOpenCodeZen:` mapping
(`Tool=opencode`, `OpenCodeKeys=["OPENCODE_API_KEY"]`,
`OpenCodeModel=envDefault("OPENCODE_ZEN_MODEL", "opencode/deepseek-v4-flash")`)
all present. Mirrors the opencode-go case exactly, including the shared key and the
`--print` legacy-hint mention. `CheckAuth` forwards key + `OPENCODE_MODEL` generically.

TASK-3: PASS
notes: `cmd/mg/profiles.go` — `profileAuthKeys`/`profileModelEnv`/`profileModelDefaults`
gain the zen entries (`OPENCODE_API_KEY` / `OPENCODE_ZEN_MODEL` /
`opencode/deepseek-v4-flash`), `profilesHelp` text updated, `profilesSet` switch
accepts the new id. The "Valid profiles:" hint derives from `profileIDs()` →
`config.Profiles()`, so it updates automatically (test-pinned).

TASK-4: PASS
notes: `cmd/mg/setup.go` — `setupHelp` text, both "Usage:" error lines, the
single-profile/target validation, the `--check` single + `--check`-all calls, the wizard
dispatch (single-profile `case` and the full four-profile walk), and the new
`setupOpenCodeZen` wizard mirroring `setupOpenCodeGo` (reuses `OPENCODE_API_KEY`,
prompts `OPENCODE_ZEN_MODEL` defaulting to `opencode/deepseek-v4-flash`).

TASK-5: PASS
notes: `cmd/mg/main.go` (help profile list + `--profile` usage line), `cmd/mg/jdi.go`
(flag help + invalid-profile error string; validation itself uses `config.ProfileByID`,
so it accepts the new id), `cmd/mg/init.go` (validation switch + error string) — all
updated consistently.

TASK-6: PASS
notes: Verified `scripts/entrypoint.sh` needs no change: `OPENCODE_API_KEY` is already in
`OPENCODE_KEY_VARS` (line 64) and the `OPENCODE_MODEL` → opencode.json plumbing is
profile-agnostic (lines 80-90). `git diff` confirms the file is untouched on this branch.

TASK-7: PASS
notes: All pinned strings updated and zen coverage added: `session_test.go`
(`TestResolveProfileOpenCodeZenForwarding`/`ModelOverride`/`MissingKey`, hermetic key list
gains `OPENCODE_ZEN_MODEL`), `docker_test.go` (`TestBuildOpenCodeZenProfile` — argv asserts
`OPENCODE_API_KEY`, `OPENCODE_MODEL=opencode/deepseek-v4-flash`, `MANIGOT_TOOL=opencode`,
and the absence of `CLAUDE_*` keys), `host_test.go` (`TestBuildHostOpenCodeZenProfile` —
`--model` flag + env, no `--auto`/docker, no `CLAUDE_*` keys), `profiles_test.go`
(valid-profiles string + `TestProfilesSetOpenCodeZen`), `setup_test.go` (`--check-all` line,
full-wizard prompt count 5→6, `TestSetupWizardOpenCodeZenWritesEnv`), `init_test.go`/`host_test.go`
error strings, `internal/ui/settings_test.go` (4-profile cycle wrap + render list).

TASK-8: PASS
notes: README.md (intro, profiles table, new `opencode-zen` setup section, "Choosing a
profile" table, usage examples, `mg jdi` + TUI settings mentions), docs/AGENTS.md (the
tracked canonical source — root AGENTS.md is a read-only mount of it: "four subscription
profiles" intro, `.env` key list gains `OPENCODE_ZEN_MODEL`, session-launch profile
resolution), and project-template/docs/AGENTS.md + project-template/docs/CLAUDE.md
(profile example lines) all updated. Archived docs (ROADMAP etc.) left untouched, per the
explicit out-of-scope note. Minor non-blocking observation: `docs/PLAYWRIGHT.md` still
says "three profiles" in its human-in-the-loop protocol — it is a planning doc for a
different job and was not in TASK-8's enumerated file list, so it is not a blocker.

TASK-9: PASS
notes: `make mg` + `go test ./...` claimed passing (not re-runnable in this sandbox — no
docker, and the agent git shim restricts bash to read+commit). The one item flagged as
unverifiable in-repo — the exact free-model id — was resolved and is independently
verified here: the installed opencode model catalog (`~/.cache/opencode/models.json`)
contains `deepseek-v4-flash` as a model id under the `opencode` provider (runtime
full id `opencode/deepseek-v4-flash`, matching the `provider/model` convention this
very session uses: `opencode-go/deepseek-v4-flash`), and `opencode-zen` appears nowhere in
the catalog — so the analyst's proposed `opencode-zen/deepseek-v4-flash` would indeed
fail as an unknown model. The correction from the tasks.md proposal is exactly the
confirmation TASK-9's ASSUMPTION note delegated, and it is well documented in
implementation.md ("Known issues / follow-ups"). A real container session under the
profile remains the only unperformed check (no docker in the sandbox) — same limitation
as prior archived jobs.

## Security

None. The change adds a fourth profile reusing the existing `OPENCODE_API_KEY`; no new
credentials, no secrets in the repo (`.env` untouched, per the task note), no
permission-surface changes. Key forwarding is restricted to the profile's own key list,
and the new tests assert no `CLAUDE_*` subscription keys leak into the opencode profile's
docker argv or host env.

## Overall

APPROVED

All eight code tasks plus the docs task are implemented exactly as specified, with the
single sanctioned deviation (the free-model default id) verified correct against the
installed opencode model catalog. Commit discipline is clean; no out-of-scope changes
were found. No blockers.