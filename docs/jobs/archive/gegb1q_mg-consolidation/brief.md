# Brief: mg consolidation

status: done
type: feature
id: gegb1q
branch: feature/gegb1q_mg-consolidation
date: 2026-08-12
author: Leander Muskalla

## What

Rewrite the host-side architecture of manigot into its end state: **one Go
binary, `mg`, that is the entire host-side tool**, with bash reduced to
exactly one file — `scripts/entrypoint.sh`, which stays bash because it runs
inside the container image and is part of the agent environment, not the
orchestrator.

Today the host side is split across 12 bash scripts (~2,600 lines) and a Go
module (`tui/`, ~8,200 non-test lines, ~8,600 test lines). Every piece of
orchestration logic exists in both languages and must be kept in sync by
hand (`find_project_root` is copy-pasted into 4 scripts; the profile table
lives in 3 places; key-var lists are duplicated run.sh ↔ entrypoint.sh).
Worse, the split forces an entire indirection machinery that exists only
because of it: `tui/internal/resolve` (locating scripts/binaries), the
`hostcmd` package (shelling out to `mg job`/`mg done`/`mg delete` with `PWD=`
env hacks), three separate artifacts (`mg.sh`, `bin/manigot-tui`,
`bin/manigot-jdi`) and two wrapper scripts (`tui.sh`, `jdi.sh`).

This job is a **strangler migration, not a rewrite from scratch**: the Go
side already implements most of the same operations (`git` package does
worktree/branch ops, `job` does discovery + stages, `config` owns the profile
table + `.env`, `orchestrate` is the jdi state machine). The bash logic moves
into Go, untyped-and-untested becomes typed-and-tested, and the duplication
dies with the duplication it policed.

### Target end state

```
go.mod                               # moved to repo root, module github.com/lmuskalla/manigot
cmd/mg/main.go                       # single binary, stdlib subcommand dispatch:
                                     #   session (bare `mg`), tui, jdi, job, done, delete,
                                     #   profiles, setup, agents, init, help
internal/config                      # profiles table, settings, .env read/write  (existing)
internal/git                         # + worktree create/remove, squash merge, branch -D  (extend)
internal/job                         # + CreateJob / FinishJob / DeleteJob lifecycle  (extend)
internal/session                     # docker launch construction: mounts, env shadowing,
                                     #   profile/auth resolution, prompt assembly  (from run.sh)
internal/orchestrate                 # jdi state machine      (existing)
internal/ui                          # Bubble Tea TUI, reachable as `mg tui`      (existing)
internal/{agentlist,editor,markdown,project}   # existing, unchanged
scripts/entrypoint.sh                # the ONLY remaining bash — container-side, stays as-is
Dockerfile, Makefile                 # mechanically updated (see Notes)
```

**Dies completely:** `scripts/mg.sh`, `run.sh`, `new-job.sh`, `finish-job.sh`,
`delete-job.sh`, `profiles.sh`, `setup.sh`, `agents.sh`, `init.sh`, `tui.sh`,
`jdi.sh`, `scripts/lib/`, `tui/internal/resolve`, `tui/internal/hostcmd`,
`bin/manigot-tui`, `bin/manigot-jdi`. The TUI and jdi become subcommands of
the one binary, so the TUI calls `mg job`/`mg done`/`mg delete` as direct
function calls, not subprocesses.

### Migration phases (each phase leaves the tool usable and `go test ./...` green)

- **Phase 1 — Restructure + binary skeleton.** Move `go.mod` to the repo
  root, rename the module, relocate the existing packages, build `bin/mg`
  whose subcommands still *delegate to the bash scripts* unchanged
  (strangler stage 0 — behavior identical, `mg` becomes the single entry
  point). Update Makefile and the Dockerfile's Go module-cache prewarm path.
- **Phase 2 — Port `mg profiles`, `mg setup`, `mg agents`, `mg init`** to Go
  subcommands; delete those four scripts.
- **Phase 3 — Port session launch** (was `run.sh`) into `internal/session`:
  profile resolution, auth validation, project-root + worktree resolution,
  docker argv construction, mounts, `.env` shadowing, git identity, quotes,
  `--print` mode. The TUI and jdi call it directly.
- **Phase 4 — Port job lifecycle** (was `new-job.sh` / `finish-job.sh` /
  `delete-job.sh`) into Go: worktree create + scaffold + first commit;
  clean-tree check, archive move/commit, squash merge, branch delete,
  worktree remove.
- **Phase 5 — Removal.** Delete all remaining scripts and the dead `resolve`/
  `hostcmd` packages, wire `mg tui` / `mg jdi`, final Makefile/README/
  `docs/AGENTS.md` sync, CI (go vet, go test, shellcheck on entrypoint.sh).

## Why

This is a 4-day-old project with zero users. The foundation chosen now must
hold for the next 500 iterations of the tool. Under that lens the current
shell+Go split is the wrong foundation: every future feature (new profiles,
new workflow stages, new automation, more UI) pays a permanent Go+bash tax —
two implementations of each piece of logic, hand-kept in sync. The sync
tests a "minimal fix" would add only *detect* the drift; they don't eliminate
it. Consolidating into one Go binary eliminates it: every piece of host-side
logic exists once, typed, covered by the existing test suite, and the whole
`resolve`/`hostcmd`/wrapper machinery — which exists *only* because of the
language split — evaporates.

Go, not TypeScript, was chosen deliberately: the consolidation target already
exists in Go with ~8,600 lines of tests, Go ships a static binary with zero
host runtime deps (a TS tool would need node or platform-specific bundles on
every host), and Bubble Tea is the strongest TUI ecosystem in any language.
The tools being wrapped (Claude Code, OpenCode) are TS, but wrapping TS tools
is not a reason to become one. Bash is kept exactly where it is genuinely the
right tool: the container-side entrypoint, part of the isolated agent
environment.

The one stable seam in the whole system is the boundary between the
orchestrator (host-side Go) and the agent environment (Docker image +
entrypoint.sh). This job makes that seam the *only* seam.

## Out of scope

- **Any TypeScript.** Decided against: existing tested Go base, static
  binary, best TUI ecosystem. Not revisited in this job.
- **Changing the container isolation model** or Dockerfile semantics beyond
  the mechanical updates in Notes (go.mod prewarm path, entrypoint COPY).
  No podman/remote/devcontainer support.
- **Changing config formats**: `.env`, `config/tui-settings.json`,
  `.manigot/manigot.json` stay exactly as they are.
- **Changing agent markdown definitions** (`agents/`, `docs/agents/`).
- **Changing the TUI's design or the orchestrate state machine logic** — they
  move as-is, behavior preserved.
- **New features.** This is a pure architecture refactor; user-visible
  behavior must be preserved 1:1.
- **Removing legacy `--tool` support** (see Notes) — it moves to Go unchanged.
- **Windows support, plugin systems, web dashboards, multi-machine.**

## Notes

Load-bearing behaviors that must be ported **exactly** — these are the trap
list the implementer must not "improve" away:

1. **Bare `mg` must work with no `docs/`** (plain session fallback to git
   root, else `$PWD`), with `docs/` (initialized project), and in `--print`
   mode. The `--print` stdout contract: the agent's own output (JSON) on
   stdout, *everything else* on stderr — in Go this is diagnostics to stderr,
   no fd-3 juggling. Existing `orchestrate`/`output` tests already parse the
   JSON; keep them green.
2. **Profile resolution precedence**: `--profile` > `--tool` (legacy alias:
   `claude-code`→claude-pro, `opencode`→legacy mode) > `$MANIGOT_PROFILE` >
   claude-pro. Legacy profile-less `--tool opencode` semantics preserved,
   including its rejection of `--print`. The profile table (ID, label, tool,
   auth key, model default) moves into `internal/config` as the single source
   of truth — it already lives there for the TUI.
3. **`--job` resolution**: exact-then-prefix branch-name match, worktree
   lookup, **hard error on branch-without-worktree** (never silently fall
   back to `PROJECT_ROOT`), and the no-branches fallback to a flat
   `docs/jobs/` directory scan. This logic already exists in Go
   (`job.Discover`, `git.WorktreeForBranch`, `job.discoverWorkingTree`) —
   unify on it, do not duplicate.
4. **Worktree layout** must keep the exact semantics of `new-job.sh`:
   sibling `<dirname(root)>/.manigot-worktrees/<basename(root)>/<id>_<slug>`
   when the parent is on the same filesystem; nested
   `<root>/.manigot-worktrees/<id>_<slug>` + `.git/info/exclude` when `root`
   is itself a mount point. Plus the git-common-dir mount needed for a
   worktree's `.git` pointer file to resolve inside the container.
5. **Docker launch flags**, preserved verbatim: `-it` only when stdin is a
   terminal (`golang.org/x/term` — already a dependency), `--rm`,
   `--user $(id -u):$(id -g)`, `--network=bridge`, `--memory=2g`,
   `--security-opt=no-new-privileges`, the container name
   `manigot-<basename(root)>-<pid>`, all `-e` vars (OAuth token, account
   UUIDs, git identity, `MANIGOT_TOOL`, `MANIGOT_PRINT`, `MANIGOT_QUOTE`,
   provider keys), and all mounts (project root, `docs/`, context file
   read-only, git-common-dir for job worktrees, `/dev/null` shadowing of
   every project `.env`/`.env.*` except `*.example`/`*.sample`). Wire
   `os.Stdin/Stdout/Stderr` through; Ctrl+C must reach the container and `mg`
   must exit with the agent's exit code.
6. **`mg done`/`mg delete` semantics**: clean-tree check, archive move +
   commit inside the job's worktree, squash merge onto the configured
   `baseBranch` (fallback `origin/HEAD` → `main`), branch delete, worktree
   remove — skipped when the job's branch is checked out in the main worktree
   (pre-worktree jobs), which is also switched back to the base branch.
   Non-git project = plain directory delete. Interactive confirmations
   (`read -rp` today) become Go prompts with identical wording.
7. **`mg init`**: works without an existing `docs/` (the only command that
   does), copies `project-template/docs/` (AGENTS.md, CLAUDE.md, empty
   `docs/jobs/` — never the example job) + `.manigot/manigot.json`, reports
   "already initialized" when `docs/` exists, optional `@prompter` hand-off
   via `--prompt` (which needs the Phase-3 session launcher's prompt
   plumbing).
8. **`mg jdi`**: sidecar `.manigot/jdi-status/<job-name>/` (`status` +
   `run.log`) in the *target* project, idempotent `.git/info/exclude`
   ensuring, bell on stop, detached (TUI-launched) vs streaming (CLI)
   modes. It must call the Go session launcher's `--print` path directly
   instead of `run.sh`.
9. **`mg setup`**: auto-reads the Claude account from `~/.claude.json`,
   prompt/paste flow, `--check` non-interactive report. Same wizard behavior,
   Go implementation (the TUI settings screen already does Go forms).
10. **Quotes + git identity**: `assets/quotes.json` random pick (skip in
    `--print` mode); git author name/email resolution order — host
    `GIT_AUTHOR_NAME/EMAIL` env vars, then the project's git config, passed
    into the container as `GIT_AUTHOR_NAME_CFG`/`GIT_AUTHOR_EMAIL_CFG`
    (entrypoint writes gitconfig from them).
11. **`scripts/entrypoint.sh` stays bash and self-contained** — nothing in the
    image may depend on Go. Its internal key list is a container-side safety
    net only; Go pre-validates keys before launch, so drift between them is
    harmless. The Dockerfile's `COPY` of entrypoint stays; only the Go
    module-cache prewarm path (currently `tui/go.mod` → `/tmp/tui/`) must be
    updated to the new root go.mod, or the image build breaks
    (`GOTOOLCHAIN=local` means no network fallback).
12. **Install story**: `make build` → `bin/mg`; `make install` → **one**
    symlink; `make tui`/`make jdi`/`make run` targets collapse. `docs/AGENTS.md`
    itself documents the old architecture and must be rewritten in Phase 5
    (it is the canonical project context — agents read it at session start).
    After Phase 5 nothing anywhere may reference a removed script or binary.
13. **Testing**: the existing Go suite must stay green throughout. Add tests
    for the new Go code at least where the bash had none: session argv/mount
    construction, the lifecycle (create→done→delete roundtrip incl. worktree
    layout and the mount-point nesting case), and the prompt/confirmation
    flows. Shellcheck scope shrinks to `entrypoint.sh`.

**Acceptance criteria (end of Phase 5):** one `bin/mg` implementing every
command today's scripts and binaries provide; zero bash in `scripts/`
except `entrypoint.sh`; no `resolve`/`hostcmd` packages; `go test ./...`
green including the new tests; manual smoke of: session launch with and
without `docs/`, `--print` output cleanliness, a full
`mg job` → work → `mg done` roundtrip (worktree layout + squash merge +
branch delete), `mg delete`, `mg profiles`, `mg setup --check`, `mg init` on
a scratch directory, the TUI (list, detail, launch, settings, jdi badge),
and a `mg jdi` end-to-end run; README and `docs/AGENTS.md` accurate against
the new architecture.
