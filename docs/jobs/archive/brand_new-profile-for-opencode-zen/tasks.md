# Tasks: new profile for opencode zen

id: brand
status: open
analyst: @analyst
date: 2026-08-17

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

Scope: add a fourth subscription profile `opencode-zen` — OpenCode CLI
(ToolOpenCode) billed via the existing `OPENCODE_API_KEY` (README already
states "the same key works for Zen"), defaulting to the free model
`opencode-zen/deepseek-v4-flash`, overridable via a new `OPENCODE_ZEN_MODEL`
.env key. Follows the `opencode-go` precedent exactly; appended to the end of
the profiles table so existing profile indices (1=claude-pro, 2=zai,
3=opencode-go) are unchanged. The default profile stays claude-pro; no change
to `scripts/entrypoint.sh` is expected (OPENCODE_API_KEY is already in
OPENCODE_KEY_VARS — verify only).

ASSUMPTION to confirm at implementation: the exact model id for "DeepSeek V4
Flash Free" is unverifiable from this repo. Proposal: `opencode-zen/deepseek-v4-flash`
(README's documented convention: "Go model ids use the opencode-go/ prefix
(e.g. opencode-go/deepseek-v4-flash)"). It is configurable via
`OPENCODE_ZEN_MODEL` anyway, but the developer should confirm the real id
against the installed `opencode` (e.g. `opencode models`) before finalizing,
since a wrong default breaks zen sessions until overridden.

TASK-1: Add the `opencode-zen` profile constant and its entry in the profiles table in the config package.
     files: internal/config/config.go (ProfileOpenCodeZen const; profiles table row {ID, Label: "OpenCode · Zen", Tool: ToolOpenCode, Auth: "OPENCODE_API_KEY"}; doc comments mentioning "three profiles")
     depends: none
     risk: low — purely additive; no existing behavior changes. The TUI (internal/ui/settings.go) and launch package read config.Profiles() dynamically, so they pick the new row up automatically.

TASK-2: Wire `opencode-zen` through session profile resolution and auth: ResolveProfile's --profile/MANIGOT_PROFILE validation lists and the profile→tool/key/model mapping (OpenCodeKeys=["OPENCODE_API_KEY"], OpenCodeModel=envDefault("OPENCODE_ZEN_MODEL", "opencode-zen/deepseek-v4-flash")); update ProfileInfo/Profile doc comments.
     files: internal/session/session.go
     depends: TASK-1
     risk: medium — the routing core; error-message strings ("claude-pro|zai|opencode-go") are pinned by tests and must gain the new id consistently, and the key/model mapping must mirror the opencode-go case exactly.

TASK-3: Add `opencode-zen` to `mg profiles`: profileAuthKeys (OPENCODE_API_KEY), profileModelEnv (OPENCODE_ZEN_MODEL), profileModelDefaults (opencode-zen/deepseek-v4-flash), and the profilesHelp text list. The "Valid profiles:" error hint derives from profileIDs() and updates automatically.
     files: cmd/mg/profiles.go
     depends: TASK-1
     risk: low — additive map entries plus one prose edit; listing/select/creds logic is generic over the table.

TASK-4: Add `opencode-zen` to `mg setup`: setupHelp text, the arg-validation and per-profile --check switches (both "check one" and "check all"), the wizard dispatch (single-profile and full walk), and a new setupOpenCodeZen wizard mirroring setupOpenCodeGo (reuses OPENCODE_API_KEY, prompts OPENCODE_ZEN_MODEL default opencode-zen/deepseek-v4-flash). The "Usage:" error line in runSetup also gains the new id.
     files: cmd/mg/setup.go
     depends: TASK-1
     risk: medium — new wizard function plus several switch/usage strings; the full-wizard test's prompt count changes (one more model prompt) and the --check-all output gains a row.

TASK-5: Update the CLI help text and inline profile enumerations: main.go help profile list + `mg --profile <name>` usage line; jdi.go --profile flag help and its invalid-profile error string; init.go --profile validation error string.
     files: cmd/mg/main.go, cmd/mg/jdi.go, cmd/mg/init.go
     depends: TASK-1
     risk: low — string-only edits; each is pinned by a test (see TASK-7) so the changes are mechanical and checked.

TASK-6: Verify scripts/entrypoint.sh needs no change: OPENCODE_API_KEY is already in OPENCODE_KEY_VARS, so a zen session boots; the OPENCODE_MODEL env plumbing is profile-agnostic. If (and only if) the free model turns out to need NO provider key at all, reconsider — but the brief says the existing OpenCode key is already in .env, and CheckAuth requires the profile's key, so the expectation is "no change".
     files: scripts/entrypoint.sh (read-only verification; no edit expected)
     depends: TASK-2
     risk: low — verification only; the container-side key list is explicitly a safety net behind Go's pre-validation, so drift is harmless by design.

TASK-7: Update tests that pin the profile enumeration/cycle and add opencode-zen coverage: session_test.go (valid-list error string; add a zen resolve test asserting OpenCodeKeys/OPENCODE_ZEN_MODEL/KeyEnv), docker_test.go/host_test.go (zen argv/key-forwarding coverage), profiles_test.go ("Valid profiles: claude-pro zai opencode-go" string; possibly a zen-set test), setup_test.go (--check-all expected lines; full-wizard prompt count 5→6 and its pre-configured .env), init_test.go and host_test.go (error strings), internal/ui/settings_test.go (profile-cycle wrap expectation — with 4 profiles three rights from claude-pro no longer wraps to claude-pro; update to a 4-profile expectation).
     files: internal/session/session_test.go, internal/session/docker_test.go, internal/session/host_test.go, cmd/mg/profiles_test.go, cmd/mg/setup_test.go, cmd/mg/init_test.go, cmd/mg/host_test.go, internal/ui/settings_test.go
     depends: TASK-2 through TASK-5
     risk: medium — several pinned strings and one behavioral expectation (the settings cycle wrap); all mechanical once the strings settle, but easy to miss one (grep for "opencode-go|zai|claude-pro" in *_test.go as a checklist).

TASK-8: Update the user-facing docs so they describe the fourth profile consistently: README.md (profiles table, per-profile setup sections — add an `opencode-zen` section, "Choosing a profile" table rows, `mg jdi` and TUI settings mentions of the profile list), AGENTS.md and docs/AGENTS.md (kept in sync with each other: "three subscription profiles" intro, .env config-files key list — add OPENCODE_ZEN_MODEL — session-launch profile resolution), and project-template/docs/AGENTS.md + project-template/docs/CLAUDE.md (profile example lines). Out of scope: archived briefs/ROADMAP/CONSOLIDATION-BRIEF.md — historical records.
     files: README.md, AGENTS.md, docs/AGENTS.md, project-template/docs/AGENTS.md, project-template/docs/CLAUDE.md
     depends: TASK-1 (naming, key, model settled)
     risk: low — prose/table updates; the AGENTS.md hard rule requires root AGENTS.md, docs/AGENTS.md and the project-template copies to stay in sync.

TASK-9: Verification: `make mg` builds; full `go test ./...` passes; `mg profiles` lists opencode-zen with its model column and creds state; `mg setup opencode-zen --check` reports ready/missing correctly; `mg --profile opencode-zen` reaches the session launcher's auth check (confirms resolution + key forwarding); ideally one real container session under the profile to confirm the model id boots (see the ASSUMPTION above — the only part not fully verifiable in-repo).
     files: none (verification)
     depends: TASK-1 through TASK-8
     risk: medium — the model-id assumption is the one item that can only be truly confirmed against the real opencode install; everything else is unit-testable.

Notes for the developer:
- Keep the new profile row at the END of config.profiles (after opencode-go): `mg profiles` interactive indices and the shared TUI/CLI MANIGOT_PROFILE contract stay stable; only the cycle-wrap test expectation in internal/ui/settings_test.go needs rework.
- The profile shares OPENCODE_API_KEY with opencode-go by design — CheckAuth forwards the key if set; both profiles coexisting is fine (the multi-provider loop forwards exactly the profile's own key list).
- Do NOT touch the host's .env from within this job; `mg setup` writes it at runtime, and no credential belongs in the repo.
