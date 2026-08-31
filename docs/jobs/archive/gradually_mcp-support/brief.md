# Brief: mcp support

status: done
type: feature
id: gradually
branch: feature/gradually_mcp-support
date: 2026-08-31
author: Leander Muskalla

## What

A global+project delivery mechanism for MCP (Model Context Protocol) servers,
modeled directly on how manigot already delivers agents and skills, plus
Context7 (a hosted documentation-lookup MCP server) as the first server
shipped through it.

- **Directory layout**: `mcp/<name>.json` in the checkout (global, delivered
  to every session), overridable/extendable per project via
  `docs/mcp/<name>.json` (same filename → project replaces global entirely;
  new filename → project adds a server). This mirrors `agents/` +
  `docs/agents/` and `skills/` + `docs/skills/` exactly.
- **Canonical per-server schema**, CLI-agnostic (manigot's own shape, not
  either CLI's native one):
  ```json
  { "type": "http", "url": "...", "headers": { "SOME_KEY": "$SOME_KEY" } }
  ```
  or for a locally-spawned server:
  ```json
  { "type": "stdio", "command": "...", "args": [...], "env": { "SOME_KEY": "$SOME_KEY" } }
  ```
  `$VARNAME` in any string value references a manigot `.env` key — servers
  never carry a literal secret in a committed file.
- **First shipped server**: `mcp/context7.json`, hosted HTTP only —
  `https://mcp.context7.com/mcp`, with an optional `CONTEXT7_API_KEY`
  (`.env` key, works unauthenticated at a lower rate limit). No local
  process, no new image dependency.
- **Merge + delivery**: at session launch, global + project server files are
  merged (project wins by filename), converted into each CLI's native
  config shape, and mounted read-only into the container — Claude Code as a
  generated `.mcp.json` at the workspace root (shadowing any real project
  `.mcp.json`, the same way `.env` files are already shadow-mounted),
  OpenCode folded into its existing `opencode.json` (which today only
  carries the resolved model). No servers configured anywhere → nothing
  generated, nothing mounted — identical behavior to today.

## Why

There's currently no way to give every manigot session access to reusable
external tools/knowledge sources via MCP — each agent works from what it
already knows plus whatever's in the repo. Context7 gives agents live,
version-accurate library documentation instead of guessing from training
data; the general mechanism means the next MCP server (or a project's own)
is a JSON file drop-in, not a code change.

## Out of scope

- `mg host` sessions: host mode deliberately never writes into the user's
  own real Claude Code / OpenCode config (same reason `OPENCODE_MODEL` and
  `OPENCODE_THEME` are already excluded there — see `host.go`). MCP servers
  in host mode would need to already be configured in the user's own CLI
  config; not this job's problem to solve.
- Any `mg mcp` CLI surface (add/rm/list) — start file-based only, the same
  way `agents/`/`skills/` shipped before `mg agents` existed.
- Shipping a second (stdio-based) server — the schema must support `type:
  "stdio"` so the mechanism isn't Context7-special-cased, but only Context7
  ships in this job.
- Relying on either CLI's own `${VAR}`-style config-file env expansion —
  manigot resolves `$VARNAME` itself, host-side, into the generated config
  (OpenCode's `{env:VAR}` form is confirmed elsewhere in this codebase
  already, but Claude Code's equivalent isn't, so don't depend on it for
  either CLI — keep both paths uniform and independently verified).

## Notes

- Reference implementation for the merge/convert step:
  `src/internal/session/agentconv.go` and `skillconv.go` (list + convert +
  temp-dir staging, cleaned up via the invocation's `Cleanup` hook).
- Reference implementation for the shadow-mount-over-a-real-file pattern:
  `BuildDockerInvocation`'s `.env` shadow-mount block in
  `src/internal/session/docker.go`.
- The OpenCode `opencode.json` write currently lives in `scripts/entrypoint.sh`
  (model only, guarded by "only if the file doesn't already exist" — see
  around line 84). That guard can't compose a second, independently-generated
  piece of content into the same file, so generating the complete
  `opencode.json` (model + mcp) needs to move host-side into Go, mounted in
  the same way agents/skills already are. `entrypoint.sh`'s `tui.json`
  (theme) write is untouched — it's a separate file with nothing to merge.
- Because `entrypoint.sh` runs Claude Code with
  `--dangerously-skip-permissions`, the normal "trust this MCP server?"
  prompt never appears for the generated `.mcp.json` — the server is trusted
  automatically, same as every other tool in this mode. Not new risk
  surface, but `@security` should look at it explicitly given it's a
  network-facing server.
- `agents/*.md`, `project-template/docs/AGENTS.md`, and this repo's own
  `.claude/CLAUDE.md` (root) all describe the agents/skills mechanism in
  parallel prose today — the new `mcp/` mechanism needs the same treatment
  in all three, not just code. Treat this as a required task, not cleanup.

