# Tasks: tui visual improvements

id: 78fgoq
status: open
analyst: claude
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Scope confirmation

The brief's own header flags the referenced screenshots as missing and
possibly stale. Per its own instruction ("at minimum confirm problems 2–5
below still hold against the live TUI, not the stale images"), I read the
current renderer source (`tui/internal/ui/app.go`, `detail.go`, `agents.go`,
`styles.go`, `markdown.go`) directly instead of the screenshots. All five
problems described in Why still hold against that source as of this
revision:

1. `recentActivityCount = 1` (app.go) plus an unconditional
   `"\n"` empty-rows branch mean a sparse list still renders almost entirely
   blank below the header.
2. `footer()` (app.go) and `renderFooter()` (detail.go) both fully replace
   the key hint with `a.status`/`d.status` — confirmed, no coexistence today.
3. `detailView.render()` writes its own title+meta chrome line, and every
   tab's raw markdown (including its own `# Brief:`/`# Tasks:`/etc. H1 and
   `key: value` frontmatter block, per `new-job.sh`'s scaffold) is rendered
   unmodified by glamour — confirmed duplicate identity/metadata.
4. `renderActionBar()` (detail.go) — confirmed: `agentMeta["developer"].key
   == "d"`, the mark-done key is `"D"`, adjacent in the same bar.
5. `renderList`'s empty-jobs branch is exactly the literal string the brief
   quotes — confirmed.

No screenshots were attached to this job directory at analysis time; none
were needed given the above. Proceeding straight to task breakdown.

## Task breakdown

Tasks are ordered to match the brief's priority order exactly (Notes →
"Priority order and acceptance criteria"), so cutting from the bottom of
this list if scope must be trimmed cuts the lowest brief priority first.

---

TASK-1: Adaptive recent-activity strip sizing (priority 1, primary
mechanism).

Replace the fixed `recentActivityCount = 1` constant with a count that
scales between 1 and a ceiling of 5, based on how much vertical room is
actually spare, per the brief's Notes resolution.

Recommended mechanism, to sidestep a real ordering hazard: `NewApp` calls
`refreshRecentCommits` before the first `tea.WindowSizeMsg` ever arrives, so
`a.width`/`a.height` are still zero at that point — any sizing formula
computed *at fetch time* would size against a stale/zero terminal height on
first render. Avoid this by decoupling fetch from display:
  - Keep `refreshRecentCommits` fetching a fixed ceiling
    (`git.RecentCommits(a.root, 5)`) every time it's called (same call sites
    as today: `NewApp`, `refreshJobs`, `updateNewJob`'s post-`mg-job`
    refresh) — one git call, cached in `a.recentCommits` same as now, just
    holding up to 5 entries instead of always 1.
  - Compute *how many of those cached commits to actually render* inside
    `renderRecentActivity` (or a small helper it calls), from the *current*
    `a.height` and `len(a.jobs)` at render time — this is naturally always
    fresh since `View()` only runs after layout is known.
  - Suggested formula, mirroring the existing 5-line chrome budget
    `detailView.bodyHeight` already documents for the detail view: list-view
    fixed chrome (title line, column header, divider, blank line before
    footer, footer) is 5 rows outside the job rows and the strip itself.
    `spare := a.height - 5 - len(a.jobs)`; render
    `clamp(spare, 1, 5)` commits (never below the existing 1-line floor —
    that's the pre-existing unconditional footprint, not new — never above
    the 5-entry ceiling). Guard `a.height == 0` (untouched-since-construction
    case, and some existing tests never set it) with the same kind of
    fallback `renderList` already uses for `a.width == 0`.
  - `len(a.recentCommits)` may be less than the computed count (few real
    commits) — render whatever's available, same graceful-degrade rule as
    today.

files: `tui/internal/ui/app.go` (`recentActivityCount` → ceiling constant +
new sizing helper, `refreshRecentCommits`, `renderRecentActivity`, possibly
`renderList` if the spare-room math needs data only it has).
depends: none.
risk: medium — the sizing formula has to hold across the full range from a
one-job project (max fill) to a job list tall enough to already fill the
screen (must degrade back to the current 1-line footprint, not add lines on
top of an already-tight layout); the fetch/render decoupling above needs to
actually get exercised by a zero-height and a many-jobs case, not just the
sparse case the brief's motivating scenario describes.

---

TASK-2: Secondary empty-space fill for a still-sparse list (priority 1,
secondary mechanism).

Per the brief: if a one-job project still has meaningful empty space once
TASK-1's strip is at its 5-entry ceiling, add one more non-interactive
information element — explicitly not a border, not new interaction.

The brief offers two options; recommending the first as the task scope:
- **A compact open/done summary line** (e.g. "3 open · 1 done") — costs
  nothing to compute (`a.jobs` is already fully loaded in memory) and needs
  no new data source.
- The alternative (a pane with the selected job's brief summary / stage /
  last-modified) needs a definition of "summary" (first paragraph? a new
  frontmatter field?) and a new per-job read (last-modified isn't currently
  parsed anywhere) — meaningfully more scope for the same acceptance
  criterion ("look intentional").

Recommend the summary-counts line for that reason; leave the fuller pane as
a documented follow-up if review finds the counts line insufficient once
built.

Placement: inline in the existing header block (same "no boxes, no borders"
constraint the brief repeats twice), only shown once TASK-1's math reports
spare room still remains after the strip is rendered at its actual (possibly
sub-5) count.

files: `tui/internal/ui/app.go` (`renderList`, a new small render helper).
depends: TASK-1 (needs the strip's real rendered line count, not just its
ceiling, to know whether space is actually left over).
risk: low — additive, read-only, no new data plumbing under the recommended
option.

---

TASK-3: Test coverage for TASK-1 and TASK-2.

Extend `tui/internal/ui/list_test.go`. The existing recent-activity tests
(`TestRenderListRecentActivityShowsMostRecentAcrossBranches`,
`TestRenderListRecentActivityEmptyOnFreshRepo`) pin behavior at a fixed
`80x24`/1-job fixture and lean on `recentActivityCount == 1` in their doc
comments and assertions (e.g. asserting the older "init" commit must NOT
also appear) — these assumptions need re-deriving against TASK-1's new
sizing math, not just left passing by accident. Add cases for:
- A sparse (1-job) list at a generous height shows more than 1 activity
  entry, up to 5, and shows the summary-counts line from TASK-2.
- A list with enough jobs to fill the screen keeps the pre-existing 1-line
  (or fewer) footprint — no regression of qge358's original constraint.
- A zero-height/never-resized `App` (`a.height` left at its zero value)
  doesn't panic and renders something sane.

files: `tui/internal/ui/list_test.go`.
depends: TASK-1, TASK-2.
risk: low — test-only, but the fixture work (multiple job counts × multiple
terminal heights) is non-trivial and is where TASK-1's actual correctness
gets proven, not in the source change itself.

---

TASK-4: Make list-view and detail-view status coexist with the key hint
(priority 2).

Per the brief: "hint stays dim on the left, status appears on the right; or
status appears above the hint and auto-clears after a beat." Recommending
the first (concatenate on one line) over the second: the codebase has no
timer/`tea.Tick` machinery today, and status is already cleared on the
user's very next keypress in both views (`updateList`'s `a.status = ""` at
the top, `updateDetail`'s equivalent), which already reads as "clears after
a beat" from the user's perspective — adding a real timer would be new
architecture for a cosmetic want the existing clear-on-next-key behavior
already mostly satisfies.

Scope:
- `App.footer()`: when `a.status != ""`, render both the (dimmed) hint and
  the (colored) status on the one line instead of the hint disappearing
  entirely.
- `detailView.renderFooter()`: same shape — combine `d.status` with the
  scroll-position + key hint instead of fully replacing it.
- Leave the existing multi-line `cmdErrorText` resolution-diagnosis case
  (covered by `TestDetailBodyHeightShrinksForMultiLineStatus` /
  `footerLines()`) as-is — that's a distinct, already-tested, genuinely
  multi-line diagnostic case, not the "user just pressed ctrl+r and lost the
  legend" problem this task targets. Confirm with review whether it also
  needs the hint appended, but don't let it block this task if the two turn
  out to conflict on width.

files: `tui/internal/ui/app.go` (`footer`), `tui/internal/ui/detail.go`
(`renderFooter`, and possibly `footerLines` if combining status+hint ever
produces a second line).
depends: none.
risk: medium — mostly the multi-line-status interaction called out above,
plus line width: concatenating a status message and the (already fairly
long) detail-view hint string on one 80-column line needs a real check it
doesn't itself start overflowing narrow terminals the way TASK-8's action
bar does.

---

TASK-5: Test coverage for TASK-4.

Add cases asserting the rendered footer/list output contains both the
status text and (some portion of) the key hint simultaneously after an
action like `ctrl+r`, for both the list and detail views. Confirm no
existing test asserted the old "status fully replaces hint" behavior as a
requirement (a scan of `list_test.go`/`detail_test.go`/`checkout_test.go`/
`donemsg_test.go`/`refresh_test.go` at analysis time found none — they all
assert on the `a.status`/`d.status` *field*, not the rendered footer string
— but re-confirm before changing render logic).

files: `tui/internal/ui/list_test.go`, `tui/internal/ui/detail_test.go`.
depends: TASK-4.
risk: low.

---

TASK-6: De-duplicate the detail screen's identity and metadata (priority 3).

Design decision (the brief leaves "app chrome or rendered content owns
identity" open — picking one so this is buildable): **app chrome owns
identity.** `detailView.render()`'s existing title+meta line (title, ID,
status, type, date, branch) already covers everything every file's own H1 +
frontmatter block repeats. Strip the redundant lines from rendered content
instead of touching `glamour`'s H1 style (the brief allows either; content
stripping needs no changes to `markdown.go`'s renderer/cache internals,
which are explicitly out of scope to touch).

All four files share `new-job.sh`'s scaffold shape: `# <Label>: <title>`,
blank line, a contiguous run of `key: value` lines (keys vary per file —
`brief.md` has `status/type/id/branch/date/author`, `tasks.md` has
`id/status/analyst/date`, etc. — so this must NOT reuse `job`'s
brief.md-specific `frontmatterKeys` list), blank line, then real content.
Strip only that leading, contiguous block — the H1 line, plus `key: value`
lines immediately following it up to the first blank line — before handing
content to `markdown.Render`. This is important for safety: it must NOT
scan the whole document, since real body content can look like `key: value`
syntactically (e.g. `TASK-1: description` inside `## Task breakdown`) — only
the *leading, contiguous* run bounded by the first blank line is frontmatter.

Also simplify `filePlaceholder` to drop its own redundant `# <label>`
heading for the same reason (chrome already shows the job title regardless
of which tab/file is showing).

files: `tui/internal/ui/detail.go` (`loadTab` or a new small preprocessing
helper it calls, `filePlaceholder`).
depends: none.
risk: medium — the leading-block-only stripping rule needs to be right or it
either leaves duplicate metadata behind (under-strip) or eats real prose
that happens to start right after the frontmatter with something
colon-shaped (over-strip); needs verification against all four real
templates (`project-template` scaffold) plus at least one already-written
example of each (e.g. this very job's own `tasks.md`/`implementation.md`
once filled in) rather than one synthetic fixture.

---

TASK-7: Test coverage for TASK-6.

Add detail-view tests asserting, for each of the four tab types: the
rendered tab body does not repeat the file's own `# <Label>:` heading text
verbatim as a second title, and does not repeat `key: value` frontmatter
lines the chrome meta line already shows — while real body content
(including a `TASK-1:`-shaped line inside a real section) still renders.
Re-run the existing suite's fixtures mentally against the new stripping:
`TestDetailViewReadsOffBranchJobViaGit`'s `tasks.md` fixture asserts
`"TASK-1"` is present in the rendered tasks tab — that line lives inside
`## Task breakdown`, well past the leading frontmatter block, so it must
still pass; if it doesn't, the stripping rule is over-reaching and needs
narrowing.

files: `tui/internal/ui/detail_test.go`.
depends: TASK-6.
risk: low — test-only, but this is the actual proof the over-strip risk in
TASK-6 didn't happen.

---

TASK-8: Resolve the action bar (priority 4).

Three separate acceptance criteria from the brief, all in scope together
since they touch the same render function:

1. **Fix the `d`/`D` collision.** Per the brief's own named example: rename
   Developer's action-bar key from `d` to `v` in `agentMeta`
   (`tui/internal/ui/agents.go`). `v` is currently unbound at detail-view
   scope (checked against the full existing keymap: `tab/h/l`, `j/k`,
   `1`-`4`, `e`, `D`, `c`, `q`, `esc`/`backspace`, `ctrl+r`) — confirm this
   still holds once TASK-4/TASK-6 land, since they don't add new bound keys,
   but re-check at implementation time regardless. `D` (mark done) keeps its
   key.
2. **One visual language for all six buttons.** Today the five agent buttons
   use `accentStyle` keys + plain labels, while `[D] Done` gets both key and
   label in `statusDoneStyle` and sits behind a `"│"` separator with extra
   spacing — inconsistent formatting is part of what reads as "a wall."
   Unify the `[key] Label` presentation for all six (same bracket/spacing
   pattern); Done may keep a distinct *color* (its existing green,
   consistent with `statusDoneStyle` used for "done" status elsewhere) to
   signal it's categorically different from the five launch actions, but the
   structural format (spacing, bracket style, grouping) should read as one
   coherent bar rather than "5 buttons + a separator + a 6th button in a
   different style."
3. **Define and document narrow-width (80 cols) behaviour.** The bar
   currently just concatenates all six buttons plus the stage label with no
   width awareness at all — at 80 columns with six buttons plus labels
   ("Product Owner" is the longest), confirm whether it currently overflows
   (needs checking against real rendered width, not assumed). Either: wrap
   to a second line, or truncate agent *labels* only (never the bracketed
   keys, which must always stay reachable/visible). Pick one, document the
   choice in the function's doc comment the way other non-obvious layout
   choices in this file already are.

files: `tui/internal/ui/agents.go` (key rename), `tui/internal/ui/detail.go`
(`renderActionBar`), `tui/internal/ui/styles.go` (if a new shared button
style is warranted).
depends: none.
risk: medium — the key rename ripples into existing tests hardcoding `"d"`
for developer (see TASK-9); the narrow-width behavior is new logic with no
existing precedent in this file to follow, and lipgloss width math for
six variable-length labels plus a stage prefix is easy to get subtly wrong
at exactly 80 columns.

---

TASK-9: Test coverage for TASK-8, including required updates to existing
tests that hardcode the old developer key.

The following existing tests assert `"d"` maps to developer and must be
updated to `"v"` once TASK-8 lands (not just left broken):
`TestAgentForKeyIgnoresStage` (`agents_test.go`, `wantByKey` map),
`TestAgentForKeyNoDetail` (`agents_test.go`, calls `a.agentForKey("d")`),
`TestBranchGuardBlocksAgentLaunch` (`branchguard_test.go`, calls
`a.updateDetail(keyMsg("d"))` with a comment `// "d" = developer`).
Add new cases for: the action bar renders all six buttons in one consistent
format, `d` no longer resolves to any agent (confirms the collision is
actually gone, not just renamed elsewhere), and the chosen narrow-width
behavior at 80 columns (either the wrap or the truncate-labels-not-keys
rule) actually holds.

files: `tui/internal/ui/agents_test.go`, `tui/internal/ui/branchguard_test.go`,
`tui/internal/ui/detail_test.go` (or extend `agents_test.go`'s
`TestRenderActionBarAlwaysShowsAllAgents`).
depends: TASK-8.
risk: low — test-only, but TASK-8 is not done until these pass; the
hardcoded-`"d"` tests above will fail the moment the rename lands and are
not optional cleanup.

---

TASK-10: Make the empty-list state an invitation (priority 5).

Replace `"No jobs yet. Press q to quit."` (`renderList`'s
`len(a.jobs) == 0` branch) with copy that leads with the `n` (new job) key,
not `q` (quit) — the brief's own framing: "the most prominent text a
zero-job user sees must be the path to creating their first job, not the
path to quitting." Stay within the existing plain-text, no-borders house
style; this is a copy/emphasis change, not a new widget.

Note the interaction with TASK-1/TASK-2: a zero-job project is the most
extreme sparse case their sizing math has to handle (`len(a.jobs) == 0`) —
sequence-check against those once built, though this task's own change
(the empty-rows message) is a separate code branch from the header block
TASK-1/TASK-2 touch, so there's no hard dependency, just shared-file merge
proximity.

files: `tui/internal/ui/app.go` (`renderList`'s empty-state branch).
depends: none (soft proximity to TASK-1/TASK-2 only, both touch
`renderList`).
risk: low — isolated string/emphasis change in a branch no existing test
pins the literal text of.

---

TASK-11: Test coverage for TASK-10.

Add a `list_test.go` case with zero jobs asserting the rendered list
mentions the `n` new-job key prominently and that quitting is not the
leading call to action.

files: `tui/internal/ui/list_test.go`.
depends: TASK-10.
risk: low.

## Notes for the developer

- Every task above changes rendered output the existing
  `tui/internal/ui/*_test.go` suite asserts on via `strings.Contains` — run
  the full suite after each task, not just the paired test task, since
  render changes in one task (e.g. TASK-8's key rename) can break assertions
  written for an earlier one (e.g. TASK-4's footer tests, if they happen to
  reference agent-bar text).
- None of these tasks touch `detail.go`'s lazy-render internals (`stale`,
  `ensureCurrentSized`, `syncViewerSize`, the renderer cache, `DetectStyle`)
  — confirmed out of scope per the brief; TASK-6's content stripping runs
  on the raw string in `loadTab` *before* it reaches `viewer.SetContent`,
  which is the existing seam, not a new one.
- TASK-1/TASK-2/TASK-10 all edit `renderList`; TASK-4 edits both `footer`
  (list) and `renderFooter` (detail); TASK-6/TASK-8 both edit `detail.go`'s
  render path but different functions (`loadTab`/`filePlaceholder` vs
  `renderActionBar`). None of these are true blocking dependencies on each
  other, but expect merge friction if implemented out of priority order —
  recommend implementing in the numbered order above.
