## Summary

Made manigot's profiles user-configurable. Previously the four subscription
profiles (`claude-pro`, `zai`, `opencode-go`, `opencode-zen`) were hardcoded
across the codebase — their auth keys, model env keys, and model defaults were
scattered across `cmd/mg/profiles.go` maps and a hardcoded 4-way switch in
`internal/session/session.go`. Now a profile is a complete data definition in
`internal/config` (id, label, tool, auth keys, model env/default), the built-ins
are a compiled-in table, and users can add/remove their own profiles via
`mg profiles add` / `mg profiles rm`, persisted in the gitignored
`config/profiles.json`. Every consumer — the CLI listing, the session launcher
(docker + host), `mg setup`, `mg init`, `mg jdi`, and the TUI — reads the merged
built-in + user set through `config.Profiles()`/`ProfileByID()`, so a user-defined
profile behaves exactly like a built-in.

## Changes

TASK-1: Extended `config.Profile` with `AuthKeys []string`, `ModelEnv string`,
`ModelDefault string`, added the metadata to the built-in `profiles` table
(now `builtInProfiles`) in `internal/config/config.go`, and pinned it in
`config_test.go`. The auth/model metadata previously living in
`cmd/mg/profiles.go` and `internal/session/session.go` is now centralized in
config.

TASK-2: Added the persisted user-defined profile store — gitignored
`config/profiles.json` in the checkout (covered by the existing `/config/`
ignore rule) — with `AddProfile`/`RemoveProfile` and `loadUserProfiles`/
`saveUserProfiles`. `Profiles()`/`ProfileByID()` merge built-ins first then user
profiles in file order; a missing/corrupt file degrades to "built-ins only" for
the read-only paths, while the write paths surface a corrupt store as an error
rather than clobbering it. Duplicate ids (including collisions with built-ins)
are rejected at write time.

TASK-3: Made `session.ResolveProfile` and `CheckAuth` data-driven — tool, auth
keys, and model env/default now come from `config.ProfileByID` instead of the
4-way switch, and `ProfileInfo` gained an `AuthKeys` field. The legacy
profile-less `--tool opencode` path and the `claude-pro` subscription protection
(requires `CLAUDE_CODE_OAUTH_TOKEN`, refuses `ANTHROPIC_API_KEY`) are preserved
verbatim. The error-message "valid profiles" list is now derived from
`config.Profiles()` via `profileValidList()`.

TASK-4: Removed the hardcoded `profileAuthKeys`/`profileModelEnv`/
`profileModelDefaults` maps from `cmd/mg/profiles.go`; the `profileModel`/`creds`/
`confirmSet` helpers and `profilesSet` validation now read from
`config.ProfileByID`. `mg setup`'s `checkProfile` was made data-driven too (it
shared the removed map).

TASK-5: Added `mg profiles add [<id>]` (interactive wizard prompting tool,
credential key(s), model env/default with built-in defaults offered) and
`mg profiles rm <id>` (removes a user profile; removing the current default
falls back to `claude-pro`). Updated `profilesHelp`, `main.go` help, and
`profiles_test.go`.

TASK-6: Made the `mg setup` wizard data-driven — `--check` and the wizard loop
iterate `config.Profiles()` instead of the hardcoded four. The three opencode
wizard functions became one generic `setupOpenCodeProfile` driven by profile data
+ a per-profile prose spec (`setupSpecs`), reproducing the built-in opencode
wizards byte-for-byte; a user-defined opencode profile falls back to a generic
spec. The ntfy block is unchanged.

TASK-7: Replaced the hardcoded `--profile` validation in `cmd/mg/init.go` with
`config.ProfileByID`. `mg jdi` already validated via `ProfileByID` (data-driven
from TASK-2), so no functional change was needed there.

TASK-8: Updated docs/help — `main.go` `printHelp`, `profilesHelp`,
`setupHelp`, and `docs/AGENTS.md` (intro, the `mg profiles`/`mg setup`
descriptions, the `internal/config` bullet, the Commands list, and a new
`config/profiles.json` entry in the Config files section) now describe profiles
as user-configurable. `agents/*.md` and `project-template/docs/AGENTS.md` needed
no change (their profile mentions are example switches, not profile management).

TASK-9: Added tests: store merge/CRUD/id-uniqueness/corrupt-degrade
(`config_test.go`), end-to-end user-defined-profile resolution through
`ResolveProfile` → `CheckAuth` → `BuildDockerInvocation`/`BuildHostInvocation`
(`session_test.go`/`docker_test.go`/`host_test.go`), and the CLI add/rm + setup
wizard user-profile paths. No existing test hardcodes a count that breaks now
that the set is extensible.

## Known issues / follow-ups

- **Built-in profiles are not deletable.** Per the task recommendation,
  `mg profiles rm` only removes user-defined profiles; built-ins are the
  defaults every user starts from.
- **The claude-code setup flow remains bespoke.** The `mg setup` opencode
  wizards are fully data-driven; the claude flow (`setupClaudePro`) is kept as
  a dedicated claude-tool wizard because the Claude OAuth account structure
  (token + account UUID/email/org UUID from `~/.claude.json`) is unique. The
  brief's use cases are all opencode profiles; a user-defined claude profile
  would route through the claude-pro flow.
- **Root `AGENTS.md` left untouched.** `/workspace/AGENTS.md` is a gitignored
  read-only context mount; per the hard rules I edited the canonical tracked
  source `docs/AGENTS.md` (they are byte-identical).
- **Sandbox git shim.** The agent session's git shim refuses `git init`/
  `git worktree`, which the git-dependent tests (`internal/session`'s
  root/docker/host suites) use for temp-repo setup. Those tests fail under the
  shim regardless of this job's changes; I verified them by running the suite
  with the real git on `PATH` (`PATH=/usr/bin:/bin go test ./...`) — all pass.
  Not a code issue, but worth knowing when running the suite in a container.
