## Summary

Built the host-side `mg diff <id>` command that docs/GIT_DIFF.md promises but
no tooling provided: show what an open job's branch changed, three-dot
(`<base>...<branch>`) against the project's configured base branch, with the
doc's "quick eyeball" as the default output. Also installed `tig` in the
Docker image (the doc already recommended it in-session), so the doc's
`tig main...HEAD` now works for humans and agents after a `make rebuild`.

## Changes

TASK-1: Added the three-dot diff + log helpers to `internal/git` (the one
git-exec point) in a new `internal/git/diff.go`: `Diff` (`git diff
<base>...<branch>`), `DiffStat` (`--stat`), `DiffNameOnly` (`--name-only`)
and `LogOneline` (`git log --oneline`), each building the range itself and
following the package's degrade rules (ErrNotARepo, wrapped errors incl. git
stderr, empty string for an undiverged range). Tests in
`internal/git/diff_test.go` (full/stat/name-only/log content, undiverged,
missing-branch, not-a-repo).

TASK-2: Wired the `mg diff <id>` subcommand — dispatch + help entry in
`cmd/mg/main.go`, new `cmd/mg/diff.go`. Resolves the project root
(`job.FindProjectRoot`), the job's branch exact-then-prefix on the id_slug
tail segment (the same wording as done/delete for not-found/ambiguous), and
the base branch via `project.Load` → `baseBranch` → `git.SymbolicRefHead`
fallback (the same chain done/delete use). Output: log + `diff --stat` by
default, `--name-only` for filenames, `--full` for the complete patch, and
`--tig` to spawn `tig <base>...<branch>` interactively on the host with a
clear "tig is not installed" error (mirroring mg host's CLI-missing check via
an injectable `tigLookPath`). Flags parse in any position relative to the id
via the existing `splitFlags` helper.

TASK-3: Added `tig` to the Docker image's system-dependency apt line
(`Dockerfile`) so the doc's in-session `tig main...HEAD` works. Takes effect
after a `make rebuild`.

TASK-4: CLI tests in `cmd/mg/diff_test.go` on the established scratch-repo
pattern (mgCheckout + runJob): no-args usage, unknown argument/flag/positional
wording, unknown job (done/delete's not-found wording + active-branch
listing), ambiguous job, successful default/`--name-only`/`--full` output on
a real job (flags in both positions), `--tig` flag parse + tig-missing error
(via a stubbed `tigLookPath`, after the mg host test pattern), and the
non-repo degrade.

TASK-5: Documentation sync — the three mandated places now agree (README's
"installed commands" table, `mg --help` text in `cmd/mg/main.go`, and the
Commands section of `docs/AGENTS.md`), plus a `docs/GIT_DIFF.md` lead-in to
the new tooling and the two adjacent subcommand enumerations (README "where
everything lives", AGENTS.md dispatcher line). `project-template/docs/AGENTS.md`
and `agents/*.md` needed no change (per-project placeholders / no command-set
claims affected).

## Known issues / follow-ups

- `git log --oneline <base>...<branch>` (three-dot, as docs/GIT_DIFF.md's
  quick-eyeball commands specify literally) shows base-only commits too once
  the base branch has moved on past the merge-base — it is the symmetric
  difference, not two-dot `base..branch`. This matches the doc's exact
  commands; the `--stat`/`--name-only`/`--full` outputs are merge-base-correct
  either way. If the log should show only branch-side commits, `LogOneline`
  would need a two-dot variant (a deliberate deviation from the doc's literal
  command, so left out).
- The `--full=false` / `--name-only=true` `=`-value flag forms are not
  accepted (splitFlags only handles space-separated tokens) — consistent with
  the other flag-based subcommands; no test pins them.
- `tig` in the image only lands after a `make rebuild` (additive apt package,
  no cache-busting).
