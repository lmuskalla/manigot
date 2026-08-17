# Implementation: remove jk keys from git modal

id: one
status: open
developer:
date: 2026-08-17

<!-- Produced by @developer after implementation. -->

## Summary

Removed the vim-style `j`/`k` navigation keys from the TUI's git panel modal,
leaving `↑`/`↓` (plus enter/esc/q) as the only navigation keys, per the
project's earlier decision to not use vim keys to navigate. The key handling,
the doc comments, the rendered footer hint, the navigation test, and the
README documentation were all updated to match.

## Changes

TASK-1: `internal/ui/gitpanel.go` — dropped `"k"` from the `"up"` case and
`"j"` from the `"down"` case in `update()`, and updated the two doc comments
(gitPanelView's description and update's key contract) that advertised
`↑/↓/k/j` navigation. The panel's dispatch (gpSubmit/gpCancel) is untouched.

TASK-2: `internal/ui/gitpanel.go` — the rendered footer key hint in `render()`
changed from `↑/↓/k/j navigate · enter run · esc/q cancel` to
`↑/↓ navigate · enter run · esc/q cancel`.

TASK-3: `internal/ui/gitpanel_test.go` — rewrote `TestGitPanelNavigation` to
drop the `j`/`k` movement steps and instead regression-pin them as no-ops:
pressed mid-list where a bound `j`/`k` would have moved the cursor, they now
leave the selection alone, with a `↑/↓` contrast step at the end — mirroring
the list view's `TestListJAndKNoLongerMoveCursor` precedent. The test's
comment was refreshed accordingly.

TASK-4: `README.md` — the git panel paragraph's "moved through with
`↑`/`↓`/`k`/`j`" now reads "moved through with `↑`/`↓`" only. No other README
section describes the panel's navigation keys (the remaining `j` references
are the mg-jdi launch key, which is unrelated).

## Known issues / follow-ups

- The agents picker (`internal/ui/agentspicker.go`) and the shared `Picker`
  (`internal/ui/picker.go`) still bind `j`/`k` — deliberately out of scope per
  the brief (git modal only). A potential follow-up, to be raised with the
  human.
- The session's git shim refuses `git init`/`git worktree`, which some
  `internal/ui` tests use to build fixtures; running the suite with the real
  git ahead of the shim on `PATH` (`PATH=/usr/bin:/bin:$PATH go test ./...`)
  passes cleanly. Purely an agent-session artifact, not a code issue.