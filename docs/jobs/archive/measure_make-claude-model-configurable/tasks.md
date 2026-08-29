# Tasks: make claude model configurable

id: measure
status: open
analyst: @analyst
date: 2026-08-29

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Generalize `config.Profile`'s `ModelEnv`/`ModelDefault` fields so they are meaningful for `claude-code`-tool profiles too, not just `opencode` ones — update the doc comments accordingly, and change the built-in `claude-pro` profile's `ModelDefault` from the display-only sentinel string `"(Claude Code default)"` to `""` (empty), since that string must not be forwarded to the CLI as a literal `--model` value once claude profiles start actually using `ModelDefault`. The "(Claude Code default)" wording moves to display-only code in TASK-5.
     files: internal/config/config.go
     depends: none
     risk: medium — `ModelDefault` for claude-pro is a test-pinned sentinel (internal/config/config_test.go, cmd/mg/profiles_test.go); changing its value is a deliberate, visible behavior change that must land together with TASK-5's display fix and the test updates in TASK-7, or the profile table will show a blank model column for claude-pro in between.

TASK-2: Make `session.ResolveProfile` populate a new `ProfileInfo.ClaudeModel` field for `claude-code`-tool profiles, mirroring how `OpenCodeModel` is populated for opencode ones (`envDefault(p.ModelEnv, p.ModelDefault)`) — empty when the profile defines neither, which is what keeps `claude-pro` behavior unchanged (no `.env` key, no default, so `ClaudeModel` resolves to `""`).
     files: internal/session/session.go, internal/session/session_test.go
     depends: TASK-1
     risk: low — additive field, resolution logic mirrors the existing opencode branch exactly.

TASK-3: Forward the resolved model to Claude Code as a CLI flag rather than an environment variable, since `scripts/entrypoint.sh` execs `claude` directly with the docker CMD args (unlike OpenCode's config-file approach) — in `internal/session/docker.go`, append `--model <ClaudeModel>` to the container command's argv (alongside the existing `agentFlag`/`promptArgs` construction) when `info.Tool == config.ToolClaudeCode && info.ClaudeModel != ""`. No `scripts/entrypoint.sh` change is needed: the flag rides through `"$@"` to the `exec claude --dangerously-skip-permissions "$@"` line already in place. Confirm `claude --model <value>` is in fact accepted by the installed Claude Code CLI version (e.g. `claude --help`) before relying on it — flag any surprise in implementation.md.
     files: internal/session/docker.go, internal/session/docker_test.go
     depends: TASK-2
     risk: medium — first real use of an argv-flag path for Claude Code specifically (today only opencode's host path does this); needs a test asserting the flag lands in the right position relative to `agentFlag`/`opts.Pass`, and needs the CLI-flag assumption verified against the actual installed `claude` binary.

TASK-4: Apply the same `--model` forwarding to `mg host`'s direct invocation — in `internal/session/host.go`, extend the existing `modelArgs` construction (currently `info.Tool == config.ToolOpenCode && info.OpenCodeModel != ""`) to also cover `info.Tool == config.ToolClaudeCode && info.ClaudeModel != ""`.
     files: internal/session/host.go, internal/session/host_test.go
     depends: TASK-2
     risk: low — same flag, same call site, opencode's branch is the direct template.

TASK-5: Extend `mg profiles add`'s interactive wizard (`cmd/mg/profiles.go`) to prompt for a model when the chosen tool is `claude-code`, mirroring the existing opencode prompt (`Model .env key`, `Default model`) but optional rather than required — leaving both blank must still produce a valid profile (matching `claude-pro`'s own no-model-override shape after TASK-1). Update `profileModel()` (the table/picker's model column) so it shows `"(Claude Code default)"` when the effective resolved value is empty and the profile's tool is `claude-code`, instead of reading that string out of `ModelDefault` directly.
     files: cmd/mg/profiles.go, cmd/mg/profiles_test.go
     depends: TASK-1
     risk: medium — changes a user-facing, test-pinned table format (padded columns) and the add-wizard's prompt flow; must keep the opencode path's required-model behavior byte-identical.

TASK-6: Give `mg setup <profile>` a way to set/override a `claude-code`-tool profile's model — today `setupProfile` routes every claude-tool profile (built-in `claude-pro` and any user-defined one) through the bespoke `setupClaudePro`, which ignores the passed-in `config.Profile` entirely and only ever prompts the four fixed `CLAUDE_*` OAuth keys. Add an optional model prompt (using `p.ModelEnv`/`p.ModelDefault`, skipped entirely when the profile defines neither — i.e. unchanged for `claude-pro`) analogous to `setupOpenCodeProfile`'s model block.
     files: cmd/mg/setup.go, cmd/mg/setup_test.go
     depends: TASK-1
     risk: medium — `setupClaudePro`'s exact wording/prompt order is heavily test-pinned; must add the new block without disturbing `claude-pro`'s existing (model-less) output, and must decide where in the wizard's flow the new prompt goes.

TASK-7: Update/add tests covering the new behavior end to end: `internal/config/config_test.go` (claude-pro's `ModelDefault` is now `""`), `internal/session/session_test.go` (`ClaudeModel` resolution for a user-defined claude profile, plus a no-model case), `internal/session/docker_test.go` and `internal/session/host_test.go` (`--model` argv forwarding for claude-tool profiles), `cmd/mg/profiles_test.go` (add-wizard's new optional model prompt, table/picker display for both "model set" and "model unset" claude profiles), `cmd/mg/setup_test.go` (new model prompt block). Also grep the whole repo for the literal string `"(Claude Code default)"` to make sure every occurrence — code and tests — moves to the new display-only path from TASK-5.
     files: internal/config/config_test.go, internal/session/session_test.go, internal/session/docker_test.go, internal/session/host_test.go, cmd/mg/profiles_test.go, cmd/mg/setup_test.go
     depends: TASK-1 through TASK-6
     risk: medium — cross-cutting; must not be written ahead of the behavior it pins, and must not weaken any existing assertion in the process of updating it.

TASK-8: Update documentation to describe the new capability — `README.md`'s profile section (only if it currently implies model configuration is opencode-only) and any of `agents/*.md` / `docs/AGENTS.md` / `AGENTS.md` / `project-template/docs/AGENTS.md` that describe `mg profiles add`'s model prompt as opencode-specific. Keep this task's scope to prose that is actually wrong after TASK-5/TASK-6 land — do not add new sections that weren't already there for opencode.
     files: README.md, docs/AGENTS.md, AGENTS.md, project-template/docs/AGENTS.md, agents/*.md
     depends: TASK-5, TASK-6
     risk: low — documentation-only, but must stay consistent with the implemented command surface; the developer should grep for "opencode profiles" / "Only meaningful for opencode" wording specifically before editing to avoid a broad unrelated rewrite.

## Design notes

- The core fix is *not* a new mechanism — `config.Profile.ModelEnv`/`ModelDefault` already exist and are wired all the way through `ResolveProfile` for `opencode`-tool profiles; the gap is purely that the `claude-code` branch of that same logic (session resolution, the docker/host argv builders, the `mg profiles add` wizard, and `mg setup`) never consults them. TASK-2 through TASK-6 close that gap by mirroring the existing opencode code paths.
- Forwarding mechanism differs deliberately from opencode's: opencode's model rides through `OPENCODE_MODEL` and a generated config file inside the container (`scripts/entrypoint.sh`) because OpenCode has no model CLI flag usable that way; Claude Code's `entrypoint.sh` branch already execs `claude` with the passthrough args, so the natural mechanism is an argv `--model <value>` flag (exactly what `mg host` already does for opencode's own host path in `host.go`). No change to `scripts/entrypoint.sh` is required by this job.
- The one real trap: `claude-pro`'s `ModelDefault` field currently holds `"(Claude Code default)"` as a *display-only* placeholder consumed by `cmd/mg/profiles.go`'s `profileModel()` — it was never meant to be forwarded to the CLI. Once claude-tool profiles start actually consuming `ModelDefault` as a real override (TASK-2), that sentinel must move out of the data field and into `profileModel()`'s display logic (TASK-1 + TASK-5), or a fresh claude-pro session would try to launch `claude --model "(Claude Code default)"`.
- Scope boundary: this job is about the model field specifically. `setupClaudePro` also ignores a user-defined claude profile's custom `AuthKeys` (it always prompts the four fixed `CLAUDE_*` keys, assuming every claude-tool profile shares the same OAuth subscription), which is a separate, pre-existing gap — out of scope here unless a task above turns out to require touching it incidentally.
- `claude --model <value>`'s exact accepted value shapes (`sonnet`/`opus`/`haiku` aliases vs. full model ids like `claude-sonnet-4-5-20250929`) should be verified by the developer against the installed CLI (`claude --help` / `claude --model bogus` behavior) rather than assumed — TASK-3 flags this explicitly.
