package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lmuskalla/manigot/tui/internal/config"
	"github.com/lmuskalla/manigot/tui/internal/project"
)

// profileOptions mirrors the profiles accepted by scripts/run.sh's --profile
// flag, in cycle order.
var profileOptions = config.Profiles()

// Number of focusable fields in the settings form: editor, base branch,
// profile — in tab cycle order. Kept as a named constant so the tab/shift+tab
// cycle and focus comparisons don't all hardcode a literal that has to be
// kept in sync field by field.
const stFieldCount = 3

// Focus indices for the settings form's fields, in tab cycle order: editor →
// base branch → profile → editor. Used by update() and render() instead of
// bare ints.
const (
	stFocusEditor = 0
	stFocusBranch = 1
	stFocusProfile = 2
)

// stAction is what update returns for the App to act on.
type stAction int

const (
	stNone   stAction = iota
	stCancel          // esc — discard and return to the list
	stSubmit          // enter — persist the settings
)

// settingsView is the "Settings" form. It edits two separate files at once:
//
//   - the global config.Settings (editor + subscription profile) — personal
//     prefs, gitignored, in the manigot checkout; and
//   - the project.Settings (base branch) — shared project convention,
//     committable, in the target project's docs/manigot.json.
//
// Like newJobView, it does not persist anything itself — the App calls
// config.Save and project.Save on submit so this stays a pure input
// component.
type settingsView struct {
	editor     textinput.Model
	baseBranch textinput.Model
	profile    int // index into profileOptions
	focus      int // stFocusEditor / stFocusBranch / stFocusProfile
	width      int
	height     int
	status     string // validation/save error message
}

// newSettingsView builds the form seeded from the current global config and
// project settings. The editor field is focused first, matching the form's
// pre-base-branch behaviour.
func newSettingsView(global config.Settings, proj project.Settings, width, height int) *settingsView {
	editor := textinput.New()
	editor.Placeholder = "$VISUAL / $EDITOR / nano"
	editor.Prompt = "" // we render our own "Editor:" label
	editor.CharLimit = 120
	editor.SetValue(global.Editor)
	editor.Focus()
	editor.Width = 60

	baseBranch := textinput.New()
	baseBranch.Placeholder = "main"
	baseBranch.Prompt = "" // we render our own "Base branch:" label
	baseBranch.CharLimit = 120
	baseBranch.SetValue(proj.BaseBranch)
	baseBranch.Width = 60

	v := &settingsView{
		editor:     editor,
		baseBranch: baseBranch,
		profile:    profileIndex(global.ProfileValue()),
		focus:      stFocusEditor,
		width:      width,
		height:     height,
	}
	if width > 0 {
		w := stInputWidth(width)
		v.editor.Width = w
		v.baseBranch.Width = w
	}
	return v
}

// stInputWidth returns the usable width for a settings text input, given the
// available form width. Both text inputs share the same width rule.
func stInputWidth(width int) int {
	w := width - 12
	if w < 20 {
		w = 20
	}
	return w
}

// profileIndex finds id in profileOptions, defaulting to index 0 (claude-pro)
// for an unrecognized value.
func profileIndex(id string) int {
	for i, p := range profileOptions {
		if p.ID == id {
			return i
		}
	}
	return 0
}

// resize updates the viewport (and the text input widths) on terminal resize.
func (v *settingsView) resize(width, height int) {
	if width == v.width && height == v.height {
		return
	}
	v.width, v.height = width, height
	if width > 0 {
		w := stInputWidth(width)
		v.editor.Width = w
		v.baseBranch.Width = w
	}
}

// update processes a key for the form and reports the resulting action.
func (v *settingsView) update(msg tea.KeyMsg) stAction {
	switch msg.String() {
	case "esc":
		return stCancel
	case "tab":
		v.setFocus((v.focus + 1) % stFieldCount)
		return stNone
	case "shift+tab":
		v.setFocus((v.focus + stFieldCount - 1) % stFieldCount)
		return stNone
	case "enter":
		return stSubmit
	}

	if v.focus == stFocusProfile {
		// Profile selector: cycle with ←/→.
		switch msg.String() {
		case "left":
			v.profile = (v.profile + len(profileOptions) - 1) % len(profileOptions)
		case "right":
			v.profile = (v.profile + 1) % len(profileOptions)
		}
		return stNone
	}

	// A text input is focused (editor or base branch): route the key to it.
	var ti *textinput.Model
	if v.focus == stFocusEditor {
		ti = &v.editor
	} else {
		ti = &v.baseBranch
	}
	m, _ := ti.Update(msg)
	*ti = m
	return stNone
}

// setFocus moves focus to field i and keeps the text inputs' focus state in
// sync: only the editor or base branch holds the text cursor; the profile
// selector is keyboard-driven (←/→), not a text input.
func (v *settingsView) setFocus(i int) {
	v.focus = i
	switch i {
	case stFocusEditor:
		v.editor.Focus()
		v.baseBranch.Blur()
	case stFocusBranch:
		v.editor.Blur()
		v.baseBranch.Focus()
	default: // stFocusProfile
		v.editor.Blur()
		v.baseBranch.Blur()
	}
}

// settingsValue returns the form's current global values as config.Settings,
// ready to be persisted with config.Save.
func (v *settingsView) settingsValue() config.Settings {
	return config.Settings{
		Editor:  strings.TrimSpace(v.editor.Value()),
		Profile: profileOptions[v.profile].ID,
	}
}

// projectValue returns the form's current project values as project.Settings,
// ready to be persisted with project.Save.
func (v *settingsView) projectValue() project.Settings {
	return project.Settings{
		BaseBranch: strings.TrimSpace(v.baseBranch.Value()),
	}
}

// render draws the form.
func (v *settingsView) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Editor row (global).
	editorLabel := "  Editor: "
	if v.focus == stFocusEditor {
		b.WriteString(editorLabel + v.editor.View())
	} else {
		b.WriteString(dimStyle.Render(editorLabel) + dimStyle.Render(v.editor.View()))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("           blank = use $VISUAL / $EDITOR / nano / vi"))
	b.WriteString("\n\n")

	// Base branch row (project). Labeled project-scoped so it's clear this
	// writes a different file (docs/manigot.json, committable) than the
	// editor/profile rows above and below it.
	branchLabel := "  Base branch (project): "
	if v.focus == stFocusBranch {
		b.WriteString(branchLabel + v.baseBranch.View())
	} else {
		b.WriteString(dimStyle.Render(branchLabel) + dimStyle.Render(v.baseBranch.View()))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("           blank = main · stored in docs/manigot.json, shared with your team"))
	b.WriteString("\n\n")

	// Profile row (global): one line per profile, the selected one highlighted.
	b.WriteString("  Profile:\n")
	for i, p := range profileOptions {
		var mark, id, rest string
		if i == v.profile && v.focus == stFocusProfile {
			mark = accentStyle.Render("▸ ")
			id = accentStyle.Render(p.ID)
		} else if i == v.profile {
			mark = lipgloss.NewStyle().Bold(true).Render("● ")
			id = lipgloss.NewStyle().Bold(true).Render(p.ID)
		} else {
			mark = "  "
			id = dimStyle.Render(p.ID)
		}
		rest = "  " + dimStyle.Render(p.Label)
		b.WriteString("    " + mark + id + rest)
		b.WriteString("\n")
	}
	// Describe the selected profile: tool, model, and what it bills against.
	sel := profileOptions[v.profile]
	selDesc := "tool: " + sel.Tool
	if sel.Model != "" {
		selDesc += " · model: " + sel.Model
	}
	selDesc += " · billed via " + sel.Auth
	b.WriteString(dimStyle.Render("    " + selDesc))
	b.WriteString("\n\n")

	// Footer: status or a focus-aware hint.
	if v.status != "" {
		b.WriteString(statusStyle.Render(v.status))
	} else {
		b.WriteString(dimStyle.Render(v.hint()))
	}
	return b.String()
}

// hint returns the footer key hint, varied by which field is focused so it
// always names the fields tab moves between rather than a static list.
func (v *settingsView) hint() string {
	prefix := "enter save · esc cancel"
	switch v.focus {
	case stFocusEditor:
		return "tab/shift+tab base branch · " + prefix
	case stFocusBranch:
		return "tab/shift+tab profile · " + prefix
	default: // stFocusProfile
		return "←/→ change profile · tab/shift+tab editor · " + prefix
	}
}
