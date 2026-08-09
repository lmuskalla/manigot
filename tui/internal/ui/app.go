// Package ui holds the Bubble Tea models that make up the safecode TUI.
//
// App is the root model; it owns the discovered job list and routes between the
// list view (list.go), the job detail view (detail.go) and overlays for actions.
package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/safecode/tui/internal/config"
	"github.com/lmuskalla/safecode/tui/internal/editor"
	"github.com/lmuskalla/safecode/tui/internal/hostcmd"
	"github.com/lmuskalla/safecode/tui/internal/job"
	"github.com/lmuskalla/safecode/tui/internal/launch"
	"github.com/lmuskalla/safecode/tui/internal/resolve"
)

// appState selects which view is active. More states (detail, form) are added
// by later tasks.
type appState int

const (
	stateList appState = iota
	stateDetail
	stateNewJob   // "n" from the list — create a job via the host sc-job command
	stateSettings // "s" from the list — edit the persisted TUI settings
)

// App is the root Bubble Tea model.
type App struct {
	root  string
	jobs  []job.Job
	state appState

	cursor int
	width  int
	height int

	// settings holds the persisted TUI preferences (editor, agent tool). It
	// is loaded once at startup and updated in place whenever the settings
	// form is submitted.
	settings config.Settings

	// detail is non-nil while state == stateDetail.
	detail *detailView

	// newJob is non-nil while state == stateNewJob.
	newJob *newJobView

	// settingsView is non-nil while state == stateSettings.
	settingsView *settingsView

	// status is a transient one-line message shown in the footer (e.g. after
	// running sc-job or an agent).
	status string
}

// NewApp builds the root model from a discovered job list. Settings are
// loaded from disk (see config.Load); a load failure (e.g. a corrupt
// tui-settings.json) is non-fatal — the app starts with default settings and
// surfaces the error in the footer instead.
func NewApp(root string, jobs []job.Job) *App {
	a := &App{root: root, jobs: jobs, state: stateList}
	settings, err := config.Load()
	a.settings = settings
	if err != nil {
		a.status = cmdErrorText(err)
	}
	return a
}

// Init starts the program. No initial commands are needed.
func (a *App) Init() tea.Cmd { return nil }

// editorDoneMsg reports the outcome of the "e" edit-shortcut's tea.ExecProcess
// once the suspended editor process returns.
type editorDoneMsg struct {
	path string
	err  error
}

// doneMsg reports the outcome of the "D" mark-done shortcut's
// tea.ExecProcess once the suspended finish-job.sh run returns.
type doneMsg struct {
	err error
}

// Update handles window resizing and routes key presses to the active view.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if a.detail != nil {
			a.detail.resize(a.width, a.height)
		}
		if a.newJob != nil {
			a.newJob.resize(a.width, a.height)
		}
		if a.settingsView != nil {
			a.settingsView.resize(a.width, a.height)
		}
		return a, nil
	case editorDoneMsg:
		if a.detail != nil {
			if msg.err != nil {
				a.detail.setStatus(cmdErrorText(msg.err))
			} else {
				a.detail.reloadCurrent()
				a.detail.setStatus("edited " + filepath.Base(msg.path))
			}
		}
		return a, nil
	case doneMsg:
		// finish-job.sh's exit code is not a reliable success/failure signal:
		// every one of its read -rp confirmation prompts does `exit 0` on
		// decline, not just the happy path. So regardless of msg.err, always
		// fall back to refreshing the job list from disk and returning to it
		// — a job that got archived is simply gone from the re-read list, one
		// that was declined or failed is still there. A non-zero exit (the
		// script itself erroring, e.g. uncommitted changes) still surfaces
		// through cmdErrorText first, same as any other host-command failure.
		a.refreshJobs()
		a.detail = nil
		a.state = stateList
		if msg.err != nil {
			a.status = cmdErrorText(msg.err)
		} else {
			a.status = "refreshed"
		}
		return a, nil
	case tea.KeyMsg:
		// Global keys handled in every state.
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		}
		switch a.state {
		case stateList:
			return a.updateList(msg)
		case stateDetail:
			return a.updateDetail(msg)
		case stateNewJob:
			return a.updateNewJob(msg)
		case stateSettings:
			return a.updateSettings(msg)
		}
	}
	return a, nil
}

// View renders the active view.
func (a *App) View() string {
	switch a.state {
	case stateDetail:
		return a.detail.render()
	case stateNewJob:
		return a.newJob.render()
	case stateSettings:
		return a.settingsView.render()
	default:
		return a.renderList()
	}
}

// selectedJob returns the job under the cursor, or false if the list is empty.
func (a *App) selectedJob() (job.Job, bool) {
	if len(a.jobs) == 0 || a.cursor < 0 || a.cursor >= len(a.jobs) {
		return job.Job{}, false
	}
	return a.jobs[a.cursor], true
}

// --- List view --------------------------------------------------------------

// refreshJobs re-reads the job list from disk (job.Discover: one os.ReadDir
// plus one small brief.md read per job) and clamps the cursor so a job that
// was archived mid-session doesn't leave it out of range. It does not touch
// an open detail view — see refresh for that, and updateDetail's
// "esc"/"backspace" case, which uses refreshJobs alone because the detail
// view it would otherwise reload is about to be discarded anyway.
func (a *App) refreshJobs() {
	if jobs, err := job.Discover(a.root); err == nil {
		a.jobs = jobs
	}
	if a.cursor > 0 && a.cursor >= len(a.jobs) {
		a.cursor = len(a.jobs) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// refresh does refreshJobs, plus reloads the open detail view's files (if
// any) so changes an agent made outside the TUI show up — used by "ctrl+r".
func (a *App) refresh() {
	a.refreshJobs()
	if a.detail != nil {
		a.detail.reload()
	}
}

// updateList handles keys while the job list is showing.
func (a *App) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a.status = "" // status is transient — cleared on every key unless a case sets it
	switch msg.String() {
	case "q", "esc":
		return a, tea.Quit
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}
	case "down", "j":
		if a.cursor < len(a.jobs)-1 {
			a.cursor++
		}
	case "home", "g":
		a.cursor = 0
	case "end", "G":
		if len(a.jobs) > 0 {
			a.cursor = len(a.jobs) - 1
		}
	case "ctrl+r":
		a.refresh()
		a.status = fmt.Sprintf("refreshed · %d job(s)", len(a.jobs))
	case "enter", "l", "right":
		if j, ok := a.selectedJob(); ok {
			a.detail = newDetailView(j, a.width, a.height)
			a.state = stateDetail
		}
	case "n":
		// Create a new job via the host sc-job command.
		a.newJob = newNewJobView(a.width, a.height)
		a.state = stateNewJob
	case "s":
		// Edit the persisted TUI settings (editor, agent tool).
		a.settingsView = newSettingsView(a.settings, a.width, a.height)
		a.state = stateSettings
	case "o":
		// Launch a bare safecode session (no agent, no job) in a detached
		// new terminal — a quick ad-hoc change that doesn't belong to any
		// specific job's workflow. Mirrors updateDetail's agent-launch
		// reporting so the two launch paths feel consistent in the footer.
		desc, err := launch.Quick(a.root, a.settings.ToolValue())
		if err != nil {
			a.status = cmdErrorText(err)
		} else {
			a.status = "→ quick session in " + desc
		}
	}
	return a, nil
}

// updateSettings handles keys in the settings form.
func (a *App) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.settingsView.update(msg) {
	case stCancel:
		a.settingsView = nil
		a.state = stateList
	case stSubmit:
		s := a.settingsView.settingsValue()
		if err := config.Save(s); err != nil {
			a.settingsView.status = cmdErrorText(err)
			return a, nil
		}
		a.settings = s
		a.settingsView = nil
		a.status = "settings saved"
		a.state = stateList
	}
	return a, nil
}

// updateNewJob handles keys in the new-job form.
func (a *App) updateNewJob(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.newJob.update(msg) {
	case njCancel:
		a.newJob = nil
		a.state = stateList
	case njSubmit:
		title := strings.TrimSpace(a.newJob.title.Value())
		if title == "" {
			a.newJob.status = "title is required"
			return a, nil
		}
		typ := a.newJob.typeValue()
		_, err := hostcmd.NewJob(title, typ, a.root)
		if err != nil {
			a.newJob.status = cmdErrorText(err)
			return a, nil
		}
		// Refresh the list so the new job appears, then return to it.
		if jobs, derr := job.Discover(a.root); derr == nil {
			a.jobs = jobs
		}
		a.cursor = 0 // newest first after Discover's date-desc sort
		a.status = "created \"" + title + "\" (" + typ + ")"
		a.newJob = nil
		a.state = stateList
	}
	return a, nil
}

// updateDetail handles keys while a job's files are showing.
func (a *App) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		// Return to the list. Only the job list itself needs refreshing
		// (a job may have been archived, or its status/title changed) —
		// reloading the detail view we're about to throw away would be pure
		// waste, and used to be a real source of lag (see refreshJobs' doc).
		a.refreshJobs()
		a.detail = nil
		a.state = stateList
		a.status = "refreshed"
		return a, nil
	case "q":
		return a, tea.Quit
	case "ctrl+r":
		a.refresh()
		a.detail.setStatus("refreshed")
		return a, nil
	case "e":
		// Only the tabs marked editable in jobFiles (currently brief.md
		// only) respond — for any other tab this falls through to the
		// default key handling below, same as any other unbound key.
		if a.detail.current().editable {
			cmd, err := a.editCmd()
			if err != nil {
				a.detail.setStatus(cmdErrorText(err))
				return a, nil
			}
			return a, cmd
		}
	case "D":
		// Mark done: not an agent action, so it is handled here rather than
		// falling into the agentForKey dispatch below.
		cmd, err := a.doneCmd()
		if err != nil {
			a.detail.setStatus(cmdErrorText(err))
			return a, nil
		}
		return a, cmd
	}
	// Action bar: fire the agent whose key matches, if it is valid for the
	// current job's stage.
	if agent := a.agentForKey(msg.String()); agent != "" {
		desc, err := launch.Agent(agent, a.detail.job.ID, a.root, a.settings.ToolValue())
		if err != nil {
			a.detail.setStatus(cmdErrorText(err))
		} else {
			a.detail.setStatus("→ " + agent + " in " + desc)
		}
		return a, nil
	}
	a.detail.setStatus("")
	a.detail.update(msg)
	return a, nil
}

// editCmd resolves an editor and returns the tea.Cmd that opens it on the
// active tab's file. tea.ExecProcess suspends the Bubble Tea renderer (unlike
// launch.Agent's detached new-window spawn) and runs the child in the same
// terminal, resuming the TUI once it exits; the result is delivered back as
// an editorDoneMsg. An error here means the editor itself could not be
// resolved (see editor.Resolve) — the caller surfaces it directly.
func (a *App) editCmd() (tea.Cmd, error) {
	path := a.detail.current().path
	cmd, err := editor.Command(path, a.settings.Editor)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{path: path, err: err}
	}), nil
}

// doneCmd resolves the sc-done invocation for the open job and returns the
// tea.Cmd that runs it. Like editCmd, this goes through tea.ExecProcess —
// finish-job.sh's several read -rp confirmations need a real interactive
// terminal, unlike launch.Agent's detached new-window spawn used for agents.
// An error here means the command itself could not be resolved (see
// hostcmd.DoneCommand) — the caller surfaces it directly.
func (a *App) doneCmd() (tea.Cmd, error) {
	cmd, err := hostcmd.DoneCommand(a.detail.job.Name, a.root)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return doneMsg{err: err}
	}), nil
}

// agentForKey returns the agent name whose action-bar key equals k ("" if no
// agent uses that key). All five agents are always eligible, regardless of
// the open job's current stage: the brief this shipped under
// ("launch agents without workflow") is explicitly about not forcing the
// ideal-workflow stage order — a user may write brief.md and tasks.md by hand
// and go straight to @developer, for example.
func (a *App) agentForKey(k string) string {
	if a.detail == nil {
		return ""
	}
	for _, agent := range agentOrder {
		if m, ok := agentMeta[agent]; ok && m.key == k {
			return agent
		}
	}
	return ""
}

// columnWidths returns the fixed column widths used by the list and detail
// headers so rows line up. Kept here so both views agree.
type columnWidths struct {
	id, status, typ, date, title int
}

func listColumns() columnWidths {
	return columnWidths{id: 8, status: 6, typ: 8, date: 12, title: 0}
}

// renderList draws the header, job rows, and footer help.
func (a *App) renderList() string {
	w := a.width
	if w == 0 {
		w = 72
	}
	cols := listColumns()
	titleColsWidth := cols.id + cols.status + cols.typ + cols.date + 4*3 // 3 spaces between cols
	cols.title = w - titleColsWidth
	if cols.title < 16 {
		cols.title = 16
	}

	var b strings.Builder

	// Header.
	b.WriteString(titleStyle.Render("safecode"))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("jobs in " + shortRoot(a.root)))
	b.WriteString("\n\n")

	// Column header row.
	header := headerStyle.Render(pad("ID", cols.id)) + "  " +
		headerStyle.Render(pad("STATUS", cols.status)) + "  " +
		headerStyle.Render(pad("TYPE", cols.typ)) + "  " +
		headerStyle.Render(pad("DATE", cols.date)) + "  " +
		headerStyle.Render("TITLE")
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", w)))
	b.WriteString("\n")

	// Rows / empty state.
	if len(a.jobs) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  No jobs yet. Press q to quit."))
		b.WriteString("\n")
	} else {
		for i, j := range a.jobs {
			row := a.renderJobRow(j, cols, i == a.cursor)
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	// Footer.
	b.WriteString("\n")
	b.WriteString(a.footer())

	return b.String()
}

// renderJobRow renders one job as a single (possibly highlighted) line.
func (a *App) renderJobRow(j job.Job, cols columnWidths, selected bool) string {
	status := statusOpenStyle.Render(pad(j.Status, cols.status))
	if j.Status == "done" {
		status = statusDoneStyle.Render(pad(j.Status, cols.status))
	}
	cells := []string{
		pad(j.ID, cols.id),
		status,
		pad(j.Type, cols.typ),
		pad(j.Date, cols.date),
		truncate(j.Title, cols.title),
	}
	line := strings.Join(cells, "  ")
	if selected {
		return selectedStyle.Render("▶ " + line)
	}
	return dimStyle.Render("  ") + line
}

// footer is the bottom help/status line.
func (a *App) footer() string {
	hint := "↑/↓ navigate · enter view · o quick · n new · s settings · ctrl+r refresh · q quit"
	if a.status != "" {
		return statusStyle.Render(a.status)
	}
	return dimStyle.Render(hint)
}

// --- helpers ----------------------------------------------------------------

// cmdErrorText formats an error from a host command for a status line.
//
// A failed command *resolution* is a setup problem, not a transient error, so it
// gets the full diagnosis over three lines: what was missing, every strategy
// that was tried in order, and how to fix it. Anything else stays a one-liner.
func cmdErrorText(err error) string {
	if err == nil {
		return ""
	}
	var nf *resolve.NotFoundError
	if errors.As(err, &nf) {
		return "error: " + nf.Spec.Label + " not found — the safecode launchers are not installed under a name the TUI knows\n" +
			"tried: " + nf.TriedList() + "\n" +
			"fix:   " + nf.Hint()
	}
	return "error: " + err.Error()
}

func shortRoot(root string) string {
	if i := strings.LastIndex(root, "/"); i >= 0 {
		return root[i+1:]
	}
	return root
}

func pad(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	if n > 0 && len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}
