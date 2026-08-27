# Implementation: character

id: symbol
status: open
developer: developer
date: 2026-08-27

## Summary

Introduced a system-wide meta prompt (`meta.md` at the manigot checkout root)
that is always injected into every mg session, sitting at the top of the
instruction hierarchy (meta prompt → agents → skills → project context). It is
delivered through the same global delivery mechanism the codebase already uses
for agents and skills: mounted read-only into the container at each CLI's
*global instruction* location, and copied (never symlinked) into the host
CLI's own global instruction file for `mg host`. Created the manigot repo's own
`meta.md` content file as the brief requested.

## Changes

TASK-1 (verification, no files): Confirmed the per-tool global instruction
file targets from the CLIs' official documentation — Claude Code loads
`~/.claude/CLAUDE.md` (the user-global memory file, loaded in every session
before the project context), OpenCode loads `~/.config/opencode/AGENTS.md`
(the global rules file, applied across all sessions). Docker is not available
in this session's environment, so in-image probing was replaced by the
documented fallback (the CLIs' docs). Both targets are loaded in interactive,
`--print` and agentless sessions alike, and a read-only mount at
`~/.claude/CLAUDE.md` is safe (Claude treats the user-scope memory file as
trusted personal config and does not need to write it).

TASK-2: `internal/session/docker.go` — added a "Global meta prompt" block to
`BuildDockerInvocation` mirroring the global-skills block: resolves
`<home>/meta.md` via `home.Root()` and, when present, mounts it read-only at
`/home/claude/.claude/CLAUDE.md` (Claude Code) or
`/home/claude/.config/opencode/AGENTS.md` (OpenCode). No conversion, no temp
dir, no Cleanup hook — plain markdown is native to both CLIs. A missing file
yields no mount. The mount is appended to the argv assembly alongside
`globalSkillMount`.

TASK-3: `internal/session/docker_test.go` — added
`TestBuildClaudeGlobalMetaMountedReadOnly` (pins
`-v <home>/meta.md:/home/claude/.claude/CLAUDE.md:ro` and no Cleanup hook),
`TestBuildOpenCodeGlobalMetaMountedReadOnly` (pins
`-v <home>/meta.md:/home/claude/.config/opencode/AGENTS.md:ro`), and
`TestBuildNoGlobalMetaNoMount` (a checkout without `meta.md` yields neither
mount), mirroring the global-skills test shape.

TASK-4: `meta.md` (new, checkout root) — manigot's own system-wide meta
prompt: general "do this, do that" character and goals that apply to every
session regardless of agent or project (work inside the job's `docs/`, respect
the job workflow and per-task commits, never touch `.env`/credentials, prefer
small focused changes, verify rendered work with `shot`, keep `agents/*.md`
and `project-template/docs/AGENTS.md` in sync with `docs/AGENTS.md`), framed
as a layer above the agents, tool-neutral and non-duplicative of the agent
files' own rules.

TASK-5: `internal/session/host.go` — added `installHostGlobalMeta` (copies
`<home>/meta.md` to `~/.claude/CLAUDE.md` for Claude Code,
`~/.config/opencode/AGENTS.md` for OpenCode via `hostGlobalMetaFile`), a
**copy, never a symlink** (a symlink would let Claude's `/memory` writes and
agent edits land back in the checkout), never clobbering an existing host
file, installing nothing when the checkout has no `meta.md`. Wired into
`BuildHostInvocation` after the global-skills step with a warn-only
"Installed : global meta prompt into ...'s host config" diag line.

TASK-6: `internal/session/host_test.go` — added
`TestInstallHostGlobalMetaClaudeCopies` (writes `~/.claude/CLAUDE.md` as a
regular file, not a symlink), `TestInstallHostGlobalMetaOpenCodeTarget`
(writes `~/.config/opencode/AGENTS.md`), `TestInstallHostGlobalMetaNeverClobbers`
(existing host file untouched), `TestInstallHostGlobalMetaNoMeta` (no
meta.md → nothing installed, no side effects), plus the integration tests
`TestBuildHostDeliversGlobalMeta` and `TestBuildHostOpenCodeDeliversGlobalMeta`
(verify the copy lands at the per-tool target and the diag line is printed).

TASK-7: Documentation — `README.md` (added `meta.md` to the checkout layout,
a "Global meta prompt" row in the "Choosing a profile" table, a delivery
paragraph after the skills paragraph, a "Meta prompt" bullet in Host mode, and
a new "Meta prompt" section describing the mechanism and precedence);
`docs/AGENTS.md` (Stack bullet for the meta prompt, the `internal/session`
mount description, the `internal/home` source list, the Dockerfile
pre-created-dirs note, and the `mg host` delivery bullet);
`project-template/docs/AGENTS.md` (user-facing paragraph describing the meta
prompt delivery above agents/skills/project context). No `agents/*.md`
changes needed — the meta prompt is a layer above them.

## Known issues / follow-ups

- TASK-1's in-image verification could not be run: docker is unavailable in
  this session. The targets were confirmed against the CLIs' official
  documentation instead (the fallback the task allows). If either CLI version
  changes its global instruction location, the hard-coded mount targets in
  TASK-2/TASK-5 (and the tests pinning them) must be revisited.
- The `mg host` meta delivery writes into the user's real `~/.claude/CLAUDE.md`
  on the host (their personal memory file) — the non-clobbering rule
  (user's own file wins) and the copy-not-symlink choice are deliberate and
  test-pinned.