# Tasks: profiles must be configurable

id: somebody
status: open
analyst: architect
date: 2026-08-22

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Centralize the profile definition data model — extend `config.Profile` with `AuthKeys []string`, `ModelEnv string`, and `ModelDefault string`, and move the per-profile auth/model metadata currently scattered across `cmd/mg/profiles.go` (`profileAuthKeys`, `profileModelEnv`, `profileModelDefaults`) and the hardcoded switch in `internal/session/session.go` into `config` as part of the built-in `profiles` table.
     files: internal/config/config.go, internal/config/config_test.go, cmd/mg/profiles.go, internal/session/session.go
     depends: none
     risk: medium — touches the shared `Profile` struct that every profile consumer reads, but the fields are additive and existing callers keep working until the later tasks consume them.

TASK-2: Add a persisted user-defined profile store — a gitignored `config/profiles.json` in the manigot checkout (machine-specific, alongside `tui-settings.json`), with load/save, CRUD helpers (`AddProfile`/`RemoveProfile`), and merge-with-builtins in `Profiles()` and `ProfileByID()` so every existing caller (CLI list, session launcher, `mg jdi`, `mg init`, TUI settings screen) sees user profiles automatically.
     files: internal/config/config.go, internal/config/config_test.go, .gitignore
     depends: TASK-1
     risk: medium — new storage layer; must decide merge semantics (built-ins always present + user-defined appended, id uniqueness enforced) and how a missing/corrupt file degrades to "built-ins only".

TASK-3: Make `session.ResolveProfile` (and `CheckAuth`) data-driven — look up the profile's tool, auth keys, and model env/default from `config.ProfileByID` instead of the hardcoded 4-way switch, preserving the legacy `--tool opencode` path (empty-profile mode) and `claude-pro` subscription protection verbatim.
     files: internal/session/session.go, internal/session/session_test.go, internal/session/host_test.go
     depends: TASK-1
     risk: high — the core session-launch path is heavily test-pinned (every `KeyEnv`, model, and key-forwarding assertion); the refactor must produce byte-identical output for the 4 built-ins to keep those tests green.

TASK-4: Refactor `cmd/mg/profiles.go` listing/creds/model helpers to read from the profile data (TASK-1/TASK-2) instead of the hardcoded maps, so user-defined profiles render correctly (model column, creds column, picker rows).
     files: cmd/mg/profiles.go, cmd/mg/profiles_test.go
     depends: TASK-2
     risk: medium — existing tests pin the exact padded-column layout and search keys for the 4 built-ins; those must stay byte-identical while the source becomes data-driven.

TASK-5: Add profile creation/deletion to the CLI — extend `mg profiles` with `add`/`rm` subcommands (the natural home given `mg profiles` already manages the profile table; the brief's "a sort of mg settings command" is satisfied by this) that write to the user-profile store, prompting for tool, credential key(s), and model with the built-in defaults offered. Deleting the currently-set `MANIGOT_PROFILE` default must fall back to `claude-pro`; decide whether built-in profiles are deletable (recommended: user profiles freely, built-ins as defaults that "bring" — flag any choice in implementation.md).
     files: cmd/mg/profiles.go, cmd/mg/profiles_test.go, cmd/mg/main.go
     depends: TASK-2, TASK-4
     risk: medium — new command surface with interactive prompts and default-fallback edge cases; needs TTY/non-TTY handling consistent with the existing `mg profiles`/`mg setup` commands.

TASK-6: Refactor the `mg setup` wizard to be data-driven — iterate over all profiles (built-in + user-defined) instead of the four hardcoded wizard functions (`setupClaudePro`/`setupZAI`/`setupOpenCodeGo`/`setupOpenCodeZen`) and the hardcoded `--check` list, keeping the per-tool wording and the ntfy block.
     files: cmd/mg/setup.go, cmd/mg/setup_test.go
     depends: TASK-2
     risk: medium — the wizard's exact output wording and prompt ordering are pinned by many tests; must keep the 4 built-ins byte-identical while generalizing the loop.

TASK-7: Replace the remaining hardcoded profile enumerations in `cmd/mg/init.go` and `cmd/mg/jdi.go` (validation against the fixed four) with `config.ProfileByID`/`config.Profiles()`.
     files: cmd/mg/init.go, cmd/mg/init_test.go, cmd/mg/jdi.go, cmd/mg/jdi_test.go
     depends: TASK-2
     risk: low — mechanical swaps from hardcoded `switch`/enumerations to the central registry; existing error-message wording should be preserved.

TASK-8: Update docs and help text — `cmd/mg/main.go` `printHelp`, `AGENTS.md`, `docs/AGENTS.md`, and the README/setup help (`cmd/mg/setup.go` `setupHelp`) to describe profiles as user-configurable (create/delete via `mg profiles add/rm`, stored in `config/profiles.json`), and keep `agents/*.md` + `project-template/docs/AGENTS.md` in sync.
     files: cmd/mg/main.go, cmd/mg/profiles.go, cmd/mg/setup.go, AGENTS.md, docs/AGENTS.md
     depends: TASK-5, TASK-6
     risk: low — documentation-only, but must stay consistent with the implemented command surface.

TASK-9: Tests — add coverage for the new store (merge, CRUD, id-uniqueness, corrupt-file degrade) and data-driven resolution (a user-defined profile resolving to its tool/keys/model end to end through `ResolveProfile` → `CheckAuth` → `BuildDockerInvocation`/`BuildHostInvocation`), and update any existing test that hardcodes "exactly 4 profiles" now that the set is extensible.
     files: internal/config/config_test.go, internal/session/session_test.go, internal/session/docker_test.go, cmd/mg/profiles_test.go, cmd/mg/setup_test.go
     depends: TASK-2 through TASK-7
     risk: medium — cross-cutting; must not be written ahead of the features it pins.

## Design notes

- A profile is: { id, label, tool (claude-code | opencode), authKeys (the credential env vars it bills against), modelEnv (the .env key overriding the model), modelDefault }. The built-ins populate all fields from the current hardcoded tables; user profiles carry full definitions.
- Storage: `config/profiles.json` in the manigot checkout, gitignored (`.gitignore` already ignores `/config/`). Merge = built-ins always first (their order drives the TUI cycle + listing), then user-defined in file order; duplicate ids are rejected at write time.
- The TUI settings screen (`internal/ui/settings.go`) reads `config.Profiles()` already, so it picks up user profiles automatically once TASK-2 lands; the `profileOptions` package-level var is computed once at init, which is fine for a session-scoped process but should be noted if profiles are edited in-process.
- Forwarding contract to preserve: only opencode profiles forward provider keys (`OpenCodeKeys`) and a model (`OPENCODE_MODEL` → docker `-e` / host `--model`); claude-pro forwards the four `CLAUDE_*` keys and refuses `ANTHROPIC_API_KEY`. A user-defined opencode profile must work exactly like the built-in opencode ones through `CheckAuth` and both invocation builders.
- Keep `.env` keys `OPENCODE_*_MODEL` and `MANIGOT_PROFILE` semantics unchanged — existing users' `.env` files must keep working.
