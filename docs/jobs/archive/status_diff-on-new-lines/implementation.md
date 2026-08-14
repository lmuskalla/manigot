# Implementation: diff on new lines

## Summary

Fixed the TUI detail view's computed "diff" tab printing several changes on
the same rendered line. The tab's content — `git log --oneline` +
`git diff --stat` output, plain text — was being fed through glamour's
markdown renderer, which treats consecutive non-blank lines as one paragraph
and joins/re-wraps them, so on a wide terminal multiple commits and stat
entries landed on a single line. The diff tab now renders verbatim: every
source line is preserved and only lines exceeding the viewport width are
wrapped, so each commit subject and each stat entry gets its own rendered
line. The CLI `mg diff` was verified (and pinned by a test) to already print
one change per line — it has no markdown step.

## Changes

TASK-1: Added a failing regression test (`TestDetailDiffTabOneChangePerLine`
in `internal/ui/detail_test.go`) — a job branch with 2 commits touching 2
files, rendered at a wide viewport, must show each commit subject and each
diff --stat entry on its own rendered line. Failed against the old glamour
path (3 commits and 3 stat entries each merged onto one line); passes after
TASK-3.

TASK-2: Added a plain-text render path to `internal/markdown`:
- `RenderPlain(src, width)` — verbatim rendering via lipgloss `Width` wrap
  (lipgloss v1.0.0's equivalent of the removed `Style.WordWrap`; `Width`
  alone wraps at render time through `ansi.Wrap`), preserving every source
  line and leading spaces (so `git diff --stat` column alignment survives),
  wrapping only lines longer than the width, and dropping trailing
  whitespace-only lines. Empty input renders to nothing.
- `Viewer.SetRaw(raw)` — per-viewer mode switch; a raw viewer renders via
  `RenderPlain` instead of glamour. A no-op when the mode is unchanged; a
  viewer with no content yet just records the mode (rebuilding there would
  fabricate a phantom empty line). The glamour path for markdown content is
  untouched.
- Unit tests in `internal/markdown/markdown_test.go` covering one-change-per-
  line, leading-space preservation, long-line wrapping, empty/trailing-
  newline inputs, the raw Viewer end to end, SetRaw re-rendering, and
  SetRaw-before-content.

TASK-3: Wired the diff tab to the raw path in `internal/ui/detail.go`: the
`isDiff` tab's viewer is constructed with `SetRaw(true)` in `newDetailView`,
so `loadTab` / `ensureCurrentSized` / `resize` flow through untouched and the
raw mode is strictly per-tab. The four markdown job tabs and the log tab keep
the glamour path, and the "No changes on ..." / error placeholders render as
single-line plain text.

TASK-4: Pinned that the CLI `mg diff` already shows one change per line by
adding explicit line-structure assertions to `TestRunDiffOutput` in
`cmd/mg/diff_test.go` (each commit subject on its own output line, each stat
entry on its own line) — answering the brief's "might also apply to the cli
one" with a test. No behavior change in `cmd/mg/diff.go`.

TASK-5: Regression sweep — `go test ./...` and `go vet ./...` pass across
all packages with no stale tests (existing diff-tab tests assert raw content,
not the old merged rendering). Documented the diff tab's verbatim
one-change-per-line rendering in `docs/AGENTS.md`'s TUI section (no sync
needed: neither `agents/*.md` nor `project-template/docs/AGENTS.md` describe
the TUI diff tab).

## Known issues / follow-ups

- The "log" tab (mg-jdi `run.log` tail) feeds plain text through the same
  `markdown.Viewer` and has the same latent paragraph-reflow issue. It was
  flagged out of scope by the task breakdown; a follow-up could render it
  with `SetRaw(true)` the same way.
- `RenderPlain` is built on lipgloss v1.0.0's `Width` wrap rather than the
  `Style.WordWrap()` the task text suggested, because lipgloss v1.0.0 removed
  `WordWrap` — `Width` performs the same word wrapping at render time.
