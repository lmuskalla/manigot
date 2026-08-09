# Tasks: opencode-compatibility

id: djss4d
status: open
analyst: @analyst
date: 2026-08-08

<!-- Produced by @analyst from brief.md. -->

## Open questions before implementation starts

The brief's checklist assumes specific facts about OpenCode (binary name/install
method, config directory layout, agent file format, auth env var names) that
are not verifiable from this repo alone. Per project hard rules ("when scope
is unclear: ask, don't guess"), these should be confirmed with the user or
against OpenCode's own docs before TASK-2, TASK-3, TASK-4, and TASK-6 are
implemented:

1. How is OpenCode installed (npm package name, curl script, apt, single
   binary release)? Dockerfile currently installs Claude Code via
   `npm install -g @anthropic-ai/claude-code`.
2. What is OpenCode's actual global agent directory and file format? The
   brief says `~/.config/opencode/agents/` — needs confirming (singular
   `agent/` vs plural, and whether it uses the same YAML-frontmatter +
   Markdown format as Claude Code's `agents/*.md`, since these are baked
   from the same source files).
3. What is OpenCode's non-interactive/onboarding-skip mechanism, if any,
   and what env var(s) it expects for provider API keys (e.g.
   `OPENCODE_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` — OpenCode is
   multi-provider, so "the" provider key is ambiguous)?
4. What is OpenCode's project-level config/mount directory
   (`/workspace/.opencode` per the brief — confirm exact expected path and
   whether it also expects a project instructions file analogous to
   `CLAUDE.md`)?
5. What is OpenCode's CLI invocation for passing an initial prompt / agent
   selection, to know how `entrypoint.sh` should `exec` it (mirrors
   `exec claude "$@"`)?

Tasks below are scoped assuming these answers land close to what the brief
states, but each task that touches OpenCode specifics should re-verify
against real OpenCode documentation during implementation rather than
guessing further.

## Task breakdown

TASK-1: Add a `--tool` flag to `scripts/run.sh` that accepts `claude-code`
(default) or `opencode`, validated and stored in a `TOOL` variable, rejecting
any other value with an error.
files: scripts/run.sh
depends: none
risk: low — isolated arg-parsing addition, same pattern as existing `--agent`/`--job` handling.

TASK-2: Install the OpenCode binary in the `Dockerfile` alongside Claude
Code, so both CLIs are present in the built image.
files: Dockerfile
depends: none
risk: medium — exact install method/package for OpenCode is unconfirmed (see Open questions #1); wrong install step breaks the whole image build.

TASK-3: Bake the global `agents/` markdown files into both
`~/.claude/agents/` and `~/.config/opencode/agents/` in the `Dockerfile`
(currently only copied to `~/.claude/agents/`).
files: Dockerfile
depends: TASK-2 (should land after OpenCode's expected config layout is confirmed, since the target path may differ from the brief's assumption)
risk: medium — OpenCode's actual agent directory name/format is unconfirmed (see Open questions #2); may require reformatting agent files rather than a straight copy.

TASK-4: Branch `scripts/entrypoint.sh` on the selected tool: keep the
existing Claude Code onboarding-bypass (`~/.claude.json` write) only for
`claude-code`, and for `opencode` skip that step and just ensure the
provider API key env var(s) are present before `exec`-ing the right CLI.
files: scripts/entrypoint.sh
depends: TASK-1 (needs `TOOL` to be passed into the container as an env var), TASK-2 (opencode binary must exist to exec it)
risk: medium — correctness depends on OpenCode's real non-interactive startup behavior and CLI invocation (see Open questions #3, #5); also must not regress the existing Claude Code path.

TASK-5: Make the `docs/` → container mount target in `scripts/run.sh`
conditional on `--tool`: `/workspace/.claude` for `claude-code` (current
behavior), `/workspace/.opencode` for `opencode`.
files: scripts/run.sh
depends: TASK-1
risk: low — mirrors an existing single mount line with a conditional path; main risk is picking the wrong target path if Open question #4 resolves differently than assumed.

TASK-6: Add OpenCode provider API key support: new variable name(s) read
from `.env` in `scripts/run.sh` and passed through to `docker run -e`,
alongside the existing `CLAUDE_CODE_OAUTH_TOKEN` handling (including the
existing guard that errors if `ANTHROPIC_API_KEY` is set for the Claude Code
path — that guard must not block the OpenCode path if it also uses
`ANTHROPIC_API_KEY`).
files: scripts/run.sh
depends: TASK-1
risk: medium — "provider API key" is ambiguous for a multi-provider tool (see Open questions #3); also interacts with the existing `ANTHROPIC_API_KEY` safety check, which needs care to avoid weakening the subscription-billing protection for Claude Code users.

TASK-7: Update `README.md` with OpenCode setup and usage: install
prerequisites, `.env` variables, `--tool opencode` usage example, and the
differing mount/agents-directory behavior.
files: README.md
depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5, TASK-6 (documents final, implemented behavior — should be written last)
risk: low — documentation only, no runtime effect.

TASK-8: Update `docs/CLAUDE.md` (this repo's own project instructions) to
describe the tool as vendor-agnostic: mention the `--tool` flag, dual agent
bake locations, and per-tool mount targets, replacing Claude Code-only
language in the Stack/Architecture sections.
files: docs/CLAUDE.md
depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5, TASK-6
risk: low — documentation only; project hard rules require this file to stay accurate to the real system.

TASK-9: Update `project-template/docs/CLAUDE.md` to match the same
vendor-agnostic description, since the hard rules require it to stay in
sync with the root project instructions.
files: project-template/docs/CLAUDE.md
depends: TASK-8
risk: low — documentation only, template copied into new projects.

## Added during implementation

Requested by the user after TASK-1..9 landed, once it became clear that
OpenCode never reads `docs/CLAUDE.md` (it looks for `AGENTS.md`/`CLAUDE.md` by
traversing up from the working directory, not inside `.opencode/`). Without
this, project context is invisible to OpenCode, which makes the tool useless
there.

TASK-10: Make the project context file tool-neutral: `docs/AGENTS.md` becomes
the canonical name, mounted where each tool actually reads it —
`/workspace/AGENTS.md` for OpenCode, `/workspace/.claude/CLAUDE.md` for Claude
Code. Fall back to `docs/CLAUDE.md` when `docs/AGENTS.md` is absent so existing
projects keep working, and warn when neither exists.
files: scripts/run.sh
depends: TASK-5
risk: medium — nested bind mount inside the `docs/` mount; must not break the
existing Claude Code context loading.

TASK-11: Rename the context file to `AGENTS.md` in this repo and in the
project template, updating every reference to it.
files: docs/CLAUDE.md → docs/AGENTS.md, project-template/docs/CLAUDE.md →
project-template/docs/AGENTS.md, README.md, agents/product-owner.md
depends: TASK-10 (rename is only safe once both tools can find AGENTS.md)
risk: low — rename plus reference updates.

TASK-12: Fix the remaining Claude Code-specific claims in `README.md`: the
`docs/` tree annotation, the agents table's tool-restriction column (not
enforced under OpenCode), and "subscription billing" in the intro.
files: README.md
depends: TASK-10, TASK-11
risk: low — documentation only.

## Explicitly not covered by this breakdown

- `scripts/new-job.sh` — brief does not mention job-creation changes; no
  tool-specific behavior identified there.
- `Makefile` — no new build targets requested in the brief; leave as-is
  unless OpenCode's install step (TASK-2) turns out to need a separate
  build stage or arg.
