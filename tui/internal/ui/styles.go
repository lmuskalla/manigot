package ui

import "github.com/charmbracelet/lipgloss"

// Brand colour, matching the accent used in the scaffold.
var accent = lipgloss.Color("#7D56F4")

var (
	// titleStyle is for the app title in the header.
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// dimStyle is for secondary text.
	dimStyle = lipgloss.NewStyle().Faint(true)

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
