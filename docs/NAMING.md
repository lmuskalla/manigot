# This Thing of Ours: manigot's Naming Scheme

## Overview

`manigot` is an anti-hype, Sopranos-themed AI agent orchestrator. Its
documented command surface is entirely technical (`mg agents`, `mg jdi`,
`mg job`, ...) — that vocabulary is what every script, help text, and doc
treats as primary. On top of it sits a small, deliberately narrow layer of
optional Sopranos-flavored alternates: a couple of subcommand aliases and a
few log lines, added purely for tone. Nothing about default behavior,
output, or architecture changes because of them, and nothing is required to
use them.

This scheme was scoped and shipped in
`docs/jobs/archive/tt45uz_naming-features/`. It replaces an earlier
`docs/NAMING_FEATURES.md`, which sketched a much larger dual-aliasing system
— short flags, a `manigot.yaml` config schema with alias-resolution
fallback logic, a multi-agent "crew" model spawning N worker agents each
with its own model — none of which describes manigot's actual architecture
(one agent CLI per session, no config file, no flag-based parser). That
document was scratch inspiration, not a spec, and was deleted once this
narrower scheme landed. This file describes what's actually implemented.

---

## The aliases

Only two commands got a thematic alias, because only two map cleanly,
1:1, onto a single existing command the way an alias needs to:

| Technical (primary, documented everywhere) | Thematic alias | What it does |
| :--- | :--- | :--- |
| `mg agents` | `mg crew` | Lists available agents, prompts for a pick, starts a session as that agent. |
| `mg jdi` | `mg made-man` | Drives a job's `@analyst` → `@developer` → `@reviewer` sequence unattended. |

Both aliases live in `scripts/mg.sh`'s dispatch `case` (`agents|crew`,
`jdi|made-man`) and `exec` the exact same underlying script as their
technical counterpart — no new script, no behavior difference. `mg -h`
lists `agents`/`jdi` as the primary entries with the alias called out
alongside, never in place of it.

No other command has a thematic alias. In particular:

- **No `--safehouse`, `--crew`, `--operation`, or `--made-man` flags.**
  `mg` has no argparse-style flag layer to hang aliases off — it's a
  single dispatcher that inspects `$1`.
- **No isolation toggle to alias as `safehouse`.** Isolation is the whole
  point of running `mg` at all, not an option you can turn off, so there's
  nothing for a `--safehouse`/`--isolated` pair to switch between.
- **No single `operation`/`workflow` command to alias.** manigot's flow
  (`mg job` → agents → `mg done`) is several distinct commands, not one
  pipeline entry point, so "operation" doesn't map onto anything.
- **No config file.** manigot has no `manigot.yaml` or equivalent, so
  there's no key-alias-resolution logic to write.

If a real need for any of the above surfaces later, it gets its own brief
scoped against what the alias would actually toggle — not bolted onto this
one.

---

## Flavor text

Three log lines carry Sopranos wording *alongside* (never replacing) their
existing technical wording. These are cosmetic only — not aliases, not
machine-readable fields:

- `scripts/run.sh`'s boxed `[INFO]` banner, printed once per session,
  opens with:
  ```
  Entering safehouse (isolated session)...
  ```
  followed by the unchanged Project/Root/Docs/Profile/Tool fields. When no
  `docs/` was found anywhere above `$PWD` (no project context, no job
  workflow — the container boundary falls back to the git root or `$PWD`
  instead), the `Docs` field reads:
  ```
  Docs    : (none — job workflow unavailable, running off the books)
  ```
- `scripts/entrypoint.sh`, right before it execs the agent CLI, prints a
  random line from `docs/quotes.json` — a flat, freely-editable list of
  Sopranos quotes and exclamations — in italics, followed by a blank line.
  `scripts/run.sh` picks the quote at random on the host (once per
  session) and hands it to the container via the `MANIGOT_QUOTE` env var;
  a missing or empty file just means no quote that session. Skipped under
  `--print` (`MANIGOT_PRINT=true`) so it can never leak into the clean
  stdout stream `mg jdi` and other non-interactive callers parse as JSON.
  Error and warning messages elsewhere in both scripts stay
  technical-only — they're for a human to act on, not to flavor. Edit
  `docs/quotes.json` directly to add, remove, or prune entries — there's
  no filtering logic, so anything left in the file is fair game to be
  printed.

There used to be a fourth: `entrypoint.sh` printing "Starting session —
you're made, welcome to the crew." on every session start. Dropped — "made"
only means something in the context of `made-man` (autonomous `mg jdi`
runs), and this line printed on every ordinary interactive session
regardless, which didn't track.

---

## Ground rules

1. **Technical names are the documented primary everywhere** — `mg -h`,
   `README.md`, `docs/AGENTS.md`. Thematic names are called out as optional
   alternates alongside them, never reordered or promoted above them.
2. **Aliases exec the same script, unchanged.** A thematic name is never an
   excuse to fork behavior — `mg crew` and `mg agents` must do the exact
   same thing forever, likewise `mg made-man` and `mg jdi`.
3. **Additive only, never required.** No script, flag, or output format
   depends on the thematic wording existing. If there's ever no appetite
   for it, dropping it costs nothing.
4. **Flavor text stays out of machine-readable output.** Anything written
   under `--print`/`--output-format json` (see `scripts/run.sh`) is
   technical wording only.
5. **New thematic surface needs its own brief.** This file documents what
   shipped in `tt45uz_naming-features`; it isn't a standing invitation to
   add more without scoping it against a real, existing 1:1 command first.

---

## The full rap sheet

Every Sopranos-flavored name currently in use, in one place:

- **manigot** — the project itself.
- **crew** (`mg crew`) — thematic alias of `mg agents`.
- **made-man** (`mg made-man`) — thematic alias of `mg jdi`.
- **safehouse** — flavor wording for an isolated session, in `run.sh`'s
  startup banner ("Entering safehouse (isolated session)...").
- **off the books** — flavor wording in `run.sh`'s banner `Docs` field when
  no `docs/` was found (no project context, no job workflow).
- **docs/quotes.json** — the flat, editable repository of Sopranos quotes
  `entrypoint.sh` prints one random line from each session.
- **This Thing of Ours** — this document's title.

That's the whole list — see "The aliases" and "Flavor text" above for the
full detail on each.

---

## Where this is documented elsewhere

- `scripts/mg.sh` — the `# ── Usage ──` header comment and `print_help()`'s
  `Commands:` section list both aliases next to their technical names.
- `docs/AGENTS.md` — the `Commands` list and the `scripts/agents.sh`/
  `scripts/jdi.sh` Architecture bullets mention the aliases.
- `README.md` — the commands table, the `## Agents` section, and the
  `### Autonomous mode (mg jdi)` intro mention the aliases; `## Job
  workflow` also carries a second, thematically-narrated walkthrough of the
  same steps ("How to get a job done") using `mg crew`/`mg made-man` where
  they fit naturally.
