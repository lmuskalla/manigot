# Verdict: Add stage column to TUI overview

id: 3iqg8j
status: open
reviewer: opencode
date: 2026-08-13

Produced by @reviewer after implementation.

## Review

TASK-1: PASS
notes: internal/ui/list.go — `columnWidths`/`listColumns()` gain a `stage`
field (width 10; longest stage name "implement" is 9 chars), `titleColsWidth`
includes the new column and its separator (5 gaps now), and `renderJobRow`
renders `pad(string(j.Stage()), cols.stage)` between status and type. Pure
rendering change reusing the existing `job.Stage()`; no data-model change,
no stage gating. Column order id / status / stage / type / date / title is
documented in tasks.md.

TASK-2: PASS
notes: internal/ui/list_test.go — the floor test's `"open    feature"`
adjacency assertion (broken once stage separated status and type) was
rebuilt via `pad()`/`join` so spacing can't drift, and the new
`TestRenderListShowsStageColumn` exercises all five stages through
`mkStageJob`, asserting the padded stage cell in both the row and the full
list render. `go test ./...`, `go vet`, and `gofmt` are all clean.

## Security

none — rendering-only change; no new I/O, shelling out, or data handling
(Stage() is a pre-existing read-only filesystem computation already used by
the detail view).

## Overall

APPROVED

Scope is clean: only internal/ui/list.go, internal/ui/list_test.go, and the
job's own docs changed; the CLI `mg jobs` listing and detail view were left
untouched per the brief. The one commit (5f8904b) is well-described and
consistent with the repo's per-feature squash convention. The stale
`columnWidths` doc comment ("used by the list and detail headers") is
pre-existing and correctly noted as out of scope in implementation.md.
