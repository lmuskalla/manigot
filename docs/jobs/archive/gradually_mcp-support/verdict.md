# Verdict: mcp support

id: gradually
status: open
reviewer: @reviewer
date: 2026-08-31

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `mcp/context7.json` matches the canonical schema exactly
(`type: "http"`, correct URL, `CONTEXT7_API_KEY` header referencing
`$CONTEXT7_API_KEY`).

TASK-2: PASS
notes: `src/internal/session/mcp.go` (`loadMCPServers`, `mergeMCPServers`,
`discoverMCPServers`) mirrors `agentconv.go`/`skillconv.go`'s discovery
pattern faithfully — missing dir → nil, non-`.json`/subdirs skipped, unknown
`type` is a hard parse error, project wins by filename on merge. Well
covered by `mcp_test.go` (missing/empty dir, http/stdio parsing, non-json
filtering, unknown-type/invalid-json errors, merge precedence, both-empty →
nil).

TASK-3: PASS
notes: `$VARNAME` resolution (`resolveEnvRefs`/`resolveEnvMap`/
`resolveEnvSlice`/`resolveMCPServers`) uses `config.EnvValue`, the same
resolution path `CheckAuth` already uses. Unset key → dropped entry rather
than an empty string (verified for Context7's optional API key case).
`claudeMCPConfig` wraps the resolved set into `{"mcpServers": {...}}`
correctly (round-trip tested). Applies to stdio command/args/env too, not
just http/headers.

TASK-4: PASS
notes: `openCodeMCPServer`/`openCodeMCPBlock` remap http→remote,
stdio→local (command+args merged into one array, env→environment) matching
the brief's documented schema. `buildOpenCodeConfig` composes model + mcp
into one file, correctly returns "nothing to write" only when both are
empty, and generates when either alone is present (tested: model-only,
mcp-only, both, neither). `scripts/entrypoint.sh`'s old model-only
write-if-missing block is removed and replaced with an explanatory comment;
the `tui.json` theme write is untouched, as required.

TASK-5: PASS
notes: `BuildDockerInvocation` computes the merged/resolved server set once,
branches on `info.Tool`, stages the right generated file via the new
`stageMCPFile` helper, and mounts `.mcp.json`
read-only at `/workspace/.mcp.json` for Claude Code or the generated
`opencode.json` at OpenCode's global config path — both cleaned up via the
composed `Cleanup` hook. The no-MCP-configured case correctly emits no mount
and no Cleanup hook (`TestBuildNoMCPServersNoMCPMount`). Four pre-existing
tests were updated because a `zai` profile now legitimately carries a
Cleanup hook for its generated `opencode.json` (an orthogonal,
well-explained consequence of TASK-4/5, not a loosened assertion — each
test's actual pin, the specific mount path, is untouched). Full repo test
suite (`go test ./...`, with the real host `git` binary ahead of the
session's git shim on `PATH`) passes, including `go build ./...` and
`go vet ./...` cleanly.

TASK-6: PASS
notes: `docs/AGENTS.md` gets a new Stack bullet, an Architecture bullet
expansion (Claude/OpenCode conversion + shadow-mount details), an
`internal/home` bullet update listing `mcp/`, and a rewritten OpenCode
model/theme paragraph reflecting the host-side move — all accurate and
consistent with the actual code. `project-template/docs/AGENTS.md` gets a
matching paragraph including the `mg host` exclusion. The `agents/*.md`
re-check claim (no existing delivery-mechanism prose found there) was
independently verified — correct, no-op as predicted in tasks.md's risk
note.

## Security

Reviewed per the brief's explicit note (TASK-5's flagged risk): Context7, a
hosted network-facing server, becomes reachable by default in every session
once configured, with no per-server "trust this MCP server?" prompt, since
sessions run under `--dangerously-skip-permissions`/`--auto`. This is
flagged (not silently introduced) in both `brief.md`'s own note and
`implementation.md`'s "Known issues" section — same trust model as every
other tool already available in that mode, not a new class of risk this job
introduces on its own. Resolved secrets are staged into a temp file with the
same permissions (`0o644`, `os.MkdirTemp` default) as the pre-existing
agent/skill conversion temp dirs, and are only ever exposed via a read-only
container mount, same as `.env`'s shadow-mount pattern. No new credential
leakage found — nothing in the response/generated files paths returns
secrets outside the container mount, and no `.env` content is ever logged.
No other security concerns found in the diff.

## Overall

APPROVED

Implementation matches the brief and `tasks.md` task-for-task, with no
undocumented scope creep — every changed file traces back to a task, and the
"out of scope" items (mg host, `mg mcp` CLI, a second stdio server) were
correctly left alone. Full test suite, build, and vet are clean. Docs are
accurate and kept in sync per the brief's explicit requirement. Nothing
blocks merge.
