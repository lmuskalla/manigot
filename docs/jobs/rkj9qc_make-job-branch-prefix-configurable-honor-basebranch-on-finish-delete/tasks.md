# Tasks: make job branch prefix configurable; honor baseBranch on finish/delete

id: rkj9qc
status: open
analyst: @analyst
date: 2026-08-12

<!-- Produced by @analyst from brief.md. -->

## Scope summary

Two problems, one feature job:

1. **Configurable job branch prefix.** `new-job.sh` hardcodes the job branch as
   `feature|fix|chore/<id>_<slug>`. A project with a pre-existing plain branch
   named exactly `feature` (or `fix`/`chore`) cannot create any job of that
   type — git refuses `refs/heads/feature/<anything>` because `refs/heads/feature`
   already exists as a file-path (I reproduced: `cannot lock ref
   'refs/heads/feature/ati6um_jtl-typ-selektor': 'refs/heads/feature' exists`).
   Make the branch *prefix* project-configurable via a new `jobBranchPrefix`
   key in `.manigot/manigot.json` (default empty = today's behavior). With
   `jobBranchPrefix: "jobs"` the branch becomes `jobs/feature/<id>_<slug>`.
   Every resolver (run.sh, finish-job.sh, delete-job.sh, TUI discovery,
   jdi) matches jobs by the `<id>_<slug>` tail segment (`${b##*/}`), never by
   the prefix, so no resolver needs to change.

2. **finish/delete must honor baseBranch.** `finish-job.sh` and
   `delete-job.sh` resolve their merge/switch target from
   `git symbolic-ref refs/remotes/origin/HEAD` (fallback `main`), ignoring the
   `baseBranch` key that `new-job.sh` and the TUI already honor. Make both
   scripts read `baseBranch` from `.manigot/manigot.json` first (same guarded
   single-key `sed` as new-job.sh), falling back to the current
   `origin/HEAD` → `main` detection when the key is absent.

Plus a **pre-flight collision check** in `new-job.sh`: before `git worktree
add`, verify no ancestor path of the branch ref exists as a plain ref (e.g.
`feature` for `feature/ati6um_...`, or `jobs`/`jobs/feature` for
`jobs/feature/ati6um_...`), and fail with a human-readable error explaining
the namespace collision and pointing at the `jobBranchPrefix` setting —
instead of git's cryptic `cannot lock ref` fatal.

Architectural template: job 6kbt43 (`baseBranch` settings plumbing) — same
touch points (project.Settings field, TUI settings form row, guarded sed in
bash, docs sync).

## Task breakdown

TASK-1: Add `JobBranchPrefix` to `tui/internal/project/settings.go`:
`JobBranchPrefix string \`json:"jobBranchPrefix,omitempty"\`` (empty = no
prefix = today's naming). Unlike BaseBranch there is no defaulting method —
empty is a meaningful value ("no prefix"). Update the package doc comment
("first such project convention" → now two) and the round-trip test in
settings_test.go to carry the new field.
files: tui/internal/project/settings.go, tui/internal/project/settings_test.go
depends: none
risk: low — pure additive struct field + test.

TASK-2: Add a "Job branch prefix" text input to the TUI settings form
(`tui/internal/ui/settings.go`), placed right after the base-branch row (both
project-scoped). Wire it through: struct field, `newSettingsView` seeding from
`proj.JobBranchPrefix`, `resize`, `setFocus`, `update`'s text-input routing,
`projectValue()` returning `JobBranchPrefix: strings.TrimSpace(...)`,
`render` row + hint, and bump `stFieldCount` 5 → 6 with a new
`stFocusJobPrefix` constant inserted between `stFocusBranch` and
`stFocusCount` (shift the count/profile/terminal constants). Update the
settings tests that encode the tab cycle (TabCyclesFocus,
ShiftTabCyclesFocusBackward, ProfileCycleOnlyWhenProfileFocused,
RecentCountEdits, TerminalEdits — all reach fields by fixed tab counts), the
Render string list (add the new label), and add a
TestSettingsJobPrefixEdits + projectValue coverage. Label the row
project-scoped like base branch ("stored in .manigot/manigot.json, shared
with your team", "blank = feature/…").
files: tui/internal/ui/settings.go, tui/internal/ui/settings_test.go
depends: TASK-1
risk: medium — the focus-cycle refactor is the touchy part; named constants
keep it mechanical, but the tab-count tests must be updated in lockstep.

TASK-3: Teach `scripts/new-job.sh` to read `jobBranchPrefix` from
`.manigot/manigot.json` (same guarded `sed` as baseBranch, default empty) and
build the branch as `${PREFIX:+${PREFIX}/}${JOB_TYPE}/${ID}_${SLUG}` — e.g.
`jobs/feature/ati6um_jtl-typ-selektor`. Empty prefix keeps today's exact
naming. Update the usage block + comments to mention the setting. The
scaffold brief.md already records `branch: $BRANCH` — no change needed there.
files: scripts/new-job.sh
depends: TASK-1 (agrees the JSON key the sed targets)
risk: low — one extra sed read + one line of branch-name composition;
default preserves current behavior.

TASK-4: Make `scripts/finish-job.sh` and `scripts/delete-job.sh` honor
`baseBranch`: read it from `.manigot/manigot.json` (same guarded sed), and
use it as `DEFAULT_BRANCH` when present; fall back to the current
`git symbolic-ref refs/remotes/origin/HEAD` (→ `main`) detection when the
key is absent. In finish-job.sh this replaces the merge target resolution at
the squash-merge step (and the info echo); in delete-job.sh it replaces the
"switch the main worktree off the branch" target. Update the comments that
describe the resolution (origin/HEAD → baseBranch-first).
files: scripts/finish-job.sh, scripts/delete-job.sh
depends: none
risk: low-medium — behavior change for projects with a configured baseBranch
(that's the point); unconfigured projects keep today's exact behavior.

TASK-5: Add a pre-flight namespace-collision check to `scripts/new-job.sh`,
just before `git worktree add`: for each proper ancestor path of the branch
name (all path segments except the leaf `<id>_<slug>`), test
`git rev-parse --verify --quiet refs/heads/<ancestor>`; if any resolves, fail
with a clear error: "cannot create job branch 'X': a branch named 'Y' already
exists, which blocks the 'Y/…' namespace; set jobBranchPrefix in
.manigot/manigot.json (or rename the branch) and retry." Only run it in the
git case (CURRENT_BRANCH non-empty). Also guard it for the non-git fallback.
files: scripts/new-job.sh
depends: TASK-3 (uses the composed branch name)
risk: low — read-only pre-check; identical outcome for the common case (no
collision → check passes silently, worktree add proceeds).

TASK-6: Sync docs. Hard rule: docs/AGENTS.md is the canonical source and must
stay in sync; project-template/docs/AGENTS.md is a generic per-project
placeholder (per 6kbt43 verdict) — do NOT add manigot-system content there.
Update `docs/AGENTS.md`:
- `new-job.sh` bullet: branch is `[<prefix>/]<type>/<id>_<slug>` with the
  prefix from `jobBranchPrefix`;
- `finish-job.sh` and `delete-job.sh` bullets: merge/switch target is the
  configured `baseBranch`, falling back to `origin/HEAD` → `main`;
- `.manigot/manigot.json` bullet: document the new `jobBranchPrefix` key
  alongside `baseBranch`;
- `mg job` command line stays (prefix is a settings key, not a flag).
Also update `README.md`'s "Branch naming:" line (line ~443) to mention the
optional configured prefix. Verify `agents/*.md` need no change.
files: docs/AGENTS.md, README.md
depends: TASK-1 … TASK-5 (docs describe final behavior)
risk: low — documentation edits.

## Suggested order
TASK-1 → TASK-2 → TASK-3 → TASK-5 → TASK-4 → TASK-6.

## Notes for the developer
- Build with `make tui && make jdi`; run `go test ./...` from `tui/`.
- Every resolver matches jobs by the id_slug tail segment (`${b##*/}`), so a
  prefixed branch (`jobs/feature/ati6um_...`) resolves identically to an
  unprefixed one — no changes to run.sh, worktree.sh, TUI discovery, or jdi.
- The TUI's new-job form (`newjob.go`) only passes `--type`; the prefix is a
  project settings key read by new-job.sh itself, so no hostcmd/newjob
  changes.
- Verify the end-to-end flow manually if possible: configure
  `jobBranchPrefix` in a scratch repo that also has a plain `feature` branch,
  run `mg job "..."`, confirm the branch is `jobs/feature/<id>_<slug>`, then
  `mg done` merges into the configured baseBranch (not origin/HEAD).
- Do not add `jobBranchPrefix` to any gitignore — it's a committable project
  convention like baseBranch.
