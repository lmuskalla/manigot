# Tasks: adjust tui to be dynamic

id: bvi7n6
status: open
analyst: @analyst
date: 2026-08-08

<!-- Produced by @analyst from brief.md. -->

## Problem as understood

Two separate problems, both rooted in the same assumption:

1. **Hardcoded command names.** The TUI resolves host commands by literal name
   on `$PATH`: `hostcmd.NewJob` does `exec.LookPath("new-job")`, and
   `launch.shellCommand` builds the literal string `manigot --agent … --job …`.
   If a user installed the launchers under any other name — or via shell
   aliases (`mg`, `mg-job`, `mg-done`) — the TUI cannot find them.
2. **`new-job` is too generic a global command name.** A tool that installs a
   binary called `new-job` into `/usr/local/bin` is squatting on a name that
   says nothing about manigot. Same for `finish-job`.

**Critical technical constraint the developer must know:** shell aliases
(`alias mg-job=…` in `~/.zshrc`) are *not* reachable from the TUI. Aliases exist
only in interactive shells; `exec.Command` and even `bash -lc` do not expand
them. So "support the user's aliases" cannot be implemented as alias lookup.
The dynamic resolution must go through env vars, config, or direct script paths.
Aliases remain a *user-facing* convenience only — the TUI must have its own way
to find the real scripts.

## Naming decision (settled)

| script | canonical name | short name | legacy name |
|---|---|---|---|
| `scripts/run.sh` | `manigot` | *(see Q1)* | — |
| `scripts/new-job.sh` | `manigot-job` | `mg-job` | `new-job` |
| `scripts/finish-job.sh` | `manigot-done` | `mg-done` | `finish-job` |
| `scripts/manigot-tui.sh` | `manigot-tui` | *(see Q1)* | — |

Both the canonical and the short name are first-class: `make install` creates
both, and the resolver accepts both. Legacy names stay in the resolver's
lookup order only, so existing installs keep working.

## Open questions

- **Q1 — short names for the other two commands.** `mg` for `manigot` and
  `mg-tui` for `manigot-tui` would be symmetrical, and the brief mentions
  `mg` as an existing personal alias — but you only specified `mg-job` and
  `mg-done`. `mg` is a short, collision-prone name to claim in `/usr/local/bin`.
  **Not guessing.** Tasks below install `mg-job`/`mg-done` only; adding `mg`
  later is a one-line change to TASK-10.
- **Q2 — back-compat.** Do `new-job` / `finish-job` need a deprecation warning
  when invoked under the legacy name, or is silent acceptance enough? Tasks
  below assume silent acceptance by the resolver, no warning in the scripts.
- **Q3 — install mechanism.** Assumed in scope: a `make install` target
  (TASK-10). Can be dropped in favour of documentation only.
- **Q4 — Go in the image.** See TASK-0B: pre-warming the module cache makes
  the image self-sufficient offline but couples it to `tui/go.sum`.

## Task breakdown

### Pre-tasks — container toolchain

The container mounts the whole project at `/workspace` (`run.sh:218`), so an
agent working on this repo can see `tui/` and `Makefile` — but the image has
neither `make` nor Go, so it cannot build or test the TUI today. These come
first because TASK-1 onwards is Go code that needs `go test` to verify.

TASK-0A: Install `make` and the Go toolchain (≥1.23, per `tui/go.mod`) in the
Dockerfile, in the existing `apt-get` layer or a dedicated one.
     files: Dockerfile
     depends: none
     risk: medium — Debian trixie's `golang-go` satisfies ≥1.23, but this adds
     several hundred MB to an image everyone must rebuild; if a specific Go
     version is needed the official tarball is the alternative.

TASK-0B: Decide and, if agreed (Q4), pre-warm the Go module cache in the image
by copying `tui/go.mod` + `tui/go.sum` and running `go mod download`, so the
TUI builds inside the container without network access.
     files: Dockerfile
     depends: TASK-0A, Q4
     risk: medium — makes the image depend on `tui/go.sum`, so dependency
     bumps then require a `make rebuild`; without it, builds fail in any
     network-restricted container.

TASK-0C: Note the new toolchain and the required `make rebuild` in the README's
Rebuilding section.
     files: README.md
     depends: TASK-0A
     risk: low — documentation only; independent of the naming decision, so it
     can land before the rename block.

### Main work

TASK-1: Add a `resolve` package to the TUI that locates a manigot host command
via an ordered strategy — explicit env override → canonical name on `$PATH` →
short name on `$PATH` → legacy name on `$PATH` → repo-relative script
fallback — and returns an absolute path plus a description of how it was found.
     files: tui/internal/resolve/resolve.go (new), tui/internal/resolve/resolve_test.go (new)
     depends: TASK-0A
     risk: low — new isolated package, no existing behaviour touched; pure
     path logic that is easy to unit test.

TASK-2: Define the env var contract for overrides and document it in the
package doc comment: `manigot_BIN`, `manigot_JOB_BIN`, `manigot_DONE_BIN`,
plus `manigot_HOME` for the repo-relative fallback.
     files: tui/internal/resolve/resolve.go
     depends: TASK-1
     risk: low — naming/documentation only, but it is a public contract, so
     changing it later is a breaking change for users.

TASK-3: Make `scripts/manigot-tui.sh` export `manigot_HOME` (it already
computes `ROOT`) so the TUI can fall back to `$manigot_HOME/scripts/*.sh` when
nothing is on `$PATH`.
     files: scripts/manigot-tui.sh
     depends: TASK-2
     risk: low — one exported variable in an existing wrapper.

TASK-4: Add a secondary fallback in the TUI that derives the repo root from
`os.Executable()` so a directly-invoked `bin/manigot-tui` (no wrapper script)
still finds `scripts/`.
     files: tui/internal/resolve/resolve.go, tui/main.go
     depends: TASK-3
     risk: medium — `os.Executable()` behaviour differs under symlinks and
     `go run`; needs explicit handling of both.

TASK-5: Rewrite `hostcmd.NewJob` to call the resolved absolute path instead of
`exec.LookPath("new-job")`, keeping the existing cwd/`$PWD` handling.
     files: tui/internal/hostcmd/hostcmd.go, tui/internal/hostcmd/hostcmd_test.go
     depends: TASK-1
     risk: low — single call site, existing test already covers the
     not-found path and needs updating to the new error text.

TASK-6: Rewrite `launch.shellCommand` to embed the resolved absolute manigot
path (still shell-quoted) rather than the bare word `manigot`.
     files: tui/internal/launch/launch.go, tui/internal/launch/launch_test.go
     depends: TASK-1
     risk: medium — the string is interpolated into `osascript` and
     `bash -lc`; quoting must stay correct for paths containing spaces.

TASK-7: Improve the failure message shown in the TUI footer when a command
cannot be resolved, listing the strategies tried and the env var to set.
     files: tui/internal/ui/app.go, tui/internal/resolve/resolve.go
     depends: TASK-5, TASK-6
     risk: low — user-facing text only.

TASK-8: Populate the resolver's candidate lists with the settled names —
`manigot-job` / `mg-job` / `new-job` and `manigot-done` / `mg-done` /
`finish-job` — in that priority order.
     files: tui/internal/resolve/resolve.go
     depends: TASK-1
     risk: high — this is the change that can break every existing
     installation; the legacy fallback must be explicit and tested.

TASK-9: Update the usage headers and self-referencing usage/error strings
inside the scripts (`new-job "…"` → `manigot-job "…"`, `finish-job <id>` →
`manigot-done <id>`), including `new-job.sh:15` and `finish-job.sh:14`.
     files: scripts/new-job.sh, scripts/finish-job.sh
     depends: TASK-8
     risk: low — comments and usage strings only; the script *filenames* stay
     as they are, only the installed command names change.

TASK-10: Add `make install` / `make uninstall` targets that symlink both the
canonical and short names (`manigot-job` + `mg-job`, `manigot-done` +
`mg-done`, `manigot`, `manigot-tui`) under a configurable `PREFIX`, and make
`make tui` print the new install hint.
     files: Makefile
     depends: TASK-8, Q3
     risk: medium — writes to a location outside the repo (`/usr/local/bin`
     by default); must not run as an implicit dependency of another target.

TASK-11: Update the README install section, the TUI section, and the workflow
examples to the new command names, and add a short "installing without
symlinks" subsection covering aliases + the env var overrides.
     files: README.md
     depends: TASK-8, TASK-10
     risk: low — documentation, but it is the primary install instruction so
     errors are costly.

TASK-12: Sync the project context files with the new command names, per the
"keep them in sync" hard rule.
     files: docs/AGENTS.md, docs/CLAUDE.md, project-template/docs/AGENTS.md
     depends: TASK-11
     risk: low — documentation sync; the risk is *missing* a file, not
     breaking one.

TASK-13: Update `docs/TASKS.md` references to the renamed commands.
     files: docs/TASKS.md
     depends: TASK-11
     risk: low — two occurrences.

TASK-14: Add an end-to-end resolution test that exercises the full order
(env override wins → canonical name → short name → legacy name → script
fallback) using a temp dir on `$PATH`.
     files: tui/internal/resolve/resolve_test.go
     depends: TASK-1, TASK-4, TASK-8
     risk: low — test-only, but it is the main guard against silent
     regressions in the resolution order.

## Explicitly out of scope

- Windows support (already excluded for the TUI generally).
- Converting `manigot` into a subcommand dispatcher, unless Q1 is answered
  that way — in which case this task list needs revising, not extending.
- Any change to how the container itself is launched or authenticated.
- A config file (`~/.config/manigot/config.toml`). Env vars cover the stated
  need; a config file is a bigger surface for no described benefit.

## Suggested order

TASK-0A → 0B → 0C first — without them nothing downstream can be verified
inside the container, and 0A requires a `make rebuild` before the next session.

Then TASK-1 → 2 → 3 → 4 → 5 → 6 → 7, the rename block 8 → 9, the install and
docs block 10 → 11 → 12 → 13, and TASK-14 last.

Q1 (whether to also install `mg` and `mg-tui`) only affects TASK-10 and
TASK-11, so it does not block the start of the work.
