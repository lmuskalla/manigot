# Verdict: new agent: git solver

id: baby
status: open
reviewer: claude
date: 2026-08-25

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `agents/git-solver.md` matches the required shape exactly.
Frontmatter (`name: git-solver`, one-line `description:`, `tools: Read,
Write, Edit, Bash, Grep, Glob`, `commit: true`) and the OpenCode
`permission:` block are byte-identical in structure to `developer.md`'s
(same broad `edit`/`bash` allow, same destructive-git-command denylist:
worktree, branch -d/-D/--delete/--move/--copy, reset, clean, gc, prune,
reflog, push, fetch, pull, checkout, switch, restore, stash, remote, tag -d,
update-ref) — no special exemption carved out, as required. Body includes
the standard "Branch" check section (verbatim match to `devops.md`'s
wording), a "What you cover" section (diagnosing detached
HEADs/conflicts/rebases/stray worktrees, resolving conflicts, safe
merge/cleanup guidance), an explicit "Container limitation" section that
correctly states what's possible inside a job/container session (diagnose,
resolve conflicts via edit+commit, inspect history) vs. what the shim
refuses (worktree fixes, force-remove, hard reset, branch delete), pointing
the user at `mg host` for the latter, and closing "Hard rules" mirroring
`developer.md`'s (no push, no merge, no touching other branches, no
routing around the shim). No architecture files touched.

TASK-2: PASS
notes: `README.md` — "Thirteen agents" → "Fourteen agents" (line ~491), and
a `@git-solver` row appended at the end of the agent table (line 511:
`| \`@git-solver\` | Git expert for tricky states — broken worktrees,
conflicts, cleanup | read + write |`), matching the table's existing column
style and append-only row ordering. Row count (14) now matches the stated
count and the actual `agents/*.md` file count (verified: `ls agents/*.md |
wc -l` → 14).

TASK-3: PASS
notes: `docs/ROADMAP.md`'s current-state paragraph — "thirteen agents" →
"fourteen agents". Confirmed no other stale "thirteen"/"Thirteen" agent-count
references remain anywhere in the tree (`grep -rn "thirteen" --include=*.md`
outside the job dir returns nothing).

TASK-4: PASS
notes: Verification claims in `implementation.md` independently
reproduced:
- `go build ./...` succeeds.
- `go test ./...` — the exact same package set passes/fails as reported
  (`cmd/mg`, `internal/git`, `internal/job`, `internal/session`,
  `internal/ui` fail solely on `git init`/`worktree`/`push` refused by this
  review session's own git shim — confirmed by inspecting the failure
  output directly, e.g. `TestCommitAll`: "git 'init' is not allowed in
  agent sessions" — a pre-existing limitation of running this suite from
  inside any manigot agent session, unrelated to this diff). All other
  packages, including `internal/agentlist`, pass.
- `git diff main...HEAD -- internal/ui/agents.go internal/agents/agents.go
  internal/orchestrate/ Dockerfile scripts/entrypoint.sh` is empty —
  confirmed no changes to the TUI action bar, the job-flow agent
  enumeration, the Dockerfile, or the entrypoint shim.
- Worktree is clean (`git status --short` empty) — no stray temp files left
  behind from the ad-hoc `agentlist.Discover` check mentioned in
  `implementation.md`.

## Commit discipline

PASS — one commit per task in the `[baby] TASK-N: ...` format
(`0ebf359`, `036e891`, `fe006ea`), plus a separate `[baby] implementation:
add summary` commit (`48cf34d`). TASK-4 is verification-only per
`tasks.md` and correctly has no commit of its own.

## Security

None — no new attack surface. The agent gets exactly the same
read+commit git surface every other committing agent already has; the
destructive-command denylist is unchanged and not weakened for this agent.
No secrets, no new external calls, no changes to the shim or the
git-common-dir mount logic itself.

## Overall

APPROVED

No blockers. Implementation matches `tasks.md` precisely: a single new
global agent file with the exact permission/frontmatter shape specified,
explicit and accurate container-shim-limitation language (does not imply
capabilities the platform blocks), and both doc-sync tasks (README table +
count, ROADMAP count) correctly applied and cross-checked against the
actual `agents/*.md` file count. No unrelated files touched, no scope
creep, commit discipline followed. `implementation.md`'s "Known issues /
follow-ups" correctly notes the container image needs `make rebuild`
before `@git-solver` is usable inside running sessions — an ops step, not
a code gap.
