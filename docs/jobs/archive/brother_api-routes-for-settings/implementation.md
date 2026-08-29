# Implementation: api routes for settings

id: brother
status: open
developer:
date: 2026-08-29

<!-- Produced by @developer after implementation. -->

## Summary

The brief asked for settings routes on `mg serve` so a web UI can control
mg: the default profile (global), and the base branch + prefix per project.
The prior session that scoped this job decided to fold the @analyst breakdown
into the developer pass (tasks.md), so the tasks below were drafted together
with the implementation.

TASK-1 (settings.go): GET/PUT `/settings` — the global default profile.
Storage and write path are exactly `mg profiles <name>`'s: `MANIGOT_PROFILE`
in manigot's `.env`, written via `config.UpsertEnv`, so CLI, TUI, and this
API share one default and a switch made over HTTP changes what the next bare
`mg`, TUI, and mg-jdi launch use. GET reports the effective default
(`config.EnvValue`, falling back to `claude-pro` — the same "Active default"
chain `mg profiles` displays); PUT validates the id against
`config.ProfileByID` (unknown → 400) and persists. The envelope carries only
a plain profile id, never credential material (readiness stays in `/health`).

TASK-2 (settings.go): GET/PUT `/projects/{project}/settings` — the project's
`baseBranch` + `jobBranchPrefix`. Storage and writer are the TUI settings
screen's: `.manigot/manigot.json` in the target project, read via
`project.Load` and written via `project.Save` — the sanctioned host-side
writer for the one directory outside `docs/` manigot tooling owns. The PUT is
a wholesale replacement (absent field clears to default: baseBranch → `main`,
prefix → none), validated as ref components (`refComponentProblem` — no
spaces/control chars, `..`, leading `-`, `.lock`, etc.) so garbage is a 400
at write time instead of an opaque git failure later, and takes the project
lock so the write serializes with create/done, which read these settings
inside their own locked sections.

TASK-3 (settings_test.go + surface pins): a dedicated settings test suite
covering the default chain, the shared-write guarantee (existing .env lines
survive), validation 400s, the project settings roundtrip through the API and
`project.Load`, the fact that a rejected PUT never writes the settings file,
and that the settings written actually drive job creation (a `jobBranchPrefix`
of `mg` makes the next API-created job branch under `mg/`). The credentials/
leak suite and the hostile-segment suite enumerate the new surface exactly as
they enumerate the existing one: PUTs' envelopes carry only profile ids and
ref names and their 400s never echo `.env` content; both the project GET and
PUT go through the same `resolveProject` choke point.

TASK-4 (docs/AGENTS.md): the listener section now documents the four routes,
their validation, the lock-taking set (project settings PUT takes the
project lock; the global settings write does not — no project scope), and the
zero-path-inputs/credentials invariants they inherit.

## Changes

- `src/internal/serve/settings.go` (new) — the four handlers, the request/
  response shapes, `activeProfile`, and `refComponentProblem` ref-component
  validation.
- `src/internal/serve/server.go` — route registration for GET/PUT `/settings`
  and GET/PUT `/projects/{project}/settings`.
- `src/internal/serve/settings_test.go` (new) — the settings test suite
  (defaults, shared write, roundtrip, validation, drives-job-creation,
  ref-shape unit checks).
- `src/internal/serve/credentials_test.go` — the credentials-never-returned
  surface now enumerates the four settings routes (including the 400 paths).
- `src/internal/serve/security_test.go` — the hostile-segment suite now covers
  the project segment on both settings routes.
- `docs/AGENTS.md` — listener/control-plane section documents the settings
  endpoints.

## Verification

- `go build ./...` passes.
- `go vet ./...` passes.
- `go test ./internal/serve/ ./internal/config/ ./internal/project/` passes
  (the packages this change touches and their dependencies).
- The full suite's failures are confined to packages whose tests shell out to
  `git init`/`git worktree` (`cmd/mg`, `internal/git`, `internal/job`,
  `internal/session`, `internal/ui`) — every one is the agent container's git
  shim refusing those commands, an environmental condition unrelated to this
  change (confirmed: 284 shim refusals, zero non-shim failure causes). The
  change touches none of those packages.

## Known issues / follow-ups

- None in scope. The settings API is deliberately limited to the brief's
  three settings (profile, baseBranch, jobBranchPrefix); the OpenCode theme
  and other `.env` values stay out of the HTTP surface, consistent with the
  "credentials never returned" invariant.