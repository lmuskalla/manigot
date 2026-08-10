# Tasks: Naming features

id: tt45uz
status: open
analyst: @analyst
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Blocking scope questions (unresolved — not guessed)

`brief.md`'s Why / Out of scope / Notes sections are all still blank, and
this is a `feature`-type job, which per `docs/AGENTS.md`'s workflow should
normally pass through `@product-owner` before `@analyst`. That step appears
to have been skipped. Combined with the gap below, I'm not comfortable
turning `docs/NAMING_FEATURES.md` directly into a task breakdown — several
parts of it describe a different tool than the one in this repo, not an
extension of it.

1. **`docs/NAMING_FEATURES.md` assumes an architecture manigot doesn't
   have.** It specifies a Python CLI (`argparse`/`click`/`typer`), a
   `manigot.yaml` config file with alias-resolution fallback logic, and a
   `--crew`/`--agents` flag that takes an **integer count of worker agents,
   each with its own model** (e.g. `gpt-4o`). manigot is a Bash-dispatched
   (`scripts/mg.sh`) single-agent-CLI-per-session tool (Claude Code or
   OpenCode, one at a time) with no YAML config file anywhere in the
   codebase, and no concept of spawning N parallel agents with per-agent
   models. There is no existing `manigot run` command either — the closest
   analog, bare `mg`, takes no config file. **Is this document meant to be
   mapped conceptually onto manigot's actual CLI (see Q2/Q3), or is it
   describing net-new functionality (multi-agent "crew" execution, a YAML
   config subsystem) that doesn't exist yet and would need its own,
   separate brief?** I'm not scoping the latter here — it's a different job
   in both size and kind.
2. **Assuming a conceptual mapping (Q1 resolved as "reuse existing
   concepts"), does "Workflow" mean the existing job workflow (`mg job`,
   `docs/jobs/<id>/`), or something new (a YAML pipeline/DAG file, as
   `docs/NAMING_FEATURES.md` §2 implies)?** These are not the same thing —
   a job is a directory of four markdown files tracked through git, not an
   executable pipeline spec. If it means the existing job workflow, is
   `mg job` supposed to gain an `operation`/`workflow` alias, or is this
   only about *documentation/terminology* (calling it "an operation" in
   prose) rather than a new CLI surface?
3. **Does "Isolated Environment" (`safehouse`/`isolated`) need an actual new
   flag at all?** Every `mg` session is already sandboxed by definition —
   there's no existing flag that toggles sandboxing on/off (`--sandbox`
   doesn't exist today; isolation is the whole point of `mg`, not an
   option). If this item is in scope, what would `--safehouse` actually
   *do* that plain `mg` doesn't already do?
4. **Command-surface direction conflict:** `docs/jobs/archive/
   9pze1x_better-cli-syntax` very recently and deliberately *collapsed*
   manigot's command surface down to a single `mg` dispatcher with a small,
   fixed set of subcommands, specifically to reduce surface area. Adding a
   parallel set of thematic subcommand/flag aliases (`mg crew`, `mg
   safehouse`, `mg operation`, `mg made-man`, plus short flags like `-s`/
   `-c`/`-o`) pulls in the opposite direction. Is that reversal intentional
   and wanted here, or should thematic naming stay confined to prose/docs
   (banners, log lines, README sections) rather than the actual command
   surface?
5. **`docs/NAMING_FEATURES.md` lives at `docs/NAMING_FEATURES.md`, not
   under this job's own directory.** Is it meant to be committed there
   permanently as project documentation (in which case it needs to be
   corrected to describe manigot's actual architecture per Q1–Q3 before
   merge, since as written it would mislead a future reader), or was it
   only meant as this job's spec/scratch input (in which case it likely
   belongs under `docs/jobs/tt45uz_naming-features/` instead, or should be
   deleted once superseded by `tasks.md`)?

None of TASK-1+ below are final. They cover only the narrow slice that
holds up regardless of how Q1–Q5 are answered — everything else should
wait for a decision rather than be guessed.

## Task breakdown (provisional — safe subset only)

TASK-1: Resolve Q1–Q5 above with the author/`@product-owner` and fill in
`brief.md`'s Why / Out of scope / Notes sections accordingly, before any
further task is started or re-scoped.
files: docs/jobs/tt45uz_naming-features/brief.md
depends: none
risk: low — a conversation/writing task, not a code change; blocking
everything else is the point.

TASK-2 (only once Q4 confirms *some* command-surface change is wanted, and
Q3 clarifies what `--safehouse`/`-s` would actually toggle beyond what `mg`
already does): Add thematic prose only (no new flags/subcommands) — update
`[INFO]`-style banner/log lines in `scripts/run.sh` (the boxed banner
around line 291) and `scripts/entrypoint.sh` to use the thematic terms from
`docs/NAMING_FEATURES.md` §3.1 (e.g. "Entering safehouse container...")
alongside the existing technical wording, as the lowest-risk, unambiguous
reading of the spec's "Documentation & Logging Guidelines" section — this
part of the doc matches manigot's real architecture with no reinterpretation
needed.
files: scripts/run.sh, scripts/entrypoint.sh
depends: TASK-1
risk: low — cosmetic log-text change only, no behavior/flag change; still
gated on TASK-1 because even this should be confirmed as wanted rather than
assumed.

TASK-3 (only once Q4 confirms new subcommand aliases are wanted): Add
`crew` (alias of `agents`) and `made-man` and/or `sunday-gravy` (alias of
`jdi`) cases to `scripts/mg.sh`'s dispatch `case` statement, exec'ing the
same `agents.sh`/`jdi.sh` scripts the existing names do, plus a `mg -h`
help-text update listing the aliases. These two are the only items from
`docs/NAMING_FEATURES.md`'s table with an unambiguous 1:1 existing
counterpart (`mg agents`, `mg jdi`) — `safehouse` (Q3) and `operation`/
`workflow` (Q2) do not have one and are explicitly excluded from this task
pending those answers.
files: scripts/mg.sh
depends: TASK-1
risk: medium — touches the dispatcher that was just deliberately simplified
in `9pze1x_better-cli-syntax`; needs a clear "yes, expand it again" decision
(Q4), not just technical correctness, and must not change existing
subcommand behavior.

## Explicitly not scoped here (pending Q1)

Anything implying a new config-file subsystem (`manigot.yaml` and its
alias-resolution fallback logic, §2 of `docs/NAMING_FEATURES.md`), a
multi-agent "crew" execution model with per-agent models, or a
`--operation`/`--workflow` YAML pipeline file — these describe capabilities
manigot does not have today. If confirmed as wanted (Q1), they need their
own brief and their own `@analyst` pass sized for genuinely new
architecture, not a handful of tasks bolted onto this one.
