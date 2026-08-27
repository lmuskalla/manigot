# Tasks: agents location

id: major
status: open
analyst:
date:

Produced by @analyst from brief.md.

## Context

Global agents currently live in the manigot checkout at `agents/*.md`
(resolved via `home.Root()`), but are **baked into the Docker image** at build
time — `Dockerfile` does `COPY agents/ /home/claude/.claude/agents/` plus an
awk-converted copy to `/home/claude/.config/opencode/agents/`. Project agents
(`docs/agents/`) are already mounted at session launch and converted for
OpenCode via `convertAgents` (temp dir shadow-mounted over the docs mount's
`agents/` subpath).

The brief wants agents moved **out of the image** so the image is purely
isolation for workspaces, agents live on the system and are **mounted** at
session launch, and they become usable with **`mg host`** (which runs the CLI
directly on the host with no image/mounts). README currently states: "manigot's
global agents are baked into the container image, not installed on the host —
`--agent` works only if the host's own CLI has that agent installed."

`agentlist.Discover` and `agentCommits` already resolve agents host-side via
`home.Root()`, so listing and commit-marker logic are unaffected; only delivery
into the session changes.

## Task breakdown

TASK-1: Mount the global agents dir (`<home>/agents/`) into the container at
session launch, shadowing the CLI's global agent location (`~/.claude/agents/`
for Claude, `~/.config/opencode/agents/` for OpenCode), mirroring how project
`docs/agents/` is handled. Must be read-only so the container cannot modify the
host's `agents/`.
     files: internal/session/docker.go, internal/session/agentconv.go,
            internal/session/docker_test.go, internal/session/agentconv_test.go
     depends: none
     risk: medium — touches the test-pinned docker argv and must preserve the
           read-only-agent boundary when mounting host files into the container

TASK-2: Convert the mounted global agents for OpenCode at launch — move the
Dockerfile's bake-time awk strip (`name:`/`tools:` frontmatter removal,
`permission:` passthrough) into the host-side `convertAgents`/`convertAgentFile`
path so the mounted global agents get the same OpenCode treatment project
agents already get.
     files: internal/session/agentconv.go, internal/session/agentconv_test.go,
            internal/session/docker.go
     depends: TASK-1
     risk: medium — must preserve the `permission:` passthrough that carries the
           read-only-agent restriction into OpenCode's schema

TASK-3: Remove the global-agents bake from the Dockerfile (the `COPY agents/`
and the awk bake loop), leaving the image purely isolation. Requires a
`make rebuild` (image change) to take effect.
     files: Dockerfile
     depends: TASK-1, TASK-2
     risk: low — a removal, but must land after the mount path works or agents
           vanish from sessions

TASK-4: Make the global agents available to `mg host` sessions, which run the
CLI directly on the host with no image/mounts — surface `<home>/agents/` to the
host CLI (e.g. install/symlink into the CLI's host config dirs, or point the CLI
at it). Open design decision; must not clobber existing host agent config.
     files: internal/session/host.go, internal/session/host_test.go,
            possibly internal/home/home.go, agents/*.md
     depends: none (structurally), conceptually parallel to TASK-1
     risk: high — no clean precedent for how the host CLI locates global agents;
           risks mutating the user's host CLI config if not handled carefully

TASK-5: Update docs/README/AGENTS to reflect the mount-based agent delivery
instead of "baked into the image at build time", including the `mg host`
"Agents" bullet which currently states global agents are image-only.
     files: README.md, docs/AGENTS.md, project-template/docs/AGENTS.md
     depends: TASK-1, TASK-2, TASK-4
     risk: low — doc-only, but must match the implemented behavior exactly

## Open design questions (do not guess — resolve before/while implementing)

- `mg host` delivery mechanism (TASK-4): symlink/install into the CLI's host
  config dirs vs. an env/config pointer, and whether that mutates the user's
  host CLI config.
- Read-only enforcement for the mounted global agents: bake produced
  root-owned world-readable files; a host mount must be read-only so the
  container cannot edit the host's `agents/`.
- Boundary of the change given the brief's empty `Why`/`Out of scope`/`Notes`:
  confirm keeping `agents/*.md` in the manigot checkout is the intended "somewhere
  on the system", and that hooks/skills are out of scope for this job (only the
  relocation that enables them).
