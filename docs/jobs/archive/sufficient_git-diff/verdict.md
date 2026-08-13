# Verdict: git diff

id: sufficient
status: open
reviewer: @reviewer
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed via `git diff main...HEAD` on branch `feature/sufficient_git-diff`
(13 files changed: Dockerfile, README.md, cmd/mg/{main,diff,diff_test}.go,
docs/{AGENTS,GIT_DIFF}.md, internal/git/{diff,diff_test}.go, plus the job's
own four files). Cross-referenced against tasks.md and docs/GIT_DIFF.md.

TASK-1: PASS
notes: internal/git/diff.go — Diff/DiffStat/DiffNameOnly/LogOneline all build
       the three-dot range `<base>...<branch>` themselves and share the
       package's degrade rules: ErrNotARepo via notARepo() (git-missing and
       not-a-repo, case-insensitive), wrapped errors incl. git stderr via
       wrapErr(), and empty-string-with-nil for an undiverged range (git exits
       0 with no output). TrimRight("\n") on stdout is safe (only strips
       trailing newlines; --stat's leading-space alignment is preserved).
       No naming collisions with existing package functions. Tests
       (internal/git/diff_test.go) use the existing initRepo/runGit/writeFile/
       commitAll helpers and cover content, undiverged, missing-branch
       (wrapped, not ErrNotARepo) and not-a-repo for all four helpers.

TASK-2: PASS
notes: cmd/mg/diff.go + main.go dispatch ("diff" case) + printHelp entry.
       Resolution chain verified identical to mg done/mg delete:
       FindProjectRoot → git.LocalBranches → resolveJobBranch (exact match on
       the id_slug tail segment via BranchTail, then unique prefix; not-found
       and ambiguous wording byte-identical to job.jobNotFoundError /
       FinishJob's ambiguity error) → project.Load → settings.BaseBranch →
       git.SymbolicRefHead fallback (which itself falls back to "main" —
       exactly finish.go's chain). Base = "main" resolves correctly in the
       scratch repos (no settings file, no origin/HEAD). Output: log + stat by
       default (the doc's quick eyeball), --name-only, --full, --tig (spawns
       `tig <base>...<branch>` with stdio wired through; clear "tig is not
       installed on the host" error via injectable tigLookPath, mirroring
       internal/session's hostLookPath). Flags parse in any position via
       splitFlags; unknown flags/positionals/--help get the single-line
       "Unknown argument:" wording; usage on no-args. Empty-range handling
       prints "No changes on <branch> relative to <base>." with exit 0.

TASK-3: PASS
notes: Dockerfile adds `tig` to the first apt-get install line (system
       dependencies). One additive package, correct layer. Takes effect after
       `make rebuild`, as documented.

TASK-4: PASS
notes: cmd/mg/diff_test.go — no-args usage; exact single-line stderr pinned
       for --help/--bogus/extra-positional; unknown job with done/delete's
       not-found wording + active-branch listing; ambiguous job (exact match
       on the two-prefix case); successful default/--name-only/--full output
       on a real job (runJob + mgJobBranch + mgWorktreePath helpers, flags in
       both positions); --tig parse in both positions + tig-missing error via
       a stubbed tigLookPath (t.Cleanup restored); non-repo degrade. All
       helpers used (mgCheckout/runJob/mgGit) exist with matching signatures
       in lifecycle_test.go; local helpers mgJobBranch/mgWorktreePath are
       defined in the test file. Traced each assertion against actual git
       output shapes — consistent.

TASK-5: PASS
notes: The three mandated places agree on the command and its flag set:
       README "installed commands" table, mg --help (printHelp), docs/AGENTS.md
       Commands section; both adjacent subcommand enumerations (README "where
       everything lives", docs/AGENTS.md dispatcher line) list `diff`.
       docs/GIT_DIFF.md gained the lead-in pointing at mg diff/--name-only/
       --full/--tig. Verified project-template/docs/AGENTS.md is a per-project
       placeholder (no mg command-set claims — no change needed) and
       agents/*.md already allow git diff/log (reviewer.md does
       `git diff <base>...HEAD`) — the "no agent changes" scope assumption
       holds.

## Security

None. All additions are read-only git queries (diff/log) run by the host-side
CLI, a TUI spawn of an interactive host process, and one additive apt package
(tig) in the container image. No new filesystem writes, no new credential
handling, no input that reaches a shell. The `git -C root` exec path is the
existing, allowlisted internal/git surface.

## Overall

APPROVED

No blockers. Non-blocking observations (noted for awareness, nothing required):

- runDiff's `flag.ErrHelp` branch is unreachable in practice (splitFlags never
  routes `--help` into flagArgs; the "-"-prefixed rest check catches it first)
  — harmless defensive code mirroring runJob.
- Cosmetic: a range with log-only content (base branch moved ahead, branch
  equal to the merge-base) prints a trailing blank line in the default view —
  a direct consequence of the doc's literal three-dot commands, and the
  implementation.md "Known issues" note already flags the underlying
  symmetric-difference behavior.
- docs/GIT_DIFF.md's "--name-only prints just the filenames" is slightly
  loose — the commit log is printed as well (the CLI's own usage/help text
  documents this).
