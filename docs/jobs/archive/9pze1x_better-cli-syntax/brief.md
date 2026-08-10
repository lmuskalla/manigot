# Brief: Better cli syntax

status: done
type: feature
id: 9pze1x
branch: feature/9pze1x_better-cli-syntax
date: 2026-08-10
author: Leander Muskalla

## What

Unify manigot's six separate top-level commands (`mg`, `mg-job`, `mg-tui`, `mg-jdi`,
`mg-done`, `mg-delete`) into a single `mg` command with subcommands: `mg job`,
`mg tui`, `mg jdi`, `mg done`, `mg delete`. Bare `mg` (no subcommand, or any
args that aren't one of the five subcommand names — e.g. `--tool`, `--agent`,
`--job`, `--print`) keeps starting a session exactly as it does today.

This is a dispatch/renaming change only. None of the underlying scripts
(`run.sh`, `new-job.sh`, `finish-job.sh`, `tui.sh`, `jdi.sh`) change their
internal logic, flags, or behavior — they're wired behind a new dispatcher,
not rewritten.

## Why

Six commands with no shared namespace is inconsistent with the single-entry-point
pattern developers already know (`git`, `docker`, `kubectl`), makes the full
command surface harder to discover (no single `--help` lists everything), and
needlessly complicates `make install`/`make uninstall` (six symlinks instead
of one). This fixes discoverability and consistency for the actual user of
this tooling — the developer running these commands — without changing what
any command does.

## Out of scope

- Rewriting or migrating any underlying shell script (`run.sh`, `new-job.sh`,
  `finish-job.sh`, `entrypoint.sh`) to Go or any other language. Evaluated and
  explicitly rejected for this job — see Notes. `entrypoint.sh` in particular
  runs inside the container image itself, so touching it would mean bundling
  and building a Go binary into the `Dockerfile` for no benefit tied to this
  job's goal.
- Any change to `tui/`'s or `tui/cmd/jdi`'s existing Go logic beyond what's
  needed to keep them invocable as `mg tui` / `mg jdi`.
- Backward-compatible aliases for the old standalone names (`mg-job`,
  `mg-tui`, `mg-jdi`, `mg-done`, `mg-delete`). This is a clean cutover, not a
  deprecation period — no external consumers depend on the old names, and
  keeping both forms around indefinitely works against the exact
  inconsistency this job exists to remove.
- Any new functionality on any subcommand. Behavior of every command stays
  identical to today; only the entry point changes.

## Notes

**Dispatch design (decided, do not re-litigate in tasks.md):**
A single `mg` script/binary is the only thing `make install` symlinks onto
`$PATH`. It inspects the first positional argument:
- `mg job ...`    → today's `mg-job` (`new-job.sh`)
- `mg tui`        → today's `mg-tui` (`tui.sh`)
- `mg jdi ...`    → today's `mg-jdi` (`jdi.sh`)
- `mg done <id>`  → today's `mg-done` (`finish-job.sh`)
- `mg delete <id>`→ today's `mg-delete` (`delete-job.sh`)
- anything else (no args, or an arg starting with `--agent`/`--job`/`--tool`/
  `--print`) → today's bare `mg` behavior (`run.sh`), unchanged. The most
  common invocation (starting a session) must keep working exactly as-is,
  with no new required verb.

No collision risk: `run.sh`'s own arguments are always flags (`--agent`,
`--job`, `--tool`, `--print`), never a bare positional value, so there's no
ambiguity with the five subcommand names.

Required side effects of this change (in scope, not optional):
- `Makefile`'s `LINKS` list and `install`/`uninstall` targets collapse to a
  single `mg` symlink.
- `README.md` and `docs/AGENTS.md` (the canonical project-context source —
  edit the source, never the read-only mounts) both document the old
  six-command surface and need updating to the new `mg <subcommand>` syntax.

**Why the Go-migration question (raised in the original ask) isn't part of
this job:** it doesn't serve the problem above — the language behind `mg job`
is invisible to the person typing it, so it can't improve discoverability or
consistency. `run.sh` carries real, security-relevant logic today (`.env`
shadowing via `/dev/null` mounts, subscription-protection auth checks, the
`--print` fd-redirection trick) that a rewrite risks regressing, for no
user-facing gain tied to this job. If there's a concrete future driver for
migrating the shell layer (e.g. genuine cross-platform support bash can't
provide), it should be raised as its own brief and evaluated on those merits.
