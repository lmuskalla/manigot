# Tasks: change config structure for mg serve

id: dozen
status: open
analyst: dozen
date: 2026-08-28

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

<!-- TASK-1: Rework the registry schema in src/internal/serve/registry.go. registryFile's Projects field becomes []entry of objects {"name": ..., "path": ...} (name required); Registry stores named, ordered entries (name + absolute cleaned path) instead of a bare []string. LoadRegistry validates every entry with the same refuse-to-start discipline as today: name non-empty, a single URL path segment (reuse the validSegment structural discipline from api.go — no "/" or "\", not "." or "..", no NUL — plus the brief's "URL-safe charset" requirement, e.g. a conservative set like [A-Za-z0-9._-]; document the chosen set in the code), names unique across the registry; path an existing directory (current os.Stat check stays). The old flat-string form must now fail to parse — no backwards compatibility, no base-name fallback (json.Unmarshal of a string into the object slice errors naturally; pin it with a test). Decide and pin the duplicate-path behavior: the old code silently collapsed identical paths — with named entries either keep collapsing or reject, but it must be explicit and tested (duplicate NAMES are always a refusal).
     files: src/internal/serve/registry.go
     depends: none
     risk: medium — the schema change is the core of the job and ripples to every consumer and test of the registry; the validation must keep the "never silently serve a subset" invariant

TASK-2: Change Registry.Project to resolve a URL segment by configured name ONLY: drop the exact-path and base-name match paths and the ambiguous-base-name case entirely (names are unique by validation, so ambiguity cannot occur). Keep the zero-path-inputs invariant — the returned root IS one of the registered paths, never a derivation from the input. Provide the API with access to each entry's configured name + path (e.g. an Entries()-style accessor or a Projects() returning the pairs) so handleProjects can render configured names; update the Registry/registryFile doc comments to the named schema.
     files: src/internal/serve/registry.go
     depends: TASK-1
     risk: low — a small, well-scoped function change; the returned-value contract (root path) is unchanged, only the matching key

TASK-3: Update src/internal/serve/api.go: handleProjects must set projectRow.Name from the configured registry entry name, not filepath.Base(root) — /projects then reports the operator-chosen identity. Update the resolveProject doc comment, which currently describes "exact path, then unique base name" resolution, to name-only resolution against Registry.Project.
     files: src/internal/serve/api.go
     depends: TASK-1, TASK-2
     risk: low — one handler body and comment updates; the JSON response shape (path + name) is unchanged

TASK-4: Rewrite src/internal/serve/registry_test.go for the object schema: existing tests change shape — writeRegistry's JSON becomes object entries; TestLoadRegistryParsesAndValidatesRoots uses named entries (drop/replace the duplicate-collapse assertion per TASK-1's decision); TestRegistryProjectLookupExactPathThenBaseName becomes name-only lookup; TestRegistryProjectAmbiguousBaseName is deleted (no base-name matching exists anymore); TestLoadRegistryRejectsNonDirectoryEntries and TestLoadRegistryAcceptsRootWithoutDocs carry named entries. Add new pins: empty name refused, name with "/" / "\" refused, name "." / ".." refused, non-URL-safe name refused, duplicate names refused, the old flat-string form ("projects": ["/path"]) refused to start, and name-only resolution (an entry registered under a name different from its directory's base name resolves by the configured name and NOT by the base name).
     files: src/internal/serve/registry_test.go
     depends: TASK-1, TASK-2
     risk: medium — the whole file changes shape and the new validation surface needs thorough pins; this is the file that proves the brief's refusal-to-start discipline

TASK-5: Update the endpoint/auth/security/credentials tests to the named schema: every &Registry{projects: []string{...}} construction (auth_test.go's testServer helper, api_test.go ~20 sites, security_test.go, credentials_test.go) becomes named entries — suggest a shared test helper (e.g. a namedRegistry(t, name, root) or testServer registering the root under a fixed name like "test-project") and URL segments that used filepath.Base(root) resolve against the configured name instead. Security tests exercise the choke point (srv.reg.Project) — the registered-name-resolves / hostile-and-unknown-doesn't assertions must key off the configured name. Verify src/cmd/mg/serve_test.go still passes: its empty-registry fixture `{"projects": []}` remains valid under the new schema (empty array), and none of the cmd/mg tests register a non-empty registry.
     files: src/internal/serve/api_test.go, src/internal/serve/auth_test.go, src/internal/serve/security_test.go, src/internal/serve/credentials_test.go, src/internal/serve/testutil_test.go (helper), src/cmd/mg/serve_test.go (verify only)
     depends: TASK-3 (API shape), TASK-4 (helper conventions)
     risk: medium — a large mechanical surface (~25 registry constructions plus URL usages); a missed base-name URL silently breaks that test's resolution, so the suite must be run to catch stragglers

TASK-6: Update the mg serve usage text in src/cmd/mg/serve.go — the flag help's registry config example (currently {"projects": ["/abs/project/root", ...]}) and any surrounding prose describe the new {"name": ..., "path": ...} object schema.
     files: src/cmd/mg/serve.go
     depends: TASK-1
     risk: low — pure help text, but it must not drift from the implemented schema

TASK-7: Update the docs to the new schema: docs/AGENTS.md's listener "Project registry" bullet (the {"projects": ["/abs/root", ...]} example and the "validated as an existing directory" wording gains the name validation), project-template/docs/AGENTS.md (kept in sync with docs/AGENTS.md per the AGENTS.md hard rule — it carries the same flat-schema example), docs/listener.md's registry scope item (currently has no explicit schema example — add/correct one to the object form), and verify README.md's mg serve section (~line 1168, same flat example) — update it if it still shows the flat form.
     files: docs/AGENTS.md, project-template/docs/AGENTS.md, docs/listener.md, README.md (verify)
     depends: TASK-1
     risk: low — documentation; must match the implemented schema exactly (name required, no back-compat, name-only resolution)

TASK-8: Verify the change end to end: go build ./... and go vet on the module, plus go test on the affected packages (internal/serve and cmd/mg — the diff/jdi endpoint tests need real git, which the existing pathWithRealGitOnly helper handles); the full serve + cmd/mg test suites must be green.
     files: none (verification)
     depends: TASK-1..TASK-7
     risk: low — the final gate; any missed base-name URL usage or stale registry construction surfaces here
-->