package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// jobTypes mirrors the --type values accepted by scripts/new-job.sh.
var jobTypes = []string{"feature", "fix", "chore"}

// njAction is what update returns for the App to act on.
type njAction int

const (
	njNone   njAction = iota
	njCancel          // esc — discard and return to the list
	njSubmit          // enter — create the job
)

// newJobView is the "New job" form: a title text input and a cycled type
// selector. It does not run the command itself — the App does that on submit so
// the view stays a pure input component.
type newJobView struct {
	title  textinput.Model
	typ    int // index into jobTypes
	focus  int // 0 = title, 1 = type
	width  int
	height int
	status string // validation/launch error message
}

func newNewJobView(width, height int) *newJobView {
	ti := textinput.New()
	ti.Placeholder = "add image gallery block"
	ti.Prompt = "" // we render our own "Title:" label
	ti.CharLimit = 120
	ti.Focus()
	ti.Width = 60
	v := &newJobView{title: ti, focus: 0, width: width, height: height}
	if width > 0 {
		v.title.Width = width - 12
		if v.title.Width < 20 {
			v.title.Width = 20
		}
	}
	return v
}

// resize updates the viewport (and the title field width) on terminal resize.
func (v *newJobView) resize(width, height int) {
	if width == v.width && height == v.height {
		return
	}
	v.width, v.height = width, height
	if width > 0 {
		w := width - 12
		if w < 20 {
			w = 20
		}
		v.title.Width = w
	}
}

// update processes a key for the form and reports the resulting action.
func (v *newJobView) update(msg tea.KeyMsg) njAction {
	switch msg.String() {
	case "esc":
		return njCancel
	case "tab":
		v.focus = 1 - v.focus
		if v.focus == 0 {
			v.title.Focus()
		} else {
			v.title.Blur()
		}
		return njNone
	case "enter":
		return njSubmit
	}

	if v.focus == 1 {
		// Type selector: cycle with ←/→.
		switch msg.String() {
		case "left":
			v.typ = (v.typ + len(jobTypes) - 1) % len(jobTypes)
		case "right":
			v.typ = (v.typ + 1) % len(jobTypes)
		}
		return njNone
	}

	// Title field: route the key to the text input (cursor movement, editing).
	m, _ := v.title.Update(msg)
	v.title = m
	return njNone
}

// typeValue returns the selected type string.
func (v *newJobView) typeValue() string { return jobTypes[v.typ] }

// render draws the form.
func (v *newJobView) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("New job"))
	b.WriteString("\n\n")

	// Title row.
	label := "  Title: "
	if v.focus == 0 {
		b.WriteString(label + v.title.View())
	} else {
		b.WriteString(dimStyle.Render(label) + dimStyle.Render(v.title.View()))
	}
	b.WriteString("\n\n")

	// Type row.
	b.WriteString("  Type:  ")
	for i, t := range jobTypes {
		var cell string
		switch {
		case i == v.typ && v.focus == 1:
			cell = accentStyle.Render("▸ " + t)
		case i == v.typ:
			cell = lipgloss.NewStyle().Bold(true).Render("● " + t)
		default:
			cell = dimStyle.Render("○ " + t)
		}
		b.WriteString(cell)
		b.WriteString("    ")
	}
	b.WriteString("\n\n")

	// Footer: status or hint.
	if v.status != "" {
		b.WriteString(statusStyle.Render(v.status))
	} else if v.focus == 1 {
		b.WriteString(dimStyle.Render("←/→ change type · tab title · enter create · esc cancel"))
	} else {
		b.WriteString(dimStyle.Render("tab type · enter create · esc cancel"))
	}
	return b.String()
}
