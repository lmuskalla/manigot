# Implementation: New agent: chat

id: shake
status: open
developer: deepseek-v4-flash
date: 2026-08-13

<!-- Produced by @developer after implementation. -->

## Summary

Added a new global agent "chat" — a general chat AI (ChatGPT-like) that is
conversational and advisory, available in-built in every project but not part
of the TUI's job-flow workflow. The change is a single new agent file plus two
doc syncs (README, ROADMAP); no Go code, Dockerfile, or entrypoint change was
needed — the new file is picked up automatically by `mg agents`, the TUI's
agent picker, and the image bake / OpenCode conversion loops.

## Changes

TASK-1: Created `agents/chat.md` — a new global agent definition following the
sibling schema (`name: chat` matching the filename, one-line `description:`,
`tools: Read, Grep, Glob`, `commit: false` so it gets the read-only git
mount). The body is a role statement in the existing style: a general-purpose
chat assistant that answers questions, explains, brainstorms, and drafts text
in-conversation, explicitly NOT an implementation agent — no file edits, no
commands, no commits. (Note: the commit also picked up the analyst's
pre-existing uncommitted `tasks.md` breakdown, which was sitting in the job
worktree; that content is the task list this job executes against.)

TASK-2: Updated `README.md`'s Agents section — "Eleven agents are available
globally" → "Twelve agents", and added a `@chat` row to the agent table
(role: "General chat assistant, like ChatGPT — conversational and advisory",
Tools (Claude Code): read-only), appended at the end so the existing row
ordering is untouched.

TASK-3: Updated `docs/ROADMAP.md`'s current-state paragraph — "eleven agents"
→ "twelve agents" in the product summary.

TASK-4: Verification only (no files changed):
- `go test ./...` passes across all packages.
- Confirmed `agentlist.Discover` picks up the new agent: a throwaway test
  pointed `MANIGOT_HOME` at the real checkout and listed all 12 agents
  including `chat` with its description parsed correctly (test removed
  afterward, not committed).
- Confirmed the "not part of the TUI workflow" constraint holds: no changes
  to `internal/ui/agents.go` (`agentMeta`/`agentOrder`),
  `internal/agents/agents.go`, `internal/orchestrate/`, `Dockerfile`, or
  `scripts/entrypoint.sh` (verified via `git diff` — empty).

## Known issues / follow-ups

- The container image must be rebuilt (`make rebuild`) before `chat` exists
  inside running containers, because `agents/` is baked in at build time.
  That is an ops step, not a code change — flagged here as planned in
  TASK-4's analysis.
