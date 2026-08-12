# Verdict: make reviewer agent diff base-branch aware

id: ojhwyg
status: open
reviewer: @reviewer
date: 2026-08-12

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: agents/reviewer.md step 5 replaces the hardcoded `git diff main...HEAD`
with a base-branch resolution reading `baseBranch` from `.manigot/manigot.json`
(fallback `main`) plus the runnable snippet:
```
BASE=$(sed -n 's/^.*"baseBranch"[[:space:]]*:[[:space:]]*"\([^"]*\)".*$/\1/p' .manigot/manigot.json | head -n1)
git diff "${BASE:-main}...HEAD"
```
Verified independently:
- `bash -n` on the snippet: parses clean.
- Scratch repo with `baseBranch: development` + a `development` branch: BASE
  resolves to `development` and `git diff development...HEAD` shows exactly the
  job-branch change set.
- No settings file: sed outputs nothing, BASE is empty, `${BASE:-main}` falls
  back to `main` (diff then resolves against `main` in a real repo; my scratch
  repo had no `main` branch only because the test renamed it).
- The sed regex is byte-identical to `scripts/new-job.sh` line 93 (the guarded
  single-key extraction), so reviewer and tooling agree. Step 2's
  parenthetical ("The diff in step 5 is only meaningful on the job's branch")
  does not assume `main`, so no change there was needed. Change is minimal:
  one step + snippet + short explanation, as briefed.
- Minor, non-blocking: the snippet omits the scripts' `[[ -f ]]` guard, so a
  missing settings file prints a harmless `sed: can't read ...` stderr line
  before falling back. Functionally equivalent (fallback still correct), and
  the brief notes the file is tracked and always present inside job worktrees.

TASK-2: PASS
notes: Audit claim verified by grep of every `agents/*.md` for `git diff`,
`origin/`, `baseBranch`, `--base-branch`, `main...`, `HEAD`, and any
`git merge|rebase|log|checkout|pull|push|fetch`: only `agents/reviewer.md`
contains base-branch/diff logic (the new fixed step 5); the other eight agents
(analyst, designer, developer, mentor, product-owner, prompter, quality,
security — `mentor.md` does exist) use only branch-agnostic
`git branch --show-current` verification against the `branch:` field, correct
for any base branch. Audit result recorded in implementation.md with the
accurate agent list. TASK-2 changed no files (per its own "files: none"
spec), so its absence of a dedicated commit is expected.

Scope: clean. `git diff main...HEAD` touches only `agents/reviewer.md` and the
four job files (`brief.md`, `tasks.md`, `implementation.md`, the scaffolded
`verdict.md` template). Nothing under `docs/jobs/archive/` or
`docs/jobs/.jdi-status/` was modified. No unrelated refactors. Commit format
is correct (`[ojhwyg] brief: ...`, `[ojhwyg] TASK-1: ...`,
`[ojhwyg] implementation: ...`). Worktree is clean.

## Security

none (no security-relevant code paths changed; single markdown instruction
file edit + audit).

## Overall

APPROVED

No blockers. TASK-1 and TASK-2 both fulfil their briefs; the snippet parses,
resolves `baseBranch` with correct `main` fallback, and matches the tooling's
own extraction pattern. The only nit (missing `[[ -f ]]` guard in the
snippet) is cosmetic and non-blocking.
