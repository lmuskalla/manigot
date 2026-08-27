# [Project Name]

<!--
This is your project context, loaded by the agent at the start of every session.
manigot is vendor-agnostic: it runs Claude Code or OpenCode against the same
project (`mg --profile claude-pro` vs `mg --profile zai`/`--profile
opencode-go`/`--profile opencode-zen`), and this one file serves both — manigot mounts it read-only
wherever the selected tool looks for it
(/workspace/AGENTS.md for OpenCode, /workspace/.claude/CLAUDE.md for Claude
Code). Those mount paths are read-only: to change this context, edit this
file (docs/AGENTS.md), never the mount paths.
The same global agents (from the manigot checkout — mounted read-only into
the container at the CLI's global agent location, or delivered into the host
CLI's config for `mg host`: symlinked for Claude Code, converted for OpenCode)
are available under @name either way, and custom
project agents in docs/agents/ work under both tools — write them in the
built-in format (name:, description:, tools: Read, Grep, ...), no per-tool
format needed. To make a custom agent read-only under OpenCode, add a
`permission:` frontmatter block (the built-in format manigot's conversion
passes through to OpenCode's schema — see the manigot README's agent section);
the read-only built-in agents' blocks deny the destructive git commands
(worktree management, branch -d/-D, reset, checkout, push, ...).
Global skills (from the manigot checkout's `skills/` — mounted read-only into
the container at the CLI's global skills location, or delivered into the host
CLI's config for `mg host`: symlinked dirs for Claude Code, copied dirs for
OpenCode) are loaded by the CLI at startup and are available to every agent
and to agentless invocations; custom project skills in `docs/skills/`
(`<name>/SKILL.md`, overriding global skills of the same name) work under both
tools the same way — write them in the standard format (a directory with a
SKILL.md carrying name:/description: frontmatter, plus optional support
files), no per-tool format needed.
A system-wide meta prompt (from the manigot checkout's `meta.md` — mounted
read-only into the container at each CLI's global instruction location
`~/.claude/CLAUDE.md` for Claude Code, `~/.config/opencode/AGENTS.md` for
OpenCode, or copied into the host CLI's own global instruction file for
`mg host`) is injected into every session above the agents, skills and this
project context. This file still wins on conflict — it is the most
project-specific layer.
Custom agents that must commit (like the built-in developer/reviewer/quality)
declare `commit: true` in their frontmatter; agents that never commit declare
`commit: false` and get a read-only git mount. The default — no agent named,
file missing, or marker absent/unknown — is a writable git mount, so a
committing agent is never broken by a missing marker.
Agent files can also deny specific commands (`deny:` — a command deny-list)
and declare a network isolation level (`network:`); both are designed but not
yet enforced. `docs/agent-template.md` in the manigot checkout is a copy-me
reference agent showing the full frontmatter surface and every guardrail.
Every job-worktree session ends with a host-side sweep-commit of leftover
changes (a `[<id>] chore: commit all` commit), so read-only agents' outputs
(like the analyst's tasks.md) are committed after their session and `mg done`
is never blocked by uncommitted leftovers. The sweep never runs for plain
sessions, `mg host` sessions, or any `--job` resolution that stays on the main
worktree — the flat-scan fallback (a repo with no local branches, or a
non-git project — there is no job worktree to sweep) and a pre-worktree job
(its branch still checked out in the main worktree itself) alike — since the
main project root holds the user's own uncommitted work.
Agent sessions also restrict git to reading history and making commits (the
session git shim): worktree management, branch deletes, resets, checkouts,
pushes, and the other destructive subcommands are refused.
The manigot TUI's job detail view also offers a `t` key that opens the job's
branch diff in tig (`mg diff <job> --tig`) in a tmux split pane / new
terminal, gated on tig being installed on the host.
Copying text from inside a session uses OSC 52: your terminal emulator must
support it, and tmux needs `set-clipboard on` when the session runs inside
tmux (mg forwards your terminal environment into the container and warns at
session start when it detects tmux would swallow the clipboard writes — see
the manigot README's "Clipboard / copying from agent sessions" section).
One deliberate exception in that forwarding: OpenCode sessions never see
`TMUX`/`TMUX_PANE`, because OpenCode's TMUX-gated tmux DCS-passthrough OSC 52
form is discarded by default tmux config (`allow-passthrough` off) — stripping
`TMUX` makes it emit plain OSC 52, which tmux's `set-clipboard on` handles.
The `shot` tool (`/usr/local/bin/shot`, see the manigot README's PLAYWRIGHT
doc) renders a URL to a PNG + model-free render report. The developer agent
uses it to verify rendered work; read-only agents consume the artifacts and
never run it.
Keep this file tool-neutral — write it for "the agent", not for one vendor.
-->

Brief description of what this project does and who it's for.

## Stack
- Backend:
- Frontend:
- Database:
- Key packages:

## Architecture
Describe the structure here — what lives where, what the key concepts are.
The more specific you are about YOUR architecture, the less Claude guesses.

## Commands
- `[test command]` — run tests
- `[build command]` — build
- `[dev command]` — start dev server

## Hard rules
- NEVER modify files outside /workspace
- NEVER run database migrations without showing them first
- NEVER install packages without asking
- NEVER touch [whatever is sensitive in this project] without flagging it
- When scope is unclear: ask, don't guess
- Do not refactor things unrelated to the current task
- Do not add abstractions not already present in the codebase