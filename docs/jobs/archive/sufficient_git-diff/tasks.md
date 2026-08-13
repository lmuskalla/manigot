# Tasks: git diff

id: sufficient
status: open
analyst: @analyst
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Scope assumptions (confirm before implementing)

The brief ("Let's build in the possibility to see what has changed. Look at
docs/GIT_DIFF.md please.") is sparse. docs/GIT_DIFF.md already exists on main
(commit `3a9c006`, "docs: add git diff viewing reference") and is the spec for
*how* a branch's changes should be viewed. This job builds the missing host-side
tooling that makes that viewing possible from the tool itself:

1. **Deliverable is a CLI command `mg diff <id>`**, not documentation
   (GIT_DIFF.md exists) and not a TUI diff view (78fgoq explicitly ruled an
   interactive/scrollable TUI diff viewer out of scope; the brief doesn't reopen
   it). Assumed: host-side CLI only.
2. **Three-dot diff against the base branch** is the one thing that matters per
   GIT_DIFF.md: `git diff <base>...<branch>`. Base branch resolves from
   `.manigot/manigot.json` `baseBranch` (fallback `origin/HEAD` → `main`), the
   same chain mg done/mg delete use. Job `<id>` resolves to its branch
   exact-then-prefix on the `<id>_<slug>` tail segment, same as done/delete.
3. **Default output is the "quick eyeball"** from the doc: `git log --oneline
   <base>...<branch>` + `git diff --stat <base>...<branch>`; a `--full`/`--patch`
   flag prints the complete `git diff <base>...<branch>`. Exact flag set is a
   developer decision within this shape — flag if you want it confirmed.
4. **Open jobs only.** Archived jobs have no branch left to diff (squash-merged
   by mg done), so resolution via branch refs covers exactly the right set.
5. **tig is now in scope (confirmed with the requester).** Not currently in
   the toolchain anywhere, yet docs/GIT_DIFF.md already recommends it
   (`sudo apt install tig`; `tig main...HEAD`) — so today the doc promises a
   tool that isn't in the container. Two confirmed additions: (a) install tig
   in the Docker image so humans/agents can browse diffs in-session per the
   doc; (b) a host-side `mg diff --tig <id>` flag that spawns
   `tig <base>...<branch>` interactively, with a clear error when tig isn't
   installed on the host (mirroring mg host's CLI-missing error). Plain-git
   output stays the default — hosts won't have tig and the command must work
   non-interactively. delta/difftool integration remains out of scope
   (external tools, doc-suggested only).
6. **No agent changes.** The container git shim already allowlists `git diff`/
   `git log`, and `agents/reviewer.md` already does `git diff <base>...HEAD`.

## Task breakdown

TASK-1: Add three-dot diff + log helpers to `internal/git` (the one git-exec
point): e.g. `Diff(root, base, branch)` running `git diff <base>...<branch>`
and a stat/name-only variant plus `LogOneline(root, base, branch)`, each with
the package's degrade rules (ErrNotARepo, wrapped errors incl. git stderr).
     files: internal/git/git.go (or new internal/git/diff.go),
            internal/git/git_test.go (or diff_test.go)
     depends: none
     risk: low — pure additive helpers mirroring the existing run/wrapErr pattern.

TASK-2: Wire the `mg diff <id>` subcommand: dispatch + help entry in main.go,
new `cmd/mg/diff.go` resolving project root (job.FindProjectRoot), job→branch
(exact then prefix via git.ExactBranchMatch/PrefixBranchMatches, same wording as
done/delete for not-found/ambiguous), base branch (project.Load
→ BaseBranchValue, fallback git.SymbolicRefHead), then printing the
quick-eyeball output (TASK-1) by default, `--full` for the complete patch, and
`--tig` to spawn `tig <base>...<branch>` on the host (clear "tig not found"
error when missing, after the mg host pattern).
     files: cmd/mg/main.go, cmd/mg/diff.go (new)
     depends: TASK-1
     risk: medium — new command surface (dispatch, flag parsing, error wording,
           exec lookup for tig) but follows the established runDone/runDelete
           and mg host command patterns.

TASK-3: Install tig in the Docker image: add `tig` to the system-dependencies
apt-get install line in the Dockerfile, so the doc's in-session `tig
main...HEAD` actually works for humans and agents (note: takes effect after a
`make rebuild`).
     files: Dockerfile
     depends: none
     risk: low — one additive apt package; the only subtlety is the image
           rebuild needed for it to land.

TASK-4: CLI tests for `mg diff`: no-args usage, unknown job, ambiguous job,
successful stat/name-only/full output on a scratch repo (mgCheckout +
runJob pattern from lifecycle_test.go), `--tig` flag parse + tig-missing error
(override PATH), non-repo degrade.
     files: cmd/mg/diff_test.go (new)
     depends: TASK-2
     risk: low — same real-git scratch-repo style as the existing lifecycle tests.

TASK-5: Documentation sync: README "installed commands" table, `mg --help`
text (printHelp in cmd/mg/main.go), the Commands section of docs/AGENTS.md —
the three must stay in sync (hard rule) — plus a GIT_DIFF.md touch pointing
readers at `mg diff <id>` / `mg diff --tig <id>` now that the tooling exists.
     files: README.md, cmd/mg/main.go, docs/AGENTS.md, docs/GIT_DIFF.md
     depends: TASK-2, TASK-3
     risk: low — doc/help-text additions only, no behavior change.
