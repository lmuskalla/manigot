# Tasks: skills

id: get
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Background (what "how agents are mounted/stored" taught us)

Recent commits moved the global agents out of the Docker image and onto the
host: they now live in the manigot checkout at `<home>/agents/*.md` and are
mounted read-only into the container at the CLI's global agent location
(`~/.claude/agents/` for Claude Code, verbatim; `~/.config/opencode/agents/`
for OpenCode, converted — `name:`/`tools:` stripped, `permission:` passed
through). Project agents ride the existing `docs/` mount from `docs/agents/`
and override global agents of the same name. `mg host` delivers the same
global agents to the host CLI — symlinks for Claude Code, converted copies
for OpenCode. This job applies the same global + project / mount + host model
to **skills**.

The one structural difference that breaks a direct copy of the agent
machinery: **a skill is a directory containing a `SKILL.md`** (plus optional
support files), not a single flat `.md` file. The existing
`convertAgentFile`/symlink logic in `internal/session` operates on single
files and cannot be reused as-is; skills need their own directory-level
copy/symlink mechanism. And unlike agents (which are per-agent files), a
skill must be visible to **every** agent and to invocations that name no
agent — under both CLIs skills are loaded globally at startup
(`~/.claude/skills/`, `~/.config/opencode/skills/`), independent of the
active agent, which is exactly the "all agents + agentless" property the
brief asks for.

## Design decision the developer must make before implementing

The brief says "Find the best possible path" and does not pin the storage
layout. The recommended shape (consistent with how agents are stored) is:

- **Global skills** shipped with manigot: `<home>/skills/<name>/SKILL.md` in
  the manigot checkout (mirrors `<home>/agents/`).
- **Project skills**: `docs/skills/<name>/SKILL.md` (mirrors
  `docs/agents/`), which ride the existing `docs/` mount and override global
  skills of the same name.

Confirm this layout before coding (TASK-0); if a different layout is chosen,
all tasks below adjust accordingly. In particular, decide whether manigot
ships any skills itself or only provisions the mechanism for user-supplied
global + project skills.

## Task breakdown

TASK-0: Decide and document the global + project skills storage layout
     files: docs/AGENTS.md, README.md (documentation only)
     depends: none
     risk: medium — this is the design decision that shapes every other task;
         it is low-risk to change early but expensive to change late

TASK-1: Add a directory-level skills discovery helper that enumerates skill
     folders (each containing a SKILL.md) under a source dir, returning
     `<name>/` → dir pairs sorted by name
     files: internal/session/skillconv.go (new), internal/session/skillconv_test.go (new)
     depends: TASK-0 (path layout)
     risk: low — a pure filesystem walk with no side effects, testable in
         isolation

TASK-2: Add a skills-copy/install function that materializes global skills
     into a temp staging dir for a read-only container mount — the directory
     equivalent of convertAgents (which stages single converted agent files);
     no frontmatter conversion is needed for skills (both CLIs read
     SKILL.md frontmatter natively), but the copy must preserve each skill's
     directory and its support files
     files: internal/session/skillconv.go (new), internal/session/skillconv_test.go (new)
     depends: TASK-1
     risk: low — a straightforward recursive copy into a temp dir, mirroring
         the existing convertAgents temp-dir lifecycle

TASK-3: Mount global skills into the container at the CLI's global skills dir
     in BuildDockerInvocation — read-only; Claude Code → /home/claude/.claude/skills,
     OpenCode → /home/claude/.config/opencode/skills; staged temp dir for
     OpenCode (or a verbatim read-only bind for Claude Code, per the chosen
     layout), cleaned up via the invocation's Cleanup hook; skip cleanly when
     the checkout has no skills dir
     files: internal/session/docker.go, internal/session/docker_test.go
     depends: TASK-0, TASK-2
     risk: medium — touches the load-bearing docker argv; the existing
         global-agent mount block is the template, and its tests pin the
         exact argv strings that must be extended without breaking

TASK-4: Make project skills available from docs/skills/ — since skills are
     directories (not single files), the flat convertAgents approach cannot
     shadow them; deliver project skills so they are visible inside the
     container at the project skills location (e.g. via a staged shadow mount
     over the docs mount's skills/ subpath, mirroring the project-agents
     handling), overriding global skills of the same name
     files: internal/session/docker.go, internal/session/docker_test.go
     depends: TASK-0, TASK-2
     risk: medium — the docs mount maps docs/ onto .claude/ or .opencode/, so
         the correct in-container project-skills target must be resolved per
         tool and shadowed without disturbing the existing docs bind mount

TASK-5: Install global skills for `mg host` into the host CLI's global skills
     dir — symlinked dirs for Claude Code (~/.claude/skills/), copied dirs for
     OpenCode (~/.config/opencode/skills/), never clobbering an existing skill
     name (user's own skills win), best-effort failure that warns and never
     blocks the session — the directory-level counterpart of
     installHostGlobalAgents
     files: internal/session/host.go, internal/session/host_test.go
     depends: TASK-0, TASK-1
     risk: medium — new host-side filesystem delivery; must reuse the existing
         non-clobber/warn-only discipline from installHostGlobalAgents

TASK-6: Add the Dockerfile mount-target parent dirs for skills if needed
     (~/.claude/skills and ~/.config/opencode/skills must exist and be
     writable by any session UID, exactly as the agents dirs already are), so
     the new mounts do not create root-owned parents
     files: Dockerfile
     depends: TASK-3, TASK-4
     risk: low — a mechanical mirror of the existing agents mkdir/chown block

TASK-7: Sync documentation (docs/AGENTS.md, README.md, and any
     project-template docs) describing how skills are stored, mounted, and
     overridden, mirroring the existing agents documentation sections
     files: docs/AGENTS.md, README.md, project-template/docs/AGENTS.md
     depends: TASK-0, TASK-3, TASK-4, TASK-5
     risk: low — documentation only; must stay consistent with the hard rule
         that docs and project-template describe the same system

## Notes for the developer

- "All agents + invocations that name no agent" is satisfied by mounting
  skills at the CLI-global skills location: both CLIs load those at startup
  regardless of the active agent, so no per-agent wiring is needed.
- Reuse the existing patterns as templates wherever possible:
  `convertAgents`/`convertAgentFile` (temp-dir staging + Cleanup hook),
  `installHostGlobalAgents` (non-clobber host install), the global-agent
  mount block in `BuildDockerInvocation`, and the `fs.IsDir`/`fs.IsFile`
  helpers.
- Skills need no frontmatter conversion: both CLIs read `SKILL.md` and its
  frontmatter (`name`, `description`, ...) natively. Only directory-level
  copy/symlink is required.
- Keep `go test ./...` green at each step; the docker argv and host-install
  behavior are test-pinned and must stay that way.
- If the storage layout or a tool's skills mount target cannot be determined
  from the brief (which is intentionally open), make the conservative choice
  and flag it rather than guessing at scope.
