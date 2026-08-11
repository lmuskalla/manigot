# Brief: Naming features

status: done
type: feature
id: tt45uz
branch: feature/tt45uz_naming-features
date: 2026-08-10
author: Leander Muskalla

## What

Add an **optional, thematic naming layer** on top of manigot's existing
technical vocabulary. Nothing about default behavior, output, or
architecture changes — this only adds a couple of alternate names a user can
choose to type/read instead of the technical ones, side by side with them.

Concretely, in this job:

- Add two subcommand aliases to `scripts/mg.sh`'s existing dispatch `case`,
  pointing at the exact same scripts as their technical counterparts (no new
  scripts, no new behavior): `crew` → same as `agents`, `made-man` → same as
  `jdi`. `mg -h` continues to list `agents`/`jdi` as the primary names, with
  the thematic aliases called out as optional flavor, not a replacement.
- Add matching thematic wording alongside (not instead of) the existing
  technical wording in the `[INFO]`-style banner/log lines in
  `scripts/run.sh` and `scripts/entrypoint.sh` (e.g. "Entering safehouse
  (isolated session)...").

`docs/NAMING_FEATURES.md`'s "Core Metaphor Mapping" table is the idea source
for terms — but only the two rows above (`Agents` ↔ `crew`, `Fully
Autonomous Mode` ↔ `made-man`) have an unambiguous, existing 1:1 command to
attach to. The rest of that document — a `manigot run` command with `-s`/
`-c`/`-o` short flags, a `manigot.yaml` config schema with alias-resolution
fallback logic, and "crew" as a *count of parallel worker agents each with
their own model* — describes a different tool's architecture (a Python CLI
with a YAML pipeline config) that manigot doesn't have. That part of the
document is not a spec for this job; see Out of scope.

## Why

Purely cosmetic — manigot's tone is meant to be a bit more irreverent than
generic AI-tool boilerplate, and a couple of the most-used commands should
optionally be able to reflect that. It has to stay genuinely optional: the
technical names (`agents`, `jdi`) remain the documented defaults everywhere,
nothing existing is renamed or deprecated, and no script, flag, or output
format is *required* to change for this to ship. If there turns out to be no
appetite for even this small a surface change, dropping it costs nothing —
no user-facing workflow depends on it.

## Out of scope

- Any new command surface for "workflow"/"operation" or "isolated
  environment"/"safehouse" as their own flags or subcommands. `mg` has no
  existing flag that toggles sandboxing — isolation is the whole point of
  `mg`, not an option — and "operation" doesn't map onto one single existing
  command the way `agents`/`jdi` do. If a real need for either surfaces
  later, it needs its own brief that can point at what the alias would
  actually toggle.
- `docs/NAMING_FEATURES.md`'s config-file subsystem (`manigot.yaml`, and its
  alias-resolution fallback logic) — manigot has no config file today, and
  this job isn't adding one.
- `docs/NAMING_FEATURES.md`'s multi-agent "crew" model (spawning N worker
  agents, each with its own model) — manigot runs one agent CLI per session.
  A parallel multi-agent execution model would be a much larger, separate
  feature with its own brief, not something bolted onto a naming job.
- Short flags (`-s`, `-c`, `-o`) and a `manigot run` command — not part of
  this job; the dispatcher only gains the two subcommand words above.

## Notes

- `docs/NAMING_FEATURES.md` was written against a different (fictional)
  architecture than manigot's actual one — see `@analyst`'s original
  blocking questions in `tasks.md` for the specifics that prompted this
  rewrite. Once this job's changes are made, delete
  `docs/NAMING_FEATURES.md` from the repo root as part of the same job — it
  was scratch/inspiration input for this brief, not accurate project
  documentation, and leaving it in place would mislead a future reader
  about manigot's architecture.
- This job does not touch `agents/*.md` or `docs/AGENTS.md`'s command
  descriptions beyond documenting the two new aliases where `mg agents`/
  `mg jdi` are already documented — everything else about how they behave
  is unchanged.
- `@analyst`: `tasks.md` already has a task breakdown written before this
  brief was corrected. TASK-1 (resolve the blocking questions) is now
  superseded by this brief — the questions it raised are answered above.
  Re-check TASK-2 and TASK-3 against the scope above; they should still
  stand largely as written, plus a new task to delete
  `docs/NAMING_FEATURES.md`.
