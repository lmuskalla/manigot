# Implementation: agents location

id: major
status: open
developer:
date:

## Summary

Moved the global agents out of the Docker image: the image is now purely
isolation for workspaces, and the agents live on the system in the manigot
checkout (`agents/*.md`) and are delivered into every session — mounted
read-only into the container at the CLI's global agent location, and delivered
into the host CLI's config dirs for `mg host` (symlinks for Claude Code,
converted copies for OpenCode).

## Changes

TASK-1: Mount the global agents dir read-only into the container.
`internal/session/docker.go` — `BuildDockerInvocation` now mounts
`<home>/agents/` (resolved via `home.Root()`) read-only at the CLI's global
agent location: `/home/claude/.claude/agents:ro` for Claude Code (files mount
verbatim — the list-form frontmatter is Claude's native subagent schema).
The mount is added to the docker argv alongside the docs/project-agent mounts;
it is skipped when the checkout has no `agents/` dir. The container can use
the host's agents but cannot modify them. Tests in
`internal/session/docker_test.go` (`TestBuildClaudeGlobalAgentsMountedReadOnly`,
`TestBuildNoGlobalAgentsNoMount`).

TASK-2: Convert the mounted global agents for OpenCode at launch.
`internal/session/docker.go` — for OpenCode sessions the mounted global agents
are converted via the existing `convertAgents`/`convertAgentFile` path (the Go
equivalent of the Dockerfile's bake-time awk: `name:`/`tools:` stripped,
`permission:` passed through — carrying the read-only agents' restriction into
OpenCode's schema), written to a temp dir shadow-mounted read-only at
`/home/claude/.config/opencode/agents:ro`. The temp dir is removed by the
invocation's Cleanup hook, which now combines the project- and global-agent
cleanups. The host's `agents/` source tree is never modified. Tests:
`TestBuildOpenCodeGlobalAgentsConvertedMount`, `TestBuildNoGlobalAgentsNoMount`.

TASK-3: Remove the global-agents bake from the Dockerfile. `Dockerfile` — the
`COPY agents/ /home/claude/.claude/agents/` and the awk bake loop for
`~/.config/opencode/agents/` are gone. Companion fix: the CLI config dirs the
agents mount into (`/home/claude/.claude` and `/home/claude/.config/opencode`)
are kept in the image (empty, owned by claude, opened by the existing final
`chmod -R o+rwX /home/claude`) so the entrypoint's `opencode.json` write and
the CLIs' own state under `~/.claude` still work for the non-root session UID —
without this, docker's `-v` mount would create the mount-target parents as
root-owned and those writes would fail. Requires `make rebuild` to take effect.

TASK-4: Make the global agents available to `mg host`. `internal/session/host.go`
— `BuildHostInvocation` now calls `installHostGlobalAgents`, which delivers
each `<home>/agents/*.md` into the host CLI's own global agents dir
(`~/.claude/agents/` for Claude, `~/.config/opencode/agents/` for OpenCode,
resolved from `$HOME`). Delivery is per-tool, matching the container path's
conversion split: Claude Code gets symlinks to the raw files (the list-form
frontmatter is its native schema — edits to `agents/` are reflected live),
while OpenCode — which hard-errors on the list-form `tools:` key — gets
converted copies (`name:`/`tools:` stripped, `permission:` passed through, via
`convertAgentFile`) written as regular files. It never clobbers existing host
agent config: a name already present in the target dir is left untouched (the
user's own agent wins), except an OpenCode symlink pointing at the checkout's
own `agents/<name>` — this installer's stale raw link from before the
conversion fix — which is replaced with a converted copy so it stops
hard-erroring OpenCode. Nothing is created when the checkout has no agents, and
a best-effort failure only warns on stderr and never blocks the session. Tests
in `internal/session/host_test.go` (`TestInstallHostGlobalAgents`,
`TestInstallHostGlobalAgentsOpenCodeTarget`,
`TestInstallHostGlobalAgentsOpenCodeReplacesStaleRawSymlink`,
`TestInstallHostGlobalAgentsNoAgents`, `TestBuildHostLinksGlobalAgents`,
`TestBuildHostOpenCodeWritesConvertedGlobalAgents`).

TASK-5: Update the docs to reflect mount-based delivery. `README.md` (the
checkout tree comment, the setup "already in the image" line, the
global-agents paragraph, and the `mg host` "Agents." bullet — the old
"baked into the container image, not installed on the host" statement is
replaced with the delivery mechanism — symlinks for Claude, converted copies
for OpenCode), `docs/AGENTS.md` (stack bullet,
`internal/session` and `Dockerfile` architecture bullets, and the `mg host`
command bullet), and `project-template/docs/AGENTS.md` (a note on where the
global agents come from). The image still bakes the CLIs and `shot` — those
"baked into the image" statements are unchanged and remain accurate.

## Known issues / follow-ups

- `make rebuild` is required for the Dockerfile change to take effect in
  running sessions; until then sessions on the old image still see the baked
  agents (harmless overlap with the mounts).
- `mg host` delivery persists in the user's host CLI config dir after
  `installHostGlobalAgents` (by design — the agents stay available for
  subsequent host sessions and direct host CLI use). Claude's symlinks point at
  the live checkout files; OpenCode's converted copies are written on install
  and re-running `mg host` does not refresh an existing copy (any existing
  name wins, so the user's own agent is never clobbered) — remove a stale copy
  by hand if ever needed. A dangling Claude symlink (e.g. after the manigot
  checkout moved) is skipped on re-run rather than re-linked.
- `installHostGlobalAgents` targets the default global agents dirs
  (`~/.claude/agents`, `~/.config/opencode/agents`); a user with a custom
  `XDG_CONFIG_HOME` for opencode would need that dir (out of scope).
- `shot`'s `--describe` vision layer is unaffected (its `ZHIPU_API_KEY`
  forwarding is per-profile, unchanged).