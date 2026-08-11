# Verdict: Add jdi in dashboard

id: b8kbwb
status: open
reviewer: deepseek-v4-flash
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Verified on branch `feature/b8kbwb_add-jdi-in-dashboard` (correct branch for
this job). Reviewed `git diff main...HEAD` in full and cross-referenced every
change against tasks.md. `go build ./...`, `go vet`, and `go test ./...`
all pass from `tui/`; gofmt clean.

TASK-1: PASS
notes: `tui/internal/ui/app.go` — new `case "j":` in `updateList` (lines
692–722) faithfully mirrors `updateDetail`'s "j" case: `selectedJob()` guard
(no-op when the list is empty, and covers an out-of-range cursor too), the
`jdiAlreadyRunning` sidecar-then-jdiSeen guard with the `@<agent>`-naming
status message, `launch.Jdi(j.ID, a.root, a.settings.ProfileValue())`,
`cmdErrorText` on launch failure, `jdiSeen`/`jdiSeenAt` seeding on success,
footer status "→ mg jdi started in the background — see the list badge"
(worded for the list context, no log tab), and `return a,
a.startSpinnerIfRunning()`. The mandatory coupled change — `"j"` removed from
`case "down", "j":` (now `case "down":`) — landed in the same commit
(02ff632), so the duplicate-case compile hazard the task warns about is
avoided. `launch.Jdi` signature `(jobID, projectRoot, profile string)` matched
correctly.

TASK-2: PASS
notes: `tui/internal/ui/app.go` line 664 — `case "up", "k":` → `case "up":`.
Committed separately (231e307) right after TASK-1 as the dependency requires.
The default option was implemented (drop `k`), as disclosed in
implementation.md; the tasks.md out-of-scope alternative (keep `k`) was a
judgment call defaulting to the task's primary wording, and no human input was
available — reasonable. Verified no test drove list navigation via "k"/"j"
before the change (the full suite passed after removal).

TASK-3: PASS
notes: `tui/internal/ui/app.go` line 1294 — hint is exactly
"↑/↓ navigate · enter view · j mg-jdi · o quick · a agent · n new · s settings
· ctrl+r refresh · q quit", matching the task's example. Existing footer tests
(`TestListFooterKeepsHintAlongsideStatus`, agentspicker footer assertions)
only assert on preserved substrings and still pass.

TASK-4: PASS
notes: `tui/internal/ui/jdilaunch_test.go` (5 new tests driving `updateList`
via the shared `gitInitRepo`/`addJobWorktree`/`markerStub`/
`waitForMarkerRuns`/`countMarkerRuns`/`keyMsg` helpers) and
`tui/internal/ui/list_test.go` (1 regression test). Coverage matches the spec
point for point: launch-from-list (stub runs, jdiSeen seeded to JDIRunning,
footer status mentions "mg jdi" and "list badge", spinner cmd non-nil, state
stays stateList), resolution failure (MANIGOT_JDI_BIN empty → status contains
"not found", jdiSeen not seeded), already-running block via on-disk sidecar
(writes job.JDIRunning + "developer" → status "already running @developer",
stub never invoked), block via in-session dedup fallback (two presses, no
sidecar written by stub → second blocked), empty-list no-op (no cmd, no
status, empty jdiSeen), and the cursor regression (j/k leave the cursor put
while up/down still move). All 6 run green; the regression test's "j" press
exercises a real successful launch (stub) proving even a launch doesn't move
the cursor.

TASK-5: PASS
notes: `README.md` — list-view keybindings table line 602 drops `k`/`j` from
the move-selection row and line 604 gains the `j` row ("run `mg jdi` against
the selected job, detached in the background — watch via the list's status
badge", with working anchor links — both `#autonomous-mode-mg-jdi` and
`#mg-jdi-status--log` resolve); "mg jdi status & log" section line 694 now
reads "Press `j` in the list (on the selected job) or in the detail view".
All documented occurrences caught; no stale references remain.

TASK-6: PASS
notes: `docs/AGENTS.md` (the canonical source, not the read-only mount) —
the `tui/cmd/jdi` bullet now reads "(`j` from the list or the detail view)".
Independently verified `agents/*.md` and `project-template/docs/AGENTS.md`
contain no reference to the `j`-in-detail-view detail, so the no-sync-change
claim is accurate.

Scope: clean. Only app.go, the two test files, README.md, and docs/AGENTS.md
changed in code/docs; nothing outside tasks.md. The agents picker's own
`"up","k"`/`"down","j"` cases and the detail view's `j` handler are untouched
(confirmed out of scope). `brief.md` diff is the human's own TUI edit; the
`tasks.md` change swept into the TASK-1 commit is the analyst's pre-existing
uncommitted edit, disclosed in implementation.md — content is identical to the
planned breakdown, no functional impact.

Commit discipline: one commit per task in `[b8kbwb] TASK-N: description`
format (02ff632, 231e307, 6bf937a, 93f41bd, c68c6f2, 633a160), plus the
implementation.md summary commit (e9dcc0c). Correct.

## Security

No security review requested for this job; the change adds a keybinding that
invokes the already-reviewed `launch.Jdi` path (same call the detail view
makes) — no new surface. Status strings and error text are rendered via the
existing styled/dim rendering, no injection concern.

## Overall

APPROVED

All six tasks implemented exactly as specified, tests green, scope clean.
No blockers.
