# Verdict: character

id: symbol
status: open
reviewer: reviewer
date: 2026-08-27

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

The brief asks for a system-wide meta prompt (a plain `.md` file) injected into
every mg session, above agents/skills in the instruction hierarchy, in a
sensible location — plus creating one for this repo. The implementation
delivers exactly that: `<home>/meta.md` mounted read-only into the container at
each CLI's global instruction file, and copied (never symlinked) into the host
CLI's own global instruction file for `mg host`. Change surface matches the
task list exactly — no out-of-scope edits (diff `main...HEAD` touches only
`meta.md`, `internal/session/{docker,host}.go` + tests, and the three
documentation files).

TASK-1: PASS
notes: Verification was done via the CLIs' official documentation rather than
in-image probing — docker is unavailable in this environment. The task's own
`files:` note explicitly allows this fallback ("docker run --rm manigot probes
or the CLIs' docs"), and the substitution is documented in implementation.md's
known issues. The one empirical claim left unverified is that a read-only
mount at `~/.claude/CLAUDE.md` does not disturb Claude Code startup or its
state writes — see follow-up 1 below. Both target paths
(`~/.claude/CLAUDE.md`, `~/.config/opencode/AGENTS.md`) match what the two
CLIs document as their global instruction files, and the target parents are
pre-created and world-writable in the image (Dockerfile lines 112-114,
`HOME=/home/claude`), so the new file mounts cannot land on root-owned
parents.

TASK-2: PASS
notes: internal/session/docker.go — "Global meta prompt" block mirrors the
global-skills block: resolves `<home>/meta.md` via `home.Root()`, mounts
read-only at `/home/claude/.claude/CLAUDE.md` (Claude Code) or
`/home/claude/.config/opencode/AGENTS.md` (OpenCode); missing file → no mount
(optional, like skills); no conversion/temp dir/Cleanup; appended to the argv
assembly alongside `globalSkillMount`. The `else` branch defaults to the Claude
target for any non-OpenCode tool — consistent with the existing agents/skills
blocks. Mount targets are distinct paths from the agents/skills mounts, so no
overlap.

TASK-3: PASS
notes: internal/session/docker_test.go — TestBuildClaudeGlobalMetaMountedReadOnly,
TestBuildOpenCodeGlobalMetaMountedReadOnly (both pin the exact `-v
<home>/meta.md:<target>:ro` argv and assert no Cleanup hook), and
TestBuildNoGlobalMetaNoMount (no meta.md → neither mount, both tools). Follows
the established global-skills test shape; helpers (`docProject`, `checkout`,
`containsAll`, new `writeMeta`) and imports all resolve.

TASK-4: PASS
notes: meta.md (checkout root) — general "do this, do that" character/goals
framed as a layer above the agents: character, guardrails (never commit `.env`
/credentials, work inside the job's `docs/`, respect the job workflow and
per-task commits, keep the AGENTS.md sync surface consistent), and
verify-your-work guidance (use `shot` for rendered output). Tool-neutral and
non-duplicative of the agent files' operative rules; consistent with
docs/AGENTS.md and the git shim restrictions.

TASK-5: PASS
notes: internal/session/host.go — `installHostGlobalMeta` copies `<home>/meta.md`
to `~/.claude/CLAUDE.md` / `~/.config/opencode/AGENTS.md` (via
`hostGlobalMetaFile`, derived from `os.UserHomeDir()`/`$HOME` so tests can
redirect it). Copy, never symlink (Claude's `/memory` writes must not reach the
checkout); never clobbers an existing host file (Lstat pre-check); installs
nothing when the checkout lacks `meta.md`; errors are warn-only. Wired into
`BuildHostInvocation` after the global-skills step with an "Installed" diag
line. New `internal/fs` import is used and correct.

TASK-6: PASS
notes: internal/session/host_test.go — TestInstallHostGlobalMetaClaudeCopies /
TestInstallHostGlobalMetaOpenCodeTarget (regular file, not a symlink, with
content check), TestInstallHostGlobalMetaNeverClobbers (user's file untouched),
TestInstallHostGlobalMetaNoMeta (no side effects), plus the integration tests
TestBuildHostDeliversGlobalMeta / TestBuildHostOpenCodeDeliversGlobalMeta
(copy lands at the per-tool target and the diag line prints). All helpers and
imports resolve.

TASK-7: PASS
notes: README.md (checkout layout entry, "Global meta prompt" row in the
"Choosing a profile" table, delivery paragraph after skills, Host-mode bullet,
new "Meta prompt" section with the precedence diagram);
docs/AGENTS.md (Stack bullet, internal/session mount description, internal/home
source list, Dockerfile pre-created-dirs note, mg host delivery bullet);
project-template/docs/AGENTS.md (user-facing paragraph — the project context
still wins on conflict). The three files are mutually consistent and stay in
line with the hard sync rule; no `agents/*.md` changes needed (the meta prompt
is a layer above them) — confirmed, none were made.

## Security

None. The only new host-side write path (`installHostGlobalMeta`) is
non-clobbering and warn-only, mirrors the existing agent/skill installers, and
cannot mutate the checkout (copy, never symlink). The container path is a
read-only mount of a checkout file — no new credential exposure. `meta.md`
itself contains no credentials.

## Follow-ups (non-blocking)

1. The TASK-1 in-image probe was never run (docker unavailable in both the
   developer's and reviewer's environments). Before merge, ideally run
   `docker run --rm manigot` with a checkout containing `meta.md` to confirm a
   read-only `~/.claude/CLAUDE.md` mount does not disturb Claude Code startup
   or its own state writes, and that the installed opencode-ai loads
   `~/.config/opencode/AGENTS.md` automatically. The hard-coded mount targets
   (and the tests pinning them) would need revisiting if either CLI changes
   its global instruction location. Acceptable as-is per the task's documented
   fallback, but worth a probe before merge.
2. Cosmetic: in `hostGlobalMetaFile`, if no home directory is derivable the
   target falls back to `/.claude/CLAUDE.md` and `installHostGlobalMeta` would
   fail at `MkdirAll` — warn-only, harmless, but slightly inconsistent with the
   function's doc comment ("no home can be located" → installs nothing).

## Overall

APPROVED

All seven tasks are implemented as specified and the change surface is exactly
the task list — nothing more. The mount targets, the copy-not-symlink host
delivery, the non-clobbering rule, and the no-meta no-op behavior are all
test-pinned. The only genuine residual risk (read-only `~/.claude/CLAUDE.md`
mount vs. Claude Code's behavior) is explicitly allowed by the task's fallback
and documented; it is a verification follow-up, not a defect in the work.
