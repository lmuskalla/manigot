# Brief: tui visual improvements

status: done
type: feature
id: 78fgoq
branch: feature/78fgoq_tui-visual-improvements
date: 2026-08-09
author: Leander Muskalla

## What

Have a look at the two screenshots in the job dir. They show the start page and the detail page for a job.
Our TUI is coming along nicely, but it feels like a first dummy iteration, not like a production-ready interface.
Have a look, analyze and please make suggestions how to improve this.
Let's make this the most pleasing and easy-to-use interface we can. If it's beautiful, that's good as well!

> **Unresolved as of this revision:** the two screenshots referenced above are
> not present in this job's directory (`docs/jobs/78fgoq_tui-visual-improvements/`).
> They also predate `qge358_git-view-and-switch` (merged since), which added the
> current-branch line, `m`/`c` checkout actions, and a 1-line recent-activity
> strip to the list header — so even once attached, they won't show today's
> actual header. Attach current screenshots before handing to `@analyst`, or at
> minimum confirm problems 2–5 below still hold against the live TUI, not the
> stale images.

## Why

Despite being functionally correct, the TUI currently reads as a prototype,
not a product. From the screenshots and the renderer source, five concrete
user-facing problems cause that impression:

1. **Empty space dominates both screens.** The list shows one row near the top
   with ~95% black below it; the detail shows a short brief with a large gap
   before the footer. The eye reads an unfinished canvas before it reads any
   individual widget. This is the dominant visual impression and the top
   priority.
2. **Status messages hide the key legend.** After `ctrl+r`, the footer's key
   hints are replaced by `refreshed` and disappear entirely (confirmed in the
   list screenshot, which shows no help bar at all). A user who just acted no
   longer knows what keys exist.
3. **The detail screen duplicates identity and metadata.** The title appears
   twice with emphasis (the app header *and* glamour's purple H1 block for
   the brief's own `# Brief:` line); the metadata appears twice in two
   formats (the header's middot line *and* the brief's frontmatter rendered
   as gray prose in the body).
4. **The action bar is a wall and has a key collision.** Five `[key] Label`
   pairs plus a `│` separator plus `[D] Done` is visually undifferentiated,
   and `[d] Developer` vs `[D] Done` differ only by Shift on adjacent buttons.
5. **The empty-list state is an exit, not an invitation.** `No jobs yet.
   Press q to quit.` is the worst possible first-run screen.

This job exists to fix those five so the TUI feels intentional and
production-ready. A sixth question — how far the list header should lean into
git context (branch, recent activity) — was raised after this brief was
written; it's resolved under Notes below and feeds into priority 1, not a
separate priority.

## Out of scope

- The Settings and New-job forms — not the screens in focus this pass.
- Filtering, fuzzy search, mouse support, click handling.
- Icons, nerd-font glyphs, or any reliance on a non-default font.
- Light-mode parity or any theme work beyond what glamour already does.
- Rewriting the list or detail onto `bubbles/list` or `bubbles/table`. The
  lazy-render machinery in `detail.go` / `markdown.go` (the `stale`-tab trick
  and the glamour renderer cache) is load-bearing — it carries documented
  fixes for real input-lag and OSC/stdin-race bugs. Re-deriving those inside
  a new component is not worth it for visual wins. (If a future job adds
  features like fuzzy search, a rewrite may be justified then.)
- New external dependencies beyond those already imported
  (`charmbracelet/bubbletea`, `bubbles/textinput`, `lipgloss`, `glamour`).
- Everything `qge358_git-view-and-switch` already ruled out and this job does
  not reopen: a generic arbitrary-branch switcher (only the existing `m`
  quick-checkout-to-main and detail-view `c` remain), an interactive/scrollable
  git log or diff viewer, drill-down or filtering on commit history, and remote
  branches. See Notes for what *is* being revisited from that job.

**Locked for this job** — changing any of these is a separate decision, not
something smuggled into a polish pass:

- The accent colour `#7D56F4` and the surrounding status palette
  (`#D7A000` open, `#3FB950` done). A rebrand is its own job.
- The vim-style keybinding convention (`j/k`, `h/l`, `g/G`, `1-4`, `tab`,
  `q/esc`, `ctrl+r`). Work within it; if a new key is needed, document the
  collision analysis. Fixing the `d`/`D` collision (problem 4) *is* in scope
  and stays within the convention (e.g. Developer = `v`, Done keeps `D`).
- Glamour as the markdown renderer. Tuning *how* glamour renders IS in scope
  — e.g. stripping the brief's own `# Brief:` H1 before rendering, or
  adjusting the H1 style so it doesn't render as a duplicate purple block.
  Swapping the library is not.

## Notes

**Ranking, when goals conflict:** clear > consistent > pleasant. "Beautiful"
is a byproduct, not a target. Fill empty space with *information*, not
decoration — no boxes-within-boxes, no decorative borders. The house style
(austere, Unix-like, semantic colour) is confirmed by the screenshots and
should be preserved.

**Decided: revisiting `qge358`'s recent-activity strip.** `qge358_git-view-
and-switch` (merged) already added a current-branch line, `m` (list) / `c`
(detail) checkout actions, and a read-only recent-activity strip to the list
header (`tui/internal/ui/app.go`: `renderList`, `renderRecentActivity`,
`git.RecentCommits`). It originally shipped showing 5 commits; review blocked
that because a fixed 5-line strip pushed every job row down, and the brief
required the new header content not compete with the job list for space. The
fix that got it merged dropped the count to a hardcoded `recentActivityCount =
1` (`app.go`), reusing the header's existing blank spacer line so the
footprint is identical whether or not there's a commit to show.

That fix solved the immediate problem but did so by giving up almost all the
value of the feature. This job's priority 1 (fill the empty list view) is the
right place to finish it properly, because the two jobs were solving for
opposite cases: `qge358` was reviewed against a list with enough jobs to fill
the screen, where extra header lines cost real space; this brief's priority 1
is about a *sparse* list, where those same lines cost nothing because the
space below is already empty.

Resolution: replace the fixed constant with an entry count that scales with
how much room is actually spare — bounded between the existing 1 and a
ceiling of 5, computed from the terminal height and the current job count
(fixed chrome + job rows) rather than hardcoded. A full list keeps today's
1-line footprint (or fewer, if genuinely tight); a sparse list shows more, up
to 5. This satisfies `qge358`'s original constraint (never push a job row
off-screen, never introduce scrolling that wasn't already there) instead of
re-triggering it, and it directly serves priority 1's goal below. Exact
sizing math is the analyst's/developer's call — the constraint is the
product requirement, not a specific formula.

Visual treatment stays as `qge358` shipped it: inline in the existing list
header block, not a separate view or a boxed "panel" — a bordered widget
would violate this brief's own no-borders rule above and reopen the "don't
turn this into a second git client" boundary `qge358` explicitly closed. If
it needs to read as more of a deliberate section now that it can hold up to 5
lines, do that with a label and/or spacing/color grouping, not a border.

This is the *primary* mechanism for priority 1, but it may not be sufficient
on its own — a single-job project could still have empty space below the strip's
5-entry ceiling. If so, cost a secondary fill from the other options listed
in priority 1 rather than growing the git strip past 5 or adding interaction
to it (still out of scope, see above).

**Priority order and acceptance criteria** (for the analyst to break into
tasks; listed in priority order — if scope must be cut, cut from the bottom):

1. Fill the empty space on the list view. Primary mechanism: the adaptive
   recent-activity strip decided above (current branch + up to 5 recent
   commits, scaled to available room, never pushing job rows down or
   introducing new scrolling). A one-job project must still look intentional
   — if meaningful empty space remains once that strip is at its ceiling,
   cost a secondary fill from: a summary header (open/done counts), or a pane
   showing the selected job's brief summary / stage / last-modified. Not
   borders.
2. Make status and key hints coexist. Pressing `ctrl+r` must never hide the
   available keys (e.g. hint stays dim on the left, status appears on the
   right; or status appears above the hint and auto-clears after a beat).
3. De-duplicate the detail screen. One title and one metadata line on the
   whole screen. The open design question is whether app chrome or rendered
   content owns identity — pick one and apply it consistently across all
   four files (brief/tasks/implementation/verdict).
4. Resolve the action bar. One visual language for all six buttons (agents +
   Done); fix the `d`/`D` collision; define and document the narrow-width
   behaviour (at 80 cols the bar must not run off the right edge — either
   wrap to a second line, or truncate the agent *labels* but never the keys).
5. Make the empty-list state an invitation. The most prominent text a
   zero-job user sees must be the path to creating their first job (`n`),
   not the path to quitting.

**For the developer:** every change here is user-visible. The existing
`tui/internal/ui/*_test.go` files assert on rendered output via
`strings.Contains` (not golden snapshots) and cover specific behaviours —
footer height accounting, edit-hint scoping per tab, tab staleness on
resize/reload. They will catch regressions in *those* behaviours but won't
auto-flag every visual change, so expect to add or update tests alongside
the render changes. Touching the lazy-render internals in `detail.go` /
`markdown.go` (the `stale` flag, `ensureCurrentSized`, `syncViewerSize`,
the renderer cache, `DetectStyle`) is out of scope — the wins are all in
*what* gets rendered, not *how*.
