# Implementation: Auto mode for claude code

id: c4ouwc
status: open
developer: glm-5.2
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

Claude Code sessions launched through safecode now start in full auto mode
with no first-run prompts. Two mechanisms were added, both scoped to the
existing `claude-code` branch of `scripts/entrypoint.sh`:

1. The `~/.claude.json` heredoc now pre-accepts (a) the folder-trust dialog
   for `/workspace` and (b) the one-time permission-bypass disclaimer.
2. The final `exec claude` now passes `--dangerously-skip-permissions`.

The opencode branch is untouched (the brief is Claude-Code-only).

TASK-1 (investigation, no code) confirmed both assumptions in `tasks.md`
against the pinned `@anthropic-ai/claude-code` (2.1.226) and surfaced one
extra requirement that the analyst couldn't have known from the live
`~/.claude.json` alone: the bypass flag is silently downgraded to default
mode in interactive TTY sessions unless `bypassPermissionsModeAccepted` is
also set. That finding drove a one-line addition to TASK-2's heredoc.

## Changes

**TASK-1 — investigation only, no files changed.** Findings:

- (a) Permission-bypass flag: `--dangerously-skip-permissions` confirmed via
  `claude --help` ("Bypass all permission checks. Recommended only for
  sandboxes with no internet access."). Distinct from
  `--allow-dangerously-skip-permissions`, which only *enables bypass as an
  option* without turning it on — not what we want.
- (b) Folder-trust key: `projects["/path"].hasTrustDialogAccepted: true`
  confirmed by extracting strings from the pinned binary — found
  `].hasTrustDialogAccepted: true`, `checkHasTrustDialogAccepted`, and the
  `Hp()` reader keyed by cwd (`t.projects[e]`).
- **Extra finding (gates TASK-3):** the binary contains logic that downgrades
  bypass mode to default in interactive sessions unless top-level
  `bypassPermissionsModeAccepted: true` is set — literal message: *"Permission
  mode downgraded to default — bypass requires accepting the disclaimer
  interactively first"*. So `--dangerously-skip-permissions` alone would NOT
  achieve the brief's goal in interactive `sc` sessions; the JSON key is also
  required. This is the exact risk TASK-1 was chartered to catch.

**TASK-2 — `scripts/entrypoint.sh`.** Extended the `~/.claude.json` heredoc
with two keys (and an explanatory comment):
- `projects["/workspace"].hasTrustDialogAccepted = true` — pre-accepts the
  folder-trust dialog for the container's fixed `WORKDIR`.
- `bypassPermissionsModeAccepted = true` (top level) — pre-accepts the bypass
  disclaimer so TASK-3's flag takes effect in interactive TTY sessions.

**TASK-3 — `scripts/entrypoint.sh`.** Changed `exec claude "$@"` to
`exec claude --dangerously-skip-permissions "$@"`. Flag placed before `"$@"`
so it composes with the `--agent <name>` / positional job-prompt passthrough.
The opencode branch (`exec opencode "$@"`) is intentionally untouched.

**TASK-4 — `docs/AGENTS.md`.** Updated the `scripts/entrypoint.sh` bullet in
the Architecture section to state that it also pre-accepts folder trust for
`/workspace` and starts Claude Code in permission-bypass mode via
`--dangerously-skip-permissions`. Scope check: among tracked files only
`docs/AGENTS.md` carries this paragraph. `docs/CLAUDE.md` is empty,
`project-template/docs/AGENTS.md` does not contain it, and the repo-root
`AGENTS.md` / `.claude/CLAUDE.md` / `.claude/AGENTS.md` are runtime mounts
(not tracked source), so they were not edited.

**TASK-5 — verification (no code).** Performed all checks possible from
inside the container:
- `bash -n scripts/entrypoint.sh` — syntax OK.
- Rendered the heredoc with sample vars → `python3 -m json.tool` — valid JSON.
- `claude --help` confirms the flag is recognized by 2.1.226.
- Current user is UID 1000 (`claude`, non-root) — the binary's
  `refuseBypassUnderRoot` guard only blocks root/sudo, so the flag is accepted.
- Empirical: `claude --dangerously-skip-permissions --print "…"` runs with no
  permission/trust prompt and exit 0; the same flag combined with
  `--agent <name>` + a positional prompt (mirroring `sc --agent --job`) is
  accepted by the arg parser without rejection.
- Confirmed via `tui/internal/launch/launch.go` that all three launch paths
  (plain `sc`, `sc --agent --job`, TUI) route through `scripts/run.sh` →
  `docker run` → `entrypoint.sh` → `exec claude …`, so the single edit covers
  all of them.
- **Not verifiable from this context:** the live interactive TTY behaviour
  (watching a real `sc`/TUI session to confirm zero on-screen prompts).
  That is inherently a human-at-the-host check and should be confirmed by the
  reviewer. The mechanism (correct keys + correct flag) is verified against
  the binary's own logic.

## Known issues / follow-ups

- **No opt-out toggle.** `--dangerously-skip-permissions` is now always on for
  Claude Code sessions, with no per-user/per-project switch (the brief's
  "Out of scope" was blank). If a toggle is wanted later, `config/tui-settings.json`
  already exists for per-user preferences and would be the natural home —
  flagged as a separate follow-up job, consistent with the analyst's open
  question in `tasks.md`.
- **Image rebuild required.** `scripts/entrypoint.sh` is `COPY`ied into the
  image at build time (`Dockerfile` line 72). The change only takes effect
  after `make rebuild`; running against a stale image keeps the old behaviour.
- **`bypassPermissionsModeAccepted` may be migrated on first run.** The binary
  contains a migration (`xLh`) that, when this key is set, records the
  equivalent consent in the newer "userSettings" store
  (`skipDangerousModePermissionPrompt: true`) and removes the old key. The
  effect persists across the migration; behaviour is unaffected. Noted for
  future maintainers reading `~/.claude.json` after a first run.
- **Live interactive-TTY verification still owed** (see TASK-5) — should be
  done by the reviewer on the host before merging.
