# Verdict: Naming features

id: tt45uz
status: reviewed
reviewer: @reviewer
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `scripts/mg.sh` gains `agents|crew` and `jdi|made-man` case arms
(both still `exec`ing the original sibling scripts, args untouched), the
`# ── Usage ──` comment block lists both aliases, and `print_help()` calls
out `mg crew`/`mg made-man` on their own indented line under the unmoved
`mg agents`/`mg jdi` primary entries. `bash -n` passes. Matches brief and
tasks.md exactly.

TASK-2: PASS
notes: `scripts/run.sh` adds exactly one line, "Entering safehouse (isolated
session)...", inside the existing boxed banner, written to fd 3 like every
other line in that block — the fd-3/stdout split for `--print` is preserved.
No existing field changed.

TASK-3: PASS
notes: `scripts/entrypoint.sh` adds one new line ("Starting session — you're
made, welcome to the crew.") guarded by `if [[ "${MANIGOT_PRINT:-false}" !=
"true" ]]`, confirmed `MANIGOT_PRINT` is the same env var `run.sh` exports
(`-e MANIGOT_PRINT="$PRINT"`) and the same one the existing
`--output-format json` branch already checks, so it can't leak into a
`--print` caller's clean stdout. Error/warning text left untouched, as
instructed. `--dangerously-skip-permissions`/`--print` logic below is
untouched.

TASK-4: PASS
notes: `docs/AGENTS.md` — both the `scripts/agents.sh`/`scripts/jdi.sh`
Architecture bullets and the top-level `mg agents`/`mg jdi` Commands entries
now note the aliases as optional, same-script/same-behavior alternates.
`agents/*.md` and `project-template/docs/AGENTS.md` correctly left untouched
(confirmed via diff — zero changes under `agents/` or `project-template/`).

TASK-5: PASS
notes: `docs/NAMING_FEATURES.md` deleted in its own commit (`ce7b488`),
confirmed absent from the working tree; commit sequencing correctly places
it after TASK-1–3 land, per tasks.md's stated dependency.

TASK-6: PASS
notes: `README.md`'s commands table, `## Agents` section sentence, and the
`### Autonomous mode (mg jdi)` intro all now mention `crew`/`made-man` as
thematic alternates, mirroring TASK-4 exactly (verified via `git show
eb45d16` — touches only the described lines).

TASK-7: PASS
notes: New `### How to get a job done` subsection added under `## Job
workflow`, narrating the same steps as the numbered list just above it in
house tone, using `mg crew`/`mg made-man` where they fit. Introduces no new
command, flag, or step. Iterated per addendum (renamed from "How the crew
gets a job done", reworded, meta-framing dropped) — final text reads as
plain narration, not a spec. Addendum to tasks.md and implementation.md both
committed in their own commits, correctly documenting the author's
mid-session scope addition.

## Security

None run — no security-relevant surface touched (no new script, no new
network/exec/permission logic; the `MANIGOT_PRINT` guard in TASK-3 was
checked above and correctly reuses the existing var/behavior).

## Overall

APPROVED, with one blocker that is **not attributable to this job's task
work** but must be resolved before this branch is merged:

- **This branch's history is contaminated with a full, unrelated,
  already-archived job (`v6h2us_commit-feature-branches`) that was never
  merged into `main`.** `git diff main...HEAD` includes ~800 lines that
  have nothing to do with naming: `tui/internal/git/git.go`'s new `Push`
  function, `tui/internal/git/push_test.go`, `tui/internal/ui/app.go`,
  `tui/internal/ui/push_test.go`, `docs/jobs/archive/v6h2us_commit-feature-branches/*`,
  and one unrelated row in `README.md`'s detail-view keybinding table (`P`
  — push branch to origin). I traced this to commit `6a605d8` ("Commit
  feature branches"), which predates this job's own scaffold commit
  (`ec59bfe`) by 4 minutes and is not an ancestor of current `main`
  (`git merge-base --is-ancestor 6a605d8 main` fails) — i.e. this branch
  was not cut cleanly from `main` as `docs/AGENTS.md` says `mg job` always
  does. None of tt45uz's own commits (`e328503` onward) touch any of these
  files — I confirmed `TASK-6`'s commit (`eb45d16`) only touches the exact
  README lines described in `implementation.md`, and the `P` row is
  present in `6a605d8`'s own diff, not any tt45uz commit.
  **Before this branch is merged, it needs to be rebased onto current
  `main` (or have the `6a605d8` commit and its follow-on merge dropped) so
  that merging `tt45uz` doesn't silently smuggle in an unrelated,
  independently-scoped TUI feature that was never part of this job's brief
  or review.** This is a merge-hygiene blocker, not a fault in the
  naming-features implementation itself — every task the developer
  actually did (TASK-1 through TASK-7) is correct, in scope, and
  individually committed as required.

What must change before merge:
1. Rebase/clean the branch so `git diff main...HEAD` contains only this
   job's changes (`scripts/mg.sh`, `scripts/run.sh`, `scripts/entrypoint.sh`,
   `docs/AGENTS.md`, `README.md`, the deletion of
   `docs/NAMING_FEATURES.md`, and this job's own `docs/jobs/tt45uz_*` files).
   No code change is required in any of those files — TASK-1 through
   TASK-7 are all correct as implemented.
