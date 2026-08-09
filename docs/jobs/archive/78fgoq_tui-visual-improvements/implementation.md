# Implementation: tui visual improvements

id: 78fgoq
status: open
developer: claude
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

Implemented all 11 tasks from tasks.md, in priority order, one commit per
task (plus one gofmt-only follow-up commit for TASK-7). All five problems
named in the brief's "Why" are addressed:

1. Empty space on the list view is now filled by an adaptive recent-activity
   strip (1–5 entries, scaled to spare terminal room) plus a secondary
   open/done summary line when room remains after the strip.
2. Status messages no longer replace the key hint in either footer — they
   render alongside it.
3. The detail screen's duplicate title/metadata is gone: every job file's
   leading H1 + frontmatter block is stripped before rendering, since the
   chrome's title+meta line already shows that information.
4. The action bar's six buttons (5 agents + Done) now share one visual
   format, the `d`/`D` key collision is fixed (Developer moved to `v`), and
   80-column overflow is handled by truncating labels (never keys).
5. The empty-job-list state now leads with "press n to create your first
   one" instead of "press q to quit".

## Changes

TASK-1 (`tui/internal/ui/app.go`): Replaced the fixed `recentActivityCount =
1` constant with `recentActivityFloor`/`recentActivityCeiling` (1/5).
`refreshRecentCommits` now always fetches up to the ceiling (decoupling fetch
from display, avoiding the zero-height-at-construction hazard). A new
`recentActivityShown()` computes how many of those cached commits to
actually render, from the current `a.height` and `len(a.jobs)` at render
time, via `spare := a.height - 5 - len(a.jobs)` clamped to
`[floor, ceiling]` and capped by how many real commits exist.
`renderRecentActivity` and `renderList` were updated to use it.

TASK-2 (`tui/internal/ui/app.go`): Added `spareHeaderRoom()` (spare room left
after the strip's actual footprint) and `renderJobSummary()` (a compact
"`<n> open · <n> done`" line), shown inline in the header only when
`spareHeaderRoom() >= 1` and the list is non-empty.

TASK-3 (`tui/internal/ui/list_test.go`): Re-derived the two existing
recent-activity tests against the new sizing math (one now pins a tight
height to exercise the floor explicitly instead of relying on the old fixed
constant); added tests for a sparse list scaling up to the ceiling +
summary line, a 20-job list keeping the pre-existing 1-line floor, and a
zero-height `App` not panicking.

TASK-4 (`tui/internal/ui/app.go`, `tui/internal/ui/detail.go`):
`App.footer()` and `detailView.renderFooter()` now concatenate the dim key
hint with the (colored) status instead of the status replacing it. The
existing multi-line `cmdErrorText` diagnosis case in `renderFooter` still
fully replaces the hint, as the brief allowed, to avoid overflowing narrow
terminals with hint + a multi-line diagnostic on top of it.

TASK-5 (`tui/internal/ui/list_test.go`, `tui/internal/ui/detail_test.go`):
Added coexistence tests for both footers, plus a test confirming the
multi-line-status exception is preserved.

TASK-6 (`tui/internal/ui/detail.go`): Added `stripLeadingFrontmatter`, which
removes a file's leading `# <Label>: <title>` H1 and immediately-following
contiguous `key: value` block (bounded strictly by the first blank line, so
real body content — e.g. a `TASK-1: ...` line deep in `## Task breakdown` —
is never touched) before the content reaches `markdown.Render` via
`loadTab`. `filePlaceholder` no longer renders its own `# <label>` heading,
for the same reason: the chrome's title line already covers it.

TASK-7 (`tui/internal/ui/detail_test.go`): Added an end-to-end test across
all four file types (brief/tasks/implementation/verdict), each in
new-job.sh's exact scaffold shape, asserting the rendered tab body neither
repeats the heading nor the frontmatter, while real content (including a
`TASK-1:`-shaped line inside a real section) still renders. Also added
direct unit tests for `stripLeadingFrontmatter`'s "leave non-scaffold content
alone" fallback and for `filePlaceholder`'s dropped heading. One existing
test (`TestDetailViewMetaLineShowsBranch`) had an assertion that was only
passing by coincidence, via the now-stripped duplicate frontmatter line —
corrected it to assert against the chrome's actual off-branch format.

TASK-8 (`tui/internal/ui/agents.go`, `tui/internal/ui/detail.go`): Renamed
Developer's key from `d` to `v` in `agentMeta` (fixing the collision with `D`
mark-done). Rewrote `renderActionBar` around a new `actionButton` type so all
six buttons (5 agents + Done) render in one consistent `[key] Label` format,
with Done keeping only its distinct color (no more `│` separator or
different spacing). Added width-aware label truncation: the bar computes the
fixed cost of the stage label plus every button's `[key] ` prefix and
separators, divides whatever's left over the six buttons, and truncates each
label to its share (via a new `truncateToWidth` helper) — at 80 columns this
now fits on one line with all six keys intact and labels like "Product
Owner" shortened to "Prod…". Wrapping to a second line was rejected in favor
of truncation because it would have required also reworking
`detailView.bodyHeight`'s fixed chrome-row budget, which is out of scope.

TASK-9 (`tui/internal/ui/agents_test.go`, `tui/internal/ui/branchguard_test.go`):
Updated the three existing tests hardcoding `"d"` for Developer to `"v"`.
Widened `TestRenderActionBarAlwaysShowsAllAgents` to a 200-column terminal
(so it tests the "full labels present" case cleanly) and added dedicated
tests for: `"d"` no longer resolving to any agent, the unified `[key] Label`
format across all six buttons, and the 80-column truncation behavior
(one line, all six keys present, longest label truncated).

TASK-10 (`tui/internal/ui/app.go`): Replaced "No jobs yet. Press q to quit."
with a message that emphasizes `n` (styled with `accentStyle`, the same
treatment given the header's current-branch tag) and never mentions quitting.

TASK-11 (`tui/internal/ui/list_test.go`): Added a test asserting the
zero-job empty state's dedicated message names `n` and does not mention
"quit" (checked on the isolated line, since the footer below it still
legitimately lists `q quit` alongside every other key).

## Known issues / follow-ups

- The recent-activity strip's per-render `recentActivityShown()` call is O(1)
  and cheap, but it does mean the strip's line count can visibly change
  between two renders if the terminal is resized while the job count stays
  fixed (or vice versa) — this is intentional per the brief's "scales with
  available room" requirement, just noting it as a UI behavior a reviewer
  should sanity-check interactively, not something a `strings.Contains` test
  can fully capture.
- The action bar's label-truncation budget is a flat integer division across
  all six buttons (`labelBudget / 6`), not weighted by each label's natural
  length — a short label like "Done" ends up with unused budget while longer
  ones get truncated at the same width. This keeps the algorithm simple and
  deterministic (a stated priority in the brief: clear > consistent >
  pleasant) but a smarter allocation (e.g. give "Done" only what it needs and
  redistribute the rest) is a possible follow-up if narrow-width legibility
  becomes a complaint.
- `isFrontmatterLine`'s "key: value" heuristic (no leading whitespace, no
  space before the colon) is deliberately narrow but not bulletproof — a line
  like "Sentence: description." immediately after a file's H1 (with no blank
  line before it) would be misidentified as frontmatter. This never triggers
  against any of the four real templates or this job's own filled-in files
  (verified manually), and the brief's own scaffold shape guards against it
  in practice, but it's worth flagging for review as a known heuristic
  boundary rather than a proven-complete parser.
