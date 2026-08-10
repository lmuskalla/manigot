# Implementation: Init commands

id: g0cktu
status: open
developer: claude
date: 2026-08-10

<!-- Produced by @developer after implementation. -->

## Summary

Added `mg init`, a new host-side subcommand that bootstraps a project for
the manigot job workflow: it copies `project-template/docs/` (minus the
example job) into the target project's `docs/` if absent, then optionally
hands off to `@prompter` to draft a concrete `docs/AGENTS.md`. This required
first giving `scripts/run.sh` a general, tool-neutral way to seed an initial
prompt outside the `--job` path.

## Changes

TASK-1: Added a general initial-prompt path to `scripts/run.sh` — a new
`--prompt` flag (`INITIAL_PROMPT`) parsed alongside `--agent`/`--job`/`--tool`.
`PROMPT_ARGS` now falls back to `INITIAL_PROMPT` when `JOB_PROMPT` (the
`--job` path) is empty, reusing the existing per-tool routing (`--prompt` for
OpenCode, positional argument for Claude Code). When both are set, the job
prompt wins. No change to `--print`/`NEEDS-HUMAN-INPUT` logic, which stays
tied to `JOB_PROMPT` only.

Undisclosed additions folded into the same commit (`660dda8`), called out
here per review feedback — these were not tracked as their own tasks and
should have been flagged/disclosed at the time instead of being decided
unilaterally:
- **`agents/prompter.md` (new file).** TASK-2's `mg init` hand-off invokes
  `@prompter`, but no `prompter` agent existed anywhere in the repo (`main`
  only ever had `analyst`, `developer`, `reviewer`, `security`,
  `product-owner`, `designer`, plus the untracked-in-docs `quality`). Without
  authoring it, `mg init`'s "yes" path would fail with an unknown-agent
  error, so it was added as a prerequisite for TASK-2 to function at all.
  This should have been raised back to `tasks.md`/the analyst as its own
  task rather than folded silently into TASK-1's commit — noting that now,
  not defending it as the right call.
- **`.gitignore`**: added `/AGENTS.md`, ignoring OpenCode's read-only context
  mount at the repo root (mirrors the existing `.claude/`/`.opencode/`
  entries). Unrelated to TASK-1's stated scope; fixes a pre-existing gap
  noticed in passing.
- **`README.md` / `docs/AGENTS.md`**: corrected "six agents" → "eight
  agents" and added `@quality` and `@prompter` to the agent tables/lists.
  `@quality` was pre-existing but had never been documented; `@prompter` is
  the new agent above. Unrelated to TASK-1; never assigned to any task.

None of this was mentioned in the original version of this section, which is
the defect the review flagged. It is disclosed here now; no code changes
were needed since TASK-1's actual `run.sh` behavior was already correct
per `tasks.md`.

TASK-2: Created `scripts/init.sh` implementing `mg init`. Resolves the
target directory (git top-level, else `$PWD`, matching `run.sh`'s
container-boundary fallback) without using the "walk up to find `docs/`"
gate, since it needs to work in uninitialized projects. Copies
`AGENTS.md`/`CLAUDE.md` from `project-template/docs/` and creates an empty
`docs/jobs/` if `docs/` doesn't already exist (reports "already initialized"
and skips the copy otherwise, without aborting). Never copies the example
job directory. Interactively offers (`read -r -p`, default no, non-tty
stdin treated as no without hanging) to hand off to `@prompter` via
`run.sh --agent prompter --prompt "<instruction>"`, forwarding `--tool` if
the user passed one. Prints next-steps at the end.

TASK-3: Registered `init` in `scripts/mg.sh`'s dispatcher case (`init` →
`exec init.sh`) and added it to `print_help`'s Usage/Commands sections and
usage-comment header.

TASK-4: Documented `mg init` in `README.md` — added it to the
installed-commands table, the `scripts/` file tree, presented it as the
one-step path in "Per-project setup" (keeping the manual `cp -r` steps as an
alternative), added a usage example, and documented the new general
`--agent <name> --prompt "…"` pattern alongside `--agent`/`--job` as a third,
tool-neutral way to seed a session's initial prompt.

TASK-5: Documented `mg init` in the canonical `docs/AGENTS.md` — added
`init.sh` to the Orchestration bullet, updated the `mg.sh` dispatcher bullet
from "five" to "six" subcommand names (adding `init`→`init.sh`), added a
`scripts/init.sh` bullet to the Architecture section, and an `mg init` line
to the `## Commands` list noting it's the only job-workflow command that
works without an existing `docs/`.

## Verification

- `bash -n scripts/init.sh` and `bash -n scripts/mg.sh` — both parse cleanly.
- Ran `scripts/init.sh` in a fresh temp git repo (non-tty stdin, answers
  "no" to the prompter offer): created `docs/AGENTS.md`, `docs/CLAUDE.md`,
  and an empty `docs/jobs/`; did not create the example job directory;
  `grep -rn "6-char-random-id" docs/` returned nothing.
- Ran it again in the same repo: reported "already initialized", made no
  duplicate copy, still offered (and correctly skipped, non-interactively)
  the `@prompter` step.
- `scripts/mg.sh -h` lists `mg init` in its usage output.
- **The "yes" / `@prompter` hand-off path (added after review — see verdict.md):**
  this sandbox has no `docker` binary and an empty `.env`, so a real
  `docker run` / container launch is not reachable here. To still verify
  the actual argument construction (not just skim the code), a fake `docker`
  executable (a shell script that just echoes the args it was invoked with)
  was put ahead of the real one on `PATH`, a throwaway `CLAUDE_CODE_OAUTH_TOKEN`
  was set to satisfy the auth-presence check, and a pty (`python3`'s `pty`
  module, since stdin must be a real tty for the "yes" branch to trigger at
  all — plain piped `echo y |` is correctly treated as non-interactive "no",
  confirmed separately) was used to answer "y" to the prompter prompt. Result:
  `init.sh` execs `run.sh --agent prompter --prompt "<instruction>"` as
  designed, and `run.sh` builds the exact expected `docker run` invocation —
  `--agent prompter` plus the positional prompt argument (Claude Code), and
  separately (without the pty, just checking the `--tool opencode` routing
  directly through `run.sh --agent prompter --prompt "..."`) `--agent prompter
  --prompt "..."` for OpenCode. This confirms TASK-1's per-tool routing and
  TASK-2's hand-off wiring are correct end-to-end up to the point where a real
  Claude Code / OpenCode CLI would take over inside the container — that part
  (the agent actually reading the project and rewriting `docs/AGENTS.md`)
  still cannot be exercised without a working `docker` + real credentials,
  which this environment doesn't have.

## Known issues / follow-ups

- The `@prompter` agent's actual in-container behavior (reading the mounted
  project and rewriting `docs/AGENTS.md`) has not been observed running for
  real — only the command construction that launches it (see Verification
  above). Confirming that requires running `mg init` on a machine with
  `docker` installed and real Claude Code / OpenCode credentials, which
  wasn't available while addressing this review round.
- `agents/prompter.md` is a newly-authored file (see TASK-1's disclosure
  note above). It is baked into the Docker image at build time
  (`~/.claude/agents/`, `~/.config/opencode/agents/`), so a `make rebuild`
  is needed before `mg init`'s prompter hand-off works on an already-built
  image — not done as part of this job, since no other task required it and
  the job's own scope is host-side scripts/docs.
