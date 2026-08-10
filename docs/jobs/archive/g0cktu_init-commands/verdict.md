# Verdict: Init commands

id: g0cktu
status: open
reviewer: claude
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PARTIAL
notes: The stated change itself is correct — `scripts/run.sh` gets `--prompt`/
`INITIAL_PROMPT`, `PROMPT_ARGS` falls back to it when `JOB_PROMPT` is unset,
per-tool routing (positional vs `--prompt`) and job-prompt-wins precedence are
implemented exactly as specified, and `--print`/`NEEDS-HUMAN-INPUT` is
untouched. Verified with `bash -n` and by reading the resulting logic; matches
tasks.md's TASK-1 spec.
However, commit `660dda8` ("TASK-1: add general --prompt flag to run.sh")
bundles in three changes that have nothing to do with that task and are not
mentioned anywhere in `implementation.md`:
  - **`agents/prompter.md` created from scratch** (a brand-new 75-line global
    agent definition). This file did not exist on `main` at all — `docs/AGENTS.md`
    on `main` lists only six agents (no `quality`, no `prompter`), even though
    `agents/quality.md` already existed untracked-in-docs on `main` too. Creating
    a whole new agent is a substantial addition that tasks.md never lists as a
    task (its scope notes talk only about `run.sh` prompt routing, never about
    the agent needing to be authored), and `implementation.md`'s "Changes"
    section for TASK-1 says only "Added a general initial-prompt path to
    `scripts/run.sh`" — it never discloses that a new agent file was created.
    Per the hard rule "When scope is unclear: ask, don't guess," and "Do not
    refactor things unrelated to the current task," this needed to be called
    out explicitly (as its own task, or at minimum flagged and documented),
    not silently folded into an unrelated commit. It happens to be functionally
    necessary — `mg init`'s prompter hand-off would fail with an unknown-agent
    error without it — which is presumably why it was added, but that
    justification belongs in `implementation.md` and arguably should have gone
    back to `tasks.md`/the analyst rather than being decided unilaterally.
  - **`.gitignore`**: adds `/AGENTS.md` (ignoring OpenCode's read-only context
    mount at the repo root). Reasonable fix, but unrelated to TASK-1's stated
    scope and never mentioned in `implementation.md`.
  - **`README.md` and `docs/AGENTS.md`**: both get "six agents" → "eight
    agents" corrected, adding `@quality` (pre-existing but never documented)
    and `@prompter` to the agent tables/lists — again reasonable content, but
    unrelated to "add `--prompt` to `run.sh`," undisclosed, and it also
    partially overlaps/pre-empts what TASK-4/TASK-5 are supposed to own (those
    later commits are clean and don't duplicate this, but the six→eight fix
    itself was never assigned to any task).
  None of this is called out anywhere in `implementation.md`'s "Changes" or
  "Known issues / follow-ups" sections (which says "none"). This is exactly
  the kind of undisclosed, unplanned scope the review process is meant to
  catch.

TASK-2: PASS
notes: `scripts/init.sh` matches the spec closely: resolves `SCRIPT_DIR` via
the standard symlink-following pattern, resolves target via git top-level
else `$PWD` with trailing slash stripped, skips the copy with an "already
initialized" message when `docs/` exists (does not abort), copies only
`AGENTS.md`/`CLAUDE.md` plus an empty `docs/jobs/` and never the example job
directory, offers the y/N prompter hand-off with a non-tty short-circuit that
doesn't hang, forwards `--tool` when given, and prints next-steps. Verified
by running it twice against a fresh temp git repo: first run creates
`docs/AGENTS.md`, `docs/CLAUDE.md`, and empty `docs/jobs/`; second run reports
"already initialized" with no duplicate copy; `grep -rn "6-char-random-id"`
on the result returns nothing. `bash -n scripts/init.sh` parses cleanly.
One gap: `implementation.md`'s own verification log only exercised the "no"
answer to the prompter offer — the "yes" path (actually launching
`run.sh --agent prompter --prompt ...`, per tasks.md's verification bullet
"`mg init --tool opencode` (answer yes) launches the prompter under OpenCode
with the instruction") was never confirmed end-to-end in this job's records.
Not a blocker on its own (full docker launch may be out of reach in this
environment), but combined with TASK-1's undisclosed creation of the
`@prompter` agent it does mean the core "hand off to @prompter" path this
whole feature exists for was never actually verified working.

TASK-3: PASS
notes: `scripts/mg.sh` gets the `init)` case execing `init.sh` with remaining
args, alongside the other five, plus `mg init` added to `print_help`'s Usage
comment and Commands list. Mechanical, matches the existing pattern exactly.

TASK-4: PASS
notes: `README.md` (commit `08a6f29`) adds `mg init` to the installed-commands
table, presents it as the one-step "Per-project setup" path while keeping the
manual `cp -r` steps as an alternative, adds a usage example, and documents
the tool-neutral `--agent <name> --prompt "…"` pattern as a third way (beyond
`--job`/`--agent`) to seed a session, including the job-prompt-wins
precedence note. Matches TASK-4 as specified. (The commit itself is clean —
the agent-count fix noted under TASK-1 landed in the earlier commit, not
here.)

TASK-5: PASS
notes: `docs/AGENTS.md` (commit `d060162`) updates the orchestration bullet,
the dispatcher description ("five" → "six" subcommands, adding
`init`→`init.sh`), adds a `scripts/init.sh` architecture bullet, and an
`mg init` line under `## Commands` noting it's the only job-workflow command
that works without existing `docs/`. Matches TASK-5 as specified; commit is
clean and scoped to this task.

## Security

Not run (`@security` not invoked for this job).

## Overall

<!-- APPROVED / REJECTED / NEEDS WORK -->

NEEDS WORK

Core functionality is solid and well-tested (init.sh's copy/skip/exclude
behavior, run.sh's --prompt routing, dispatcher/help/doc wiring all check out
against tasks.md and pass manual verification). The blocker is entirely about
disclosure and commit hygiene, not correctness of the shipped behavior:

- `implementation.md` must be updated to disclose the creation of
  `agents/prompter.md`, the `.gitignore` addition, and the "six → eight
  agents" doc corrections — all currently bundled into the TASK-1 commit with
  zero mention anywhere in the implementation record. At minimum this needs
  its own explanation (why `@prompter` didn't already exist and had to be
  authored as part of this job) since it's a new, unplanned agent definition,
  not a doc tweak.
- Confirm (and record in `implementation.md`) that the "yes" path of the
  `@prompter` hand-off — `mg init` actually launching `run.sh --agent
  prompter --prompt "..."` and starting a session — was exercised at least
  once, since that's the feature's actual payoff and it was never in this
  job's verification log.
- No code changes required for TASK-2/3/4/5 as they stand; once the above is
  addressed in `implementation.md` (a docs-only fix, no re-implementation
  needed for the already-correct commits), this should be approvable.
