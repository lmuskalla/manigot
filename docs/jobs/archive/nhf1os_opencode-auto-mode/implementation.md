# Implementation: opencode auto mode

id: nhf1os
status: open
developer: deepseek-v4-flash
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

OpenCode sessions launched through manigot now start in full auto mode,
mirroring the Claude Code behavior (which already runs with
`--dangerously-skip-permissions`): the per-tool confirmation prompt ("can I
run this python script?") no longer appears. Two changes in
`scripts/entrypoint.sh`, scoped entirely to the existing `opencode` branch:

1. Interactive sessions (`exec opencode "$@"`) now pass `--auto` — covers
   every interactive launch path (plain `mg --profile zai|opencode-go`,
   `mg --agent`, job-prompted runs, TUI-triggered agent windows, legacy
   `--tool opencode`).
2. Headless `--print` sessions (`mg jdi --profile zai|opencode-go`) now pass
   `--auto` alongside `--format json`.

The claude-code branch and its `--dangerously-skip-permissions` flag are
untouched. Documentation (`docs/AGENTS.md`, `README.md`) updated to match.

TASK-1 (investigation, no code) confirmed the flag exists on the exact
opencode-ai version the image installs (1.18.16 in this environment) on both
the interactive `opencode [project]` command and the headless `opencode run`
subcommand, described in `--help` as "auto-approve permissions that are not
explicitly denied (dangerous!)" — the direct OpenCode analog of Claude Code's
`--dangerously-skip-permissions`. It also confirmed (a) OpenCode has no
folder-trust/onboarding dialog equivalent to Claude's `hasTrustDialogAccepted`
— the brief's complaint is per-tool permission prompts, not a trust dialog —
and (b) neither the global opencode config (written by entrypoint.sh only when
`OPENCODE_MODEL` is set) nor any baked agent frontmatter contains explicit
`permission:` deny rules, so `--auto`'s "not explicitly denied" semantics
yield full auto in practice.

## Changes

TASK-1: investigation only, no files changed. Verified live against the
installed `opencode` 1.18.16 binary via `opencode --help` and `opencode run
--help`: `--auto` (boolean, "auto-approve permissions that are not explicitly
denied (dangerous!)") is present on both the interactive `opencode [project]`
command and the headless `opencode run` subcommand, composing cleanly with
`--format json`. Grepped the container's opencode config
(`~/.config/opencode/opencode.json`: only `$schema` + `model`) and all baked
agents under `~/.config/opencode/agents/` — zero `permission:` deny rules, so
`--auto` = full auto here.

TASK-2: `scripts/entrypoint.sh` — the interactive opencode exec is now
`exec opencode --auto "$@"` (was `exec opencode "$@"`), flag placed before
`"$@"` so it composes with the `--agent <name>` / `--prompt <text>` / positional
passthrough, with a comment mirroring the claude-code branch's
`--dangerously-skip-permissions` comment.

TASK-3: `scripts/entrypoint.sh` — the headless (`MANIGOT_PRINT`) opencode
branch now builds `OC_ARGS+=(--auto --format json)` (was `--format json`), so
`opencode run <prompt> --agent <agent> --auto --format json` is the exact
translated shape. Comment notes the foycfl finding (headless run auto-executes
bash/write even without `--auto`) and frames the flag as making the intent
explicit and guarding other tools (webfetch, task, lsp, mcp) against an
unanswered "ask" prompt stalling an unattended non-TTY run.

TASK-4: `docs/AGENTS.md` (canonical source) — rewrote the `scripts/entrypoint.sh`
bullet's opencode half: "checks for a provider API key and starts OpenCode in
auto mode via `--auto` (full auto, no per-tool prompts — the direct OpenCode
analog of Claude's `--dangerously-skip-permissions`...)" and updated the
headless description to `opencode run <message> --agent <agent> --auto
--format json`. `README.md` — added a "Permissions" row to the "Choosing a
profile" table (auto-approved via `--dangerously-skip-permissions` vs
auto-approved via `--auto`) and a paragraph after the table stating both CLIs
start with all permissions auto-approved. Confirmed `docs/CLAUDE.md` (empty)
and `project-template/docs/AGENTS.md` (no entrypoint bullet) need no change.

TASK-5: verification, no code change. All checks that are possible in this
sandbox pass:
- `bash -n scripts/entrypoint.sh` — syntax OK.
- Headless translation reproduced live: `opencode run "Reply with exactly:
  FLAG-OK" --agent developer --auto --format json` → exit 0, JSONL events on
  stdout (`step_start`/`text`/`step_finish`), agent replied "FLAG-OK".
- Interactive shape reproduced live: `opencode --auto --prompt "Reply with
  exactly: TTY-CHECK" --agent developer` → exit 0, TUI session ran with the
  `auto` mode badge shown in the status bar, agent replied "TTY-CHECK".
- `git diff f4a0adc..HEAD` (pre-job merge-base `f4a0adc`) — only intended
  changes: `scripts/entrypoint.sh` (opencode branch only; the claude-code
  branch appears as context), `docs/AGENTS.md`, `README.md`, and the job's own
  `docs/jobs/nhf1os_opencode-auto-mode/` files. Unrelated safeguards
  (provider-key forwarding in run.sh, OPENCODE_MODEL config write, git
  identity, docs mount) are untouched.
- Full end-to-end through the real Docker container is not possible here (no
  `docker` binary — same limitation the foycfl job hit). Outstanding human
  check: one real `mg --profile zai` session and one `mg jdi --job <id>
  --profile zai` run, confirming no permission prompt appears. Note the image
  must be rebuilt (`make rebuild`) since `scripts/entrypoint.sh` is COPYied in
  at build time.

## Known issues / follow-ups

- `--auto`'s semantics are "auto-approve permissions that are not explicitly
  denied" — slightly narrower than Claude's blanket
  `--dangerously-skip-permissions`. Net effect is identical in the container
  (no explicit denies exist), and the narrower semantics are deliberate: a
  future config-level `deny` (opencode.json `permission:` or agent frontmatter)
  stays enforced even in auto mode. No action needed.
- No opt-out toggle for opencode auto mode — same decision as the c4ouwc
  claude-code job. A per-user toggle (e.g. via `config/tui-settings.json`)
  would be a separate follow-up.
- Live interactive-TTY verification through the real container still owed (see
  TASK-5) — should be done by a human on the host before/after merging, plus
  `make rebuild` is required for the change to take effect.
