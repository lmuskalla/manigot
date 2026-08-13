# Verdict: agent format consolidation

id: yec6o6
status: open
reviewer: @reviewer
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
     notes: `internal/session/agentconv.go` (new) + `internal/session/docker.go`.
     `convertAgentFile` faithfully mirrors the Dockerfile bake-time awk — I
     verified by running the exact awk from the Dockerfile against the real
     `agents/analyst.md` and comparing to the Go converter's pinned test
     expectation: identical. The documented extension (whole map-form `tools:`
     block dropped, not just the header line) is implemented and tested.
     `convertAgents` returns no-op for Claude Code / missing / empty dirs and
     cleans up its own temp dir on error. `docker.go` mounts the converted
     temp dir at `/workspace/.opencode/agents:z` for OpenCode sessions only,
     shadowing the docs mount's subpath; Claude Code sessions are untouched.
     Both callers (`cmd/mg/session.go:36`, `cmd/mg/jdi.go:268`) handle the new
     error path. The legacy `--tool opencode` path is correctly covered via
     `info.Tool == ToolOpenCode`.

TASK-2: PASS
     notes: `DockerInvocation.Cleanup` field + `defer` in `Run` — both the
     interactive `mg` path and `mg jdi`'s `--print` runs get cleanup with no
     caller changes (the task's "files likely affected" predicted edits to the
     two callers; the hook-on-invocation design makes them unnecessary, a
     benign deviation documented in implementation.md). No leak on Build error:
     `convertAgents` removes its own temp dir before returning an error, and
     there are no error paths in `BuildDockerInvocation` after the conversion
     block.

TASK-3: PASS
     notes: `agentconv_test.go` (6 unit tests: list-form strip, multi-line
     map-form strip, no-frontmatter passthrough, body `tools:` untouched,
     non-`.md` ignored, claude/missing/empty no-op cases) + `docker_test.go`
     (2 argv tests: opencode + `docs/agents/` → conversion mount present and
     Cleanup hook set while the host source file is unmodified; claude-pro
     with `docs/agents/` and opencode without → no mount, no hook). `go vet
     ./...` and `go test ./...` green; `gofmt` clean.

TASK-4: PASS
     notes: README ("Choosing a profile" caveat paragraph + "Agents" override
     paragraph), `docs/AGENTS.md` (`internal/session` + `Dockerfile` bullets),
     and `project-template/docs/AGENTS.md` + `project-template/docs/CLAUDE.md`
     mirrors (the latter within the task's conditional scope — its wording
     touches agents). All four are consistent with each other and accurately
     describe the implementation. The keep-in-sync hard rule is satisfied:
     `docs/AGENTS.md` is the canonical doc and the project-template mirror was
     updated with it; `agents/*.md` content needed no change (list form stays
     canonical).

## Security

No security findings. The one security-relevant consideration was decided
correctly and is preserved by this change: the read-only agents' tool
restriction remains enforced under Claude Code because the canonical format
stays the list form (Claude silently ignores map-form `tools:`, which would
have quietly widened those agents' access). Under OpenCode the pre-existing,
documented "unrestricted tools" caveat stays uniform for built-in and custom
agents. The new temp dir is created by the host user and mounted with the
same `:z` handling as every other mount; no secrets or new privileges
involved. @security was not run as a separate pass; nothing in the diff
warrants it.

## Overall

APPROVED

Non-blocking observations (no changes required before merge):

1. `convertAgentFile` toggles `inFrontmatter` on every exact `---` line, so a
   markdown horizontal rule (`---`) appearing in an agent *body* would re-enter
   frontmatter mode and cause subsequent body lines starting with `name:`/
   `tools:` to be stripped from the OpenCode copy — a slight divergence from
   the awk, which converts only between the first two `---` delimiters. No
   built-in body contains `---`, the consequence is benign and easily noticed,
   and the trigger (a body line exactly `---` followed by `name:`/`tools:`) is
   contrived. Could be tightened to convert only between the 1st and 2nd
   delimiter in a future pass.
2. Docker is unavailable in this environment, so the shadow-mount behavior is
   pinned by argv/unit tests and the converted output was verified to load in
   the real `opencode` 1.18.16 binary (`opencode agent list`), but a live
   `docker run` sanity check (opencode profile + a list-form custom agent) on
   a host with Docker is advisable — already flagged in implementation.md.
3. The Cleanup closure discards `os.RemoveAll`'s error (harmless /tmp
   cleanup; the pattern matches the codebase's other best-effort cleanups).
