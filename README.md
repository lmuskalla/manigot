<p align="center">
  <img src="assets/manigot.png" />
</p>

Isolated agent environment per project. One Docker image, real filesystem
containment, structured agent workflow.

Runs a session under one of five **subscription profiles** — `claude-pro`
(Claude Code, billed to your Claude Pro/Max subscription), `zai` (OpenCode,
billed to your Z.AI Coding Plan), `opencode-go` (OpenCode, billed to the
OpenCode Go subscription), `opencode-zen` (OpenCode, billed to OpenCode
Zen — DeepSeek V4 Flash, pay-as-you-go), and `opencode-zen-free` (OpenCode,
billed to OpenCode Zen — DeepSeek V4 Flash Free costs no credits). Pick per
session with
`--profile`, set the default
used by bare `mg` with `mg profiles`, and configure credentials with
`mg setup`.

---

## Where everything lives

### The manigot repo

```
manigot/
  Dockerfile              ← build once, rebuild on Claude Code / OpenCode updates
  Makefile                ← build / rebuild / install / help; `make mg` builds bin/mg
  src/                    ← the Go module (module root: go.mod, cmd/, internal/)
    cmd/mg/               ← the one host-side binary ('mg'); every command is a subcommand
                            (session, profiles, theme, setup, agents, job, done, delete, diff, init, tui, jdi)
    internal/             ← the host-side logic as Go packages
      session/            ← docker launch construction (mounts, env, profiles)
      job/                ← job lifecycle: create / finish / delete
      git/                ← worktree/branch operations
      ui/                 ← the Bubble Tea TUI (reached via `mg tui`)
      orchestrate/        ← the `mg jdi` state machine
      config/             ← profiles table, .env, settings
      ...                 ← agentlist, cli, editor, home, launch, markdown, project
  scripts/                ← one script only
    entrypoint.sh         ← runs inside the container before the agent CLI starts
    shot.js               ← the `shot` render tool (baked into the image as /usr/local/bin/shot;
                            renders a URL to PNG + model-free render report — see docs/PLAYWRIGHT.md)
  bin/                    ← built binaries (gitignored) — `make mg` produces bin/mg
  .env                    ← your credentials + default profile (gitignored, never committed)
  .gitignore
  README.md
  assets/                 ← quotes.json (flavor quotes)
  agents/                 ← global agents, mounted into the container (and delivered for mg host)
    analyst.md
    developer.md
    reviewer.md
    security.md
    owner.md
    designer.md
    quality.md
    prompter.md
    mentor.md
    architect.md
    devops.md
    sysadmin.md
    chat.md
  skills/                 ← global skills (<name>/SKILL.md), delivered the same way as agents
  prompts/                ← prompt material delivered into every session
    meta.md               ← the system-wide meta prompt (see "Meta prompt")
  project-template/       ← copy this into each new project to get started (via `mg init`)
    docs/
      AGENTS.md
      CLAUDE.md
      jobs/               ← empty; `mg job` fills it
```

### Each project

```
your-project/
  docs/                         ← mounted into the container at runtime
                                  (at /workspace/.claude for Claude Code,
                                   /workspace/.opencode for OpenCode)
    AGENTS.md                   ← project context, loaded by whichever tool you run
                                  (CLAUDE.md still works as a fallback)
    agents/                     ← optional: override a global agent for this project only
      developer.md              ← same filename = this replaces the global one
    skills/                     ← optional: project skills (<name>/SKILL.md), override global ones
    jobs/                     ← one directory per job
        a3f9k2_add-gallery/
          brief.md
          tasks.md
          implementation.md
          verdict.md
        7bx1q4_fix-uploads/
          brief.md
          tasks.md
          implementation.md
          verdict.md
  [rest of your project]
```

The agent can only see your project directory. SSH keys, `.env` files,
other projects — not mounted, not reachable. `.env` files inside the project
are shadowed with `/dev/null` at container start — host files are never touched.

`docs/` is optional. Run `mg` in a project with no `docs/` directory and you
still get a fully isolated, sandboxed session — just without project context
or the job workflow. Add `docs/` (see [Per-project setup](#per-project-setup))
whenever you want those too. This makes `mg` a drop-in replacement for
running `claude`/`opencode` directly, in any project, initialized or not.

---

## Setup (once)

### Profiles

A profile bundles an agent CLI with one of your subscriptions:

| Profile | Agent CLI | Billing | Credential in `.env` |
|---|---|---|---|
| `claude-pro` | Claude Code | Claude Pro/Max subscription | `CLAUDE_CODE_OAUTH_TOKEN` + account UUIDs |
| `zai` | OpenCode | Z.AI Coding Plan | `ZHIPU_API_KEY` |
| `opencode-go` | OpenCode | OpenCode Go subscription | `OPENCODE_API_KEY` |
| `opencode-zen` | OpenCode | OpenCode Zen (DeepSeek V4 Flash, pay-as-you-go) | `OPENCODE_API_KEY` |
| `opencode-zen-free` | OpenCode | OpenCode Zen (DeepSeek V4 Flash Free) | `OPENCODE_API_KEY` |

Which provider key rides into the container is pinned per profile: only `zai`
forwards `ZHIPU_API_KEY`, only `opencode-go` forwards `OPENCODE_API_KEY`, and
`claude-pro` forwards the OAuth token + account UUIDs. This matters for the
`shot` render tool's `--describe` vision layer (see `docs/PLAYWRIGHT.md`): it
needs `ZHIPU_API_KEY` in the session env, so it is available under `zai`
today. The other profiles get a clear "no key" error from `shot --describe`
and rely on the model-free render report instead; the perception matrix may
later widen the key forwarding (see the job's probe protocol).

The quickest way to get going:

```bash
cd manigot/
make build
mg setup              # interactive wizard: walks through each profile,
                      # auto-applying what it can read off your host (e.g.
                      # your Claude account from ~/.claude.json) and letting
                      # you paste the rest into manigot/.env
mg profiles           # see the profiles, which are ready, and the default;
                      # on an interactive terminal it then lets you pick the
                      # default right there
mg profiles zai       # make bare `mg` use the zai profile
```

`mg setup <name>` sets up a single profile; `mg setup --check` reports status
non-interactively. Everything ends up in the same gitignored `manigot/.env`
that the session launcher reads. Manual instructions for each profile, if you'd
rather fill `.env` by hand:

```bash
# 1. Build the image
cd manigot/
make build

# 2. Put the launchers on your PATH
make install                            # ~/.local/bin — no sudo needed (default)
# make install PREFIX=/usr/local        # ...or a system-wide location (may need sudo)
```

### `claude-pro` — Claude Code, Claude Pro subscription

```bash
# 1. Extract your account info from your host (requires Claude Code installed locally)
cat ~/.claude.json | python3 -c "
import json,sys
d=json.load(sys.stdin)
print(json.dumps(d.get('oauthAccount'), indent=2))"

# 2. Add credentials to .env
# Get CLAUDE_CODE_OAUTH_TOKEN by running: claude setup-token
cat > manigot/.env << EOF
CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-...
CLAUDE_ACCOUNT_UUID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
CLAUDE_EMAIL=your@email.com
CLAUDE_ORG_UUID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
EOF
```

### `zai` — OpenCode, Z.AI Coding Plan

OpenCode is already in the image — no extra install. It authenticates from
environment variables, so add your Z.AI Coding Plan key to the same `.env`:

```bash
cat >> manigot/.env << EOF
ZHIPU_API_KEY=xxxxx.xxxxx      # Z.AI Coding Plan key

# optional: which model this profile defaults to, as provider/model
OPENCODE_ZAI_MODEL=zai-coding-plan/glm-5.2
EOF
```

### `opencode-go` — OpenCode, OpenCode Go subscription

OpenCode Go uses your OpenCode API key from [opencode.ai/auth](https://opencode.ai/auth),
billed against the Go subscription (the same key works for Zen — billing
follows your subscription):

```bash
cat >> manigot/.env << EOF
OPENCODE_API_KEY=sk-...        # your key from https://opencode.ai/auth

# optional: which model this profile defaults to, as provider/model.
# Go model ids use the opencode-go/ prefix (e.g. opencode-go/deepseek-v4-flash)
OPENCODE_GO_MODEL=opencode-go/glm-5.2
EOF
```

### `opencode-zen` — OpenCode, OpenCode Zen

OpenCode Zen uses the same OpenCode API key from [opencode.ai/auth](https://opencode.ai/auth)
as `opencode-go` — billing follows your subscription. This profile's DeepSeek
V4 Flash model is billed pay-as-you-go against your Zen account credits:

```bash
cat >> manigot/.env << EOF
OPENCODE_API_KEY=sk-...        # your key from https://opencode.ai/auth

# optional: which model this profile defaults to, as provider/model.
# Zen's (billed) DeepSeek V4 Flash model is opencode/deepseek-v4-flash
OPENCODE_ZEN_MODEL=opencode/deepseek-v4-flash
EOF
```

### `opencode-zen-free` — OpenCode, OpenCode Zen (free)

Same OpenCode API key as `opencode-zen`. The free DeepSeek V4 Flash model
costs no credits, so this profile works with a key alone:

```bash
cat >> manigot/.env << EOF
OPENCODE_API_KEY=sk-...        # your key from https://opencode.ai/auth

# optional: which model this profile defaults to, as provider/model.
# Zen's free DeepSeek V4 Flash model is opencode/deepseek-v4-flash-free
OPENCODE_ZEN_FREE_MODEL=opencode/deepseek-v4-flash-free
EOF
```

The `--profile` selector and `mg profiles`/`mg setup` are the supported way to
manage these. `--tool claude-code|opencode` is still accepted as a legacy
alias: `--tool claude-code` behaves exactly like `--profile claude-pro`, and
`--tool opencode` keeps its old behavior of forwarding every configured
OpenCode key and using `OPENCODE_MODEL` unchanged.

Note: `ANTHROPIC_API_KEY` is rejected on the claude-pro path — it would
override your subscription and bill per token. On the OpenCode profiles it is
ignored (only the profile's own key is forwarded).

### Theme (OpenCode)

If you run a themed terminal (Nord, Gruvbox, ...) you probably want OpenCode
to match it instead of booting with its own black-and-default look. Unlike
the model, the theme is a **global** setting — one value shared across every
OpenCode profile:

```bash
mg theme           # show the current theme + a reference list of OpenCode's
                    # known built-in themes (nord, tokyonight, gruvbox, ...);
                    # on an interactive terminal, also offers a picker
mg theme nord       # set it — any name is accepted, even one not in the
                    # reference list (OpenCode itself rejects an invalid name
                    # at launch)
```

This writes `OPENCODE_THEME` into the same `manigot/.env`, which the session
launcher forwards into OpenCode container sessions; `scripts/entrypoint.sh`
turns it into OpenCode's `~/.config/opencode/tui.json` (`{"theme": "..."}`) —
a separate file from the model's `opencode.json`, since OpenCode's own docs
mark `opencode.json`'s `theme` key as legacy/deprecated. The TUI's settings
screen has the same field. Claude Code already respects your terminal's own
theme, so there's nothing to configure there.

### The installed commands

`make install` puts a single command, `mg`, on your `PATH`. It dispatches on
its first argument:

| command | does |
|---|---|
| `mg` | start a session in the current project (works with or without `docs/` — see above); uses the default profile, or the one given with `--profile` |
| `mg profiles` | list the profiles (which are ready, and which is the default) — `mg profiles <name>` sets the default used by bare `mg`, or pick it interactively on a TTY; the TUI's settings screen shares the same default |
| `mg theme` | show the global OpenCode theme + a reference list of known built-in themes — `mg theme <name>` sets it (any name accepted), or pick it interactively on a TTY; the TUI's settings screen shares the same setting |
| `mg setup` | configure credentials for your subscriptions, interactively — `mg setup <name>` for one, `mg setup --check` for a non-interactive status report |
| `mg agents` | list available agents (global + any `docs/agents/` overrides/additions) and pick one to start a session in, via an interactive picker on a TTY (type to filter, enter to choose; thematic alias: `mg crew`, same command/behavior) |
| `mg init` | bootstrap this project for the job workflow — copies `docs/` from the template (unless already present) and optionally hands off to `@prompter` to draft `docs/AGENTS.md`; the one command that works **without** `docs/` already existing |
| `mg job` | create a job: directory + branch, checked out in the job's own worktree (off `main`); needs `docs/` |
| `mg jobs` | list open jobs with state and pick one to start a session in, via an interactive picker on a TTY (type to filter, enter to choose); the session launches in the agent appropriate to the job's stage (analyst/developer/reviewer); also surfaces orphaned worktrees (leftover `.manigot-worktrees/` dirs with no git registration) and offers to remove them; needs `docs/` |
| `mg done` | archive a finished job — merges it into the base branch and removes its worktree; needs `docs/` |
| `mg delete` | permanently delete a job (worktree + branch, no merge), or an orphaned worktree by its name; needs `docs/` |
| `mg prune` | remove orphaned docker containers left behind by abnormal session ends — exited manigot-* containers only; running manigot-* and foreign containers are never touched. Prints the removed and running counts (or "Nothing to prune."), and exits 1 with a clear error when docker is missing or the daemon is down. The launch path prunes automatically before every session — this is the explicit form for on-demand or cron use |
| `mg diff` | show what a job's branch changed, three-dot against the base branch — log + `diff --stat` by default, `--name-only` for filenames, `--full` for the complete patch, `--tig` to browse it in tig on the host; needs `docs/` |
| `mg tui` | the terminal UI, running in-process; needs `docs/` |
| `mg jdi` | drive a job's `@analyst` → `@developer` → `@reviewer` sequence unattended, in-process; needs `docs/` (thematic alias: `mg made-man`, same command/behavior) |
| `mg host` | run a session directly on the host, without the docker container — the profile's CLI runs as-is from the project root, so the agent can touch the host itself (thematic alias: `mg wild`, same command/behavior) |
| `mg --help` | print usage and exit — no docker/auth setup touched |

`mg` is a symlink back into the repo, so `git pull` updates it. `make
uninstall` removes it again.

### Installing without symlinks

If you would rather not use `make install` at all, define a shell alias
instead:

```bash
# ~/.zshrc or ~/.bashrc
alias mg='~/code/manigot/bin/mg'
```

The binary locates its own checkout (for `manigot/.env`, `config/`, `agents/`,
`assets/` and `project-template/`) from its own location, so an alias to
`bin/mg` needs no environment variables at all. `$MANIGOT_HOME` is still
honored as an explicit override if the binary is copied somewhere unusual.

---

## Per-project setup

```bash
cd your-project/
mg init
```

`mg init` copies `docs/AGENTS.md`, `docs/CLAUDE.md`, and an empty `docs/jobs/`
from `project-template/docs/` into your project (skipping the copy — and
reporting "already initialized" — if `docs/` already exists), then offers to
hand off to `@prompter` to read your project and draft a concrete
`docs/AGENTS.md` for you (`y`/`N`, defaults to no). Run it again any time to
get that offer without re-copying. Add `--profile zai` (or any other profile)
to run the prompter hand-off under that subscription instead of claude-pro.

Equivalent manual steps, if you'd rather not run the prompter or want more
control:

```bash
# Copy the template into your project
cp -r manigot/project-template/docs/ your-project/docs/

# Fill in your project context
$EDITOR your-project/docs/AGENTS.md
```

Either way, that's it. The global agents and skills come from the manigot
checkout and are delivered into sessions automatically (mounted into the
container, or delivered into the host CLI's config for `mg host`) — nothing
else to copy.

`docs/AGENTS.md` is the one project context file, and it works for both tools:
manigot mounts it read-only at whatever path the selected CLI reads context
from (`/workspace/AGENTS.md` for OpenCode, `/workspace/.claude/CLAUDE.md` for
Claude Code). Projects that still have a `docs/CLAUDE.md` keep working — it is
used as a fallback when `docs/AGENTS.md` is absent.

---

## Usage

```bash
# Bootstrap a project for the job workflow (one-time, see Per-project setup)
cd your-project/
mg init

# Start a manigot session (run from anywhere, in any project — docs/ optional)
mg                            # the default profile (claude-pro), or whatever `mg profiles` set
mg --profile zai              # this session on your Z.AI Coding Plan
mg --profile opencode-go      # this session on your OpenCode Go subscription
mg --profile opencode-zen     # this session on OpenCode Zen (billed model)
mg --profile opencode-zen-free # this session on OpenCode Zen (free model)
mg profiles                   # list profiles + the current default (then pick a new one on a TTY)
mg theme nord                 # set the global OpenCode theme (shared across every OpenCode profile)
mg setup --check              # which profiles are ready to use

# List all commands
mg --help

# Start straight in an agent, or on a job
mg --agent analyst
mg -a analyst                      # same as --agent analyst
mg --profile zai --job a3f9k2
mg -j a3f9k2                       # same as --job a3f9k2

# Start an agent with an ad-hoc initial prompt, outside the job workflow
mg --agent prompter --prompt "help me write a good project prompt"

# Create a new job
mg job "add image gallery block"
mg job "fix tenant isolation on media uploads" --type fix
mg job "upgrade dependencies" --type chore

# Archive a finished job
mg done a3f9k2

# Remove orphaned docker containers left behind by abnormal session ends
mg prune

# Drive a job unattended: @analyst -> @developer -> @reviewer, end to end
mg jdi --job a3f9k2
mg jdi -j a3f9k2                     # same as --job a3f9k2

# Run a session on the host — no docker container (work that must touch the host)
mg host
mg host --job a3f9k2              # same flags as a docker session, host-pathed job prompt
mg wild --profile zai             # thematic alias, same command/behavior
```

Three ways to seed a session's initial prompt: `--job <id>` (the job's
`brief.md`, for the job workflow), `--agent <name>` (starts the CLI directly
in that agent, with no prompt text of its own), and `--prompt "…"` (a
free-form initial prompt for an ad-hoc, non-job session — what `mg init` uses
to hand off to `@prompter`). `--job` and `--agent` also accept the short forms
`-j <id>` and `-a <name>`. All three are tool-neutral: the session launcher
routes the prompt to the right place per tool (positional for Claude Code,
`--prompt` for OpenCode) regardless of which of `--job`/`--prompt` you used to
set it. `--job` and `--prompt` can't both be honored at once — if you pass
both, the job prompt wins.

---

## Choosing a profile

A profile bundles the agent CLI with the subscription it is billed against.
`--profile claude-pro` (default), `--profile zai`, `--profile opencode-go`,
`--profile opencode-zen`, or `--profile opencode-zen-free`. What differs per
profile:

| | `claude-pro` | `zai` / `opencode-go` / `opencode-zen` / `opencode-zen-free` |
|---|---|---|
| CLI in container | `claude` | `opencode` |
| Auth | `CLAUDE_CODE_OAUTH_TOKEN` + account UUIDs (Claude subscription) | `ZHIPU_API_KEY` / `OPENCODE_API_KEY` |
| Onboarding | bypassed by writing `~/.claude.json` | nothing to bypass |
| Permissions | auto-approved via `--dangerously-skip-permissions` | auto-approved via `--auto` |
| Global agents | `~/.claude/agents/` | `~/.config/opencode/agents/` |
| Global skills | `~/.claude/skills/` | `~/.config/opencode/skills/` |
| Global meta prompt | `~/.claude/CLAUDE.md` | `~/.config/opencode/AGENTS.md` |
| `docs/` mounted at | `/workspace/.claude` | `/workspace/.opencode` |
| Project agents | `/workspace/.claude/agents/` | `/workspace/.opencode/agents/` |
| Project skills | `/workspace/.claude/skills/` | `/workspace/.opencode/skills/` |
| `docs/AGENTS.md` mounted at | `/workspace/.claude/CLAUDE.md` | `/workspace/AGENTS.md` |
| Initial job prompt | positional argument | `--prompt` |
| Billing | your Claude subscription | your Z.AI Coding Plan / OpenCode Go / OpenCode Zen subscription |
| Non-interactive (`--print` / `mg jdi`) | supported | supported |

Both tools get the same `agents/*.md` files, delivered from the manigot
checkout into every session rather than baked into the image at build time.
For a container session the global agents dir is mounted read-only at the
CLI's global agent location (`~/.claude/agents/` for Claude Code,
`~/.config/opencode/agents/` for OpenCode — see the table above), so the
container uses the host's agents but cannot modify them; project
`docs/agents/` overrides a same-named global agent. The OpenCode copies are
converted from the same sources with the `name` and `tools` frontmatter keys
removed, since OpenCode derives the agent name from the filename and uses a
different schema for tool permissions. Custom project agents in
`your-project/docs/agents/` are written in the same list form and converted
the same way at session launch — manigot strips `name`/`tools` from the
mounted copies before an OpenCode session sees them (OpenCode hard-errors on
the list form, so this is what keeps one file working under both CLIs).
Your `docs/agents/` source files are never modified.

Skills are delivered the same way, at the same global + project locations
(see the table above). Global skills (`skills/` in the manigot checkout) are
mounted read-only at the CLI's global skills dir — verbatim for Claude Code,
a staged copy for OpenCode (skills need no conversion, but the staged dir
keeps the CLI's skills path a fresh, disposable snapshot) — so they are
loaded at startup by every agent and by agentless invocations alike. Project
skills in `docs/skills/` ride the `docs/` mount into the container and
override a global skill of the same name — no conversion, no shadow mount,
because skills are plain directories both CLIs read natively. Your
`docs/skills/` source files are never modified.

The system-wide meta prompt (`prompts/meta.md` in the manigot checkout — see
["Meta prompt"](#meta-prompt)) is delivered the same way, at each CLI's
*global instruction* file: `~/.claude/CLAUDE.md` for Claude Code,
`~/.config/opencode/AGENTS.md` for OpenCode. For a container session the
checkout file is mounted read-only at that path — plain markdown, so no
conversion and no temp dir; a checkout without `prompts/meta.md` simply yields no
mount.

The read-only agents — `@reviewer`, `@security`, `@analyst` and `@owner` —
express their restriction in both tools' schemas: the Claude-Code `tools:`
list under Claude Code, and an OpenCode `permission:` frontmatter block that
the strip leaves intact, so the same `agents/*.md` source file enforces
read-only under OpenCode too. Under OpenCode the reviewer/security/analyst
can edit only their own report file (`verdict.md`/`tasks.md`) and run only
read-only git commands; `task`, `webfetch`, `websearch` and `question` are
denied. Everything is auto-approved except these explicit denies — the
container is isolated and ephemeral, so this is safe: Claude Code runs with
`--dangerously-skip-permissions`, OpenCode with `--auto` (which auto-approves
anything not explicitly denied; the container's opencode config defines no
denies). `mg host` runs without the container and therefore without these
flags — see [Host mode](#host-mode-mg-host).

---

### Host mode (`mg host`)

Most sessions run inside the isolated Docker container. `mg host` (thematic
alias: `mg wild`) is the deliberate exception: it launches the profile's
agent CLI directly on the host — no container, no image, no mounts — as a
launcher for work that must touch the host itself.

- It reuses the same session machinery: `--profile`, `--agent` (`-a`),
  `--job` (`-j`), `--prompt` and passthrough behave exactly as in a docker
  session, and the credentials resolve the same way — the profile's keys go
  into the CLI's environment, nothing is written to your config.
- The CLI runs from the resolved project root (the job's worktree with
  `--job`), and the job prompt names the job's **host** path.
- The CLI must be installed on the host — the docker path has both CLIs in
  the image. `mg host` fails with a clear error if it is not.
- **No auto-approval flags.** The container path starts Claude Code with
  `--dangerously-skip-permissions` and OpenCode with `--auto`, safe only
  because the container is isolated and ephemeral. On the host there is no
  isolation, so `mg host` deliberately does not pass them: the CLI keeps its
  normal per-tool confirmation prompts and you supervise every action.
- **Agents.** manigot's global agents are available to `mg host` too: at
  launch mg delivers the checkout's `agents/*.md` files into the host CLI's
  own global agents dir (`~/.claude/agents/` for Claude Code,
  `~/.config/opencode/agents/` for OpenCode), so `--agent` works without the
  image. Claude Code gets symlinks to the live checkout files; OpenCode gets
  converted copies — the `name`/`tools` frontmatter keys stripped and
  `permission:` passed through, the same conversion the container path
  applies, since OpenCode hard-errors on the list-form `tools:` key. mg never
  clobbers existing host agent config — a name you already have in that dir is
  left untouched (your own agent wins).
- **Skills.** Global skills are available to `mg host` too, delivered the same
  way: at launch mg puts the checkout's `skills/` dirs into the host CLI's own
  global skills dir (`~/.claude/skills/` for Claude Code,
  `~/.config/opencode/skills/` for OpenCode) — symlinked skill dirs for Claude
  Code, copied dirs for OpenCode — and never clobbers an existing host skill
  of the same name (your own skill wins).
- **Meta prompt.** The system-wide meta prompt (`prompts/meta.md`) is delivered into
  the host CLI's own global instruction file (`~/.claude/CLAUDE.md` for Claude
  Code, `~/.config/opencode/AGENTS.md` for OpenCode) as a **copy — never a
  symlink** (a symlink would let Claude's `/memory` writes and agent edits land
  back in the manigot checkout) — and never clobbers an existing host file
  (your own file wins).
- **OpenCode model.** The zai/opencode-go/opencode-zen/opencode-zen-free
  profiles' plan model is forwarded via opencode's `--model` flag; mg never
  writes your host's opencode config.
- **`--print` stays a container path.** `mg host --print` is rejected with a
  clear error — non-interactive runs (and `mg jdi`) still use the container.

---

## Clipboard / copying from agent sessions

Copying text from inside an agent session (selecting agent output, the
"copied" indicator) relies on the **OSC 52** terminal escape sequence
(`ESC ] 52 ; c ; <base64> BEL`): the agent CLIs are full-screen TUIs inside
the Docker container, and the only way such an app can write the *host* OS
clipboard is by emitting OSC 52, which flows container TUI → docker pty →
docker client → host terminal (possibly through tmux). mg forwards the bytes
unmodified — the prerequisites are on the terminal side:

- **Your terminal emulator must support OSC 52.** Most modern terminals do
  (iTerm2, kitty, WezTerm, Windows Terminal, recent VTE-based terminals, ...);
  some older or minimal terminals ignore the sequence entirely. A terminal
  without OSC 52 support cannot be fixed by mg.
- **tmux needs `set-clipboard on` when the session runs inside tmux.** tmux
  intercepts all pane output; with `set-clipboard off` (the pre-tmux-3.2
  default and a common deliberate setting) the OSC 52 sequence is stripped and
  your host clipboard is never touched — while the app still shows "copied".
  Fix it with `tmux set -g set-clipboard on` (or `external` on tmux ≥ 3.2,
  which forwards to the outer terminal). mg detects this at session start and
  prints a warning on stderr when it finds `set-clipboard off` — strictly
  read-only, mg never mutates your tmux state.

To make the in-container TUIs see the real terminal — the identity many of
them key their copy/clipboard behavior off — mg forwards the host's terminal
environment into the container: `TERM`, `COLORTERM`, `TERM_PROGRAM`,
`TERM_PROGRAM_VERSION`, `VTE_VERSION`, `KITTY_WINDOW_ID`, `TMUX`, `TMUX_PANE`,
`WT_SESSION`, and every `WEZTERM_*` variable, each forwarded only when set and
non-empty on the host. One deliberate exception: `TMUX`/`TMUX_PANE` are
forwarded for Claude Code but **not** for OpenCode — when OpenCode sees `TMUX`
set it wraps its OSC 52 clipboard write in tmux's DCS-passthrough escape,
which default tmux configuration discards entirely (`allow-passthrough`
defaults to off), so your host clipboard is never touched even with
`set-clipboard on`. Stripping `TMUX` makes OpenCode emit plain OSC 52, which
tmux's `set-clipboard on` handles correctly. `mg host` sessions apply the same
exception: the OpenCode child env is filtered of `TMUX`/`TMUX_PANE`, while
Claude Code keeps the full host environment.

---

## Agents

Fourteen agents are available globally in every project. Call them with `@name`
in your session, or run `mg agents` (or its thematic alias, `mg crew`) from
the host to list them (with any `docs/agents/` overrides) and pick one —
via the interactive picker on a TTY — to start straight into.

| Agent | Role | Tools (Claude Code) |
|---|---|---|
| `@analyst` | Breaks a brief into atomic tasks | read-only |
| `@developer` | Implements one task at a time | read + write |
| `@reviewer` | Verifies implementation against tasks | read-only |
| `@security` | Audits for vulnerabilities | read-only |
| `@owner` | Evaluates features from the user's perspective | read-only |
| `@designer` | Reviews and directs UI/UX — typography, colour, layout | read + write |
| `@quality` | Reviews code quality — readability, DRY, modularity | read + write |
| `@prompter` | Crafts and refines prompts for LLMs and agents | read + write |
| `@mentor` | Grounded tech mentor for skill growth and sustainable practice | read-only |
| `@architect` | Plans how to best build a system — stack, components, deployment | read-only |
| `@devops` | Expert for pipelines and getting things running — CI/CD, builds | read + write |
| `@sysadmin` | Manages and administers servers — services, TLS, updates, uptime | read + write |
| `@chat` | General chat assistant, like ChatGPT — conversational and advisory | read-only |
| `@git-solver` | Git expert for tricky states — broken worktrees, conflicts, cleanup | read + write |

The Tools column is enforced under Claude Code; the read-only agents are
enforced under OpenCode too, via the `permission:` frontmatter they carry —
see [Choosing a profile](#choosing-a-profile).

To override an agent for a specific project, create a file with the same name
in `your-project/docs/agents/`. Project agents take precedence over global ones.
Write them in the same format as the built-ins (`name:`, `description:`,
`tools: Read, Grep, ...`) — the OpenCode copy is generated from that file at
launch, so you never need to hand-write OpenCode's object form.

### Agent frontmatter and guardrails

Every agent is a single markdown file whose frontmatter carries both its
identity and its restrictions. The keys manigot understands:

| Key | Effect | Enforced |
|---|---|---|
| `name:` | the agent's `@name` (must match the filename) | both CLIs |
| `description:` | one line for `mg agents` and the TUI picker | both CLIs |
| `tools:` | Claude Code tool allowlist (`Read, Grep, Glob, Write, Edit, Bash`) | Claude Code |
| `commit:` | `true` → writable git mount; `false` → read-only gitdir, the hard boundary | both CLIs |
| `permission:` | OpenCode deny/allow block (`edit`/`bash`/`task`/`webfetch`/`websearch`/`question`), last-match-wins | OpenCode |
| `deny:` | command deny-list (e.g. keep an agent off a binary the image lacks) | *designed, not yet enforced* |
| `network:` | session network isolation (`none`/`loopback`/`bridge`) | *designed, not yet enforced* |

Beyond the frontmatter, every session is guarded regardless of agent: the git
shim restricts git to read + commit, `commit: false` agents get the job's
git-common-dir mounted read-only (with `GIT_OPTIONAL_LOCKS=0`), the gitdir's
`hooks/` and other jobs' worktree gitdirs are read-only overlays, `.env` files
are shadowed to `/dev/null`, profile credentials are pinned per profile, only
committing agents may run `shot`, and `mg jdi` stops on a `NEEDS-HUMAN-INPUT:`
marker or a verdict that isn't APPROVED.

`docs/agent-template.md` is a copy-me reference agent showing every frontmatter
key, every restriction, and the designed guardrails — copy it to
`docs/agents/<name>.md` for a project agent or `agents/<name>.md` for a global
one.

---

## Skills

Skills are packaged instructions any agent can load on demand. Unlike agents
(which are bound to a named `@agent`), skills are loaded globally at startup
by both CLIs — `~/.claude/skills/` for Claude Code,
`~/.config/opencode/skills/` for OpenCode — independent of the active agent,
so a skill is available to **every** agent and to invocations that name no
agent.

A skill is a **directory** containing a `SKILL.md` (with `name:`/`description:`
frontmatter, which both CLIs read natively) plus optional support files.
manigot stores and delivers them exactly like agents, with the same global +
project split:

- **Global skills** live in the manigot checkout at `skills/<name>/SKILL.md`
  (mirrors `agents/`). For a container session they are mounted read-only at
  the CLI's global skills location; for `mg host` they are delivered into the
  host CLI's own skills dir the same way agents are.
- **Project skills** live in `your-project/docs/skills/<name>/SKILL.md`
  (mirrors `docs/agents/`). They ride the existing `docs/` mount into the
  container at the project skills location and override a global skill of the
  same name — the same project-overrides-global precedence agents have.

manigot ships one example skill, `skills/job-brief/`; drop yours into
`skills/` in the checkout (or `docs/skills/` in a project) to make them
available everywhere.

---

## Meta prompt

The meta prompt (`prompts/meta.md` in the manigot checkout) is a system-wide
instruction file that is injected into **every** session — the top of the
instruction hierarchy:

```
meta prompt (system-wide)  →  agents (per role)  →  skills (on demand)  →  project context (docs/AGENTS.md)
```

It carries the general "do this, do that" character and goals that apply
regardless of agent, project, or interactive/`--print` mode — e.g. prefer
small, focused changes and concrete verification over assumption. The agent
files remain the operative per-role instructions (job workflow, commit
discipline, guardrails, `shot` usage all live there); the meta prompt is
deliberately tool-neutral and does not duplicate their rules.

Delivery mirrors the global agents/skills mechanism, at each CLI's **global
instruction** location — the file both CLIs load in every session, independent
of agent, project, or mode:

- **Claude Code**: `~/.claude/CLAUDE.md` (the user-global memory file). It is
  loaded before the project context at `/workspace/.claude/CLAUDE.md`, so the
  project-level context still wins on conflict — the desired precedence.
- **OpenCode**: `~/.config/opencode/AGENTS.md` (the global rules file), loaded
  alongside the project `/workspace/AGENTS.md` context mount.

For a container session the checkout's `prompts/meta.md` is mounted read-only at the
per-tool target (no conversion — plain markdown is native to both CLIs). For
`mg host` it is delivered into the host CLI's own global instruction file as a
**copy, never a symlink** — `~/.claude/CLAUDE.md` is Claude's user-writable
memory file, and a symlink would let `/memory` writes and agent edits land
back in the checkout. Delivery is non-clobbering (an existing host file wins)
and warn-only on failure, exactly like the agent/skill installers. A checkout
without `prompts/meta.md` simply yields no delivery — the file is optional.

---

## Job workflow

Each piece of work gets its own directory under `docs/jobs/`,
named with an English-word ID (e.g. `flower`, never re-used across open or
archived jobs) and a slugified title. A git branch is
created automatically with the same ID, checked out in the job's **own git
worktree** — a sibling directory of the project root, under
`<parent>/.manigot-worktrees/<project-name>/` — so every job gets its own
directory and multiple jobs can be worked on (interactively or via
`mg jdi`) in parallel, while the project root stays on the base branch.

```bash
mg job "add image gallery block"
# creates: docs/jobs/flower_add-image-gallery-block/   (inside the job's worktree)
#   brief.md    ← you fill in: what and why
#   tasks.md    ← @analyst fills in: atomic task breakdown
#   implementation.md  ← @developer fills in: what was implemented
#   verdict.md  ← @reviewer and/or @security fill in: pass/fail per task
# creates branch: feature/flower_add-image-gallery-block
# creates worktree: ../.manigot-worktrees/<project>/flower_add-image-gallery-block
```

**Typical flow for a feature:**

```
1.  mg job "feature name"               → creates dir + branch
2.  Fill in brief.md
3.  @owner                              → SHIP / REVISIT / REJECT
4.  @analyst                            → writes tasks.md
5.  Review tasks.md yourself
6.  @developer TASK-1                   → implements, commits as it goes
7.  @developer TASK-2                   → implements, commits as it goes
8.  @reviewer                           → reads diff, writes verdict.md
9.  @security                           → appends security findings to verdict.md
10. Fix anything blocking, re-run 8–9
11. Merge branch when verdict is APPROVED
12. Update status: open → done in brief.md
```

**For a bug fix, skip steps 3–4 and go straight to the developer.**

Per-task commit hygiene is deliberately relaxed — `mg done` squashes the whole
branch into one commit anyway. Whatever an agent leaves uncommitted is
auto-committed when its session ends (a host-side sweep with a
`[<id>] chore: commit all` commit), so `mg done`'s clean-tree check is never
tripped by leftovers — including read-only agents' outputs like the
analyst's `tasks.md`, which the agent itself cannot commit.

`mg jobs` lists every open job and lets you pick one; the session then starts
in the agent appropriate to the job's stage — `@analyst` while it's being
planned, `@developer` while it's being implemented, `@reviewer` once there's
work to review (an explicit `mg jobs --agent <name>` always wins).

Job types: `feature` (default), `fix`, `chore`.
Branch naming: `feature/ID_slug`, `fix/ID_slug`, `chore/ID_slug` — or
`<prefix>/<type>/<id>_<slug>` when the project sets `jobBranchPrefix` in
`.manigot/manigot.json` (e.g. `jobs/feature/ab12cd_x`), which lets projects
with a pre-existing plain branch named `feature`/`fix`/`chore` keep using the
job workflow.

### How to get a job done

1. **Open the case.** `mg job "job name"` cuts you a directory, a branch, and
   a worktree to keep the job in.
2. **Write the brief.** Fill in `brief.md` — the job, and why it needs doing.
3. **Run it past the boss.** `@owner` calls it: SHIP / REVISIT /
   REJECT.
4. **Case the job.** `@analyst` breaks it into a task list.
5. **Read the plan yourself** before anyone touches code.
6. **Send in the crew.** `mg crew` rounds up an agent — `@developer` does the
   actual work, one task at a time, committing as it goes (anything left
   uncommitted is auto-committed when the session ends), safe inside its own
   safehouse where nothing outside the project is reachable.
7. **Get it checked.** `@reviewer`, then `@security`, go over the work and
   write the verdict.
8. **Clean up loose ends.** Fix what's blocking, run it past them again.
9. **Close it out.** Merge once the verdict's APPROVED, mark the brief done.

A job scaffolded and then abandoned leaves a **dead worktree directory**
behind — a `.manigot-worktrees/` dir whose git registration is gone, with no
branch and no entry in `mg jobs`. `mg jobs` surfaces these orphans and offers
to remove them, and `mg delete <name>` removes one by name — both with the
same "This cannot be undone." confirmation as a normal delete. `mg done` and
`mg delete` (and orphan removal) also clean up the job's `mg jdi` status
sidecar and `run.log` under `.manigot/jdi-status/` — the archive keeps the
job's docs, and `mg jdi` never runs against a finished or deleted job, so the
sidecar would otherwise be dead weight forever.

Don't feel like babysitting the whole thing? Send `mg made-man --job <id>`
instead — it runs the analyst, the developer, and the reviewer back to back,
on its own, and still won't touch the merge button without you.
### Autonomous mode (`mg jdi`)

Steps 4–8 above — `@analyst` → `@developer` → `@reviewer` — can run
unattended instead of one agent at a time:

```bash
mg jdi --job a3f9k2
```

(`mg made-man --job a3f9k2` is a purely thematic alias of the same command —
same script, same behavior.)

It drives that fixed sequence, the same one regardless of job `type`, in a
loop: ask what's next given the job's current stage and verdict history, run
that agent non-interactively, check whether it needs a human, repeat. It
stops — never auto-merging; you still merge the branch yourself via `mg done`
(and `mg jdi` never checks anything out in the project root — every job's own
worktree is resolved per invocation) — when:

- `verdict.md`'s `## Overall` says **APPROVED**, or
- a REJECTED/NEEDS WORK verdict bounces back to `@developer` **once**, and
  the re-review still isn't APPROVED — no further retries, or
- an agent decides it's blocked and can't proceed without you: it prints a
  line starting with exactly `NEEDS-HUMAN-INPUT:` followed by why (a
  convention only `--print`/`mg jdi` invocations ask for — an ordinary
  interactive session never sees this instruction and just asks its
  question in conversation, since you're right there to answer it), or
- the same agent makes no progress on two consecutive runs (a stall
  backstop, independent of the marker above).

`@owner` and `@security` are not part of the sequence `mg jdi`
drives — both stay available as ordinary agents, unaffected.

`mg jdi` runs under the `claude-pro` profile by default; pass `--profile
zai`, `--profile opencode-go`, `--profile opencode-zen`, or `--profile
opencode-zen-free` to drive the sequence under an OpenCode subscription
instead:

```bash
mg jdi --job a3f9k2 --profile zai
```

The TUI's `j` keybinding (see below) passes its settings profile — the shared
`MANIGOT_PROFILE` default — the same way `@name` agent launches already do;
no separate configuration needed there.

**Watching a run.** A direct `mg jdi --job <id>` run streams each agent's
output to its own terminal as each invocation completes, and rings the
terminal bell when it stops. One honesty note: `claude --print` returns an
agent's *final response text* per invocation, not a blow-by-blow of every
tool call/file edit — so this is "see each agent's final answer as it comes
in," not a live diff of its work. A TUI-launched run (see below) has no
terminal of its own at all — the list's status badge and the detail view's
log tab are how you watch it there instead.

**Getting notified.** When `mg jdi` runs unattended on a VPS, you don't want
to watch the terminal for it to stop. Set `NTFY_TOPIC` in `manigot/.env`
(`mg setup` walks you through the three keys) and it pushes an ntfy
notification to your phone whenever a run stops: a success notification when
it finishes, a high-priority attention one when it stops needing a human —
and, at the next run's start, if it finds the previous run crashed or
killed. `NTFY_URL` defaults to `https://ntfy.sh`; `NTFY_TOKEN` is optional
(sent as a `Bearer` auth header). Notifications are opt-in: without
`NTFY_TOPIC`, nothing is sent and behavior is unchanged.

---

## TUI

manigot ships an optional terminal UI for browsing jobs and launching agents
without remembering command syntax. It is **host-side**: it runs on your
machine as `mg tui` (in-process — no separate binary), reads a project's
`docs/jobs/`, and launches sessions through the same in-process session
launcher the CLI uses. It does **not** run in the container and needs no
credentials itself.

Every open job lives in its own **git worktree** (see
[Job workflow](#job-workflow)) — one worktree per job branch, created by
`mg job` and removed by `mg done`/`mg delete` — so the job list is
discovered straight from `git worktree list`: each worktree's
`docs/jobs/` is read off its own disk, and the project root's own working
tree stays on the base branch. There is no "wrong branch checked out" state
to worry about: every action — editing a brief, launching an agent, marking
done, deleting — targets the job's own worktree directly, so nothing is ever
guarded on the currently checked-out branch.

### Supported platforms

macOS and Linux. Firing an agent opens `mg --profile <profile> --agent <name>
--job <id>` in a new terminal (`<profile>` from the settings screen — see
below), picked in this order:

1. a **tmux** split pane in the TUI's own window, if the TUI is itself running
   inside tmux (`$TMUX` set) — each new launch replaces the pane manigot opened
   before, so at most one agent pane exists at a time. Inside tmux this always
   wins, even when a **Terminal** is set (see below): the session opens in a
   tmux pane, never in a separate window.
2. the **Terminal** from the settings screen (`s` from the job list), when
   set — applied only when the TUI is NOT inside tmux
3. **Terminal.app** via `osascript` on macOS
4. a Linux terminal emulator — `gnome-terminal`, `x-terminal-emulator`,
   `konsole`, or `xterm`, whichever is found first on `PATH`

Setting a **Terminal** in the settings screen (`s` from the job list)
replaces the macOS/Linux auto-detect chain above (steps 3–4). It only applies
when the TUI is not running inside tmux: inside tmux the split-pane behavior
wins, so a session launched from a TUI inside tmux always opens in a tmux
pane. Leave it blank to keep auto-detect behavior.

The list view's `o` shortcut (see Keybindings) opens a bare `mg --profile
<profile>` instead — same spawn paths, but with no agent and no job, for a quick ad-hoc
session that isn't tied to a specific job's workflow. `a` is the middle
ground: it opens a picker of every agent available to the project (global
`agents/*.md`, with any `docs/agents/` overrides/additions swapped in — the
same set `mg agents` lists), then launches the one you pick the same way `o`
does, `mg --profile <profile> --agent <name>`, still with no `--job`.

Windows is not supported in this version.

### Build & install

```bash
cd manigot/
make mg         # builds bin/mg — the one binary, covering mg tui, mg jdi, ...
make install    # puts the single mg binary on your PATH
```

`make mg` builds the single static `bin/mg` binary (requires Go 1.23+ and
network access the first time, to fetch the Charm modules). `make install`
symlinks it onto your `PATH`; `mg tui` and `mg jdi` are subcommands of that
same binary, so no other artifacts or wrapper scripts exist.

### Run

```bash
cd your-project/
mg tui                # from anywhere inside a project that has a docs/
```

### Keybindings

List view:

| key | action |
|---|---|
| `↑`/`↓` | move selection |
| `enter` | open the job's detail view |
| `j` | run `mg jdi` against the selected job, detached in the background — watch via the list's status badge (see [Autonomous mode](#autonomous-mode-mg-jdi) and [mg jdi status & log](#mg-jdi-status--log) below) |
| `o` | launch a quick manigot session (no agent, no job) |
| `a` | pick any agent and launch it as a quick session (no job) |
| `n` | create a new job (runs the in-process job lifecycle) |
| `s` | open settings (editor, subscription profile) |
| `ctrl+r` | refresh — re-read job files from disk |
| `q` | quit |

Detail view:

| key | action |
|---|---|
| `tab` / `1`-`6` | switch tab: brief · tasks · implementation · verdict · log · diff |
| `pgup`/`pgdn`, `home`/`G` | scroll |
| `p` `a` `d` `r` `s` | run the agent shown in the action bar (Owner, Analyst, Developer, Reviewer, Security — all five are always available, regardless of the job's stage) |
| `e` | edit `brief.md` in `$EDITOR` (only on the brief tab — tasks/implementation/verdict/log/diff are agent-, mg jdi-, or TUI-computed) |
| `D` | mark the job done — shows an in-TUI confirmation, then runs the in-process finish lifecycle (squash-merge + archive) |
| `j` | run `mg jdi` against this job, detached in the background — no window is opened; watch it via the log tab or the list's status badge (see [Autonomous mode](#autonomous-mode-mg-jdi) and [mg jdi status & log](#mg-jdi-status--log) below) |
| `x` / `del` | permanently delete the job — shows an in-TUI confirmation (with a dirty-worktree warning when the job's worktree has uncommitted changes), then runs the in-process delete lifecycle. `x` exists because the physical Delete/Entf key's escape sequence isn't decoded consistently by every terminal — both trigger the same action |
| `P` | push this job's branch to `origin` (`git push -u origin <branch>`) — a quick way to make it visible on another host via `git pull` |
| `t` | open the job's branch diff in tig (`mg diff <id> --tig`) — spawns in a tmux split pane / new terminal like agent launches; only available when tig is installed on the host |
| `c` | commit all uncommitted changes in this job's worktree (`git add -A` + `git commit` with a `[<id>] chore: commit all` message) — a catch-all sweep for the files agents sometimes leave behind, so `D`'s clean-tree check isn't tripped |
| `g` | open the git panel — pick one of the detail view's git commands for this job's worktree: commit all, push to origin, or merge default branch (see below) |
| `ctrl+r` | refresh |
| `esc` | back to list |

The **log** tab shows `mg jdi`'s `run.log` for this job — one section per
agent invocation, with a timestamp/agent/attempt header; the attempt number
counts that agent's invocations within the run (per agent, not per run), so
a bounce back to the developer shows that developer's second call as attempt
2. It reads
"_no mg jdi run has happened for this job yet_" until the first `mg jdi` run
against it; large files are tailed, not loaded in full. Never editable.

The **diff** tab is the TUI's representation of `mg diff <id>`: it computes
what the job's branch changed relative to the project's base branch — the
same quick eyeball the CLI prints by default (`git log --oneline` followed by
`git diff --stat` over the three-dot range `<base>...<branch>`, per
[docs/GIT_DIFF.md](docs/GIT_DIFF.md)), so you can review the job's changes
in the TUI before marking it done with `D`. The base branch resolves exactly
as `mg diff` does — the project's configured `baseBranch`, falling back to
`origin/HEAD` → `main` — and the tab degrades instead of crashing: a job
with no branch (not a git worktree job) or a git error shows a plain-text
placeholder, and an undiverged branch reads "No changes on `<branch>`
relative to `<base>`." It is recomputed on every `ctrl+r`, so newly
committed changes appear on refresh. Never editable.

Below the footer, the job view also shows a **git-log strip** — the same dim
`shortHash  subject  relTime  branch` lines the job list's recent-activity
strip renders at the bottom of the screen, scoped to just this job's own
branch (`git.BranchCommits`, the job-branch counterpart of the list's
cross-branch `git.RecentCommits`). It is refreshed when the job opens and on
`ctrl+r`, sized like the list's strip (up to the settings'
recent-activity count, floor of one line), and disappears entirely for a
job with no branch or a project that isn't a git repo.

The **git panel** (`g`) is a small modal listing the detail view's git
commands — **Commit all**, **Push to origin**, and **Merge default branch**
— one per row, moved through with `↑`/`↓` and run with `enter`
(`esc`/`q` cancels back to the detail view). The first two are the same
actions as the `c`/`P` accelerators; **Merge default branch** brings a job's
worktree up to speed before starting work: it runs `git merge --no-edit`
inside the job's own worktree, merging in the project's locally-resolved
base branch (the configured `baseBranch`, falling back to `origin/HEAD` →
`main` — the same local base the diff tab diffs against and `D` merges into).
It never fetches, so it merges whatever state that base branch is in
locally; a worktree that is already up to date is a successful no-op, and a
merge conflict leaves the tree in git's conflicted state for you to resolve
manually (the detail view's status line reports the conflict).

`e` resolves the editor to run as: the settings screen's Editor field (see
below), if set; otherwise `$VISUAL`, then `$EDITOR`, then whichever of
`nano`/`vi` is found first on `PATH`.

New-job form: type a title, `tab` to the type field and use `←`/`→` to pick
`feature`/`fix`/`chore`, `enter` to create, `esc` to cancel.

### Settings

Press `s` from the job list to open the settings screen:

- **Editor** — the command `e` (in the detail view) runs to open `brief.md`.
  Leave blank to fall back to `$VISUAL`/`$EDITOR`/`nano`/`vi`.
- **Profile** — `claude-pro`, `zai`, `opencode-go`, `opencode-zen`, or
  `opencode-zen-free`, cycled with `←`/`→`
  (the selected profile's tool, model, and billing are shown beneath the list).
  Selects which subscription firing an agent from the action bar launches
  (adds `--profile` to the `mg --agent ... --job ...` command the same way
  `mg --profile zai` would on the command line). It is stored as
  `MANIGOT_PROFILE` in `manigot/.env` — the one default shared between CLI and
  TUI — so switching it here also switches what bare `mg` uses, and a profile
  set with `mg profiles` shows up here.
- **Theme** — the global OpenCode theme (e.g. `nord`, `tokyonight`), free-text
  (not a fixed-list cycling selector, since OpenCode may ship themes manigot
  doesn't know about — see `mg theme` above). Leave blank to let OpenCode use
  its own default/config. Shared across every OpenCode profile, unlike the
  per-profile model. Stored as `OPENCODE_THEME` in `manigot/.env`, same as
  Profile, so a theme set with `mg theme` shows up here too. Claude Code
  already respects your terminal's own theme, so this has no effect there.

`tab` moves between fields, `enter` saves, `esc` discards. The editor persists
to `config/tui-settings.json` and the profile + theme to `manigot/.env` as
`MANIGOT_PROFILE`/`OPENCODE_THEME` (both gitignored, both in the manigot
checkout) and apply immediately; a missing file just means nothing has been
saved yet, and every setting falls back to its default above.

### Stage timeline

The detail view's action bar shows a horizontal timeline of every stage —
done stages checked, the current one highlighted, stages still ahead dim —
as an informational hint of where the job's files say it is in the ideal
workflow above. It no longer restricts which agents can be launched from
there. Any of the five agents can be fired at any time, so a job worked on
outside the ideal flow (e.g. a hand-written `brief.md` and `tasks.md`,
straight to `@developer`) isn't blocked by the TUI.

| stage | when |
|---|---|
| define | `brief.md` not yet written |
| plan | `brief.md` written, `tasks.md` not yet |
| implement | `tasks.md` written, `implementation.md` not yet |
| review | `implementation.md` written, `verdict.md` not yet |
| finished | `verdict.md` written and its `## Overall` verdict is APPROVED |

A file counts as "written" once it has real content beyond its `mg job`
scaffold (template comments, empty headings, and frontmatter don't count). A
verdict that's written but not approved (REJECTED, NEEDS WORK, or anything
else) bounces the stage back to implement rather than resolving to review or
finished — the job needs more work before it goes through review again.

Press `D` from the detail view at any point to mark the job done: a
confirmation screen (showing the job's verdict warning when a verdict is
missing or not approved — the same warnings the CLI's `mg done` shows) leads
to the in-process finish lifecycle, which squash-merges the job branch into
the default branch, archives the job directory under `docs/jobs/archive/`,
and sets `status: done`. This is available from any stage too.

### mg jdi status & log

Press `j` in the list (on the selected job) or in the detail view to start
[`mg jdi`](#autonomous-mode-mg-jdi) against that job. Unlike the agent-launch
keys above, this opens **no
terminal window at all** — `mg jdi` has no interactive session for a human
or a subprocess to attach to, so it runs fully detached in the background.
Two places show you what it's doing instead, both polled the same
refresh-triggered way as everything else in the TUI (pressing `ctrl+r`,
returning to the list, etc. — no separate live-streaming subsystem). The one
exception is a narrow timer-driven redraw that runs *only while a run is
active*, driving the animated indicator below:

- **List-row badge** — a `[running @<agent>]`, `[finished]`, or
  `[needs human]` tag next to a job's row.
  While a run is active the running badge is prefixed
  with an animated activity indicator — a small spinner
  (`⠋ [running @<agent>]`), the same idea as the one opencode shows in its
  bottom-left corner — so a watched run visibly indicates it's alive instead
  of sitting as a static tag that only changes on the next refresh. Shown
  only while there's something live or recent
  to report; a stale status left behind by a killed `mg jdi` process is
  never shown as if it were current.
- **Log tab** — see [Keybindings](#keybindings) above.

**Notification.** A direct `mg jdi --job <id>` run rings the terminal bell
itself when it stops (see [Autonomous mode](#autonomous-mode-mg-jdi)). A
`j`-launched run has no terminal to ring into, so the TUI rings it instead,
on its own next poll, the first time it notices that job's status turn into
`finished` or `needs human` — once per fresh stop, not on every poll while
it stays stopped, and never for a job that was already stopped before this
TUI session started watching it.

---

## Rebuilding

When Claude Code or OpenCode releases an update worth taking:

```bash
cd manigot/
make rebuild
```

The image also ships `make` and the Go toolchain (Debian trixie's `golang-go`,
currently Go 1.24) so the host-side tool in `src/cmd/` can be built and tested
from inside a container. `GOTOOLCHAIN=local` is set, so if `src/go.mod` ever
requires a newer Go than the image has, the build fails loudly instead of
silently downloading one.

The Go module cache is pre-warmed at build time from `src/go.mod` and
`src/go.sum`, which means `make mg` and `go test ./...` (from `src/`) work
inside the container without network access — but also that **bumping a
dependency requires a `make rebuild`**, otherwise the new module is missing
from the cache.