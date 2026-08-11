# manigot

Isolated agent environment per project: one Docker image, subscription
billing via mounted OAuth credentials, real filesystem containment, and a
structured brief → tasks → implementation → verdict job workflow. Runs a
session under one of three subscription profiles — `claude-pro` (Claude Code,
billed to Claude Pro/Max), `zai` (OpenCode, billed to a Z.AI Coding Plan), and
`opencode-go` (OpenCode, billed to the OpenCode Go subscription) — chosen per
session with `mg --profile <name>`, defaulted with `mg profiles`, and
configured with `mg setup`.

## Stack
- Runtime: Docker (single image, built from `Dockerfile`)
- Agent CLIs: Claude Code (`claude`) and OpenCode (`opencode`), both installed in the image
- Orchestration: Bash scripts in `scripts/` (`mg.sh`, `run.sh`, `agents.sh`,
  `new-job.sh`, `finish-job.sh`, `delete-job.sh`, `tui.sh`, `jdi.sh`, `init.sh`,
  `entrypoint.sh`)
- Build/CLI: `Makefile` (`make build`, `make rebuild`, `make install`, `make tui`, `make jdi`)
- Host-side TUI: Go, in `tui/` — built with `make tui`, never runs in the container
- Autonomous mode: Go, in `tui/cmd/jdi` (same module as the TUI) — built with
  `make jdi` into the binary `mg jdi` runs, also host-side, never runs in the container
- Agent definitions: Markdown files in `agents/`, baked into the image at build time

## Architecture
- `Dockerfile` — builds the image; installs both agent CLIs. Rebuild after a
  Claude Code or OpenCode update via `make rebuild`.
- `scripts/mg.sh` — the single dispatcher, symlinked as `mg` in PATH. Inspects
  its first argument: `-h`/`--help`/`help` prints usage and exits immediately
  (no docker/auth setup touched); one of the subcommand names
  (`profiles`→`profiles.sh`, `setup`→`setup.sh`, `agents`→`agents.sh`,
  `job`→`new-job.sh`, `tui`→`tui.sh`, `jdi`→`jdi.sh`,
  `done`→`finish-job.sh`, `delete`→`delete-job.sh`, `init`→`init.sh`) execs
  the matching sibling script unchanged; anything else (no args, or any other
  first token, including `run.sh`'s own
  `--agent`/`--job`/`--tool`/`--profile`/`--print`
  flags) falls through to `run.sh` with all original args untouched.
- `scripts/run.sh` — container launcher, reached via bare `mg` (no
  subcommand). Mounts the current project root into the container at
  `/workspace`; nothing outside it on the host is reachable from inside. A
  `docs/` directory (walked up from `$PWD`) marks the project as
  *initialized*: when present, it's additionally mounted at
  `/workspace/.claude` (Claude Code) or `/workspace/.opencode` (OpenCode),
  giving access to project context and the job workflow. `docs/` is optional
  — its absence doesn't block `mg`, it just runs a plain isolated session
  with no project context and no job workflow (job-workflow subcommands
  like `mg job`/`mg jdi` still require it). When no `docs/` is found, the
  container boundary falls back to the git root, else `$PWD`. Resolves the
  session's subscription profile (`--profile`, else legacy `--tool`, else the
  `MANIGOT_PROFILE` default set by `mg profiles`, else `claude-pro`), validates
  the profile's auth, and passes the choice on as `manigot_TOOL`.
- `scripts/profiles.sh` — reached via `mg profiles`. Lists the three profiles
  (which are ready, and which is the default); `mg profiles <name>` writes
  `MANIGOT_PROFILE=<name>` into manigot's own `.env` so bare `mg` runs use it.
- `scripts/setup.sh` — reached via `mg setup`. Interactive wizard that guides
  you through configuring each profile's credentials into manigot's `.env`,
  auto-applying what it can read off the host (e.g. the Claude account from
  `~/.claude.json`) and letting you paste the rest. `mg setup <name>` for one
  profile, `mg setup --check` for a non-interactive status report.
- `scripts/agents.sh` — reached via `mg agents` (thematic alias: `mg crew`,
  same script, same behavior). Lists every agent available to the current
  project — the global `agents/*.md` files, each swapped for its
  `docs/agents/` override when one exists, plus any project-only
  additions — prompts for a numbered selection, then execs `run.sh --agent
  <name>` with any other args (e.g. `--profile`) passed through. Works without
  `docs/` too, same as bare `mg` — it just has no overrides to show.
- `scripts/new-job.sh` — reached via `mg job`. Creates a new job directory
  under `docs/jobs/<id>_<slug>/` and a matching git branch, always branched from
  the configured base branch (regardless of the branch the user is currently
  on). The base branch comes from `docs/manigot.json` (default `main`); the
  `--base-branch <name>` flag overrides it for one invocation.
- `scripts/finish-job.sh` — reached via `mg done`. Archives a finished job.
- `scripts/delete-job.sh` — reached via `mg delete`. Permanently deletes a
  job: its directory under `docs/jobs/` and, when the job has a branch, the
  branch itself (`git branch -D` — no merge, unlike `mg done`).
- `scripts/tui.sh` — reached via `mg tui`; wrapper around
  `bin/manigot-tui` that exports `manigot_HOME` so the TUI can find the scripts.
- `scripts/jdi.sh` — reached via `mg jdi` (thematic alias: `mg made-man`,
  same script, same behavior); wrapper around `bin/manigot-jdi`, mirroring
  `tui.sh` exactly.
- `scripts/init.sh` — reached via `mg init`. Bootstraps a project for the job
  workflow: copies `project-template/docs/` (`AGENTS.md`, `CLAUDE.md`, a seeded
  `manigot.json`, and an empty `docs/jobs/` — never the example job under it)
  into the target project's `docs/` if absent, reporting "already initialized"
  and skipping the copy otherwise, then optionally hands off to `@prompter`
  (via `run.sh`'s `--prompt` flag) to draft a concrete `docs/AGENTS.md`. Unlike
  every other job-workflow subcommand, it deliberately works **without** an
  existing `docs/` — it's the one that creates it.
- `tui/internal/resolve` — locates the host commands for the TUI (and
  `mg jdi`, which shares this package): env override (`manigot_BIN`,
  `manigot_JOB_BIN`, `manigot_DONE_BIN`, `manigot_DELETE_BIN`,
  `manigot_JDI_BIN`) → canonical name on `$PATH` → `$manigot_HOME/scripts/*.sh`.
  Nothing in the TUI may hardcode a command name; shell aliases are
  unreachable from it.
- `tui/cmd/jdi` — `mg jdi` ("just do it"), fully autonomous mode: drives a
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
  checkout, so `mg jdi` also ensures that project's own `.git/info/exclude`
  excludes it (idempotent, at startup) rather than assuming its tracked
  `.gitignore` already does — manigot's own `.gitignore` entry for this path
  only covers manigot's own repo. A direct `mg jdi --job <id>` run streams that same
  output live to its own terminal and rings the terminal bell (`\a`) when it
  stops; a TUI-launched run (`j` in the detail view) has no terminal of its
  own at all — it starts fully detached, with the status badge and log tab as
  its only visibility, and the TUI itself rings the bell on its next poll
  when it notices the status transition into a stopped state.
- `config/tui-settings.json` (gitignored) — local TUI **personal** preferences:
  which editor opens `brief.md` and which subscription profile
  (`claude-pro`/`zai`/`opencode-go`) agent launches use. Project-scoped
  conventions (currently: the base branch) are NOT here — they live in the
  target project's `docs/manigot.json` (see next bullet), so they travel with
  the project and are shared across a team rather than being a per-user pref.
  Written by the TUI's settings screen (`s` from the job list), read/written
  via `tui/internal/config`. Missing is not an error —
  every reader falls back to defaults (`$VISUAL`/`$EDITOR`/`nano`/`vi` for the
  editor, `claude-pro` for the profile). This is the TUI's own default — it
  always passes `--profile` explicitly, independent of the `MANIGOT_PROFILE`
  default in `.env` set by `mg profiles`. A legacy `tool` field in the file is
  migrated on load (`claude-code`→`claude-pro`, `opencode`→`zai`).
- `docs/manigot.json` (in the target project, committable) — project-scoped
  manigot conventions, the project-level counterpart to the personal
  `config/tui-settings.json`. Currently holds `baseBranch`: the ref new job
  branches are cut from (`scripts/new-job.sh`) and the TUI list view's "b"
  quick-checkout lands on, defaulting to `main` when unset. Read by the TUI
  (via `tui/internal/project`, loaded at startup and on ctrl+r refresh) and
  by `scripts/new-job.sh` directly (guarded single-key `sed` extraction — no
  `jq` dependency yet). Seeded by `mg init` and created on first TUI settings
  save; contains only a public ref name, no secrets, so it's meant to be
  committed and shared across a team.
- `manigot/.env` (gitignored) — holds credentials and defaults for the
  profiles: `CLAUDE_CODE_OAUTH_TOKEN`/`CLAUDE_ACCOUNT_UUID`/`CLAUDE_EMAIL`/
  `CLAUDE_ORG_UUID` (claude-pro), `ZHIPU_API_KEY` + `OPENCODE_ZAI_MODEL`
  (zai), `OPENCODE_API_KEY` + `OPENCODE_GO_MODEL` (opencode-go), and
  `MANIGOT_PROFILE` (the default profile for bare `mg`, set by `mg profiles`).
  Written by `mg setup`/`mg profiles`, sourced by `scripts/run.sh`. Never
  committed.
- `scripts/entrypoint.sh` — runs inside the container before the agent CLI starts.
  Branches on `manigot_TOOL`: writes `~/.claude.json` to skip Claude Code's
  onboarding wizard, pre-accept folder trust for `/workspace`, and start it in
  permission-bypass mode (full auto, no per-tool prompts) via
  `--dangerously-skip-permissions`; or checks for a provider API key and execs
  `opencode`. When `manigot_PRINT` is set, each branch execs its CLI's own
  non-interactive/headless invocation instead of the interactive one: `claude
  --print --output-format json` (a single JSON object with a `"result"`
  field), or, for OpenCode, `opencode run <message> --agent <agent> --format
  json` (translated from the interactive `--agent`/`--prompt` passthrough —
  OpenCode's headless mode takes the prompt as a positional argument, not a
  flag; its JSON output is a JSONL stream of typed events, the response text
  living in `"text"`-typed events' `part.text`).
- `scripts/run.sh`'s `--print` flag — non-interactive invocation (used by
  automated/unattended runs, e.g. `mg jdi`, not by a human's own `mg`/TUI
  session) that appends one extra sentence to the job prompt defining the
  `NEEDS-HUMAN-INPUT:` marker: an agent that cannot proceed without a human
  decision stops and prints a line starting with exactly that string instead
  of guessing. This is deliberately not a rule in `agents/*.md` itself — those
  files are read identically by attended sessions, where a human can just
  answer a question in conversation instead of the session halting. Supported
  under every profile (`claude-pro`, `zai`, `opencode-go`) — only the legacy,
  profile-less `--tool opencode` path still rejects it.
- `agents/` — the eight global agents (`analyst`, `developer`, `reviewer`,
  `security`, `product-owner`, `designer`, `quality`, `prompter`), available in every project via
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
- Project-level `.env` files in a target project are shadowed with `/dev/null`
  at container start.

## Commands
- `mg -h` / `mg --help` / `mg help` — print usage and exit (no docker/auth setup touched)
- `make build` — build the image (skips if already built)
- `make rebuild` — force rebuild with no cache, after a Claude Code / OpenCode update
- `make install` / `make uninstall` — symlink the single `mg` dispatcher into
  `PREFIX/bin` (default `/usr/local`)
- `make tui` — build the host-side TUI into `bin/manigot-tui`
- `make jdi` — build the host-side autonomous-mode binary into `bin/manigot-jdi`
- `mg` — start an isolated session from inside any project directory; `docs/`
  is optional (see `scripts/run.sh` above) — without it you still get a
  plain agent session, just no project context or job workflow
- `mg --profile <name>` — same, but under the given subscription profile
  (`claude-pro`/`zai`/`opencode-go`); `--tool` is accepted as a legacy alias
- `mg profiles [name]` — list the profiles and which is the default, or set
  the default profile bare `mg` uses (`MANIGOT_PROFILE` in manigot's `.env`)
- `mg setup [name] [--check]` — configure credentials for the profiles,
  interactively, or report status with `--check`
- `mg agents` — list available agents (global + any `docs/agents/`
  overrides/additions) and pick one interactively to start a session in
  (thematic alias: `mg crew`, same script/behavior)
- `mg init [--profile <name>]` — bootstrap a project for the job
  workflow (creates `docs/` if absent, optionally hands off to `@prompter`);
  the only job-workflow command that works without an existing `docs/`
- `mg job "<title>" [--type fix|chore] [--base-branch <name>]` — create a job
  dir + branch (the branch is cut from the configured base branch — see
  `docs/manigot.json`; `--base-branch` overrides it for one invocation)
- `mg done <id>` — archive a finished job
- `mg delete <id>` — permanently delete a job (directory + branch, no merge)
- `mg tui` — host-side terminal UI for browsing jobs and firing agents
- `mg jdi --job <id> [--profile <name>]` — drive a job's `@analyst` →
  `@developer` → `@reviewer` sequence end to end, unattended, under the given
  subscription profile (default `claude-pro`; see Job workflow)
  (thematic alias: `mg made-man --job <id>`, same script/behavior)

## Job workflow
Each job lives in `docs/jobs/<id>_<slug>/` with four files:
`brief.md` (what/why, filled in by the user), `tasks.md` (`@analyst`),
`implementation.md` (`@developer`), `verdict.md` (`@reviewer` / `@security`).
A branch `feature|fix|chore/<id>_<slug>` is created alongside it.

Typical feature flow: `mg job` → fill `brief.md` → `@product-owner` →
`@analyst` → review `tasks.md` → `@developer` per task → `@reviewer` →
`@security` → fix and re-review → merge → mark `brief.md` status `done`.
Bug fixes skip the `@product-owner`/`@analyst` steps and go straight to
`@developer`.

`mg jdi` automates the middle of that flow — `@analyst` → `@developer` →
`@reviewer`, the same fixed sequence for every job `type` — without a human
manually triggering each stage. `@product-owner` and `@security` are not
part of it; both remain ordinary manually-launched agents, unaffected. It
stops (never auto-merging) when `verdict.md`'s `## Overall` says APPROVED, or
hands control back to a human when: the one allowed bounce back to
`@developer` after a REJECTED/NEEDS WORK verdict still isn't approved, an
agent prints the `NEEDS-HUMAN-INPUT:` marker (see the `--print` bullet
above), or the same agent makes no progress on two consecutive runs. `mg
jdi --profile <name>` selects which subscription profile drives the agent
sequence (`claude-pro`, `zai`, or `opencode-go`), defaulting to `claude-pro`
when unset; the TUI's `j` keybinding passes its own settings profile
(`config.Settings.ProfileValue()`) through the same way `@name` launches do.

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
