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
	"github.com/lmuskalla/safecode/tui/internal/git"
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

	// currentBranch is the branch checked out in root right now (git.
	// CurrentBranch), refreshed alongside the job list so it never goes
	// stale relative to renderJobRow's "· <branch>" tags. Empty for a
	// detached HEAD or a non-repo project (job.Discover's working-tree-only
	// fallback) — both render nothing rather than an awkward empty label.
	currentBranch string

	// recentCommits backs the list header's read-only "recent activity"
	// strip: the last few commits across all local branches (git.
	// RecentCommits), refreshed alongside currentBranch. nil when the repo
	// has no commits yet or RecentCommits errors (e.g. a non-repo project)
	// — renderRecentActivity degrades to rendering nothing in that case.
	recentCommits []git.Commit

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
	a.currentBranch, _ = git.CurrentBranch(root) // "" on detached HEAD / non-repo
	a.refreshRecentCommits()
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

// checkoutMsg reports the outcome of the detail view's "b" switch-to-job-
// branch action (a `git checkout <branch>`, run off the UI thread via
// checkoutCmd so a slow git operation doesn't block rendering).
type checkoutMsg struct {
	branch string
	err    error
}

// Update handles window resizing and routes key presses to the active view.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w, h := msg.Width-2*uiPaddingX, msg.Height-2*uiPaddingY
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		a.width, a.height = w, h
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
	case checkoutMsg:
		if msg.err != nil {
			// Checkout refused (e.g. uncommitted changes it would clobber)
			// — surface git's own reason without touching the job list or
			// the open detail view.
			if a.detail != nil {
				a.detail.setStatus(cmdErrorText(msg.err))
			} else {
				a.status = cmdErrorText(msg.err)
			}
			return a, nil
		}
		// Checkout succeeded: the working tree now reflects msg.branch, so
		// re-discover (branch tags / OnCurrentBranch change for every job)
		// and, if a detail view is open, rebuild it against the same job id
		// so its tabs switch from git-show reads to the working tree.
		a.refreshJobs()
		if a.detail != nil {
			id := a.detail.job.ID
			if j, ok := a.jobByID(id); ok {
				a.detail = newDetailView(j, a.width, a.height)
				if idx := a.indexOfJob(id); idx >= 0 {
					a.cursor = idx
				}
				a.detail.setStatus("switched to " + msg.branch)
			} else {
				// The job vanished from the re-discovered list (e.g. it only
				// ever existed on the branch we just left) — fall back to
				// the list rather than show a stale detail view.
				a.detail = nil
				a.state = stateList
				a.status = "switched to " + msg.branch + ", but the job is no longer listed"
			}
		} else {
			// No detail view open — the list view's "back to main" quick
			// checkout. refreshJobs above already picked up the new
			// currentBranch; still surface a status so the checkout
			// (including an already-on-<branch> no-op, which git itself
			// treats as success) isn't silent.
			a.status = "switched to " + msg.branch
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

// View renders the active view, padded out to the terminal edge — see
// uiPaddingStyle.
func (a *App) View() string {
	var content string
	switch a.state {
	case stateDetail:
		content = a.detail.render()
	case stateNewJob:
		content = a.newJob.render()
	case stateSettings:
		content = a.settingsView.render()
	default:
		content = a.renderList()
	}
	return uiPaddingStyle.Render(content)
}

// selectedJob returns the job under the cursor, or false if the list is empty.
func (a *App) selectedJob() (job.Job, bool) {
	if len(a.jobs) == 0 || a.cursor < 0 || a.cursor >= len(a.jobs) {
		return job.Job{}, false
	}
	return a.jobs[a.cursor], true
}

// jobByID returns the job with the given ID from the current list, or false
// if it is no longer present — used by the checkoutMsg handler to rebuild the
// open detail view against the re-discovered copy of the same job after a
// branch switch (the old job.Job snapshot is stale: it still points at the
// pre-checkout Branch/OnCurrentBranch).
func (a *App) jobByID(id string) (job.Job, bool) {
	for _, j := range a.jobs {
		if j.ID == id {
			return j, true
		}
	}
	return job.Job{}, false
}

// indexOfJob returns the list index of the job with the given ID, or -1.
// Used alongside jobByID to keep the list cursor in sync with the job left
// open in the detail view after a re-discover (checkoutMsg, refresh).
func (a *App) indexOfJob(id string) int {
	for i, j := range a.jobs {
		if j.ID == id {
			return i
		}
	}
	return -1
}

// --- List view --------------------------------------------------------------

// recentActivityFloor / recentActivityCeiling bound how many commits the
// bottom-of-screen "recent activity" strip can show. The floor (1) is the
// strip's minimum footprint. The ceiling (5) is fixed at fetch time
// regardless of how many will actually render — see recentActivityShown for
// the part of this that scales with available room.
const (
	recentActivityFloor   = 1
	recentActivityCeiling = 5
)

// dashboardFixedChrome is the number of renderList rows that are always
// present outside the job rows and the recent-activity strip's own variable
// footprint: title line, blank spacer beneath it, "jobs" headline, divider,
// blank line before the footer, the footer itself, the blank spacer before
// the git log section, and "log" headline.
const dashboardFixedChrome = 8

// refreshRecentCommits re-reads the recent-activity strip from git, always
// fetching up to the ceiling. How many of those cached commits actually get
// rendered is decided later, at render time, by recentActivityShown — not
// here. This split matters because refreshRecentCommits runs before the first
// tea.WindowSizeMsg ever arrives (see NewApp), when a.width/a.height are still
// zero; sizing the *fetch* against that would size against a stale terminal
// height on first render. Fetching a fixed ceiling and deciding the display
// count at render time (once layout is known) avoids that hazard.
//
// Like currentBranch, an error (e.g. a non-repo project) degrades to an empty
// strip rather than surfacing in the status line — this is decorative,
// optional header content, not an action the user asked for.
func (a *App) refreshRecentCommits() {
	commits, err := git.RecentCommits(a.root, recentActivityCeiling)
	if err != nil {
		a.recentCommits = nil
		return
	}
	a.recentCommits = commits
}

// recentActivityShown returns how many of a.recentCommits should actually be
// rendered, given the current terminal height and job count. It scales
// between recentActivityFloor and recentActivityCeiling based on how much
// vertical room is spare, per the brief's resolution: a full list keeps the
// floor's 1-line footprint (never pushing job rows down further than the
// pre-existing header already did); a sparse list shows more, up to the
// ceiling.
//
// The fixed chrome outside the job rows and the strip itself — title line,
// blank spacer beneath it, "jobs" headline, divider, blank line before the
// footer, footer, the blank spacer before the git log section, "log"
// headline — is 8 rows, mirroring the same kind of budget
// detailView.bodyHeight documents for the detail view. spare is what's left
// of the terminal height once that chrome and every job row are accounted
// for.
//
// a.height == 0 (an App that has never received a tea.WindowSizeMsg, e.g.
// some existing tests) falls back to the floor, the same kind of guard
// renderList already applies to a.width == 0.
func (a *App) recentActivityShown() int {
	if a.height == 0 {
		return recentActivityFloor
	}
	spare := a.height - dashboardFixedChrome - len(a.jobs)
	n := clamp(spare, recentActivityFloor, recentActivityCeiling)
	if n > len(a.recentCommits) {
		// Fewer real commits than the computed count — render whatever's
		// available, same graceful-degrade rule renderRecentActivity already
		// applied before this task.
		n = len(a.recentCommits)
	}
	return n
}

// clamp bounds n to [lo, hi].
func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// spareHeaderRoom reports how many more header lines TASK-2's secondary fill
// (renderJobSummary) can add without pushing the job rows down, once the
// recent-activity strip has claimed its actual rendered footprint.
//
// renderList's activity block always uses at least 1 line (either the strip
// itself, or the blank spacer line it falls back to when there's nothing to
// show — see renderList), so the strip's real footprint is
// max(recentActivityShown(), 1), not recentActivityShown() alone (which can
// be 0 when there are no commits at all). Returns 0 — never negative, never
// computed — when a.height is unknown (never resized), matching the same
// "no real layout to measure against" fallback recentActivityShown uses.
func (a *App) spareHeaderRoom() int {
	if a.height == 0 {
		return 0
	}
	spare := a.height - dashboardFixedChrome - len(a.jobs)
	stripLines := a.recentActivityShown()
	if stripLines < 1 {
		stripLines = 1
	}
	room := spare - stripLines
	if room < 0 {
		room = 0
	}
	return room
}

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
	a.currentBranch, _ = git.CurrentBranch(a.root) // "" on detached HEAD / non-repo
	a.refreshRecentCommits()
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
	case "m":
		// Quick "back to main" checkout — the one friction point the brief
		// calls out (finishing a job branch and wanting a quick launch
		// against main without manually switching first). Not a generic
		// branch picker: "main" is the project's hardcoded base-branch
		// convention (scripts/new-job.sh), same as detail view's "b" reuses
		// git.Checkout/checkoutCmd. Runs as a tea.Cmd so a slow git
		// operation doesn't block rendering; checkoutMsg's a.detail == nil
		// branch reports the outcome to a.status.
		return a, a.checkoutCmd("main")
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
		a.currentBranch, _ = git.CurrentBranch(a.root) // "" on detached HEAD / non-repo
		a.refreshRecentCommits()
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
			if status, blocked := a.branchGuard(); blocked {
				a.detail.setStatus(status)
				return a, nil
			}
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
		if status, blocked := a.branchGuard(); blocked {
			a.detail.setStatus(status)
			return a, nil
		}
		cmd, err := a.doneCmd()
		if err != nil {
			a.detail.setStatus(cmdErrorText(err))
			return a, nil
		}
		return a, cmd
	case "b":
		// Switch to this job's branch (the mechanism the branch-mismatch
		// guards on launch/edit/done point the user at). Not gated on
		// job.OnCurrentBranch — that flag is a discovery-time snapshot that
		// could be stale, so this always dispatches the checkout and lets
		// git itself decide (a no-op "Already on '<branch>'" checkout still
		// succeeds). Runs as a tea.Cmd so a slow git operation doesn't block
		// the UI.
		if a.detail.job.Branch == "" {
			a.detail.setStatus("no branch known for this job")
			return a, nil
		}
		return a, a.checkoutCmd(a.detail.job.Branch)
	}
	// Action bar: fire the agent whose key matches, if it is valid for the
	// current job's stage.
	if agent := a.agentForKey(msg.String()); agent != "" {
		if status, blocked := a.branchGuard(); blocked {
			a.detail.setStatus(status)
			return a, nil
		}
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

// branchGuard reports whether the open job's branch differs from the branch
// actually checked out right now, and if so a status message pointing the
// user at "b" — the guard behind the three mutating actions (launch agent,
// "e" edit, "D" mark-done) named in the "keep-track-of-jobs" brief's coupled
// scope: none of them may silently run against the wrong branch's working
// tree once discovery is cross-branch.
//
// The current branch is re-checked fresh via git.CurrentBranch rather than
// trusted from job.OnCurrentBranch's discovery-time snapshot, since "b" (or a
// checkout run outside the TUI) may have moved it since the job list was last
// read.
//
// A job with no known Branch — job.Discover's working-tree-only fallback for
// a project that isn't a git repo — is never guarded: there is nothing to
// compare against, and OnCurrentBranch is unconditionally true there, so the
// three actions keep their exact pre-guard behaviour for non-git projects.
func (a *App) branchGuard() (status string, blocked bool) {
	j := a.detail.job
	if j.Branch == "" {
		return "", false
	}
	cur, _ := git.CurrentBranch(a.root) // "" on detached HEAD / not-a-repo
	if cur == j.Branch {
		return "", false
	}
	curLabel := cur
	if curLabel == "" {
		curLabel = "(detached HEAD)"
	}
	return fmt.Sprintf("on branch %s, this job is on %s — press b to switch", curLabel, j.Branch), true
}

// checkoutCmd returns the tea.Cmd behind the "b" switch-to-job-branch action:
// it runs `git checkout branch` in a.root off the UI goroutine (unlike
// editCmd/doneCmd, this does not need tea.ExecProcess — there is no
// interactive process to hand the terminal to, just a git call) and reports
// the outcome as a checkoutMsg once it returns.
func (a *App) checkoutCmd(branch string) tea.Cmd {
	root := a.root
	return func() tea.Msg {
		err := git.Checkout(root, branch)
		return checkoutMsg{branch: branch, err: err}
	}
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

	// Title line: "safecode - <project> - on <branch>".
	title := "safecode - " + shortRoot(a.root)
	if a.currentBranch != "" {
		title += " - on " + a.currentBranch
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Jobs section.
	b.WriteString(headerStyle.Render("jobs"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", w)))
	b.WriteString("\n")

	// Rows / empty state. The most prominent text a zero-job user sees must
	// be the path to creating their first job ("n"), not the path to
	// quitting — so the key itself is emphasized (accentStyle, the same
	// treatment the header gives the current branch) and "quit" isn't
	// mentioned at all here; it's still in the footer hint below like every
	// other key, just not competing for attention in this dedicated message.
	if len(a.jobs) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  No jobs yet — press "))
		b.WriteString(accentStyle.Render("n"))
		b.WriteString(dimStyle.Render(" to create your first one."))
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

	// Git log section: kept below the footer, out of the way, since it's
	// read-only supplementary info rather than something the job list
	// depends on.
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render("log"))
	b.WriteString("\n")
	// The activity strip's line count is recentActivityShown() — computed
	// from the actual spare room the terminal height leaves once the fixed
	// chrome and every job row are accounted for, so it can grow past its
	// one-line floor on a sparse list without ever making the total render
	// taller than the terminal.
	if activity := a.renderRecentActivity(w); activity != "" {
		b.WriteString(activity)
	} else {
		b.WriteString("\n")
	}
	// Secondary empty-space fill (TASK-2): only rendered when the strip above
	// still leaves spare room after claiming its actual footprint — see
	// spareHeaderRoom.
	if summary := a.renderJobSummary(); summary != "" {
		b.WriteString(summary)
	}

	return b.String()
}

// renderRecentActivity renders the list header's read-only "recent activity"
// strip: up to recentActivityShown() commits across all local branches, one
// compact dimmed line each, most-recent first (see git.RecentCommits for the
// dedup/attribution rules). Renders nothing — not even a heading — when
// a.recentCommits is empty (a fresh repo, or refreshRecentCommits degrading a
// non-repo project's error away) or when recentActivityShown() computes zero,
// matching the rest of the header's optional-content handling. This is fixed
// and non-interactive: no scrolling, no drill-down, per the brief's explicit
// rejection of a git log viewer.
//
// Deliberately no separate "recent activity" label line even when several
// commits are shown: recentActivityShown()'s line budget is exactly the
// commit count (see its doc comment), so a label would either overrun the
// budget (risking pushing job rows down, the one thing this strip must never
// do) or eat one commit slot to stay within it. The existing dim styling
// plus the strip's fixed position directly under the branch line is the
// grouping cue instead — spacing/color, not a border or a heading, per the
// brief's "no boxes-within-boxes, no decorative borders" rule.
func (a *App) renderRecentActivity(w int) string {
	n := a.recentActivityShown()
	if n == 0 {
		return ""
	}
	commits := a.recentCommits[:n]

	const hashW, relW, branchW = 7, 10, 16
	// Subject gets whatever's left after the fixed-width columns and their
	// spacing, same truncate() job titles use so a long commit message can't
	// wrap the line.
	subjectW := w - hashW - relW - branchW - 6
	if subjectW < 12 {
		subjectW = 12
	}
	var b strings.Builder
	for _, c := range commits {
		line := pad(c.ShortHash, hashW) + "  " +
			pad(truncate(c.Subject, subjectW), subjectW) + "  " +
			pad(c.RelTime, relW) + "  " +
			truncate(c.Branch, branchW)
		b.WriteString(dimStyle.Render(strings.TrimRight(line, " ")))
		b.WriteString("\n")
	}
	return b.String()
}

// renderJobSummary renders the header's secondary empty-space fill (TASK-2):
// a compact "<n> open · <n> done" counts line. It's the fallback for the
// case the recent-activity strip alone can't fill — a sparse list whose
// strip is already at its ceiling, or a repo with few/no real commits to
// show — costs nothing to compute (a.jobs is already fully loaded) and needs
// no new data source, unlike the brief's other offered option (a per-job
// summary pane).
//
// Only renders when spareHeaderRoom reports room left after the strip's
// actual footprint, and only for a non-empty list — an empty list gets its
// own dedicated invitation (TASK-10, renderList's zero-jobs branch), where a
// "0 open · 0 done" line would just be noise ahead of it.
func (a *App) renderJobSummary() string {
	if len(a.jobs) == 0 || a.spareHeaderRoom() < 1 {
		return ""
	}
	var open, done int
	for _, j := range a.jobs {
		if j.Status == "done" {
			done++
		} else {
			open++
		}
	}
	return dimStyle.Render(fmt.Sprintf("%d open · %d done", open, done)) + "\n"
}

// renderJobRow renders one job as a single (possibly highlighted) line.
//
// A job living on a branch other than the one currently checked out gets a
// compact trailing "· <branch>" tag (not a new column, so the fixed column
// layout stays stable) so the user can tell at a glance which jobs are
// "elsewhere" before opening one.
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
	if j.Branch != "" && !j.OnCurrentBranch {
		line += "  " + dimStyle.Render("· "+j.Branch)
	}
	if selected {
		return selectedStyle.Render("▶ " + line)
	}
	return dimStyle.Render("  ") + line
}

// footer is the bottom help/status line.
// footer renders the dim key hint and, when a.status is set (e.g. right
// after "ctrl+r"), the status alongside it rather than replacing it — a
// status message must never leave the user not knowing what keys exist.
func (a *App) footer() string {
	hint := "↑/↓ navigate · enter view · o quick · n new · m main · s settings · ctrl+r refresh · q quit"
	if a.status != "" {
		return dimStyle.Render(hint) + "  " + statusStyle.Render(a.status)
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
