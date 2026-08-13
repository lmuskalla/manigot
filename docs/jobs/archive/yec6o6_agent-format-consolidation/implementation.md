# Implementation: agent format consolidation

id: yec6o6
status: open
developer:
date: 2026-08-13

<!-- Produced by @developer after implementation. -->

## Summary

Consolidated the agent format: the Claude Code list form (`name:`,
`description:`, `tools: Read, Grep, ...`) is now the single canonical format
for **all** agent files, built-in and custom. Built-in agents already used it
and are converted for OpenCode at Docker build time (awk strips `name`/`tools`);
the missing half of the brief — custom project agents in `docs/agents/`, which
were mounted verbatim to both tools and therefore had to be hand-written as
OpenCode objects — now runs through the same conversion at session launch:
the host-side session launcher strips `name`/`tools` from `docs/agents/*.md`
for OpenCode sessions and shadow-mounts the converted copies over the docs
mount's `agents/` subpath, leaving the host source tree untouched.

The direction was confirmed with the human after live verification (Claude
Code 2.1.228, OpenCode 1.18.16): OpenCode hard-errors on list-form `tools:`
("Expected object | undefined, got ..."), and Claude Code silently does NOT
enforce map-form `tools:` (a `tools: {read: true}` agent could still run
Bash), which would quietly break the read-only agents' restriction under
Claude. Claude also requires a `name:` key OpenCode's native format lacks.
There is therefore no format both tools recognize natively — the OpenCode
direction needs the pipeline, and that pipeline is what this job extends to
custom agents.

## Changes

TASK-1: Added the launch-time conversion. New `internal/session/agentconv.go`
     with `convertAgents` (reads every `*.md` in `docs/agents/`, converts for
     OpenCode into a fresh temp dir; no-op for Claude Code or missing/empty
     dirs) and `convertAgentFile` (the Go equivalent of the Dockerfile's
     bake-time awk, dropping frontmatter `name:`/`tools:` — with one
     deliberate extension: a multi-line map-form `tools:` block is dropped
     whole, so today's object-form custom agents convert cleanly instead of
     leaving orphaned indented keys). `internal/session/docker.go` now calls
     it and, for OpenCode sessions with project agents, adds the mount
     `-v <tmp>:/workspace/.opencode/agents:z`, shadowing the docs mount's
     subpath so the tool sees converted copies while `docs/agents/` on the
     host stays pristine. Claude Code sessions are untouched — the docs mount
     already serves the raw list-form files, which are Claude's native schema.

TASK-2: Owned the temp-dir lifecycle on the invocation. `DockerInvocation`
     gained a `Cleanup` field; `Run` defers it, so both call sites — the
     interactive `mg` session path (`cmd/mg/session.go`) and `mg jdi`
     (`cmd/mg/jdi.go`) — get cleanup for free. No caller changes were needed
     (the task list's "files likely affected" predicted edits to those two
     callers; the hook-on-invocation design makes them unnecessary), and the
     error path inside `BuildDockerInvocation` cannot leak because
     `convertAgents` removes its own temp dir before returning an error.

TASK-3: Tests. New `internal/session/agentconv_test.go` covers the converter:
     list-form strip, multi-line map-form strip, no-frontmatter passthrough,
     body `tools:` untouched, non-`.md` ignored, and the no-op cases (Claude
     tool, missing dir, empty dir). `internal/session/docker_test.go` gained
     two argv tests: opencode profile with `docs/agents/` present → the
     conversion mount appears and the invocation carries a Cleanup hook while
     the host source file is unmodified; claude-pro with `docs/agents/` and
     opencode without `docs/agents/` → no conversion mount, no Cleanup hook.
     Full suite (`go vet` + `go test ./...`) is green.

TASK-4: Documentation sync. README's "Choosing a profile" caveat paragraph
     and "Agents" override paragraph now state the canonical list form and
     that OpenCode copies are generated (build time for built-ins, launch
     time for `docs/agents/`). `docs/AGENTS.md` architecture bullets
     (`internal/session`, `Dockerfile`) describe the same pipeline;
     `project-template/docs/AGENTS.md` and `project-template/docs/CLAUDE.md`
     mirror the format guidance for end projects, per the keep-in-sync hard
     rule.

## Known issues / follow-ups

- Docker is not available in this environment, so the shadow-mount behavior
  could not be exercised with a live `docker run`; it is pinned by the
  argv/unit tests, and the conversion output was verified to load cleanly in
  the real `opencode` 1.18.16 binary (`opencode agent list` — raw list-form
  files hard-error, converted files load). A live session with an opencode
  profile + a list-form custom agent is worth one manual check on a host with
  Docker.
- The README caveat stands by design: conversion strips `tools:` rather than
  converting to a map, so custom agents — like the baked-in built-ins — are
  unrestricted under OpenCode. Enforcing the read-only restriction under
  OpenCode via `permission:` frontmatter remains the documented follow-up.
- The strip now exists twice (Dockerfile awk for built-ins, Go
  `convertAgentFile` for project agents). They are kept equivalent by the
  unit tests, but a future consolidation could move the bake-time conversion
  into the same Go pipeline (convert built-ins at launch, drop the awk
  block).
