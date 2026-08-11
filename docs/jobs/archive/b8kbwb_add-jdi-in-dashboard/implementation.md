# Implementation: Add jdi in dashboard

## Summary

Added a `j` keybinding to the TUI's dashboard (list view) that starts `mg jdi`
for the job under the cursor, detached in the background — mirroring the
detail view's existing `j` handler (already-running guard via the on-disk
sidecar with the in-session `jdiSeen` fallback, status message, dedup seeding,
spinner tick). The `j`/`k` vim-style cursor-move aliases were removed from the
list so the keys are freed (and a broken `j`-moves-down vs `j`-launches-jdi
pair is avoided), the footer hint and docs now advertise the new binding, and
test coverage mirrors the detail-view `jdilaunch_test.go` suite for the list.

## Changes

TASK-1: Added `case "j":` to `updateList` in `tui/internal/ui/app.go`,
launching `launch.Jdi(j.ID, a.root, a.settings.ProfileValue())` for the job
under the cursor with the same already-running guard (`jdiAlreadyRunning`,
naming the running agent), `jdiSeen`/`jdiSeenAt` seeding on success, footer
status ("→ mg jdi started in the background — see the list badge"), and
`return a, a.startSpinnerIfRunning()`; a no-op when the list is empty. In the
same commit, removed `"j"` from the `case "down", "j":` cursor-move alias (the
two bindings cannot coexist in the same switch).

TASK-2: Removed `"k"` from the list's `case "up", "k":` cursor-move alias in
`tui/internal/ui/app.go`, leaving dashboard navigation purely `↑`/`↓` plus
`home`/`end`, `g`/`G` — consistent with the detail view's post-vac06k
no-vim-keys state.

TASK-3: Updated `App.footer()`'s list hint in `tui/internal/ui/app.go` to
include `j mg-jdi` ("↑/↓ navigate · enter view · j mg-jdi · o quick · a agent
· n new · s settings · ctrl+r refresh · q quit").

TASK-4: Added test coverage in `tui/internal/ui/jdilaunch_test.go` driving
`updateList` instead of `updateDetail`: launch-from-list (stub mg-jdi runs,
`jdiSeen` seeded, footer status set pointing at the list badge, spinner cmd
returned, state stays list), resolution failure ("not found", `jdiSeen` not
seeded), already-running block via on-disk sidecar (naming `@developer`),
block via in-session dedup fallback (no sidecar), and empty-list no-op. Added
a cursor regression test in `tui/internal/ui/list_test.go` proving `j`/`k` no
longer move the list cursor while `up`/`down` still do.

TASK-5: Updated `README.md` — the list-view keybindings table's move-selection
row dropped `k`/`j` (leaving `↑`/`↓`) and gained a `j` row ("run `mg jdi`
against the selected job, detached in the background — watch via the list's
status badge"); the "mg jdi status & log" section now says "Press `j` in the
list (on the selected job) or in the detail view".

TASK-6: Updated the `tui/cmd/jdi` architecture bullet in the canonical
`docs/AGENTS.md` (not the read-only `/workspace/AGENTS.md` mount) to say "(`j`
from the list or the detail view)". Verified `agents/*.md` and
`project-template/docs/AGENTS.md` don't reference this detail, so no sync
change was needed there.

## Known issues / follow-ups

- The dashboard's `j` handler intentionally does not open the detail view's
  log tab — the launch is from the list, so there is no log tab open yet; the
  status message and the list badge are the visibility (matching the brief).
- `docs/jobs/b8kbwb_add-jdi-in-dashboard/tasks.md` contained the analyst's
  uncommitted edits when TASK-1 started; those were swept into the TASK-1
  commit by `git add -A` (standard workflow behavior).
- TASK-2's removal of `k` was implemented per the tasks.md default (the
  "out of scope" note allowed keeping `k` if the human preferred; no human
  input was available, so the default was followed).
