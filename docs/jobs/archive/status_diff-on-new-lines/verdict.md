# Verdict: diff on new lines

id: status
status: open
reviewer: @reviewer
date: 2026-08-14

## Review

Reviewed `git diff main...HEAD` (base branch `main` per `.manigot/manigot.json`)
against the brief and the five tasks in `tasks.md`. The reported bug — the
TUI detail view's computed diff tab merging several commits/stat entries onto
one rendered line via glamour's paragraph reflow — is correctly root-caused
and fixed: the diff tab now renders verbatim through a new plain-text path
(`RenderPlain` + `Viewer` raw mode), so each `git log --oneline` line and each
`git diff --stat` line keeps its own rendered line, with only over-width lines
word-wrapped. The CLI `mg diff` was confirmed (and pinned by test) to already
print one change per line — no markdown step there — so the brief's "might
also apply to the cli one" is answered by a test, not a fix.

TASK-1: PASS
  `TestDetailDiffTabOneChangePerLine` (internal/ui/detail_test.go:689)
  builds a real scratch repo with a job worktree, 2+ commits touching 2+
  files, renders the diff tab at a wide 160x40 viewport, and asserts each
  commit subject and each stat entry sits on its own rendered line. Reuses
  the existing gitInitRepo / addJobWorktree / newDetailView helpers as
  specified. Reasoning confirms it fails against the old glamour path (the
  paragraph of 3 short log lines and the paragraph of 3 short stat lines
  each fit on one 156-wide line) and passes against the raw path. Note: the
  stat assertions were corrected in the TASK-3 commit from an exact
  `"one.txt |"` match to a `path\s+\|` regex, because git pads the stat path
  column to a fixed width — the final state is correct and complete.

TASK-2: PASS
  `RenderPlain(src, width)` (internal/markdown/markdown.go:143) preserves
  every source line and leading spaces (git diff --stat column alignment),
  wraps only lines exceeding the width, and drops trailing whitespace-only
  lines; empty input renders to nothing. `Viewer.SetRaw` (markdown.go:192)
  is a per-viewer mode switch that re-renders only when content exists; the
  glamour path (`Render`, `rendererFor`, cache, style handling) is untouched
  — `rebuild()` branches on `v.raw` with the markdown branch byte-for-byte
  identical to before. Unit tests (markdown_test.go:196-365) cover the
  required cases end to end. The one deviation from the task text — using
  `lipgloss.NewStyle().Width(w)` rather than the suggested
  `NewStyle().Width(w).WordWrap()` — is legitimate and documented:
  lipgloss v1.0.0 (per go.mod) removed `WordWrap`, and `Width` performs the
  same word wrapping at render time; the task's own recommended approach is
  preserved.

TASK-3: PASS
  The diff tab's viewer is constructed with `SetRaw(true)` in
  `newDetailView` (internal/ui/detail.go:128-134), so `loadTab` /
  `ensureCurrentSized` / `resize` / `syncViewerSize` flow through untouched
  and the raw mode is strictly per-tab: the four markdown job tabs and the
  log tab keep the glamour path (verified — no other `NewViewer` call site
  gets `SetRaw`). The "No changes on ..." and error placeholders remain
  single-line plain text. Also correctly guards the empty-content case in
  `SetRaw` (no rebuild on `src == ""`), avoiding a phantom rendered line.

TASK-4: PASS
  `TestRunDiffOutput` (cmd/mg/diff_test.go:129-131) now asserts each commit
  subject and each stat entry appears on its own output line via
  `assertEachEntryOnOwnLine`. `cmd/mg/diff.go` is unchanged — the CLI
  already prints one change per line, so the brief's question is answered by
  a test as specified.

TASK-5: PASS
  `go test ./...` / `go vet ./...` claimed green; a manual sweep confirms no
  existing test asserted the old merged rendering (existing diff-tab tests
  assert raw `t.content` and would pass either way). `docs/AGENTS.md`'s TUI
  section now documents the one-change-per-line verbatim rendering. The
  hard-rule sync check is correct: neither `agents/*.md` nor
  `project-template/docs/AGENTS.md` describe the TUI diff tab, so no sync
  was needed.

Commit discipline: PASS — one commit per task in the `[status] TASK-N: ...`
format (eb59052, f08d798, 36188e7, 3a48138, 807a21a) plus the separate
`[status] implementation: add summary` commit (df046fa). The analyst's
completed tasks.md was swept into the TASK-1 commit; minor hygiene, not a
blocker.

Scope: PASS — the diff touches exactly the files tasks.md lists
(internal/markdown, internal/ui/detail, cmd/mg/diff_test.go, docs/AGENTS.md)
plus the job's own docs. No unrelated refactors.

## Security

none — the change adds a plain-text rendering path and per-tab viewer mode;
no new inputs, network, or privilege boundary.

## Overall

APPROVED

No blockers. The fix matches the brief (one change per line in the TUI diff
tab, CLI verified by test), is strictly scoped to the diff tab, leaves the
glamour path and the log tab untouched, and is pinned by regression tests at
both the unit level (RenderPlain/Viewer) and the integration level (detail
view end-to-end against a real scratch repo). Known follow-up, correctly
flagged out of scope: the log tab (mg-jdi run.log tail) has the same latent
paragraph-reflow issue and can later switch to `SetRaw(true)` the same way.
