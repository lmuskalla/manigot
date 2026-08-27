# Tasks: character

id: symbol
status: open
analyst: analyst
date: 2026-08-27

Produced by @analyst from brief.md.

## Context — the design in one paragraph

The brief asks for a **system-wide meta prompt** — a plain `.md` file that is
always injected into every mg session, sitting *above* agents and skills in
the instruction hierarchy ("system-wide 'do this, do that', then agents,
skills, etc."), in a sensible, accessible location — and to create one for the
manigot repo itself.

The natural mechanism is the one the codebase already uses for global agents
and global skills (`internal/session/docker.go` + `host.go`): a source file in
the manigot checkout, delivered per-tool at each CLI's *global instruction*
location — the layer both CLIs load in every session, independent of agent,
project, or interactive/`--print` mode:

- **Claude Code**: `~/.claude/CLAUDE.md` (the user-global memory file, loaded
  before the project context at `/workspace/.claude/CLAUDE.md`, so the
  project-level context still wins on conflict — the desired precedence).
- **OpenCode**: `~/.config/opencode/AGENTS.md` (the global rules file, loaded
  alongside the project `/workspace/AGENTS.md` context mount).

Container sessions get a read-only bind mount of the checkout file at that
location (no conversion — plain markdown is native to both CLIs; no temp dir,
so no Cleanup hook). `mg host` sessions get the file delivered into the host
CLI's own global instruction path — as a **copy**, not a symlink, because
Claude's `CLAUDE.md` is a user-writable memory file and a symlink would write
agent edits through into the checkout — non-clobbering (a user's existing
file wins) and warn-only on failure, exactly like the existing host agent/skill
installers.

The one genuinely load-bearing unknown is TASK-1: confirm the exact global
instruction file each installed CLI version reads (especially OpenCode's
global AGENTS.md, and that a read-only `~/.claude/CLAUDE.md` mount does not
disturb Claude Code startup). Everything else is a mechanical mirror of the
existing skills/agents delivery.

Explicitly out of scope (not requested by the brief): a *project-level* meta
prompt override (`docs/meta.md` riding the docs mount) — the brief asks for a
system-wide one only; per-tool meta-prompt variants; Dockerfile changes (the
mount targets' parent dirs `~/.claude` and `~/.config/opencode` are already
pre-created in the image, Dockerfile lines 112-114).

## Task breakdown

TASK-1: Verify the per-tool global instruction file targets: confirm inside
the built image that Claude Code loads `~/.claude/CLAUDE.md` (user-global
memory, loaded in interactive, `--print` and agentless sessions alike) and
that the installed `opencode-ai` loads a global rules file at
`~/.config/opencode/AGENTS.md` (or, if not, identify the correct equivalent —
e.g. opencode's `instructions` config array — and note the fallback for
TASK-2); also confirm a read-only mount at `~/.claude/CLAUDE.md` does not
break Claude Code startup or its own state writes.
     files: none (empirical verification against the image's installed
            claude/opencode versions, e.g. `docker run --rm manigot` probes or
            the CLIs' docs; findings feed the targets hard-coded in TASK-2)
     depends: none
     risk: low — verification only, but it pins every other task's mount
           targets; if OpenCode's global AGENTS.md is not auto-loaded, TASK-2's
           OpenCode delivery must switch to a documented alternative.

TASK-2: Add the global meta prompt mount to the container session builder:
in `BuildDockerInvocation`, add a block mirroring the global-skills block —
resolve the checkout file `<home>/meta.md` via `home.Root()` and, when present,
mount it read-only at `~/.claude/CLAUDE.md` for Claude Code and
`~/.config/opencode/AGENTS.md` for OpenCode; a missing file yields no mount
(optional, like skills); no conversion and no temp dir, so no Cleanup hook;
append the mount to the argv assembly alongside `globalSkillMount`.
     files: internal/session/docker.go
     depends: TASK-1 (confirms the two mount targets)
     risk: medium — a new mount in the argv-pinned assembly; the ro
           `~/.claude/CLAUDE.md` target is also Claude's memory file, so the
           ro mount must be verified not to disturb Claude's own behavior
           (TASK-1), and the OpenCode target depends on TASK-1's finding.

TASK-3: Pin the meta prompt mounts in the docker argv tests: extend
`internal/session/docker_test.go` with a test that a checkout containing
`meta.md` yields `-v <home>/meta.md:/home/claude/.claude/CLAUDE.md:ro` for
claude-pro and `-v <home>/meta.md:/home/claude/.config/opencode/AGENTS.md:ro`
for an opencode profile, and that a checkout without `meta.md` yields neither
mount (mirroring the existing `TestBuildNoGlobalSkillsNoMount` shape).
     files: internal/session/docker_test.go
     depends: TASK-2 (the mounts it pins)
     risk: low — follows the established global-skills test pattern exactly.

TASK-4: Create the manigot meta prompt content file: write `meta.md` at the
checkout root — the brief's "do create it for this repo as well" deliverable —
with manigot's system-wide character/goals: general "do this, do that"
guidance that applies to every session regardless of agent or project (e.g.
work inside the job's `docs/`, respect the job workflow and per-task commits,
never touch `.env`/credentials, prefer small focused changes, verify rendered
work with `shot`, keep `agents/*.md` and `project-template/docs/AGENTS.md` in
sync with `docs/AGENTS.md`), framed as a layer above the agents; keep it
tool-neutral and non-duplicative of the agent files' own rules (the agent
files remain the operative per-role instructions).
     files: meta.md (new, checkout root)
     depends: TASK-2 (fixes the file's location in the delivery chain)
     risk: medium — content is inherently subjective; must not contradict or
           duplicate `agents/*.md`/`docs/AGENTS.md` rules, and the wording is
           a judgment call worth a human review pass before merge.

TASK-5: Add `mg host` delivery of the meta prompt: in `internal/session/
host.go`, add an `installHostGlobalMeta` step mirroring
`installHostGlobalSkills` — copy `<home>/meta.md` into the host CLI's global
instruction file (`~/.claude/CLAUDE.md` for Claude Code,
`~/.config/opencode/AGENTS.md` for OpenCode), **copy, never symlink** (a
symlink would make Claude's `/memory` writes and agent edits land in the
checkout), never clobber an existing file (the user's own file wins), and
warn-only on failure with an "Installed" diag line like the agent/skill
installers.
     files: internal/session/host.go
     depends: TASK-2 (the source location `<home>/meta.md` it delivers)
     risk: medium — writes into the user's real `~/.claude/CLAUDE.md` on the
           host (their personal memory file); the non-clobbering rule is
           mandatory, and the copy-not-symlink choice must be respected so
           host delivery can never mutate the checkout.

TASK-6: Pin the host delivery in tests: extend `internal/session/host_test.go`
with tests for `installHostGlobalMeta` — Claude writes
`~/.claude/CLAUDE.md`, OpenCode writes `~/.config/opencode/AGENTS.md`, an
existing target file is left untouched, a checkout without `meta.md` installs
nothing, and the delivered file is a copy (not a symlink into the checkout).
     files: internal/session/host_test.go
     depends: TASK-5 (the installer it tests)
     risk: low — follows the established host agent/skill installer test
           pattern.

TASK-7: Update the documentation and keep the sync surface in line: add the
meta prompt to the README's checkout layout ("Where everything lives") and
the "Choosing a profile" table (global meta location per tool), add a short
"Meta prompt" section describing the mechanism and precedence (system-wide →
agents → skills → project context) in the README, and document the new
delivery in `docs/AGENTS.md` plus the mirror in
`project-template/docs/AGENTS.md` (per the hard sync rule); verify no
`agents/*.md` changes are needed (the meta prompt is a layer above them).
     files: README.md, docs/AGENTS.md, project-template/docs/AGENTS.md
     depends: TASK-2, TASK-5 (the mechanisms being documented)
     risk: low — pure documentation, but the three files must stay mutually
           consistent per the repo's hard rules.
