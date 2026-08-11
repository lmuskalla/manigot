# Tasks: Set default branch in settings

id: 6kbt43
status: open
analyst: @analyst
date: 2026-08-11

<!-- Produced by @analyst from brief.md, revised after human decisions. -->

## Scope summary

Make the base branch (hardcoded as `main`) user-configurable, defaulting to
`main`, so projects on `develop`/`master`/`trunk` can use the TUI. Per human
decision #2, the base branch lives in a **project-scoped settings file**
(`docs/manigot.json`), not the global TUI settings — base branch is a shared
project convention, so it travels with the project (committable) and is read
by both the TUI and the bare CLI `mg job`.

### Architecture (revised)
- **Global** (unchanged): `config/tui-settings.json` in the manigot checkout
  holds **personal** prefs — `editor`, `profile`. Gitignored, not shared.
- **Project** (new): `docs/manigot.json` in the *target project* holds
  **shared** project conventions — starting with `baseBranch`. Committable,
  no secrets. Read by:
  - the TUI (`m` shortcut, and the Settings form's base-branch field), via a
    new Go package;
  - `scripts/new-job.sh` directly (so the CLI `mg job "..."` respects it with
    no Go→bash parameter passing — the script is the single reader at
    job-creation time).

This means `hostcmd.NewJob`'s signature does **not** change: the TUI still
calls it as today, and `new-job.sh` reads `docs/manigot.json` for the base
branch itself.

### Decisions applied (from human)
1. `finish-job.sh` / `delete-job.sh` **stay as-is** — they already auto-detect
   via `git symbolic-ref refs/remotes/origin/HEAD` (fallback `main`); they are
   not hardcoded. Out of scope.
2. Base branch is **project-scoped** (`docs/manigot.json`); CLI `mg job` reads
   it too.
3. List-view hint becomes **"m base branch"**.

### Assumptions (confirm if wrong)
- `editor` and `profile` remain **global/personal** (my editor, my
  subscription); only `baseBranch` moves to the project file.
- `docs/manigot.json` is meant to be **committed** by the target project
  (shared team convention; contains only a public ref name, no secrets).

### Remaining decisions (analyst recommendation in brackets)
- **File name/location**: `docs/manigot.json`. [`docs/manigot.json` — clearly
  manigot-owned, lives beside `docs/AGENTS.md`, reachable by the existing
  project-root walk-up on both sides.]
- **Format**: JSON. [JSON, for consistency with `tui-settings.json` and clean
  Go I/O. Bash reads the single `baseBranch` key via a guarded `sed`
  extraction; revisit (add `jq`) only if more project keys appear.]
- **Go package**: new `tui/internal/project` with `Settings`/`Load`/`Save`/
  `Path`. [New package — distinct lifecycle/storage/sharing model from the
  global `config` package. Alternative: extend `config`.]
- **Seeding**: seed `project-template/docs/manigot.json` (default
  `{"baseBranch":"main"}`) and copy it in `init.sh`. [Recommended — makes the
  feature discoverable and committable from day one. Existing projects get it
  created on first TUI save.]
- **`--base-branch` override on `new-job.sh`**: keep as an optional override
  flag. [Recommended — cheap, useful for ad-hoc one-offs.]

## Task breakdown

TASK-1: Add a project-scoped settings package: `Settings{BaseBranch string}`
(json `baseBranch,omitempty`), `Path(root)` → `<root>/docs/manigot.json`,
`Load(root)` (missing/unreadable file ⇒ zero value, never an error, mirroring
`config.Load`), `Save(root, s)` (writes JSON, `docs/` already exists in any
initialized project), and `BaseBranchValue()` returning `"main"` when empty.
Include a round-trip test + a default-when-empty test.
files: tui/internal/project/settings.go (new), tui/internal/project/settings_test.go (new)
depends: none
risk: low — new, isolated package; pure additive I/O mirroring the proven
`config` package shape.

TASK-2: Add a base-branch text input to the Settings form and generalize the
tab/shift+tab focus cycle from its current 2-field toggle (editor ↔ profile)
to a 3-field cycle (editor → base branch → profile). The base-branch field
reads/writes the **project** file, while editor+profile keep reading/writing
the **global** file: `settingsView` carries both a global `config.Settings`
and a project base-branch value; `settingsValue()` returns the global slice
and a new `projectValue()` returns `project.Settings{BaseBranch}`. Label the
base-branch row so it's clear it's project-scoped. Update the focus/render
tests (they encode today's 2-field model) and add a `projectValue` test.
files: tui/internal/ui/settings.go, tui/internal/ui/settings_test.go
depends: TASK-1
risk: medium — the tab/focus logic is a small but touchy refactor; four
existing tests (TabTogglesFocus, ShiftTabTogglesFocusBackward,
ProfileCycleOnlyWhenProfileFocused, Render) encode the 2-field assumption.

TASK-3: Wire the project settings into the App: add a `projectSettings`
field on `App`, load it (`project.Load(a.root)`) at startup, and on the
settings form's submit call **both** `config.Save(global)` and
`project.Save(a.root, projectVal)` (surface either error in the form's
status). Pass the loaded project base branch into `newSettingsView`.
files: tui/internal/ui/app.go (App struct, startup Load near line 132,
updateSettings stSubmit around line 673, newSettingsView call around 639)
depends: TASK-1, TASK-2
risk: low-medium — straightforward wiring; the dual-save needs both errors
handled; verify existing settings save still works.

TASK-4: Make the list-view `m` quick-checkout use the configured base branch
(`a.projectSettings.BaseBranchValue()`) instead of the literal `"main"`; change
the footer hint at line 1220 from "m main" to **"m base branch"**; refresh
`a.projectSettings` on the ctrl+r refresh path (so externally-edited
`docs/manigot.json` is picked up); update the "back to main" comments
(lines 316, 652–660) to be base-branch-neutral. Confirm the `m`-key tests in
checkout_test.go still pass (default settings ⇒ `BaseBranchValue()` ⇒
`"main"`, so assertions unchanged unless we add a non-default case).
files: tui/internal/ui/app.go
depends: TASK-1, TASK-3
risk: low — single call-site change; default behavior preserved; well-covered
by checkout_test.go.

TASK-5: Teach `scripts/new-job.sh` to read the base branch from
`docs/manigot.json` (guarded single-key `sed` extraction; default `"main"` when
the file/key is absent), replace the `BASE_BRANCH="main"` literal at line 70,
add an optional `--base-branch <name>` override to the arg-parse loop, and
update the usage block + `Usage:` error line. Validate the resolved base
branch exists as a local ref (the existing check at lines 74–77 already does).
files: scripts/new-job.sh
depends: TASK-1 (agrees the JSON shape the bash extractor targets)
risk: low — isolated script edit; default preserves current behavior. The
bash JSON extraction is the one fragile spot (single known key, regex-bounded);
note in a code comment that adding more project keys warrants `jq`.

TASK-6: Seed the project settings file in the template/init flow and sync all
documentation (hard rule: AGENTS.md / project-template stay in sync):
- add `project-template/docs/manigot.json` with `{"baseBranch":"main"}`;
- add a `cp` line + echo to `scripts/init.sh` (fresh projects get it; existing
  projects get it created on first TUI save);
- in `docs/AGENTS.md` **and** `project-template/docs/AGENTS.md`: document
  `docs/manigot.json` as the new project settings file; update the
  `scripts/new-job.sh` description ("branched from main" → from the configured
  base branch); update the `mg job` command line; clarify in the
  `config/tui-settings.json` bullet that it holds personal prefs (editor/
  profile) and that base branch now lives in the project file;
- verify `agents/*.md` need no change (earlier grep showed only comments/test
  fixtures referencing "main", nothing functional).
files: project-template/docs/manigot.json (new), scripts/init.sh,
docs/AGENTS.md, project-template/docs/AGENTS.md
depends: TASK-1 … TASK-5 (so docs describe the final behavior)
risk: low — documentation + template seeding; mechanical edits.

## Suggested order
TASK-1 → TASK-2 + TASK-5 (parallel) → TASK-3 → TASK-4 → TASK-6.
Build with `make tui && make jdi`; run `go test ./...` from `tui/`.

## Notes for the developer
- No change to `hostcmd.NewJob` or its tests, and no change to `updateNewJob`'s
  call site: `new-job.sh` reads `docs/manigot.json` itself, so the TUI keeps
  invoking job creation exactly as today.
- `finish-job.sh` / `delete-job.sh` are intentionally untouched (decision #1).
- The project file is committable (no secrets); do not add it to any gitignore.
- If `docs/` is absent (TUI baseline is that it exists, since jobs live in
  `docs/jobs/`), `project.Load` returns the zero value ⇒ `"main"`, and
  `project.Save` would simply fail and surface in the form status.
