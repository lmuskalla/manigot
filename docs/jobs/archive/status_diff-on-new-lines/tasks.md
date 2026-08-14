# Tasks: diff on new lines

id: status
status: open
analyst: @analyst
date: 2026-08-14

<!-- Produced by @analyst from brief.md. -->

## Root cause

The TUI detail view's computed "diff" tab (`internal/ui/detail.go` `loadDiff`)
builds its content as `git log --oneline <base>...<branch>` lines followed by
`git diff --stat <base>...<branch>` lines — one commit / one file per line —
and hands that **plain text** to the shared `markdown.Viewer`, which renders it
through glamour as if it were markdown.

Glamour (v0.8.0) treats consecutive non-blank lines as a single markdown
*paragraph*: `ansi/paragraph.go` runs the paragraph through
`wordwrap.NewWriter` with `KeepNewlines = PreserveNewLines` (false by default;
manigot's `rendererFor` never enables `WithPreservedNewLines`), so the lines are
joined with spaces and re-wrapped at the tab width. On a wide terminal several
commits (and several stat entries) land on the **same rendered line** — exactly
the reported "prints changes on the same lines". Git output is not markdown, so
feeding it to a markdown renderer is the bug.

The CLI `mg diff` (`cmd/mg/diff.go`) prints the same raw strings via
`fmt.Fprintln` with no markdown step, so it is **not** affected — the brief's
"might also apply to the cli one" needs to be verified and pinned by a test,
not fixed.

## Fix approach (recommended)

Render the diff tab's content as literal/verbatim text that preserves each
source line and only wraps lines longer than the viewport width — not through
glamour. Concretely: add a plain-text render path to `internal/markdown`
(e.g. `RenderPlain(src, width)` and/or a raw mode on `Viewer`) built on
lipgloss `NewStyle().Width(w).WordWrap()` (which wraps per line and preserves
existing `\n`, keeping stat column alignment), and use it for the `isDiff`
tab only. The four markdown job tabs keep the glamour path untouched.

Considered and rejected as primary: wrapping the diff content in a fenced code
block — glamour preserves code-block lines but does **not** wrap them, so long
lines would be truncated on narrow terminals instead of wrapped, and the
content would pick up code-block styling.

## Out of scope (flagged)

The "log" tab (mg-jdi `run.log` tail) feeds plain text through the same
`markdown.Viewer` and has the same latent reflow issue. The brief is only about
`mg diff`, so it stays out of scope here — note it as a follow-up in
`implementation.md` / for @owner.

## Task breakdown

TASK-1: Add a failing regression test pinning "one change per line" in the
rendered TUI diff tab: a job branch with 2+ commits and 2+ changed files,
rendered at a wide width, must show each commit subject on its own rendered
line and each diff --stat entry on its own line (today the glamour paragraph
reflow merges them).
     files: internal/ui/detail_test.go
     depends: none
     risk: low — test-only, reuses the existing scratch-repo diff-tab test
           helpers (gitInitRepo / addJobWorktree / newDetailView).

TASK-2: Add a plain-text (verbatim) render path to internal/markdown — e.g.
`RenderPlain(src, width)` plus a raw mode on `Viewer` (constructor flag or
setter) — that bypasses glamour, preserves every source line, and wraps only
lines exceeding the width (lipgloss `NewStyle().Width(w).WordWrap()`, which
keeps existing newlines and leading spaces); cover with unit tests.
     files: internal/markdown/markdown.go, internal/markdown/markdown_test.go
     depends: none
     risk: low-medium — additive only; must not change the glamour path used
           by the four markdown job tabs (glamour still handles those).

TASK-3: Wire the diff tab to the plain path: in internal/ui/detail.go make the
`isDiff` tab's viewer render verbatim (flag it in newDetailView so loadTab /
ensureCurrentSized / resize flow through untouched), and keep the "No changes
on ..." and error placeholders single-line plain text. TASK-1's test must
pass; the four markdown tabs and the log tab must render exactly as before.
     files: internal/ui/detail.go
     depends: TASK-1, TASK-2
     risk: medium — touches the shared Viewer wiring; the diff tab currently
           renders through the same viewer as the markdown tabs, so the raw
           mode must be strictly per-tab and degrade to the old path if it
           errors.

TASK-4: Verify and pin that the CLI `mg diff` already shows one change per
line: add explicit line-structure assertions to TestRunDiffOutput in
cmd/mg/diff_test.go (each commit subject on its own output line, each stat
entry on its own line), so the brief's "might also apply to the cli one" is
answered by a test. No behavior change expected in cmd/mg/diff.go.
     files: cmd/mg/diff_test.go
     depends: none
     risk: low — test-only, extends the existing runDiff output test.

TASK-5: Regression sweep + docs sync: run `go test ./...`; update any existing
test that asserted glamour styling or line-merging on the diff tab; and check
whether docs/GIT_DIFF.md / docs/AGENTS.md should note the diff tab's
one-change-per-line rendering — if docs/AGENTS.md changes, sync
project-template/docs/AGENTS.md and agents/*.md per the hard rule.
     files: (whatever the sweep turns up), docs/GIT_DIFF.md, docs/AGENTS.md,
            project-template/docs/AGENTS.md (only if behavior text changes)
     depends: TASK-3, TASK-4
     risk: low — verification and doc touch-ups only, no behavior change.
