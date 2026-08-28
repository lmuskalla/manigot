# Brief: change config structure for mg serve

status: done
type: chore
id: dozen
branch: chore/dozen_change-config-structure-for-mg-serve
date: 2026-08-28
author: Leander Muskalla

## What

The `mg serve` project registry currently derives a project's name (its URL
segment) from the filesystem base name of the registered path — the schema is
a flat list of paths and the operator has no say. Change the registry so each
entry is an object with an explicit, operator-chosen name:

```json
{
    "projects": [
        { "name": "solyto-api", "path": "/run/media/leo/25b6c130-5545-4931-95d1-686b3d2b0815/code/solyto/api" },
        { "name": "solyto-app", "path": "/run/media/leo/25b6c130-5545-4931-95d1-686b3d2b0815/code/solyto/app" }
    ]
}
```

Notable changes:

- **Registry schema** (`src/internal/serve/registry.go`): entries become
  `{"name": ..., "path": ...}`, `name` required. The flat-string form is
  dropped — no backwards compatibility, no base-name fallback.
- **Resolution** (`Registry.Project`): URL segment resolves by `name` only.
  The exact-path and base-name match paths go away, which also kills the
  ambiguous-base-name case — names are unique by validation.
- **Validation at startup** (same refusal-to-start discipline as today):
  `name` non-empty, a single URL path segment (no `/`, not `.`/`..`, URL-safe
  charset), unique across the registry; `path` an existing directory. A bad
  entry refuses to start — never silently serves a subset.
- **`/projects` response** (`src/internal/serve/api.go`, `projectRow`):
  `name` comes from the configured value, not `filepath.Base`.
- **Usage text** (`src/cmd/mg/serve.go` flag help) and docs (`docs/listener.md`,
  `docs/AGENTS.md` listener section) updated to the new schema.

## Why

The operator needs to choose how a project is addressed over the API.
Auto-deriving from the directory name is wrong when the directory's base name
isn't the identity the operator wants to expose.

## Out of scope

- Mutating endpoints, run supervision, streaming, any frontend (control-plane
  job two / three — untouched).
- Auth model changes (bearer token, loopback-tokenless default stays).
- Separate display labels distinct from URL names.
- Migration tooling for flat-string configs (none exist in the wild).

## Notes

- Only the listener touches the registry — the TUI, CLI, and job workflow are
  unaffected.
- The operator's existing `serve.json` already uses the object shape with a
  slashy name (`"solyto/api"`) — after this job the name must be a single
  segment; `solyto-api` is the agreed form.
- `config/serve.json` in the checkout doesn't exist yet, so there's no in-repo
  config to migrate.
- The security posture depends on this staying tight: names are URL segments,
  so validation should reuse the existing segment-validation discipline from
  `api.go`.%

