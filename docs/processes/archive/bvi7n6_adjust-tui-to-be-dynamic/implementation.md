# Implementation: adjust tui to be dynamic

id: bvi7n6
status: done
developer: @developer
date: 2026-08-08

## Summary

The TUI no longer assumes how safecode was installed. A new `tui/internal/resolve`
package locates each host command through an ordered strategy — env override →
canonical name on `$PATH` → short name → legacy name → the script inside a
checkout — and the two places that previously hardcoded a name (`hostcmd.NewJob`
and `launch.shellCommand`) now execute the resolved absolute path.

Alongside that, the commands got safecode-specific names: `new-job` →
`safecode-job` (`sc-job`) and `finish-job` → `safecode-done` (`sc-done`), with
the old names kept in the resolver's lookup order so existing installations keep
working without migration. `make install` / `make uninstall` create and remove
the symlinks, and the docs explain the alias + env-var route for users who don't
want anything in `/usr/local/bin`.

Prerequisite work: the container image now has `make` and Go, so the TUI can be
built and tested from inside safecode at all.

All work verified with `gofmt -l`, `go vet ./...`, `go test ./...` (all packages
pass), `make tui`, and `make install`/`make uninstall` against a throwaway
`PREFIX`.

## Changes

TASK-0A: `Dockerfile` — added a dedicated apt layer installing `make` and
`golang-go` (Debian trixie ships Go 1.24, satisfying `tui/go.mod`'s `go 1.23`),
kept separate from the PHP/Python layer so that layer stays cached, plus
`ENV GOTOOLCHAIN=local` so a future `go.mod` bump fails loudly instead of
silently downloading a toolchain. Without this the image has no way to compile
the TUI. The change was already present (uncommitted) in the working tree from
the previous session's handover; the whole file had been accidentally indented by
two spaces, which was undone before committing.

TASK-0B: `Dockerfile` — Q4 answered yes: copy `tui/go.mod` + `tui/go.sum` and run
`go mod download` after `USER claude`, so the module cache lands in
`/home/claude/go/pkg/mod` with the right owner and the TUI builds without
network access. Trade-off accepted: the image is now coupled to `tui/go.sum`.

TASK-0C: `README.md` — documented the in-image toolchain, `GOTOOLCHAIN=local`,
and that bumping a TUI dependency now requires `make rebuild`.

TASK-1: `tui/internal/resolve/resolve.go` + `resolve_test.go` (new) — the
resolver. `Spec` describes a command (label, env var, ordered candidate names,
repo-relative script); `Resolve` returns a `Result{Path, How}` or a
`*NotFoundError` carrying everything that was tried. A *set but unusable*
override is a hard error rather than a silent fall-through, so a typo in the env
var is visible. Helpers: `resolveOverride` (path vs. bare name), `repoRoots`,
`isExecutableFile` (rejects directories, follows symlinks), `absOrSame`.

TASK-2: `tui/internal/resolve/resolve.go` — the public contract:
`SAFECODE_BIN`, `SAFECODE_JOB_BIN`, `SAFECODE_DONE_BIN`, `SAFECODE_HOME` as
exported constants (`EnvSafecode`, `EnvJob`, `EnvDone`, `EnvHome`) so no caller
or message spells them out by hand, documented in the package doc together with
the reason aliases cannot be supported directly.

TASK-3: `scripts/safecode-tui.sh` — exports `SAFECODE_HOME` from the `ROOT` it
already computes (respecting a value the user set themselves), so the
wrapper-script install needs no configuration for the script fallback to work.

TASK-4: `tui/internal/resolve/resolve.go`, `tui/main.go` — `executableRoots()`
derives candidate checkouts from `os.Executable()`, considering the binary's own
directory and its parent (covers `bin/safecode-tui`) for both the literal and
the `EvalSymlinks`-resolved path. Every candidate is validated by
`looksLikeCheckout()`, which keys off `scripts/run.sh`; that is also what rejects
the `go run` temp build directory instead of inventing a bogus root. `main.go`
calls the new `resolve.SeedHome()` at startup so spawned scripts inherit
`$SAFECODE_HOME`.

TASK-5: `tui/internal/hostcmd/hostcmd.go`, `hostcmd_test.go` — `NewJob` resolves
`resolve.Job()` and runs the returned absolute path, keeping `cmd.Dir` *and* the
explicit `PWD=` env entry the script's `find_project_root` depends on. Also added
`tui/internal/resolve/commands.go` holding the specs. The test file was rewritten
to be immune to the host environment (empty `$PATH`, cleared env vars) and now
asserts the stub is invoked by absolute path, in the project root, with `$PWD`
set, and that `--type` is omitted when the type is empty.

TASK-6: `tui/internal/launch/launch.go`, `launch_test.go` — `Agent` resolves
`resolve.Safecode()`; `shellCommand` takes the path as a parameter and quotes it
with the existing `shellQuote`, so the string stays safe inside `osascript` and
`bash -lc` even for a checkout in a directory with spaces. Passing the path in
also keeps the format test environment-independent.

TASK-7: `tui/internal/resolve/resolve.go`, `tui/internal/ui/app.go` —
`NotFoundError` gained `TriedList()` and `Hint()`; the new `cmdErrorText()`
renders a resolution failure as a three-line footer diagnosis (what is missing /
every strategy tried, in order / how to fix it) while ordinary command failures
stay one-liners. Wired into both the new-job form and the agent-launch status.
Covered by `tui/internal/ui/cmderror_test.go` (new).

TASK-8: `tui/internal/resolve/commands.go`, `commands_test.go` (new) — the
settled names: `safecode-job` / `sc-job` / `new-job` and `safecode-done` /
`sc-done` / `finish-job`, plus a `Done()` spec so the whole command surface is
covered. Tests pin the exact lists, prove the canonical name wins when several
are installed side by side, prove a pre-rename install (legacy name only) still
resolves silently, and check each spec's `Script` path exists in the repo.

TASK-9: `scripts/new-job.sh`, `scripts/finish-job.sh` (and the one stale mention
in `scripts/safecode-tui.sh`) — usage headers and argument-error messages now say
`safecode-job` / `safecode-done`, noting the short alias and that the legacy name
still works. Script *filenames* unchanged.

TASK-10: `Makefile` — `make install` symlinks all four launchers plus `sc-job` /
`sc-done` into `$(PREFIX)/bin` (default `/usr/local`), from a single `LINKS`
list; `make uninstall` removes them but only when they are symlinks. Symlinks
rather than copies so `git pull` updates the installed commands. Warns when
`BINDIR` is not on `PATH` and when `bin/safecode-tui` has not been built. Neither
target is a prerequisite of anything else, since `install` is the only thing here
that writes outside the repo. `make tui` and the TUI wrapper now point at
`make install`.

TASK-11: `README.md` — repo tree shows the installed name for each script
(including `finish-job.sh`, which was missing entirely); the install step is
`make install` with a `PREFIX` example; a new "The installed commands" table and
an "Installing without symlinks" section (aliases + `SAFECODE_*` overrides, and
why aliases alone cannot work for the TUI). Usage, job-workflow, TUI and
keybinding sections use the new names. The shared install docs were moved below
the OpenCode subsection, since the launchers are not Claude Code specific.

TASK-12: `docs/AGENTS.md` — new command names, the `safecode-tui` wrapper and
`SAFECODE_HOME`, the `resolve` package and its lookup order (with the rule that
nothing in the TUI may hardcode a command name), and the new `make install` /
`make tui` targets.

TASK-13: `docs/TASKS.md` — `new-job` → `safecode-job` in the workflow note and
the TUI shortcut item; ticked off the "Add `make install` target" housekeeping
item that TASK-10 delivered.

TASK-14: `tui/internal/resolve/resolve_test.go` — end-to-end order test:
installs every strategy at once, then removes the current winner one step at a
time and asserts each fall-through lands on exactly the next strategy, failing
once all are gone. Confirmed meaningful by temporarily perturbing the order in
`Resolve` and watching it fail.

## Review follow-up

`@reviewer` found TASK-7 PARTIAL: `cmdErrorText`'s three-line resolution
diagnosis was wired into `detailView`'s footer, but `bodyHeight()` hardcoded a
single-row footer, so the extra two lines pushed the total rendered height
past the alt-screen viewport — clipping exactly the "fix:" line the task was
meant to surface, whenever an agent-launch resolution failed from the detail
view.

Fixed in `tui/internal/ui/detail.go` and `tui/internal/ui/app.go`:
`bodyHeight()` now subtracts the actual footer line count (a new
`footerLines()` helper, `strings.Count(d.status, "\n") + 1`) instead of a
constant, and a new `setStatus()` method resizes the viewers whenever the
status changes so the body always shrinks to make room. `app.go`'s three call
sites that wrote `a.detail.status` directly now go through `setStatus()`.
Regression test: `tui/internal/ui/detail_test.go`
(`TestDetailBodyHeightShrinksForMultiLineStatus`) renders a full-viewport body
with both a one-line and a three-line status and asserts the total row count
never exceeds the viewport height, and that the "fix:" line survives in the
rendered output.

Verified with `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...`
(all green).

## Known issues / follow-ups

- **Q1 stayed as decided in the handover.** Only `sc-job` and `sc-done` are
  installed; no bare `sc`, no `sc-tui`. Adding them is one line in the
  Makefile's `LINKS` list. The README's alias example uses `sc` and `sc-tui`
  as *aliases*, which is exactly the case the env-var overrides exist for.
- **`resolve.Done()` has no caller yet.** The TUI has no finish-job action
  (`docs/TASKS.md` still lists "New job shortcut" and friends). The spec and its
  `SAFECODE_DONE_BIN` contract are in place for when that action is added.
- **`docs/CLAUDE.md` is a 0-byte file** although TASK-12 lists it. Nothing to
  sync into it. Either delete it or fill it in — `docs/AGENTS.md` is the real
  context file and `run.sh` mounts that. Not touched here.
- **`project-template/docs/AGENTS.md` and `agents/*.md` mention none of the
  renamed commands**, so TASK-12 left them alone. Verified by grep.
- **No `.dockerignore` exists**, so the build context includes `.env`. Nothing
  `COPY`s it, so it does not reach the image, but adding a `.dockerignore` is
  worth a separate chore job (noted in the previous session's handover too).
- **The image grew by roughly 500 MB** (`golang-go` plus the pre-warmed module
  cache) and everyone must `make rebuild` once. If that proves too costly, the
  alternative is the official Go tarball with a pinned version, sketched in
  `handover.md` §2.
