package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
)

// listView is the job list view — the TUI's default state, split out of the
// old App god-file. It owns the cursor, the
// recent-activity strip data, and the list rendering and cursor keys. App
// keeps the job list itself, routing, refresh, and cross-view state (status,
// spinner, jdi dedup), feeding listView what it needs to render.
type listView struct {
	// root is the project root — constant for the life of the app, so the
	// list holds it (for the title line and the jdi status badges) instead
	// of threading it through every render.
	root string

	// cursor is the selected job row.
	cursor int

	// recentCommits backs the list header's read-only "recent activity"
	// strip: the last few commits across all local branches (git.
	// RecentCommits), refreshed alongside currentBranch. nil when the repo
	// has no commits yet or RecentCommits errors (e.g. a non-repo project)
	// — renderRecentActivity degrades to rendering nothing in that case.
	recentCommits []git.Commit

	// currentBranch is the branch checked out in root right now (git.
	// CurrentBranch), refreshed alongside the job list so it never goes
	// stale relative to the title's "on <branch>" tag. Empty for a
	// detached HEAD or a non-repo project (job.Discover's working-tree-only
	// fallback) — both render nothing rather than an awkward empty label.
	currentBranch string
}

// newListView builds the list view for the project at root.
func newListView(root string) *listView {
	return &listView{root: root}
}

// update handles the list's own keys — cursor movement over jobCount jobs.
// Routing keys (enter, q, ctrl+r, j, n, s, o, a) and the transient status
// clearing belong to App's updateList; anything else is ignored.
func (v *listView) update(msg tea.KeyMsg, jobCount int) {
	switch msg.String() {
	case "up":
		if v.cursor > 0 {
			v.cursor--
		}
	case "down":
		if v.cursor < jobCount-1 {
			v.cursor++
		}
	case "home", "g":
		v.cursor = 0
	case "end", "G":
		if jobCount > 0 {
			v.cursor = jobCount - 1
		}
	}
}

// selectedJob returns the job under the cursor, or false if the list is empty.
func (v *listView) selectedJob(jobs []job.Job) (job.Job, bool) {
	if len(jobs) == 0 || v.cursor < 0 || v.cursor >= len(jobs) {
		return job.Job{}, false
	}
	return jobs[v.cursor], true
}

// recentActivityFloor bounds how few commits the bottom-of-screen "recent
// activity" strip can show: the strip's minimum footprint, so it never
// pushes job rows down further than the pre-existing header already did. The
// upper bound is configurable — Settings.RecentActivityCountValue
// (DefaultRecentActivityCount = 5) — see recentActivityShown for the part of
// this that scales with available room.
const recentActivityFloor = 1

// dashboardFixedChrome is the number of render rows that are always
// present outside the job rows, the job-summary line, and the recent-activity
// strip's own variable footprint: title line, blank spacer beneath it, "jobs"
// headline, divider, blank line before the footer, the footer itself, and the
// blank spacer before the recent-activity strip.
const dashboardFixedChrome = 7

// recentActivityShown returns how many of v.recentCommits should actually be
// rendered, given the terminal height and job count. It scales
// between recentActivityFloor and the configured maximum (maxRecent, i.e.
// Settings.RecentActivityCountValue) based on how much vertical room is
// spare, per the brief's resolution: a full list keeps the floor's 1-line
// footprint (never pushing job rows down further than the pre-existing header
// already did); a sparse list shows more, up to the configured maximum.
//
// The fixed chrome outside the job rows and the strip itself — title line,
// blank spacer beneath it, "jobs" headline, divider, blank line before the
// footer, footer, the blank spacer before the recent-activity strip — is 7
// rows, mirroring the same kind of budget detailView.bodyHeight documents for
// the detail view. spare is what's left of the terminal height once that
// chrome and every job row are accounted for.
//
// height == 0 (a view that has never received a tea.WindowSizeMsg, e.g.
// some existing tests) falls back to the floor, the same kind of guard
// render already applies to width == 0.
//
// Whichever path computes the count, the result is clamped to
// len(v.recentCommits): a repo with no commits yet (an unborn HEAD — exactly
// the state right after mg init on a brand-new project) degrades to 0, so
// renderRecentActivity renders no strip at all instead of slicing past an
// empty cache.
func (v *listView) recentActivityShown(jobCount, maxRecent, height int) int {
	var n int
	if height == 0 {
		n = recentActivityFloor
	} else {
		spare := height - dashboardFixedChrome - jobCount
		n = clamp(spare, recentActivityFloor, maxRecent)
	}
	if n > len(v.recentCommits) {
		// Fewer real commits than the computed count — render whatever's
		// available, same graceful-degrade rule renderRecentActivity already
		// applied before this task.
		n = len(v.recentCommits)
	}
	return n
}

// columnWidths returns the fixed column widths used by the list and detail
// headers so rows line up. Kept here so both views agree.
type columnWidths struct {
	id, status, stage, typ, date, title int
}

// listColumns returns the list view's fixed column widths. id is 13 rather
// than 8 so the longest word-based job id (12 chars, e.g. "unemployment")
// plus its display "#" prefix renders untruncated — pad() would cut a
// longer word at the column edge. stage is wider than typ because the
// longest stage name ("implement") is 9 chars.
func listColumns() columnWidths {
	return columnWidths{id: 13, status: 6, stage: 10, typ: 8, date: 12, title: 0}
}

// render draws the list view — title, job rows (or the empty-state invite),
// footer, and the recent-activity strip — given the data App owns: the job
// list, the footer status message, the settings' recent-activity maximum, the
// activity-indicator frame step for the running badges, and the viewport size
// (App keeps the terminal dimensions; the list view takes what it needs).
func (v *listView) render(jobs []job.Job, status string, maxRecent, spinnerStep, width, height int) string {
	w := width
	if w == 0 {
		w = 72
	}
	cols := listColumns()
	titleColsWidth := cols.id + cols.status + cols.stage + cols.typ + cols.date + 5*3 // 3 spaces between cols
	cols.title = w - titleColsWidth
	if cols.title < 16 {
		cols.title = 16
	}

	var b strings.Builder

	// Title line: "manigot - <project> - on <branch>".
	title := "manigot - " + shortRoot(v.root)
	if v.currentBranch != "" {
		title += " - on " + v.currentBranch
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
	if len(jobs) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  No jobs yet — press "))
		b.WriteString(accentStyle.Render("n"))
		b.WriteString(dimStyle.Render(" to create your first one."))
		b.WriteString("\n")
	} else {
		for i, j := range jobs {
			row := v.renderJobRow(j, cols, i == v.cursor, spinnerStep)
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	// Footer.
	b.WriteString("\n")
	b.WriteString(listFooter(status))

	// Recent-activity strip: kept below the footer, out of the way, since
	// it's read-only supplementary info rather than something the job list
	// depends on.
	b.WriteString("\n\n")
	// The activity strip's line count is recentActivityShown() — computed
	// from the actual spare room the terminal height leaves once the fixed
	// chrome and every job row are accounted for, so it can grow past its
	// one-line floor on a sparse list without ever making the total render
	// taller than the terminal.
	if activity := v.renderRecentActivity(w, v.recentActivityShown(len(jobs), maxRecent, height)); activity != "" {
		b.WriteString(activity)
	} else {
		b.WriteString("\n")
	}

	return b.String()
}

// renderRecentActivity renders the list header's read-only "recent activity"
// strip: up to n commits across all local branches, one compact dimmed line
// each, most-recent first (see git.RecentCommits for the dedup/attribution
// rules). Renders nothing — not even a heading — when v.recentCommits is
// empty (a fresh repo, or a refresh degrading a non-repo project's error
// away) or when n computes zero, matching the rest of the header's
// optional-content handling. This is fixed and non-interactive: no scrolling,
// no drill-down, per the brief's explicit rejection of a git log viewer.
//
// Deliberately no separate "recent activity" label line even when several
// commits are shown: recentActivityShown()'s line budget is exactly the
// commit count (see its doc comment), so a label would either overrun the
// budget (risking pushing job rows down, the one thing this strip must never
// do) or eat one commit slot to stay within it. The existing dim styling
// plus the strip's fixed position directly under the branch line is the
// grouping cue instead — spacing/color, not a border or a heading, per the
// brief's "no boxes-within-boxes, no decorative borders" rule.
func (v *listView) renderRecentActivity(w, n int) string {
	if n == 0 {
		return ""
	}
	return renderActivityLines(v.recentCommits[:n], w)
}

// renderActivityLines formats the per-commit lines of a git-log strip — one
// dim `shortHash  subject  relTime  branch` line per commit, most-recent
// first — and is the single shared formatter behind both the list view's
// recent-activity strip and the detail view's per-job strip, so the two
// views' visuals can never drift.
func renderActivityLines(commits []git.Commit, w int) string {
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
		b.WriteString(activityStyle.Render(strings.TrimRight(line, " ")))
		b.WriteString("\n")
	}
	return b.String()
}

// renderJobRow renders one job as a single (possibly highlighted) line.
func (v *listView) renderJobRow(j job.Job, cols columnWidths, selected bool, spinnerStep int) string {
	status := statusOpenStyle.Render(pad(j.Status, cols.status))
	if j.Status == "done" {
		status = statusDoneStyle.Render(pad(j.Status, cols.status))
	}
	cells := []string{
		pad("#"+j.ID, cols.id),
		status,
		pad(string(j.Stage()), cols.stage),
		pad(j.Type, cols.typ),
		pad(j.Date, cols.date),
		truncate(j.Title, cols.title),
	}
	line := strings.Join(cells, "  ")
	if badge := jdiStatusBadge(v.root, j, spinnerStep); badge != "" {
		line += "  " + badge
	}
	if selected {
		return selectedStyle.Render("▶ " + line)
	}
	return dimStyle.Render("  ") + line
}

// listFooter is the bottom help/status line of the list view: the dim key
// hint and, when status is set (e.g. right after "ctrl+r"), the status
// alongside it rather than replacing it — a status message must never leave
// the user not knowing what keys exist.
func listFooter(status string) string {
	hint := "↑/↓ navigate · enter view · j just do it · o quick · a agent · n new · s settings · ctrl+r refresh · q quit"
	if status != "" {
		return dimStyle.Render(hint) + "  " + statusStyle.Render(status)
	}
	return dimStyle.Render(hint)
}
