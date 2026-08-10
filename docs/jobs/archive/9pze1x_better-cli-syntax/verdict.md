# Verdict: Better cli syntax

id: 9pze1x
status: open
reviewer: @reviewer
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review (re-review, follow-up to prior NEEDS WORK verdict)

TASK-1: PASS
notes: `scripts/mg.sh` exact-matches `$1` against `job`/`tui`/`jdi`/`done`/`delete`,
shifts and execs the right sibling script, and falls through to `run.sh` with
args untouched for everything else. Unchanged since prior pass.

TASK-2: PASS
notes: `Makefile`'s `LINKS` collapsed to `mg:mg.sh`; install/uninstall verified.
Unchanged since prior pass.

TASK-3: PASS
notes: `README.md` updated throughout. Unchanged since prior pass, plus the
`[mg-jdi: ...]` badge-text bullet now updated to `[mg jdi: ...]` as part of
TASK-10 (see below) — confirmed at README.md:267-274.

TASK-4: PASS
notes: `docs/AGENTS.md` matches the task list. Unchanged since prior pass.

TASK-5: PASS
notes: `tui/main.go`/`tui/cmd/jdi/main.go` string literals updated. Unchanged
since prior pass.

TASK-6: PASS
notes: `agents/quality.md`/`agents/reviewer.md` prose updated. Unchanged since
prior pass.

TASK-7: PASS
notes: `Spec.Label` fields, package docs, and `commands_test.go`'s `label`
column updated; `Names`/`Script`/`EnvVar` and the `mg-job` assertion in
`TestCommandSpecsAreCopies` correctly left untouched per scope decision 4.
Unchanged since prior pass.

TASK-8: PASS (verification only, no code change)
notes: unchanged since prior pass.

TASK-9: PASS (manual smoke test, no code change)
notes: unchanged since prior pass.

TASK-10: PASS
notes: This task was added specifically to close the blocking scope gap
raised in the prior verdict — every one of the 8 flagged occurrences is now
fixed exactly as specified:
- `tui/internal/ui/app.go:740` "mg-jdi is already running..." → "mg jdi is
  already running..." ✓
- `app.go:757` "→ mg-jdi started..." → "→ mg jdi started..." ✓
- `app.go:1112/1118/1120` the three list-row badge variants
  (`[mg-jdi: running @<agent>]`/`finished`/`needs human`) → `[mg jdi: ...]` ✓
- `detail.go:614` `[j] mg-jdi` action button → `[j] mg jdi` ✓
- `detail.go:722` hint bar `j run mg-jdi` → `j run mg jdi` ✓
- `detail.go:163/167` log tab placeholders → `mg jdi` ✓
- `launch.go:117` `"start mg-jdi: %w"` → `"start mg jdi: %w"` ✓
- `README.md`'s "List-row badge" bullet → `[mg jdi: ...]` ✓ (README.md:267-274)

Matching test assertions in `detail_test.go`, `jdilaunch_test.go`, and
`list_test.go` were updated in lockstep with the production strings — spot
checked each and they match the new literals exactly, no stale expectation
left behind. `go build ./...`, `go vet ./...`, and `go test ./...` all pass
(re-ran independently). The `git diff` since the prior verdict's commit
(`7eb1e7a..HEAD`) touches exactly the files TASK-10 claims and nothing else
— `README.md`, `tui/internal/launch/launch.go`, `tui/internal/ui/{app,detail}.go`,
their three test files, plus `tasks.md`/`implementation.md` for the task's
own paper trail. Doc-comment-only mentions of `mg-jdi` in prose (not
rendered/returned to a user) were correctly left untouched, consistent with
scope decision 1's framing and this task's own stated boundary (e.g.
`app.go`'s and `launch.go`'s surrounding `// mg-jdi drives...`-style comments,
`resolve`'s `Names: []string{"mg-jdi"}`).

## Scope

No new out-of-scope changes found in this follow-up pass. The diff since the
prior verdict is scoped exactly to what TASK-10 describes.

## Commit discipline

`[9pze1x] TASK-10: rename mg-jdi runtime strings in TUI to mg jdi` and
`[9pze1x] implementation: document TASK-10 (mg-jdi runtime string fix)` both
follow the correct `[ID] TASK-N: description` / `[ID] implementation: ...`
format. All prior task commits are untouched (no amends, no history rewrite).

## Security

None run (no @security pass requested).

## Overall

APPROVED

All ten tasks (TASK-1 through TASK-10) are individually correct and match
their descriptions in `tasks.md`. Build (`go build ./...`, `go vet ./...`),
tests (`go test ./...`), and manual dispatch/install smoke tests all pass.
The previously-blocking scope gap (user-visible `mg-jdi` runtime strings in
the TUI) is fully closed, with no regressions or new scope creep introduced
while fixing it. No further changes required before merge.
