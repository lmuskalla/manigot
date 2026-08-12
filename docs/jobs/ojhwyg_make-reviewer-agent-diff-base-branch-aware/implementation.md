# Implementation: make reviewer agent diff base-branch aware

id: ojhwyg
status: open
developer: @developer
date: 2026-08-12

## Summary

`agents/reviewer.md` instructed the reviewer to run `git diff main...HEAD` to
see a job's changes. For any project whose base branch is not `main`, this
diffs against the wrong ref — the review surface is wrong or empty. The
reviewer now resolves the base branch the same way the tooling does: it reads
`baseBranch` from `.manigot/manigot.json` (the same guarded single-key `sed`
extraction `scripts/new-job.sh`/`finish-job.sh`/`delete-job.sh` use), falling
back to `main` when the key is absent, and diffs with `git diff <base>...HEAD`.

Also audited all other agents for hardcoded base-branch / `main` assumptions
in git commands — none found; only reviewer.md had one, now fixed.

## Changes

TASK-1: `agents/reviewer.md` — step 5 of "How to start" no longer hardcodes
`git diff main...HEAD`. It now reads the base branch from
`.manigot/manigot.json` (fallback `main`) and diffs against it, with a
concrete shell snippet the reviewer can run:
```
BASE=$(sed -n 's/^.*"baseBranch"[[:space:]]*:[[:space:]]*"\([^"]*\)".*$/\1/p' .manigot/manigot.json | head -n1)
git diff "${BASE:-main}...HEAD"
```
Verified live in scratch repos: with `baseBranch: development` the diff is
computed against `development`; with no settings file it falls back to `main`
— both produce the expected per-branch change sets.

TASK-2: audit of `agents/*.md` for hardcoded base branches — grep for
`git diff`, `origin/`, `baseBranch`, `--base-branch`, `main...`, `HEAD` in
git commands. Result: only `reviewer.md` contained one (now fixed). The other
eight agents (analyst, designer, developer, mentor, product-owner, prompter,
quality, security) use only branch-agnostic verification — `git branch
--show-current` compared against the `branch:` field in `brief.md` — which is
correct for any branch name, prefixed or not. No changes needed.

## Known issues / follow-ups

- Historical archived verdicts and mg-jdi run logs under `docs/jobs/archive/`
  and `docs/jobs/.jdi-status/` mention `git diff main...HEAD`; these are
  records of past reviews, not instructions, and were deliberately left
  untouched.
- The reviewer's `sed` extraction duplicates the pattern used in the bash
  scripts rather than sharing it (bash scripts can't be sourced by the
  reviewer agent's shell). The pattern is stable and documented in
  `scripts/new-job.sh`; a third project key would justify centralizing via
  `jq` on the script side, but the agent copy stays a literal instruction.
