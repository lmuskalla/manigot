# Tasks: mg serve config via cli

id: spending
status: open
analyst: glm-5.2 (analyst pass)
date: 2026-08-29

## Task breakdown

TASK-1: registry store functions — `AddRegistryEntry` / `RemoveRegistryEntry`
in `internal/serve`, reusing `LoadRegistry`'s validation as the single source
of truth so a CLI write can never produce a file the daemon would refuse to
start on (unique URL-safe name, existing directory, no duplicate paths; a
corrupt file is never silently rewritten).
files: src/internal/serve/registry.go, src/internal/serve/registry_store_test.go
depends: none
risk: low — additive functions on an existing, well-tested file format

TASK-2: `mg serve-projects [list|add|rm]` CLI command in the `mg` binary,
following the `mg profiles`/`mg serve-token` precedents: list the registered
projects, `add [path] [name]` (path defaults to `$PWD`, name to the path's
base name), `rm <name>`, `--registry` override, restart hint on every
mutation, and the no-docs warning on add.
files: src/cmd/mg/serveprojects.go, src/cmd/mg/serveprojects_test.go, src/cmd/mg/main.go
depends: TASK-1
risk: low — a new subcommand; no existing command's behavior changes

TASK-3: documentation sync — `mg serve`'s usage/startup strings, the command
lists in README.md, docs/AGENTS.md, project-template/docs/AGENTS.md, and the
"edit the file and restart" wording in the listener sections now point at
`mg serve-projects` as the primary way to change the registry.
files: src/cmd/mg/serve.go, README.md, docs/AGENTS.md, project-template/docs/AGENTS.md
depends: TASK-2
risk: low — text only
