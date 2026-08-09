package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lmuskalla/safecode/tui/internal/job"
	"github.com/lmuskalla/safecode/tui/internal/markdown"
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
}

// newDetailView loads all four files for job j at the given viewport size.
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
	d.loadTabs()
	return d
}

// jobFileNotWritten is the placeholder markdown shown for a file new-job.sh has
// not created or an agent has not filled in yet.
func filePlaceholder(label, filename string) string {
	return "# " + label + "\n\n_" + filename + " has not been written yet._"
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

// loadTab re-reads one tab's file from disk into t.content. If the tab is
// currently active, its viewer is rebuilt immediately; otherwise the viewer
// is left alone and the tab is marked stale so ensureCurrentSized catches it
// up lazily once it actually becomes active.
func (d *detailView) loadTab(i int) {
	t := &d.tabs[i]
	data, err := os.ReadFile(t.path)
	if err != nil {
		t.exists = false
		t.content = filePlaceholder(t.label, filepath.Base(t.path))
	} else {
		t.exists = true
		t.content = string(data)
	}
	if i == d.cur {
		t.viewer.SetContent(t.content)
		t.stale = false
	} else {
		t.stale = true
	}
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
	case "tab", "l", "right":
		if d.cur < len(d.tabs)-1 {
			d.cur++
		}
	case "shift+tab", "h", "left":
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
	case "down", "j":
		t.viewer.ScrollDown(1)
	case "up", "k":
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

// bodyWidth / bodyHeight compute the markdown viewport given the chrome the
// detail view draws around it (title, tab bar, footer).
func (d *detailView) bodyWidth() int {
	w := d.width - 2 // small left/right margin
	if w < 1 {
		w = 1
	}
	return w
}

func (d *detailView) bodyHeight() int {
	// title(1) + tab bar(1) + action bar(1) + blank(1) + body + blank(1) + footer(footerLines)
	// = (5 + footerLines) chrome rows around the body.
	h := d.height - 5 - d.footerLines()
	if h < 1 {
		h = 1
	}
	return h
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
	b.WriteString("\n")

	// Tab bar.
	b.WriteString(d.renderTabs(w))
	b.WriteString("\n")

	// Agent action bar keyed to the job's workflow stage.
	b.WriteString(d.renderActionBar())
	b.WriteString("\n\n")

	// Body: the active viewer.
	b.WriteString(d.current().viewer.View())
	b.WriteString("\n\n")

	// Footer: scroll position + keys.
	b.WriteString(d.renderFooter())

	return b.String()
}

// renderActionBar draws the action bar: all five agent buttons, always, in
// agentOrder — launching any of them is no longer gated by the job's stage
// (see app.go's agentForKey). "stage: <name>" is kept as an informational-only
// hint of where the job's files say it is in the ideal workflow; it no longer
// restricts which buttons appear.
//
// The "D" mark-done button is appended after a "│" separator and styled
// differently (statusDoneStyle, not accentStyle) since it is not an agent
// action — it archives the job via sc-done rather than launching a session.
func (d *detailView) renderActionBar() string {
	left := dimStyle.Render("stage: " + string(d.job.Stage()))

	buttons := make([]string, 0, len(agentOrder))
	for _, a := range agentOrder {
		m, ok := agentMeta[a]
		if !ok {
			// Unknown agent name — render it literally without a key.
			buttons = append(buttons, "[?] "+a)
			continue
		}
		key := accentStyle.Render("[" + m.key + "]")
		buttons = append(buttons, key+" "+m.display)
	}

	doneButton := statusDoneStyle.Render("[D]") + " " + statusDoneStyle.Render("Done")
	return left + "   " + strings.Join(buttons, "    ") + "    " + dimStyle.Render("│") + "  " + doneButton
}

// renderTabs draws [brief] tasks implementation verdict with the active tab
// highlighted and not-yet-written files dimmed.
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
	return bar
}

func (d *detailView) renderFooter() string {
	if d.status != "" {
		return statusStyle.Render(d.status)
	}
	pos := d.current().viewer.Position()
	hint := "tab/1-4 files · j/k scroll"
	if d.current().editable {
		// "e" only does anything on editable tabs (brief.md today), so the
		// hint is scoped to when it would actually work.
		hint += " · e edit"
	}
	hint += " · agent keys above · D mark done · ctrl+r refresh · esc back · q quit"
	return dimStyle.Render(fmt.Sprintf("%s   %s", pos, hint))
}
