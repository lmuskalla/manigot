# Implementation: make claude model configurable

id: measure
developer: @developer
date: 2026-08-29

## Summary

Claude Code profiles can now be given a model override the same way OpenCode
profiles already could. `config.Profile.ModelEnv`/`ModelDefault` are no
longer opencode-only: `session.ResolveProfile` resolves a new
`ProfileInfo.ClaudeModel` field for claude-code profiles (mirroring
`OpenCodeModel`), and both the docker session path and `mg host` forward it
to the `claude` CLI as a real `--model <value>` argv flag (verified accepted
by the installed CLI via `claude --help`). `mg profiles add` now offers an
optional model prompt for claude-code profiles, and `mg setup <profile>`
prompts for it too. `claude-pro`'s `ModelDefault` — previously the
display-only sentinel string `"(Claude Code default)"` — is now empty, with
that sentinel moved into `cmd/mg/profiles.go`'s `profileModel()` display
logic, so `claude-pro`'s own behavior (no override, CLI's own default) is
unchanged.

## Changes

TASK-1: `internal/config/config.go` — generalized the `ModelEnv`/
`ModelDefault` doc comments to cover claude-code profiles, and changed the
built-in `claude-pro` profile's `ModelDefault` from `"(Claude Code default)"`
to unset (no `ModelEnv`/`ModelDefault` at all), since that sentinel string
must never be forwarded to the CLI as a literal `--model` value.

TASK-2: `internal/session/session.go` — added `ProfileInfo.ClaudeModel`,
populated in `ResolveProfile` for `claude-code`-tool profiles via
`envDefault(p.ModelEnv, p.ModelDefault)`, mirroring the existing
`OpenCodeModel` resolution. Added `session_test.go` coverage: a user-defined
claude profile with an env override, one falling back to its stored default,
and `claude-pro` resolving to `""`.

TASK-3: `internal/session/docker.go` — `BuildDockerInvocation` now appends
`--model <ClaudeModel>` to the container command's argv (after `--agent`,
before the prompt/passthrough args) when the resolved profile is
`claude-code` with a non-empty `ClaudeModel`. Verified `claude --help` lists
`--model` as an accepted flag on the installed CLI. Added `docker_test.go`
coverage for both the flag-present and no-model (`claude-pro`) cases,
including flag position relative to `--agent`.

TASK-4: `internal/session/host.go` — extended `BuildHostInvocation`'s
`modelArgs` construction to also cover `claude-code` profiles with a
non-empty `ClaudeModel`, using the exact same `--model` flag mechanism
already used for opencode's host path. Added `host_test.go` coverage
mirroring the existing opencode model tests.

TASK-5: `cmd/mg/profiles.go` — the `mg profiles add` wizard now prompts for
a model `.env` key and default when the chosen tool is `claude-code`,
mirroring the opencode prompt but optional (leaving both blank is valid,
matching `claude-pro`'s own shape). `profileModel()` (the table/picker's
model column) now shows `"(Claude Code default)"` when the effective
resolved value is empty and the profile's tool is `claude-code`, rather than
reading that string out of `ModelDefault`. Added `profiles_test.go` coverage
for both the model-blank and model-set add-wizard paths.

TASK-6: `cmd/mg/setup.go` — `setupClaudePro` now takes the profile's
`config.Profile` and, after its existing OAuth-credential flow, calls a new
`setupClaudeModel` — an optional model prompt using `p.ModelEnv`/
`p.ModelDefault`, skipped entirely when a profile defines neither (which is
`claude-pro`'s own case, so its wizard output is byte-identical to before).
Added `setup_test.go` coverage for a user-defined claude profile's model
prompt and confirmed `claude-pro`'s wizard shows no model prompt.

TASK-7: `internal/config/config_test.go` — updated the pinned
`TestBuiltInProfilesCarryAuthAndModelMetadata` assertion for `claude-pro`'s
`ModelDefault` to `""`. Grepped the whole repo for the literal string
`"(Claude Code default)"`: every remaining occurrence is in
`cmd/mg/profiles.go`'s `profileModel()` (the new display-only path) or its
accompanying doc comments/tests — none left in a data field.

TASK-8: `README.md`'s `mg host` bullet described model forwarding as an
OpenCode-only mechanism ("**OpenCode model.**"); reworded to
"**Model.**" and extended to mention a claude-code profile's own model is
forwarded the same way via claude's `--model` flag. Also fixed the matching
doc comment on `profilesAdd` in `cmd/mg/profiles.go`, which described the
model prompt as opencode-only. No other doc files (`docs/AGENTS.md`,
`AGENTS.md`, `project-template/docs/AGENTS.md`, `agents/*.md`) contained
wording that was actually wrong after TASK-5/TASK-6 — none of them claimed
model configuration was opencode-specific, so none were touched, per the
task's scope boundary against unrelated rewrites.

## Known issues / follow-ups

- `setupClaudePro`'s wizard prose and title ("claude-pro — Claude Code,
  billed to your Claude Pro/Max subscription...") are still specific to
  `claude-pro`'s own wording even when run for a user-defined claude-code
  profile, and it still prompts the four fixed `CLAUDE_*` OAuth keys
  regardless of the profile's own `AuthKeys`. Both are pre-existing gaps
  called out as out of scope in tasks.md's design notes; only the new model
  prompt block was added on top.
- `claude --model <value>`'s exact accepted value shapes were spot-checked
  via `claude --help` (which documents both short aliases like `sonnet`/
  `opus` and full model ids) but not exhaustively tested against every value
  shape at runtime — no environment with real Claude Code credentials was
  available in this session to launch an actual container/host session.
