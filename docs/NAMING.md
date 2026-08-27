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

Both aliases live in `cmd/mg/main.go`'s dispatch `switch` (`agents|crew`,
`jdi|made-man`) and run the exact same in-process command as their
technical counterpart — no behavior difference. `mg -h`
lists `agents`/`jdi` as the primary entries with the alias called out
alongside, never in place of it.

No other command has a thematic alias. In particular:

- **No `--safehouse`, `--crew`, `--operation`, or `--made-man` flags.**
  `mg` has no argparse-style flag layer to hang aliases off — it's a
  single dispatcher that inspects its first argument.
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

- the session launcher's boxed `[INFO]` banner, printed once per session,
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
  random line from `assets/quotes.json` — a flat, freely-editable list of
  Sopranos quotes and exclamations — in italics, followed by a blank line.
  the session launcher (internal/session) picks the quote at random on the
  host (once per session) and hands it to the container via the
  `MANIGOT_QUOTE` env var; a missing or empty file just means no quote that
  session. Skipped under `--print` (`MANIGOT_PRINT=true`) so it can never
  leak into the clean stdout stream `mg jdi` and other non-interactive
  callers parse as JSON. Error and warning messages elsewhere stay
  technical-only — they're for a human to act on, not to flavor. Edit
  `assets/quotes.json` directly to add, remove, or prune entries — there's
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
   under `--print`/`--output-format json` (see the session launcher's
   `--print` path) is technical wording only.
5. **New thematic surface needs its own brief.** This file documents what
   shipped in `tt45uz_naming-features`; it isn't a standing invitation to
   add more without scoping it against a real, existing 1:1 command first.

---

## The full rap sheet

Every Sopranos-flavored name currently in use, in one place:

- **manigot** — the project itself.
- **crew** (`mg crew`) — thematic alias of `mg agents`.
- **made-man** (`mg made-man`) — thematic alias of `mg jdi`.
- **safehouse** — flavor wording for an isolated session, in the session
  launcher's startup banner ("Entering safehouse (isolated session)...").
- **off the books** — flavor wording in the session launcher's banner
  `Docs` field when
  no `docs/` was found (no project context, no job workflow).
- **assets/quotes.json** — the flat, editable repository of Sopranos quotes
  `entrypoint.sh` prints one random line from each session.
- **assets/manigot.txt** — the ASCII logo (the three-window mark, with the
  `*#@*` censor-bleep glyphs), shown in the session banner and the TUI list
  header.
- **This Thing of Ours** — this document's title.

That's the whole list — see "The aliases" and "Flavor text" above for the
full detail on each.

---

## Parking lot (not scoped, not shipped)

Ideas floated but not yet brief'd per rule 5 above — written down so they
aren't lost, not a commitment that they'll land:

- **associate** — a single agent (e.g. one `@developer` session). Distinct
  from `made-man`, which specifically means having completed a full
  unattended `mg jdi` sequence — an ordinary one-off agent session hasn't
  earned that.
- **boss** — the TUI (`mg tui`). You survey all the jobs and hand them off
  to associates from there, which is just what a boss does.

If either gets picked up for real, it needs its own scoped brief like
`tt45uz_naming-features` before touching any script or doc.

---

## Where this is documented elsewhere

- `cmd/mg/main.go` — the subcommand `switch` and `printHelp()`'s
  `Commands:` section list both aliases next to their technical names.
- `docs/AGENTS.md` — the `Commands` list and the `mg agents`/`mg jdi`
  Command bullets mention the aliases.
- `README.md` — the commands table, the `## Agents` section, and the
  `### Autonomous mode (mg jdi)` intro mention the aliases; `## Job
  workflow` also carries a second, thematically-narrated walkthrough of the
  same steps ("How to get a job done") using `mg crew`/`mg made-man` where
  they fit naturally.
