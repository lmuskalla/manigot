# Tasks: opencode auto mode

id: nhf1os
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Confirm the auto-approve mechanism against the exact `opencode-ai`
version the image installs (Dockerfile's unpinned `npm install -g
opencode-ai`; 1.18.16 in this environment). Expected: the `--auto` boolean
flag, present on both the interactive `opencode [project]` command and the
headless `opencode run` subcommand, described in `--help` as
"auto-approve permissions that are not explicitly denied (dangerous!)" —
the direct OpenCode analog of Claude Code's
`--dangerously-skip-permissions` (verified live via `--help` at 1.18.16,
composes cleanly with `--format json`). Also confirm (a) OpenCode has no
folder-trust/onboarding dialog equivalent to Claude's
`~/.claude.json` `hasTrustDialogAccepted` — the brief's complaint is
per-tool permission prompts (e.g. running a python script), not a trust
dialog — and (b) the container's global opencode config (written by
entrypoint.sh only when `OPENCODE_MODEL` is set) contains no explicit
`permission:` deny rules, so `--auto`'s "not explicitly denied" semantics
yield full auto in practice. No code change — investigation only, gates
TASK-2/3.
- files: none (reference: `Dockerfile`, `scripts/entrypoint.sh`, `opencode
  --help` / `opencode run --help` against the installed binary)
- depends: none
- risk: low — read-only investigation; the version-drift/flag-name risk it
  catches would otherwise land in TASK-2/3.

TASK-2: Add `--auto` to the interactive OpenCode exec in
`scripts/entrypoint.sh`: `exec opencode "$@"` → `exec opencode --auto "$@"`
(flag placed before `"$@"`, mirroring the claude-code branch's
`exec claude --dangerously-skip-permissions "$@"`). This single line covers
every interactive opencode launch path — plain `mg --profile zai|opencode-go`,
`mg --agent`, job-prompted runs, TUI-triggered agent windows, and the legacy
profile-less `--tool opencode` path — so the per-tool "can I run this
python script?" confirmation stops appearing, exactly like Claude's auto
mode. The claude-code branch and its flag stay untouched.
- files: `scripts/entrypoint.sh`
- depends: TASK-1
- risk: medium — changes every interactive opencode session; the flag is
  officially marked "dangerous", but the container is isolated and
  ephemeral and this mirrors the established c4ouwc precedent for Claude.
  Needs confirming it composes with the existing `"$@"` passthrough
  (`--prompt`/`--agent`/positional args).

TASK-3: Add `--auto` to the headless OpenCode invocation in
`scripts/entrypoint.sh`'s `MANIGOT_PRINT` branch: `OC_ARGS+=(--auto
--format json)`, so `mg jdi --profile zai|opencode-go` and `--print`
sessions run explicitly auto-approved. Note: the archived foycfl job
verified headless `opencode run` auto-executes `bash`/`write` tool calls
even without `--auto`, so this is partly a safety net — it makes the intent
explicit and guards other tools (webfetch, task, lsp, mcp) against an
unanswered "ask" prompt stalling an unattended non-TTY run.
- files: `scripts/entrypoint.sh`
- depends: TASK-1
- risk: low — additive flag on a path already verified to auto-execute in
  practice; worst case it's redundant. Slightly beyond the strict brief
  (which describes interactive prompting) — included for consistency with
  the claude-code branch, which applies the bypass on both paths; call out
  at review.

TASK-4: Update documentation to match the new behavior:
`docs/AGENTS.md`'s `scripts/entrypoint.sh` bullet — the opencode half
currently reads "checks for a provider API key and execs `opencode`" and
the headless description (`opencode run <message> --agent <agent> --format
json`) — to state opencode sessions start in auto mode via `--auto` (full
auto, no per-tool prompts, like Claude's `--dangerously-skip-permissions`),
and README.md's "Choosing a profile" table (row: "Onboarding | bypassed by
writing `~/.claude.json` | nothing to bypass") / surrounding text to note
opencode sessions start with all permissions auto-approved. Confirm
`docs/CLAUDE.md` (currently empty) and `project-template/docs/AGENTS.md`
(no entrypoint bullet) need no change.
- files: `docs/AGENTS.md` (canonical source), `README.md`
- depends: TASK-2, TASK-3 (doc should describe what was actually built)
- risk: low — documentation only, no behavior change.

TASK-5: Verify: `bash -n scripts/entrypoint.sh` passes; reproduce the exact
headless translation (`opencode run <prompt> --agent <agent> --auto --format
json`) and the interactive shape (`opencode --auto --prompt ...`) against
the real `opencode` binary for flag acceptance; confirm the claude-code
branch and unrelated safeguards (provider-key forwarding, OPENCODE_MODEL
config write, git identity, docs mount) are untouched by diffing against
the pre-job commit. Full end-to-end through the real Docker container is
not possible in this sandbox (no `docker` binary — same limitation the
foycfl job hit); a human with a working `mg` install should do one real
`mg --profile zai` session and one `mg jdi --job <id> --profile zai` run
and confirm no permission prompt appears. Verification only, no code change.
- files: none (exercises `scripts/run.sh` + `scripts/entrypoint.sh` end to
  end where possible)
- depends: TASK-2, TASK-3
- risk: low as an activity, but it's the task that actually confirms the
  feature works — a failure here means TASK-2 or TASK-3 needs rework.

## Open questions for review

- `--auto`'s semantics are "auto-approve permissions that are not
  explicitly denied" — slightly narrower than Claude's blanket
  `--dangerously-skip-permissions`. Net effect is identical in the
  container (no explicit denies exist), and the narrower semantics are
  deliberately kept: a future config-level `deny` (opencode.json
  `permission:` or agent frontmatter) stays enforced even in auto mode.
- No opt-out toggle is planned for opencode auto mode — same decision as
  the c4ouwc claude-code job. A per-user toggle (e.g. via
  `config/tui-settings.json`) would be a separate follow-up.
