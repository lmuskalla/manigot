# Verdict: change config structure for mg serve

id: dozen
status: open
reviewer: claude (automated review)
date: 2026-08-28

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `registry.go` — `registryEntry{Name,Path}` replaces the flat string,
`Registry` now stores `[]Entry`. `LoadRegistry` validates name (non-empty,
`validSegment` + `projectNamePattern` `[A-Za-z0-9._-]+`), uniqueness of
names, uniqueness of paths (explicit refusal, matching the brief's "explicit
and tested" requirement), and existing-directory `path`. The old flat-string
form fails to unmarshal into `[]registryEntry` and is pinned by
`TestLoadRegistryRejectsFlatStringForm`. Duplicate-path behavior changed from
silent-collapse to refusal, as documented and pinned
(`TestLoadRegistryRejectsDuplicatePaths`) — a reasonable, explicit choice per
the brief's "either... but it must be explicit and tested."

TASK-2: PASS
notes: `Registry.Project` now matches by `Name` only; exact-path and
base-name matching (and the ambiguous-base-name case) are removed. Zero
path-inputs invariant preserved — `Project` returns one of the registered
`Entry.Path` values verbatim. `Entries()` accessor added for API consumption
without breaking the existing `Projects()` (paths-only) caller in
`cmd/mg/serve.go`. Pinned by `TestRegistryProjectNameOnlyResolution`, which
explicitly checks that a differently-named entry does NOT resolve by its
directory's base name or by its full path.

TASK-3: PASS
notes: `api.go`'s `handleProjects` now builds `projectRow` from
`s.reg.Entries()`, setting `Name` from the configured entry name instead of
`filepath.Base(root)`. `resolveProject`'s doc comment updated to describe
name-only resolution. JSON response shape (`path`+`name` fields) unchanged,
as expected.

TASK-4: PASS
notes: `registry_test.go` rewritten for the object schema via a
`namedProjectsJSON` helper. All the pins the task asked for are present:
flat-string-form rejection, empty name, name with `/`/`\`, name `.`/`..`,
non-URL-safe charset (including a non-ASCII case, `"sölyto"`), duplicate
names, duplicate paths, and name-only resolution (registered-name resolves,
base-name and full-path do not). The old ambiguous-base-name test is
correctly deleted (no longer reachable).

TASK-5: PASS
notes: Every `&Registry{projects: []string{...}}` construction across
`api_test.go`, `auth_test.go`, `security_test.go`, `credentials_test.go` was
converted to `&Registry{entries: []Entry{entryFor(root)}}` via the new
`entryFor` helper in `testutil_test.go`, which names each entry after
`filepath.Base(root)` — preserving every existing URL-construction call site
(`"/projects/"+filepath.Base(root)+..."`) unchanged while routing resolution
through the new `Name` field. `cmd/mg/serve_test.go`'s empty-registry
fixture (`{"projects": []}`) was correctly left untouched — verified it
still parses and its tests pass.

TASK-6: PASS
notes: `serve.go`'s `--help` text now shows the `{"name","path"}` object
form and explains `name` is required/operator-chosen/URL-safe.

TASK-7: PASS
notes: `docs/AGENTS.md`, `project-template/docs/AGENTS.md`,
`docs/listener.md`, and `README.md` (~line 1160) all updated consistently to
the named schema, with matching validation wording (name-only resolution,
refusal discipline). `docs/AGENTS.md` and `project-template/docs/AGENTS.md`
stay in sync per the AGENTS.md hard rule.

TASK-8: PASS
notes: Verified independently: `go build ./...`, `go vet ./...`, and
`go test ./internal/serve/...` all clean; `go test ./cmd/mg/... -run
TestServe` (the serve-specific subset) all pass. The wider `cmd/mg`
lifecycle tests (`TestRunJobCreatesJob` etc.) fail in this sandboxed review
environment on `git init` being blocked by the session's git shim — confirmed
these are unrelated to this diff (the failing tests are in
`lifecycle_test.go`, untouched by this job, and the failure mode is the
documented git-shim restriction, not a registry/serve issue). `gofmt -l`
clean on all touched Go files.

## Security

Registry name validation is the correct choke point: `projectNamePattern`
(`^[A-Za-z0-9._-]+$`) combined with `validSegment`'s `.`/`..`/separator/NUL
checks keeps names conservative and unambiguous as URL segments, consistent
with the brief's "reuse the segment-validation discipline from api.go" note.
Zero-path-inputs invariant holds: `Registry.Project` still only ever returns
a value from the registered `Entry.Path` set, never a derivation of the
input segment. No credentials/auth model changes, consistent with the
out-of-scope list.

## Overall

APPROVED

No blockers found. Implementation matches brief and tasks.md precisely:
schema change, validation discipline, name-only resolution, API/help/docs
updates, and test coverage are all present and correct. No scope creep — the
diff touches exactly the files tasks.md scoped (registry, api, serve.go,
their tests, and the four doc files), plus the job's own docs.
