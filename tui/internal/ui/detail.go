package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lmuskalla/manigot/tui/internal/git"
	"github.com/lmuskalla/manigot/tui/internal/job"
	"github.com/lmuskalla/manigot/tui/internal/markdown"
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

	// isLog marks the fifth "log" tab (TASK-9): unlike the four job files,
	// its content comes from mg-jdi's sidecar run.log
	// (job.ReadJDIRunLogTail(d.job.Root, d.job.Name)) rather than d.path —
	// it is not part of either job.OnCurrentBranch's working-tree read or
	// the off-branch git-show read (the sidecar isn't tracked in git at
	// all, tied only to the job name, not a branch), and it is never
	// editable.
	isLog bool

	// stale marks a viewer that is out of date with content and/or the
	// current body size, because the tab wasn't active when that changed —
	// either a resize (syncViewerSize, TASK-3) or a (re)load (loadTab, see
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
	// an agent launch (set by TASK-8) or report a launch error.
	status string

	// branchFlash marks the off-branch hint in the title+meta line (see
	// render) for blink treatment — set the moment branchGuard blocks an
	// action (blockedByBranch), so the *existing* red hint up top flashes to
	// draw the eye there instead of a second warning appearing at the
	// footer. Cleared a few seconds later by app.go's branchFlashDoneMsg
	// handler (see branchFlashDuration) so the flash reads as a momentary
	// reaction rather than a state that lingers until the user switches
	// branches.
	branchFlash bool

	// branchFlashGen is bumped every time blockedByBranch (re)triggers a
	// flash. Round-tripped through branchFlashDoneMsg so a delayed clear from
	// an earlier blocked attempt can't cut short a flash a later attempt
	// (re)started in the meantime — see app.go's branchFlashDoneMsg handler.
	branchFlashGen int
}

// newDetailView loads all four job files, plus TASK-9's fifth "log" tab, for
// job j at the given viewport size.
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
// on refresh (TASK-10): agents edit files outside the TUI, so the detail view
// must re-read to pick up their changes.
//
// Only the active tab's viewer is actually re-rendered here; the other three
// are cheap disk reads whose markdown render is deferred until they become
// active (see loadTab, ensureCurrentSized). Previously this eagerly rendered
// all four tabs' markdown — non-trivial cost (glamour.Render is not free) —
// on every job open and every return to the list, which is what made
// selecting a job and pressing esc/backspace to leave one feel laggy even
// after TASK-3 fixed the same problem for resize-triggered re-renders.
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
// A job on the current branch is read from the working tree (os.ReadFile) so
// uncommitted edits still show; a job discovered on another branch is read
// via `git show <Branch>:…`, so its four files appear in the tabs even though
// they don't exist under j.Dir in this working tree. Either way a missing
// file falls back to the placeholder.
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

// stripLeadingFrontmatter removes the leading H1 + "key: value" frontmatter
// block every job file's new-job.sh scaffold begins with, so glamour doesn't
// render a second title/metadata block duplicating what the detail view's
// own chrome (title+meta line) already shows (TASK-6 de-dup; app chrome was
// picked to own identity — see the brief).
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

// readFile reads one tab's file bytes from the working tree (current-branch
// job) or via git show (off-branch job). ok is false when the file isn't
// available through the chosen path.
func (d *detailView) readFile(t *fileTab) (data []byte, ok bool) {
	if d.job.OnCurrentBranch {
		b, err := os.ReadFile(t.path)
		if err != nil {
			return nil, false
		}
		return b, true
	}
	rel := filepath.ToSlash(filepath.Join(job.JobsRelDir, d.job.Name, filepath.Base(t.path)))
	b, err := git.ShowFile(d.job.Root, d.job.Branch, rel)
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

// blockedByBranch reacts to a branchGuard block: it clears any footer status
// (the reaction lives entirely in the off-branch hint already shown in the
// title+meta line, see render — not as a second message down at the
// footer) and flags branchFlash so that hint blinks. Returns the new
// branchFlashGen, which the caller (app.go's blockedByBranchCmd) rounds back
// through a delayed branchFlashDoneMsg to clear the flash again.
func (d *detailView) blockedByBranch() int {
	d.status = ""
	d.branchFlash = true
	d.branchFlashGen++
	d.syncViewerSize()
	return d.branchFlashGen
}

// syncViewerSize resizes the active tab's viewer to the current body
// dimensions immediately, and marks the other three as stale instead of
// eagerly re-rendering them too.
//
// Re-rendering all four tabs' markdown here was a major source of the
// reported input lag (TASK-1): this is called from setStatus, which itself
// fires on nearly every keypress in the detail view (including every agent
// launch attempt), so every keypress was paying for three re-renders that
// weren't even visible. Deferred tabs catch up lazily in render() the next
// time they actually become the active one (TASK-3).
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
	// = (7 + footerLines) chrome rows around the body.
	h := d.height - 7 - d.footerLines()
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
	// Branch: show which branch the job lives on, and flag it when that's a
	// different branch than the one currently checked out (so the user
	// understands why edits are guarded and knows to press "b" to switch).
	// The off-branch case renders in warnStyle's red — a plain foreground
	// color, no background fill — rather than dimStyle, so it reads as a
	// warning at a glance instead of blending into ordinary meta text; once
	// branchFlash is set (a blocked action was just attempted, see
	// blockedByBranch), it also blinks to draw the eye back to this exact
	// hint instead of a second message appearing at the footer.
	if d.job.Branch != "" {
		if d.job.OnCurrentBranch {
			b.WriteString(dimStyle.Render(" · branch: " + d.job.Branch))
		} else {
			style := warnStyle
			if d.branchFlash {
				style = style.Blink(true)
			}
			b.WriteString(style.Render(" · " + d.job.Branch + " (other branch — press b to switch)"))
		}
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
// consistent "[key] Label" format; then, on its own line beneath, the stage
// timeline (see renderStageTimeline) alongside the "[D] Done" mark-done
// button and "[j] mg-jdi" (TASK-12) — a bigger, composite action like Done,
// not a single-agent launch, hence its own key rather than living in
// agentOrder.
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
// "Product Owner" alone is 18 characters. Once the full-label agents line
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
	stageLine.WriteString(renderStageTimeline(d.job.Stage()))
	stageLine.WriteString(sep)
	stageLine.WriteString(statusDoneStyle.Render("[D]"))
	stageLine.WriteString(" ")
	stageLine.WriteString(statusDoneStyle.Render("Done"))
	stageLine.WriteString(sep)
	stageLine.WriteString(accentStyle.Render("[j]"))
	stageLine.WriteString(" ")
	stageLine.WriteString(accentStyle.Render("mg jdi"))
	// A live running/stopped indicator right next to the button that starts
	// it (TASK-2 of the "multiple jdi instances" job): reuses the same
	// job.ReadJDIStatus-backed jdiStatusBadge formatting the job-list row
	// already renders, so a user sitting in the detail view — exactly where
	// "j" is pressed, and where TASK-1's block message appears — can see at
	// a glance whether mg-jdi is still going, not only from the list. Like
	// the list badge, this has no polling timer of its own: it reads the
	// sidecar fresh on every render() call, so it updates whenever Bubble
	// Tea next re-renders (any keypress), not continuously.
	if badge := jdiStatusBadge(d.job.Root, d.job); badge != "" {
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
	hint := "tab/1-5 files"
	if d.current().editable {
		// "e" only does anything on editable tabs (brief.md today), so the
		// hint is scoped to when it would actually work.
		hint += " · e edit"
	}
	if !d.job.OnCurrentBranch {
		// "b" only does anything for a job that isn't already on the
		// checked-out branch — scoped the same way "e edit" is above.
		hint += " · b switch branch"
	}
	hint += " · agent keys above · D mark done · j run mg jdi · x/del remove job · ctrl+r refresh · esc back · q quit"

	if d.status != "" {
		if strings.Contains(d.status, "\n") {
			return statusStyle.Render(d.status)
		}
		return dimStyle.Render(fmt.Sprintf("%s   %s", pos, hint)) + "  " + statusStyle.Render(d.status)
	}
	return dimStyle.Render(fmt.Sprintf("%s   %s", pos, hint))
}
