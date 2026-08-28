# Implementation: change config structure for mg serve

## Summary

Changed `mg serve`'s project registry (`config/serve.json`) from a flat list
of path strings to a list of `{"name": ..., "path": ...}` objects, giving the
operator explicit control over the URL segment ("name") each project is
served under instead of deriving it from the directory's base name. The old
flat-string form no longer parses — there is no backwards compatibility and
no base-name fallback, matching the brief.

## Changes

TASK-1/TASK-2: `src/internal/serve/registry.go` — `registryFile.Projects` is
now `[]registryEntry{Name, Path}`; `Registry` stores ordered `[]Entry{Name,
Path}`. `LoadRegistry` validates every entry: `name` must be non-empty, a
single URL-safe path segment (structural discipline reused from `api.go`'s
`validSegment`, plus a conservative `[A-Za-z0-9._-]` charset enforced by a
new `validProjectName`/`projectNamePattern`), and unique across the registry;
`path` must resolve to an existing directory, as before. Duplicate paths
under different names are refused (pinned, documented choice — the old
silent-collapse behavior for duplicate paths is replaced by a refusal, in
line with the "never silently serve a subset" discipline). `Registry.Project`
now matches by configured `name` only — the exact-path and base-name match
paths, and the ambiguous-base-name case, are gone entirely. Added
`Registry.Entries()` so callers can access the name+path pairs; kept
`Registry.Projects()` (paths only) for the one remaining caller
(`cmd/mg/serve.go`'s empty-registry warning).

TASK-3: `src/internal/serve/api.go` — `handleProjects` now sets
`projectRow.Name` from the registry's configured entry name
(`s.reg.Entries()`) instead of `filepath.Base(root)`; `resolveProject`'s doc
comment updated to describe name-only resolution.

TASK-4: `src/internal/serve/registry_test.go` rewritten for the object
schema: existing tests moved to named entries; the old
exact-path-then-base-name test was replaced by a single name-only resolution
test (`TestRegistryProjectNameOnlyResolution`, which also pins that a
differently-named entry does NOT resolve by its directory's base name); the
ambiguous-base-name test was deleted (no longer possible). New pins added:
the flat-string form fails to parse, and every validation rule (empty name,
name with `/`/`\`, name `.`/`..`, non-URL-safe charset, duplicate names,
duplicate paths) is refused.

TASK-5: `src/internal/serve/api_test.go`, `auth_test.go`, `security_test.go`,
`credentials_test.go`, `testutil_test.go` — every
`&Registry{projects: []string{...}}` construction moved to the named-entry
schema via a new shared helper, `entryFor(root)` (in `testutil_test.go`),
which names an entry after the root's base name. This keeps the large
existing surface of `filepath.Base(root)`-based URL construction unchanged
while making resolution strictly by configured `Name` (proven separately by
`TestRegistryProjectNameOnlyResolution` in TASK-4). `cmd/mg/serve_test.go`
needed no change — its `{"projects": []}` fixture parses unchanged under the
object schema (an empty array of objects).

TASK-6: `src/cmd/mg/serve.go` — the `--help` output's registry example now
shows the `{"name", "path"}` object form and explains that `name` is
required and operator-chosen.

TASK-7: `docs/AGENTS.md`, `project-template/docs/AGENTS.md`,
`docs/listener.md`, `README.md` — the registry schema example and validation
wording updated to the named-entry form across all four. (Historical/archived
job docs under `docs/jobs/archive/` and this job's own `brief.md`/`tasks.md`
still show the old flat form deliberately — they are a record of the past,
not live documentation.)

TASK-8: Verified `go build ./...`, `go vet ./...`, and `go test
./internal/serve/... ./cmd/mg/...` (serve-related tests) all pass. The wider
`go test ./...` run surfaces many pre-existing failures across
`internal/git`, `internal/job`, `internal/session`, `internal/ui`, and most of
`cmd/mg` — all caused by this sandboxed session's own git shim refusing `git
init` for any test that doesn't restrict `PATH` to the real git binary first
(confirmed identical on `internal/git`'s own tests, which this job never
touched). This is a pre-existing environment constraint unrelated to this
job's change; the `internal/serve` package tests that need real git already
use the `pathWithRealGitOnly` helper and pass cleanly.

## Known issues / follow-ups

None. The duplicate-path behavior for named entries was underspecified in
the brief ("either keep collapsing or reject") — this implementation chose
"reject" (refuse to start), documented in `LoadRegistry`'s doc comment and
pinned by `TestLoadRegistryRejectsDuplicatePaths`.
