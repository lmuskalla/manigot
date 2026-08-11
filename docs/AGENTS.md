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
  `MANIGOT_PROFILE` default set by `mg profiles` or the TUI settings screen,
  else `claude-pro`), validates
  the profile's auth, and passes the choice on as `manigot_TOOL`.
  When `--job <id>` is passed, the mount root is the job's own git worktree
  (207bfu_git-worktrees), not `PROJECT_ROOT`: the job is resolved by matching
  its id_slug against local branch names, that branch's worktree is looked up
  via `scripts/lib/worktree.sh`, and `PROJECT_ROOT` is reassigned to it so the
  `docs/` mount, context-file mount, `.env`-shadow scan, and the primary
  `-v ...:/workspace:z` mount all key off the same resolved root. A branch
  match with no registered worktree is a hard error — no fallback to
  `PROJECT_ROOT`, which would silently show the wrong job's content. The one
  exception: a project with no local branches at all (not a git repo, or a
  fresh repo before its first commit) has no worktrees possible, so `--job`
  falls back to the pre-worktree directory-scan of `docs/jobs/` and
  `PROJECT_ROOT` is left untouched — mirroring the Go side's
  `job.discoverWorkingTree` trigger condition.
- `scripts/profiles.sh` — reached via `mg profiles`. Lists the three profiles
  (which are ready, and which is the default) and, on an interactive terminal,
  prompts to select the default right there; `mg profiles <name>` writes
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
  on). The base branch comes from `.manigot/manigot.json` (default `main`); the
  `--base-branch <name>` flag overrides it for one invocation. The branch is
  created as the job's own git worktree (207bfu_git-worktrees, Decision 1/3) at
  `<dirname(PROJECT_ROOT)>/.manigot-worktrees/<basename(PROJECT_ROOT)>/<id>_<slug>`,
  a sibling of `PROJECT_ROOT` rather than nested inside it — every file-write
  and the scaffold commit happen there, and `PROJECT_ROOT` itself is never
  switched. When `PROJECT_ROOT` is itself a mount point (its parent is on a
  different filesystem — e.g. `/workspace` mounted into a container), the
  sibling would land outside the project's persistent storage, so the
  worktree is nested at `<PROJECT_ROOT>/.manigot-worktrees/<id>_<slug>`
  instead and that path is excluded from the main worktree's git status via
  `.git/info/exclude` (so a `git add -A` there never sweeps the nested
  checkouts' content into a commit). A non-git-repo project keeps the
  pre-worktree fallback: no branch, no worktree, the scaffold is written
  straight into `PROJECT_ROOT`.
- `scripts/finish-job.sh` — reached via `mg done`. Archives a finished job:
  the clean-tree check, the archive move, and the archive commit all run
  inside the job's own worktree; `PROJECT_ROOT` (the main worktree) is used
  only for the squash-merge + branch delete; then the job's worktree is
  removed (`git worktree remove`, best-effort `git worktree prune`) — except
  when the job's branch is checked out in the main worktree itself (a
  pre-worktree job), where the removal step is skipped (the main worktree
  can't be removed) and the branch delete alone finishes the job.
- `scripts/delete-job.sh` — reached via `mg delete`. Permanently deletes a
  job: its worktree (force-removed via `git worktree remove --force`, with an
  explicit "uncommitted changes will be discarded" warning when dirty) and its
  branch (`git branch -D` — no merge, unlike `mg done`). When the job's
  branch is checked out in the main worktree itself (a pre-worktree job), the
  worktree removal is skipped — the main worktree is switched off the branch
  and the branch delete alone suffices. A non-git project's
  job is a plain directory delete, no git involved.
- `scripts/tui.sh` — reached via `mg tui`; wrapper around
  `bin/manigot-tui` that exports `manigot_HOME` so the TUI can find the scripts.
- `scripts/jdi.sh` — reached via `mg jdi` (thematic alias: `mg made-man`,
  same script, same behavior); wrapper around `bin/manigot-jdi`, mirroring
  `tui.sh` exactly.
- `scripts/init.sh` — reached via `mg init`. Bootstraps a project for the job
  workflow: copies `project-template/docs/` (`AGENTS.md`, `CLAUDE.md`, and an
  empty `docs/jobs/` — never the example job under it) into the target
  project's `docs/`, plus `project-template/.manigot/manigot.json` (the
  seeded project settings) into the target's `.manigot/`, if `docs/` is
  absent — reporting "already initialized" and skipping the copy otherwise —
  then optionally hands off to `@prompter`
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
  auto-merges — the human still merges the branch via `mg done`, and `mg jdi`
  never checks anything out in the main worktree: every invocation resolves
  the job's own worktree itself, 207bfu_git-worktrees). Every
  invocation's captured output and a `running`/`stopped:finished`/
  `stopped:needs-human` status are written to a sidecar directory,
  `.manigot/jdi-status/<job-name>/`, outside every job's own directory so
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
  which editor opens `brief.md`, how many entries the dashboard's
  recent-activity strip may show (`recentActivityCount`, default 5, valid
  1–100), and which terminal `launch.Agent`/`Quick`/`AgentQuick` spawn a
  session in (`terminal`, e.g. `"kitty"` or `"alacritty -e"`). The subscription
  profile is NOT stored here —
  it lives in manigot's `.env` as `MANIGOT_PROFILE` (see the `.env` bullet), the
  one default shared between CLI and TUI: the TUI's settings screen (`s` from
  the job list) reads and writes that value via `tui/internal/config`, the same
  key `mg profiles` writes and bare `mg` resolves to. Project-scoped
  conventions (currently: the base branch) are NOT here either — they live in
  the target project's `.manigot/manigot.json` (see next bullet), so they travel
  with the project and are shared across a team rather than being a per-user
  pref. Written by the TUI's settings screen, read/written
  via `tui/internal/config`. Missing is not an error —
  every reader falls back to defaults (`$VISUAL`/`$EDITOR`/`nano`/`vi` for the
  editor, `claude-pro` for the profile, 5 for the recent-activity count, and
  today's fixed tmux/Terminal.app/Linux-emulator auto-detect spawn order —
  see `tui/internal/launch` — for the terminal). When `terminal` is set, it
  overrides that whole spawn order unconditionally, including the tmux
  split-pane behavior. A
  legacy `profile` field in the file
  is honored as a migration fallback while `.env` has no `MANIGOT_PROFILE` yet,
  and the older `tool` field still migrates (`claude-code`→`claude-pro`,
  `opencode`→`zai`); neither is written back.
- `.manigot/manigot.json` (in the target project, committable) — project-scoped
  manigot conventions, the project-level counterpart to the personal
  `config/tui-settings.json`. Currently holds `baseBranch`: the ref new job
  branches are cut from (`scripts/new-job.sh`), defaulting to `main` when
  unset. Read by the TUI
  (via `tui/internal/project`, loaded at startup and on ctrl+r refresh) and
  by `scripts/new-job.sh` directly (guarded single-key `sed` extraction — no
  `jq` dependency yet). Seeded by `mg init` and created on first TUI settings
  save; contains only a public ref name, no secrets, so it's meant to be
  committed and shared across a team.
- `.manigot/` (in the target project) — manigot's own directory for host-side
  tooling state that is not job content and does not belong in docs/: the
  committable `.manigot/manigot.json` project settings (previous bullet) and
  the gitignored `.manigot/jdi-status/` ephemeral mg-jdi run state (see the
  `tui/cmd/jdi` bullet). Nothing else belongs here, and host-side tooling is
  the only writer.
- `manigot/.env` (gitignored) — holds credentials and defaults for the
  profiles: `CLAUDE_CODE_OAUTH_TOKEN`/`CLAUDE_ACCOUNT_UUID`/`CLAUDE_EMAIL`/
  `CLAUDE_ORG_UUID` (claude-pro), `ZHIPU_API_KEY` + `OPENCODE_ZAI_MODEL`
  (zai), `OPENCODE_API_KEY` + `OPENCODE_GO_MODEL` (opencode-go), and
  `MANIGOT_PROFILE` — the default profile, the ONE value shared between CLI and
  TUI: bare `mg` resolves to it, `mg profiles` writes it, and the TUI's
  settings screen reads/writes the same key.
  Written by `mg setup`/`mg profiles`/the TUI settings screen, sourced by
  `scripts/run.sh`. Never committed.
- `scripts/entrypoint.sh` — runs inside the container before the agent CLI starts.
  Branches on `manigot_TOOL`: writes `~/.claude.json` to skip Claude Code's
  onboarding wizard, pre-accept folder trust for `/workspace`, and start it in
  permission-bypass mode (full auto, no per-tool prompts) via
  `--dangerously-skip-permissions`; or checks for a provider API key and starts
  OpenCode in auto mode via `--auto` (full auto, no per-tool prompts — the
  direct OpenCode analog of Claude's `--dangerously-skip-permissions`:
  `--auto` auto-approves any permission that isn't explicitly denied, and the
  container's opencode config contains no deny rules, so every session runs
  fully automatic). When `manigot_PRINT` is set, each branch execs its CLI's
  own non-interactive/headless invocation instead of the interactive one:
  `claude --print --output-format json` (a single JSON object with a `"result"`
  field), or, for OpenCode, `opencode run <message> --agent <agent> --auto
  --format json` (translated from the interactive `--agent`/`--prompt`
  passthrough — OpenCode's headless mode takes the prompt as a positional
  argument, not a flag; its JSON output is a JSONL stream of typed events, the
  response text living in `"text"`-typed events' `part.text`).
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
- `mg profiles [name]` — list the profiles and which is the default, set the
  default profile bare `mg` uses (`MANIGOT_PROFILE` in manigot's `.env`), or
  pick it interactively (no name, on a TTY). The TUI's settings screen shares
  the same default.
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
  `.manigot/manigot.json`; `--base-branch` overrides it for one invocation; the
  branch is checked out in the job's own git worktree, see Job workflow)
- `mg done <id>` — archive a finished job (merges it into the base branch and
  removes its worktree)
- `mg delete <id>` — permanently delete a job (worktree + branch, no merge)
- `mg tui` — host-side terminal UI for browsing jobs and firing agents
- `mg jdi --job <id> [--profile <name>]` — drive a job's `@analyst` →
  `@developer` → `@reviewer` sequence end to end, unattended, under the given
  subscription profile (default `claude-pro`; see Job workflow)
  (thematic alias: `mg made-man --job <id>`, same script/behavior)

## Job workflow
Each job lives in `docs/jobs/<id>_<slug>/` with four files:
`brief.md` (what/why, filled in by the user), `tasks.md` (`@analyst`),
`implementation.md` (`@developer`), `verdict.md` (`@reviewer` / `@security`).
A branch `feature|fix|chore/<id>_<slug>` is created alongside it, checked out
in the job's own git worktree (207bfu_git-worktrees) — every job gets its own
directory, so multiple jobs — interactive or autonomous — can run in
parallel, and `PROJECT_ROOT` stays on the base branch in steady state.

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
when unset; the TUI's `j` keybinding passes its settings profile — the same
shared `MANIGOT_PROFILE` (`config.Settings.ProfileValue()`) — through the same
way `@name` launches do.

## Hard rules
- NEVER commit `.env` or any file containing OAuth tokens / account UUIDs
- NEVER touch a mounted project's files outside its `docs/` directory from
  within manigot tooling itself — the one deliberate exception is the target
  project's `.manigot/` directory, which host-side manigot tooling itself
  owns and writes: `.manigot/manigot.json` (project settings) and
  `.manigot/jdi-status/` (mg-jdi run state). Agents must treat `.manigot/`
  like any other tool-managed state: read the settings file if needed, but
  never edit either path by hand.
- NEVER edit the read-only context mounts `/workspace/AGENTS.md` or
  `/workspace/.claude/CLAUDE.md` — they are read-only overlays of `docs/AGENTS.md`.
  Change the canonical source `docs/AGENTS.md` instead
- Keep `agents/*.md` and `project-template/docs/AGENTS.md` in sync with
  whatever this file documents — they're meant to describe the same system
- When scope is unclear: ask, don't guess
- Do not refactor things unrelated to the current task
