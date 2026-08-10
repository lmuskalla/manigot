# manigot

Isolated agent environment per project: one Docker image, subscription
billing via mounted OAuth credentials, real filesystem containment, and a
structured brief → tasks → implementation → verdict job workflow. Vendor-agnostic
— the same image runs either Claude Code (default) or OpenCode, chosen per
session with `mg --tool claude-code|opencode`.

## Stack
- Runtime: Docker (single image, built from `Dockerfile`)
- Agent CLIs: Claude Code (`claude`) and OpenCode (`opencode`), both installed in the image
- Orchestration: Bash scripts in `scripts/` (`run.sh`, `new-job.sh`, `finish-job.sh`,
  `tui.sh`, `jdi.sh`, `entrypoint.sh`)
- Build/CLI: `Makefile` (`make build`, `make rebuild`, `make install`, `make tui`, `make jdi`)
- Host-side TUI: Go, in `tui/` — built with `make tui`, never runs in the container
- Autonomous mode: Go, in `tui/cmd/jdi` (same module as the TUI) — built with
  `make jdi` into `mg-jdi`, also host-side, never runs in the container
- Agent definitions: Markdown files in `agents/`, baked into the image at build time

## Architecture
- `Dockerfile` — builds the image; installs both agent CLIs. Rebuild after a
  Claude Code or OpenCode update via `make rebuild`.
- `scripts/run.sh` — container launcher, symlinked as `mg` in PATH. Mounts
  the current project's `docs/` into the container; nothing else on the host
  is reachable from inside. Mount target depends on the tool:
  `/workspace/.claude` for Claude Code, `/workspace/.opencode` for OpenCode.
  Validates auth per tool and passes the choice on as `manigot_TOOL`.
- `scripts/new-job.sh` — installed as `mg-job`. Creates a new job directory
  under `docs/jobs/<id>_<slug>/` and a matching git branch, always branched from
  `main` (regardless of the branch the user is currently on).
- `scripts/finish-job.sh` — installed as `mg-done`. Archives a finished job.
- `scripts/tui.sh` — installed as `mg-tui`; wrapper around
  `bin/manigot-tui` that exports `manigot_HOME` so the TUI can find the scripts.
- `scripts/jdi.sh` — installed as `mg-jdi`; wrapper around `bin/manigot-jdi`,
  mirroring `tui.sh` exactly.
- `tui/internal/resolve` — locates the host commands for the TUI (and
  `mg-jdi`, which shares this package): env override (`manigot_BIN`,
  `manigot_JOB_BIN`, `manigot_DONE_BIN`, `manigot_DELETE_BIN`,
  `manigot_JDI_BIN`) → canonical name on `$PATH` → `$manigot_HOME/scripts/*.sh`.
  Nothing in the TUI may hardcode a command name; shell aliases are
  unreachable from it.
- `tui/cmd/jdi` — `mg-jdi` ("just do it"), fully autonomous mode: drives a
  job's fixed `@analyst` → `@developer` → `@reviewer` sequence end to end via
  `scripts/run.sh`'s non-interactive `--print` flag (see below), stopping at
  `verdict.md`'s `## Overall` saying APPROVED, a `NEEDS-HUMAN-INPUT:` marker,
  or a bounce-back to `@developer` that still isn't approved after one retry
  (`tui/internal/orchestrate` implements this state machine; it never
  auto-merges — the human still checks out and merges the branch). Every
  invocation's captured output and a `running`/`stopped:finished`/
  `stopped:needs-human` status are written to a sidecar directory,
  `docs/jobs/.jdi-status/<job-name>/`, outside every job's own directory so
  it can never be swept into an agent's `git add -A`: `status` (polled by
  the TUI's list-row badge) and `run.log` (polled by the TUI's detail-view
  log tab). This directory lives in the *target project*, not manigot's own
  checkout, so `mg-jdi` also ensures that project's own `.git/info/exclude`
  excludes it (idempotent, at startup) rather than assuming its tracked
  `.gitignore` already does — manigot's own `.gitignore` entry for this path
  only covers manigot's own repo. A direct `mg-jdi --job <id>` run streams that same
  output live to its own terminal and rings the terminal bell (`\a`) when it
  stops; a TUI-launched run (`j` in the detail view) has no terminal of its
  own at all — it starts fully detached, with the status badge and log tab as
  its only visibility, and the TUI itself rings the bell on its next poll
  when it notices the status transition into a stopped state.
- `config/tui-settings.json` (gitignored) — local TUI preferences: which
  editor opens `brief.md` and which agent tool (`claude-code`/`opencode`)
  agent launches use. Written by the TUI's settings screen (`s` from the job
  list), read/written via `tui/internal/config`. Missing is not an error —
  every reader falls back to defaults (`$VISUAL`/`$EDITOR`/`nano`/`vi` for the
  editor, `claude-code` for the tool).
- `scripts/entrypoint.sh` — runs inside the container before the agent CLI starts.
  Branches on `manigot_TOOL`: writes `~/.claude.json` to skip Claude Code's
  onboarding wizard, pre-accept folder trust for `/workspace`, and start it in
  permission-bypass mode (full auto, no per-tool prompts) via
  `--dangerously-skip-permissions`; or checks for a provider API key and execs
  `opencode`.
- `scripts/run.sh`'s `--print` flag — non-interactive invocation (used by
  automated/unattended runs, e.g. `mg-jdi`, not by a human's own `mg`/TUI
  session) that appends one extra sentence to the job prompt defining the
  `NEEDS-HUMAN-INPUT:` marker: an agent that cannot proceed without a human
  decision stops and prints a line starting with exactly that string instead
  of guessing. This is deliberately not a rule in `agents/*.md` itself — those
  files are read identically by attended sessions, where a human can just
  answer a question in conversation instead of the session halting.
- `agents/` — the six global agents (`analyst`, `developer`, `reviewer`,
  `security`, `product-owner`, `designer`), available in every project via
  `@name`. Baked in twice: verbatim to `~/.claude/agents/`, and to
  `~/.config/opencode/agents/` with the `name`/`tools` frontmatter keys stripped
  (OpenCode takes the name from the filename and uses a different tools schema).
  A project can override one by adding a same-named file under its own `docs/agents/`.
- `project-template/` — what gets copied into a new project (`docs/AGENTS.md`
  plus `docs/jobs/`) to bootstrap the job workflow there.
- `docs/AGENTS.md` — the project context file, tool-neutral by name, and the
  canonical source agents read at session start. Neither CLI reads it from inside
  the `docs/` mount, so `run.sh` mounts it read-only a second time at the path each
  tool actually looks in: `/workspace/AGENTS.md` (OpenCode) or
  `/workspace/.claude/CLAUDE.md` (Claude Code). Those mount paths are **read-only**
  — to change the project context, always edit the source `docs/AGENTS.md`, never
  the mounts `/workspace/AGENTS.md` or `/workspace/.claude/CLAUDE.md`.
  `docs/CLAUDE.md` still works as a fallback for older projects.
- `.env` (gitignored) — holds `CLAUDE_CODE_OAUTH_TOKEN`, `CLAUDE_ACCOUNT_UUID`,
  `CLAUDE_EMAIL`, `CLAUDE_ORG_UUID` for Claude Code, and for OpenCode at least one
  provider key (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`,
  `GOOGLE_GENERATIVE_AI_API_KEY`, `GROQ_API_KEY`, `XAI_API_KEY`,
  `DEEPSEEK_API_KEY`, `OPENCODE_API_KEY`, `ZHIPU_API_KEY`) plus optional
  `OPENCODE_MODEL`. Never
  committed. Project-level `.env` files are shadowed with `/dev/null` at container start.

## Commands
- `make build` — build the image (skips if already built)
- `make rebuild` — force rebuild with no cache, after a Claude Code / OpenCode update
- `make install` / `make uninstall` — symlink the launchers (`mg`, `mg-job`,
  `mg-done`, `mg-tui`, `mg-jdi`) into `PREFIX/bin` (default `/usr/local`)
- `make tui` — build the host-side TUI into `bin/manigot-tui`
- `make jdi` — build the host-side autonomous-mode binary into `bin/manigot-jdi`
- `mg` — start a session from inside a project directory
- `mg --tool opencode` — same, but running OpenCode instead of Claude Code
- `mg-job "<title>" [--type fix|chore]` — create a job dir + branch
- `mg-done <id>` — archive a finished job
- `mg-tui` — host-side terminal UI for browsing jobs and firing agents
- `mg-jdi --job <id>` — drive a job's `@analyst` → `@developer` → `@reviewer`
  sequence end to end, unattended (Claude Code only for v1 — see Job workflow)

## Job workflow
Each job lives in `docs/jobs/<id>_<slug>/` with four files:
`brief.md` (what/why, filled in by the user), `tasks.md` (`@analyst`),
`implementation.md` (`@developer`), `verdict.md` (`@reviewer` / `@security`).
A branch `feature|fix|chore/<id>_<slug>` is created alongside it.

Typical feature flow: `mg-job` → fill `brief.md` → `@product-owner` →
`@analyst` → review `tasks.md` → `@developer` per task → `@reviewer` →
`@security` → fix and re-review → merge → mark `brief.md` status `done`.
Bug fixes skip the `@product-owner`/`@analyst` steps and go straight to
`@developer`.

`mg-jdi` automates the middle of that flow — `@analyst` → `@developer` →
`@reviewer`, the same fixed sequence for every job `type` — without a human
manually triggering each stage. `@product-owner` and `@security` are not
part of it; both remain ordinary manually-launched agents, unaffected. It
stops (never auto-merging) when `verdict.md`'s `## Overall` says APPROVED, or
hands control back to a human when: the one allowed bounce back to
`@developer` after a REJECTED/NEEDS WORK verdict still isn't approved, an
agent prints the `NEEDS-HUMAN-INPUT:` marker (see the `--print` bullet
above), or the same agent makes no progress on two consecutive runs. v1 is
Claude Code only (`--tool opencode` is rejected by `--print`).

## Hard rules
- NEVER commit `.env` or any file containing OAuth tokens / account UUIDs
- NEVER touch a mounted project's files outside its `docs/` directory from
  within manigot tooling itself
- NEVER edit the read-only context mounts `/workspace/AGENTS.md` or
  `/workspace/.claude/CLAUDE.md` — they are read-only overlays of `docs/AGENTS.md`.
  Change the canonical source `docs/AGENTS.md` instead
- Keep `agents/*.md` and `project-template/docs/AGENTS.md` in sync with
  whatever this file documents — they're meant to describe the same system
- When scope is unclear: ask, don't guess
- Do not refactor things unrelated to the current task
