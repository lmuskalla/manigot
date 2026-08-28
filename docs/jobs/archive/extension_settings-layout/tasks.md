# Tasks: settings layout

id: extension
status: open
analyst: @analyst
date: 2026-08-28

<!-- Produced by @analyst from brief.md. -->

## Scope summary

The brief: the mg tui settings form has grown to seven fields (editor, base
branch, job branch prefix, recent activity count, profile, terminal, theme)
and now reads as one flat wall of label+input+long-dim-example triples. The
author wants more separation, bold headlines per setting, the value next to
(or below) the headline, and the example text less visually present.

No render report / screenshots exist in this job dir, and the settings view
is a plain `a.settingsView.render()` string (no viewport/scroll — see
`app.go` `View()`'s `stateSettings` branch), so the developer must verify
fitted height by hand in a real `mg tui` session, not via a scroll test.

### Confirmed current render shape (from `settings.go` `render()`)

- Title `Settings`, then seven flat blocks, each: `  Label: [input]` on one
  line (plain when focused, `dimStyle` when not), then a long
  `dimStyle` example line indented 11 spaces, then a blank line.
- The profile block is taller: `  Profile:` label + one row per profile
  (5) + two dim description lines.
- Focus state is conveyed by dimming the whole row (label + input together),
  which is part of why the form feels heavy: six rows are dimmed at once.
- Examples mix three kinds of info per line: the default/placeholder, a
  usage note, and the persistence location ("stored in ...").

### Recommended design (analyst's proposal — the developer may adjust
details, but the three goals are: separation, bold headlines, de-emphasized
examples)

1. **Group the seven settings into two sections with bold section
   headlines**, matching the storage split the code already documents
   (`settings.go`'s `settingsView` doc comment):
   - **Personal settings** — editor, recent activity, profile, terminal,
     theme (global: `config/tui-settings.json` + `manigot/.env`).
   - **Project settings** — base branch, job branch prefix (committable
     `.manigot/manigot.json`, shared with the team).
   The existing "(project)" label suffixes can then be dropped — the section
   headline carries that meaning.
2. **Bold headline per setting** (`headerStyle` or a new
   `settingsLabelStyle`), with the value/input **next to it** (keep
   label+input on one line — going full "value below" costs one extra line
   per field, which the form cannot afford, see height note below).
   Keep the focused/unfocused distinction, but **dim only the input value
   when unfocused, not the headline** — a bold headline that dims to faint
   defeats the purpose.
3. **Examples less visually present**: shorten them (drop the
   persistence-location clause — that belongs once per section, e.g. in the
   section headline or a single dim note under it), keep them `dimStyle`,
   and reduce vertical noise (e.g. no blank line after every example; a
   blank only between sections).

### Height constraint (important)

The form already renders ~30 lines at 80×24 test fixtures and is **not
scrollable** — `stateSettings` renders the raw string, so anything below the
terminal's last row is silently cut off. The redesign must not grow the
form's line count, and should ideally shrink it: adding two section
headlines (~+2 lines) must be paid for by dropping redundant blank lines and
shortening example lines. Verify in a real terminal at 24 rows (and a
narrow-ish width) after implementing.

## Task breakdown

TASK-1: Restructure the settings form render into two bold-headed sections
(Personal / Project) with a bold headline per setting, the value next to the
headline, and shortened, less visually present example lines.

- Concrete shape to aim for (not a spec, adjust as long as the three goals
  hold):
  ```
  Settings

  Personal settings

  Editor                [input........]
    blank = use $VISUAL / $EDITOR / nano / vi

  Recent activity       [input........]
    1–100 · blank = 5 · dashboard recent activity strip

  Profile
    ▸ claude-pro   Claude Code, billed to Claude Pro/Max
    ...
    saved as MANIGOT_PROFILE in manigot/.env — shared with the CLI

  Terminal              [input........]
    blank = auto-detect (tmux / Terminal.app / ...) · in tmux the split pane always wins

  Theme                 [input........]
    blank = OpenCode's own default · OpenCode only

  Project settings

  Base branch           [input........]
    blank = main

  Job branch prefix     [input........]
    blank = feature/… · namespace for job branches

    enter save · esc cancel
  ```
- Keep every field's label text intact enough that the field is still
  recognizable (the existing `TestSettingsRender` asserts substrings like
  "Editor:", "Base branch", "Recent activity:", "Terminal:", "Theme:",
  "OPENCODE_THEME", "recent activity strip" — TASK-2 updates those
  assertions to the new strings, so do not treat them as frozen).
- Keep all interaction semantics byte-for-byte: tab/shift+tab cycle order
  (`stFocus*` constants), profile ←/→ cycling, enter/esc actions, `hint()`
  footer, focus highlighting of the *input value* when its field is focused.
- Reuse existing styles where possible (`headerStyle` for section + setting
  headlines); add a small style only if needed (e.g. a
  `settingsLabelStyle`). Do not introduce boxes/borders — the codebase's
  house style is plain text + dim + accent (see `styles.go`).
- Update the `settingsView` doc comment to describe the new two-section
  grouping.
files: `src/internal/ui/settings.go` (render, doc comment; possibly a new
style in `src/internal/ui/styles.go`), `src/internal/ui/settings_test.go`
(updated by TASK-2, not here)
depends: none
risk: medium — the render is asserted by several existing tests that TASK-2
must update in lockstep; the height budget (24 rows) must be held while
adding two section headlines; dim-only-the-value focus logic is a small
behavior change that could subtly regress the focused/unfocused look.

TASK-2: Update `settings_test.go` for the new layout and extend coverage of
the new structure.

- Update `TestSettingsRender`'s asserted substrings to the new render
  output: section headlines ("Personal settings", "Project settings"),
  each setting's label, the typed value, the shortened example text (drop
  assertions on removed persistence-location clauses like "stored in
  config/tui-settings.json" unless the new design keeps them), and the
  profile rows.
- Add assertions that both section headlines render, and that the
  persistence-location note appears only once per section (not per field)
  if that is where the design puts it.
- Re-run the whole `ui` package suite: focus-cycle tests
  (`TestSettingsTabCyclesFocus`, `TestSettingsShiftTabCyclesFocusBackward`),
  profile cycling, and the per-field edit/seeding tests all operate on the
  model, not the render, so they should pass unchanged — confirm, don't
  assume.
files: `src/internal/ui/settings_test.go`
depends: TASK-1 (asserts TASK-1's actual output strings)
risk: low — test-only, but must land in the same commit as TASK-1 or the
suite goes red; the risk is picking assertion strings that don't match
TASK-1's final render, so write them from the implemented output, not from
this proposal's sketch.

## Notes for the developer

- Build and run the full suite from the module root: `cd src && go build
  ./... && go test ./...`. The TUI lives in `src/internal/ui`.
- The form is not scrollable and already sits at the edge of a 24-row
  terminal — after implementing, run `mg tui`, open settings (`s`), and
  check at 24 rows and at a narrow width that the footer ("enter save · esc
  cancel") is still visible and nothing important is cut off. This is a
  manual check; there is no render-report artifact for this job.
- Do not reorder the fields or change the tab-cycle indices — several tests
  and the `stFocus*` constants pin the order, and reordering is not what the
  brief asks for.
- The profile block is the tallest section; if the height budget is tight,
  the two dim description lines under the profile list are the first
  candidates to shorten (merge "tool/billed via" and the "saved as
  MANIGOT_PROFILE" note into one line), not the section separation.
- Out of scope: `mg theme`, `mg profiles`, the new-job form, the list/detail
  views, and any settings persistence logic — this is a render-only change.