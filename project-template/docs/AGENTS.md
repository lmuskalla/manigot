# [Project Name]

<!--
This is your project context, loaded by the agent at the start of every session.
safecode is vendor-agnostic: it runs Claude Code (`sc`) or OpenCode
(`sc --tool opencode`) against the same project, and this one file serves
both — safecode mounts it read-only wherever the selected tool looks for it
(/workspace/AGENTS.md for OpenCode, /workspace/.claude/CLAUDE.md for Claude
Code). Those mount paths are read-only: to change this context, edit this
file (docs/AGENTS.md), never the mount paths.
The same global agents are available under @name either way.
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