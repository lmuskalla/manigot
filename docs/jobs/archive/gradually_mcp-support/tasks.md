# Tasks: mcp support

id: gradually
status: open
analyst: @analyst
date: 2026-08-31

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Ship `mcp/context7.json` — the first MCP server definition file, in
manigot's canonical schema (`{"type": "http", "url":
"https://mcp.context7.com/mcp", "headers": {"CONTEXT7_API_KEY":
"$CONTEXT7_API_KEY"}}`, working unauthenticated when the key is unset).
     files: mcp/context7.json (new)
     depends: none
     risk: low — a single static data file, no code.

TASK-2: Implement the canonical MCP schema parser plus global/project
directory discovery and merge, mirroring `listSkills`/`convertAgents`'s
existing discovery pattern: list every `*.json` file in a directory, parse
each into a server struct (`type: "http"|"stdio"`, and either `url`+`headers`
or `command`+`args`+`env`), and merge the global set (`mcp/`) with the
project set (`docs/mcp/`) by filename, project winning — exactly the
agents/skills global+project precedence.
     files: src/internal/session/mcp.go (new), src/internal/session/mcp_test.go (new)
     depends: none
     risk: low — new, self-contained code; not wired into the docker builder
     yet, so it cannot affect any existing session.

TASK-3: Implement `$VARNAME` resolution — walk every string value in a merged
server set and substitute `$VARNAME` tokens with the value of that key in
manigot's `.env` (via `internal/config`, the same source `KeyEnv` resolution
already uses elsewhere in this package) — plus the Claude Code conversion:
serialize the resolved set into Claude Code's native `.mcp.json` shape
(`{"mcpServers": {"<name>": {...}}}`).
     files: src/internal/session/mcp.go, src/internal/session/mcp_test.go
     depends: TASK-2
     risk: medium — the canonical schema in the brief already closely mirrors
     Claude's real `.mcp.json` shape (low risk for the wrapping/serialization
     itself), but getting a field wrong (e.g. the exact stdio shape) fails
     silently — Claude just won't see the server, with no loud error to catch
     it in review.

TASK-4: Implement the OpenCode conversion — fold the resolved server set
(TASK-3's resolver) into OpenCode's native `mcp` config block, and generate
the *complete* `opencode.json` (the existing `OPENCODE_MODEL` logic, moved
here, plus `mcp`) host-side in Go, replacing the model-only write currently
done in `scripts/entrypoint.sh`. Remove that now-redundant block from
`entrypoint.sh` (leave its `tui.json` theme write untouched — separate file,
nothing to merge there).
     files: src/internal/session/mcp.go, src/internal/session/mcp_test.go, scripts/entrypoint.sh
     depends: TASK-2, TASK-3
     risk: high — two compounding risks: (a) OpenCode's actual `mcp` JSON
     schema (remote vs. local server shape, exact field names) is not
     verified anywhere in this codebase today and must be confirmed against
     the installed `opencode-ai` package or its docs before relying on it —
     getting this wrong means OpenCode sessions silently get no MCP servers;
     (b) moving the model-write out of `entrypoint.sh` changes a working,
     unrelated code path (every OpenCode session's model selection, across
     every profile), so a mistake here regresses far beyond MCP.

TASK-5: Wire TASK-3/TASK-4's generated configs into `BuildDockerInvocation`
(docker.go): compute the merged server set once per session; when non-empty,
write the per-tool generated config into a temp dir/file and mount it
read-only — `.mcp.json` shadow-mounted at `/workspace/.mcp.json` for Claude
Code (mirroring this file's existing `.env` shadow-mount pattern, shadowing
any real project `.mcp.json`), the generated `opencode.json` mounted at
OpenCode's config location for OpenCode — cleaned up via the invocation's
existing `Cleanup` hook (composed alongside the agent/skill temp-dir
cleanups already there). When no servers are configured anywhere (no
`mcp/*.json`, no `docs/mcp/*.json`), nothing is generated and nothing is
mounted — the argv must stay byte-identical to today for that case.
     files: src/internal/session/docker.go, src/internal/session/docker_test.go
     depends: TASK-3, TASK-4
     risk: medium — touches the shared, test-pinned docker argv construction
     that every session path goes through (interactive, `--print`, `mg jdi`,
     and `mg serve`'s one-shot agent launch); the change is additive and
     gated on "servers configured" so the no-MCP path should be unaffected,
     but a mount-order or Cleanup-composition mistake affects every future
     session, not just MCP ones. This step is also what makes Context7 (a
     network-facing hosted server) reachable by default in every session, with
     no per-server "trust this MCP server?" prompt since sessions run under
     `--dangerously-skip-permissions` — flag explicitly for @security's
     review pass per the brief's note, no code action needed here beyond
     flagging it.

TASK-6: Update the mechanism documentation in parallel prose, per the
brief's explicit "required task, not cleanup" note: `docs/AGENTS.md` (this
repo's own project context, describing manigot's own architecture — mirrored
read-only into every session as `.claude/CLAUDE.md`/`AGENTS.md`) and
`project-template/docs/AGENTS.md` (the blank template every new project
starts from) both need a new `mcp/` + `docs/mcp/` section alongside the
existing agents/skills/meta-prompt descriptions there. Also re-check
`agents/*.md` for any existing prose describing the agents/skills delivery
mechanism and update it to match if found.
     files: docs/AGENTS.md, project-template/docs/AGENTS.md, agents/*.md (verify only — see note)
     depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5
     risk: low — prose-only, no code; the one open question is `agents/*.md`:
     a repo-wide check at analysis time found no existing prose describing the
     agents/skills delivery mechanism in any individual `agents/*.md` file
     (only `docs/AGENTS.md`, `project-template/docs/AGENTS.md`, and `README.md`
     carry it) — this part of the task may turn out to be a no-op, but should
     be re-verified at implementation time rather than assumed.

## Notes for @developer

- Out of scope (per brief): `mg host` sessions (no code change needed —
  host.go already documents why `OPENCODE_MODEL`/`OPENCODE_THEME` are
  excluded there; MCP follows the same reasoning), any `mg mcp` CLI surface,
  and shipping a second (stdio) server — TASK-2/TASK-3's schema must support
  `type: "stdio"` so the mechanism isn't Context7-special-cased, but no
  second server ships in this job.
- Do not rely on either CLI's own `${VAR}`/`{env:VAR}` config-file expansion
  for secrets — manigot resolves `$VARNAME` itself, host-side (TASK-3), for
  both CLIs uniformly.
- Reference implementations: `src/internal/session/agentconv.go` and
  `skillconv.go` for the list+convert+temp-dir-staging shape TASK-2–TASK-5
  should mirror; `BuildDockerInvocation`'s `.env` shadow-mount block in
  `docker.go` for the shadow-mount-over-a-real-file pattern TASK-5 needs for
  Claude Code's `.mcp.json`.
