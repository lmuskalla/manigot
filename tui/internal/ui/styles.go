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
	// prominent than the job list and footer above it.
	activityStyle = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#585858"))

	// headerStyle is for column headers.
	headerStyle = lipgloss.NewStyle().Bold(true)

	// selectedStyle highlights the current list row.
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// accentStyle emphasises short tokens like action-bar keys.
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// statusOpen / statusDone colour the status column.
	statusOpenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D7A000"))
	statusDoneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))

	// statusStyle is for the detail view's status line (launch confirmations).
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))
)
