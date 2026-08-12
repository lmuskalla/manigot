# Tasks: mg consolidation

id: gegb1q
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

Strangler migration, five phases, each leaving `go test ./...` green and the
tool usable. The phases are the brief's own; the tasks below decompose them
into atomic steps grounded in the current codebase. Baseline verified:
`go build ./...` and `go test ./...` are green today in `tui/` (13 packages,
35 test files). Everything below assumes the module relocation of TASK-1
first, since the whole tree moves under `internal/` and `cmd/`.

Cross-cutting constraints (brief "Notes", all load-bearing — do not
"improve" away):

- Bare `mg` works with no `docs/`, with `docs/`, and in `--print` mode;
  `--print` stdout carries ONLY the agent's JSON output, diagnostics to
  stderr.
- Profile precedence: `--profile` > `--tool` (legacy map) > `$MANIGOT_PROFILE`
  > claude-pro; legacy profile-less `--tool opencode` semantics preserved
  including its rejection of `--print`.
- `--job` resolution: exact→prefix branch match, worktree lookup, HARD error
  on branch-without-worktree, no-branches fallback to the flat `docs/jobs/`
  scan — unify on existing Go code (`job.Discover`, `git.WorktreeForBranch`),
  never duplicate it.
- Worktree layout semantics identical to `new-job.sh` (sibling vs. nested +
  `.git/info/exclude` when root is a mount point) + git-common-dir mount.
- Docker flags preserved verbatim (note 5); interactive confirmations become
  Go prompts with IDENTICAL wording (note 6).
- `scripts/entrypoint.sh` stays bash and self-contained; the Dockerfile's Go
  module-cache prewarm path must point at the new root go.mod or the image
  build breaks (`GOTOOLCHAIN=local`, note 11).
- Config formats (`.env`, `config/tui-settings.json`, `.manigot/manigot.json`)
  unchanged; TUI design and `orchestrate` state machine move as-is.

---

## Phase 1 — Restructure + binary skeleton

### TASK-1: Relocate the Go module to the repo root and rename it

Move `tui/go.mod` + `tui/go.sum` to the repo root, change the module line to
`github.com/lmuskalla/manigot`, and relocate the packages to the target
layout: `tui/internal/*` → `internal/*`, `tui/cmd/jdi` → `cmd/jdi`, and
`tui/main.go` → `cmd/tui/main.go` (kept as its own buildable entrypoint until
Phase 5 wires it in-process). Update every import from
`github.com/lmuskalla/manigot/tui/...` to `github.com/lmuskalla/manigot/...`
(53 references across ~50 files, including all test files). Result must be a
working root module: `go build ./...` and `go test ./...` green from the root.

files: `tui/go.mod`, `tui/go.sum`, every `*.go` under `tui/` (imports),
  directory moves `tui/internal/*` → `internal/*`, `tui/cmd/jdi` → `cmd/jdi`,
  `tui/main.go` → `cmd/tui/main.go`
depends: none
risk: high — touches every Go file; a missed import breaks the whole build.
  Mitigate with a mechanical replace (e.g. `sed`/`gofmt`) then `go build ./...`.

### TASK-2: New `cmd/mg/main.go` subcommand dispatcher (strangler stage 0)

Create `bin/mg` as the single entry point, dispatching exactly like
`scripts/mg.sh`: `help`/`-h`/`--help` prints the same usage text and exits
before anything else; `profiles`, `setup`, `agents`/`crew`, `job`, `tui`,
`jdi`/`made-man`, `done`, `delete`, `init` each exec their still-present bash
script unchanged with args passed through; anything else (bare `mg`, or
`--agent`/`--job`/`--prompt`/`--tool`/`--profile`/`--print` + passthrough)
falls through to `run.sh` — behavior identical to today. `tui` and `jdi` may
delegate to the still-built `bin/manigot-tui` / `bin/manigot-jdi` binaries
(Phase 5 wires them in-process). Version via `-ldflags "-X main.version=…"`.
Update `make install` to symlink `bin/mg` (one link, not `scripts/mg.sh`),
and keep the `tui`/`jdi` make targets building from their relocated paths.

files: `cmd/mg/main.go` (new), `Makefile`, `scripts/mg.sh` (reference only —
  dies in Phase 5)
depends: TASK-1
risk: medium — the dispatcher contract (mg.sh's exact dispatch, including the
  `crew`/`made-man` aliases and the help text) must be preserved 1:1; an
  argparse deviation silently changes user-visible behavior.

### TASK-3: Fix the Dockerfile's Go module-cache prewarm path

Change the prewarm `COPY` from `tui/go.mod tui/go.sum /tmp/tui/` to the new
root `go.mod go.sum` (note 11: with `GOTOOLCHAIN=local` a stale path breaks
the image build). Verify `docker build` succeeds if docker is available in
the environment; otherwise flag for the manual smoke.

files: `Dockerfile`
depends: TASK-1
risk: low — a two-line mechanical change, but a forgotten update breaks the
  entire image build (GOTOOLCHAIN=local means no network fallback).

---

## Phase 2 — Port `mg profiles`, `mg setup`, `mg agents`, `mg init`

### TASK-4: Shared CLI plumbing: `.env` read/write helpers + interactive prompts

Two shared pieces every ported command needs, both in Go with tests:
(a) generalize `config`'s unexported `readEnvProfile`/`writeEnvProfile` into
public `GetEnv(key)` / `UpsertEnv(key, value)` helpers over `config.EnvFile()`
(preserving all other lines, tolerating `export ` prefix and quotes — the
profiles.sh/setup.sh awk upsert behavior); (b) a small prompt package (e.g.
`internal/cli`) with `Confirm(prompt string, r io.Reader, w io.Writer) (bool, error)`,
`PromptSecret(label, current string, …)`, `PromptValue(label, current, def string, …)`,
`Select` for numbered menus, and TTY detection (`golang.org/x/term`, already a
dependency), so `read -rp` flows become testable Go prompts with identical
wording. Non-TTY handling must match the scripts (e.g. setup refuses
interactive, agents refuses to pick).

files: `internal/config/config.go`, `internal/cli/*` (new), plus their tests
depends: TASK-1
risk: low — new code, no existing behavior changes; the wording contract is
  captured by tests that assert the exact prompt strings.

### TASK-5: Port `mg profiles`; delete `scripts/profiles.sh`

Go subcommand with the same three modes: `mg profiles <name>` sets the
default (validated, upserted as `MANIGOT_PROFILE` via TASK-4's helper,
followed by the same "Default profile set …" confirmation + missing-creds
warning), bare `mg profiles` lists the table (profile, label, tool, model,
creds ready/missing — claude-pro checks all four keys, others their auth
key) with the active default marked `*`, and an interactive numbered select
on a TTY (same `[1-3, Enter keeps, q quits]` loop). Profile table comes from
`config.Profiles()` — the single source of truth. Same help text.

files: `cmd/mg/main.go`, `internal/config/config.go`, `internal/cli/*`,
  `scripts/profiles.sh` (deleted)
depends: TASK-4
risk: low — pure port of a small script onto existing Go data.

### TASK-6: Port `mg setup`; delete `scripts/setup.sh`

Go subcommand with the same wizard: per-profile credential prompts
(`claude-pro`: token + account UUID/email/org UUID, auto-extracted from
`~/.claude.json` via Go JSON parsing — replaces the python3 heredoc;
`zai`/`opencode-go`: API key + optional model default), the mask/keep-on-enter
prompt semantics, `--check` non-interactive report (`✓ ready` / `✗ missing …`
per profile, single-profile filter support), "interactive setup needs a
terminal" refusal when not a TTY, and identical wording throughout. Values
written via TASK-4's `UpsertEnv`.

files: `cmd/mg/main.go`, `internal/config/config.go`, `internal/cli/*`,
  `internal/setup/*` (new, if split beyond config), `scripts/setup.sh` (deleted)
depends: TASK-4
risk: medium — the wizard has the most bespoke interactive behavior of the
  four Phase-2 scripts (masking, defaults, auto-extraction); every prompt
  string must match the script's output for the 1:1 behavior requirement.

### TASK-7: Port `mg agents` (+ `mg crew` alias); delete `scripts/agents.sh`

Go subcommand listing agents via `agentlist.Discover` (already mirrors
agents.sh's global→override→project-only ordering and description
extraction), the same numbered menu + "(project override)"/"(project)" tags,
non-TTY refusal, then launches the session. Because Phase 3 hasn't landed
yet, the launch must reproduce `exec run.sh --agent <name> "$@"` — the
simplest future-proof mechanism is re-exec'ing the same binary (`os.Executable()`)
with the passthrough args (`mg --agent <name> [--profile …] …`), which keeps
working unchanged once the session subcommand goes in-process in Phase 3.

files: `cmd/mg/main.go`, `internal/agentlist/agentlist.go` (unchanged), new
  `internal/agents` command logic or inline in `cmd/mg`, `scripts/agents.sh`
  (deleted)
depends: TASK-4
risk: low — listing already exists in Go; the only new surface is the menu
  prompt and the launch mechanism.

### TASK-8: Port `mg init`; delete `scripts/init.sh`

Go subcommand: resolves the target dir (git top-level, else `$PWD` — NOT the
docs-walk-up, since init creates docs/), validates the template files exist,
copies `project-template/docs/` (AGENTS.md + CLAUDE.md + empty `docs/jobs/`,
NEVER the example job under it) plus `.manigot/manigot.json`, reports
"already initialized" and skips when `docs/` exists, accepts
`--profile`/`--tool` (legacy map, claude-code→claude-pro, opencode→zai),
offers the @prompter hand-off on a TTY only (non-TTY → skip with the note),
and on "yes" launches the prompter via re-exec (`mg --agent prompter --prompt
"<INSTRUCTION>" [--profile …]`, exact instruction text preserved) — which
delegates to run.sh until Phase 3. Same output wording.

files: `cmd/mg/main.go`, `internal/cli/*`, `project-template/` (read-only
  reference), `scripts/init.sh` (deleted)
depends: TASK-4
risk: low — mostly file copying plus one conditional launch; template path
  resolution from the binary's checkout must stay correct.

---

## Phase 3 — Port session launch into `internal/session`

### TASK-9: `internal/session` — profile/tool resolution + auth validation

Port run.sh's resolution block verbatim in behavior: `--profile` >
`--tool` (claude-code→claude-pro, opencode→legacy empty-profile mode) >
`$MANIGOT_PROFILE` (validated) > claude-pro; validation errors with the
same messages; the profile→tool/key-var/model mapping (claude-pro, zai with
ZHIPU_API_KEY + `OPENCODE_ZAI_MODEL`, opencode-go with OPENCODE_API_KEY +
`OPENCODE_GO_MODEL`, legacy opencode forwarding the nine-key list and
`OPENCODE_MODEL` as-is); the `--print` + legacy-opencode rejection. Auth
checks: claude-pro requires `CLAUDE_CODE_OAUTH_TOKEN` and refuses a set
`ANTHROPIC_API_KEY` (subscription protection); opencode profiles require at
least one of their key vars. Profile table from `internal/config` (single
source of truth — delete the run.sh table). Tests for precedence, legacy
mapping, and every error path.

files: `internal/session/session.go` + `session_test.go` (new), `internal/config/config.go`
depends: TASK-1
risk: medium — the precedence and legacy-`--tool` semantics are the trap
  list's note 2; tests must pin each branch including the legacy `--print`
  rejection.

### TASK-10: `internal/session` — project root + `--job` worktree resolution

Port run.sh's root resolution: docs-walk-up (reuse `job.FindProjectRoot`),
fallback to git root then `$PWD`, `DOCS_INITIALIZED` flag, trailing-slash
trim, `INVOCATION_ROOT` capture. Port `--job` resolution: requires an
initialized project; no-branches fallback to the flat `docs/jobs/` scan
(exact then prefix, exclude `archive/`); otherwise exact→prefix branch match
on the `<id>_<slug>` tail with ambiguity error, worktree lookup via
`git.WorktreeForBranch`, HARD error on branch-without-worktree (identical
wording), `PROJECT_ROOT` reassignment, and `GIT_COMMON_DIR` resolution
(`git rev-parse --path-format=absolute --git-common-dir`, with the pre-2.31
relative fallback). Unify on existing Go code; do not re-implement the
branch scan. Tests for every branch (no docs, no branches, exact, prefix,
ambiguous, worktree-less).

files: `internal/session/root.go` + test (new), `internal/job/discover.go`,
  `internal/git/git.go` (existing `WorktreeForBranch`)
depends: TASK-9
risk: high — note 3's hard-error-on-worktree-less-branch is a deliberate
  no-fallback trap; a "helpful" fallback to PROJECT_ROOT would silently show
  the wrong job's content.

### TASK-11: `internal/session` — docker argv/mount/env construction and run

Port run.sh's launch construction (notes 1 and 5, flags verbatim): the full
mount set — primary `-v <root>:/workspace:z`, docs mount (only when it
exists, target `/workspace/.claude` or `/workspace/.opencode` by tool),
context file read-only mount (`docs/AGENTS.md` → `/workspace/AGENTS.md` for
opencode or `/workspace/.claude/CLAUDE.md` for claude, with the
`docs/CLAUDE.md` fallback and the no-context warning/note lines), git-common-dir
mount for job worktrees, and `/dev/null` shadowing of every project
`.env`/`.env.*` except `*.example`/`*.sample`; all `-e` vars (OAuth token +
account UUIDs, git identity `GIT_AUTHOR_NAME_CFG`/`GIT_AUTHOR_EMAIL_CFG`
from host env then project git config, `MANIGOT_TOOL`, `MANIGOT_PRINT`,
`MANIGOT_QUOTE`, provider keys, `OPENCODE_MODEL`); `-it` only when stdin is a
terminal (`x/term.IsTerminal`), `--rm`, container name
`manigot-<basename(root)>-<pid>`, `--user <uid>:<gid>`, `--network=bridge`,
`--memory=2g`, `--security-opt=no-new-privileges`; quote pick from
`assets/quotes.json` (proper JSON parse, skipped in `--print`); the info
banner to stderr; prompt assembly (job prompt wins over `--prompt`,
`--agent` as a CLI flag not text, positional for claude vs `--prompt` for
opencode, and the `NEEDS-HUMAN-INPUT:` sentence appended only in `--print`
mode); `os.Stdin/Stdout/Stderr` wired through, Ctrl+C reaching the
container, and `mg` exiting with the agent's exit code. Constructing tests
must pin the exact argv/mount/env list (note 13).

files: `internal/session/docker.go` + `docker_test.go` (new),
  `internal/session/session.go`, `assets/quotes.json` (read)
depends: TASK-9, TASK-10
risk: high — this is the trap-list heart (notes 1, 5, 10); every flag,
  mount, and env var is load-bearing and the `--print` stdout/stderr split
  is what keeps the existing orchestrate/output tests green.

### TASK-12: Wire the `mg` session subcommand to `internal/session`

Bare `mg` (and `--agent`/`--job`/`--prompt`/`--tool`/`--profile`/`--print`
with passthrough) now calls the session package in-process instead of
exec'ing run.sh. run.sh remains on disk but is no longer reachable from Go
code (it dies in Phase 5). `mg init`'s and `mg agents`' re-exec launches
continue to work unchanged because they re-exec the binary's session path.

files: `cmd/mg/main.go`, `internal/session/*`
depends: TASK-9, TASK-10, TASK-11
risk: medium — first cutover from bash to Go for the most-used command;
  the 1:1 behavior (banner, mounts, exit codes) must survive, verified by
  the TASK-11 construction tests plus a manual session smoke if docker is
  available.

### TASK-13: Rewire mg-jdi's agent runner to `internal/session --print` (note 8)

`cmd/jdi`'s `commandAgentRunner.Run` must stop exec'ing the resolved run.sh
(`resolve.Resolve(resolve.Manigot())`) and instead call the session package's
`--print` path in-process, capturing stdout (the agent's JSON) and routing
diagnostics to stderr, with `--profile`/`--agent`/`--job` semantics
unchanged. Existing `orchestrate`/`output`/`cmd/jdi` tests (which parse the
JSON output contract) must stay green; adapt the runner's fake/interface if
needed. The TUI's detached jdi launch (launch.Jdi) may stay on the `mg jdi`
subcommand.

files: `cmd/jdi/main.go`, `internal/session/*`, `cmd/jdi/main_test.go`
depends: TASK-11, TASK-12
risk: medium — stdout-cleanliness is the whole point of `--print`; a
  diagnostic leaking onto stdout breaks DetectSignal and the log dedup.

### TASK-14: Rewire TUI session launches onto the binary's session subcommand

`internal/launch`'s `Agent`/`Quick`/`AgentQuick` shell strings must run the
Go binary's session subcommand (`os.Executable()` — i.e. `bin/mg
--profile … --agent … --job …`) instead of `resolve.Resolve(resolve.Manigot())`
(run.sh), preserving the cd-first, quoting, hold-on-failure wrapping and
`$PWD` handling; `launch.Jdi` should exec the `mg jdi` subcommand of the
same binary. `resolve` stays alive for hostcmd/config until Phase 5. Update
the launch tests' expected command strings.

files: `internal/launch/launch.go` + `launch_test.go`, `cmd/mg/main.go`
depends: TASK-12
risk: low — the shell-string shape is unchanged, only the manigot path
  source changes; tests pin the format.

---

## Phase 4 — Port job lifecycle into Go

### TASK-15: Extend `internal/git` with the lifecycle operations

Add to the git package (all exec-backed like the existing helpers, all with
graceful ErrNotARepo degrades where sensible): `WorktreeAdd(root, path, branch, base)`,
`WorktreeRemove(root, path)` + `WorktreeRemoveForce`, `WorktreePrune`,
`BranchDelete(root, branch)` (`-D`), `SquashMerge(root, branch)` +
`Commit(root, message)`, `WorkingTreeDirty(root)` (diff + diff --cached),
`SymbolicRefHead(root)` for `origin/HEAD` → default-branch resolution (with
the `main` fallback), `RefExists(root, ref)` for the namespace-collision
pre-check, `GitCommonDir(root)` (`--path-format=absolute` with the pre-2.31
fallback), `RevParseToplevel(root)`, `ConfigUserName/Email(root)`, and
`CommitUser`-style helpers as needed. Tests for each against a scratch repo.

files: `internal/git/git.go` + new tests
depends: TASK-1
risk: low — mechanical exec wrappers mirroring existing package style; the
  dirty-check and worktree ops are the ones `mg done`/`mg delete` correctness
  depends on.

### TASK-16: `job.CreateJob` — port `new-job.sh`

Go lifecycle function reproducing new-job.sh exactly: docs-walk-up root
resolution (error when absent), `.manigot/manigot.json` via `project.Load`
(baseBranch default `main`, jobBranchPrefix), arg validation (type
feature/fix/chore, `--base-branch` override), ID generation (6 chars
`a-z0-9` — `crypto/rand`, not math/rand), slug (same lowercase /
non-alphanumeric→`-` / collapse / trim pipeline), author (`git config
user.name` → `unknown`), branch name `[prefix/]type/<id>_<slug>`, the
pre-flight ancestor namespace-collision check with the exact error + fix
hint, the worktree-layout decision (sibling
`<dirname(root)>/.manigot-worktrees/<basename(root)>/<id>_<slug>` vs nested
`<root>/.manigot-worktrees/<id>_<slug>` when root's parent is on a different
device — `os.SameFile`/`syscall.Stat_t.Dev`, with the device check injectable
for tests — plus the `.git/info/exclude` append), `git worktree add`, the
four scaffold files byte-identical to the current templates, and the
"Scaffold job <id>_<slug>" commit inside the worktree; non-git fallback
writes the scaffold straight into `<root>/docs/jobs/`. Same summary output.
Tests: full create in a scratch repo incl. the nested/mount-point case
(note 13) and the non-git case.

files: `internal/job/create.go` + `create_test.go` (new), `internal/git/git.go`,
  `internal/project/settings.go` (existing)
depends: TASK-15
risk: high — the worktree layout (note 4) and the mount-point nesting case
  are the subtlest behavior in the whole system; the scaffold templates must
  be copied byte-for-byte or stage detection (`job.Stage`/`FileIsWritten`)
  and reviewers' expectations drift.

### TASK-17: `job.FinishJob` — port `finish-job.sh`

Go lifecycle function reproducing finish-job.sh exactly: branch resolution
(exact→prefix on the tail segment, ambiguity error listing branches),
worktree lookup + hard error when missing, worktree-branch guard (must equal
the resolved branch), brief.md presence check, verdict checks with the same
warnings (`grep '^## Overall' -A5 | grep -iE 'APPROVED|REJECTED|NEEDS WORK'`
semantics — reuse `job.Stage`/`verdictApproved` logic where it matches) and
the same `Continue anyway? [y/N]` confirmations, clean-tree check with the
same error, base-branch resolution (`baseBranch` → `origin/HEAD` → `main`),
archive move into `docs/jobs/archive/<name>` + `status: done` rewrite +
`archive: <name>` commit inside the job's own worktree, squash merge onto
the base branch with the `<title>\n\nJob: <name>` commit, worktree remove
(skipped when the worktree IS the main worktree, which is switched onto the
base branch) + best-effort prune, branch `-D`, identical info/output lines.
Prompts go through the TASK-4 `cli.Confirm` so the CLI and TUI share them.
Tests: full create→work→done roundtrip in a scratch repo incl. the
main-worktree-job case (note 13).

files: `internal/job/finish.go` + `finish_test.go` (new), `internal/git/git.go`,
  `internal/cli/*`
depends: TASK-15, TASK-4
risk: high — clean-tree + archive-commit + squash-merge + branch-delete
  ordering is destructive when wrong; every git step must run in the same
  worktree/root each bash step does.

### TASK-18: `job.DeleteJob` — port `delete-job.sh`

Go lifecycle function reproducing delete-job.sh exactly: the non-git plain
directory-delete path (exact then prefix under `docs/jobs/`, exclude
archive, same confirmation), the git path's branch resolution + worktree
lookup + hard error, dirty-worktree detection for the warning wording,
main-worktree-case detection (for both wording and skipping removal),
switch-off of the main worktree onto the base branch when it sits on the
job's branch, `worktree remove --force` + best-effort prune, branch `-D`,
and the identical confirmations (incl. "This cannot be undone."). Tests:
delete of a freshly created job; non-git delete.

files: `internal/job/delete.go` + `delete_test.go` (new), `internal/git/git.go`,
  `internal/cli/*`
depends: TASK-15, TASK-4
risk: medium — destructive but mechanically mirrors TASK-17's resolution
  code; the dirty-warning wording must match.

### TASK-19: Wire `mg job`/`mg done`/`mg delete` subcommands + TUI direct calls

`cmd/mg`'s `job`, `done`, `delete` subcommands call `job.CreateJob` /
`job.FinishJob` / `job.DeleteJob` with the CLI's interactive prompts
(identical wording). Replace the TUI's `hostcmd` shell-outs with direct
function calls (the brief's "direct function calls, not subprocesses"):
`ui/app.go` `updateNewJob` → `job.CreateJob`; the done/delete flows
(currently `hostcmd.DoneCommand`/`DeleteCommand` run in the foreground via
`tea.ExecProcess` because the scripts need an interactive terminal) call the
Go lifecycle with a TUI-side confirmation — either a Bubble Tea confirm
prompt or a non-interactive "already confirmed" path — so the TUI no longer
spawns a subprocess for these. `hostcmd` and its tests are deleted here (it
dies in the brief's Phase-5 list, but its only callers are these three TUI
sites plus resolve — removing it now keeps Phase 5 small).

files: `cmd/mg/main.go`, `internal/ui/app.go`, `internal/ui/newjob.go`,
  `internal/ui/*done/delete*` tests, `internal/hostcmd/` (deleted)
depends: TASK-16, TASK-17, TASK-18
risk: high — the TUI's done/delete flow changes from "subprocess with its
  own terminal for prompts" to "in-process with TUI-owned confirmation";
  getting the confirm UX wrong silently changes behavior users rely on.

---

## Phase 5 — Removal

### TASK-20: Delete `internal/resolve` and `internal/hostcmd` + all call sites

`resolve` dies (the binary IS the command now), so rework every remaining
user: `internal/config`'s `EnvFile`/`Dir`/`Path`/`Home` get a new
checkout-home resolution (move the `executableRoots`/`looksLikeCheckout`
logic — binary at `<root>/bin/mg`, or symlinked install — into `config` or a
small `internal/home` package), `internal/agentlist.Discover` uses the same,
`ui/cmdErrorText` drops the `resolve.NotFoundError` branch (its errors now
come from the Go lifecycle/session), and `cmd/tui`/`cmd/jdi`/`launch` must
have no `resolve` references left after TASK-13/14/19. Delete both packages
and their tests; `go vet ./...` + `go test ./...` green.

files: `internal/resolve/`, `internal/hostcmd/`, `internal/config/config.go`,
  `internal/agentlist/agentlist.go`, `internal/ui/app.go`, `internal/ui/cmderror_test.go`
depends: TASK-13, TASK-14, TASK-19
risk: medium — resolve is woven into config/agentlist/ui; the new
  checkout-home logic must keep `.env`/settings file discovery working from
  an installed symlink, a checkout `bin/mg`, and `go run`.

### TASK-21: Wire `mg tui` / `mg jdi` in-process; delete every remaining script

Fold the `cmd/tui` and `cmd/jdi` mains into `cmd/mg` as callable entrypoints
(extract `runTUI()`/`runJDI()` — the TUI's Bubble Tea program, jdi's
flag-parse + loop; both keep `-X main.version` injection), so `mg tui` and
`mg jdi`/`mg made-man` run in-process. Delete `bin/manigot-tui`,
`bin/manigot-jdi`, `scripts/tui.sh`, `scripts/jdi.sh`, and every remaining
script: `mg.sh`, `run.sh`, `profiles.sh`, `setup.sh`, `agents.sh`, `init.sh`,
`new-job.sh`, `finish-job.sh`, `delete-job.sh`, and `scripts/lib/`. Only
`scripts/entrypoint.sh` remains. Remove the stale tracked `Makefile.txt`
(references `./run.sh`; verify nothing else references it). `scripts/lib/`
no longer has consumers (the Go `git.WorktreeForBranch` is the
equivalent). After this, grep the whole repo: nothing may reference a
removed script or binary.

files: `cmd/mg/main.go`, `cmd/tui/`, `cmd/jdi/`, `bin/` (removed artifacts),
  `scripts/*` (all except `entrypoint.sh` deleted), `Makefile.txt` (deleted)
depends: TASK-20
risk: medium — the tui/jdi fold-in touches package-main structure that the
  tests currently build against (`cmd/jdi` tests, `tui` main); deleting 11
  scripts in one step is where "behavior identical" is easiest to break
  silently — each subcommand still needs its smoke.

### TASK-22: Final Makefile, README, and `docs/AGENTS.md` sync

Finalize `Makefile`: `make build` → `bin/mg`; `make install` → ONE symlink to
`bin/mg` (drop the `scripts/mg.sh` link and the tui/jdi notes); `tui`/`jdi`/
`run` targets collapse or become aliases; `rebuild`/`uninstall`/`help`
updated. Rewrite `README.md` (34KB, documents the old architecture) and
`docs/AGENTS.md` (canonical project context agents read at session start —
must describe the one-binary architecture: `cmd/mg` subcommands,
`internal/session`/`job`/`ui`/`orchestrate`, `scripts/entrypoint.sh` as the
only bash). Per the sync hard rule, `project-template/docs/AGENTS.md` gets
the same rewrite. Grep-verify nothing anywhere references a removed script
or binary (incl. `docs/CLAUDE.md`, `CONSOLIDATION-BRIEF.md` if it does).

files: `Makefile`, `README.md`, `docs/AGENTS.md`, `project-template/docs/AGENTS.md`,
  possibly `docs/CLAUDE.md`
depends: TASK-21
risk: medium — docs are the acceptance criteria's "accurate against the new
  architecture"; a stale README is the easiest regression to leave behind.

### TASK-23: CI (go vet, go test, shellcheck) + acceptance smoke

Add CI/`make check`: `go vet ./...`, `go test ./...`, `shellcheck
scripts/entrypoint.sh` (scope shrinks to the one remaining script, note 13).
The brief calls for CI but no `.github/` exists today — if GitHub Actions is
the intended vehicle, add a minimal workflow; otherwise a `make check`
target suffices — flag the ambiguity rather than guessing wrong. Run the
full acceptance smoke from the brief: session launch with and without
`docs/`; `--print` output cleanliness; a full `mg job` → work → `mg done`
roundtrip (worktree layout + squash merge + branch delete); `mg delete`;
`mg profiles`; `mg setup --check`; `mg init` on a scratch directory; the TUI
(list, detail, launch, settings, jdi badge); and a `mg jdi` end-to-end run.
Docker-dependent smokes only run if a usable docker daemon is present in
the environment — otherwise record them as the known manual-verification
gap.

files: `.github/workflows/*` (new, if CI is Actions), `Makefile`,
  `scripts/entrypoint.sh` (read-only)
depends: TASK-22
risk: low — verification-only; the only open question is the CI vehicle
  (no existing CI to extend).
