# Verdict: New agent: chat

id: shake
status: open
reviewer: deepseek-v4-flash
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `agents/chat.md` — frontmatter matches the sibling schema exactly:
`name: chat` matches the filename, one-line `description:`, `tools: Read,
Grep, Glob` (read-only, no Bash/Write/Edit), `commit: false` (read-only git
mount). Body follows the established style (role statement, what it does /
does not do, how to approach a conversation) and is explicitly
non-implementing: no file edits, no commands, no commits. Verified the file
is picked up everywhere automatically: `agentlist.Discover` globs
`agents/*.md` (internal/agentlist/agentlist.go:103-120) and reads the
`description:` frontmatter (readDescription, line 133) — chat.md conforms;
the Dockerfile OpenCode conversion loop iterates every `*.md`
(Dockerfile:73-75) and strips `name:`/`tools:` — no change needed. Minor
observation (non-blocking, disclosed in implementation.md): the TASK-1
commit also carried the analyst's pre-existing uncommitted `tasks.md`
breakdown into the commit; it is the job's own doc file and the disclosure
is explicit, so this does not affect correctness.

TASK-2: PASS
notes: `README.md:413` — "Eleven agents are available globally" → "Twelve
agents"; `README.md:431` — `@chat` row added ("General chat assistant, like
ChatGPT — conversational and advisory" | read-only), appended at the end so
the existing row ordering is untouched; column style consistent with the
other rows. Count and table are consistent: 12 rows in the table, 12 files
in `agents/`.

TASK-3: PASS
notes: `docs/ROADMAP.md:15` — "eleven agents" → "twelve agents" in the
current-state paragraph. No behavioral content; one-word count sync as
specified.

TASK-4: PASS
notes: Verification-only task; the constraint was independently confirmed
via `git diff main...HEAD` — the only non-job files changed are
`README.md`, `agents/chat.md`, and `docs/ROADMAP.md`. No changes to
`internal/ui/agents.go` (agentMeta/agentOrder at lines 15-30 still contain
exactly the five workflow agents — chat deliberately absent), no change to
`internal/agents/agents.go` (only the five workflow constants), no changes
to `internal/orchestrate/`, `Dockerfile`, or `scripts/entrypoint.sh`.
`agentlist.Discover` picks up the new agent (verified by code reading
above). The brief's "available in-built but not part of the TUI workflow"
requirement holds: chat is discoverable via `mg agents` and the TUI's `a`
picker (agentspicker → `agentlist.Discover`) but is not in the action bar,
not in the workflow-constant set, and not in the mg-jdi sequence. Could not
independently re-run `go test ./...` (reviewer environment restricts shell
to git commands only); the agentlist/ui tests use hermetic temp fixtures
(`t.TempDir()`, `MANIGOT_HOME` isolation) and no test references the real
checkout's agent count, so a new agent file cannot affect them — the
implementation's claim is credible and low-risk. Note chat.md carries no
`permission:` block, but neither do the other non-workflow agents
(designer/mentor/architect/prompter/devops/quality) — the task spec only
required `name`/`description`/`tools`/`commit`, and chat matches the
established non-workflow pattern exactly.

## Security

No security concerns: chat is a read-only, non-committing agent (no Bash,
no Write/Edit, `commit: false`), so it gets the read-only git-common-dir
mount and cannot modify files or git metadata. The known follow-up —
`make rebuild` is required before `chat` exists inside running containers —
is an ops step correctly flagged in implementation.md, not a code defect.

## Overall

APPROVED

All four tasks implemented as specified; scope is exactly the three
declared files plus the job's own docs. No blockers.
