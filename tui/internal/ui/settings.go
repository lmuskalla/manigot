package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lmuskalla/manigot/tui/internal/config"
)

// toolOptions mirrors the --tool values accepted by scripts/run.sh.
var toolOptions = []string{config.ToolClaudeCode, config.ToolOpenCode}

// stAction is what update returns for the App to act on.
type stAction int

const (
	stNone   stAction = iota
	stCancel          // esc — discard and return to the list
	stSubmit          // enter — persist the settings
)

// settingsView is the "Settings" form: an editor command text input and a
// cycled tool selector. Like newJobView, it does not persist anything itself
// — the App calls config.Save on submit so this stays a pure input
// component.
type settingsView struct {
	editor textinput.Model
	tool   int // index into toolOptions
	focus  int // 0 = editor, 1 = tool
	width  int
	height int
	status string // validation/save error message
}

func newSettingsView(s config.Settings, width, height int) *settingsView {
	ti := textinput.New()
	ti.Placeholder = "$VISUAL / $EDITOR / nano"
	ti.Prompt = "" // we render our own "Editor:" label
	ti.CharLimit = 120
	ti.SetValue(s.Editor)
	ti.Focus()
	ti.Width = 60
	v := &settingsView{editor: ti, tool: toolIndex(s.ToolValue()), focus: 0, width: width, height: height}
	if width > 0 {
		v.editor.Width = width - 12
		if v.editor.Width < 20 {
			v.editor.Width = 20
		}
	}
	return v
}

// toolIndex finds tool in toolOptions, defaulting to index 0 (claude-code)
// for an unrecognized value.
func toolIndex(tool string) int {
	for i, t := range toolOptions {
		if t == tool {
			return i
		}
	}
	return 0
}

// resize updates the viewport (and the editor field width) on terminal resize.
func (v *settingsView) resize(width, height int) {
	if width == v.width && height == v.height {
		return
	}
	v.width, v.height = width, height
	if width > 0 {
		w := width - 12
		if w < 20 {
			w = 20
		}
		v.editor.Width = w
	}
}

// update processes a key for the form and reports the resulting action.
func (v *settingsView) update(msg tea.KeyMsg) stAction {
	switch msg.String() {
	case "esc":
		return stCancel
	case "tab", "shift+tab":
		v.focus = 1 - v.focus
		if v.focus == 0 {
			v.editor.Focus()
		} else {
			v.editor.Blur()
		}
		return stNone
	case "enter":
		return stSubmit
	}

	if v.focus == 1 {
		// Tool selector: cycle with ←/→.
		switch msg.String() {
		case "left":
			v.tool = (v.tool + len(toolOptions) - 1) % len(toolOptions)
		case "right":
			v.tool = (v.tool + 1) % len(toolOptions)
		}
		return stNone
	}

	// Editor field: route the key to the text input (cursor movement, editing).
	m, _ := v.editor.Update(msg)
	v.editor = m
	return stNone
}

// settingsValue returns the form's current values as config.Settings, ready
// to be persisted with config.Save.
func (v *settingsView) settingsValue() config.Settings {
	return config.Settings{
		Editor: strings.TrimSpace(v.editor.Value()),
		Tool:   toolOptions[v.tool],
	}
}

// render draws the form.
func (v *settingsView) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Editor row.
	label := "  Editor: "
	if v.focus == 0 {
		b.WriteString(label + v.editor.View())
	} else {
		b.WriteString(dimStyle.Render(label) + dimStyle.Render(v.editor.View()))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("           blank = use $VISUAL / $EDITOR / nano / vi"))
	b.WriteString("\n\n")

	// Tool row.
	b.WriteString("  Tool:    ")
	for i, t := range toolOptions {
		var cell string
		switch {
		case i == v.tool && v.focus == 1:
			cell = accentStyle.Render("▸ " + t)
		case i == v.tool:
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
		b.WriteString(dimStyle.Render("←/→ change tool · tab/shift+tab editor · enter save · esc cancel"))
	} else {
		b.WriteString(dimStyle.Render("tab/shift+tab tool · enter save · esc cancel"))
	}
	return b.String()
}
