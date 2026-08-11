# Verdict: Set default branch in settings

id: 6kbt43
status: open
reviewer: @reviewer
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed against `tasks.md` on branch `feature/6kbt43_set-default-branch-in-settings`.
Cross-referenced `git diff main...HEAD`, read each touched file, built the TUI
(`go build ./...`), and ran `go test ./...` from `tui/` — all packages pass,
including the four new tests this branch adds.

TASK-1: PASS
notes: `tui/internal/project/settings.go` mirrors the proven `config` package
shape exactly: `Settings{BaseBranch}` (json `baseBranch,omitempty`), `Path(root)`
→ `<root>/docs/manigot.json`, `Load` treats a missing file as a zero-value
non-error (matching `config.Load`'s degrade-to-default contract), `Save`
creates `docs/` if absent, and `BaseBranchValue()` returns `"main"` when empty.
`settings_test.go` covers the round-trip, the missing-file zero-value, and the
default-when-empty behaviour. Clean, isolated, additive.

TASK-2: PASS
notes: `settings.go` adds a base-branch `textinput` between editor and profile
and generalizes the focus cycle from a 2-field toggle to a 3-field cycle using
named constants (`stFieldCount`, `stFocusEditor/Branch/Profile`) — better than
bare ints. The dual-file split is clean: `settingsValue()` returns the global
slice unchanged, `projectValue()` returns `project.Settings{BaseBranch}` with
TrimSpace. `setFocus` correctly keeps the text cursor on the right input and
blurs it when profile is focused. The base-branch row is clearly labeled
`(project):` with a hint noting `docs/manigot.json`. Tests cover both cycle
directions, separate editor/base-branch editing (no leak), seeding from project
settings, and trim/separation from global.

TASK-3: PASS
notes: `App` got a `projectSettings project.Settings` field; `NewApp` loads it
alongside `config.Load` with the same degrade-to-default shape. Error handling
is correct: a `project.Load` error *appends* to `a.status` rather than
overwriting a config-load error so both can be seen. The settings form's submit
handler dual-saves (`config.Save` then `project.Save`), surfaces either error in
`settingsView.status`, leaves the form open on failure, and only commits the
in-memory copies + returns to the list once both succeed. `newSettingsView` is
seeded with `a.projectSettings`.

TASK-4: PASS
notes: The list-view `m` key now calls
`a.checkoutCmd(a.projectSettings.BaseBranchValue())` instead of the literal
`"main"`; footer hint changed from `m main` to `m base branch` (line 1265);
`refresh()` (the ctrl+r path) re-reads `docs/manigot.json` so an externally
edited base branch is picked up without an app restart; the two "back to main"
comments are now base-branch-neutral (lines 338–339, 685–694). Default behaviour
is preserved — the three pre-existing `m`-key tests pass unchanged because the
default `BaseBranchValue()` is `"main"`. New `TestMainKeyUsesConfiguredBaseBranch`
exercises a non-default branch end-to-end.

TASK-5: PASS
notes: `scripts/new-job.sh` adds `--base-branch <name>` to the arg-parse loop,
reads `baseBranch` from `docs/manigot.json` via a guarded single-key `sed`
regex (defaulting to `main` when the file/key is absent), lets `--base-branch`
override the file, and updates the usage block, the `Usage:` error line, and
the git-branch comment. The existing local-ref validation at lines 103–106
covers both sources. The sed regex correctly handles both pretty-printed
(`json.MarshalIndent`) and one-line JSON, and an empty configured value falls
through to `main` via the `[[ -n "$VALUE" ]]` guard. Comment notes `jq` is
warranted only if more project keys appear — reasonable.

TASK-6: PASS (with one documented, justified deviation)
notes: `project-template/docs/manigot.json` seeded with `{"baseBranch":"main"}`;
`scripts/init.sh` header comment, validation loop, copy block, and echo all
updated to include `manigot.json`; `docs/AGENTS.md` updated in all four places
the task called out (the `new-job.sh` bullet, the `init.sh` bullet, the
`config/tui-settings.json` bullet now clarified as **personal** with a
forward-pointer, and the new `docs/manigot.json` bullet; plus the `mg job`
command line mentions `--base-branch`).
Deviation: `project-template/docs/AGENTS.md` was intentionally NOT updated. The
task list quoted the hard rule about keeping it in sync with `docs/AGENTS.md`,
but as the developer's `implementation.md` notes and a direct read of the
template confirms, `project-template/docs/AGENTS.md` is a generic project
placeholder (`# [Project Name]`, `Brief description of what this project
does…`) filled in per-target-project, NOT a mirror of manigot's own system
docs. Adding manigot-system content there would corrupt its purpose. The hard
rule as stated does not actually apply to this file. The deviation is honest,
documented in `implementation.md`, and correct — not a blocker.

## Security

Not run. No secrets, OAuth tokens, or credentials are introduced or moved by
this change. The new `docs/manigot.json` contains only a public git ref name
(the base branch), is correctly described as committable/shareable, and is
explicitly NOT added to any gitignore — matching both the brief and the
existing `config/tui-settings.json` (personal prefs) separation of concerns.
The `sed` extraction in `new-job.sh` only reads a ref-name string and feeds it
into `git checkout -b <branch> <base>`; an attacker who can write
`docs/manigot.json` can already commit arbitrary code to the repo, so there is
no new attack surface.

## Overall

APPROVED

All six tasks are implemented as specified, the build is green, and the test
suite (including four new tests) passes. Commit discipline is clean: one
commit per task in the correct `[6kbt43] TASK-N: …` format, plus a separate
implementation-summary commit; no out-of-scope refactoring or stray file
edits in the diff.

The three "known issues / follow-ups" the developer flagged in
`implementation.md` are all correctly out of scope for this job:
- `project-template/docs/AGENTS.md` not updated — justified (see TASK-6 notes).
- `agents/reviewer.md` still says `git diff main...HEAD` — explicitly excluded
  by TASK-6 ("verify `agents/*.md` need no change"); a real but separate
  follow-up for a future base-branch-neutral reviewer job.
- `/workspace/AGENTS.md` read-only overlay not re-mirrored — correct per the
  hard rule that only `docs/AGENTS.md` is edited; the next image rebuild picks
  up the change.
- Missing trailing newline in seeded `manigot.json` / `project.Save` output —
  cosmetic, acknowledged.

Nothing here blocks merge.
