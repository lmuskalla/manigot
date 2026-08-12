## Summary

Added two new global agents to the manigot repo: `systems-architect` (a
planning/advisory agent that plans how to best build a software project or
system — framework selection, component/module design, hosting and deployment
architecture) and `devops-engineer` (a hands-on execution agent that is the
expert for pipelines and getting things running — CI/CD, builds, deployment,
infrastructure, and getting services up locally or remotely). Also updated the
documented global-agent roster in `docs/AGENTS.md` to match the filesystem (11
agents, absorbing the previously-undocumented `mentor`).

## Changes

TASK-1: Created `agents/systems-architect.md` — new global agent following the
established frontmatter schema (`name: systems-architect` matching the
kebab-case filename, one-line `description:`, `tools:` list of
Read/Grep/Glob/Write/Edit) and the body style of the sibling advisory agents
(product-owner/designer): role statement, branch check, what it covers
(requirements, framework/language selection, component design, hosting), what
it does NOT do, approach, and output format. It plans and recommends — it does
not implement, mirroring the product-owner/designer posture.

TASK-2: Created `agents/devops-engineer.md` — new global agent with the same
frontmatter conventions (`name: devops-engineer`, one-line `description:`,
`tools:` list of Read/Write/Edit/Bash/Grep/Glob) and body style. Unlike
TASK-1's architect, this is a hands-on/execution agent: it has Bash and the
read/write tools so it can inspect configs and run commands, while its hard
rules keep the workflow's constraints (no push, no merge).

TASK-3: Updated the `agents/` bullet in `docs/AGENTS.md` — the roster now reads
"the eleven global agents" and enumerates all 11 files in `agents/` (added
`mentor`, `systems-architect`, `devops-engineer` to the previously-listed
eight). `project-template/docs/AGENTS.md` needed no change (it does not
enumerate agents). Verified nothing regressed with `go test ./...` in `tui/`
— all packages pass (tests use temp checkouts, unaffected by new agent files).

## Known issues / follow-ups

- The container image must be rebuilt (`make rebuild`) before the new agents
  exist inside running containers: `agents/` is baked into the image at build
  time (`COPY agents/` in the Dockerfile, plus the OpenCode frontmatter-strip
  loop that iterates every `*.md`). This is an ops step, not a code change.
- The TUI action bar (`tui/internal/ui/agents.go` `agentMeta`/`agentOrder`) is
  a hardcoded five-agent job-flow subset and deliberately does not include the
  new agents (nor `designer`, `quality`, `prompter`, `mentor`) — out of scope
  per tasks.md.
