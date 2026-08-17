# Verdict: remove jk keys from git modal

id: one
status: open
reviewer:
date: 2026-08-17

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: internal/ui/gitpanel.go — `case "up"` (line 78) and `case "down"` (line 82) in `update()` dropped the `"k"`/`"j"` alternatives; only ↑/↓ move the cursor (clamped to the three rows). Doc comments at lines 47 and 70 updated from `↑/↓/k/j` to `↑/↓`. Dispatch (gpSubmit/gpCancel) untouched. Verified at the App level too: stateGitPanel routes every key through `updateGitPanel` (app.go:448-449, 897-898), so j/k are no-ops end to end while the panel is open.

TASK-2: PASS
notes: internal/ui/gitpanel.go line 115 — footer hint is now `↑/↓ navigate · enter run · esc/q cancel`. TestGitPanelRender and TestGitPanelOverlayKeepsDetailVisible assert only the substrings "enter run"/"esc/q cancel", both still present in the new hint, so no test breakage.

TASK-3: PASS
notes: internal/ui/gitpanel_test.go — TestGitPanelNavigation rewritten: drops the j/k movement steps, adds mid-list regression presses of `j` and `k` asserting the cursor stays put (lines 118-127), and ends with a ↑/↓ contrast step (lines 129-133). Test comment refreshed (lines 87-91). Mirrors the TestListJAndKNoLongerMoveCursor precedent (list_test.go:574-624). The push/merge tests move the cursor with `down` arrows and are unaffected. Test logic verified by inspection against the new `update()` — all cursor transitions and clamps check out.

TASK-4: PASS
notes: README.md line 771 — "moved through with `↑`/`↓`/`k`/`j`" → "moved through with `↑`/`↓`". No other README section documents the git panel's navigation keys: the keybinding-table `g` row (line 735) names no keys, and the `j` row (line 730) is the mg-jdi launch key, unrelated and correctly untouched. docs/AGENTS.md does not document git-panel keys, so leaving it alone is correct.

Scope: PASS — diff confined to README.md, internal/ui/gitpanel.go, internal/ui/gitpanel_test.go, and the job's own docs (brief/tasks/implementation/verdict). No out-of-scope refactors. The agents picker and shared Picker still bind j/k, deliberately out of scope per the brief and documented in tasks.md/implementation.md.

Commit discipline: PASS — one commit per task in `[one] TASK-N:` format (6d22af8 TASK-1, 5775b26 TASK-2, ce052e8 TASK-3, 7791dba TASK-4), implementation.md has its own commit (ddc78ca). Working tree clean.

## Security

none — no security-sensitive code touched; the change is a key-binding removal plus docs/tests.

## Overall

APPROVED

All four tasks are implemented exactly as specified. Nothing needs to change before merge.