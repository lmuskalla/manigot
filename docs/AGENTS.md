# manigot

Isolated agent environment per project: one Docker image, subscription
billing via mounted OAuth credentials, real filesystem containment, and a
structured brief → tasks → implementation → verdict job workflow. Runs a
session under one of five subscription profiles — `claude-pro` (Claude Code,
billed to Claude Pro/Max), `zai` (OpenCode, billed to a Z.AI Coding Plan),
`opencode-go` (OpenCode, billed to the OpenCode Go subscription),
`opencode-zen` (OpenCode, billed to OpenCode Zen), and `opencode-zen-free`
(OpenCode, billed to OpenCode Zen's free DeepSeek V4 Flash Free model) —
chosen per session with `mg --profile <name>`, defaulted with `mg profiles`,
and configured with `mg setup`.

## Stack
- Runtime: Docker (single image, built from `Dockerfile`)
- Agent CLIs: Claude Code (`claude`) and OpenCode (`opencode`), both installed in the image
- Host-side tool: **one Go binary, `mg`** (`src/cmd/mg`), the entire host-side
  orchestrator — bash is reduced to exactly one file,
  `scripts/entrypoint.sh`, which runs inside the container image and is part
  of the agent environment, not the orchestrator
- Host-side logic: Go packages under `src/internal/` (`session`, `job`, `git`,
  `ui`, `orchestrate`, `config`, `agentlist`, `cli`, `home`, `launch`,
  `markdown`, `project`, `editor`) — the Go module root is `src/`
  (`src/go.mod`)
- Agent definitions: Markdown files in `agents/`, delivered into every session
  from the manigot checkout (mounted read-only into the container at the CLI's
  global agent location; for `mg host`, symlinked into Claude Code's config
  dir or written as converted copies into OpenCode's)
- Skills: directories in `skills/` (`<name>/SKILL.md`, mirroring `agents/`),
  delivered into every session the same way — mounted read-only into the
  container at the CLI's global skills location (both CLIs load skills at
  startup, so every agent and every agentless invocation sees them), and for
  `mg host` delivered into the host CLI's config dir. Project skills in
  `docs/skills/` ride the docs mount and override global skills of the same
  name. manigot ships a small curated set of skills under `skills/` —
  `frontend-design`, `webapp-testing`, `test-driven-development`,
  `systematic-debugging`, plus the example `job-brief`; the mechanism is
  otherwise provisioned for user-supplied global + project skills.
- Meta prompt: a system-wide instruction file `prompts/meta.md` in the
  checkout, injected into every session — the top of the instruction hierarchy
  (meta prompt → agents → skills → project context). Delivered like global
  skills: mounted read-only into the container at each CLI's *global
  instruction* location (`~/.claude/CLAUDE.md` for Claude Code, the user-global
  memory file; `~/.config/opencode/AGENTS.md` for OpenCode, the global rules
  file), and for `mg host` copied — never symlinked — into the host CLI's own
  global instruction file (a symlink would let Claude's `/memory` writes and
  agent edits land back in the checkout). Non-clobbering and optional: a
  checkout without `prompts/meta.md` yields no delivery.
- The `shot` render tool (`scripts/shot.js`, baked into the image as
  `/usr/local/bin/shot`): Playwright (chromium-headless-shell, installed via
  `--with-deps` at build) renders a URL to a PNG and produces a model-free
  render report of exact DOM facts — element inventory, WCAG AA/AAA contrast,
  overflow/clipping, alignment/spacing, font status, z-index. Agent-neutral
  tooling: developer may run it (verify own rendered work); read-only agents
  consume the artifacts and never run it (see docs/PLAYWRIGHT.md).

## Architecture

The seam between the orchestrator (host-side Go) and the agent environment
(Docker image + `scripts/entrypoint.sh`) is the **only** seam in the system.
The Go module lives in `src/` (module root: `src/go.mod`, `src/cmd/`,
`src/internal/`); all paths below are relative to `src/` unless stated.

- `cmd/mg/main.go` — the single binary's dispatcher. Every command is a
  subcommand run in-process: bare `mg` (session), `profiles`, `setup`,
  `agents`/`crew`, `job`, `jobs`, `done`, `delete`, `diff`, `init`, `tui`,
  `jdi`/`made-man`, `help`. `make mg` builds it to `bin/mg`; `make install`
  symlinks one `mg` onto `PATH`.
- `internal/session` — the docker session launcher (was `scripts/run.sh`):
  profile/tool resolution, auth validation, project-root + `--job` worktree
  resolution, docker argv/mount/env construction, and the run itself.
  The TUI and mg-jdi call it directly. For OpenCode sessions it also converts
  agent files (`docs/agents/*.md` and the mounted global `agents/*.md`) to
  OpenCode's schema at launch (the same
  `name`/`tools` strip the Dockerfile used to apply to the baked-in agents — a
  `permission:` block passes through untouched, which is how the read-only
  agents express their restriction under OpenCode), writing
  the converted copies to a temp dir shadow-mounted over the target
  `agents/` subpath — the host's source files are never modified, and the
  temp dir is cleaned up after the run. Skills need no such conversion (both
  CLIs read `SKILL.md` frontmatter natively), so global skills
  (`<home>/skills/`) are mounted read-only at the CLI's global skills
  location — verbatim for Claude Code, staged into a temp dir for OpenCode —
  and project skills (`docs/skills/`) ride the existing docs mount, overriding
  global skills of the same name. The system-wide meta prompt
  (`<home>/prompts/meta.md`)
  is mounted read-only the same way at each CLI's *global instruction*
  location (`~/.claude/CLAUDE.md` for Claude Code,
  `~/.config/opencode/AGENTS.md` for OpenCode) — plain markdown, so no
  conversion and no temp dir. It also decides the job git-common-dir
  mount mode from the resolved agent's `commit:` frontmatter marker (see
  "Read-only git mount for non-committing agents").
- `internal/job` — the job lifecycle (was `new-job.sh`/`finish-job.sh`/
  `delete-job.sh`): `CreateJob` (scaffold + worktree + first commit),
  `FinishJob` (archive + squash merge + branch delete), `DeleteJob`
  (worktree force-remove + branch `-D`). Discovery (`job.Discover`) lists
  open jobs from `git worktree list`, one worktree per job.
- `internal/git` — the only place that shells out to git: worktree
  create/remove, squash merge, branch delete, dirty checks, branch lookup,
  and worktree-gitdir enumeration (`WorktreeGitDirs`, used by the session
  launcher's gitdir overlay mounts).
- `internal/ui` — the Bubble Tea UI: the TUI, reached as `mg tui`, plus the
  reusable single-select picker (`Picker`, run with the alt screen) behind
  `mg jobs`'/`mg agents`' interactive selection on a TTY.
- `internal/orchestrate` — the `mg jdi` state machine (`@analyst` →
  `@developer` → `@reviewer`).
- `internal/config` — the profiles table, `config/tui-settings.json`
  settings, and manigot's `.env` read/write (`GetEnv`/`UpsertEnv`).
- `internal/home` — locates the manigot checkout the binary belongs to
  (`$MANIGOT_HOME`, the binary's own location, or the working directory) —
  the source of `.env`, `config/`, `agents/`, `skills/`, `assets/` (quotes.json),
  `prompts/` (the meta prompt) and `project-template/`.
- `scripts/entrypoint.sh` — the ONLY bash, container-side. Branches on
  `manigot_TOOL`: writes `~/.claude.json` to skip Claude Code's onboarding
  wizard, pre-accepts folder trust for `/workspace`, and starts it in
  permission-bypass mode via `--dangerously-skip-permissions`; or checks for
  a provider API key and starts OpenCode in auto mode via `--auto`. When
  `manigot_PRINT` is set, each branch execs its CLI's non-interactive
  invocation instead: `claude --print --verbose --output-format stream-json`, or
  `opencode run <message> --agent <agent> --auto --format json`. It also
  installs a PATH-first `git` shim that restricts agents to read + commit git
  commands (see "Session git shim" below).
- `Dockerfile` — builds the image; installs both agent CLIs (the image is
  purely isolation for workspaces — the global `agents/` and `skills/` are no
  longer baked in, they are mounted from the host at session launch), and
  pre-warms the Go module cache from `src/go.mod`/`src/go.sum` (with
  `GOTOOLCHAIN=local` a stale path breaks the build). The meta-prompt mount
  targets' parent dirs (`~/.claude` and `~/.config/opencode`) are pre-created
  in the image.

### Session launch (bare `mg`)

Bare `mg` (and `--agent`/`-a`, `--job`/`-j`, `--prompt`, `--tool`,
`--profile`, `--print` with passthrough) resolves the profile and project
root, validates credentials, builds the docker invocation, and runs it with
stdio wired through — Ctrl+C reaches the container and `mg` exits with the
agent's exit code. `docs/` is optional: when found (walked up from `$PWD`),
the project is "initialized" and its context is mounted; when absent, the
container boundary falls back to the git root, else `$PWD` — a plain isolated
session with no project context or job workflow.

When `--job <id>` is passed, the mount root is the job's own git worktree,
not the project root: the job is resolved by matching its id_slug against
local branch names (exact then prefix), that branch's worktree is looked up
via `git.WorktreeForBranch`, and the root is reassigned to it so the docs
mount, context-file mount, `.env`-shadow scan, and the primary
`-v ...:/workspace:z` mount all key off the same resolved root. A branch
match with no registered worktree is a **hard error** — no fallback to the
project root, which would silently show the wrong job's content. The one
exception: a project with no local branches at all (not a git repo, or a
fresh repo before its first commit) has no worktrees possible, so `--job`
falls back to a flat scan of `docs/jobs/` and the root is left untouched.

Profile resolution precedence: `--profile` > `--tool` (legacy alias:
`claude-code`→claude-pro, `opencode`→legacy profile-less mode) >
`$MANIGOT_PROFILE` > claude-pro. Legacy profile-less `--tool opencode`
semantics are preserved, including its rejection of `--print`. Auth checks:
claude-pro requires `CLAUDE_CODE_OAUTH_TOKEN` and refuses a set
`ANTHROPIC_API_KEY` (subscription protection); opencode profiles require at
least one of their key vars.

For OpenCode sessions, the launcher also forwards the global theme setting
(`OPENCODE_THEME`, set via `mg theme` — see below) into the container as
`-e OPENCODE_THEME=<value>`, independent of which profile/API key is in use.
`scripts/entrypoint.sh` turns the per-profile `OPENCODE_MODEL` and the global
`OPENCODE_THEME` into two separate OpenCode config files via `{env:...}`
substitution: `~/.config/opencode/opencode.json`'s `model` key for the model,
and a separate `~/.config/opencode/tui.json`'s `theme` key for the theme —
OpenCode's own docs mark `opencode.json`'s top-level `theme` key as legacy/
deprecated (auto-migrated when possible), so the theme is written to its
current, non-deprecated home instead. Each file is written once, only when it
doesn't already exist and its respective env var is set. Claude Code already
respects the host terminal's own theme, so no equivalent exists there.

The `--print` stdout contract: the agent's own output (JSON) on stdout,
*everything else* (diagnostics, banner, warnings) on stderr — Go separates
the streams natively, so there is no fd juggling. `--print` also appends the
`NEEDS-HUMAN-INPUT:` marker definition to the job prompt: an agent that
cannot proceed without a human decision stops and prints a line starting with
exactly that string instead of guessing. This is deliberately not a rule in
`agents/*.md` — interactive sessions never see it (a human is there to answer
questions).

### Clipboard / copying from agent sessions

Copying text from inside an agent session (selecting agent output, the
"copied" indicator) relies on the **OSC 52** terminal escape sequence
(`ESC ] 52 ; c ; <base64> BEL`): the agent CLIs are full-screen TUIs inside
the Docker container, and the only way such an app can write the *host* OS
clipboard is by emitting OSC 52, which flows container TUI → docker pty →
docker client → host terminal (possibly through tmux). mg forwards the bytes
unmodified — the prerequisites are on the terminal side:

- **The host terminal emulator must support OSC 52.** Most modern terminals
  do (iTerm2, kitty, WezTerm, Windows Terminal, recent VTE-based terminals,
  ...); some older or minimal terminals ignore the sequence entirely. A
  terminal without OSC 52 support cannot be fixed by mg.
- **tmux needs `set-clipboard on` when the session runs inside tmux.** tmux
  intercepts all pane output; with `set-clipboard off` (the pre-tmux-3.2
  default and a common deliberate setting) the OSC 52 sequence is stripped and
  the host clipboard is never touched — while the app still shows "copied".
  Fix it with `tmux set -g set-clipboard on` (or `external` on tmux ≥ 3.2,
  which forwards to the outer terminal). mg detects this at session start and
  prints a warning on stderr when it finds `set-clipboard off` (strictly
  read-only — mg never mutates tmux state).

To make the in-container TUIs see the real terminal — the identity many of
them key their copy/clipboard behavior off — mg forwards the host's terminal
environment into the container: `TERM`, `COLORTERM`, `TERM_PROGRAM`,
`TERM_PROGRAM_VERSION`, `VTE_VERSION`, `KITTY_WINDOW_ID`, `TMUX`, `TMUX_PANE`,
`WT_SESSION`, and every `WEZTERM_*` variable, each forwarded only when set and
non-empty on the host (no var set → a byte-identical argv to a build without
this feature). One deliberate exception: `TMUX`/`TMUX_PANE` are forwarded for
Claude Code but **not** for OpenCode — when OpenCode sees `TMUX` set it wraps
its OSC 52 clipboard write in tmux's DCS-passthrough escape, which default
tmux configuration discards entirely (`allow-passthrough` defaults to off),
so the host clipboard is never touched even with `set-clipboard on`. Stripping
`TMUX` makes OpenCode emit plain OSC 52, which tmux's `set-clipboard on`
handles correctly. `mg host` sessions apply the same exception: the OpenCode
child env is filtered of `TMUX`/`TMUX_PANE`, while Claude Code keeps the full
host environment.

### Container cleanup

Container cleanup: sessions run with `--rm`, so containers normally self-remove
on exit; the residue of abnormal ends (killed client, killed pane/window,
host/daemon restart, hung CLI) is pruned automatically before every container
launch — exited containers whose name matches the manigot prefix are removed,
running manigot containers and foreign containers are never touched, and a
prune failure only warns and never aborts the launch. The explicit `mg prune`
command (see Commands) covers on-demand or cron use.

### Session git shim

Every container session (under both CLIs) gets a PATH-first `git` shim,
generated by `scripts/entrypoint.sh`, that allowlists read + commit git
subcommands (`add`, `commit`, `log`, `diff`, `show`, `status`, `rev-parse`,
`branch` read-only, `config` read-only, ...) and refuses everything else —
`worktree`, `branch -d/-D`, `reset`, `clean`, `gc`, `prune`, `reflog`, `push`,
`fetch`, `pull`, `checkout`, `switch`, `restore`, `stash`, `remote`,
`update-ref`, `tag` writes, `merge`, `rebase`, ... — with a clear message, so
an agent can "read the git log and make commits" and nothing more. `git add`
of a pathspec under `.opencode/` or `.claude/` is refused too: the container
mounts `docs/` at those repo-relative paths (see "Session launch" below), and
staging through the mount would commit a stale duplicate of the job — the
host side additionally excludes both paths from git in every manigot-managed
worktree (`git.ExcludeMountTargets`, at job creation and session launch), so
the shim rule is a belt-and-braces second layer. It parses
leading `git -C <dir>`-style global options before deciding and execs the
real git by absolute path (no recursion). It is deliberately a *soft* layer:
a determined agent can exec the real git directly or write the mounted gitdir
— the hard filesystem boundary for non-committing agents is the read-only
git-common-dir mount the session launcher sets up (see below). `mg host`
sessions run the agent CLI directly on the host with no container, so the
shim does not apply there — host sessions have no isolation by design.

### Read-only git mount for non-committing agents

Agents declare whether they commit via a `commit:` frontmatter marker in
`agents/*.md`: `commit: true` for the committing agents (`developer`,
`reviewer` — reviewer commits `verdict.md`, and `quality` — quality commits
its review note to `quality.md`), `commit: false` for the read-only agents.
The session launcher reads the resolved agent's file
(global via `agents/*.md`, project override via `docs/agents/*.md` — the
override wins wholesale, mirroring `mg agents`' own resolution) and mounts the
job's git common dir accordingly: writable (`:z`) for committing agents,
**read-only** (`:ro`) with `GIT_OPTIONAL_LOCKS=0` for non-committing ones, so
a read-only agent physically cannot touch git metadata — the hard boundary
behind the soft shim. The default when no agent is named, the file is missing,
or the marker is absent/unknown is *writable* — a committing agent is never
broken by a missing marker. `GIT_OPTIONAL_LOCKS=0` is required because read
commands that refresh the index (`git status`, `git diff`) would otherwise try
to lock it on the ro mount.

Under OpenCode the `permission:` bash blocks are the first deny layer: the
read-only agents' allowlists no longer carry a broad `git branch *` allow
(which matched `git branch -D <branch>`), and both they and `developer` deny
the destructive git set — `worktree`, `branch -d/-D`, `reset`, `clean`, `gc`,
`prune`, `reflog`, `push`, `fetch`, `pull`, `checkout`, `switch`, `restore`,
`stash`, `remote`, `tag -d`, `update-ref` — with the denies listed after the
allows (OpenCode's rules are last-match-wins). Claude Code ignores these
blocks entirely; it is covered by the git shim instead.

The complete per-agent frontmatter surface is `name:` (must match the
filename), `description:` (one line for `mg agents`/the TUI picker), `tools:`
(the Claude Code allowlist — OpenCode strips it), `commit:` (the git mount
mode above) and `permission:` (the OpenCode block above). Two further keys are
part of the designed guardrail surface but **not yet enforced**: `deny:` — a
command deny-list of OpenCode-style glob patterns (e.g. keep an agent off a
binary the image lacks, like virtualenv), intended to expand into
`permission.bash` denies under OpenCode and PATH-first shims (the git-shim
pattern) under both CLIs — and `network:` — a per-session network isolation
level (`none`/`loopback`/`bridge`, today's default being full egress).
`docs/agent-template.md` is a copy-me reference agent showing every key, every
restriction, and the designed guardrails.

Job-worktree sessions also get read-only overlay mounts shadowing the gitdir's
sensitive subpaths (each skipped when its source is missing — docker would
otherwise create an empty, root-owned directory at the target):
`<GitCommonDir>/hooks`, so an agent cannot plant a hook that would later
execute on host-side git operations (`mg done`, `mg delete`) with host
privileges, and every other job's worktree gitdir under
`<GitCommonDir>/worktrees/`, so a misbehaving agent cannot delete or corrupt
another job's worktree registration (the current job's own worktree gitdir
stays writable — it must, for commits). The worktree-gitdir list is
enumerated once at launch (`git.WorktreeGitDirs` via `git worktree list
--porcelain`), so a job created mid-session is not covered until the next
launch — acceptable, since the overlay list cannot be refreshed from inside
the container.

### Job worktrees are kept committed

Every job-worktree session ends with a host-side sweep-commit of whatever the
agent left uncommitted (`git add -A` + a `[<id>] chore: commit all` commit —
the same message convention as the TUI's "c" key), so `mg done`'s clean-tree
check is never tripped by agent leftovers. This is what makes "every agent
always commits" true without trusting agent instructions: read-only agents'
outputs — the analyst's `tasks.md` above all — are committed by the host
right after their session ends, closing the "no agent feels responsible" gap.
The sweep runs only for job-worktree sessions (`--job`, `mg jobs` launches,
TUI launches, and every `mg jdi` --print invocation) and only when the
container actually ran — a docker launch that never started an agent must not
trigger a commit. It never runs against the main worktree, in either shape
that can resolve `--job` there: the flat-scan fallback (a repo with no local
branches, or a non-git project, where the job's files live directly in the
main project root — there is no job worktree at all) and a pre-worktree job
(the job's branch still checked out in the main worktree itself, an
explicitly supported transitional state — `git.WorktreeForBranch` then
resolves to the main worktree, not a linked one). Sweeping either would commit
the user's own uncommitted work — an unignored `.env` included — onto the job
branch; the gate is "the resolved worktree differs from the invocation root",
which holds for a linked job worktree in every case. Plain sessions, where
the user's own uncommitted work lives, are never swept, and `mg host` sessions
are out of scope (no
isolation by design). The sweep commit deliberately does not match the
`[<id>] verdict:` pattern the mg-jdi state machine counts, so retry and
re-review decisions are unaffected.

### Job lifecycle

Each job lives in its own git worktree (created by `mg job`), at
`<dirname(project)>/.manigot-worktrees/<basename(project)>/<id>_<slug>` —
sibling to the project root — or, when the project root is itself a mount
point (its parent is on a different filesystem), nested at
`<root>/.manigot-worktrees/<id>_<slug>` with that path excluded from the main
worktree's git status via `.git/info/exclude`. The project root is never
switched. A non-git project keeps the pre-worktree fallback: no branch, no
worktree, the scaffold is written straight into `<root>/docs/jobs/`.

The branch is `[<prefix>/]<type>/<id>_<slug>` — the type segment
(`feature`/`fix`/`chore`) with the project's configured `jobBranchPrefix`
(from `.manigot/manigot.json`) prepended. `mg job` pre-checks the composed
branch name against existing refs and fails with a clear error if any
ancestor path segment is already a plain branch (git stores refs as
filesystem paths, so a plain branch `feature` blocks the whole
`feature/...` namespace).

`mg done` (`job.FinishJob`) archives a finished job: clean-tree check, the
archive move + commit inside the job's own worktree, squash merge onto the
configured `baseBranch` (from `.manigot/manigot.json`, falling back to
`origin/HEAD` → `main`), branch delete, and worktree remove — skipped when
the job's branch is checked out in the main worktree itself (a pre-worktree
job), which is also switched back to the base branch. It also removes the
job's `.manigot/jdi-status/` sidecar — the archive keeps the job's docs, and
mg-jdi never runs against an archived job, so the status/run.log sidecar
would otherwise be dead weight forever. Interactive
confirmations go through `internal/cli` prompts with the scripts' original
wording.

`mg delete` (`job.DeleteJob`) permanently deletes a job: worktree
(force-removed, with an explicit "uncommitted changes will be discarded"
warning when dirty) and branch (`-D` — no merge). A non-git project's job is
a plain directory delete. It likewise removes the job's `.manigot/jdi-status/`
sidecar. Same confirmations, including "This cannot be
undone."

Orphaned worktrees — leftover directories under `.manigot-worktrees/` whose
git registration is gone (a `.git` file pointing at a gitdir that no longer
exists, the shape a job scaffolded and then abandoned leaves behind) — are
surfaced by `mg jobs` (`job.DiscoverOrphans`) after the job list, and removed
through either `mg jobs`' interactive "Remove orphaned worktrees?" offer or
`mg delete <name>` (which resolves orphan names the way it resolves job
ids). Removal (`job.RemoveOrphans`) mirrors `git worktree prune` semantics —
it also prunes stale worktree metadata and the abandoned job's
`.manigot/jdi-status/` sidecar — but applies `mg delete`'s
confirmation discipline, including "This cannot be undone." Detection scans
both the sibling and nested `.manigot-worktrees` layouts and never reports a
live worktree (its `.git` file names an existing gitdir) or a standalone
repository (a `.git` directory).

### `mg init`, `mg profiles`, `mg theme`, `mg setup`, `mg agents`

- `mg init` bootstraps a project for the job workflow: the only command that
  works **without** an existing `docs/` (it creates it). Copies
  `project-template/docs/` (`AGENTS.md`, `CLAUDE.md`, and an empty
  `docs/jobs/`) plus `project-template/.manigot/manigot.json` into the target
  (git top-level, else `$PWD`), reporting "already initialized" when `docs/`
  exists, then optionally hands off to `@prompter` (via `--prompt`) to draft
  a concrete `docs/AGENTS.md`.
- `mg profiles [name]` lists the profiles (which are ready, and which
  is the default), sets the default (`MANIGOT_PROFILE` in manigot's `.env`),
  or picks it interactively on a TTY. The TUI's settings screen shares the
  same default.
- `mg theme [name]` shows or sets manigot's global OpenCode theme
  (`OPENCODE_THEME` in manigot's `.env`) — one value shared across every
  OpenCode profile, unlike the per-profile `OPENCODE_*_MODEL` keys — or picks
  it interactively on a TTY. With no name it also prints a reference list of
  OpenCode's known built-in themes (https://opencode.ai/docs/themes/), but
  unlike a profile id, an unrecognized name passed to `mg theme <name>` is
  still accepted — OpenCode itself rejects an invalid name at launch. The
  TUI's settings screen shares the same setting (a free-text field, not a
  fixed-list selector, for the same reason). Claude Code already respects the
  host terminal's own theme, so this has no effect there.
- `mg setup [name] [--check]` configures each profile's credentials into
  manigot's `.env`, auto-applying what it can read off the host (e.g. the
  Claude account from `~/.claude.json`) and letting you paste the rest.
- `mg agents` (alias `mg crew`) lists every agent available to the current
  project — the global `agents/*.md` files, each swapped for its
  `docs/agents/` override when one exists, plus any project-only additions —
  and on a TTY opens an interactive picker (↑/↓/k/j navigate, type to filter,
  enter to choose, esc/q cancel) before launching the session; off a TTY it
  prints the plain list and refuses to pick.

### TUI and `mg jdi`

`mg tui` runs the Bubble Tea TUI in-process. It lists open jobs (from
`git worktree list` via `job.Discover`), opens each job's four files plus a
computed `diff` tab (the `mg diff` quick eyeball — `git log --oneline` +
`git diff --stat` over `<base>...<branch>`, rendered as plain text with one
change per line — git output is not markdown — see the `mg diff` command
above),
edits `brief.md`, launches agents, and runs the job lifecycle directly —
`mg job`, `mg done` and `mg delete` are function calls, not subprocesses, and
the done/delete confirmations are in-TUI views with the scripts' wording. Its
"a" launch-an-agent picker behaves like the CLI's `mg agents` picker:
↑/↓/k/j navigate, type to filter (case-insensitive substring over the
agent's name and description, narrowing the list until esc clears it), enter
launches the highlighted agent, esc clears the filter before cancelling, and
q cancels. The detail view's `t` key opens the job's branch diff in tig on
the host (`mg diff <job> --tig`) in a tmux split pane / new terminal exactly
like an agent launch; the footer hint and the key are gated on tig being
installed on the host.

`mg jdi` drives a job's fixed `@analyst` → `@developer` → `@reviewer`
sequence end to end via the session launcher's `--print` path, stopping at
`verdict.md`'s `## Overall` saying APPROVED, a `NEEDS-HUMAN-INPUT:` marker,
or a bounce-back to `@developer` that still isn't approved after one retry.
It never auto-merges — the human still merges via `mg done`. Every
invocation's captured output and a `running`/`stopped:finished`/
`stopped:needs-human` status are written to a sidecar directory,
`.manigot/jdi-status/<job-name>/` in the *target* project (excluded from the
project's git via `.git/info/exclude`), which the TUI's list-row badge and
log tab poll. A direct CLI run streams to its own terminal and rings the
terminal bell on stop; a TUI-launched run is fully detached, and the TUI
rings the bell itself on its next poll.

Both stop paths also push an opt-in ntfy notification when `NTFY_TOPIC` is
set in manigot's `.env` (a strict no-op when unset — unconfigured behavior
is byte-for-byte unchanged): a success notification (tag `white_check_mark`)
when the run finishes, and a high-priority attention notification (tag
`warning`, priority 4) when it stops needing a human. The next run's start
also pushes an attention notification when it finds the previous run
crashed or killed — the on-disk signature is a `running` status sidecar
stale past `jdiRunningStaleAfter` (a SIGKILLed/OOM-killed process cannot
notify from inside itself, so this next-start check is the self-contained
approximation; an external watchdog is out of scope). `NTFY_URL` defaults to
`https://ntfy.sh` and `NTFY_TOKEN` is optional (sent as a `Bearer` header).
A send failure is a stderr warning, never an abort — `mg jdi` continues
either way.

### Config files

- `manigot/.env` (gitignored) — credentials and defaults for the profiles
  (`CLAUDE_CODE_OAUTH_TOKEN`/`CLAUDE_ACCOUNT_UUID`/`CLAUDE_EMAIL`/
  `CLAUDE_ORG_UUID` for claude-pro, `ZHIPU_API_KEY` + `OPENCODE_ZAI_MODEL`
  for zai, `OPENCODE_API_KEY` + `OPENCODE_GO_MODEL` for opencode-go,
  `OPENCODE_API_KEY` + `OPENCODE_ZEN_MODEL` for opencode-zen,
  `OPENCODE_API_KEY` + `OPENCODE_ZEN_FREE_MODEL` for opencode-zen-free, and
  `MANIGOT_PROFILE` — the default profile shared between CLI and TUI), plus
  `OPENCODE_THEME` — the global OpenCode theme (set via `mg theme`), shared
  across every OpenCode profile rather than pinned per-profile like the
  `OPENCODE_*_MODEL` keys — and the optional ntfy push-notification keys
  `NTFY_URL`/`NTFY_TOPIC`/`NTFY_TOKEN` for `mg jdi` (see the `mg jdi` section
  — `NTFY_TOPIC` unset means no notifications at all).
  Written by `mg setup`/`mg profiles`/`mg theme`/the TUI settings screen; read
  via `config.GetEnv`/`EnvValue`. Never committed.
  Key forwarding is pinned per profile (test-pinned in `internal/session`):
  only `zai` forwards `ZHIPU_API_KEY` into its container, only `opencode-go`
  forwards `OPENCODE_API_KEY`, and `claude-pro` forwards the OAuth token +
  account UUIDs. Consequence for the `shot` render tool (docs/PLAYWRIGHT.md):
  its `--describe` vision layer needs `ZHIPU_API_KEY` in the session env, so
  it works under `zai` today; other profiles get a clear "no key" error and
  fall back to the model-free render report. The perception matrix may later
  widen the forwarding (see the job's probe protocol) — a one-line
  `OpenCodeKeys` change plus its tests, deliberately not made pre-matrix.
- `config/tui-settings.json` (gitignored) — the TUI's personal preferences:
  which editor opens `brief.md`, the recent-activity count, and which
  terminal spawns a session (used only when the TUI is not inside tmux —
  inside tmux the launch is always a tmux split pane, regardless of the
  setting). Missing is not an error — every reader falls
  back to defaults.
- `.manigot/manigot.json` (in the target project, committable) — project
  conventions: `baseBranch` (the ref new job branches are cut from and
  finished jobs are merged into, default `main`) and `jobBranchPrefix` (the
  namespace job branches live under, default empty). Read by `mg job`/`mg
  done`/`mg delete` via `internal/project` and by the TUI.
- `.manigot/` (in the target project) — host-side tooling state only:
  the committable `manigot.json` settings and the gitignored
  `.manigot/jdi-status/` mg-jdi run state (removed when a job is finished,
  deleted, or its orphaned worktree cleaned up). Agents must treat it like
  any other tool-managed state: read the settings file if needed, never edit
  either path by hand.

## Commands
- `make build` — build the image (skips if already built)
- `make rebuild` — force rebuild with no cache, after a Claude Code / OpenCode update
- `make mg` — build the host-side `bin/mg` binary
- `make install` / `make uninstall` — symlink the single `mg` binary into
  `PREFIX/bin` (default `~/.local/bin`)
- `mg` — start an isolated session from inside any project directory; `docs/`
  is optional (see Architecture above)
- `mg --profile <name>` — same, but under the given subscription profile
  (`claude-pro`/`zai`/`opencode-go`/`opencode-zen`/`opencode-zen-free`); `--tool` is accepted as a legacy alias
- `mg profiles [name]` — list the profiles (and which is the default), set the
  default bare `mg` uses, or pick it interactively (no name, on a TTY)
- `mg theme [name]` — show or set manigot's global OpenCode theme
  (`OPENCODE_THEME`, shared across every OpenCode profile), or pick it
  interactively (no name, on a TTY) from a reference list of OpenCode's known
  built-in themes; an unrecognized name is still accepted (OpenCode rejects an
  invalid one at launch)
- `mg setup [name] [--check]` — configure credentials for the profiles,
  interactively, or report status with `--check`
- `mg agents` — list available agents (global + any `docs/agents/`
  overrides/additions) and pick one to start a session in, via an interactive
  picker on a TTY (type to filter, enter to choose; thematic alias:
  `mg crew`, same command/behavior)
- `mg init [--profile <name>]` — bootstrap a project for the job workflow
  (creates `docs/` if absent, optionally hands off to `@prompter`); the only
  job-workflow command that works without an existing `docs/`
- `mg job "<title>" [--type fix|chore] [--base-branch <name>]` — create a job
  dir + branch + worktree (the branch is cut from the configured base branch;
  `--base-branch` overrides it for one invocation)
- `mg jobs` — list open jobs with state and pick one to start a session in,
  via an interactive picker on a TTY (type to filter, enter to choose); the
  session launches in the agent appropriate to the job's stage (analyst /
  developer / reviewer — an explicit `--agent` in passthrough wins). Also
  surfaces orphaned worktrees (leftover `.manigot-worktrees/` dirs with
  no git registration) and offers to remove them
- `mg done <id>` — archive a finished job (squash-merge into the base branch
  and remove its worktree; the merge target is the configured `baseBranch`,
  falling back to the remote default branch when unset)
- `mg delete <id>` — permanently delete a job (worktree + branch, no merge),
  or an orphaned worktree by its name
- `mg prune` — remove orphaned manigot docker containers: every EXITED
  container whose name matches the manigot prefix (the residue of abnormal
  session ends — sessions run with `--rm`), never touching running manigot
  containers or foreign ones. Reports the removed and running counts; exits 1
  with a clear error when docker is missing or the daemon is down. The launch
  path already prunes automatically before every session — this is the
  explicit form for on-demand or cron use
- `mg diff <id> [--full|--name-only|--tig]` — show what a job's branch changed,
  three-dot `<base>...<branch>` per docs/GIT_DIFF.md: `git log --oneline` +
  `git diff --stat` by default, `--name-only` for filenames, `--full` for the
  complete patch, `--tig` to browse it interactively in tig on the host (a
  clear error when tig isn't installed). Open jobs only — `mg done`
  squash-merges the branch away, leaving nothing to diff. The same default
  quick eyeball is also available in the TUI as the detail view's computed
  `diff` tab (key `6`), which resolves the base branch the same way
- `mg tui` — host-side terminal UI for browsing jobs and firing agents
- `mg jdi --job/-j <id> [--profile <name>]` — drive a job's `@analyst` →
  `@developer` → `@reviewer` sequence end to end, unattended, under the given
  subscription profile (default `claude-pro`)
  (thematic alias: `mg made-man`, same command/behavior)
- `mg host` — run a session directly on the host, without the docker
  container: the profile's agent CLI (`claude`/`opencode`) runs as-is from
  the resolved project root (the job's worktree with `--job`), with the
  profile's credentials in its environment and the job prompt naming the
  job's host path. The CLI must be installed on the host, and it keeps its
  normal per-tool confirmation prompts — the container path's auto-approval
  flags (`--dangerously-skip-permissions`/`--auto`) are deliberately not
  passed, since host sessions have no isolation. The global agents are still
  available: mg delivers the checkout's `agents/*.md` into the CLI's own host
  config dir — symlinks for Claude Code, converted copies for OpenCode (which
  hard-errors on the list form) — never clobbering an existing name. Global
  skills are delivered the same way: symlinked skill dirs for Claude Code,
  copied dirs for OpenCode, never clobbering an existing skill name. The
  system-wide meta prompt (`prompts/meta.md`) is delivered into the CLI's own global
  instruction file (`~/.claude/CLAUDE.md` for Claude Code,
  `~/.config/opencode/AGENTS.md` for OpenCode) as a copy — never a symlink,
  since a symlink would let Claude's `/memory` writes and agent edits land
  back in the checkout — and never clobbering an existing host file. For work
  that must touch the host itself (thematic alias: `mg wild`, same
  command/behavior)

## Job workflow
Each job lives in `docs/jobs/<id>_<slug>/` with four files:
`brief.md` (what/why, filled in by the user), `tasks.md` (`@analyst`),
`implementation.md` (`@developer`), `verdict.md` (`@reviewer` / `@security`).
A branch `[<prefix>/]feature|fix|chore/<id>_<slug>` is created alongside it
(the optional prefix comes from the project's `jobBranchPrefix` setting),
checked out in the job's own git worktree — every job gets its own directory,
so multiple jobs — interactive or autonomous — can run in parallel, and the
project root stays on the base branch in steady state.

Typical feature flow: `mg job` → fill `brief.md` → `@owner` →
`@analyst` → review `tasks.md` → `@developer` per task → `@reviewer` →
`@security` → fix and re-review → merge → mark `brief.md` status `done`.
Bug fixes skip the `@owner`/`@analyst` steps and go straight to
`@developer`.

`mg jdi` automates the middle of that flow — `@analyst` → `@developer` →
`@reviewer`, the same fixed sequence for every job `type` — without a human
manually triggering each stage. `@owner` and `@security` are not
part of it; both remain ordinary manually-launched agents, unaffected. It
stops (never auto-merging) when `verdict.md`'s `## Overall` says APPROVED, or
hands control back to a human when: the one allowed bounce back to
`@developer` after a REJECTED/NEEDS WORK verdict still isn't approved, an
agent prints the `NEEDS-HUMAN-INPUT:` marker (see the `--print` bullet
above), or the same agent makes no progress on two consecutive runs. `mg
jdi --profile <name>` selects which subscription profile drives the agent
sequence, defaulting to `claude-pro` when unset; the TUI's `j` keybinding
passes its settings profile — the same shared `MANIGOT_PROFILE` — the same
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
- `scripts/entrypoint.sh` is the only bash in the repo and must stay
  self-contained — nothing in the container image may depend on Go. Its
  internal key list is a container-side safety net only; the Go session
  launcher pre-validates keys before launch, so drift between them is
  harmless.
- Keep `agents/*.md` and `project-template/docs/AGENTS.md` in sync with
  whatever this file documents — they're meant to describe the same system
  (the skills mechanism and the shipped skills included)
- When scope is unclear: ask, don't guess
- Do not refactor things unrelated to the current task
