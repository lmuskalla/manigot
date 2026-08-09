# safecode

Isolated agent environment per project: one Docker image, subscription
billing via mounted OAuth credentials, real filesystem containment, and a
structured brief → tasks → implementation → verdict job workflow. Vendor-agnostic
— the same image runs either Claude Code (default) or OpenCode, chosen per
session with `sc --tool claude-code|opencode`.

## Stack
- Runtime: Docker (single image, built from `Dockerfile`)
- Agent CLIs: Claude Code (`claude`) and OpenCode (`opencode`), both installed in the image
- Orchestration: Bash scripts in `scripts/` (`run.sh`, `new-job.sh`, `finish-job.sh`,
  `tui.sh`, `entrypoint.sh`)
- Build/CLI: `Makefile` (`make build`, `make rebuild`, `make install`, `make tui`)
- Host-side TUI: Go, in `tui/` — built with `make tui`, never runs in the container
- Agent definitions: Markdown files in `agents/`, baked into the image at build time

## Architecture
- `Dockerfile` — builds the image; installs both agent CLIs. Rebuild after a
  Claude Code or OpenCode update via `make rebuild`.
- `scripts/run.sh` — container launcher, symlinked as `sc` in PATH. Mounts
  the current project's `docs/` into the container; nothing else on the host
  is reachable from inside. Mount target depends on the tool:
  `/workspace/.claude` for Claude Code, `/workspace/.opencode` for OpenCode.
  Validates auth per tool and passes the choice on as `SAFECODE_TOOL`.
- `scripts/new-job.sh` — installed as `sc-job`. Creates a new job directory
  under `docs/jobs/<id>_<slug>/` and a matching git branch.
- `scripts/finish-job.sh` — installed as `sc-done`. Archives a finished job.
- `scripts/tui.sh` — installed as `sc-tui`; wrapper around
  `bin/safecode-tui` that exports `SAFECODE_HOME` so the TUI can find the scripts.
- `tui/internal/resolve` — locates the host commands for the TUI: env override
  (`SAFECODE_BIN`, `SAFECODE_JOB_BIN`, `SAFECODE_DONE_BIN`) → canonical name on
  `$PATH` → `$SAFECODE_HOME/scripts/*.sh`. Nothing in the TUI may hardcode a
  command name; shell aliases are unreachable from it.
- `config/tui-settings.json` (gitignored) — local TUI preferences: which
  editor opens `brief.md` and which agent tool (`claude-code`/`opencode`)
  agent launches use. Written by the TUI's settings screen (`s` from the job
  list), read/written via `tui/internal/config`. Missing is not an error —
  every reader falls back to defaults (`$VISUAL`/`$EDITOR`/`nano`/`vi` for the
  editor, `claude-code` for the tool).
- `scripts/entrypoint.sh` — runs inside the container before the agent CLI starts.
  Branches on `SAFECODE_TOOL`: writes `~/.claude.json` to skip Claude Code's
  onboarding wizard, pre-accept folder trust for `/workspace`, and start it in
  permission-bypass mode (full auto, no per-tool prompts) via
  `--dangerously-skip-permissions`; or checks for a provider API key and execs
  `opencode`.
- `agents/` — the six global agents (`analyst`, `developer`, `reviewer`,
  `security`, `product-owner`, `designer`), available in every project via
  `@name`. Baked in twice: verbatim to `~/.claude/agents/`, and to
  `~/.config/opencode/agents/` with the `name`/`tools` frontmatter keys stripped
  (OpenCode takes the name from the filename and uses a different tools schema).
  A project can override one by adding a same-named file under its own `docs/agents/`.
- `project-template/` — what gets copied into a new project (`docs/AGENTS.md`
  plus `docs/jobs/`) to bootstrap the job workflow there.
- `docs/AGENTS.md` — the project context file, tool-neutral by name. Neither CLI
  reads it from inside the `docs/` mount, so `run.sh` mounts it read-only a second
  time at `/workspace/AGENTS.md` (OpenCode) or `/workspace/.claude/CLAUDE.md`
  (Claude Code). `docs/CLAUDE.md` still works as a fallback for older projects.
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
- `make install` / `make uninstall` — symlink the launchers (`sc`, `sc-job`,
  `sc-done`, `sc-tui`) into `PREFIX/bin` (default `/usr/local`)
- `make tui` — build the host-side TUI into `bin/safecode-tui`
- `sc` — start a session from inside a project directory
- `sc --tool opencode` — same, but running OpenCode instead of Claude Code
- `sc-job "<title>" [--type fix|chore]` — create a job dir + branch
- `sc-done <id>` — archive a finished job
- `sc-tui` — host-side terminal UI for browsing jobs and firing agents

## Job workflow
Each job lives in `docs/jobs/<id>_<slug>/` with four files:
`brief.md` (what/why, filled in by the user), `tasks.md` (`@analyst`),
`implementation.md` (`@developer`), `verdict.md` (`@reviewer` / `@security`).
A branch `feature|fix|chore/<id>_<slug>` is created alongside it.

Typical feature flow: `sc-job` → fill `brief.md` → `@product-owner` →
`@analyst` → review `tasks.md` → `@developer` per task → `@reviewer` →
`@security` → fix and re-review → merge → mark `brief.md` status `done`.
Bug fixes skip the `@product-owner`/`@analyst` steps and go straight to
`@developer`.

## Hard rules
- NEVER commit `.env` or any file containing OAuth tokens / account UUIDs
- NEVER touch a mounted project's files outside its `docs/` directory from
  within safecode tooling itself
- Keep `agents/*.md` and `project-template/docs/AGENTS.md` in sync with
  whatever this file documents — they're meant to describe the same system
- When scope is unclear: ask, don't guess
- Do not refactor things unrelated to the current task
