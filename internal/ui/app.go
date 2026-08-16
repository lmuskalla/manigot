// Package ui holds the Bubble Tea models that make up the manigot TUI.
//
// App is the root model; it owns the discovered job list and routes between the
// list view (list.go), the job detail view (detail.go) and overlays for actions.
package ui

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/internal/agentlist"
	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/editor"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/launch"
	"github.com/lmuskalla/manigot/internal/project"
)

// appState selects which view is active. More states (detail, form) are added
// by later tasks.
type appState int

const (
	stateList appState = iota
	stateDetail
	stateNewJob   // "n" from the list — create a job via the in-process job.CreateJob
	stateSettings // "s" from the list — edit the persisted TUI settings
	stateAgents   // "a" from the list — pick and launch any agent as a jobless quick session
	stateConfirm  // "D" / "delete" / "x" from the detail — confirm a destructive lifecycle action
)

// App is the root Bubble Tea model.
type App struct {
	root  string
	jobs  []job.Job
	state appState

	// list is the job list view (stateList) — see list.go. It owns the
	// cursor, the recent-activity strip data, and the list rendering; App
	// feeds it the job list, settings, and cross-view state on each render.
	list *listView

	// width / height are the viewport size, used to construct and resize the
	// overlay views (detail, new-job, settings, agents picker, confirm) and
	// passed to the list view on each render.
	width  int
	height int

	// settings holds the persisted TUI preferences (editor, recent-activity
	// count, subscription profile). It is loaded once at startup and updated
	// in place whenever the settings form is submitted.
	settings config.Settings

	// projectSettings holds the project-scoped conventions (base branch),
	// read from .manigot/manigot.json in the target project root. Unlike
	// settings, this file is meant to be committed and shared across a
	// team — it travels with the project, not the user. Loaded once at
	// startup and updated in place whenever the settings form is submitted.
	projectSettings project.Settings

	// detail is non-nil while state == stateDetail.
	detail *detailView

	// newJob is non-nil while state == stateNewJob.
	newJob *newJobView

	// settingsView is non-nil while state == stateSettings.
	settingsView *settingsView

	// agentsPicker is non-nil while state == stateAgents.
	agentsPicker *agentsPickerView

	// confirm is non-nil while state == stateConfirm — the destructive-action
	// confirmation for "D" (mark done) and "delete"/"x" (delete).
	confirm *confirmView

	// status is a transient one-line message shown in the footer (e.g. after
	// running mg-job or an agent).
	status string

	// jdiSeen tracks, per job Name, the last job.JDIState observed for that
	// job's mg-jdi status sidecar — the TUI's own stop-notification dedup. The very first observation of any given job
	// (no prior entry) always just seeds this map rather than ringing the
	// bell, even if that first-seen status is already stopped: a restarted
	// TUI re-observing an already-stopped job isn't a new event worth
	// alerting on. Reset (empty) on every TUI restart — in-memory only, no
	// new event-streaming subsystem or persisted state.
	//
	// jdiAlreadyRunning's launch-block guard also falls back to this same
	// map when no on-disk sidecar exists yet — see jdiSeenAt below for why that fallback has its
	// own expiry, independent of this dedup's own (unrelated) purpose.
	jdiSeen map[string]job.JDIState

	// jdiSeenAt records when each jdiSeen entry was last written, so
	// jdiAlreadyRunning's fallback to jdiSeen (used only while no on-disk
	// sidecar exists) can expire it after jdiSeenFallbackTTL rather than
	// trusting it forever. Without this, a "j" press whose mg-jdi process
	// then crashed or was killed before ever writing its first status file
	// left a JDIRunning entry in jdiSeen with nothing to ever clear it —
	// permanently blocking a re-launch for the rest of the TUI session, with
	// no visible indicator anywhere (the sidecar-driven badges correctly show
	// nothing running) explaining why. Keyed the same as jdiSeen.
	jdiSeenAt map[string]time.Time

	// spinnerStep is the current activity-indicator frame index (see
	// activity.go), advanced once per spinnerTickMsg while any mg-jdi run is
	// active. Threaded into the running badges (jdiStatusBadge) and into the
	// open detail view (detailView.spinnerStep) so both the list row and the
	// action-bar badge animate off the same counter. 0 (the zero value) is
	// the first frame; nothing special about it.
	spinnerStep int

	// spinnerTicking guards the activity-indicator tick chain: true while a
	// spinnerTickMsg is scheduled or in flight, so no second concurrent chain
	// can be started (startSpinnerIfRunning is a no-op while it's set). Cleared
	// by the tick handler itself the moment no job is running any more — the
	// chain self-terminates, so a run ending is all it takes to stop the
	// redraws and return the app to its idle (no timer) behaviour.
	spinnerTicking bool
}

// spinnerTickMsg advances the activity indicator one frame. It is the app's
// only timer-driven message — the one narrow exception to "no separate
// timer-driven tick" (see pollJDIBell's doc, and the README's "mg jdi status
// & log" section): a ~activityInterval redraw that exists only while an
// mg-jdi run is active, so a watched run visibly indicates it's alive instead
// of sitting as a static badge.
type spinnerTickMsg struct{}

// hostGitTimeout bounds the TUI's background git cmds (push-to-origin, the
// auto-commit after a brief.md edit). These run off the UI goroutine, so an
// unbounded git call — most realistically a stalled network push — would hang
// the app's command channel forever (GIT_TERMINAL_PROMPT=0 covers only the
// credential case). The interactive session and mg done/mg delete keep no
// timeout: the user is waiting on git there. The mg-jdi probes use their own,
// shorter timeout (see cmd/mg/jdi.go).
const hostGitTimeout = 30 * time.Second

// jdiSeenFallbackTTL bounds how long jdiAlreadyRunning trusts a jdiSeen
// JDIRunning entry that has no corroborating on-disk sidecar file at all.
// It exists to bridge the real race this fallback is for — the gap between
// launch.Jdi's Start() returning and mg-jdi's own first WriteJDIStatus call,
// which happens before mg-jdi invokes its first agent, so
// normally within seconds (a git checkout, not a full agent run) — while
// still being short enough that a launch whose mg-jdi process crashed or was
// killed before ever writing that first status file recovers on its own
// within the same TUI session, rather than requiring a restart. Deliberately
// much shorter than jdiRunningStaleAfter (job.ReadJDIStatus's own 30-minute
// staleness window for a sidecar file that *does* exist): that window has to
// stay generous enough not to mistake one long agent call for a killed
// process, but this one only has to survive process startup, not an entire
// agent invocation.
const jdiSeenFallbackTTL = 2 * time.Minute

// jdiNow is time.Now, indirected so tests can simulate the passage of time
// for jdiSeenFallbackTTL without an actual sleep.
var jdiNow = time.Now

// NewApp builds the root model from a discovered job list. Settings are
// loaded from disk (see config.Load); a load failure (e.g. a corrupt
// tui-settings.json) is non-fatal — the app starts with default settings and
// surfaces the error in the footer instead.
func NewApp(root string, jobs []job.Job) *App {
	a := &App{root: root, jobs: jobs, state: stateList, list: newListView(root), jdiSeen: map[string]job.JDIState{}, jdiSeenAt: map[string]time.Time{}}
	settings, err := config.Load()
	a.settings = settings
	if err != nil {
		a.status = cmdErrorText(err)
	}
	a.list.currentBranch, _ = git.CurrentBranch(root) // "" on detached HEAD / non-repo
	// Settings must be loaded before the initial recent-commits fetch so the
	// strip honors the configured maximum count on the very first render —
	// not just after the first settings submit.
	a.refreshRecentCommits()
	// Project settings (base branch) live in the target project, not the
	// manigot checkout, so a missing file is the normal pre-first-save state
	// and project.Load already degrades it to a zero value — only a real I/O
	// or parse error surfaces here. Append rather than overwrite a config
	// load error so both can be seen.
	projSettings, perr := project.Load(root)
	a.projectSettings = projSettings
	if perr != nil {
		if a.status != "" {
			a.status += " · " + cmdErrorText(perr)
		} else {
			a.status = cmdErrorText(perr)
		}
	}
	return a
}

// Init starts the program. No initial commands are needed — except the
// activity-indicator tick chain when an mg-jdi run is already active at
// startup (startSpinnerIfRunning), so a TUI opened mid-run animates its
// badge without waiting for a keypress or refresh.
func (a *App) Init() tea.Cmd {
	return a.startSpinnerIfRunning()
}

// editorDoneMsg reports the outcome of the "e" edit-shortcut's tea.ExecProcess
// once the suspended editor process returns.
type editorDoneMsg struct {
	path string
	err  error
}

// doneMsg reports the outcome of the "D" mark-done shortcut's background
// job.FinishJob run once it returns.
type doneMsg struct {
	err error
}

// deleteMsg reports the outcome of the "delete" shortcut's background
// job.DeleteJob run once it returns.
type deleteMsg struct {
	err error
}

// commitMsg reports the outcome of the auto-commit that follows a successful
// brief.md edit (see editorDoneMsg and commitBriefCmd).
type commitMsg struct {
	err error
}

// pushMsg reports the outcome of the detail view's "P" push-to-origin
// action (a `git push -u origin <branch>`, run off the UI thread via
// pushCmd so a slow/blocked network call doesn't block rendering).
type pushMsg struct {
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
		if a.agentsPicker != nil {
			a.agentsPicker.resize(a.width, a.height)
		}
		if a.confirm != nil {
			a.confirm.resize(a.width, a.height)
		}
		return a, nil
	case editorDoneMsg:
		if a.detail != nil {
			if msg.err != nil {
				a.detail.setStatus(cmdErrorText(msg.err))
			} else {
				a.detail.reloadCurrent()
				a.detail.setStatus("edited " + filepath.Base(msg.path))
				// brief.md is the one editable tab (jobFiles) and the one
				// file the job workflow expects to always be committed —
				// auto-commit it so an edit never lingers as an uncommitted
				// change the way finish-job.sh's clean-tree check would
				// otherwise reject. Any other future editable tab is left
				// alone, same as before this behavior existed.
				if filepath.Base(msg.path) == "brief.md" {
					return a, a.commitBriefCmd()
				}
			}
		}
		return a, nil
	case commitMsg:
		if a.detail != nil {
			if msg.err != nil {
				a.detail.setStatus(cmdErrorText(msg.err))
			} else {
				a.detail.setStatus("edited and committed brief.md")
			}
		}
		return a, nil
	case doneMsg:
		// The lifecycle outcome is not a reliable done signal: a declined
		// confirmation (ErrCancelled) and a real failure both carry an error,
		// while a successful archive carries none — and a declined-but-kept
		// job looks identical to an archived one from the error alone. So
		// regardless of msg.err, always fall back to refreshing the job list
		// from disk and returning to it — a job that got archived is simply
		// gone from the re-read list, one that was declined or failed is
		// still there. An error still surfaces through cmdErrorText first,
		// same as any other host-command failure.
		spinnerCmd := a.refreshJobs()
		a.detail = nil
		a.state = stateList
		if msg.err != nil {
			a.status = cmdErrorText(msg.err)
		} else {
			a.status = "refreshed"
		}
		return a, spinnerCmd
	case deleteMsg:
		// Same reasoning as doneMsg: a declined delete (ErrCancelled) leaves
		// the job present in the re-read list — refreshJobs and returning to
		// the list handles both outcomes uniformly.
		spinnerCmd := a.refreshJobs()
		a.detail = nil
		a.state = stateList
		if msg.err != nil {
			a.status = cmdErrorText(msg.err)
		} else {
			a.status = "refreshed"
		}
		return a, spinnerCmd
	case pushMsg:
		// A push never changes what the working tree or job list looks
		// like — only origin's state — so there is nothing to refresh or
		// rebuild here, just a status line.
		if a.detail != nil {
			if msg.err != nil {
				a.detail.setStatus(cmdErrorText(msg.err))
			} else {
				a.detail.setStatus("→ pushed " + msg.branch + " to origin")
			}
		}
		return a, nil
	case spinnerTickMsg:
		// Advance the activity indicator. The step always moves (even on the
		// tick that observes the run has ended — the check below happens
		// after), but the chain only continues while a job is actually
		// running: the moment every sidecar has flipped to a stopped state
		// (or the in-session jdiSeen fallback expired), return nil and clear
		// the guard so no further timer-driven redraws happen. The guard is
		// (re)asserted on the continuing path too, not just by
		// startSpinnerIfRunning, so "guard set" stays equivalent to "chain
		// alive" no matter how a tick reached the handler.
		a.spinnerStep++
		if a.detail != nil {
			a.detail.spinnerStep = a.spinnerStep
		}
		if !a.anyJDIRunning() {
			a.spinnerTicking = false
			return a, nil
		}
		a.spinnerTicking = true
		return a, a.spinnerTickCmd()
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
		case stateAgents:
			return a.updateAgentsPicker(msg)
		case stateConfirm:
			return a.updateConfirm(msg)
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
	case stateAgents:
		content = a.agentsPicker.render()
	case stateConfirm:
		content = a.confirm.render()
	default:
		content = a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
	}
	return uiPaddingStyle.Render(content)
}

// refreshRecentCommits re-reads the recent-activity strip from git, always
// fetching up to the configured maximum (Settings.RecentActivityCountValue).
// How many of those cached commits actually get rendered is decided later, at
// render time, by listView.recentActivityShown — not here. This split matters
// because refreshRecentCommits runs before the first tea.WindowSizeMsg ever
// arrives (see NewApp), when the viewport size is still zero; sizing the
// *fetch* against that would size against a stale terminal height on first
// render. Fetching the configured maximum and deciding the display count at
// render time (once layout is known) avoids that hazard.
//
// Like currentBranch, an error (e.g. a non-repo project) degrades to an empty
// strip rather than surfacing in the status line — this is decorative,
// optional header content, not an action the user asked for.
func (a *App) refreshRecentCommits() {
	commits, err := git.RecentCommits(a.root, a.settings.RecentActivityCountValue())
	if err != nil {
		a.list.recentCommits = nil
		return
	}
	a.list.recentCommits = commits
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

// refreshJobs re-reads the job list from disk (job.Discover: one os.ReadDir
// plus one small brief.md read per job) and clamps the cursor so a job that
// was archived mid-session doesn't leave it out of range. It does not touch
// an open detail view — see refresh for that, and updateDetail's
// "esc"/"backspace" case, which uses refreshJobs alone because the detail
// view it would otherwise reload is about to be discarded anyway.
//
// It also returns the activity-indicator tick cmd when a refresh just
// (re)discovered an mg-jdi run while the tick chain wasn't already going
// (startSpinnerIfRunning) — the "run started somewhere other than this
// session's 'j' key" path, e.g. ctrl+r picking up a run another TUI or a
// direct mg-jdi launch started. Callers propagate the cmd; the spinnerTicking
// guard makes it a no-op (nil) when the chain is already live.
func (a *App) refreshJobs() tea.Cmd {
	if jobs, err := job.Discover(a.root); err == nil {
		a.jobs = jobs
	}
	a.list.currentBranch, _ = git.CurrentBranch(a.root) // "" on detached HEAD / non-repo
	a.refreshRecentCommits()
	if a.list.cursor > 0 && a.list.cursor >= len(a.jobs) {
		a.list.cursor = len(a.jobs) - 1
	}
	if a.list.cursor < 0 {
		a.list.cursor = 0
	}
	a.pollJDIBell()
	return a.startSpinnerIfRunning()
}

// pollJDIBell is the TUI-side half of the stop notification: a TUI-launched
// mg-jdi run has no terminal of its own to
// ring `\a` into (see cmd/mg/jdi.go's own CLI-path bell), so instead the TUI
// rings it on its own next poll — this function, called from refreshJobs,
// which is every "poll tick" this app has (ctrl+r, returning to list, a
// checkout, etc.) — the first time it observes a job's status transition
// into a stopped:* state it hadn't already notified for. The one exception
// to "refresh-triggered polling only" is the activity indicator's
// spinnerTickMsg: a narrow timer-driven redraw that runs only while an
// mg-jdi run is active (see spinnerTickMsg's doc), deliberately kept out of
// the notification path — a run stopping is exactly when that timer ends.
//
// Dedup is in-memory (a.jdiSeen) and keyed by job Name: the first
// observation of any given job only seeds the map, never rings, even if
// that first-seen status is already a stopped:* one — a freshly (re)started
// TUI re-observing a job that was already stopped before it started
// watching isn't a new event. A later transition away from stopped back to
// running (a fresh mg-jdi run against the same job) and then stopped again
// rings a second time, since the intervening JDIRunning observation clears
// the "already notified for this stop" condition.
func (a *App) pollJDIBell() {
	for _, j := range a.jobs {
		st, ok := job.ReadJDIStatus(a.root, j.Name)
		if !ok {
			continue
		}
		prev, seen := a.jdiSeen[j.Name]
		a.jdiSeen[j.Name] = st.State
		a.jdiSeenAt[j.Name] = jdiNow()
		if !seen {
			continue
		}
		if isJDIStopped(st.State) && prev != st.State {
			ringBell()
		}
	}
}

// isJDIStopped reports whether s is one of the two terminal mg-jdi states.
func isJDIStopped(s job.JDIState) bool {
	return s == job.JDIStoppedFinished || s == job.JDIStoppedNeedsHuman
}

// jdiAlreadyRunning reports whether j already has an mg-jdi run in progress,
// for the "j" key's launch-block guard. The on-disk sidecar (job.ReadJDIStatus) is the source
// of truth once mg-jdi has written it — a JDIStoppedFinished/
// JDIStoppedNeedsHuman status there always wins, even if a.jdiSeen still
// remembers this same session having launched it earlier, so a job is never
// permanently blocked after mg-jdi actually stops.
//
// The gap the sidecar alone can't cover is the brief moment between
// launch.Jdi's Start() returning and mg-jdi's own first WriteJDIStatus call
// (see job/jdistatus.go): nothing has been written yet, so
// job.ReadJDIStatus reports ok=false. Only in that no-sidecar-yet case does
// this fall back to a.jdiSeen, which updateDetail's "j" handler itself seeds
// to job.JDIRunning the instant launch.Jdi succeeds — catching a second
// press that lands in that same narrow window. That fallback entry is only
// trusted for jdiSeenFallbackTTL (see its own doc): once it's older than
// that, mg-jdi almost certainly crashed or was killed before ever writing a
// sidecar, and this reports "not running" rather than blocking forever.
func (a *App) jdiAlreadyRunning(j job.Job) (job.JDIStatus, bool) {
	if st, ok := job.ReadJDIStatus(a.root, j.Name); ok {
		return st, st.State == job.JDIRunning
	}
	if a.jdiSeen[j.Name] == job.JDIRunning && jdiNow().Sub(a.jdiSeenAt[j.Name]) <= jdiSeenFallbackTTL {
		return job.JDIStatus{State: job.JDIRunning}, true
	}
	return job.JDIStatus{}, false
}

// anyJDIRunning reports whether any job in the current list has an mg-jdi
// run in progress, using the same combined source of truth jdiAlreadyRunning
// uses per job: the on-disk sidecar first (job.ReadJDIStatus — which also
// degrades a stale "running" away), falling back to the in-session
// jdiSeen/jdiSeenAt dedup map only while no sidecar exists yet, so a
// just-launched run whose mg-jdi process hasn't written its first status file
// animates immediately (see jdiAlreadyRunning's doc for the fallback's TTL).
// This is the spinner tick chain's liveness check: the chain keeps going
// while it returns true and self-terminates the moment it returns false.
func (a *App) anyJDIRunning() bool {
	for _, j := range a.jobs {
		if _, running := a.jdiAlreadyRunning(j); running {
			return true
		}
	}
	return false
}

// spinnerTickCmd returns the tea.Cmd that delivers the next spinnerTickMsg
// after activityInterval — one frame of the activity indicator.
func (a *App) spinnerTickCmd() tea.Cmd {
	return tea.Tick(activityInterval, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// startSpinnerIfRunning starts the activity-indicator tick chain when (a)
// it isn't already going (the spinnerTicking guard — no duplicate concurrent
// chains) and (b) some job is currently running. Returns nil otherwise, so
// callers can `return a, a.startSpinnerIfRunning()` and keep their previous
// nil-cmd behaviour unchanged when there is nothing to animate. The chain is
// stopped by the tick handler itself once anyJDIRunning turns false, so a
// run ending (sidecar flipping to a stopped state) is what ends the timer —
// no separate stop call needed.
func (a *App) startSpinnerIfRunning() tea.Cmd {
	if a.spinnerTicking || !a.anyJDIRunning() {
		return nil
	}
	a.spinnerTicking = true
	return a.spinnerTickCmd()
}

// ringBell writes a bare terminal bell character. A BEL byte reaches the
// terminal (and triggers whatever bell/notification behavior it's
// configured for) regardless of Bubble Tea's alt-screen rendering state, so
// this is safe to call as a direct side effect from within Update rather
// than needing its own tea.Cmd. A package-level var (not a plain func) so
// tests can swap it out instead of actually beeping the test runner.
var ringBell = func() {
	fmt.Print("\a")
}

// refresh does refreshJobs, plus reloads the open detail view's files (if
// any) so changes an agent made outside the TUI show up — used by "ctrl+r".
// It also re-reads the project settings, so a .manigot/manigot.json edited
// outside the TUI (e.g. base branch changed by hand or pulled from origin)
// is picked up without an app restart. The settings form's own submit path
// is the only writer to a.projectSettings and it never runs concurrently
// with refresh() — ctrl+r isn't routed here while the form is open.
//
// Returns the activity-indicator tick cmd refreshJobs produced (a ctrl+r is
// a discovery point for runs started outside this session's "j" key), so the
// callers can propagate it.
func (a *App) refresh() tea.Cmd {
	spinnerCmd := a.refreshJobs()
	if ps, err := project.Load(a.root); err == nil {
		a.projectSettings = ps
	}
	if a.detail != nil {
		a.detail.reload()
		a.detail.refreshCommits(a.settings.RecentActivityCountValue())
	}
	return spinnerCmd
}

// updateList handles keys while the job list is showing. The cursor-movement
// keys delegate to the list view (a.list.update); everything else is App
// routing — view switches, refresh, launches.
func (a *App) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a.status = "" // status is transient — cleared on every key unless a case sets it
	switch msg.String() {
	case "q", "esc":
		return a, tea.Quit
	case "ctrl+r":
		// Refresh first, then report the *post*-refresh job count — the
		// original ordering (see main), preserved across the
		// refresh-return-value refactor. The refresh is the discovery point
		// for runs started
		// outside this session (its returned tick cmd is propagated below),
		// and len(a.jobs) must reflect the re-read list, not the stale one.
		spinnerCmd := a.refresh()
		a.status = fmt.Sprintf("refreshed · %d job(s)", len(a.jobs))
		return a, spinnerCmd
	case "enter", "l", "right":
		if j, ok := a.list.selectedJob(a.jobs); ok {
			a.detail = newDetailView(j, a.width, a.height)
			a.detail.refreshCommits(a.settings.RecentActivityCountValue())
			a.state = stateDetail
		}
	case "j":
		// Run mg-jdi against the job under the cursor — the list-view
		// counterpart of updateDetail's "j" case, sharing its already-running
		// guard (on-disk sidecar + in-session jdiSeen fallback) and its
		// detached-background launch (launch.Jdi opens no terminal at all).
		// Status lands in the list footer, and the wording points at the list
		// badge — the list has no log tab open to point at. No-op when the
		// list is empty (nothing under the cursor).
		j, ok := a.list.selectedJob(a.jobs)
		if !ok {
			return a, nil
		}
		if st, running := a.jdiAlreadyRunning(j); running {
			label := "mg jdi is already running for this job"
			if st.Agent != "" {
				label += " @" + st.Agent
			}
			a.status = label
			return a, nil
		}
		if err := launch.Jdi(j.ID, a.root, a.settings.ProfileValue()); err != nil {
			a.status = cmdErrorText(err)
			return a, nil
		}
		// Seed the stop-notification dedup as "running" right away (see
		// updateDetail's "j" case for why), then start the activity-indicator
		// tick chain if it isn't already going.
		a.jdiSeen[j.Name] = job.JDIRunning
		a.jdiSeenAt[j.Name] = jdiNow()
		a.status = "→ mg jdi started in the background — see the list badge"
		return a, a.startSpinnerIfRunning()
	case "n":
		// Create a new job via the host mg-job command.
		a.newJob = newNewJobView(a.width, a.height)
		a.state = stateNewJob
	case "s":
		// Edit the persisted TUI settings: the global personal prefs
		// (editor, recent activity count, subscription profile) and the
		// project base branch. Both are seeded from their on-disk files
		// (see NewApp).
		a.settingsView = newSettingsView(a.settings, a.projectSettings, a.width, a.height)
		a.state = stateSettings
	case "o":
		// Launch a bare manigot session (no agent, no job) in a detached
		// new terminal — a quick ad-hoc change that doesn't belong to any
		// specific job's workflow. Mirrors updateDetail's agent-launch
		// reporting so the two launch paths feel consistent in the footer.
		desc, err := launch.Quick(a.root, a.settings.ProfileValue(), a.settings.Terminal)
		if err != nil {
			a.status = cmdErrorText(err)
		} else {
			a.status = "→ quick session in " + desc
		}
	case "a":
		// Pick any agent and launch it as a jobless quick session — the
		// native-TUI counterpart to `mg agents` (scripts/agents.sh),
		// mirroring "o" (bare quick session) but for a specific agent.
		// A discovery failure (e.g. no resolvable manigot checkout) or an
		// empty agent list degrades to a status line
		// instead of opening the picker, the same "never crash on a
		// host-command error" convention every other action in this file
		// follows (cmdErrorText).
		agents, err := agentlist.Discover(a.root)
		if err != nil {
			a.status = cmdErrorText(err)
		} else if len(agents) == 0 {
			a.status = "no agents found"
		} else {
			a.agentsPicker = newAgentsPickerView(agents, a.width, a.height)
			a.state = stateAgents
		}
	default:
		// Cursor movement — the list view's own key handling. "g"/"G" land
		// here too, below the routing keys (none of which collide).
		a.list.update(msg, len(a.jobs))
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
		// Validate the recent-activity count field first (blank = default;
		// anything unparseable or outside 1–100 keeps the form open with a
		// status message and persists nothing). Persist both files — the
		// global personal prefs (config.Save) and the project base branch
		// (project.Save). Surface whichever fails first in the form's status,
		// leaving the form open so the user can retry without retyping; only
		// update the in-memory copies and return to the list once both
		// succeed.
		if _, err := a.settingsView.recentActivityCount(); err != nil {
			a.settingsView.status = cmdErrorText(err)
			return a, nil
		}
		s := a.settingsView.settingsValue()
		ps := a.settingsView.projectValue()
		if err := config.Save(s); err != nil {
			a.settingsView.status = cmdErrorText(err)
			return a, nil
		}
		if err := project.Save(a.root, ps); err != nil {
			a.settingsView.status = cmdErrorText(err)
			return a, nil
		}
		a.settings = s
		a.projectSettings = ps
		// The recent-activity strip's maximum may have just changed — refetch
		// so the list reflects the new count immediately on return.
		a.refreshRecentCommits()
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
		// In-process job creation (the port of hostcmd.NewJob): the script's
		// full stdout summary is discarded — the TUI shows its own status
		// line, matching how the old call ignored the captured output.
		if _, err := job.CreateJob(a.root, title, job.CreateOptions{Type: typ}, io.Discard); err != nil {
			a.newJob.status = cmdErrorText(err)
			return a, nil
		}
		// Refresh the list so the new job appears, then return to it.
		if jobs, derr := job.Discover(a.root); derr == nil {
			a.jobs = jobs
		}
		a.list.currentBranch, _ = git.CurrentBranch(a.root) // "" on detached HEAD / non-repo
		a.refreshRecentCommits()
		a.list.cursor = 0 // newest first after Discover's date-desc sort
		a.status = "created \"" + title + "\" (" + typ + ")"
		a.newJob = nil
		a.state = stateList
	}
	return a, nil
}

// updateAgentsPicker handles keys in the "Launch an agent" picker opened by
// "a" from the list view.
func (a *App) updateAgentsPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.agentsPicker.update(msg) {
	case apCancel:
		a.agentsPicker = nil
		a.state = stateList
	case apSubmit:
		agent, ok := a.agentsPicker.selected()
		a.agentsPicker = nil
		a.state = stateList
		if !ok {
			return a, nil
		}
		// Mirrors updateDetail's agent-launch status reporting ("→ <agent>
		// in <desc>") so both launch paths feel consistent in the footer.
		desc, err := launch.AgentQuick(agent.Name, a.root, a.settings.ProfileValue(), a.settings.Terminal)
		if err != nil {
			a.status = cmdErrorText(err)
		} else {
			a.status = "→ " + agent.Name + " in " + desc
		}
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
		spinnerCmd := a.refreshJobs()
		a.detail = nil
		a.state = stateList
		a.status = "refreshed"
		return a, spinnerCmd
	case "q":
		return a, tea.Quit
	case "ctrl+r":
		spinnerCmd := a.refresh()
		a.detail.setStatus("refreshed")
		return a, spinnerCmd
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
		// falling into the agentForKey dispatch below. Shows the TUI-side
		// confirmation (replacing finish-job.sh's subprocess prompts); on
		// confirmation the lifecycle runs in-process.
		a.openConfirm(confirmDone)
		return a, nil
	case "j":
		// Run mg-jdi: a bigger, composite action like "D", not a
		// single-agent launch, so it's handled here rather than through
		// agentForKey. Unlike launch.Agent, this starts detached in the
		// background with no spawned terminal window at all — see launch.Jdi's
		// own doc for why. Every job now has its own worktree, so there is
		// no "wrong branch checked out" state to guard against
		// here: mg-jdi resolves its own correct worktree per invocation.
		// Block a second concurrent launch against the same job — the brief
		// this job exists for ("press j ... multiple times" spawns several
		// processes with no indication). jdiAlreadyRunning combines the
		// on-disk sidecar with the in-session dedup map so a press landing
		// before mg-jdi has written its very first status file is caught
		// too.
		if st, running := a.jdiAlreadyRunning(a.detail.job); running {
			label := "mg jdi is already running for this job"
			if st.Agent != "" {
				label += " @" + st.Agent
			}
			a.detail.setStatus(label)
			return a, nil
		}
		if err := launch.Jdi(a.detail.job.ID, a.root, a.settings.ProfileValue()); err != nil {
			a.detail.setStatus(cmdErrorText(err))
			return a, nil
		}
		// Seed the stop-notification dedup as "running" right
		// away rather than waiting for the first poll to discover it —
		// otherwise a run that finishes before the next refresh would
		// look like a first-ever observation of an already-stopped job
		// and never ring.
		a.jdiSeen[a.detail.job.Name] = job.JDIRunning
		a.jdiSeenAt[a.detail.job.Name] = jdiNow()
		a.detail.setStatus("→ mg jdi started in the background — see the log tab or list badge")
		// Start the activity-indicator tick chain so the just-launched run
		// animates immediately — the sidecar doesn't exist yet, but the
		// jdiSeen entry seeded above is enough for anyJDIRunning (via
		// jdiAlreadyRunning's fallback). startSpinnerIfRunning is a no-op
		// (nil cmd) when a chain is already going, keeping this a plain
		// `return a, nil` in that case.
		return a, a.startSpinnerIfRunning()
	case "delete", "x":
		// Permanently delete the job. Bound to both the physical Delete/Entf
		// key and "x": the forward-delete key's escape sequence (CSI 3~) is
		// not decoded consistently across every terminal emulator/keyboard
		// layout, so "x" (same mnemonic tmux/ranger use for a destructive
		// remove) is a reliable fallback rather than a hidden alias — it's
		// named in the footer hint too (see renderFooter). The destructive
		// confirmation itself is the TUI-side confirm view (replacing
		// delete-job.sh's subprocess prompt).
		a.openConfirm(confirmDelete)
		return a, nil
	case "P":
		// Push this job's branch to origin — the "quick way to push feature
		// branches". git.Push
		// pushes the named branch ref directly (git push origin <branch>),
		// which does not require that branch to be checked out in the
		// working tree.
		if a.detail.job.Branch == "" {
			a.detail.setStatus("no branch known for this job")
			return a, nil
		}
		return a, a.pushCmd(a.detail.job.Branch)
	case "t":
		// Open the job's branch diff in tig — a host-side TUI launched just
		// like an agent session (tmux split pane / new terminal, see
		// launch.Tig), not a background process like "j". The availability
		// gate is cached on the detail view at open time (tigAvailable); the
		// launch path re-checks it itself as an authoritative backstop, so a
		// stale cached gate surfaces a synchronous error here rather than a
		// doomed pane.
		if !a.detail.tigAvailable {
			a.detail.setStatus("tig is not installed on the host — install it, or use the diff tab")
			return a, nil
		}
		if a.detail.job.Branch == "" {
			a.detail.setStatus("no branch known for this job")
			return a, nil
		}
		// The job is passed by Name (id_slug), not ID: `mg diff` resolves the
		// job's branch via an exact match on that tail segment (see launch.
		// Tig's own doc), and there is no profile flag — mg diff is a host
		// git command, not a session launch.
		desc, err := launch.Tig(a.detail.job.Name, a.root, a.settings.Terminal)
		if err != nil {
			a.detail.setStatus(cmdErrorText(err))
		} else {
			a.detail.setStatus("→ tig in " + desc)
		}
		return a, nil
	}
	// Action bar: fire the agent whose key matches, if it is valid for the
	// current job's stage.
	if agent := a.agentForKey(msg.String()); agent != "" {
		desc, err := launch.Agent(agent, a.detail.job.ID, a.root, a.settings.ProfileValue(), a.settings.Terminal)
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

// openConfirm switches to the confirmation view for the given action on the
// open job (called by the "D" / "delete" / "x" keys in the detail view).
func (a *App) openConfirm(action confirmAction) {
	a.confirm = newConfirmView(action, a.root, a.detail.job)
	a.confirm.resize(a.width, a.height)
	a.state = stateConfirm
}

// updateConfirm handles keys in the confirmation view: y/enter runs the
// confirmed lifecycle action in the background (a tea.Cmd goroutine), n/esc/q
// cancels back to the detail view.
func (a *App) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		action := a.confirm.action
		a.confirm = nil
		a.state = stateDetail
		switch action {
		case confirmDone:
			return a, a.runDoneCmd()
		case confirmDelete:
			return a, a.runDeleteCmd()
		}
	case "n", "N", "esc", "q":
		a.confirm = nil
		a.state = stateDetail
	}
	return a, nil
}

// runDoneCmd runs job.FinishJob in the background and reports the outcome as
// a doneMsg. The user has already confirmed in the confirm view, so every
// internal prompt is pre-approved; informational output is discarded — the
// TUI renders its own status lines.
func (a *App) runDoneCmd() tea.Cmd {
	root := a.root
	jobName := a.detail.job.Name
	return func() tea.Msg {
		_, err := job.FinishJob(root, jobName, yesConfirm, io.Discard)
		return doneMsg{err: err}
	}
}

// runDeleteCmd runs job.DeleteJob in the background and reports the outcome
// as a deleteMsg — the same pre-approved shape as runDoneCmd.
func (a *App) runDeleteCmd() tea.Cmd {
	root := a.root
	jobName := a.detail.job.Name
	return func() tea.Msg {
		_, err := job.DeleteJob(root, jobName, yesConfirm, io.Discard)
		return deleteMsg{err: err}
	}
}

// yesConfirm pre-approves every lifecycle confirmation prompt — used once the
// user has already confirmed the action in the TUI's confirm view.
func yesConfirm(prompt string) (bool, error) { return true, nil }

// pushCmd returns the tea.Cmd behind the "P" push-to-origin action: a plain
// git call off the UI goroutine — no interactive process, just a git push —
// and reports the outcome as a pushMsg once it returns. The call is bounded
// by a timeout (see hostGitTimeout) so a stalled network can't hang the TUI's
// command channel forever; the timeout surfaces as an ordinary pushMsg error.
func (a *App) pushCmd(branch string) tea.Cmd {
	root := a.root
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), hostGitTimeout)
		defer cancel()
		err := git.PushWithContext(ctx, root, branch)
		return pushMsg{branch: branch, err: err}
	}
}

// commitBriefCmd returns the tea.Cmd that auto-commits brief.md right after a
// successful "e" edit (see the editorDoneMsg case above), following the same
// "[ID] <type>: <summary>" subject convention every agent's own commits use
// (agents/developer.md, agents/reviewer.md, agents/quality.md). A plain git
// call off the UI goroutine — no interactive terminal is needed. The path is
// rebuilt from a.detail.job rather than threaded through editorDoneMsg since
// only brief.md ever reaches here.
//
// The commit runs inside the job's own worktree (git -C <job-worktree>), not
// a.root: since the worktree model a job's files live in its own worktree,
// a sibling of the project root, so a pathspec relative to a.root would
// escape the main worktree ("outside repository") — and even if it didn't, it
// would stage/commit against the main worktree's branch and index instead of
// the job's branch. The worktree root is derived from the job dir: a job
// lives at <worktree>/docs/jobs/<id>_<slug>/, so two Dir() hops up lands on
// the worktree root. The non-repo working-tree fallback (no worktrees at
// all) degrades to the same derivation with job dirs under a.root itself.
func (a *App) commitBriefCmd() tea.Cmd {
	worktreeRoot := filepath.Dir(filepath.Dir(a.detail.job.Dir))
	id := a.detail.job.ID
	path := filepath.Join(a.detail.job.Dir, "brief.md")
	return func() tea.Msg {
		rel, err := filepath.Rel(worktreeRoot, path)
		if err != nil {
			return commitMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), hostGitTimeout)
		defer cancel()
		err = git.CommitFileWithContext(ctx, worktreeRoot, rel, fmt.Sprintf("[%s] brief: edit via TUI", id))
		return commitMsg{err: err}
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

// jdiStatusBadge renders a short "[...]" tag for a job's list row
// when an autonomous run has something to report:
// running (naming the active agent), or one of the two stop reasons.
// Renders "" when there's nothing to show — no sidecar status file for this
// job, or one job.ReadJDIStatus itself degrades away (a missing/unparseable
// file, or a "running" status stale enough to mean mg-jdi was killed
// mid-run — see job.ReadJDIStatus's own doc, which is the sole gate here;
// this function only formats what it returns, on the same
// derived-from-polling-job-files basis every other list-row element uses).
//
// spinnerStep drives the animated activity-indicator frame prefixed to the
// running badge (`⠋ [running @<agent>]`) — see activity.go. It is threaded
// from the App (list row) or the open detailView (action bar) so both badge
// call sites animate off the same counter and can't drift apart. The
// [finished] / [needs human] variants render no frame: nothing is happening.
func jdiStatusBadge(root string, j job.Job, spinnerStep int) string {
	st, ok := job.ReadJDIStatus(root, j.Name)
	if !ok {
		return ""
	}
	switch st.State {
	case job.JDIRunning:
		label := "running"
		if st.Agent != "" {
			label += " @" + st.Agent
		}
		// The frame goes *around* the label, not inside it: tests assert on
		// the "running @developer" substring, so the glyph must not replace
		// the label's own text. Styled with the badge's own accentStyle so
		// the whole tag reads as one unit.
		return accentStyle.Render(activityFrame(spinnerStep) + " [" + label + "]")
	case job.JDIStoppedFinished:
		return statusDoneStyle.Render("[finished]")
	case job.JDIStoppedNeedsHuman:
		return warnStyle.Render("[needs human]")
	default:
		return ""
	}
}

// footer is the App's bottom help/status line — the list view's footer,
// rendering the dim key hint and, when a.status is set (e.g. right after
// "ctrl+r"), the status alongside it rather than replacing it. Kept on App
// because status is cross-view state; the list view renders the same footer
// via listFooter.
func (a *App) footer() string {
	return listFooter(a.status)
}

// --- helpers ----------------------------------------------------------------

// cmdErrorText formats an error from a host command for a status line.
//
// Everything here now comes from the in-process Go lifecycle/session (the
// scripts and their resolution machinery are gone), so every error is a
// one-liner.
func cmdErrorText(err error) string {
	if err == nil {
		return ""
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

// AgentDescriptionWidth is the shared cap for rendering an agent's
// description in every surface that lists agents — the `mg agents` plain
// listing, the `mg agents` interactive picker, and the TUI's "a" agents
// picker — so all three truncate identically (via Truncate's ellipsis)
// instead of running the full 100–200 char description into a wall of text.
// The full description is deliberately kept out of the truncation: the
// pickers' SearchKey still carries it, so type-to-filter keeps matching on
// the whole description.
const AgentDescriptionWidth = 60

// Truncate shortens s to at most n characters when it is longer: n-1
// characters plus an ellipsis ("…"). Strings at or under n characters are
// returned unchanged, and n <= 0 leaves s untouched. This is the shared
// truncation used by every agents-list surface (see AgentDescriptionWidth)
// and the internal building block behind truncate.
func Truncate(s string, n int) string {
	if n > 0 && len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func truncate(s string, n int) string {
	return Truncate(s, n)
}
