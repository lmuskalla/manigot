# Brief: Harden mg jdi on OpenCode profiles and enforce read-only agents under OpenCode

status: done
type: feature
id: 63quv2
branch: feature/63quv2_harden-mg-jdi-on-opencode-profiles-and-enforce-read-only-agents-under-opencode
date: 2026-08-13
author: Leander Muskalla

## What

Two halves of the same "can I trust an unattended run" question — scoped together per the roadmap.

1. **Prove `mg jdi` end-to-end on all three profiles, and fix what breaks.** The JSONL event parsing in `internal/orchestrate/signal.go` and the retry-budget state machine in `internal/orchestrate/orchestrate.go` look well-engineered, but history says the OpenCode path has burned us repeatedly (two scaffolded jobs literally named "jdi is broken" and "opencode jdi issues" were abandoned rather than resolved; plus archived `4i5tcx_jdi-does-not-work`, `foycfl_jdi-for-opencode`, `gezlwy_attempts-in-jdi`, `nrv5sa_multiple-jdi-instances`). Success criterion: a real, non-trivial job driven end-to-end under `zai` and `opencode-go`, with `run.log` and the status sidecars checked for correctness — not just unit tests of the parsers.

2. **Enforce read-only agents under OpenCode.** `@reviewer`, `@security`, `@analyst` and `@owner` are read-only under Claude Code but **not** under OpenCode, because the `tools:` frontmatter key is stripped. Express the restriction as OpenCode `permission:` frontmatter so the reviewer can't edit `implementation.md` or the code it's supposed to be verifying. Keep `agents/*.md` and `project-template/docs/AGENTS.md` (and the Dockerfile / session-launcher strip logic) in sync.

## Why

`mg jdi` is the flagship autonomous feature, and it is the one that fails silently if it's wrong. A false "needs human" stop, a stall, or a reviewer that isn't actually read-only under OpenCode each make the autonomy promise untrustworthy. If this passes cleanly, the worry tax is paid off for good.

## Out of scope

- Extending the `mg jdi` sequence with `@owner`/`@security` (separate job `ru97hg`, scheduled after this lands).
- Orphaned-worktree cleanup (separate job `nepbxu`).
- CODE_QUALITY_TASKS refactoring of the duplicated branch-matching / git-exec code (separate chore job `ui5f6q`), unless a fix here is impossible without it.

## Notes

- The concrete risk from a user's perspective: a false "needs human" stop, a stall, or a reviewer that isn't actually read-only under OpenCode.
- Item 2's fix (OpenCode `permission:` frontmatter) was already identified as a follow-up; this job makes it stop being a follow-up.
- Verification must include real runs under both `zai` and `opencode-go`, checking `run.log` and status sidecars — the abandoned worktrees `o3kk3n_jdi-is-broken` and `a75hdc_opencode-jdi-issues` are the cautionary evidence.

