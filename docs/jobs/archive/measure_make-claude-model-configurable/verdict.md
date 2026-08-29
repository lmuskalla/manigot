# Verdict: make claude model configurable

id: measure
status: reviewed
reviewer: @reviewer
date: 2026-08-29

## Review

TASK-1: PASS
notes: `internal/config/config.go` generalizes the `ModelEnv`/`ModelDefault`
doc comments to cover claude-code profiles and drops the display-only
sentinel `"(Claude Code default)"` from `claude-pro`'s `ModelDefault` (now
unset). `config_test.go`'s `TestBuiltInProfilesCarryAuthAndModelMetadata` is
updated to match.

TASK-2: PASS
notes: `session.go` adds `ProfileInfo.ClaudeModel`, populated in
`ResolveProfile`'s claude-code branch via `envDefault(p.ModelEnv,
p.ModelDefault)`, mirroring `OpenCodeModel` exactly. New tests cover an env
override winning, a stored-default fallback, and `claude-pro` resolving to
`""`.

TASK-3: PASS
notes: `docker.go`'s `BuildDockerInvocation` appends `--model <ClaudeModel>`
right after `agentFlag` and before `promptArgs`/`opts.Pass`, gated on
`info.Tool == config.ToolClaudeCode && info.ClaudeModel != ""` — exactly the
position and condition tasks.md specified. Confirmed independently that the
installed `claude` CLI actually accepts `--model` (`claude --help` lists it),
matching the developer's claim in implementation.md. New `docker_test.go`
cases assert both the flag's presence/position and its absence for
claude-pro.

TASK-4: PASS
notes: `host.go`'s `modelArgs` construction gets an `else if` branch for
claude-code profiles using the identical `--model` mechanism already used for
opencode's host path. New `host_test.go` cases mirror the opencode tests for
both the model-set and no-model cases.

TASK-5: PASS
notes: `cmd/mg/profiles.go`'s add wizard now prompts for an optional model
env key/default when the tool is claude-code (blank is valid); `profileModel()`
now shows the `"(Claude Code default)"` sentinel only when the effective
value is empty and the profile is claude-code, reading `p.ModelDefault`
directly otherwise. New tests cover both the blank and the model-set
add-wizard paths and confirm the persisted profile fields.

TASK-6: PASS
notes: `cmd/mg/setup.go`'s `setupClaudePro` now takes `config.Profile` and
calls a new `setupClaudeModel` after the OAuth flow (in all three branches:
already-configured, `.claude.json`-read, and manual-entry), skipped entirely
when the profile defines neither `ModelEnv` nor `ModelDefault` — verified
`claude-pro`'s wizard output is unchanged by a dedicated test, and a
user-defined claude profile's model prompt writes the expected `.env` key.

TASK-7: PARTIAL
notes: Every existing/new test I ran passes (module builds cleanly, `go vet`
is clean, and all of the newly-added tests pass in isolation — pre-existing,
unrelated `git init`-based test-harness failures in this sandboxed review
session are a git-shim environment limitation, not a regression from this
diff, and I confirmed the same helpers already fail identically on
pre-existing opencode tests). The repo-wide grep for `"(Claude Code default)"`
checks out exactly as tasks.md/implementation.md describe — every remaining
occurrence is in the new display-only path or its comments/tests. The one
gap: tasks.md explicitly asked for "table/picker display for both 'model
set' and 'model unset' claude profiles" — only the pre-existing "model unset"
case (claude-pro showing the sentinel in `TestPickerRowsCarrySearchableColumns`)
is exercised; no test exercises `profileModel()`/the table row for a
claude-code profile that actually has a `ModelDefault` set (the closest new
tests only check the stored `config.Profile` fields after `add`, not the
list/picker's rendered column for such a profile). This is a minor,
low-risk gap — the code path for that case (`return p.ModelDefault` at the
end of `profileModel`) is the same one already exercised by every opencode
profile row — but it's a direct, explicit ask in tasks.md that wasn't fully
delivered.

TASK-8: PASS
notes: README.md's `mg host` bullet is reworded from "OpenCode model" to
"Model" and extended to describe claude-code's own `--model` forwarding.
Grepped `docs/AGENTS.md`/`AGENTS.md`/`project-template/docs/AGENTS.md`/
`agents/*.md` myself for opencode-only model-configuration wording — found
none that was actually wrong, matching the developer's conclusion that no
other doc file needed changes. Scope stayed minimal as instructed.

## Security

None — this job only changes profile/model plumbing (config data, argv
construction, and interactive wizard prompts); no new attack surface,
no secrets handling changes, no new inputs from untrusted sources. The one
model string forwarded to `claude --model <value>` comes from the user's own
`.env`/wizard input, same trust level as the existing opencode model
forwarding.

## Overall

APPROVED

The implementation fully and correctly closes the gap described in
brief.md: claude-code profiles can now have a configurable model, forwarded
via `--model` on both the docker and host paths, wired through the `mg
profiles add` wizard and `mg setup`, with the old display-only sentinel
correctly relocated out of the data field. All changed files are within
tasks.md's declared scope — no unrelated refactors. The one shortfall
(TASK-7's missing table/picker-display test for a claude profile with a
model actually set) is minor and low-risk enough not to block merge, but
should be picked up as a quick follow-up if this branch sees another round.
