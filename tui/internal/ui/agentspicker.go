package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lmuskalla/manigot/tui/internal/agentlist"
)

// apAction is what agentsPickerView.update returns for the App to act on.
type apAction int

const (
	apNone   apAction = iota
	apCancel          // esc — discard and return to the list
	apSubmit          // enter — launch the highlighted agent
)

// agentPickerNameWidth is the fixed column width the agent name is padded
// to, so every description lines up regardless of name length — the same
// fixed-column convention listColumns() uses for the job list.
const agentPickerNameWidth = 16

// agentsPickerView is the "Launch an agent" picker opened by "a" from the
// list view (dashboard): every agent available to the project (TASK-1's
// agentlist.Discover), one per row, moved through with ↑/↓/k/j and launched
// with enter. Like newJobView/settingsView, it does not launch anything
// itself — the App calls launch.AgentQuick on apSubmit so this stays a pure
// input component. The agent list itself is discovered once, by the App,
// before the view is opened (see updateList's "a" case) rather than here, so
// a discovery failure or an empty result can be surfaced as a status line
// without ever constructing a picker with nothing to show.
type agentsPickerView struct {
	agents []agentlist.Agent
	cursor int
	width  int
	height int
}

// newAgentsPickerView builds the picker from an already-discovered agent
// list. Callers must not pass an empty slice — see updateList's "a" case,
// which degrades that case to a status line instead of opening the picker.
func newAgentsPickerView(agents []agentlist.Agent, width, height int) *agentsPickerView {
	return &agentsPickerView{agents: agents, width: width, height: height}
}

// resize updates the viewport on terminal resize.
func (v *agentsPickerView) resize(width, height int) {
	v.width, v.height = width, height
}

// update processes a key for the picker and reports the resulting action.
func (v *agentsPickerView) update(msg tea.KeyMsg) apAction {
	switch msg.String() {
	case "esc":
		return apCancel
	case "up", "k":
		if v.cursor > 0 {
			v.cursor--
		}
	case "down", "j":
		if v.cursor < len(v.agents)-1 {
			v.cursor++
		}
	case "home", "g":
		v.cursor = 0
	case "end", "G":
		if len(v.agents) > 0 {
			v.cursor = len(v.agents) - 1
		}
	case "enter":
		return apSubmit
	}
	return apNone
}

// selected returns the agent under the cursor, or false if the list is empty
// — mirroring App.selectedJob's same guard for the job list cursor.
func (v *agentsPickerView) selected() (agentlist.Agent, bool) {
	if len(v.agents) == 0 || v.cursor < 0 || v.cursor >= len(v.agents) {
		return agentlist.Agent{}, false
	}
	return v.agents[v.cursor], true
}

// render draws the picker: title, one row per agent (name padded to
// agentPickerNameWidth, description dimmed alongside it, the cursor row
// highlighted the same way renderJobRow highlights the selected job), and a
// footer key hint.
func (v *agentsPickerView) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Launch an agent"))
	b.WriteString("\n\n")

	for i, ag := range v.agents {
		name := pad(ag.Name, agentPickerNameWidth)
		if i == v.cursor {
			line := "▶ " + name + "  " + ag.Description
			b.WriteString(selectedStyle.Render(line))
		} else {
			line := "  " + name + "  "
			b.WriteString(dimStyle.Render(line) + dimStyle.Render(ag.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ navigate · enter launch · esc cancel"))
	return b.String()
}
