# Implementation: mg serve config via cli

id: spending
status: open
developer: glm-5.2 (developer pass)
date: 2026-08-29

## Summary

Implemented `mg serve-projects [list|add|rm] [--registry <path>]` — the CLI
for managing the projects registered for `mg serve` in `config/serve.json`,
so nobody has to remember the JSON shape by hand.

- TASK-1: PASS — store functions in `internal/serve`:
  `AddRegistryEntry`/`RemoveRegistryEntry` (plus the exported
  `ValidProjectName` wrapper and an internal `saveRegistry`). Both reuse
  `LoadRegistry`'s full validation on every write, so the CLI can never
  produce a registry the daemon would refuse to start on, and a corrupt file
  is never silently rewritten. Removing the last entry writes an empty
  `projects` list (never `null`, never a deleted file).
- TASK-2: PASS — the `mg serve-projects` command in the `mg` binary:
  list (table + the daemon's missing-docs warnings), `add [path] [name]`
  (path defaults to `$PWD`, name to the path's base name; prints the
  registered entry, the missing-docs warning when applicable, and the
  restart hint), `rm <name>`, `--registry` override with the same default
  resolution as `mg serve`, a `help` subcommand, and dispatcher + `-h` help
  wiring.
- TASK-3: PASS — docs synced: `mg serve`'s usage text and no-projects
  warning now point at `mg serve-projects`; README.md (command table +
  Listener registry paragraph), docs/AGENTS.md (Commands + "Project
  registry" bullet), and project-template/docs/AGENTS.md (listener summary)
  all document the command.

## Changes

- `src/internal/serve/registry.go` — added `ValidProjectName`,
  `AddRegistryEntry`, `RemoveRegistryEntry`, `saveRegistry`.
- `src/internal/serve/registry_store_test.go` — new: 11 tests covering
  create/append/duplicates (name and path, including a relative spelling of
  an registered path)/invalid names/missing + non-directory roots/corrupt
  file left untouched/remove-by-name/empty-list shape/unknown name.
- `src/cmd/mg/serveprojects.go` — new: the command and its three
  subcommands.
- `src/cmd/mg/serveprojects_test.go` — new: 9 tests (empty list, add+list
  round-trip via `serve.LoadRegistry`, the `$PWD`/basename defaults,
  duplicate + missing-root errors, rm + empty-after-rm, unknown rm, usage
  errors + `help`, and the no-registry-location error).
- `src/cmd/mg/main.go` — dispatcher case + help entry.
- `src/cmd/mg/serve.go` — usage text and no-projects warning wording.
- `README.md`, `docs/AGENTS.md`, `project-template/docs/AGENTS.md` —
  documentation.

Verification: `go build ./...`, `go vet`, `gofmt` (touched files clean), and
the full `go test ./...` suite pass (run with the real git ahead of the
session's PATH-first git shim — the shim's refusal of `git init` breaks the
unrelated job/ui test setups inside the container, not this change), plus a
manual end-to-end smoke test of the built binary (empty list → add with
default name → duplicate-path refusal → list → file contents → rm → empty
`projects` list).

## Known issues / follow-ups

- A running daemon still needs a restart to see registry changes (v1
  design; unchanged by this job — every mutating output says so).
- `rm` takes the registered name only; if a user forgets the name,
  `mg serve-projects` (list) shows it. An interactive picker on a TTY could
  be added later, mirroring `mg profiles`/`mg jobs`, but was not asked for.
