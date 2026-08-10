# Tasks: Better cli syntax

id: 9pze1x
status: open
analyst: @analyst
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Scope decisions (confirmed)

The previous pass flagged four scope questions instead of guessing. All four
are now resolved, so nothing below is provisional:

1. **`tui/main.go`/`tui/cmd/jdi/main.go` `--help`/error text (TASK-5), and
   `agents/quality.md`/`agents/reviewer.md` prose (TASK-6): in scope.**
   Both are string-literal-only changes that spell out the exact old command
   names this job renames. Leaving them stale would directly undercut the
   job's own purpose (a consistent, discoverable command surface), and
   neither changes any behavior/control flow. The brief's out-of-scope note
   ("no change to `tui/`'s existing Go logic beyond what's needed to keep
   them invocable") is read as targeting behavioral/algorithmic changes
   (e.g. redesigning resolution), not literal string corrections of the
   renamed names themselves — see decision 4 for the same principle applied
   to `tui/internal/resolve`.
2. **`Usage: mg-job ...`-style strings inside `new-job.sh`/`finish-job.sh`/
   `delete-job.sh`: confirmed out of scope, left unchanged.** Unlike
   decision 1, the brief is categorical and specific here: "None of the
   underlying scripts (`run.sh`, `new-job.sh`, `finish-job.sh`, `tui.sh`,
   `jdi.sh`) change their internal logic, flags, or behavior" — naming these
   five files explicitly. These usage strings will read as slightly stale
   post-rename (e.g. `mg job` with no args now prints "Usage: mg-job ..."
   instead of "Usage: mg job ..."); that is an accepted, documented
   consequence of the brief's own constraint, not an oversight.
3. **`docs/AGENTS.md`'s missing `delete-job.sh`/`mg-delete` documentation:
   in scope — TASK-4 adds it.** Updating the four bullets/lines that do
   exist while leaving the file silent on the fifth command would leave the
   canonical project-context file inconsistent with README.md and with the
   brief's own five-way dispatch table. Filling the gap is a small, low-risk
   addition directly adjacent to what TASK-4 is already touching.
4. **`tui/internal/resolve`/`tui/internal/hostcmd` `Spec.Label` fields and
   doc comments (e.g. `Label: "mg-job"`, "shells out to ... mg-job"): in
   scope for text only — TASK-7 (new).** These are user-facing in the one
   path where `Resolve` fails outright (`NotFoundError`'s message) and
   developer-facing everywhere else (doc comments). They get corrected to
   the new `mg <subcommand>` phrasing. The `Names`/`Script`/`EnvVar` fields
   and the resolution algorithm itself (env override → `$PATH` → script
   fallback, in that order) are explicitly **not** touched: a standalone
   `mg-job`-named executable can never be installed by `make install` again,
   but keeping it as a harmless, always-tried-first `$PATH` candidate costs
   nothing and preserves a manual compatibility escape hatch for anyone who
   creates one by hand. Redesigning `Resolve` to express a genuine two-word
   invocation (`mg job`) is a real algorithmic change and stays out of
   scope, per the brief's restriction on `tui/`'s existing Go logic.

## Task breakdown

TASK-1: Create the new dispatcher script `scripts/mg.sh`, the sole `mg`
symlink target, that inspects `$1` and execs the matching existing script
unchanged (`job`→`new-job.sh`, `tui`→`tui.sh`, `jdi`→`jdi.sh`,
`done`→`finish-job.sh`, `delete`→`delete-job.sh`; anything else — no args, or
any other first token, including the `--agent`/`--job`/`--tool`/`--print`
flags — falls through to `run.sh` with all original args untouched), per the
brief's Notes. It must resolve its own real directory the same
symlink-following way `run.sh`/`tui.sh`/`jdi.sh` already do (`make install`
symlinks it into `PREFIX/bin`), so it can find its sibling scripts regardless
of install location.
files: scripts/mg.sh (new)
depends: none
risk: medium — this is the only new logic in the job; a mismatch in the
exact-match on `$1` (e.g. accidentally matching a prefix, or mishandling the
empty-args case) breaks either a subcommand or the default `mg` session
start, which is the most common invocation.

TASK-2: Update `Makefile`'s `LINKS` list to a single `mg:mg.sh` entry
(dropping the other five), and correct the two `mg-tui`/`mg-jdi`-worded hints
in the `install` target (the binaries `make tui`/`make jdi` build) plus the
adjacent "canonical sc- names" comment block above `LINKS`, which already
describes the exact mechanism being collapsed, to describe the new
single-symlink install accurately. `install`/`uninstall` themselves loop
over `LINKS` generically and need no other change.
files: Makefile
depends: TASK-1
risk: low — mechanical list/comment edit; `install`/`uninstall` logic itself
is untouched.

TASK-3: Update `README.md` throughout: the repo file tree under "The
manigot repo" (`scripts/` listing and what each installs as), "The installed
commands" table (collapses to one row), "Installing without symlinks"
(alias block and env-var list), the top-level "Usage" examples, "Job
workflow" examples (`mg-job "..."` etc.), "Autonomous mode (`mg-jdi`)"
examples, and the "TUI" section's build/run instructions and keybinding
descriptions that name `mg-job`/`mg-done`/`mg-delete`/`mg-tui`/`mg-jdi` —
all to the new `mg <subcommand>` syntax.
files: README.md
depends: TASK-1, TASK-2
risk: low — docs-only, but the old names appear in many places; easy to miss
one on a single pass.

TASK-4: Update `docs/AGENTS.md` (the canonical source — never edit the
read-only mounts `/workspace/AGENTS.md` / `/workspace/.claude/CLAUDE.md`):
the "Stack" bullet, the "Architecture" bullets that currently say
"installed as `mg-job`" / "`mg-tui`; wrapper around..." / "`mg-jdi`; wrapper
around...", and the "## Commands" list's `mg-job`/`mg-done`/`mg-tui`/`mg-jdi`
lines — to the new `mg <subcommand>` syntax, adding a bullet for the new
`scripts/mg.sh` dispatcher itself alongside the existing script bullets, and
(per scope decision 3) adding the previously-missing `delete-job.sh`/
`mg delete` bullet and Commands-list line so all five subcommands are
documented, not just the four that were already there. Per the hard rules,
keep `agents/*.md` and `project-template/docs/AGENTS.md` in sync with
whatever this file ends up saying (`agents/*.md` is covered by TASK-6;
`project-template/docs/AGENTS.md` doesn't currently mention any of these
command names, so no change expected there — confirm on a pass).
files: docs/AGENTS.md
depends: TASK-1
risk: low — same nature as TASK-3, plus one small net-new line (the
`mg-delete` gap-fill) rather than a pure rename.

TASK-5: Update the two host-side binaries' own `--help`/usage/error-prefix
string literals to match the new invocation: `tui/main.go`'s `flag.Usage`
text and `mg-tui:`-prefixed error lines, and its package doc's "shells out to
the host commands sc and mg-job" sentence; `tui/cmd/jdi/main.go`'s
`flag.Usage` text and `mg-jdi:`-prefixed error lines. String-literal changes
only — no behavioral change to either binary. (Scope decision 1.)
files: tui/main.go, tui/cmd/jdi/main.go
depends: TASK-1
risk: low — string literals only, no control-flow change.

TASK-6: Update the two agent files that name the standalone `mg-done`
command in prose (`agents/quality.md`: "blocks `mg-done`"; `agents/reviewer.md`:
"blocks `mg-done`") to `mg done`. (Scope decision 1.)
files: agents/quality.md, agents/reviewer.md
depends: TASK-1
risk: low — two one-line text edits.

TASK-7: Update the `Spec.Label` fields and doc comments in
`tui/internal/resolve/commands.go` (e.g. `Label: "mg-job"` → `"mg job"` for
`Job()`/`Done()`/`Delete()`/`Jdi()`, and the package doc's naming-decision
comment), `tui/internal/resolve/resolve.go`'s package doc (which spells out
`mg-job`/`mg-done`/`mg-delete`/`mg-jdi` in its overview), and
`tui/internal/hostcmd/hostcmd.go`'s package doc ("the existing manigot host
commands (mg-job, etc.)") to the new `mg <subcommand>` phrasing. Update
`tui/internal/resolve/commands_test.go`'s `TestCommandSpecs` table (its
`label` column) to match the new `Label` values — its `Names`/`env`/`script`
columns, and `TestCommandSpecsAreCopies`' assertion on `Names[0] ==
"mg-job"`, stay unchanged since `Names`/`Script`/`EnvVar` are not touched
(scope decision 4). `Names` and the resolution order/algorithm in
`resolve.go` (`Resolve`, `resolveOverride`, `repoRoots`, etc.) are explicitly
out of scope for this task.
files: tui/internal/resolve/commands.go, tui/internal/resolve/resolve.go,
tui/internal/resolve/commands_test.go, tui/internal/hostcmd/hostcmd.go
depends: TASK-1
risk: low — text/comment-only changes plus one test-table update; the
resolution algorithm itself is untouched, so this cannot change what
`Resolve` returns for any input, only what it's labeled as.

TASK-8: Verify — expected to require no further code change beyond TASK-7 —
that `tui/internal/resolve`'s resolution chain (env override → `$PATH` →
`$MANIGOT_HOME/scripts/*.sh` fallback) still correctly locates the job/done/
delete/jdi host commands for `tui/internal/hostcmd` and `tui/internal/launch`
once only `mg` is symlinked. The `Names` fields (`mg-job`, `mg-done`,
`mg-delete`, `mg-jdi`) will never match on `$PATH` anymore (`make install`
no longer creates them), so every resolution should fall through to the
unaffected `scripts/*.sh` fallback — confirm this by running the TUI's "n"
(new job), "D" (done), "x" (delete), and "j" (mg-jdi) actions after `make
install` with the collapsed `LINKS`, both with and without `$MANIGOT_HOME`
set by hand. If this uncovers an actual resolution failure (not just the
label-text change from TASK-7), that's new information for a scope
discussion, not something to patch by guessing.
files: none expected beyond TASK-7's; tui/internal/resolve/resolve_test.go
only if a real functional gap is found.
depends: TASK-1, TASK-2, TASK-7
risk: medium — if the "falls through to script fallback" assumption is
wrong, this is the one place a silent functional regression (not just stale
wording) could hide.

TASK-9: Manual smoke test of the full new command surface after TASK-1/2
land and `make install` is re-run: bare `mg`, `mg --tool opencode`,
`mg --agent analyst --job <id>`, `mg job "title"` (with and without
`--type`), `mg tui`, `mg jdi --job <id>`, `mg done <id>`, `mg delete <id>`,
plus the edge cases called out in the brief (no args at all; an unrecognized
first word falling through to `run.sh`'s own passthrough/error handling).
files: none (verification of TASK-1/TASK-2's combined behavior)
depends: TASK-1, TASK-2
risk: low — verification only, but this is the one user-facing behavior
change in the whole job.

TASK-10 (added post-review, per verdict.md's blocking scope gap): Update the
`mg-jdi` runtime strings the TUI actually displays to a user — the ones
`verdict.md` lists explicitly — to `mg jdi`: `tui/internal/ui/app.go`'s
"already running" status line, the "started in the background" status line,
and the three list-row badge variants (`running`/`finished`/`needs human`);
`tui/internal/ui/detail.go`'s `[j] mg-jdi` action button label, the hint
bar's `j run mg-jdi`, and the log tab's two placeholder strings; and
`tui/internal/launch/launch.go`'s wrapped start error (`"start mg-jdi:
%w"`). Also update `README.md`'s `[mg-jdi: ...]` badge-text references (the
"List-row badge" bullet) to stay in sync once the TUI's actual string
changes. String-literal changes only, no behavioral change — same kind and
risk profile as TASK-5/TASK-6. Doc comments elsewhere in these same files
that merely *mention* `mg-jdi` in prose (not rendered/returned to a user)
are out of scope, consistent with scope decision 1's "string-literal
changes only" framing.
files: tui/internal/ui/app.go, tui/internal/ui/detail.go,
tui/internal/launch/launch.go, README.md
depends: TASK-1, TASK-5
risk: low — string literals only, no control-flow change; confirmed by
`go build ./...`/`go vet ./...` after the edit.
