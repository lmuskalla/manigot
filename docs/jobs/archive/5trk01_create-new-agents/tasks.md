# Tasks: Create new agents

id: 5trk01
status: open
analyst: opencode
date: 2026-08-12

<!-- Produced by @analyst from brief.md. -->

## Context

The brief asks for two new global agents: a **systems architect** (plans how
to best build a software project/system — frameworks, components, hosting)
and a **devops engineer** (expert for pipelines and getting things running).
The "Why" is empty and out-of-scope is blank; the "What" is concrete enough
to act on.

Established pattern for adding an agent (precedent: commit 65a361b "Mentor
agent") is a single new file `agents/<name>.md` — nothing else. A new file
there is picked up automatically everywhere:

- `scripts/agents.sh` / `mg agents` — globs `agents/*.md`, no code change.
- TUI agent picker (`tui/internal/agentlist`) — globs `agents/*.md` from the
  manigot checkout, no code change.
- Dockerfile — `COPY agents/` bakes the whole dir into `~/.claude/agents/`;
  the OpenCode conversion loop iterates every `*.md` (stripping `name:` and
  `tools:` frontmatter), so new files are handled with no Dockerfile change.

Two things deliberately NOT in scope:

- The TUI action bar (`tui/internal/ui/agents.go` `agentMeta`/`agentOrder`)
  is a hardcoded five-agent job-flow subset; `designer`, `quality`,
  `prompter` and `mentor` are not in it. Do NOT add the new agents there.
- The container image must be rebuilt (`make rebuild`) before the new agents
  exist inside running containers, because `agents/` is baked in at build
  time. That is an ops step, not a code change — note it in
  `implementation.md`'s "Known issues / follow-ups" instead.

Agent-file conventions to follow (see sibling files, e.g. `agents/designer.md`,
`agents/quality.md`): kebab-case filename that matches the `name:` frontmatter
value; one-line `description:` (shown by `mg agents` and the TUI picker);
`tools:` as a plain list (Claude Code schema — it is stripped for OpenCode);
body in the existing style (role statement, what it does / does not do, how to
approach a request).

## Task breakdown

TASK-1: Create `agents/systems-architect.md`, a new global agent that plans
how to best build a software project or system — framework selection,
component/module design, hosting and deployment architecture — as a
planning/advisory agent (it plans and recommends, it does not implement,
mirroring the product-owner/designer posture). Use the existing frontmatter
schema (`name: systems-architect` matching the filename, one-line
`description:`, `tools:` list) and write the body in the style of the sibling
agent files.
     files: `agents/systems-architect.md` (new)
     depends: none
     risk: low — a new standalone file with an established schema; nothing in
       the repo (code or tests) enumerates the roster, so nothing can break;
       the only risk is format/style drift from the sibling files, which a
       template-based write avoids.

TASK-2: Create `agents/devops-engineer.md`, a new global agent that is the
expert for pipelines and getting things running — CI/CD, builds, deployment,
infrastructure, and getting services up locally or remotely. Unlike
TASK-1's architect this is a hands-on/execution agent: give it Bash and the
read/write tools so it can inspect configs and run commands, while keeping
the workflow's hard rules (no push, no merge). Same frontmatter conventions
as TASK-1 (`name: devops-engineer` matching the filename, one-line
`description:`, `tools:` list) and the same body style as the sibling files.
     files: `agents/devops-engineer.md` (new)
     depends: none — independent of TASK-1; may be done in either order
     risk: low — same reasoning as TASK-1: standalone new file, nothing
       enumerates the roster; only risk is style drift.

TASK-3: Update the `agents/` bullet in `docs/AGENTS.md` so the documented
global-agent roster stays in sync with the filesystem (hard rule: the docs
and `agents/*.md` must describe the same system): add `systems-architect` and
`devops-engineer` to the enumerated list and set the count to the real number
of files in `agents/` (11 after this job — the current text says "eight" and
already omits `mentor`, which the count should absorb while touching the same
sentence). `project-template/docs/AGENTS.md` needs no change (it does not
enumerate agents). Then verify with `go test ./...` in `tui/` that nothing
regressed (tests use temp checkouts, so none should be affected — this is
verification only).
     files: `docs/AGENTS.md`
     depends: TASK-1, TASK-2
     risk: low — a one-line documentation edit plus a verification test run;
       the only subtlety is counting the actual files in `agents/` correctly.
