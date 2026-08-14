package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/markdown"
	"github.com/lmuskalla/manigot/internal/project"
)

// The four markdown files every job directory holds, in the order the detail
// view presents them. Matches new-job.sh and the README's job-workflow section.
//
// editable gates the in-TUI edit shortcut (see app.go's "e" handling). Only
// brief.md is user-authored — the other three are meant to be written by
// agents, so the shortcut is a no-op there. This is a per-tab flag rather
// than a hardcoded brief.md check so a future job could open editing on more
// tabs by flipping this table alone.
var jobFiles = []struct {
	label    string // tab label
	filename string // file within the job dir
	editable bool   // whether the "e" shortcut may open this tab in $EDITOR
}{
	{"brief", "brief.md", true},
	{"tasks", "tasks.md", false},
	{"implementation", "implementation.md", false},
	{"verdict", "verdict.md", false},
}

// fileTab is one rendered file in the detail view.
type fileTab struct {
	label    string
	path     string
	exists   bool
	editable bool
	content  string // raw markdown (or the placeholder) as last read from disk
	viewer   *markdown.Viewer

	// isLog marks the fifth "log" tab: unlike the four job files,
	// its content comes from mg-jdi's sidecar run.log
	// (job.ReadJDIRunLogTail(d.job.Root, d.job.Name)) rather than d.path —
	// the sidecar lives outside any job's worktree (tied only to the job
	// name, not a branch), and it is never editable.
	isLog bool

	// isDiff marks the sixth "diff" tab: its content is computed, not read —
	// the quick-eyeball of what the job's branch changed relative to the
	// project's base branch (git.LogOneline + git.DiffStat over
	// <base>...<branch>, the same output `mg diff`'s default prints, see
	// loadDiff), built fresh on every load so ctrl+r picks up new commits.
	// Like the log tab it is never editable; a job with no branch (the
	// working-tree fallback) or a git error gets a plain-text placeholder.
	isDiff bool

	// stale marks a viewer that is out of date with content and/or the
	// current body size, because the tab wasn't active when that changed —
	// either a resize (syncViewerSize) or a (re)load (loadTab, see
	// the "selecting/leaving a job is laggy" fix below). It is cleared the
	// next time the tab becomes active and is caught up in render() via
	// ensureCurrentSized.
	stale bool
}

// detailView shows the four job files as scrollable markdown with a tab bar.
type detailView struct {
	job    job.Job
	tabs   []fileTab
	cur    int
	width  int
	height int

	// status, when non-empty, replaces the footer's key hint — used to confirm
	// an agent launch or report a launch error.
	status string

	// spinnerStep is the current activity-indicator frame index (see
	// activity.go), threaded in from the App by the spinnerTickMsg handler so
	// the action-bar's running badge animates in sync with the list row.
	// Defaults to 0 for a freshly built view; the next tick re-syncs it even
	// if the App was already mid-animation when the view opened.
	spinnerStep int

	// recentCommits backs the bottom-of-view git-log strip: the last few
	// commits on this job's own branch (git.BranchCommits, fetched via
	// refreshCommits), the detail view's counterpart of
	// listView.recentCommits. nil when the job has no branch, the repo has no
	// commits yet, or BranchCommits errors (e.g. a non-repo project) —
	// renderCommitStrip degrades to rendering nothing in that case. Populated
	// by the App on open and on refresh; a detail view constructed directly
	// (as the tests do) simply has no strip.
	recentCommits []git.Commit

	// recentMax is the maximum number of strip entries (the settings'
	// RecentActivityCountValue, stored by refreshCommits) that
	// commitStripShown clamps to, mirroring listView.recentActivityShown's
	// use of its maxRecent parameter.
	recentMax int
}

// newDetailView loads all four job files, plus the fifth "log" tab and the
// sixth computed "diff" tab, for job j at the given viewport size.
func newDetailView(j job.Job, width, height int) *detailView {
	d := &detailView{job: j, width: width, height: height}
	for _, f := range jobFiles {
		d.tabs = append(d.tabs, fileTab{
			label:    f.label,
			path:     filepath.Join(j.Dir, f.filename),
			editable: f.editable,
			viewer:   markdown.NewViewer(d.bodyWidth(), d.bodyHeight()),
		})
	}
	d.tabs = append(d.tabs, fileTab{
		label:  "log",
		isLog:  true,
		viewer: markdown.NewViewer(d.bodyWidth(), d.bodyHeight()),
	})
	d.tabs = append(d.tabs, fileTab{
		label:  "diff",
		isDiff: true,
		viewer: markdown.NewViewer(d.bodyWidth(), d.bodyHeight()),
	})
	d.loadTabs()
	return d
}

// jobFileNotWritten is the placeholder markdown shown for a file new-job.sh has
// not created or an agent has not filled in yet.
//
// No "# <label>" heading of its own: the detail view's own title+meta chrome
// line already shows the job title regardless of which tab is showing, so a
// second heading here would just be the same duplication
// stripLeadingFrontmatter removes from real file content below.
func filePlaceholder(filename string) string {
	return "_" + filename + " has not been written yet._"
}

// loadTabs (re)reads every tab's file from disk. Called on construction and
// on refresh: agents edit files outside the TUI, so the detail view
// must re-read to pick up their changes.
//
// Only the active tab's viewer is actually re-rendered here; the other three
// are cheap disk reads whose markdown render is deferred until they become
// active (see loadTab, ensureCurrentSized). Previously this eagerly rendered
// all four tabs' markdown — non-trivial cost (glamour.Render is not free) —
// on every job open and every return to the list, which is what made
// selecting a job and pressing esc/backspace to leave one feel laggy even
// after the same problem was fixed for resize-triggered re-renders.
func (d *detailView) loadTabs() {
	for i := range d.tabs {
		d.loadTab(i)
	}
}

// loadTab re-reads one tab's file into t.content. If the tab is
// currently active, its viewer is rebuilt immediately; otherwise the viewer
// is left alone and the tab is marked stale so ensureCurrentSized catches it
// up lazily once it actually becomes active.
//
// Every job file is read straight off disk via d.readFile (a job's own
// worktree — or, for an archived job, the main worktree's archive/ — is
// unconditionally the live, correct place to read it from, see the job
// package doc). A missing file falls back to the placeholder.
//
// Real content goes through stripLeadingFrontmatter first: every job file's
// new-job.sh scaffold repeats the title (as its own "# <Label>: <title>" H1)
// and metadata (as a "key: value" block) that the detail view's chrome
// title+meta line already shows — see loadTab's caller-facing doc and the
// helper's own comment for why only the leading block is ever touched.
func (d *detailView) loadTab(i int) {
	t := &d.tabs[i]
	if t.isLog {
		if text, ok := job.ReadJDIRunLogTail(d.job.Root, d.job.Name); ok {
			t.exists = true
			t.content = text
			if t.content == "" {
				t.content = "_run.log is empty — mg jdi may still be starting its first invocation._"
			}
		} else {
			t.exists = false
			t.content = "_no mg jdi run has happened for this job yet._"
		}
		if i == d.cur {
			t.viewer.SetContent(t.content)
			t.stale = false
		} else {
			t.stale = true
		}
		return
	}
	if t.isDiff {
		d.loadDiff(t)
		if i == d.cur {
			t.viewer.SetContent(t.content)
			t.stale = false
		} else {
			t.stale = true
		}
		return
	}
	data, ok := d.readFile(t)
	if ok {
		t.exists = true
		t.content = stripLeadingFrontmatter(string(data))
	} else {
		t.exists = false
		t.content = filePlaceholder(filepath.Base(t.path))
	}
	if i == d.cur {
		t.viewer.SetContent(t.content)
		t.stale = false
	} else {
		t.stale = true
	}
}

// loadDiff fills the computed "diff" tab's content: the quick-eyeball of
// what the job's branch changed relative to the project's base branch —
// `git log --oneline <base>...<branch>` followed by
// `git diff --stat <base>...<branch>` — mirroring `mg diff`'s default output
// (cmd/mg/diff.go) and its base-branch resolution chain (see
// diffBaseBranch), so the TUI's diff tab and the CLI's `mg diff` always
// agree on the range shown.
//
// Degrades, never crashes: a job with no branch (the working-tree fallback /
// non-repo project) and a git error (e.g. the branch deleted out from under
// the TUI) each set exists=false and a plain-text placeholder; an undiverged
// branch gets the same "No changes on <branch> relative to <base>." line
// `mg diff` prints, with exists=true — it is a real, informative result.
func (d *detailView) loadDiff(t *fileTab) {
	t.exists = false
	if d.job.Branch == "" {
		t.content = "_this job has no branch to diff (not a git worktree job)._"
		return
	}
	base := d.diffBaseBranch()
	logs, err := git.LogOneline(d.job.Root, base, d.job.Branch)
	if err != nil {
		t.content = diffErrorPlaceholder(err)
		return
	}
	files, err := git.DiffStat(d.job.Root, base, d.job.Branch)
	if err != nil {
		t.content = diffErrorPlaceholder(err)
		return
	}
	if logs == "" && files == "" {
		t.content = fmt.Sprintf("No changes on %s relative to %s.", d.job.Branch, base)
		t.exists = true
		return
	}
	t.exists = true
	t.content = logs + "\n\n" + files
}

// diffBaseBranch resolves the base branch the diff tab diffs against,
// mirroring cmd/mg/diff.go's chain: the project's configured baseBranch
// (.manigot/manigot.json), falling back to git.SymbolicRefHead
// (origin/HEAD → "main") when unset — deliberately NOT
// Settings.BaseBranchValue(), which would default to "main" and skip the
// origin/HEAD fallback. The same chain doneConfirmLines uses for its
// "Branch : <job> → <base>" line, so the diff tab and the done confirmation
// never disagree about what the job will be merged into.
func (d *detailView) diffBaseBranch() string {
	settings, _ := project.Load(d.job.Root)
	base := settings.BaseBranch
	if base == "" {
		base = git.SymbolicRefHead(d.job.Root)
	}
	return base
}

// diffErrorPlaceholder is the plain-text content for the diff tab when git
// fails to produce the range (e.g. a branch deleted out from under the TUI):
// the error's message, dimmed so it reads as a degrade, not a crash.
func diffErrorPlaceholder(err error) string {
	return "_could not compute the diff: " + err.Error() + "_"
}

// stripLeadingFrontmatter removes the leading H1 + "key: value" frontmatter
// block every job file's new-job.sh scaffold begins with, so glamour doesn't
// render a second title/metadata block duplicating what the detail view's
// own chrome (title+meta line) already shows — app chrome owns identity.
//
// Only the leading, contiguous run bounded by the first blank line is ever
// touched: the first line if it's an H1 ("# "), an optional single blank
// line right after it, then a run of "key: value" lines, stopping at the
// first blank line found in that run. Real body content past that point is
// never scanned — a line like "TASK-1: description" inside "## Task
// breakdown" is well past the block's terminating blank line by the time any
// real content is reached, so stripping never reaches it even though it is
// syntactically colon-shaped too.
//
// A file that doesn't start with an H1, or whose line right after the H1
// (and optional blank) isn't itself a "key: value" line, doesn't match the
// scaffold shape at all and is returned unchanged rather than guessed at.
func stripLeadingFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return content
	}
	i := 1
	if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	start := i
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
		if !isFrontmatterLine(lines[i]) {
			// Not the scaffold shape (e.g. the H1 is directly followed by
			// prose, no frontmatter at all) — leave the whole file alone.
			return content
		}
		i++
	}
	if i == start {
		// H1 with nothing (not even a blank line) after it.
		return content
	}
	// Skip the blank line terminating the frontmatter block, if present.
	if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return strings.Join(lines[i:], "\n")
}

// isFrontmatterLine reports whether a line matches the "key: value" shape
// new-job.sh's scaffold frontmatter uses: a bare key with no leading
// whitespace and no spaces before its colon (e.g. "status: open",
// "developer:"). Deliberately narrow so it only ever matches inside the
// leading contiguous block stripLeadingFrontmatter bounds by the first blank
// line — real prose elsewhere in the document is never passed to this.
func isFrontmatterLine(line string) bool {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return false
	}
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return false
	}
	return !strings.ContainsAny(line[:idx], " \t")
}

// readFile reads one tab's file bytes straight off disk from t.path — a
// job's own worktree (or the main worktree's archive/) is unconditionally
// the live, correct place to read its four files from (see the job package
// doc), so there is no branch check and no git-show fallback. ok is false
// when the file isn't available at that path (missing).
func (d *detailView) readFile(t *fileTab) (data []byte, ok bool) {
	b, err := os.ReadFile(t.path)
	if err != nil {
		return nil, false
	}
	return b, true
}

// reload re-reads all four files (used by App.refresh).
func (d *detailView) reload() { d.loadTabs() }

// reloadCurrent re-reads just the active tab's file — used after the "e"
// edit shortcut returns, so a file that didn't exist before (and was shown
// as a placeholder) flips over to its real content once the editor creates
// it on save.
func (d *detailView) reloadCurrent() { d.loadTab(d.cur) }

// refreshCommits re-reads the job-branch git-log strip from git (git.
// BranchCommits against d.job.Root, d.job.Branch — the branch ref is shared
// across the whole repo, so the main worktree's root resolves it fine),
// always fetching up to maxRecent (Settings.RecentActivityCountValue). How
// many of those cached commits actually get rendered is decided later, at
// render time, by commitStripShown — the same split the list view uses
// between refreshRecentCommits and recentActivityShown, so a view that never
// received a WindowSizeMsg isn't sized against a stale terminal height.
//
// Like the list view's strip fetch, an error (e.g. a non-repo project) or an
// empty job branch degrades to an empty strip rather than surfacing in the
// status line — this is decorative, optional content, not an action the user
// asked for.
func (d *detailView) refreshCommits(maxRecent int) {
	d.recentMax = maxRecent
	if d.job.Branch == "" {
		d.recentCommits = nil
		d.syncViewerSize()
		return
	}
	commits, err := git.BranchCommits(d.job.Root, d.job.Branch, maxRecent)
	if err != nil {
		d.recentCommits = nil
		d.syncViewerSize()
		return
	}
	d.recentCommits = commits
	// The strip's footprint may have changed (appeared, disappeared, or
	// grown), which changes how many chrome rows the total render needs —
	// resize the active viewer and mark the others stale so the body shrinks
	// and the total still fits the viewport, mirroring setStatus's own
	// budget handling. refreshCommits runs right after newDetailView (whose
	// viewers were sized with no strip yet), so without this the strip would
	// overflow the alt-screen on open.
	d.syncViewerSize()
}

// resize propagates a window-size change to every viewer (a width change means
// the markdown must be re-wrapped).
func (d *detailView) resize(width, height int) {
	if width == d.width && height == d.height {
		return
	}
	d.width, d.height = width, height
	d.syncViewerSize()
}

// setStatus sets the footer status line and, since a multi-line status (e.g.
// cmdErrorText's resolution diagnosis) changes how many chrome rows the
// footer needs, resizes the viewers so the total rendered height still fits
// the terminal — otherwise the alt-screen viewport clips the bottom of the
// status instead of shrinking the body.
func (d *detailView) setStatus(s string) {
	d.status = s
	d.syncViewerSize()
}

// syncViewerSize resizes the active tab's viewer to the current body
// dimensions immediately, and marks the other three as stale instead of
// eagerly re-rendering them too.
//
// Re-rendering all four tabs' markdown here was a major source of the
// reported input lag: this is called from setStatus, which itself fires on
// nearly every keypress in the detail view (including every agent launch
// attempt), so every keypress was paying for three re-renders that weren't
// even visible. Deferred tabs catch up lazily in render() the next time
// they actually become the active one.
func (d *detailView) syncViewerSize() {
	w, h := d.bodyWidth(), d.bodyHeight()
	for i := range d.tabs {
		if i == d.cur {
			d.tabs[i].viewer.Resize(w, h)
			d.tabs[i].stale = false
		} else {
			d.tabs[i].stale = true
		}
	}
}

// ensureCurrentSized brings the active tab's viewer up to date if it was left
// stale — by a resize that happened while a different tab was showing (see
// syncViewerSize), or by a (re)load that only updated t.content because the
// tab wasn't active at the time (see loadTab). Called from render() so a tab
// is always correctly rendered by the time it is actually drawn.
//
// This always goes through SetSize+SetContent rather than the guarded
// Resize: Resize is a no-op when the width/height haven't changed, which is
// exactly the common case here (content changed, size didn't) — using it
// would leave a content-stale tab showing its old rendered text.
func (d *detailView) ensureCurrentSized() {
	t := &d.tabs[d.cur]
	if !t.stale {
		return
	}
	t.viewer.SetSize(d.bodyWidth(), d.bodyHeight())
	t.viewer.SetContent(t.content)
	t.stale = false
}

// update handles detail-view keys: file switching and scrolling.
func (d *detailView) update(msg tea.KeyMsg) {
	switch msg.String() {
	case "tab", "right":
		if d.cur < len(d.tabs)-1 {
			d.cur++
		}
	case "shift+tab", "left":
		if d.cur > 0 {
			d.cur--
		}
	case "1":
		d.cur = 0
	case "2":
		d.cur = 1
	case "3":
		d.cur = 2
	case "4":
		d.cur = 3
	case "5":
		d.cur = 4
	case "6":
		d.cur = 5
	default:
		d.active().scroll(msg)
	}
}

// current returns the active tab.
func (d *detailView) current() *fileTab { return &d.tabs[d.cur] }

// active is an alias for current used by update; kept for readability at the
// call site where we mean "the tab the scroll key applies to".
func (d *detailView) active() *fileTab { return d.current() }

// scroll routes scroll keys to the active viewer.
func (t *fileTab) scroll(msg tea.KeyMsg) {
	switch msg.String() {
	case "down":
		t.viewer.ScrollDown(1)
	case "up":
		t.viewer.ScrollUp(1)
	case "pgdown", " ":
		t.viewer.PageDown()
	case "pgup":
		t.viewer.PageUp()
	case "g", "home":
		t.viewer.Top()
	case "G", "end":
		t.viewer.Bottom()
	}
}

// bodyGutter is the width of the vertical rule + gap rendered to the left of
// the doc body (see renderBody) — reserved out of bodyWidth so the
// rule-prefixed lines still fit within d.width.
const bodyGutter = 2

// bodyWidth / bodyHeight compute the markdown viewport given the chrome the
// detail view draws around it (title, tab bar, footer, and the left-hand
// vertical rule).
func (d *detailView) bodyWidth() int {
	w := d.width - 2 - bodyGutter // small right margin + left rule/gap
	if w < 1 {
		w = 1
	}
	return w
}

func (d *detailView) bodyHeight() int {
	// title(1) + blank(1) + tab bar(1) + agents line(1) + stage/done line(1) + blank(1) + body + blank(1) + footer(footerLines)
	// = (7 + footerLines) chrome rows around the body, plus the bottom
	// git-log strip's own footprint (1 spacer + its commit rows, see
	// commitStripRows) — the body shrinks so the total render still fits the
	// alt-screen viewport, the same budget discipline
	// TestDetailBodyHeightShrinksForMultiLineStatus guards for the footer.
	h := d.height - 7 - d.footerLines() - d.commitStripRows()
	if h < 1 {
		h = 1
	}
	return h
}

// renderBody prefixes the active viewer's rendered lines with a dim vertical
// rule, so the doc content — already indented by glamour's own document
// margin — reads as a distinct panel rather than chrome-level text.
func (d *detailView) renderBody() string {
	body := d.current().viewer.View()
	if body == "" {
		return body
	}
	rule := dimStyle.Render("│") + " "
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = rule + l
	}
	return strings.Join(lines, "\n")
}

// footerLines returns how many rows renderFooter will occupy. Normally the
// footer is a single line (the scroll position / key hint, or a short status
// message), but cmdErrorText (app.go) can return a multi-line resolution
// diagnosis for a failed agent launch — bodyHeight must account for that so
// the total rendered height still fits the fixed alt-screen viewport, instead
// of pushing the last line(s) of the footer (the "fix:" hint) off screen.
func (d *detailView) footerLines() int {
	if d.status == "" {
		return 1
	}
	return strings.Count(d.status, "\n") + 1
}

// commitStripShown returns how many of d.recentCommits the bottom git-log
// strip should actually render, mirroring listView.recentActivityShown's
// adaptive sizing: between recentActivityFloor and the configured maximum
// (d.recentMax, stored by refreshCommits), clamped to the spare vertical room
// and the available commits. 0 when the cache is empty — no strip at all.
//
// The spare-room formula is the detail view's analogue of the list's
// `height - dashboardFixedChrome - jobCount`: the fixed chrome (7 rows +
// footerLines) is what the list's dashboardFixedChrome is; the body's own
// 1-row floor stands in for the job rows (the body is scrollable but must
// never be squeezed out entirely); and the strip's own 1-row spacer is
// accounted for so the body budget and the rendered output agree.
//
// height == 0 (a view that has never received a tea.WindowSizeMsg, e.g.
// some existing tests) falls back to the floor, the same kind of guard the
// list view applies.
func (d *detailView) commitStripShown() int {
	if len(d.recentCommits) == 0 {
		return 0
	}
	var n int
	if d.height == 0 {
		n = recentActivityFloor
	} else {
		spare := d.height - 7 - d.footerLines() - 1 - 1 // body floor + strip spacer
		n = clamp(spare, recentActivityFloor, d.recentMax)
	}
	if n > len(d.recentCommits) {
		// Fewer real commits than the computed count — render whatever's
		// available, same graceful-degrade rule the list view applies.
		n = len(d.recentCommits)
	}
	return n
}

// commitStripRows is the strip's total vertical footprint below the footer:
// its 1-row blank spacer plus one row per commit line it renders (0 when
// nothing renders). bodyHeight subtracts this from the viewport so the body
// shrinks and the total render still fits — the detail view's analogue of
// the list's dashboardFixedChrome strip-spacer row.
func (d *detailView) commitStripRows() int {
	if n := d.commitStripShown(); n > 0 {
		return 1 + n
	}
	return 0
}

// render draws the detail view.
func (d *detailView) render() string {
	w := d.width
	if w == 0 {
		w = 72
	}

	d.ensureCurrentSized()

	var b strings.Builder

	// Title + meta line.
	b.WriteString(titleStyle.Render(d.job.Title))
	meta := fmt.Sprintf("  %s · %s · %s · %s", d.job.ID, d.job.Status, d.job.Type, d.job.Date)
	b.WriteString(dimStyle.Render(meta))
	// Branch: show which branch the job's own worktree is checked out to.
	// Purely informational — every job has its own worktree, so there is no
	// "wrong branch checked out" state to warn about anymore.
	if d.job.Branch != "" {
		b.WriteString(dimStyle.Render(" · branch: " + d.job.Branch))
	}
	b.WriteString("\n\n")

	// Tab bar ("docs:" line).
	b.WriteString(d.renderTabs(w))
	b.WriteString("\n")

	// Agent action bar: an "agents:" line, then the stage timeline plus the
	// Done button on its own line beneath.
	b.WriteString(d.renderActionBar())
	b.WriteString("\n\n")

	// Body: the active viewer, set off from the chrome by a left rule.
	b.WriteString(d.renderBody())
	b.WriteString("\n\n")

	// Footer: scroll position + keys.
	b.WriteString(d.renderFooter())

	// Job-branch git-log strip: below the footer, mirroring the list view's
	// recent-activity strip position — read-only supplementary info, kept out
	// of the way of the job files and the chrome (see renderCommitStrip).
	b.WriteString(d.renderCommitStrip(w))

	return b.String()
}

// actionButton is one entry in the agents line: an agent-launch button,
// rendered "[key] Label" — see renderActionBar's doc comment.
type actionButton struct {
	key, label string
}

// renderActionBar draws the two-line action bar: an "agents:" line listing
// the five agents in agentOrder (always shown regardless of the job's
// stage — app.go's agentForKey is not stage-gated either), each in a
// consistent "[key] Label" format; then, on its own line beneath, a "stage:"
// label (mirroring "docs:" and "agents:") followed by the stage timeline
// (see renderStageTimeline) alongside the "[D] Done" mark-done button and
// "[j] mg-jdi" — a bigger, composite action like Done, not a
// single-agent launch, hence its own key rather than living in agentOrder.
//
// The stage timeline stays purely informational: an at-a-glance sense of
// where the job's files say it is in the ideal workflow, and how far it has
// come. It no longer restricts which buttons appear (launching any agent is
// not gated by stage). Done keeps its own colour (statusDoneStyle, the same
// green used for "done" status elsewhere) to flag that it's categorically
// different from the five launch actions — it archives the job via mg-done
// rather than starting a session.
//
// Narrow-width handling: this is designed against an 80-column baseline, and
// there isn't room there for five full "[key] Label" agent buttons —
// "Developer" alone is 9 characters. Once the full-label agents line
// would overflow d.width, every button's *label* — never its key, which must
// always stay reachable/visible — is truncated to share whatever room
// remains, using the same "…" convention truncate() (app.go) already applies
// to job titles. Wrapping to a further line was the brief's other offered
// option; truncation was chosen instead because it would add a row the fixed
// chrome-row budget in bodyHeight doesn't account for (it assumes a
// two-line action bar) — the same kind of alt-screen-clipping risk
// TestDetailBodyHeightShrinksForMultiLineStatus guards against for the
// footer, without that guard existing here.
func (d *detailView) renderActionBar() string {
	buttons := make([]actionButton, 0, len(agentOrder))
	for _, a := range agentOrder {
		m, ok := agentMeta[a]
		if !ok {
			// Unknown agent name — render it literally without a bound key.
			buttons = append(buttons, actionButton{key: "?", label: a})
			continue
		}
		buttons = append(buttons, actionButton{key: m.key, label: m.display})
	}

	w := d.width
	if w == 0 {
		w = 72
	}
	const sep = "  "
	const agentsLabel = "agents:"

	// Fixed cost: the "agents:" label, every button's "[key] " prefix, and
	// the separators between all elements — everything but the label text,
	// which is the only part allowed to shrink.
	fixed := len(agentsLabel)
	for _, btn := range buttons {
		fixed += len(sep) + len("["+btn.key+"] ")
	}
	labelBudget := w - fixed
	perLabel := 0
	if len(buttons) > 0 {
		perLabel = labelBudget / len(buttons)
	}

	var agentsLine strings.Builder
	agentsLine.WriteString(dimStyle.Render(agentsLabel))
	for _, btn := range buttons {
		label := btn.label
		if perLabel < len(label) {
			label = truncateToWidth(label, perLabel)
		}
		agentsLine.WriteString(sep)
		agentsLine.WriteString(accentStyle.Render("[" + btn.key + "]"))
		if label != "" {
			agentsLine.WriteString(" ")
			agentsLine.WriteString(label)
		}
	}

	var stageLine strings.Builder
	stageLine.WriteString(dimStyle.Render("stage:"))
	stageLine.WriteString(sep)
	stageLine.WriteString(renderStageTimeline(d.job.Stage()))
	stageLine.WriteString(sep)
	stageLine.WriteString(statusDoneStyle.Render("[D]"))
	stageLine.WriteString(" ")
	stageLine.WriteString(statusDoneStyle.Render("Done"))
	stageLine.WriteString(sep)
	stageLine.WriteString(accentStyle.Render("[j]"))
	stageLine.WriteString(" ")
	stageLine.WriteString(accentStyle.Render("just do it"))
	// A live running/stopped indicator right next to the button that starts
	// it: reuses the same job.ReadJDIStatus-backed jdiStatusBadge formatting
	// the job-list row already renders, so a user sitting in the detail view
	// — exactly where "j" is pressed, and where the already-running block
	// message appears — can see at a glance whether mg-jdi is still going,
	// not only from the list. The
	// running variant's animated frame comes from d.spinnerStep, threaded in
	// by the App's spinnerTickMsg handler, so it animates in sync with the
	// list row while a run is active. Like the list badge, the underlying
	// status has no polling timer of its own: it reads the sidecar fresh on
	// every render() call, so it updates whenever Bubble Tea next re-renders
	// (the spinner tick, or any keypress).
	if badge := jdiStatusBadge(d.job.Root, d.job, d.spinnerStep); badge != "" {
		stageLine.WriteString(sep)
		stageLine.WriteString(badge)
	}

	return agentsLine.String() + "\n" + stageLine.String()
}

// renderStageTimeline renders every job.Stages entry as a compact horizontal
// timeline: a checked, done-coloured marker for every stage behind the
// current one, a highlighted marker for the current stage, and a dim, hollow
// marker for stages still ahead. Replaces the old bare "stage: <name>" label
// with an at-a-glance sense of how far along the job actually is — including
// a verdict that bounced the job back to implement showing review/finished
// as still ahead, not behind, since job.Stage() already encodes that.
func renderStageTimeline(current job.Stage) string {
	idx := -1
	for i, s := range job.Stages {
		if s == current {
			idx = i
			break
		}
	}
	parts := make([]string, len(job.Stages))
	for i, s := range job.Stages {
		label := string(s)
		switch {
		case idx >= 0 && i < idx:
			parts[i] = statusDoneStyle.Render("✓ " + label)
		case i == idx:
			parts[i] = accentStyle.Render("● " + label)
		default:
			parts[i] = dimStyle.Render("○ " + label)
		}
	}
	return strings.Join(parts, dimStyle.Render(" → "))
}

// truncateToWidth is truncate() (app.go) with a floor of 0 instead of
// truncate's own "n <= 0 leaves s unchanged" behaviour: the action bar's
// narrow-width handling can legitimately compute a zero or negative label
// budget (six buttons is a lot to fit at 80 columns), in which case the
// label should disappear entirely — leaving only the reachable "[key]" — not
// render at full length as truncate(s, 0) would.
func truncateToWidth(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return truncate(s, n)
}

// renderTabs draws "docs: [brief] tasks implementation verdict" with the
// active tab highlighted and not-yet-written files dimmed.
func (d *detailView) renderTabs(width int) string {
	parts := make([]string, len(d.tabs))
	for i, t := range d.tabs {
		label := t.label
		if !t.exists {
			label = "(" + t.label + ")"
		}
		if i == d.cur {
			parts[i] = lipgloss.NewStyle().Bold(true).
				Background(accent).Foreground(lipgloss.Color("#ffffff")).
				Render(" " + label + " ")
		} else if !t.exists {
			parts[i] = dimStyle.Render(label)
		} else {
			parts[i] = label
		}
	}
	bar := strings.Join(parts, "  ")
	return dimStyle.Render("docs:") + "  " + bar
}

// renderFooter draws the scroll position, key hint, and (when set) the
// status message — a status must coexist with the hint, not replace it, so a
// user who just pressed "ctrl+r" or launched an agent still knows what keys
// exist.
//
// The one exception is a multi-line status (cmdErrorText's resolution
// diagnosis for a failed host-command lookup — see footerLines): that's a
// distinct, already-tested diagnostic case, not the "lost the legend" problem
// this coexistence handles, and appending the (fairly long) hint to it risks
// overflowing narrow terminals on top of an already multi-line block. It
// keeps replacing the hint entirely, same as before.
func (d *detailView) renderFooter() string {
	pos := d.current().viewer.Position()
	hint := "tab/1-6 files"
	if d.current().editable {
		// "e" only does anything on editable tabs (brief.md today), so the
		// hint is scoped to when it would actually work.
		hint += " · e edit"
	}
	hint += " · P push to origin · x/del remove job · ctrl+r refresh · esc back · q quit"

	if d.status != "" {
		if strings.Contains(d.status, "\n") {
			return statusStyle.Render(d.status)
		}
		return dimStyle.Render(fmt.Sprintf("%s   %s", pos, hint)) + "  " + statusStyle.Render(d.status)
	}
	return dimStyle.Render(fmt.Sprintf("%s   %s", pos, hint))
}

// renderCommitStrip renders the job-branch git-log strip below the footer —
// the detail view's counterpart of the list view's recent-activity strip,
// scoped to just this job's own branch (d.recentCommits, fed by
// refreshCommits). It reuses the same renderActivityLines formatter, so the
// visuals are byte-for-byte identical to the list's (the branch column
// included — "same visuals" per the brief, even though it is redundant when
// every line belongs to one branch). The blank spacer line before the first
// commit line is part of the strip's footprint, which bodyHeight's
// commitStripRows already budgets for.
//
// Renders nothing — not even the spacer — when the cache is empty (a
// non-repo project, a job with no branch, or a job branch with no commits
// yet): the strip is optional supplementary content, same as the list view's.
func (d *detailView) renderCommitStrip(w int) string {
	n := d.commitStripShown()
	if n == 0 {
		return ""
	}
	return "\n\n" + renderActivityLines(d.recentCommits[:n], w)
}
