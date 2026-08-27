# Verdict: agents location

id: major
status: open
reviewer: @reviewer
date: 2026-08-27

## Review

Re-review of the branch after the two blockers from the previous verdict
(commit e8773fd) were addressed in 2806ff0 (OpenCode host delivery converted
copies), 42d867d (re-link claim corrected) and 8ee3684 (implementation.md
wording alignment).

TASK-1: PASS
notes: `internal/session/docker.go` (lines 221-254) — `BuildDockerInvocation`
resolves `<home>/agents/` via `home.Root()` and mounts it read-only at the
CLI's global agent location: `-v <home>/agents:/home/claude/.claude/agents:ro`
for Claude Code (verbatim — list-form frontmatter is Claude's native schema).
Skipped cleanly when `home.Root()` is "" or the checkout has no `agents/` dir
(`fs.IsDir` guard). Read-only requirement satisfied (`:ro` on the host bind
mount — a session cannot modify the host's `agents/` even though the container
runs as the host user UID). The mount is appended to the argv alongside the
docs/project-agent mounts (line 399), and no mount target overlaps with the
docs mount (`/workspace/.claude`) or the OpenCode project-agent mount
(`/workspace/.opencode/agents`). Tests `TestBuildClaudeGlobalAgentsMountedReadOnly`
/ `TestBuildNoGlobalAgentsNoMount` pin the argv, the no-op cases and the
absent-Cleanup case; the pre-existing tests' fake checkouts have no `agents/`
dir, so their pinned argv is unaffected.

TASK-2: PASS
notes: for OpenCode sessions the mounted global agents run through the existing
`convertAgents`/`convertAgentFile` path (`internal/session/agentconv.go`), the
Go equivalent of the removed Dockerfile awk: `name:`/`tools:` stripped
(including multi-line map-form `tools:` blocks), `permission:` passed through
untouched — carrying the read-only agents' restriction into OpenCode's schema.
Converted copies land in a temp dir shadow-mounted read-only at
`/home/claude/.config/opencode/agents:ro` (line 247). The temp dir is removed
by the invocation's Cleanup hook, now correctly combining the project- and
global-agent cleanups (lines 425-437). The host's `agents/` source tree is
never modified (verified by the source-unchanged assertion in
`TestBuildOpenCodeGlobalAgentsConvertedMount`). The `permission:` passthrough
is pinned by the pre-existing agentconv_test.go tests and by
`TestBuildOpenCodeGlobalAgentsConvertedMount`/`TestBuildNoGlobalAgentsNoMount`.
The `:ro` on the converted mount preserves the read-only boundary.

TASK-3: PASS
notes: `Dockerfile` — the `COPY agents/ /home/claude/.claude/agents/` and the
awk bake loop for `~/.config/opencode/agents/` are gone (lines 98-110). The
companion fix is correct and necessary: `RUN mkdir -p /home/claude/.claude
/home/claude/.config/opencode && chown -R claude:claude ...` (lines 109-110)
runs as root before `USER claude` (line 116) and is covered by the final
`chmod -R o+rwX /home/claude` (line 140), so the mount-target parents exist in
the image and are writable by any session UID — without this, docker's `-v`
would create the parents as root-owned at runtime and the entrypoint's
`opencode.json` write would fail for the non-root session user. The agents
subdirs themselves come from the read-only mounts. Requires `make rebuild`,
correctly documented in implementation.md's known issues.

TASK-4: PASS
notes: Both prior blockers are resolved.
(1) OpenCode host delivery: `installHostGlobalAgents` (`internal/session/host.go`
lines 266-338) now writes **converted copies** (`convertAgentFile` — `name:`/
`tools:` stripped, `permission:` passed through) into
`~/.config/opencode/agents/` instead of raw list-form symlinks, so OpenCode can
actually load the global agents under `mg host` (it hard-errors on the list-form
`tools:` key). It never clobbers existing host agent config: a regular file or a
foreign symlink at the target name is left untouched; the one deliberate
exception is a symlink pointing at the checkout's own `agents/<name>` — this
installer's stale raw link from the pre-fix build — which is replaced with a
converted copy. The README `mg host` "Agents." bullet, docs/AGENTS.md, and the
diag line ("Installed : N global agent(s) into <binary>'s host config") all now
match this per-tool delivery. Claude Code keeps the raw symlinks (list form is
its native schema), never clobbering an existing name (Lstat skip, dangling
included). Nothing is created when the checkout has no `agents/` or no home can
be located, and a best-effort failure only warns on stderr and never blocks the
session. New tests cover the OpenCode converted target
(`TestInstallHostGlobalAgentsOpenCodeTarget`), the stale-raw-symlink
replacement while preserving a foreign symlink
(`TestInstallHostGlobalAgentsOpenCodeReplacesStaleRawSymlink`), the
no-agents no-side-effects case (`TestInstallHostGlobalAgentsNoAgents`), and the
full `BuildHostInvocation` paths for both tools
(`TestBuildHostLinksGlobalAgents`, `TestBuildHostOpenCodeWritesConvertedGlobalAgents`).
(2) The implementation.md "Known issues" re-link claim is corrected: a dangling
Claude symlink (after a checkout move) is skipped on re-run rather than
re-linked (line 96), matching the code's Lstat behavior.

TASK-5: PASS
notes: `README.md` (tree comment, the setup "already in the image" line, the
global-agents paragraph — now describing the read-only mount with the
per-tool locations matching the profile table's "Global agents" row — and the
`mg host` "Agents." bullet — symlinks for Claude, converted copies for
OpenCode, never clobbering), `docs/AGENTS.md` (stack bullet, `internal/session`
and `Dockerfile` architecture bullets, `mg host` command bullet) and
`project-template/docs/AGENTS.md` are all updated to describe mount-based
delivery, and the `mg host` claims are now accurate for both tools. The
remaining "baked into the image" statements (CLIs, `shot`, Playwright) are
accurate and unchanged. The root `AGENTS.md` overlay mirrors `docs/AGENTS.md`.
No stale live references remain outside the immutable job archive.

## Security

- Read-only enforcement is sound end to end: both container mounts (Claude
  verbatim and OpenCode converted temp dir) are `:ro`; the OpenCode temp dir is
  removed after the run. A container session cannot modify the host's `agents/`
  or the converted copies.
- Host delivery is non-destructive: existing host agent config (regular files
  or foreign symlinks) is never clobbered; the only mutation is the installer's
  own stale raw OpenCode symlink being replaced by a converted copy. The
  previous side effect — raw list-form symlinks making the user's own host
  OpenCode hard-error on load — is gone.
- No secrets committed; `.env` untouched; `MANIGOT_HOME`-based resolution
  unchanged.

## Overall

APPROVED

No blockers. Non-blocking observations (no action required for merge):
- The review session could not execute `go test` (the session's git shim
  restricts bash to git commands); the new tests were statically reviewed and
  are consistent with the code and the existing test helpers
  (`checkout`/`docProject`/`writeAgent`/`fakeHostBinary`).
- Cosmetic: the diag line reads "Installed : N global agent(s)" (extra space
  before the colon).
- Narrow edge: a stale raw OpenCode symlink left by the pre-fix build whose
  checkout has since moved would be skipped (its target no longer equals the
  current `srcDir/name`) rather than replaced — mirroring the documented
  Claude-side dangling-symlink behavior; removable by hand if ever hit.