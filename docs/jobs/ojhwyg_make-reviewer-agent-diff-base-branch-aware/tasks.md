# Tasks: make reviewer agent diff base-branch aware

id: ojhwyg
status: open
analyst: @analyst
date: 2026-08-12

<!-- Produced by @analyst from brief.md. -->

## Scope summary

`agents/reviewer.md` line 17 hardcodes `git diff main...HEAD`. For any
project whose base branch is not `main`, this diffs against the wrong ref.
Fix: resolve the base branch the same way the scripts do — read `baseBranch`
from `.manigot/manigot.json` (guarded single-key `sed`), fall back to `main`
when absent — and use `git diff <base>...HEAD`. Audit the other agents for
similar hardcoded base-branch assumptions; fix any found (audit says only
reviewer.md has one).

The file is inside job worktrees (tracked), so the reviewer can read it
directly. The reviewer runs in the job's own worktree on the job branch.

## Task breakdown

TASK-1: Fix `agents/reviewer.md` step 5: replace the hardcoded
`git diff main...HEAD` with a base-branch resolution + `git diff <base>...HEAD`.
Suggested wording: "5. Determine the base branch: read `baseBranch` from
`.manigot/manigot.json` (default `main` when absent), then run
`git diff <base>...HEAD` to see every actual change made on this branch".
Give a concrete shell snippet the reviewer can run, e.g.:
`BASE=$(sed -n 's/^.*"baseBranch"[[:space:]]*:[[:space:]]*"\([^"]*\)".*$/\1/p' .manigot/manigot.json | head -n1); git diff "${BASE:-main}...HEAD"`
Also update step 2's parenthetical if it references "the diff in step 5" in
a way that assumes main, and adjust the wording of step 5 in the numbered
list so the base-branch resolution is explicit. Keep it minimal — one step
plus a short explanation.
files: agents/reviewer.md
depends: none
risk: low — single-file wording change; no tooling behavior.

TASK-2: Audit the remaining agents for hardcoded base branches: grep all
`agents/*.md` for `git diff`, `main`, `origin/`, `baseBranch`,
`--base-branch`, `HEAD` in git commands. Verify the audit result: only
reviewer.md has a hardcoded base branch; the other agents (analyst,
designer, developer, mentor, product-owner, prompter, quality, security)
only use branch-agnostic `git branch --show-current` verification. Record
the audit result in implementation.md. No changes expected.
files: none (audit only, findings recorded in implementation.md)
depends: none
risk: low — read-only grep; report-only.

## Suggested order
TASK-1 → TASK-2.

## Notes for the developer
- Do not touch archived verdicts/run logs that mention `git diff main...HEAD`
  (docs/jobs/archive/*, docs/jobs/.jdi-status/*) — records, not instructions.
- The `sed` pattern must match the one the scripts use (see
  scripts/new-job.sh's baseBranch read) so reviewer and tooling agree.
- Verify the snippet parses and runs correctly (bash -n / run it in a
  scratch repo with baseBranch: development and confirm it diffs against
  development).
