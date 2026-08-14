package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lmuskalla/manigot/internal/agentlist"
)

// apAction is what agentsPickerView.update returns for the App to act on.
type apAction int

const (
	apNone   apAction = iota
	apCancel          // esc (no filter) / q — discard and return to the list
	apSubmit          // enter — launch the highlighted agent
)

// agentPickerNameWidth is the fixed column width the agent name is padded
// to, so every description lines up regardless of name length — the same
// fixed-column convention listColumns() uses for the job list.
const agentPickerNameWidth = 16

// agentsPickerView is the "Launch an agent" picker opened by "a" from the
// list view (dashboard): every agent available to the project
// (agentlist.Discover), one per row, moved through with ↑/↓/k/j and launched
// with enter. Typing narrows the list, mirroring the shared ui.Picker the CLI
// mg agents path uses, so both menus behave identically. Like
// newJobView/settingsView, it does not launch anything itself — the App calls
// launch.AgentQuick on apSubmit so this stays a pure input component. The
// agent list itself is discovered once, by the App, before the view is opened
// (see updateList's "a" case) rather than here, so a discovery failure or an
// empty result can be surfaced as a status line without ever constructing a
// picker with nothing to show.
type agentsPickerView struct {
	agents []agentlist.Agent
	cursor int
	filter string // type-to-filter query; empty = full list
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

// update processes a key for the picker and reports the resulting action. Its
// key semantics mirror the shared ui.Picker (CLI mg agents path) exactly: esc
// clears the filter first and only falls through to cancel once it is empty;
// up/down/home/end always navigate the filtered list; backspace edits the
// filter; while a filter is active every printable key extends it (so j/k/g/G/
// q type); with no filter j/k navigate, g/G jump, q cancels, and any other
// printable key starts a filter. Enter with a filter that matches nothing is a
// no-op instead of closing the picker.
func (v *agentsPickerView) update(msg tea.KeyMsg) apAction {
	// Keys that mean the same thing whether or not a filter is active.
	switch msg.String() {
	case "enter":
		if len(v.filtered()) > 0 {
			return apSubmit
		}
		return apNone
	case "esc":
		// First esc clears a filter; only once the filter is already empty
		// does it fall through to cancel.
		if v.filter != "" {
			v.filter = ""
			v.clampCursor()
			return apNone
		}
		return apCancel
	case "up":
		if v.cursor > 0 {
			v.cursor--
		}
		return apNone
	case "down":
		if v.cursor < len(v.filtered())-1 {
			v.cursor++
		}
		return apNone
	case "home":
		v.cursor = 0
		return apNone
	case "end":
		if n := len(v.filtered()); n > 0 {
			v.cursor = n - 1
		}
		return apNone
	case "backspace":
		if v.filter != "" {
			r := []rune(v.filter)
			v.filter = string(r[:len(r)-1])
			v.clampCursor()
		}
		return apNone
	}
	// Remaining keys: printable runes (control keys, which real terminals
	// send with a non-KeyRunes type, are ignored).
	if msg.Type != tea.KeyRunes {
		return apNone
	}
	if v.filter != "" {
		// While a filter is active every printable key — including
		// j/k/q/g — extends it; navigation stays on the arrow and home/end
		// keys so the two never fight.
		v.filter += string(msg.Runes)
		v.clampCursor()
		return apNone
	}
	switch msg.String() {
	case "q":
		return apCancel
	case "k":
		if v.cursor > 0 {
			v.cursor--
		}
	case "j":
		if v.cursor < len(v.filtered())-1 {
			v.cursor++
		}
	case "g":
		v.cursor = 0
	case "G":
		if n := len(v.filtered()); n > 0 {
			v.cursor = n - 1
		}
	default:
		v.filter += string(msg.Runes)
		v.clampCursor()
	}
	return apNone
}

// filtered returns the agents matching the active filter — the full list when
// the filter is empty. Matching is a case-insensitive substring test against
// the agent's name and description joined with a space, the same SearchKey the
// CLI mg agents path builds for its shared Picker.
func (v *agentsPickerView) filtered() []agentlist.Agent {
	if v.filter == "" {
		return v.agents
	}
	q := strings.ToLower(v.filter)
	out := make([]agentlist.Agent, 0, len(v.agents))
	for _, ag := range v.agents {
		if strings.Contains(strings.ToLower(ag.Name+" "+ag.Description), q) {
			out = append(out, ag)
		}
	}
	return out
}

// clampCursor keeps the cursor inside the filtered agent list, so a filter
// that narrows the list can never leave it pointing at a row that no longer
// matches (mirroring the shared Picker's clampView).
func (v *agentsPickerView) clampCursor() {
	n := len(v.filtered())
	if n == 0 {
		v.cursor = 0
		return
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= n {
		v.cursor = n - 1
	}
}

// selected returns the agent under the cursor, or false if the filtered list
// is empty — mirroring App.selectedJob's same guard for the job list cursor.
func (v *agentsPickerView) selected() (agentlist.Agent, bool) {
	rows := v.filtered()
	if len(rows) == 0 || v.cursor < 0 || v.cursor >= len(rows) {
		return agentlist.Agent{}, false
	}
	return rows[v.cursor], true
}

// render draws the picker: title, the active filter line when one is set,
// one row per matching agent (name padded to agentPickerNameWidth,
// description dimmed alongside it, the cursor row highlighted the same way
// renderJobRow highlights the selected job), a "no matches" hint when a
// filter matches nothing, and a footer key hint that changes while a filter
// is active (mirroring the shared Picker's View).
func (v *agentsPickerView) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Launch an agent"))
	b.WriteString("\n\n")
	if v.filter != "" {
		b.WriteString(accentStyle.Render("filter: " + v.filter))
		b.WriteString("\n")
	}

	agents := v.filtered()
	if len(agents) == 0 {
		// "no matches" only makes sense for a filter that matched nothing; a
		// picker with no agents at all renders an empty list instead.
		if v.filter != "" {
			b.WriteString(dimStyle.Render("  no matches"))
			b.WriteString("\n")
		}
	} else {
		for i, ag := range agents {
			name := pad(ag.Name, agentPickerNameWidth)
			// Cap the description to the shared AgentDescriptionWidth and to the
			// room left on the row after its fixed prefix (2-col marker + name
			// column + 2-col gap = 20), so a long description can never push a
			// row past the terminal edge. Short descriptions pass through whole.
			desc := Truncate(ag.Description, clamp(v.width-20, 1, AgentDescriptionWidth))
			if i == v.cursor {
				line := "▶ " + name + "  " + desc
				b.WriteString(selectedStyle.Render(line))
			} else {
				line := "  " + name + "  "
				b.WriteString(dimStyle.Render(line) + dimStyle.Render(desc))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(v.footer()))
	return b.String()
}

// footer renders the key hint, which changes while a filter is active: every
// printable key extends it and esc's first press clears it rather than
// cancelling — the same split the shared Picker's footer uses.
func (v *agentsPickerView) footer() string {
	if v.filter != "" {
		return "↑/↓ navigate · enter launch · esc clear filter"
	}
	return "↑/↓/k/j navigate · enter launch · type to filter · esc/q cancel"
}
