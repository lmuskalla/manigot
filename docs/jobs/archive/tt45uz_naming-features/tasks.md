# Tasks: Naming features

id: tt45uz
status: open
analyst: @analyst
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Note on the earlier blocking questions

The blocking questions below this line were raised against an earlier,
incomplete draft of `brief.md` (see the original text, still visible in git
history / above the task breakdown in prior revisions) and against
`docs/NAMING_FEATURES.md` read as a literal spec. `brief.md` has since been
corrected and now directly answers Q1–Q5: it scopes this job to exactly two
subcommand aliases (`crew`→`agents`, `made-man`→`jdi`) plus thematic wording
alongside (not replacing) existing `[INFO]`-style log lines, explicitly
excludes the config-file subsystem, the multi-agent "crew" model, short
flags, and any `safehouse`/`operation` command surface, and confirms
`docs/NAMING_FEATURES.md` was scratch input to be deleted once this job's
changes land. TASK-1 from the prior pass (the resolve-questions task) is
therefore superseded and dropped. The task breakdown below replaces the
prior "provisional — safe subset only" one.

## Task breakdown

TASK-1: Add `crew` (alias of `agents`) and `made-man` (alias of `jdi`) cases
to `scripts/mg.sh`'s dispatch `case` statement, each `exec`ing the exact same
sibling script as its technical counterpart (`agents.sh` / `jdi.sh`
respectively) with args passed through unchanged — no new script, no
behavior change. Update `print_help()`'s `Commands:` section so `mg agents`
and `mg jdi` remain the documented primary entries, with the two aliases
called out alongside them as optional/thematic (e.g. a trailing "(alias:
crew)" / "(alias: made-man)" note or a short separate line), not replacing
or reordering the primary names. Update the top-of-file `# ── Usage ──`
comment block similarly if it enumerates subcommands.
files: scripts/mg.sh
depends: none
risk: medium — touches the dispatcher `case` that
`docs/jobs/archive/9pze1x_better-cli-syntax` deliberately simplified;
low complexity (two more `case` arms) but must not alter any existing
subcommand's matching/behavior, and must keep `agents`/`jdi` as the
documented primary names per the brief.

TASK-2: Add thematic wording alongside (not instead of) the existing
technical wording in `scripts/run.sh`'s `[INFO]`-style banner/log lines
(the diagnostic lines written to fd 3 — the boxed banner near the end of the
file and/or the "Note:"/"Warning:" lines earlier in the script), following
the brief's example phrasing (e.g. "Entering safehouse (isolated
session)..."). The existing technical wording and machine-relevant fields
(Project/Root/Docs/Profile/Tool/etc.) must remain unchanged — this only adds
text, it doesn't replace or restructure the banner.
files: scripts/run.sh
depends: none
risk: low — cosmetic log-text addition only, no flag/behavior/exit-code
change; the fd-3-vs-stdout split for `--print` mode (see comment at the top
of `scripts/run.sh`) must be preserved for any new line.

TASK-3: Add matching thematic wording alongside the existing technical
wording in `scripts/entrypoint.sh`'s log-style `echo` lines (e.g. around
session start, before the final `exec claude`/`exec opencode`), in the same
spirit as TASK-2. `scripts/entrypoint.sh` currently has few or no "banner"
lines (mostly error/warning messages) — if there is no natural existing line
to extend, add one short new informational line rather than restructuring
error-handling output; error/warning message text itself should stay
technical-only (these are for a human to act on, not to flavor).
files: scripts/entrypoint.sh
depends: none
risk: low — cosmetic log-text addition only; same fd/behavior caution as
TASK-2, and must not touch the `--dangerously-skip-permissions` /
`--print`/`--output-format json` logic below it.

TASK-4: Update `docs/AGENTS.md`'s existing `mg agents` and `mg jdi` entries
(in the `Commands` list and the `scripts/agents.sh`/`scripts/jdi.sh`
Architecture bullets) to mention the new `crew`/`made-man` aliases added in
TASK-1, worded as optional alternates — not a rename, not a new primary
name, and not a change to what those commands do. Do not touch `agents/*.md`
or any other command's description; do not touch `project-template/docs/AGENTS.md`
(it's a generic template for other projects, not documentation of manigot's
own commands, so it has nothing to update here). `README.md` also documents
`mg agents`/`mg jdi` extensively but is not named in `brief.md`'s scope — out
of scope for this task; flag to the author as a possible fast-follow rather
than editing it here.
files: docs/AGENTS.md
depends: TASK-1 (documents the aliases TASK-1 creates)
risk: low — documentation-only change, small and additive.

TASK-5: Delete `docs/NAMING_FEATURES.md` from the repository root.
files: docs/NAMING_FEATURES.md (deleted)
depends: TASK-1, TASK-2, TASK-3 (per `brief.md`'s Notes: delete only once
this job's changes are made, since it was this job's scratch/inspiration
input)
risk: low — removes a file nothing in the codebase references or imports;
purely a documentation-hygiene cleanup.

## Addendum — added mid-implementation at the author's direct request

`README.md` was flagged in TASK-4 as out of `brief.md`'s named scope (a
possible fast-follow, not edited there). The author then asked directly, in
the same session, to extend it too — TASK-6 below does that. TASK-7 is a
further direct request for a small thematic README addition (a "how to get
a job done" walkthrough) — still documentation-only, still just narrating
the existing flow with the same two aliases, no new command surface.

TASK-6: Update `README.md`'s existing `mg agents`/`mg jdi` references to
mention the `crew`/`made-man` aliases added in TASK-1, mirroring TASK-4's
treatment of `docs/AGENTS.md`: the commands table, the `## Agents` section's
"or run `mg agents`..." sentence, and the `### Autonomous mode (mg jdi)`
section's intro. Worded as optional alternates, not a rename.
files: README.md
depends: TASK-1 (documents the aliases TASK-1 creates)
risk: low — documentation-only change, small and additive.

TASK-7: Add a short thematic subsection to `README.md`'s `## Job workflow`,
"How to get a job done" — a plain-language recap of the same numbered
feature flow already documented just above it, narrated once more in
manigot's house tone and using the `mg crew`/`mg made-man` aliases where
they naturally fit. Adds no new command, flag, or step; purely a second,
differently-voiced walkthrough of the existing process.
files: README.md
depends: TASK-1, TASK-6
risk: low — documentation-only, additive; must not read as a spec for any
new command surface (see brief.md's Out of scope).

## Out of scope (confirmed by brief.md, not just provisional)

- Any `manigot run` command, short flags (`-s`/`-c`/`-o`), a `manigot.yaml`
  config file or its alias-resolution fallback logic, and the multi-agent
  "crew" (N parallel worker agents with per-agent models) execution model —
  all describe a different tool's architecture than manigot's; see brief.md
  Out of scope.
- Any new `safehouse`/`isolated`/`operation`/`workflow` flag or subcommand —
  `mg` has no existing sandboxing toggle to alias, and no single existing
  command `operation`/`workflow` maps onto the way `agents`/`jdi` do.
