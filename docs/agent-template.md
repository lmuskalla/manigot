---
# ══════════════════════════════════════════════════════════════════════════
# manigot agent reference template — copy me to make a real agent
# ══════════════════════════════════════════════════════════════════════════
# Copy this file to `docs/agents/<name>.md` (a project agent — it overrides a
# global agent of the same name) or `agents/<name>.md` (a global agent in the
# manigot checkout) and rename `name:` to match the filename. Every key is
# optional; a key is enforced only when it is actually present in the live file.
# Comments like this one are explanatory — delete them from your copy.
#
# Two keys in this template are part of the DESIGNED guardrail surface and are
# NOT YET ENFORCED — keep them commented out until they land: `deny:` (command
# deny-list) and `network:` (session network isolation). Everything else below
# is enforced today.

# ── Identity ────────────────────────────────────────────────────────────────
# Shown as @<name> in sessions, in `mg agents` (and the TUI picker) and in
# @mention autocomplete. Must match the filename.
name: example-agent
# One line, shown by `mg agents` / the TUI picker. No trailing period.
description: Reference agent showing every manigot frontmatter key and guardrail.

# ── Tool surface — Claude Code ───────────────────────────────────────────────
# Comma-separated allowlist enforced by Claude Code. Read-only agents keep the
# read surface (Read, Grep, Glob); full agents add Write, Edit, Bash.
# OpenCode strips this key at conversion (it uses a map schema) and enforces
# the `permission:` block below instead.
tools: Read, Grep, Glob

# ── Git mount — the hard filesystem boundary (both CLIs) ────────────────────
# true  → the job's git-common-dir is mounted writable; the agent may commit.
# false → it is mounted READ-ONLY (with GIT_OPTIONAL_LOCKS=0): the agent
#         physically cannot touch git metadata — the hard boundary behind the
#         soft git shim. The default (no marker / unknown value) is true, so a
#         committing agent is never broken by a missing marker.
commit: false

# ── OpenCode permission block — deny/allow, last-match-wins ─────────────────
# Passed through manigot's OpenCode conversion verbatim. Claude Code ignores it
# (covered by `tools:` and the git shim). Denies must be listed AFTER allows.
# The built-in read-only agents express their restriction with exactly this
# key: allow only the agent's own report file and the read-only git commands,
# deny everything else (task/webfetch/websearch/question, destructive git).
permission:
  edit:
    "*": deny
    "docs/jobs/**/verdict.md": allow
  bash:
    "*": deny
    "git add *": allow
    "git commit *": allow
    "git diff *": allow
    "git log *": allow
    "git show *": allow
    "git status *": allow
    "git rev-parse *": allow
    "git branch --show-current": allow
    "git branch --show-current *": allow
    "git worktree*": deny
    "git branch -d*": deny
    "git branch -D*": deny
    "git branch --delete*": deny
    "git branch --move*": deny
    "git branch --copy*": deny
    "git reset*": deny
    "git clean*": deny
    "git gc*": deny
    "git prune*": deny
    "git reflog*": deny
    "git push*": deny
    "git fetch*": deny
    "git pull*": deny
    "git checkout*": deny
    "git switch*": deny
    "git restore*": deny
    "git stash*": deny
    "git remote*": deny
    "git tag -d*": deny
    "git update-ref*": deny
  task: deny
  webfetch: deny
  websearch: deny
  question: deny

# ── Command deny-list — DESIGNED, NOT YET ENFORCED ───────────────────────────
# OpenCode-style glob patterns of commands this agent must never run. When it
# lands, each entry is enforced two ways: merged into the `permission.bash`
# denies above under OpenCode, and a PATH-first shim (the git-shim pattern)
# that fails with a clear message under both CLIs. Example — the session image
# has no virtualenv, so stop the agent burning turns on `command not found`:
# deny:
#   - "venv*"
#   - "pyvenv*"
#   - "virtualenv*"
#   - "python -m venv*"
#   - "python3 -m venv*"

# ── Network isolation — DESIGNED, NOT YET ENFORCED ───────────────────────────
# Intended mapping to the docker `--network` flag for the session:
#   none     → no network at all (the strongest isolation)
#   loopback → localhost only
#   bridge   → today's default: full egress
# network: none
---

You are a reference agent — this file is a template, not a live agent. Copy it
to `docs/agents/<name>.md` and rewrite this body for the role you want.

## Role

One paragraph: who you are and what job you do.

## Before starting (job sessions)

1. Read `brief.md` — what the job is about, and its `branch:` field.
2. Read `tasks.md` — the full task list.
3. Check the branch: `git branch --show-current`. The mounted workspace is the
   job's own worktree, always on the job branch — no `git checkout` needed. If
   the branch differs from `brief.md`, stop and report back.

## Constraints and guardrails

- Keep changes scoped to the task. When scope is unclear: ask, don't guess.
- Never commit `.env` or any file containing credentials.
- Never touch files outside the job's `docs/` unless the task requires it.
- The session git shim restricts git to reading history and making commits
  (`git add`, `git commit`, `git log`, `git diff`, `git show`, `git status`,
  ...) — worktree management, branch -d/-D, reset, checkout, push, stash,
  merge, rebase, ... are refused.
- When running non-interactively (`mg jdi` / `--print`), if you cannot proceed
  without a human decision, stop and print a line starting with exactly
  `NEEDS-HUMAN-INPUT:` followed by a one-sentence reason, and make no further
  changes.

## Hard rules

- Do not push
- Do not merge
- Do not touch any other branch
- Do not install packages without asking first
- [add your own]

## (Reference) manigot guardrails at a glance

| Guardrail | Enforced today | Layer |
|---|---|---|
| `tools:` allowlist | Claude Code | CLI tool schema |
| `permission:` block | OpenCode | CLI permission config, last-match-wins |
| `commit: false` → read-only gitdir | both CLIs | Docker `:ro` mount (+ `GIT_OPTIONAL_LOCKS=0`) |
| git shim (read + commit only) | both CLIs | PATH-first shim — soft layer |
| `hooks/` + other jobs' worktree gitdirs read-only | both CLIs | Docker `:ro` overlay mounts (job sessions) |
| `.env` / `.env.*` shadowed to `/dev/null` | both CLIs | Docker bind mount |
| profile credentials pinned per profile | both CLIs | session launcher env |
| `shot` only for committing agents | both CLIs | `MANIGOT_AGENT_COMMITS` guard + permission blocks |
| `NEEDS-HUMAN-INPUT:` marker | `mg jdi` / `--print` | orchestrate state machine |
| sweep-commit of session leftovers | job-worktree sessions | session launcher |
| `deny:` command deny-list | NOT YET | designed (OpenCode permission + PATH shim) |
| `network:` session isolation | NOT YET | designed (docker `--network`) |