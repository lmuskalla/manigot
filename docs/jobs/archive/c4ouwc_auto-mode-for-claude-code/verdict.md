# Verdict: Auto mode for claude code

id: c4ouwc
reviewer: glm-5.2
date: 2026-08-09

## Per-task findings

TASK-1: PASS
notes: Investigation verified against the actually-installed binary
(`claude --version` → 2.1.226 in this environment). `--dangerously-skip-permissions`
is present in `claude --help` and is distinct from
`--allow-dangerously-skip-permissions` (the latter only *enables* the option,
doesn't turn it on) — confirming the analyst's flag choice. The folder-trust
key and the extra `bypassPermissionsModeAccepted` finding are consistent with
the resulting JSON being accepted. No code produced, correctly. No commit, correct.

TASK-2: PASS
notes: `scripts/entrypoint.sh` heredoc now writes both
`bypassPermissionsModeAccepted: true` (top level) and
`projects["/workspace"].hasTrustDialogAccepted: true`. Rendered the heredoc
with sample env vars and parsed with `json.tool` → valid JSON, both keys
present and correctly typed/placed. `bash -n` clean. Trust path `/workspace`
matches the image's `WORKDIR /workspace` (Dockerfile:85). The file is only
written `if [[ ! -f "$CLAUDE_JSON" ]]`, and the container is `--rm`, so it is
regenerated every ephemeral run — correct.

TASK-3: PASS
notes: `exec claude "$@"` → `exec claude --dangerously-skip-permissions "$@"`
in the `else` (claude-code) branch only. Flag placed *before* `"$@"`, which is
correct: run.sh builds `"$@"` as `(--agent <name>) (<positional job prompt>)`
for claude-code (PROMPT_ARGS is positional, not `--prompt`), so composition is
clean for plain `sc`, `sc --agent --job`, and TUI launches. The opencode
branch (`exec opencode "$@"`) is genuinely untouched. Non-root user (`claude`,
UID 1000) avoids the binary's `refuseBypassUnderRoot` guard.

TASK-4: PASS
notes: `docs/AGENTS.md` bullet updated to mention pre-accepted folder trust +
`--dangerously-skip-permissions`. Scope decision verified correct: at HEAD the
only tracked file carrying this paragraph is `docs/AGENTS.md`.
`project-template/docs/AGENTS.md` does not contain it; `docs/CLAUDE.md` is
empty. The repo-root `AGENTS.md` is tracked but committed *empty*
(blob `e69de29`) as a placeholder that is populated at runtime via the bind
mount (it shares an inode with `docs/AGENTS.md` in this container) — so not
editing it as source is right. (See minor note below re: implementation.md
wording.)

TASK-5: PASS (with one acknowledged gap)
notes: Everything verifiable from inside the container was done: syntax check,
JSON render, flag presence in `--help`, non-root check, and confirmation via
`tui/internal/launch/launch.go` that all three launch paths route through
`run.sh` → `docker run` → `entrypoint.sh` → `exec claude …`. The live
interactive-TTY observation (watching a real session for zero prompts) is
inherently a host-side human check and is explicitly flagged as owed by the
developer. Mechanism is sound; the remaining check is confirmatory, not
correctness-gating.

## Commit discipline

- `[c4ouwc] TASK-2`, `[c4ouwc] TASK-3`, `[c4ouwc] TASK-4` — one commit each,
  correct `[ID] TASK-N: description` format.
- TASK-1 (investigation) and TASK-5 (verification) produce no code and
  correctly have no commits.
- `implementation.md` has its own commit (`[c4ouwc] implementation: add summary`).
- No task is squashed into another; no extra/out-of-scope commits.

## Scope

- Changed files: `scripts/entrypoint.sh`, `docs/AGENTS.md`, and the new
  `implementation.md` only. No unrelated refactoring.

## Bugs / edge cases

- None found. Flag ordering, trust-path value, ephemeral regeneration, and
  non-root execution all check out.

## Non-blockers (noting for completeness, no action required to merge)

1. **Uncommitted working-tree change to root `AGENTS.md`.** Because root
   `AGENTS.md` is bind-mounted/hardlinked to `docs/AGENTS.md` (same inode),
   editing the latter also changed the working-tree bytes of the former, so
   `git status` shows ` M AGENTS.md`. This is *not* committed (the TASK-4
   commit staged only `docs/AGENTS.md`) and will not travel with the PR — but
   the developer should make sure not to `git add AGENTS.md` before merge.
2. **Minor wording inaccuracy in implementation.md.** It states the repo-root
   `AGENTS.md` is "not tracked source"; it is in fact tracked, just committed
   empty as a placeholder. The *decision* (don't edit it) was correct either
   way; only the justification is slightly off. Cosmetic.
3. **No opt-out toggle** for `--dangerously-skip-permissions`. This matches the
   blank "Out of scope" in the brief and is called out as a future follow-up —
   not a defect.
4. **File has no trailing newline** at EOF — pre-existing (main and HEAD both
   end with `fi`), not introduced by this change.

## Overall: APPROVED

The implementation does exactly what the brief and tasks asked, is correctly
scoped to the claude-code branch, surfaces and handles a real gotcha
(`bypassPermissionsModeAccepted` being required for the flag to take effect in
interactive sessions), and is cleanly committed per task. Nothing here blocks
merge.

Recommended pre-merge host check (the only thing TASK-5 couldn't do from inside
the container): run `make rebuild`, then `sc` and a TUI agent launch, and
confirm no folder-trust or permission prompt appears.
