# Tasks: Init commands

id: g0cktu
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Goal

Add `mg init` — a host-side setup command that bootstraps a project for the
manigot job workflow: copy `project-template/docs/` (minus the example job)
into the project if `docs/` is absent, then optionally hand off to the
`@prompter` agent to write a good project context in `docs/AGENTS.md`.

## Scope notes / assumptions (confirm before implementing)

The brief is explicit on intent but leaves a few decisions open. The tasks
below assume the most conservative reading; flag any you want to change.

1. **Target directory.** init must *create* `docs/`, so it cannot reuse the
   "walk up to find `docs/`" logic every other script uses. Assume the target
   is the **git root** when inside a repo, else `$PWD` — matching
   `run.sh`'s container-boundary fallback. (Alternatives: always `$PWD`, or
   walk up to the git root only.)
2. **`docs/` already present.** Assume: report "already initialized", **skip
   the copy**, and still offer the `@prompter` step (the copied `AGENTS.md`
   is a bare template, so help filling it in is useful either way).
   Alternative: abort entirely.
3. **"minus the example job".** Confirmed path to exclude:
   `project-template/docs/jobs/6-char-random-id_title-of-job/` (4 files). Keep
   an **empty `docs/jobs/`** directory in the destination so `mg job` is
   ready to go (it `mkdir -p`s each job dir, but the workflow expects
   `docs/jobs/` to exist).
4. **Prompter hand-off must work under both tools.** OpenCode takes an
   initial prompt via `--prompt`; Claude Code takes it as a positional.
   `run.sh` already knows this — it branches per tool — but only on the
   `--job` path (`JOB_PROMPT` is the sole source of `PROMPT_ARGS`). There is
   currently no way to give `run.sh` a free-form initial prompt for a
   non-job session that is guaranteed to route correctly under both tools.
   TASK-1 below adds that mechanism (a `--prompt` flag that reuses the
   existing per-tool routing); `mg init` then works identically whether the
   user runs Claude Code or OpenCode. (Clarified after review — OpenCode
   has no prompt limitation; the gap was purely in `run.sh`.)
5. **Files written into the user's project.** init only writes under the
   target's `docs/` (`AGENTS.md`, `CLAUDE.md`, `jobs/`) — compliant with the
   "never touch a project's files outside `docs/`" hard rule. It is a
   host-side command the user runs explicitly, not something invoked against
   a mounted project from inside manigot tooling.

## Task breakdown

<!-- TASK-1: description
     files: list of files likely affected
     depends: none
     risk: low / medium / high — reason

TASK-2: ...
-->

TASK-1: Add a general initial-prompt path to `run.sh` (enabler)
- files: `scripts/run.sh`
- depends: none
- risk: low — small, additive change that reuses the script's existing
  per-tool prompt routing; behavior is unchanged when no prompt is given.

Details:
- Add `INITIAL_PROMPT=""` and a `--prompt)` case to the arg parser
  (`INITIAL_PROMPT="$2"; shift 2`), mirroring `--agent`/`--job`/`--tool`.
- When building `PROMPT_ARGS`: today it is populated only from `JOB_PROMPT`
  (the `--job` path). Extend so that, when `JOB_PROMPT` is empty but
  `INITIAL_PROMPT` is set, the same per-tool routing applies
  (`--prompt "$INITIAL_PROMPT"` for opencode, the positional for
  claude-code). If both are set, prefer the job prompt (or concatenate —
  pick one and document it; job-only is simplest).
- No change to the `--print` / `NEEDS-HUMAN-INPUT` sentence logic (that's
  job-specific and stays tied to `JOB_PROMPT`).
- Net effect: `mg --agent prompter --prompt "…"`, and `mg --tool opencode
  --agent prompter --prompt "…"`, both start the agent with that prompt.

TASK-2: Create `scripts/init.sh` implementing the `mg init` subcommand
- files: `scripts/init.sh` (new)
- depends: TASK-1 (uses the new `--prompt` routing for the prompter hand-off)
- risk: medium — new script that copies files into the user's project and
  runs an interactive prompt; copy-exclusion and target-dir resolution need
  to be right.

Details:
- `set -euo pipefail`; reuse `resolve_script_dir` (copy the pattern from
  `run.sh`/`mg.sh`) to find `SCRIPT_DIR`, then
  `MANIGOT_ROOT="$(dirname "$SCRIPT_DIR")"` and
  `TEMPLATE_DIR="$MANIGOT_ROOT/project-template/docs"`. Fail loudly if
  `$TEMPLATE_DIR` is missing.
- Resolve target: git top-level if `git rev-parse --show-toplevel` succeeds,
  else `$PWD`. Strip trailing slash.
- If `$TARGET/docs` exists: print "already initialized" and skip copy.
  Otherwise: copy `AGENTS.md` and `CLAUDE.md` from `$TEMPLATE_DIR` into
  `$TARGET/docs/`, and create `$TARGET/docs/jobs/` (empty). Do **not** copy
  `$TEMPLATE_DIR/jobs/6-char-random-id_title-of-job/`.
- Interactive prompt: `read -r -p` a yes/no "Generate a project prompt with
  @prompter? [y/N]". Default no. Non-interactive (non-tty) stdin → treat as
  no, with a note (don't hang).
- On yes: `exec "$SCRIPT_DIR/run.sh" --agent prompter --prompt
  "<instruction>"`, where the instruction tells the prompter to read the
  project and write a good project context into `docs/AGENTS.md` (run.sh
  mounts `$TARGET` at `/workspace`, so refer to `/workspace/docs/AGENTS.md`).
  Forward `--tool` if the user passed one (e.g. `mg init --tool opencode`)
  so the hand-off respects their tool choice.
- Echo clear next-steps at the end (edit `docs/AGENTS.md`, then `mg job`).

TASK-3: Register `init` in the dispatcher and help text
- files: `scripts/mg.sh`
- depends: TASK-2
- risk: low — mechanical, follows the exact pattern of the five existing
  subcommand cases.

Details:
- Add `init)` to the `case` in `scripts/mg.sh` → `shift; exec
  "$SCRIPT_DIR/init.sh" "$@"`, alongside `job`/`tui`/`jdi`/`done`/`delete`.
- Add an `mg init` line to `print_help`'s Usage and Commands sections.
- Note: `init` must NOT use the "walk up to find `docs/`" gate — it creates
  `docs/`, so it works in uninitialized projects by design.

TASK-4: Document `mg init` in `README.md`
- files: `README.md`
- depends: TASK-2
- risk: low — doc-only.

Details:
- Add `mg init` to the installed-commands table (the `| command | does |`
  block under "The installed commands").
- Update the "Per-project setup" section: present `mg init` as the
  one-step path (replacing/augmenting the manual
  `cp -r project-template/docs/`), and keep the prompter-offer mention.
- Add a short example under "Usage".
- Document the new general pattern TASK-1 enables:
  `mg --agent <name> --prompt "…"` (tool-neutral — works with
  `--tool opencode` too). Mention it wherever `--agent`/`--job` are
  documented as the ways to seed a session, since `--prompt` is now a
  third, for ad-hoc non-job sessions.

TASK-5: Document `mg init` in the repo's own `docs/AGENTS.md` (keep in sync)
- files: `docs/AGENTS.md` (the canonical source mounted as the session
  context — NOT `project-template/docs/AGENTS.md`, which is a blank template
  for other projects and does not list manigot commands)
- depends: TASK-2
- risk: low — doc-only, but required by the "keep in sync" hard rule.

Details:
- Add `init` to the `scripts/mg.sh` dispatcher bullet (the five-name list:
  `job`/`tui`/`jdi`/`done`/`delete` → add `init`→`init.sh`).
- Add a `scripts/init.sh` bullet to the Architecture section's per-script
  list, and an `mg init` line to the `## Commands` list.
- Mention that `init` works without an existing `docs/` (unlike
  `job`/`jdi`/`tui`), since it creates it.

## Verification

- `bash -n scripts/init.sh` and `bash -n scripts/mg.sh` parse cleanly.
- TASK-1 sanity: `mg --agent prompter --prompt "say hi"` and
  `mg --tool opencode --agent prompter --prompt "say hi"` both start the
  agent with the prompt (no `--job` involved).
- `mg init` in a temp empty git repo: creates `docs/AGENTS.md`,
  `docs/CLAUDE.md`, empty `docs/jobs/`, and does **not** create the example
  job dir.
- `mg init` again in the same repo: reports "already initialized", no
  duplicate copy, still offers the prompter.
- `mg init --tool opencode` (answer yes) launches the prompter under
  OpenCode with the instruction.
- `mg -h` lists `init`; `mg init` (unknown-arg passthrough) does not break.
- `grep -rn "6-char-random-id"` on a freshly-initialized target returns
  nothing.
