# Result: opencode-compatibility

id: djss4d
status: done
developer: @developer
date: 2026-08-08

<!-- Produced by @developer after implementation. -->

## Summary

manigot is now vendor-agnostic: the same image ships both Claude Code and
OpenCode, and `manigot --tool claude-code|opencode` picks which one runs.
Claude Code stays the default and its path is unchanged — subscription auth via
`CLAUDE_CODE_OAUTH_TOKEN`, onboarding bypass, `docs/` at `/workspace/.claude`.
OpenCode gets its own auth (one of several provider API keys), its own global
agent directory, its own `docs/` mount target, and its own prompt/CLI invocation.

All OpenCode specifics were verified against the official docs
(opencode.ai/docs: config, agents, rules, providers, cli) and the npm registry
rather than assumed, which answered the five open questions in `tasks.md`:

1. Install: `npm install -g opencode-ai` (bin `opencode`, prebuilt binary pulled
   in via optional platform deps).
2. Global agents: `~/.config/opencode/agents/` (plural confirmed, singular
   supported for backwards compatibility); markdown + YAML frontmatter, agent
   name comes from the filename, `description` required, `tools` is a map and
   deprecated in favour of `permission`, unknown keys are forwarded to the
   provider as model options.
3. No onboarding wizard; provider keys are read from the environment at startup
   (multi-provider, one `<PROVIDER>_API_KEY` per provider).
4. Project config dir: `.opencode/` (with `agents/` inside it).
5. TUI takes `--prompt` and `--agent`; the prompt is not positional.

## Changes

TASK-1 (already implemented in a previous session, only committed here):
`scripts/run.sh` — `--tool` flag with `claude-code` default, validated against
`claude-code|opencode`, error on anything else.

TASK-2: `Dockerfile` — added `RUN npm install -g opencode-ai` next to the Claude
Code install so both CLIs exist in the image.

TASK-3: `Dockerfile` — derives `~/.config/opencode/agents/*.md` from the same
`agents/` sources already copied to `~/.claude/agents/`, stripping the `name:`
and `tools:` frontmatter keys with a small awk pass (name comes from the
filename in OpenCode, and its `tools` schema is a map, not a comma list, so
copying them verbatim would misconfigure the agents). Bodies stay identical.

TASK-4: `scripts/entrypoint.sh` — reads `manigot_TOOL` and branches: the
`~/.claude.json` onboarding bypass and the `CLAUDE_*` requirement now only apply
to `claude-code`; for `opencode` it verifies at least one provider key is
present and `exec`s `opencode` instead of `claude`. Git config handling is
shared and unchanged. `scripts/run.sh` — passes `-e manigot_TOOL` and shapes
the initial job prompt per tool (positional for Claude Code, `--prompt` for
OpenCode); `--agent` is spelled the same for both.

TASK-5: `scripts/run.sh` — `docs/` mount target is now
`/workspace/.opencode` for OpenCode, `/workspace/.claude` for Claude Code, so
each tool finds project-level agents where it looks for them.

TASK-6: `scripts/run.sh` — allowlist of provider key env vars
(`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`,
`GOOGLE_GENERATIVE_AI_API_KEY`, `GROQ_API_KEY`, `XAI_API_KEY`,
`DEEPSEEK_API_KEY`, `OPENCODE_API_KEY`) read from `.env` and forwarded with `-e`
on the OpenCode path only, with an error if none is set. Optional
`OPENCODE_MODEL` is forwarded too, otherwise a fresh container has no model
selected. The `ANTHROPIC_API_KEY` subscription guard and the
`CLAUDE_CODE_OAUTH_TOKEN` requirement are now scoped to the `claude-code` path,
so they neither weaken Claude Code's protection nor block OpenCode, which uses
that key legitimately.

TASK-7: `README.md` — OpenCode setup section (provider keys, `OPENCODE_MODEL`,
why `ANTHROPIC_API_KEY` is allowed there but not for Claude Code), `--tool`
usage examples, and a comparison table of the per-tool differences.

TASK-8: `docs/CLAUDE.md` — Stack/Architecture/Commands rewritten as
vendor-agnostic: both CLIs in the image, `--tool`, dual agent bake locations,
per-tool mount targets, `manigot_TOOL` branching, OpenCode `.env` variables.

TASK-9: `project-template/docs/CLAUDE.md` — tool-neutral header note explaining
that the file is loaded by whichever CLI is selected, where `docs/` is mounted
per tool, and that project context should be written for "the agent".

TASK-10: `scripts/run.sh` — `docs/AGENTS.md` is now the canonical project
context file, mounted read-only a second time at the path the selected tool
actually reads context from: `/workspace/AGENTS.md` for OpenCode,
`/workspace/.claude/CLAUDE.md` for Claude Code. `docs/CLAUDE.md` is used as a
fallback when `docs/AGENTS.md` is absent, so existing projects keep working, and
a warning is printed when neither exists. The resolved mapping is shown in the
startup banner. This closes the gap that made OpenCode blind to project context.

TASK-11: renamed `docs/CLAUDE.md` → `docs/AGENTS.md` and
`project-template/docs/CLAUDE.md` → `project-template/docs/AGENTS.md`, and
updated every reference: `README.md` tree and per-project setup, the
architecture and hard-rule entries in `docs/AGENTS.md`, and
`agents/product-owner.md`, which told the agent to read `CLAUDE.md` by name.

TASK-12: `README.md` — scoped "subscription billing" to the Claude Code path and
stated OpenCode's per-token billing, marked the agents table's Tools column as
Claude Code-only with an explicit warning that read-only agents are unrestricted
under OpenCode, added the context-file mount row to the comparison table, and
documented that one `docs/AGENTS.md` serves both tools.

Out-of-task housekeeping (separate commit): `.gitignore` now ignores `.claude/`
and `.opencode/`. These are the in-container bind-mount targets for `docs/`, so
a `git add -A` from inside a session would otherwise commit a duplicate copy of
`docs/`.

## Known issues / follow-ups

- ~~OpenCode does not see `docs/CLAUDE.md`~~ — resolved by TASK-10/TASK-11:
  `docs/AGENTS.md` is now the canonical name and is mounted where each tool
  looks for it. Worth verifying in a real session, since the context mount is
  nested inside the `docs/` mount and relies on the container runtime ordering
  mounts parent-first (both Docker and Podman sort by target depth).
- **The image was not built.** No Docker daemon is available inside manigot, so
  `npm install -g opencode-ai` and the agent-transform `RUN` step in the
  Dockerfile are unverified in a real build. The awk transform itself was run
  against all six `agents/*.md` files on the host and produces valid frontmatter
  (`description` only) with byte-identical bodies. Run `make rebuild` and one
  `manigot --tool opencode` session before merging.
- **The provider key allowlist is duplicated** in `scripts/run.sh` and
  `scripts/entrypoint.sh` (host-side vs container-side check). Both carry a
  comment to keep them in sync; if the list grows, consider passing the names
  through as a single env var instead.
- **Agent tool restrictions are not enforced under OpenCode.** The Claude Code
  agents declare `tools:` (e.g. read-only for `@reviewer`), which is dropped in
  the OpenCode copies. OpenCode's equivalent is a `permission:` block, so
  read-only agents are effectively unrestricted there. Adding per-agent
  `permission:` frontmatter would be a follow-up job.
- **No `--tool` support in `new-job.sh`** and no separate build target for
  OpenCode — both explicitly out of scope per `tasks.md`.
