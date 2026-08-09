# Verdict: adjust tui to be dynamic

id: bvi7n6
status: done
reviewer: @reviewer
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Verified against `git diff main...HEAD`, `go build ./...`, `go vet ./...`,
`gofmt -l .`, `go test ./...` (all green), and a manual `make install PREFIX=...`
/ `make uninstall PREFIX=...` round trip. Every task has its own commit with the
correct `[bvi7n6] TASK-N: …` format, in the order tasks.md suggested, plus one
non-task `handover` commit that only touches job docs (brief/tasks/verdict/
handover.md) — legitimate scaffolding, not scope creep.

TASK-0A: PASS
notes: `Dockerfile` — dedicated apt layer for `make` + `golang-go`,
`GOTOOLCHAIN=local` set. Kept separate from the PHP/Python layer as planned.

TASK-0B: PASS
notes: `Dockerfile` — `go mod download` after `USER claude`, correct ownership
path (`/home/claude/go/pkg/mod`). Q4 answered explicitly in implementation.md.

TASK-0C: PASS
notes: README "Rebuilding" section documents the toolchain, `GOTOOLCHAIN=local`,
and the module-cache/`make rebuild` coupling.

TASK-1: PASS
notes: `tui/internal/resolve/resolve.go` + `resolve_test.go`. Ordered strategy
implemented as specified; hard-fails on an unusable override rather than
silently falling through, which is the right call and is tested
(`TestResolveEnvOverrideBrokenDoesNotFallThrough`).

TASK-2: PASS
notes: `Envmanigot`/`EnvJob`/`EnvDone`/`EnvHome` constants + package doc
contract, matches tasks.md naming exactly.

TASK-3: PASS
notes: `scripts/manigot-tui.sh` exports `manigot_HOME="${manigot_HOME:-$ROOT}"`,
respects a pre-set value as required.

TASK-4: PASS
notes: `executableRoots()` covers literal + symlink-resolved binary path, both
its own dir and parent; `looksLikeCheckout` correctly rejects `go run` temp
dirs (verified by `TestExecutableRootsOutsideCheckout`, which passes in this
container). `main.go` calls `resolve.SeedHome()` at startup.

TASK-5: PASS
notes: `hostcmd.NewJob` resolves via `resolve.Job()`, keeps `cmd.Dir` and the
explicit `PWD=` env var. Test rewritten to isolate `$PATH`/env vars and asserts
absolute-path invocation, cwd, `$PWD`, and `--type` omission — good coverage.

TASK-6: PASS
notes: `launch.shellCommand` now takes the resolved path as a parameter and
quotes it with the existing `shellQuote`; a spaces-in-path test was added.

TASK-7: PASS (re-reviewed 2026-08-09)
notes: Follow-up commit `074527c` fixes the footer-height bug found in the
first pass. `detailView.bodyHeight()` (`tui/internal/ui/detail.go:178-186`) now
subtracts `d.footerLines()` — a new helper returning
`strings.Count(d.status, "\n") + 1` — instead of a hardcoded `6`. A new
`setStatus()` method (`detail.go:107-110`) is now the only way `app.go` writes
`a.detail.status` (all four call sites in `app.go` — `refreshed`,
`cmdErrorText(err)`, the launch confirmation, and the clear-on-key-default —
were switched from direct field writes to `setStatus()`), and it calls
`syncViewerSize()` so every tab's viewer is re-sized/re-clamped to the new,
smaller body height whenever the status line count changes. Traced the
arithmetic by hand: chrome is title(1) + tabs(1) + actionbar(1) + blank(1) +
body(bodyHeight) + blank(1) + footer(footerLines) = `5 + footerLines +
bodyHeight`, and `bodyHeight = height - 5 - footerLines`, so total rendered
rows == `height` exactly whenever the markdown viewer has enough content to
fill the requested height — confirmed `markdown.Viewer.View()`
(`tui/internal/markdown/markdown.go:82-91`) never returns more than `v.height`
lines, and `Resize`→`rebuild`→`clamp` re-clamps `offset` so a shrinking height
can't leave the viewport past the end of the content. Regression test
`TestDetailBodyHeightShrinksForMultiLineStatus`
(`tui/internal/ui/detail_test.go`) renders a full 24-row viewport with a
1-line and then a 3-line status and asserts total rendered rows never exceed
24 and that the "fix:" line survives — confirmed the test fails against the
pre-fix `bodyHeight()` (reverted locally, `go test ./tui/internal/ui/...`
fails with "used 26 rows, want <= 24") and passes against the fix. Checked the
one place this touches that isn't in the diff's own test: `resize()` (window
resize) goes through the same `syncViewerSize()`/`bodyHeight()` path, so a
terminal resize while a multi-line status is showing stays consistent too.
`newjob.go`'s status line was correctly left untouched — it has no fixed
viewport budget, so a multi-line status there just grows the (unconstrained)
form output rather than overflowing anything.
`gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...` all green on
`HEAD` (`7a22881`).

TASK-8: PASS
notes: `commands.go` candidate lists match the settled table exactly; tests
(`commands_test.go`) pin priority order, prove canonical-wins and legacy-still-
resolves, and prove each `Script` path exists in the repo.

TASK-9: PASS
notes: `scripts/new-job.sh`/`finish-job.sh` usage headers and error strings
renamed; filenames untouched as required. Bonus: the one stale `new-job`
mention in `manigot-tui.sh`'s comment was also fixed, which is in scope (same
kind of change, same file family) and disclosed in implementation.md.

TASK-10: PASS
notes: `Makefile` `install`/`uninstall` verified manually — creates
`manigot`, `manigot-tui`, `manigot-job`, `manigot-done`, `mg-job`, `mg-done`
symlinks under `PREFIX/bin`, `uninstall` removes only symlinks it manages.
Neither target is a prerequisite of another. `make tui` hint updated.

TASK-11: PASS
notes: README repo tree now lists `finish-job.sh` (previously missing
entirely), install section uses `make install`, new "installed commands" table
and "Installing without symlinks" section cover aliases + env vars and clearly
explain why aliases don't reach the TUI.

TASK-12: PASS
notes: `docs/AGENTS.md` synced. `docs/CLAUDE.md` correctly identified as an
empty 0-byte file with nothing to sync, and `project-template/docs/AGENTS.md`
confirmed (by grep, and independently here) to contain no `new-job`/`finish-job`
references — leaving both alone is the right call, and it's disclosed rather
than silently skipped.

TASK-13: PASS
notes: `docs/TASKS.md` — `new-job` → `manigot-job` in the workflow line and the
TUI shortcut item; the `make install` housekeeping item correctly ticked with
the job ID.

TASK-14: PASS
notes: `resolve_test.go` `TestResolutionOrderEndToEnd` installs every strategy
at once and disables the current winner one step at a time, asserting the exact
next strategy wins at each stage, ending in failure once everything is gone —
matches the task description precisely.

## Commit discipline

`implementation.md`'s follow-up commit (`7a22881`) is a docs-only commit
recording the TASK-7 fix and is correctly not itself labelled `TASK-N`. The
fix commit (`074527c`) reuses the `TASK-7:` prefix for the follow-up, which is
reasonable since it's a direct correction of that task rather than new scope.

## Security

Nothing concerning found in this diff. `make install`/`uninstall` only touch
`PREFIX/bin` (default `/usr/local/bin`, user-controlled), create symlinks (not
copies) back into the checkout, and never run as an implicit dependency of
another target. The env-var override path fails closed (hard error) rather than
silently trusting an unresolvable value. No secrets, no new network calls, no
change to how the container is launched or authenticated. Not run as a
dedicated `@security` pass — flagging that this verdict covers correctness/
quality only.

## Overall

APPROVED

All fourteen tasks are solid — the resolver, the rename, the install target,
and the docs are all correct, tested, and match tasks.md. The TASK-7
footer-height bug found in the first pass is fixed and verified: `bodyHeight()`
now sizes off the actual status line count, status writes are funneled through
`setStatus()` so the viewers always stay in sync, and a regression test locks
the behaviour in (confirmed to fail against the pre-fix code). No blockers
remain. Ready to merge.
