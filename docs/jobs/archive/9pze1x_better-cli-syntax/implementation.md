# Implementation: Better cli syntax

id: 9pze1x
status: open
developer: @developer
date: 2026-08-10

<!-- Produced by @developer after implementation. -->

## Summary

Collapsed manigot's six separate top-level commands (`mg`, `mg-job`, `mg-tui`,
`mg-jdi`, `mg-done`, `mg-delete`) into a single `mg` command with subcommands
(`mg job`, `mg tui`, `mg jdi`, `mg done`, `mg delete`), plus bare `mg` for
today's default session-start behavior. This is purely a dispatch/renaming
change — `run.sh`, `new-job.sh`, `finish-job.sh`, `tui.sh`, `jdi.sh` keep
their exact internal logic, flags, and behavior; they're wired behind the new
`scripts/mg.sh` dispatcher, not rewritten.

## Changes

TASK-1: Added `scripts/mg.sh`, the sole new dispatcher script. It resolves
its own real directory the same symlink-following way `run.sh`/`tui.sh`/
`jdi.sh` already do, then does an exact match on `$1`: `job`→`new-job.sh`,
`tui`→`tui.sh`, `jdi`→`jdi.sh`, `done`→`finish-job.sh`,
`delete`→`delete-job.sh`; anything else (no args, or any other first token,
including `run.sh`'s own `--agent`/`--job`/`--tool`/`--print` flags) falls
through to `run.sh` with all original args untouched.

TASK-2: `Makefile`'s `LINKS` list collapsed to a single `mg:mg.sh` entry.
Updated the `install` target's two `mg-tui`/`mg-jdi`-worded hints and the
"canonical sc- names" comment block above `LINKS` to describe the new
single-symlink install. Verified `make install`/`make uninstall` still work
correctly (tested against a scratch `PREFIX`).

TASK-3: Updated `README.md` throughout — the repo file tree, "The installed
commands" table, "Installing without symlinks" (alias block reduced to the
single `mg` alias), the "Usage" examples, "Job workflow" and "Autonomous
mode" examples, and the "TUI" section's build/run instructions and
keybinding descriptions — to the new `mg <subcommand>` syntax. Left the
`[mg-jdi: ...]` list-row badge text and its markdown anchor references
untouched at the time — those are literal runtime strings printed by
`tui/internal/ui/app.go`, which was not in any task's file list yet. The
reviewer flagged this as a scope gap; TASK-10 below adds the badge string
(and its README counterpart) and fixes it.

TASK-4: Updated `docs/AGENTS.md` (the canonical source): the "Stack" bullet,
the Architecture bullets for each script (added a new bullet for
`scripts/mg.sh` itself, and a previously-missing bullet for
`scripts/delete-job.sh`/`mg delete`), and the "## Commands" list — all to the
new `mg <subcommand>` syntax. Confirmed `project-template/docs/AGENTS.md`
doesn't mention any of these command names, so it needed no change.

TASK-5: Updated `tui/main.go`'s `flag.Usage` text, `mg-tui:`-prefixed error
lines, and its package doc's "shells out to ... sc and mg-job" sentence; and
`tui/cmd/jdi/main.go`'s `flag.Usage` text and `mg-jdi:`-prefixed
error/status lines — all to the new `mg tui`/`mg jdi` phrasing. String-literal
changes only, verified with `go build ./...` and `go vet`. Left the rest of
`cmd/jdi/main.go`'s doc comments (package doc header, other prose mentioning
`mg-jdi`) untouched, per the task's narrower scope for that file.

TASK-6: Updated `agents/quality.md` and `agents/reviewer.md`'s "blocks
`mg-done`" prose to "blocks `mg done`".

TASK-7: Updated `tui/internal/resolve/commands.go`'s `Spec.Label` fields
(`Job()`/`Done()`/`Delete()`/`Jdi()`) and package/function doc comments,
`tui/internal/resolve/resolve.go`'s package doc overview line, and
`tui/internal/hostcmd/hostcmd.go`'s package doc — to the new `mg
<subcommand>` phrasing. `Names`/`Script`/`EnvVar` fields and the resolution
algorithm itself are untouched, per scope decision 4. Updated
`tui/internal/resolve/commands_test.go`'s `TestCommandSpecs` table's `label`
column to match; left `names`/`env`/`script` columns and
`TestCommandSpecsAreCopies`'s `Names[0] == "mg-job"` assertion unchanged.
Ran `go build ./...` and `go test ./...` — all pass.

TASK-8 (verification, no code change): Confirmed the resolution chain (env
override → `$PATH` → `$MANIGOT_HOME/scripts/*.sh` fallback) behaves as
expected once only `mg` is installed. With `make install` using the
collapsed `LINKS` and `$MANIGOT_HOME` unset, resolving `Job()`/`Done()`/
`Delete()`/`Jdi()` fails outright (their `Names` — `mg-job` etc. — no longer
exist on `$PATH`, and there's no env var or `$MANIGOT_HOME` to fall back to).
With `$MANIGOT_HOME` set, all four resolve correctly to their
`scripts/*.sh`. In the TUI's actual real invocation paths this is a
non-issue: `scripts/tui.sh`/`scripts/jdi.sh` always export `$MANIGOT_HOME`
before exec'ing the Go binaries, and `resolve.SeedHome()` (called at the top
of both binaries' `main()`) independently derives and sets it from the
binary's own location as a second line of defense even if invoked directly.
No functional regression found; no code change made.

TASK-9 (manual smoke test, no code change): Built `mg` into a scratch
`PREFIX` and exercised the full new command surface against a throwaway
scratch project (to avoid side effects on this repo): `mg job "title"`, `mg
job "title" --type fix`, `mg delete <id>`, `mg done <id>` (full archive +
squash-merge flow), `mg tui`, `mg jdi --job <id>`, bare `mg`, `mg --tool
opencode`, `mg --agent analyst --job <id>`, and an unrecognized first word
(`mg banana`). All dispatched correctly — the five subcommands routed to
their matching scripts, and every other invocation fell through to `run.sh`
with args passed through untouched, failing only on `run.sh`'s own expected
preconditions (missing `.env`/credentials), not on dispatch.

TASK-10 (added post-review, per verdict.md's blocking scope gap): Updated
the `mg-jdi` runtime strings the TUI actually displays to a user, in
`tui/internal/ui/app.go` (the "already running"/"started in the background"
status lines, the three list-row badge variants, and the `jdiStatusBadge`
doc comment describing its own output format), `tui/internal/ui/detail.go`
(the `[j] mg jdi` action button label, the hint bar's `j run mg jdi`, and
the log tab's two placeholder strings), and `tui/internal/launch/launch.go`
(the wrapped start error) — all to `mg jdi`. Updated the matching test
assertions and doc comments in `tui/internal/ui/detail_test.go`,
`tui/internal/ui/jdilaunch_test.go`, and `tui/internal/ui/list_test.go`
that quote these literal strings. Updated `README.md`'s "List-row badge"
bullet (`[mg-jdi: ...]` → `[mg jdi: ...]`) to stay in sync. String-literal
changes only, verified with `go build ./...`, `go vet ./...`, and
`go test ./...` (all pass). Left prose-only comments that merely mention
`mg-jdi` without quoting a rendered/returned string untouched (e.g.
"mg-jdi's sidecar run.log" in a doc comment), consistent with scope
decision 1's "string-literal changes only" framing, and left
`tui/internal/resolve`'s `Names: []string{"mg-jdi"}` untouched per scope
decision 4 (unaffected by this task).

## Known issues / follow-ups

None. The `Usage: mg-job ...`-style strings inside `new-job.sh`/
`finish-job.sh`/`delete-job.sh` are left stale post-rename — an explicitly
accepted, documented consequence of the brief's scope decision 2, not an
oversight. The `mg-jdi` TUI runtime strings flagged in the first review
pass are now fixed (TASK-10).
