## Summary

Added an optional, purely cosmetic thematic naming layer on top of manigot's
existing technical vocabulary: two dispatcher aliases (`crew` for `agents`,
`made-man` for `jdi`) that exec the exact same underlying scripts, thematic
wording added alongside (not replacing) the existing `[INFO]`-style
banner/log lines in `scripts/run.sh` and `scripts/entrypoint.sh`, and
matching documentation updates in `docs/AGENTS.md` and `README.md`
(including a new, differently-voiced "How to get a job done" walkthrough).
No default behavior, output format, or architecture changed.

## Changes

TASK-1: Added `crew` and `made-man` as additional match arms in
`scripts/mg.sh`'s dispatch `case` statement (`agents|crew`, `jdi|made-man`),
each still `exec`ing the same sibling script (`agents.sh` / `jdi.sh`) with
args passed through unchanged. Updated the top-of-file `# ── Usage ──`
comment block to list the two aliases next to their technical counterparts,
and updated `print_help()`'s `Commands:` section to call out `mg crew` and
`mg made-man` as thematic aliases on their own line under the existing
`mg agents` / `mg jdi` entries — `agents`/`jdi` remain the documented primary
names, unmoved and unreordered.
File: `scripts/mg.sh`

TASK-2: Added one new line to the boxed `[INFO]` banner in `scripts/run.sh`
(written to fd 3, preserving the existing fd-3-vs-stdout split for
`--print` mode): "Entering safehouse (isolated session)..." — the brief's own
example phrase — placed right after the box header, before the existing
Project/Root/Docs/Profile/Tool/etc. fields, which are all unchanged.
File: `scripts/run.sh`

TASK-3: Added one new informational line in `scripts/entrypoint.sh`, printed
right before the final `exec claude`/`exec opencode` dispatch: "Starting
session — you're made, welcome to the crew." This line is skipped entirely
when `MANIGOT_PRINT=true` so it can never leak into the clean stdout stream
`--print` callers (e.g. `mg jdi`) parse as JSON. Existing error/warning
messages were left technical-only, as instructed — they're for a human to
act on, not to flavor. The `--dangerously-skip-permissions` /
`--print`/`--output-format json` logic below it was not touched.
File: `scripts/entrypoint.sh`

TASK-4: Updated `docs/AGENTS.md`'s existing `mg agents` and `mg jdi` entries
(both in the `Commands` list and the `scripts/agents.sh`/`scripts/jdi.sh`
Architecture bullets) to mention the new `crew`/`made-man` aliases as
optional, same-script/same-behavior alternates. No other command's
description was touched; `project-template/docs/AGENTS.md` and `README.md`
were left untouched per the brief/tasks scope (`README.md` documents
`mg agents`/`mg jdi` extensively but wasn't named in brief.md's scope — noted
here as a possible fast-follow rather than edited).
File: `docs/AGENTS.md`

TASK-5: Deleted `docs/NAMING_FEATURES.md` — it was scratch/inspiration input
for this brief describing a different (fictional) tool architecture
(`manigot run` with short flags, a `manigot.yaml` config schema, and a
multi-agent "crew" execution model), not accurate documentation of manigot's
actual architecture, and nothing in the codebase referenced it.
File: `docs/NAMING_FEATURES.md` (deleted)

TASK-6: Added directly after TASK-5, at the author's request in the same
session. Updated `README.md`'s existing `mg agents`/`mg jdi` references to
mention the `crew`/`made-man` aliases — the commands table, the `## Agents`
section's "or run `mg agents`..." sentence, and the `### Autonomous mode
(mg jdi)` intro — mirroring TASK-4's treatment of `docs/AGENTS.md`. Worded as
optional alternates only; `mg agents`/`mg jdi` remain the primary names
everywhere.
File: `README.md`

TASK-7: Also added at the author's request. Added a new "How to get a job
done" subsection under `## Job workflow` in `README.md`: a plain-language,
differently-voiced recap of the same numbered feature-flow steps already
documented just above it, using `mg crew`/`mg made-man` where they fit
naturally. Iterated on wording twice per author feedback — first to state
the alias relationship without dry "thematic alias for X" meta-commentary,
then to drop the "flavor only" framing sentence entirely and just narrate
the steps directly. No new command, flag, or step was introduced.
File: `README.md`

## Known issues / follow-ups

- None of the out-of-scope items from `brief.md` (a `manigot run` command,
  short flags, a `manigot.yaml` config file, the multi-agent "crew" execution
  model, or any `safehouse`/`operation`/`workflow` command surface) were
  implemented, as directed.
