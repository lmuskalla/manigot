# Tasks: Auto mode for claude code

id: c4ouwc
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Confirm, against the exact `@anthropic-ai/claude-code` version pinned
in the image, (a) the current CLI flag for full permission-bypass ("auto
mode" / no per-tool-call confirmation) — expected to be
`--dangerously-skip-permissions`, and (b) the `~/.claude.json` schema key
that pre-accepts the folder-trust dialog for a given path — expected to be
`projects["<path>"].hasTrustDialogAccepted: true`. Both are assumptions
based on reading the live `~/.claude.json` of the current session, not on
the CLI's own docs/help output, and CLI flags/JSON schemas can change
between releases. No code change — investigation only, gates TASK-2/3.
- files: none (reference: `Dockerfile` for the pinned package, `claude
  --help` inside a built image)
- depends: none
- risk: low — read-only investigation; the risk it's meant to catch (wrong
  flag/key name) would otherwise land in TASK-2/3.

TASK-2: Extend the `~/.claude.json` heredoc written in the claude-code
branch of `scripts/entrypoint.sh` to also pre-accept the folder-trust
dialog for `/workspace` (the container's fixed `WORKDIR`), so the "do you
trust the files in this folder?" prompt never appears on first start,
regardless of launch path (plain `sc`, `sc --agent`, `sc --job`,
TUI-launched agent).
- files: `scripts/entrypoint.sh`
- depends: TASK-1
- risk: low — additive field to a JSON file this script already fully
  owns and regenerates per (ephemeral, `--rm`) container run; worst case
  the key is ignored by a newer CLI version and the prompt simply
  reappears, no destructive effect.

TASK-3: Add the confirmed permission-bypass flag to the final `exec claude
"$@"` invocation in `scripts/entrypoint.sh`, so every Claude Code session
started through safecode — interactive `sc`, `sc --agent`, job-prompted
runs, and TUI-triggered agent windows — begins already in full auto mode
instead of requiring a manual mode switch. Scoped to the existing
`claude-code` branch of the `SAFECODE_TOOL` conditional only; the
`opencode` branch and its `exec opencode "$@"` call are untouched — the
brief is explicit this is a Claude-Code-only request.
- files: `scripts/entrypoint.sh`
- depends: TASK-1
- risk: medium — this is a flag Anthropic itself names "dangerous"; it
  removes per-tool-call confirmation for every session with no opt-out
  toggle (brief's "Out of scope" section is blank, so scope is read as
  "always on for claude-code," but this should be called out, not
  assumed, when reviewed). Needs checking that it composes correctly with
  the pre-existing `"$@"` passthrough (positional job prompt / `--agent
  <name>`) and that it isn't rejected for any reason specific to running
  as the non-root `claude` user already in place.

TASK-4: Update the `scripts/entrypoint.sh` bullet in `docs/AGENTS.md`'s
Architecture section (currently: "writes `~/.claude.json` to skip Claude
Code's onboarding wizard") to also state that it pre-accepts folder trust
for `/workspace` and starts Claude Code in permission-bypass mode, per the
project's own rule to keep this file accurate.
- files: `docs/AGENTS.md` (canonical source per this repo's own
  instructions); `.claude/CLAUDE.md` and `.claude/AGENTS.md` in the repo
  root carry the same paragraph and may need the same edit — first
  confirm whether they're generated/synced from `docs/AGENTS.md` or
  independently tracked copies, rather than assuming.
- depends: TASK-2, TASK-3 (doc should describe what was actually built)
- risk: low — documentation only, no behavior change.

TASK-5: Manually verify, across all three launch paths (`sc` with no
flags, `sc --agent <agent> --job <id>`, and a TUI-triggered agent launch),
that no folder-trust or per-tool permission prompts appear, and that
unrelated safeguards still work as before (git identity from
`GIT_AUTHOR_NAME_CFG`/`GIT_AUTHOR_EMAIL_CFG`, `docs/` mount, job-prompt
text, `--agent` flag). Verification only, not a code change.
- files: none (exercises `scripts/run.sh`, `scripts/entrypoint.sh`, and
  `tui/` end to end)
- depends: TASK-2, TASK-3
- risk: low as an activity, but it's the task that actually confirms the
  feature works — a failure here means TASK-2 or TASK-3 needs rework.

## Open questions for review

- No opt-out is planned for the permission-bypass flag (TASK-3) — the
  brief's "Out of scope" section was left blank. If a per-user or
  per-project toggle is wanted later (e.g. via `config/tui-settings.json`,
  which already exists for other per-user preferences), that's a separate
  follow-up job, not part of this one.
