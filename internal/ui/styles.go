package ui

import "github.com/charmbracelet/lipgloss"

// Brand colour, matching the accent used in the scaffold.
var accent = lipgloss.Color("#7D56F4")

// uiPaddingX / uiPaddingY are the outer margin the whole TUI keeps between
// its content and the terminal edge. Asymmetric because terminal cells are
// roughly twice as tall as they are wide, so 1 row / 2 columns reads as an
// even margin rather than a wider gap on the sides. Every view sizes its
// layout against a.width/a.height, which the WindowSizeMsg handler already
// shrinks by this amount (see app.go's Update) — uiPaddingStyle then adds
// it back as literal padding around the rendered frame, so the padded
// output still exactly fills the alt-screen terminal instead of overflowing it.
const (
	uiPaddingX = 2
	uiPaddingY = 1
)

// uiPaddingStyle wraps the top-level rendered frame in View().
var uiPaddingStyle = lipgloss.NewStyle().Padding(uiPaddingY, uiPaddingX)

var (
	// titleStyle is for the app title in the header.
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// dimStyle is for secondary text.
	dimStyle = lipgloss.NewStyle().Faint(true)

	// activityStyle is for the recent-activity (git log) strip — a notch
	// lighter than dimStyle so it reads as background information, less
	// prominent than the job list and footer above it. The gray was
	// lightened from #585858 per the "make git log a bit brighter" brief:
	// the old value faded to near-invisible on low-contrast screens.
	activityStyle = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#808080"))

	// headerStyle is for column headers.
	headerStyle = lipgloss.NewStyle().Bold(true)

	// selectedStyle highlights the current list row.
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// accentStyle emphasises short tokens like action-bar keys.
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// logoStyle is for the ASCII logo above the list title — the same accent
	// as the title itself, so the mark and the wordmark read as one brand
	// block (deliberately not bold: the logo is a graphic, weight would just
	// thicken every glyph).
	logoStyle = lipgloss.NewStyle().Foreground(accent)

	// statusOpen / statusDone colour the status column.
	statusOpenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D7A000"))
	statusDoneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))

	// statusStyle is for the detail view's status line (launch confirmations).
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))

	// warnStyle is for the detail view's "[needs human]" mg-jdi badge and any
	// other warning-level tag — plain red text, no background fill, per
	// explicit feedback that a background banner was the wrong call. ANSI
	// palette index 9 (bright red), not a custom hex, so it renders from the
	// terminal's own configured red instead of depending on truecolor
	// support.
	warnStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))

	// gitPanelModalStyle is the popup box that floats the git panel over the
	// detail view: a rounded, accent-bordered box with a solid background
	// fill, so the cells it covers are fully replaced rather than letting the
	// detail content show through (see App.gitPanelOverlay's placeOverlay).
	gitPanelModalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accent).
				Background(lipgloss.Color("#2D2A3E")).
				Padding(1, 2)
)
