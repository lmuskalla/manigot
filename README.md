# safecode

Isolated agent environment per project. One Docker image, real filesystem
containment, structured agent workflow.

Runs either **Claude Code** (default, billed against your subscription via
mounted OAuth credentials) or **OpenCode** (billed per token against whichever
provider key you give it) — pick per session with `--tool`.

---

## Where everything lives

### The safecode repo

```
safecode/
  Dockerfile              ← build once, rebuild on Claude Code / OpenCode updates
  Makefile                ← build / rebuild / install / make tui
  scripts/                ← launcher and utility scripts
    run.sh                ← container launcher      → 'sc'
    new-job.sh            ← job generator           → 'sc-job'
    finish-job.sh         ← job archiver            → 'sc-done'
    tui.sh                ← TUI launcher            → 'sc-tui'
    entrypoint.sh         ← runs inside the container before the agent CLI starts
  tui/                    ← host-side TUI source (Go); `make tui` builds bin/safecode-tui
  bin/                    ← built binaries (gitignored)
  .env                    ← your credentials (gitignored, never committed)
  .gitignore
  README.md
  agents/                 ← global agents, baked into the image for both tools
    analyst.md
    developer.md
    reviewer.md
    security.md
    product-owner.md
    designer.md
  project-template/       ← copy this into each new project to get started
    docs/
      AGENTS.md
      templates/
        processes/
          6-char-random-id_title-of-job/  ← placeholder job showing all four files
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
    processes/                ← one directory per job
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

---

## Setup (once)

### Claude Code (default)

```bash
# 1. Extract your account info from your host (requires Claude Code installed locally)
cat ~/.claude.json | python3 -c "
import json,sys
d=json.load(sys.stdin)
print(json.dumps(d.get('oauthAccount'), indent=2))"

# 2. Add credentials to .env
cat > safecode/.env << EOF
CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-...
CLAUDE_ACCOUNT_UUID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
CLAUDE_EMAIL=your@email.com
CLAUDE_ORG_UUID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
EOF

# 3. Build the image
cd safecode/
make build

# 4. Put the launchers on your PATH
make install                            # /usr/local/bin — may need sudo
# make install PREFIX="$HOME/.local"    # ...or somewhere you own
```

### OpenCode

OpenCode is already in the image — no extra install. It is multi-provider and
authenticates from environment variables, so add at least one provider key to
the same `.env`:

```bash
cat >> safecode/.env << EOF
# any one of these is enough
ANTHROPIC_API_KEY=sk-ant-...
# OPENAI_API_KEY=sk-...
# OPENROUTER_API_KEY=sk-or-...
# GOOGLE_GENERATIVE_AI_API_KEY=...
# GROQ_API_KEY=...
# XAI_API_KEY=...
# DEEPSEEK_API_KEY=...
# OPENCODE_API_KEY=...            # OpenCode Zen
# ZHIPU_API_KEY=...               # Z.AI / Z.AI Coding Plan

# optional: model to start with, as provider/model
OPENCODE_MODEL=anthropic/claude-sonnet-4-5
EOF
```

Recognised keys: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`,
`GOOGLE_GENERATIVE_AI_API_KEY`, `GROQ_API_KEY`, `XAI_API_KEY`,
`DEEPSEEK_API_KEY`, `OPENCODE_API_KEY`, `ZHIPU_API_KEY`. Only the ones you set
are passed into the container, and only when you run with `--tool opencode`.

Note: `ANTHROPIC_API_KEY` is rejected on the Claude Code path — it would
override your subscription and bill per token. On the OpenCode path it is
allowed, because there it is the normal way to authenticate.

### The installed commands

`make install` puts four commands on your `PATH`:

| command | does |
|---|---|
| `sc` | start a session in the current project |
| `sc-job` | create a job directory + branch (off `main`) |
| `sc-done` | archive a finished job |
| `sc-tui` | the terminal UI (needs `make tui` first) |

They are symlinks back into the repo, so `git pull` updates them. `make
uninstall` removes them again.

### Installing without symlinks

If you would rather not write to `/usr/local/bin`, define shell aliases instead:

```bash
# ~/.zshrc or ~/.bashrc
alias sc='~/code/safecode/scripts/run.sh'
alias sc-job='~/code/safecode/scripts/new-job.sh'
alias sc-done='~/code/safecode/scripts/finish-job.sh'
alias sc-tui='~/code/safecode/scripts/tui.sh'
```

One catch: aliases only exist inside your interactive shell, so the TUI cannot
see them — it has to find the real scripts itself. Point it at them with env
vars, which override discovery completely:

```bash
export SAFECODE_HOME="$HOME/code/safecode"   # covers all of the below at once
export SAFECODE_BIN=…                        # or one at a time: the launcher
export SAFECODE_JOB_BIN=…                    #   job creation
export SAFECODE_DONE_BIN=…                   #   job completion
```

Each `*_BIN` takes either a path or a bare command name to look up on `PATH`. A
value that cannot be resolved is reported as an error rather than silently
ignored, so a typo is visible. `SAFECODE_HOME` is set automatically when you
start the TUI through `scripts/tui.sh`, so in that case there is
nothing to configure.

---

## Per-project setup

```bash
# Copy the template into your project
cp -r safecode/project-template/docs/ your-project/docs/

# Fill in your project context
$EDITOR your-project/docs/AGENTS.md
```

That's it. The global agents are already in the image — nothing else to copy.

`docs/AGENTS.md` is the one project context file, and it works for both tools:
safecode mounts it read-only at whatever path the selected CLI reads context
from (`/workspace/AGENTS.md` for OpenCode, `/workspace/.claude/CLAUDE.md` for
Claude Code). Projects that still have a `docs/CLAUDE.md` keep working — it is
used as a fallback when `docs/AGENTS.md` is absent.

---

## Usage

```bash
# Start a safecode session (run from anywhere inside your project)
cd your-project/
sc                        # Claude Code (default)
sc --tool opencode        # OpenCode

# Start straight in an agent, or on a job
sc --agent analyst
sc --tool opencode --job a3f9k2

# Create a new job
sc-job "add image gallery block"
sc-job "fix tenant isolation on media uploads" --type fix
sc-job "upgrade dependencies" --type chore

# Archive a finished job
sc-done a3f9k2
```

---

## Choosing a tool

`--tool claude-code` (default) or `--tool opencode`. What differs:

| | `claude-code` | `opencode` |
|---|---|---|
| CLI in container | `claude` | `opencode` |
| Auth | `CLAUDE_CODE_OAUTH_TOKEN` + account UUIDs (subscription) | one provider API key |
| Onboarding | bypassed by writing `~/.claude.json` | nothing to bypass |
| Global agents | `~/.claude/agents/` | `~/.config/opencode/agents/` |
| `docs/` mounted at | `/workspace/.claude` | `/workspace/.opencode` |
| Project agents | `/workspace/.claude/agents/` | `/workspace/.opencode/agents/` |
| `docs/AGENTS.md` mounted at | `/workspace/.claude/CLAUDE.md` | `/workspace/AGENTS.md` |
| Initial job prompt | positional argument | `--prompt` |
| Billing | your Claude subscription | per token, on your provider key |

Both tools get the same `agents/*.md` files baked in at build time. The
OpenCode copies are generated from the same sources with the `name` and `tools`
frontmatter keys removed, since OpenCode derives the agent name from the
filename and uses a different schema for tool permissions.

One caveat: because `tools:` is dropped, the read-only agents are **not**
restricted under OpenCode — `@reviewer`, `@security`, `@analyst` and
`@product-owner` can write files there. Under Claude Code the restriction is
enforced. Expressing it as OpenCode `permission:` frontmatter is a follow-up.

---

## Agents

Six agents are available globally in every project. Call them with `@name` in
your session.

| Agent | Role | Tools (Claude Code) |
|---|---|---|
| `@analyst` | Breaks a brief into atomic tasks | read-only |
| `@developer` | Implements one task at a time | read + write |
| `@reviewer` | Verifies implementation against tasks | read-only |
| `@security` | Audits for vulnerabilities | read-only |
| `@product-owner` | Evaluates features from the user's perspective | read-only |
| `@designer` | Reviews and directs UI/UX — typography, colour, layout | read + write |

The Tools column is enforced under Claude Code only — see the caveat under
[Choosing a tool](#choosing-a-tool).

To override an agent for a specific project, create a file with the same name
in `your-project/docs/agents/`. Project agents take precedence over global ones.

---

## Job workflow

Each piece of work gets its own directory under `docs/jobs/`,
named with a 6-character random ID and a slugified title. A git branch is
created automatically with the same ID.

```bash
sc-job "add image gallery block"
# creates: docs/jobs/a3f9k2_add-image-gallery-block/
#   brief.md    ← you fill in: what and why
#   tasks.md    ← @analyst fills in: atomic task breakdown
#   implementation.md  ← @developer fills in: what was implemented
#   verdict.md  ← @reviewer and/or @security fill in: pass/fail per task
# creates branch: feature/a3f9k2_add-image-gallery-block
```

**Typical flow for a feature:**

```
1.  sc-job "feature name"               → creates dir + branch
2.  Fill in brief.md
3.  @product-owner                      → SHIP / REVISIT / REJECT
4.  @analyst                            → writes tasks.md
5.  Review tasks.md yourself
6.  @developer TASK-1                   → implements, commits [ID] TASK-1: ...
7.  @developer TASK-2                   → implements, commits [ID] TASK-2: ...
8.  @reviewer                           → reads diff, writes verdict.md
9.  @security                           → appends security findings to verdict.md
10. Fix anything blocking, re-run 8–9
11. Merge branch when verdict is APPROVED
12. Update status: open → done in brief.md
```

**For a bug fix, skip steps 3–4 and go straight to the developer.**

Job types: `feature` (default), `fix`, `chore`.
Branch naming: `feature/ID_slug`, `fix/ID_slug`, `chore/ID_slug`.

---

## TUI

safecode ships an optional terminal UI for browsing jobs and launching agents
without remembering command syntax. It is **host-side**: it runs on your
machine, reads a project's `docs/jobs/`, and shells out to the `sc`
and `sc-job` commands. It does **not** run in the container and needs no
credentials itself. It finds those commands dynamically — see
[Installing without symlinks](#installing-without-symlinks) if they are not on
your `PATH`.

The job list is discovered across **every local branch**, not just the one
currently checked out — since each job's docs live on its own branch, this is
the only way to see everything in flight at once. A row for a job that isn't
on the current branch is dimmed with a trailing `· <branch>` tag; open it and
press `c` in the detail view to check out that branch before editing its
brief, launching an agent, or marking it done — those three actions refuse
(and point you at `c`) while a job's branch and the checked-out branch differ.

### Supported platforms

macOS and Linux. Firing an agent opens `sc --tool <tool> --agent <name> --job
<id>` in a new terminal (`<tool>` from the settings screen — see below),
picked in this order:

1. a new **tmux** window, if the TUI is itself running inside tmux (`$TMUX` set)
2. **Terminal.app** via `osascript` on macOS
3. a Linux terminal emulator — `gnome-terminal`, `x-terminal-emulator`,
   `konsole`, or `xterm`, whichever is found first on `PATH`

The list view's `o` shortcut (see Keybindings) opens a bare `sc --tool <tool>`
instead — same spawn paths, but with no agent and no job, for a quick ad-hoc
session that isn't tied to a specific job's workflow.

Windows is not supported in this version.

### Build & install

```bash
cd safecode/
make tui        # builds bin/safecode-tui
make install    # puts sc-tui (and the other launchers) on your PATH
```

`make tui` builds a single static binary at `bin/safecode-tui` (requires
Go 1.23+ and network access the first time, to fetch the Charm modules).
`make install` symlinks `scripts/tui.sh` onto your `PATH` alongside
`sc` and `sc-job`; that wrapper also tells the binary where this
checkout is, so the TUI can find the scripts even when nothing else is
installed.

### Run

```bash
cd your-project/
sc-tui                # from anywhere inside a project that has a docs/
```

### Keybindings

List view:

| key | action |
|---|---|
| `↑`/`↓` or `k`/`j` | move selection |
| `enter` | open the job's detail view |
| `o` | launch a quick safecode session (no agent, no job) |
| `n` | create a new job (runs the host `sc-job`) |
| `s` | open settings (editor, agent tool) |
| `ctrl+r` | refresh — re-read job files from disk |
| `q` | quit |

Detail view:

| key | action |
|---|---|
| `tab` / `1`-`4` | switch file: brief · tasks · implementation · verdict |
| `j`/`k`, `pgup`/`pgdn`, `g`/`G` | scroll |
| `p` `a` `d` `r` `s` | run the agent shown in the action bar (Product Owner, Analyst, Developer, Reviewer, Security — all five are always available, regardless of the job's stage) |
| `e` | edit `brief.md` in `$EDITOR` (only on the brief tab — tasks/implementation/verdict are agent-written) |
| `D` | mark the job done (runs the host `sc-done`, in the foreground so its confirmation prompts work) |
| `c` | switch to this job's branch (`git checkout`) — needed before `e`/`D`/agent keys work on a job that isn't on the current branch |
| `ctrl+r` | refresh |
| `esc` | back to list |

`e` resolves the editor to run as: the settings screen's Editor field (see
below), if set; otherwise `$VISUAL`, then `$EDITOR`, then whichever of
`nano`/`vi` is found first on `PATH`.

New-job form: type a title, `tab` to the type field and use `←`/`→` to pick
`feature`/`fix`/`chore`, `enter` to create, `esc` to cancel.

### Settings

Press `s` from the job list to open the settings screen:

- **Editor** — the command `e` (in the detail view) runs to open `brief.md`.
  Leave blank to fall back to `$VISUAL`/`$EDITOR`/`nano`/`vi`.
- **Tool** — `claude-code` or `opencode`, cycled with `←`/`→`. Selects which
  agent CLI firing an agent from the action bar launches (adds `--tool` to
  the `sc --agent ... --job ...` command the same way `sc --tool opencode`
  would on the command line).

`tab` moves between fields, `enter` saves, `esc` discards. Settings persist to
`config/tui-settings.json` in the safecode checkout (gitignored — it's a
local preference, not shared) and apply immediately; a missing file just
means nothing has been saved yet, and every setting falls back to its default
above.

### Stage label

The detail view's action bar shows `stage: <name>` as an informational hint of
where the job's files say it is in the ideal workflow above — it no longer
restricts which agents can be launched from there. Any of the five agents can
be fired at any time, so a job worked on outside the ideal flow (e.g. a
hand-written `brief.md` and `tasks.md`, straight to `@developer`) isn't
blocked by the TUI.

| stage | when |
|---|---|
| analyze | job open, tasks not yet written |
| develop | `tasks.md` written |
| review | `implementation.md` written |

A file counts as "written" once it has real content beyond its `sc-job`
scaffold (template comments, empty headings, and frontmatter don't count).

Press `D` from the detail view at any point to run the host `sc-done`
(`scripts/finish-job.sh`) and mark the job done — it squash-merges the job
branch into the default branch, archives the job directory under
`docs/jobs/archive/`, and sets `status: done`. This runs in the foreground
(suspending the TUI, like `e`) because `sc-done` asks for interactive
confirmation along the way; per its own behavior, it warns rather than blocks
on a missing or unapproved verdict, so this is available from any stage too.

---

## Rebuilding

When Claude Code or OpenCode releases an update worth taking:

```bash
cd safecode/
make rebuild
```

The image also ships `make` and the Go toolchain (Debian trixie's `golang-go`,
currently Go 1.24) so the host-side TUI in `tui/` can be built and tested from
inside a container. `GOTOOLCHAIN=local` is set, so if `tui/go.mod` ever requires
a newer Go than the image has, the build fails loudly instead of silently
downloading one.

The Go module cache is pre-warmed at build time from `tui/go.mod` and
`tui/go.sum`, which means `make tui` and `go test ./...` work inside the
container without network access — but also that **bumping a TUI dependency
requires a `make rebuild`**, otherwise the new module is missing from the cache.