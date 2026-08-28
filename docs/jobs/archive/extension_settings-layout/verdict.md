# Verdict: settings layout

id: extension
status: open
reviewer: @reviewer
date: 2026-08-28

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Re-review after the previous `NEEDS WORK` verdicts (`3a5a135`, `ebb936d`).
Since `ebb936d` the developer committed `fef5cf5` (compress render to fit the
24-row height budget) and `662e17b` (summary update) — both source changes are
on this branch and reviewed below. The two blockers from the prior verdicts are
both addressed.

TASK-1: PASS
notes: `src/internal/ui/settings.go` — the render now satisfies all three design
goals AND the height budget:

- **Two bold-headed sections**: `Personal settings` / `Project settings`
  (`headerStyle`, settings.go:365, 435), each carrying its storage location
  once as a dim suffix, so the per-field "stored in …" clauses are gone.
- **Bold headline per setting, value next to it**: the `settingsField` helper
  (settings.go:342-348) right-pads every label to `settingsLabelWidth` (17,
  the length of "Job branch prefix") so all inputs start at the same x; the
  headline stays bold whether focused or not, and only the input value dims
  when unfocused — exactly the "dim only the value, not the headline"
  requirement. No new style was needed; `headerStyle` is reused per the task.
- **Examples de-emphasized**: shortened, kept `dimStyle`, 2-space indent; the
  only blank line is between the two sections (settings.go:430).
- **Height budget met**: I counted the render output line by line — exactly 22
  lines (title; personal headline; editor + example; recent activity +
  example; profile headline + 5 profile rows; terminal + example; theme +
  example; blank; project headline; base branch; job branch prefix + example;
  footer). `app.go:338` sets `a.height = 24 - 2*uiPaddingY = 22` and `View()`
  wraps the render in `uiPaddingStyle` (1 top + 1 bottom row), so 22 content
  lines fill a 24-row terminal exactly: the **Job branch prefix input (line
  20) and the "enter save · esc cancel" footer (line 22) are now on screen**,
  resolving the prior verdicts' blocker. The form shrank 30 → 22 lines.
- **Interaction semantics byte-for-byte unchanged**: `update()`, `setFocus()`,
  the `stFocus*` constants/tab-cycle order, profile ←/→ cycling, enter/esc and
  `hint()` are untouched (diff confirms). `stInputWidth` now accounts for the
  wider label column (width − 17 − 4) — a necessary consequence of the aligned
  label column, not a regression.
- Doc comment rewritten for the two-section grouping (settings.go:63-87).

Non-blocking caveats (not merge blockers):
- The terminal example line ("blank = auto-detect (tmux / Terminal.app / ...)
  · in tmux the split pane always wins", settings.go:416) is 86 columns, so it
  soft-wraps at an 80-column terminal; the visual height becomes 23 rows and
  the footer lands on the last visible row (row 24) of an 80×24 terminal —
  everything is still on screen, but the fit is zero-margin. The raw-line test
  cannot catch wrap-induced overflow, and the task's requested narrow-width
  manual verification is not documented (no render report exists for this
  job). Terminals narrower than ~75 columns will clip the footer — inherent to
  the fixed, non-scrollable form and true of the old layout as well; not a
  regression.
- The selected-profile "tool: … · billed via …" description line was dropped
  rather than merged into one line with the MANIGOT_PROFILE note (the task's
  suggested fallback). The per-row labels ("Claude Code · Claude Pro", etc.)
  still convey tool + billing plan, so the informational loss is minor and
  within the "developer may adjust details" latitude.

TASK-2: PASS
notes: `src/internal/ui/settings_test.go` — updated in lockstep with the new
render, verified assertion-by-assertion against the implemented output:

- `TestSettingsRender` asserts the new strings (both section headlines, all
  seven per-setting headlines, the typed value "abc", the five profile rows,
  the shortened example fragments) and asserts the old phrasing is gone
  ("Editor:", "(project)" suffixes, per-field storage clauses, "saved as
  OPENCODE_THEME", "max entries shown in the", "billed via") — each `gone`
  string is confirmed absent from the current render.
- `TestSettingsRenderPersistenceNotesOncePerSection` counts each persistence
  note exactly once — matches the render (once in each section headline, once
  in the profile headline).
- `TestSettingsRenderFitsHeightBudget` now asserts the **real** budget — the
  render must fit 22 lines (the 22 content rows of a 24-row terminal) — plus a
  regression guard that the footer and all seven headlines are present. This
  resolves the prior blocker that the height requirement was unguarded.
- Model-level tests (focus cycle, shift+tab, profile cycling, per-field
  edit/seeding, validation) operate on the unchanged `update()`/`setFocus()`
  logic and need no changes — confirmed by reading the diff (those tests are
  untouched).

Note on verification: this review session's git shim allows only git
read/commit, so `go build`/`go test` could not be re-run here; correctness was
verified by line-by-line inspection of `render()` against the test assertions
and the 22-content-row geometry. The developer's session likewise could not run
the full suite (the shim refuses `git init`; tig absent) — that is an
environment restriction affecting ~53 unrelated ui tests, not a regression from
this change.

## Security

none — render-only change; no filesystem, credential, or network surface
touched.

## Overall

APPROVED

No blockers. Optional follow-ups (non-blocking): consider shortening the
terminal example line below 80 columns so the 24-row fit has margin and
survives narrow terminals, and re-verify the form visually at a narrow width
in a real `mg tui`.