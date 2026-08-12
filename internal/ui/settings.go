package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/project"
)

// profileOptions mirrors the profiles accepted by scripts/run.sh's --profile
// flag, in cycle order.
var profileOptions = config.Profiles()

// recentActivityCountMax is the upper end of the valid range for the
// recent-activity count setting (the floor is 1). Values outside 1–100 are
// rejected by the settings form.
const recentActivityCountMax = 100

// Number of focusable fields in the settings form: editor, base branch, job
// branch prefix, recent activity count, profile, terminal — in tab cycle
// order. Kept as a named constant so the tab/shift+tab cycle and focus
// comparisons don't all hardcode a literal that has to be kept in sync field
// by field.
const stFieldCount = 6

// Focus indices for the settings form's fields, in tab cycle order: editor →
// base branch → job branch prefix → recent activity count → profile →
// terminal → editor. Used by update() and render() instead of bare ints.
// Terminal was appended after Profile (TASK-1 point 4 of
// docs/jobs/r5x2a7_add-new-global-setting/tasks.md) rather than inserted
// next to Editor, so the existing constants keep their values unchanged.
const (
	stFocusEditor    = 0
	stFocusBranch    = 1
	stFocusJobPrefix = 2
	stFocusCount     = 3
	stFocusProfile   = 4
	stFocusTerminal  = 5
)

// stAction is what update returns for the App to act on.
type stAction int

const (
	stNone   stAction = iota
	stCancel          // esc — discard and return to the list
	stSubmit          // enter — persist the settings
)

// settingsView is the "Settings" form. It edits three files at once:
//
//   - config/tui-settings.json — the editor preference and the recent-activity
//     count (personal, gitignored, in the manigot checkout);
//   - manigot/.env — the subscription profile, as MANIGOT_PROFILE: the one
//     default shared between CLI and TUI (bare `mg` resolves to it, `mg
//     profiles` writes it, and this form reads/writes the same key); and
//   - the project.Settings (base branch, job branch prefix) — shared project
//     conventions, committable, in the target project's .manigot/manigot.json.
//
// Like newJobView, it does not persist anything itself — the App calls
// config.Save and project.Save on submit so this stays a pure input
// component.
type settingsView struct {
	editor          textinput.Model
	baseBranch      textinput.Model
	jobBranchPrefix textinput.Model
	recentCount     textinput.Model
	terminal        textinput.Model
	profile         int // index into profileOptions
	focus           int // stFocusEditor / stFocusBranch / stFocusJobPrefix / stFocusCount / stFocusProfile / stFocusTerminal
	width           int
	height          int
	status          string // validation/save error message
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

	jobBranchPrefix := textinput.New()
	jobBranchPrefix.Placeholder = "jobs"
	jobBranchPrefix.Prompt = "" // we render our own "Job branch prefix:" label
	jobBranchPrefix.CharLimit = 120
	jobBranchPrefix.SetValue(proj.JobBranchPrefix)
	jobBranchPrefix.Width = 60

	recentCount := textinput.New()
	recentCount.Placeholder = strconv.Itoa(config.DefaultRecentActivityCount)
	recentCount.Prompt = "" // we render our own "Recent activity:" label
	recentCount.CharLimit = 3
	// Seed with the resolved number so the user sees the effective value;
	// clearing it means "default" (see recentActivityCount).
	recentCount.SetValue(strconv.Itoa(global.RecentActivityCountValue()))
	recentCount.Width = 60

	terminal := textinput.New()
	terminal.Placeholder = "blank = auto-detect"
	terminal.Prompt = "" // we render our own "Terminal:" label
	terminal.CharLimit = 120
	terminal.SetValue(global.Terminal)
	terminal.Width = 60

	v := &settingsView{
		editor:          editor,
		baseBranch:      baseBranch,
		jobBranchPrefix: jobBranchPrefix,
		recentCount:     recentCount,
		terminal:        terminal,
		profile:         profileIndex(global.ProfileValue()),
		focus:           stFocusEditor,
		width:           width,
		height:          height,
	}
	if width > 0 {
		w := stInputWidth(width)
		v.editor.Width = w
		v.baseBranch.Width = w
		v.jobBranchPrefix.Width = w
		v.recentCount.Width = w
		v.terminal.Width = w
	}
	return v
}

// stInputWidth returns the usable width for a settings text input, given the
// available form width. All text inputs share the same width rule.
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
		v.jobBranchPrefix.Width = w
		v.recentCount.Width = w
		v.terminal.Width = w
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

	// A text input is focused (editor, base branch, job branch prefix, recent
	// count or terminal): route the key to it.
	var ti *textinput.Model
	switch v.focus {
	case stFocusEditor:
		ti = &v.editor
	case stFocusBranch:
		ti = &v.baseBranch
	case stFocusJobPrefix:
		ti = &v.jobBranchPrefix
	case stFocusCount:
		ti = &v.recentCount
	default: // stFocusTerminal
		ti = &v.terminal
	}
	m, _ := ti.Update(msg)
	*ti = m
	return stNone
}

// setFocus moves focus to field i and keeps the text inputs' focus state in
// sync: only the editor, base branch, job branch prefix, recent count or
// terminal holds the text cursor; the profile selector is keyboard-driven
// (←/→), not a text input.
func (v *settingsView) setFocus(i int) {
	v.focus = i
	v.editor.Blur()
	v.baseBranch.Blur()
	v.jobBranchPrefix.Blur()
	v.recentCount.Blur()
	v.terminal.Blur()
	switch i {
	case stFocusEditor:
		v.editor.Focus()
	case stFocusBranch:
		v.baseBranch.Focus()
	case stFocusJobPrefix:
		v.jobBranchPrefix.Focus()
	case stFocusCount:
		v.recentCount.Focus()
	case stFocusTerminal:
		v.terminal.Focus()
	default: // stFocusProfile
		// no text input to focus — the profile selector is keyboard-driven
	}
}

// recentActivityCount parses and validates the form's recent-activity count
// field. A trimmed empty input means "unset → default"
// (config.DefaultRecentActivityCount). Anything that does not parse as an
// integer in 1–recentActivityCountMax returns an error so the caller can
// surface it and keep the form open without persisting anything.
func (v *settingsView) recentActivityCount() (int, error) {
	raw := strings.TrimSpace(v.recentCount.Value())
	if raw == "" {
		return config.DefaultRecentActivityCount, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > recentActivityCountMax {
		return 0, fmt.Errorf("recent activity count must be an integer between 1 and %d (blank = %d)", recentActivityCountMax, config.DefaultRecentActivityCount)
	}
	return n, nil
}

// settingsValue returns the form's current global values as config.Settings,
// ready to be persisted with config.Save. The recent-activity count is
// included only when valid — callers must run recentActivityCount() first (as
// updateSettings does) so an invalid value never silently reaches disk as 0.
func (v *settingsView) settingsValue() config.Settings {
	s := config.Settings{
		Editor:   strings.TrimSpace(v.editor.Value()),
		Profile:  profileOptions[v.profile].ID,
		Terminal: strings.TrimSpace(v.terminal.Value()),
	}
	if n, err := v.recentActivityCount(); err == nil {
		s.RecentActivityCount = n
	}
	return s
}

// projectValue returns the form's current project values as project.Settings,
// ready to be persisted with project.Save.
func (v *settingsView) projectValue() project.Settings {
	return project.Settings{
		BaseBranch:      strings.TrimSpace(v.baseBranch.Value()),
		JobBranchPrefix: strings.TrimSpace(v.jobBranchPrefix.Value()),
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
	// writes a different file (.manigot/manigot.json, committable) than the
	// editor/profile rows above and below it.
	branchLabel := "  Base branch (project): "
	if v.focus == stFocusBranch {
		b.WriteString(branchLabel + v.baseBranch.View())
	} else {
		b.WriteString(dimStyle.Render(branchLabel) + dimStyle.Render(v.baseBranch.View()))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("           blank = main · stored in .manigot/manigot.json, shared with your team"))
	b.WriteString("\n\n")

	// Job branch prefix row (project): the namespace job branches live under,
	// e.g. "jobs" makes a feature job's branch "jobs/feature/<id>_<slug>".
	// Project-scoped like base branch, so the same project-file note applies.
	prefixLabel := "  Job branch prefix (project): "
	if v.focus == stFocusJobPrefix {
		b.WriteString(prefixLabel + v.jobBranchPrefix.View())
	} else {
		b.WriteString(dimStyle.Render(prefixLabel) + dimStyle.Render(v.jobBranchPrefix.View()))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("           blank = feature/… · stored in .manigot/manigot.json, shared with your team"))
	b.WriteString("\n\n")

	// Recent activity count row (global): the maximum number of entries the
	// dashboard's recent-activity strip may show.
	countLabel := "  Recent activity: "
	if v.focus == stFocusCount {
		b.WriteString(countLabel + v.recentCount.View())
	} else {
		b.WriteString(dimStyle.Render(countLabel) + dimStyle.Render(v.recentCount.View()))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("           blank = 5 · max entries shown in the dashboard's recent activity strip, stored in config/tui-settings.json"))
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
	// Describe the selected profile: which agent CLI it launches and what it
	// bills against, then where the choice is stored. The model is
	// deliberately not shown — the opencode profiles' model lives in manigot's
	// .env (OPENCODE_ZAI_MODEL/OPENCODE_GO_MODEL), which the TUI does not read.
	sel := profileOptions[v.profile]
	selDesc := "tool: " + sel.Tool
	selDesc += " · billed via " + sel.Auth
	b.WriteString(dimStyle.Render("    " + selDesc))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("    saved as MANIGOT_PROFILE in manigot/.env — shared with the CLI (bare mg / mg profiles)"))
	b.WriteString("\n\n")

	// Terminal row (global): the command used to spawn an agent session's
	// terminal, overriding launch's auto-detect spawn order when set.
	terminalLabel := "  Terminal: "
	if v.focus == stFocusTerminal {
		b.WriteString(terminalLabel + v.terminal.View())
	} else {
		b.WriteString(dimStyle.Render(terminalLabel) + dimStyle.Render(v.terminal.View()))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("           blank = auto-detect (tmux / Terminal.app / gnome-terminal / ...) · stored in config/tui-settings.json"))
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
		return "tab/shift+tab job branch prefix · " + prefix
	case stFocusJobPrefix:
		return "tab/shift+tab recent activity · " + prefix
	case stFocusCount:
		return "tab/shift+tab profile · " + prefix
	case stFocusProfile:
		return "←/→ change profile · tab/shift+tab terminal · " + prefix
	default: // stFocusTerminal
		return "tab/shift+tab editor · " + prefix
	}
}
