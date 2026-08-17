# Tasks: remove jk keys from git modal

id: one
status: open
analyst:
date: 2026-08-17

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Remove the `k`/`j` key bindings from the git panel's `update()` in `internal/ui/gitpanel.go`, leaving only `↑`/`↓` (plus enter/esc/q unchanged); update the doc comments (lines 47, 70) that advertise `↑/↓/k/j` navigation.
     files: internal/ui/gitpanel.go
     depends: none
     risk: low — a two-line key-case removal plus comment text; the panel's dispatch (gpSubmit/gpCancel) is untouched.

TASK-2: Update the git panel's footer key hint from `↑/↓/k/j navigate · enter run · esc/q cancel` to `↑/↓ navigate · enter run · esc/q cancel` in `render()`.
     files: internal/ui/gitpanel.go
     depends: TASK-1 (same file, but the hint text is independent of the key handling)
     risk: low — a single string literal; check tests that assert footer content (TestGitPanelRender, TestGitPanelOverlayKeepsDetailVisible) for the exact substring.

TASK-3: Update `TestGitPanelNavigation` in `internal/ui/gitpanel_test.go` to drop the `j`/`k` movement steps and add a regression assertion that `j` and `k` are now no-ops (cursor unmoved), mirroring the list view's `TestListJAndKNoLongerMoveCursor` precedent; refresh the test's comment.
     files: internal/ui/gitpanel_test.go
     depends: TASK-1 (the test's j/k expectations change with the behavior)
     risk: medium — the test must flip from asserting j/k move the cursor to asserting they do not, and the merge/push tests use `down` arrows (unaffected), but the navigation test needs a careful rewrite.

TASK-4: Update the git panel documentation in `README.md` (line 771) from "moved through with `↑`/`↓`/`k`/`j`" to arrows only, so the docs match the implemented behavior.
     files: README.md
     depends: none (documentation; can land with or before the code)
     risk: low — a one-line prose edit; verify no other README section describes the panel's keys.

NOTE: The agents picker (`internal/ui/agentspicker.go`) and the shared `Picker` (`internal/ui/picker.go`) still bind `j`/`k` — the brief explicitly scopes this job to the git modal, so those are deliberately out of scope here (a potential follow-up, to be raised with the human).