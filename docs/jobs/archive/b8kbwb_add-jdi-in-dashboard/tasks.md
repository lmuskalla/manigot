# Tasks: Add jdi in dashboard

  id: b8kbwb
  status: open
  analyst: claude
  date: 2026-08-11

  <!-- Produced by @analyst from brief.md. -->

  ## Task breakdown

  TASK-1: Add a `j` keybinding to the dashboard (list view) that starts `mg
  jdi` for the job under the cursor — mirroring `updateDetail`'s existing "j"
  handler (already-running guard via `jdiAlreadyRunning` with a status message
  naming the running agent, `launch.Jdi(job.ID, a.root,
  a.settings.ProfileValue())`, seeding `a.jdiSeen`/`a.jdiSeenAt` on success,
  a footer status message, and returning `a.startSpinnerIfRunning()`; a no-op
  when the list is empty) — and, in the same change, free the key by removing
  `"j"` from the list's `case "down", "j":` cursor-move alias (the two
  bindings cannot coexist in the same switch — a duplicate case is a compile
  error — so they must land in one commit). The status message should be
  worded for the list context (there is no log tab open yet), e.g. "→ mg jdi
  started in the background — see the list badge".
       files: tui/internal/ui/app.go (updateList — new `case "j":` and the
         `"down", "j"` case at ~line 668)
       depends: none
       risk: medium — a new launch entry point in the list switch; the handler
         logic itself is a proven copy of updateDetail's "j" case, but the
         mandatory nav-alias removal is coupled into the same switch, and the
         empty-list guard / footer-status (vs detail-status) differences are
         new call sites.

  TASK-2: Remove `"k"` from the list's `case "up", "k":` cursor-move alias,
  leaving the dashboard's navigation purely `↑`/`↓` (plus `home`/`end`,
  `g`/`G`) — consistent with the detail view's post-vac06k no-vim-keys state
  ("we're not gaining anything meaningful with j/k here — we're just using up
  keys") and avoids leaving a broken pair where `k` moves up but `j` launches
  jdi. Judgment call: keeping `k` is a valid alternative — see the out-of-scope
  note; drop this task if the human prefers it.
       files: tui/internal/ui/app.go (updateList's `"up", "k"` case at ~line
         664)
       depends: TASK-1 (the j/k pair breaks the moment TASK-1 lands, so this
         should ship with or right after it)
       risk: low — a one-token alias removal; arrow keys and g/G/home/end
         remain, and no existing test drives list-view navigation via "k" or
         "j" (verified by search).

  TASK-3: Update the list footer hint (`App.footer()` in app.go, ~line 1263)
  to advertise the new key — e.g. "↑/↓ navigate · enter view · j mg-jdi · o
  quick · a agent · n new · s settings · ctrl+r refresh · q quit" (the hint
  does not currently mention k/j, so only the new "j mg-jdi" element is
  added).
       files: tui/internal/ui/app.go (footer's hint string)
       depends: TASK-1
       risk: low — string-literal change; existing footer tests assert only on
         "q quit" / "a agent" substrings, both preserved.

  TASK-4: Add test coverage for the dashboard "j" flow, mirroring
  jdilaunch_test.go's detail-view coverage: launch-from-list (stub mg-jdi
  runs, jdiSeen seeded, footer status set, spinner cmd returned),
  already-running block via on-disk sidecar, block via in-session dedup
  fallback (no sidecar), resolution failure (status "not found", jdiSeen not
  seeded), and empty-list no-op; plus a regression test that "j"/"k" no
  longer move the list cursor (contrast with `down`/`up` still working).
       files: tui/internal/ui/jdilaunch_test.go (reuse markerStub /
         waitForMarkerRuns / addJobWorktree helpers; the tests drive
         updateList instead of updateDetail), possibly tui/internal/ui/
         list_test.go
       depends: TASK-1, TASK-2
       risk: low — additive tests following the established patterns in
         jdilaunch_test.go.

  TASK-5: Update README.md to match: the list-view keybindings table's
  move-selection row loses `k`/`j` (leaving `↑`/`↓`) and gains a `j` row
  ("run `mg jdi` against the selected job, detached in the background — watch
  via the list's status badge"), and the "mg jdi status & log" section's
  "Press `j` in the detail view to start `mg jdi`…" line becomes "in the list
  (on the selected job) or in the detail view".
       files: README.md (list-view table at ~line 602, "mg jdi status & log"
         at ~line 693)
       depends: TASK-1
       risk: low — documentation only, but several separate occurrences to
         catch; missing one leaves the docs self-inconsistent rather than
         breaking anything.

  TASK-6: Update docs/AGENTS.md's `tui/cmd/jdi` architecture bullet, which
  says "a TUI-launched run (`j` in the detail view) has no terminal of its own
  at all" (~line 156) — make it reflect that `j` now also launches from the
  dashboard list, e.g. "(`j` from the list or the detail view)".
       files: docs/AGENTS.md (not the gitignored /workspace/AGENTS.md
         mount — the hard rule: edit the canonical docs/AGENTS.md only)
       depends: TASK-1
       risk: low — single-line documentation; verified `agents/*.md` and
         `project-template/docs/AGENTS.md` don't reference this detail, so no
         sync change needed there.

  ## Out of scope (confirmed, not tasks)

  - The detail view's `j` keybinding and its already-running guard: already
    implemented (nrv5sa, vac06k), untouched.
  - The agents picker's own `↑/↓/k/j` navigation (agentspicker.go): a separate
    view, not the dashboard; the brief scopes this job to the dashboard.
  - `l` / `right` / `enter` opening the detail view from the list: not
    mentioned by the brief, left as-is.
  - The dashboard's jdi status badge and its sidecar plumbing
    (job/jdistatus.go, launch.Jdi): already implemented, untouched.
  - Keeping `k` for up-navigation on the dashboard: a valid alternative to
    TASK-2 — if the human prefers it, strike TASK-2 and leave
    `case "up", "k":` alone.
