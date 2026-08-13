# Tasks: New agent: chat

id: shake
status: open
analyst: deepseek-v4-flash
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Context

The brief asks for one new global agent: **chat** — a general chat AI that
behaves like ChatGPT. It must be available in-built (baked into the image like
the other global agents) but is **not** part of the "workflow" of the TUI,
i.e. not one of the action-bar job-flow agents. The "Why" and "Out of scope"
sections are empty, but the "What" is concrete enough to act on.

Established pattern for adding a global agent (precedent: the
`5trk01_create-new-agents` job, and the earlier "Mentor agent" commit) is a
single new file `agents/<name>.md` — nothing else. A new file there is picked
up automatically everywhere:

- `mg agents` / `agentlist.Discover` (and the TUI's `a` agent picker, which
  lists the same set) — glob `agents/*.md`, no code change.
- Dockerfile — `COPY agents/` bakes the whole dir into
  `~/.claude/agents/`; the OpenCode conversion loop iterates every `*.md`
  (stripping `name:`/`tools:` frontmatter), so a new file needs no
  Dockerfile change.

Two things deliberately NOT in scope:

- The TUI action bar (`internal/ui/agents.go` `agentMeta`/`agentOrder`) is a
  hardcoded five-agent job-flow subset (owner/analyst/developer/reviewer/
  security); `designer`, `quality`, `prompter`, `mentor`, `architect` and
  `devops` are not in it. Do NOT add `chat` there — this is the brief's
  "not part of the workflow of the TUI" requirement. Also do NOT add a
  constant to `internal/agents/agents.go` (the Go-side single source of truth
  for the workflow agent names) or touch `internal/orchestrate` (the mg-jdi
  sequence) — chat is a plain conversational agent, not a workflow one.
- The container image must be rebuilt (`make rebuild`) before `chat` exists
  inside running containers, because `agents/` is baked in at build time.
  That is an ops step, not a code change — note it in `implementation.md`'s
  "Known issues / follow-ups" instead.

Agent-file conventions to follow (see sibling files, e.g.
`agents/designer.md`, `agents/mentor.md`): filename that matches the `name:`
frontmatter value; one-line `description:` (shown by `mg agents` and the TUI
picker); `tools:` as a plain list (Claude Code schema — stripped for
OpenCode); body in the existing style (role statement, what it does / does
not do, how to approach a request). A general chat agent that never modifies
files should be `commit: false` (read-only git mount) and carry read-only
tools — Read, Grep, Glob — with no Bash and no Write/Edit, so it can reference
the project when asked but cannot change it. The exact tool list is a
developer decision; keep it non-destructive.

Doc-sync scope (verified): the only user-facing roster enumerations that
exist today are `README.md` ("Eleven agents are available globally" + the
agent table) and `docs/ROADMAP.md` (the "eleven agents" current-state
mention). `docs/AGENTS.md` no longer enumerates the roster (the old bullet
was removed by the code-quality consolidation) and
`project-template/docs/AGENTS.md` does not enumerate agents, so neither needs
a roster change.

## Task breakdown

TASK-1: Create `agents/chat.md`, a new global agent "chat" that acts as a
general chat AI (ChatGPT-like): conversational and advisory, available in
every project. Frontmatter schema as the siblings (`name: chat` matching the
filename, one-line `description:`, `tools: Read, Grep, Glob`, `commit: false`)
and a body in the existing style — role statement, what it does / does not do,
how to approach a conversation. It is explicitly NOT an implementation agent:
no file edits, no commits.
     files: `agents/chat.md` (new)
     depends: none
     risk: low — a new standalone file with an established schema; nothing in
       the repo (code or tests) enumerates the global roster, so nothing can
       break; the only risk is format/style drift from the sibling files,
       which a template-based write avoids.

TASK-2: Update `README.md`'s Agents section so the documented roster matches
the filesystem: change "Eleven agents are available globally in every
project." to twelve, and add a `@chat` row to the agent table (role: general
chat assistant, like ChatGPT; Tools (Claude Code): read-only), keeping the
table's existing column style and ordering.
     files: `README.md`
     depends: TASK-1
     risk: low — a mechanical doc edit; the only subtlety is keeping the
       count and the table consistent with each other.

TASK-3: Update `docs/ROADMAP.md`'s current-state paragraph, which states
"eleven agents" in the product summary — change it to "twelve agents" so the
factual summary stays accurate after TASK-1.
     files: `docs/ROADMAP.md`
     depends: TASK-1
     risk: low — a one-word count sync in a summary paragraph; no behavioral
       content.

TASK-4: Verify the change end to end: run `go test ./...` (all packages use
hermetic temp fixtures/checkouts, so a new agent file should not affect them
— this is verification only), confirm `agentlist.Discover` picks up the new
agent, and explicitly confirm the "not part of the TUI workflow" constraint
holds: no changes to `internal/ui/agents.go` (`agentMeta`/`agentOrder`),
`internal/agents/agents.go`, `internal/orchestrate/`, `Dockerfile`, or
`scripts/entrypoint.sh`.
     files: none (verification only)
     depends: TASK-1, TASK-2, TASK-3
     risk: low — no production code is changed by this task itself; the only
       risk is discovering an unexpected dependency on the roster that the
       earlier analysis missed, which would be surfaced here and reported.
