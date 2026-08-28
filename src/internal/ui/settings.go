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
// branch prefix, recent activity count, profile, terminal, theme — in tab
// cycle order. Kept as a named constant so the tab/shift+tab cycle and focus
// comparisons don't all hardcode a literal that has to be kept in sync field
// by field.
const stFieldCount = 7

// settingsLabelWidth is the display width of the settings form's label
// column — the length of the longest setting headline ("Job branch prefix")
// — so every text input starts at the same x and the form reads as aligned
// columns rather than a ragged wall of label+input pairs.
const settingsLabelWidth = 17

// Focus indices for the settings form's fields, in tab cycle order: editor →
// recent activity count → profile → terminal → theme → base branch → job
// branch prefix → editor. This mirrors the form's top-to-bottom render order
// (render(), below) so tab always moves down the visible layout instead of
// jumping between sections. Used by update() and render() instead of bare
// ints.
const (
	stFocusEditor    = 0
	stFocusCount     = 1
	stFocusProfile   = 2
	stFocusTerminal  = 3
	stFocusTheme     = 4
	stFocusBranch    = 5
	stFocusJobPrefix = 6
)

// stAction is what update returns for the App to act on.
type stAction int

const (
	stNone   stAction = iota
	stCancel          // esc — discard and return to the list
	stSubmit          // enter — persist the settings
)

// settingsView is the "Settings" form, laid out as two bold-headed sections
// that mirror the storage split the form edits:
//
//   - Personal settings — the editor preference, the recent-activity count,
//     the subscription profile, the terminal and the theme: global, personal
//     preferences stored in config/tui-settings.json (editor, recent activity,
//     terminal) and manigot/.env (the profile as MANIGOT_PROFILE — the one
//     default shared between CLI and TUI: bare `mg` resolves to it, `mg
//     profiles` writes it, and this form reads/writes the same key — and the
//     theme as OPENCODE_THEME), both gitignored, in the manigot checkout;
//   - Project settings — the base branch and job branch prefix: shared
//     project conventions, committable, in the target project's
//     .manigot/manigot.json.
//
// Within each section every setting has a bold headline with its value next
// to it (only the input value dims when unfocused — the headline stays bold),
// and the examples are short dim lines; the persistence location is stated
// once per section (in the section headline, and for the profile in its
// headline), not per field. A blank line separates the title from the first
// section, follows each section headline, and separates each setting from the
// next (within a section and between the two sections), so the form reads as
// distinct rows rather than a packed wall of label+input pairs. The render is
// a fixed-height, non-scrollable string, so on a small terminal the bottom of
// the form may clip or scroll.
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
	theme           textinput.Model
	profile         int // index into profileOptions
	focus           int // stFocusEditor / stFocusCount / stFocusProfile / stFocusTerminal / stFocusTheme / stFocusBranch / stFocusJobPrefix
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

	theme := textinput.New()
	theme.Placeholder = "blank = OpenCode's own default"
	theme.Prompt = "" // we render our own "Theme:" label
	theme.CharLimit = 120
	theme.SetValue(global.ThemeValue())
	theme.Width = 60

	v := &settingsView{
		editor:          editor,
		baseBranch:      baseBranch,
		jobBranchPrefix: jobBranchPrefix,
		recentCount:     recentCount,
		terminal:        terminal,
		theme:           theme,
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
		v.theme.Width = w
	}
	return v
}

// stInputWidth returns the usable width for a settings text input, given the
// available form width. All text inputs share the same width rule: the label
// column (settingsLabelWidth) plus the 2-space indent and the 2-space gap
// between label and input.
func stInputWidth(width int) int {
	w := width - settingsLabelWidth - 4
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
		v.theme.Width = w
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
	// count, terminal or theme): route the key to it.
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
	case stFocusTerminal:
		ti = &v.terminal
	default: // stFocusTheme
		ti = &v.theme
	}
	m, _ := ti.Update(msg)
	*ti = m
	return stNone
}

// setFocus moves focus to field i and keeps the text inputs' focus state in
// sync: only the editor, base branch, job branch prefix, recent count,
// terminal or theme holds the text cursor; the profile selector is
// keyboard-driven (←/→), not a text input.
func (v *settingsView) setFocus(i int) {
	v.focus = i
	v.editor.Blur()
	v.baseBranch.Blur()
	v.jobBranchPrefix.Blur()
	v.recentCount.Blur()
	v.terminal.Blur()
	v.theme.Blur()
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
	case stFocusTheme:
		v.theme.Focus()
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
		Theme:    strings.TrimSpace(v.theme.Value()),
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

// settingsField renders one setting's headline row: the bold label and the
// value (a text input view) next to it, the label right-padded to
// settingsLabelWidth so every input starts at the same x. The label stays
// bold whether focused or not — only the value dims when its field is
// unfocused, so a glance at the form still finds each setting by its bold
// headline.
func settingsField(focused bool, label, value string) string {
	padded := label + strings.Repeat(" ", settingsLabelWidth-len(label))
	if focused {
		return "  " + headerStyle.Render(padded) + "  " + value
	}
	return "  " + headerStyle.Render(padded) + "  " + dimStyle.Render(value)
}

// render draws the form. The render is a raw, non-scrollable string (app.go's
// stateSettings branch): a blank line separates the title from each section
// headline, follows each section headline, and separates each setting from
// the next within a section, so the form no longer reads as a packed wall of
// label+input pairs. This exceeds the 22 content rows available at a 24-row
// terminal (24 - 2*uiPaddingY) — a small terminal will clip or scroll the
// bottom of the form. The base branch row deliberately has no example line: its
// placeholder ("main") already communicates the blank default.
func (v *settingsView) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Personal settings section (global): editor, recent activity count,
	// profile, terminal, theme. The section headline carries the storage
	// location once, so the per-field examples below can stay short.
	b.WriteString(headerStyle.Render("Personal settings"))
	b.WriteString(" — ")
	b.WriteString(dimStyle.Render("saved in config/tui-settings.json + manigot/.env"))
	b.WriteString("\n\n")

	// Editor row (global).
	b.WriteString(settingsField(v.focus == stFocusEditor, "Editor", v.editor.View()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  blank = use $VISUAL / $EDITOR / nano / vi"))
	b.WriteString("\n\n")

	// Recent activity count row (global): the maximum number of entries the
	// dashboard's recent-activity strip may show.
	b.WriteString(settingsField(v.focus == stFocusCount, "Recent activity", v.recentCount.View()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  1–100 · blank = 5 · dashboard recent activity strip"))
	b.WriteString("\n\n")

	// Profile row (global): the label has no text input next to it — the
	// options are listed below, one per line, the selected one highlighted.
	// Where the choice is stored and who it is shared with rides on the
	// headline line (saving a row the packed layout cannot afford). The model
	// is deliberately not shown — the opencode profiles' model lives in
	// manigot's .env (OPENCODE_ZAI_MODEL/OPENCODE_GO_MODEL/OPENCODE_ZEN_MODEL/
	// OPENCODE_ZEN_FREE_MODEL), which the TUI does not read.
	b.WriteString("  " + headerStyle.Render("Profile"))
	b.WriteString(" — ")
	b.WriteString(dimStyle.Render("saved as MANIGOT_PROFILE in manigot/.env, shared with the CLI"))
	b.WriteString("\n")
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
	b.WriteString("\n")

	// Terminal row (global): the command used to spawn an agent session's
	// terminal, replacing launch's auto-detect spawn order when set. Inside
	// tmux the split pane always wins, regardless of this setting.
	b.WriteString(settingsField(v.focus == stFocusTerminal, "Terminal", v.terminal.View()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  blank = auto-detect (tmux / Terminal.app / ...) · in tmux the split pane always wins"))
	b.WriteString("\n\n")

	// Theme row (global): the OpenCode theme name (e.g. "nord", "tokyonight"),
	// shared across every OpenCode profile — Claude Code already respects the
	// host terminal's own theme, so this has no effect there. Not validated
	// against a fixed list here, matching `mg theme`.
	b.WriteString(settingsField(v.focus == stFocusTheme, "Theme", v.theme.View()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  blank = OpenCode's own default · OpenCode only"))
	b.WriteString("\n")

	// Blank line separating the two sections.
	b.WriteString("\n")

	// Project settings section: base branch and job branch prefix — shared
	// project conventions, committable, in the target project's
	// .manigot/manigot.json.
	b.WriteString(headerStyle.Render("Project settings"))
	b.WriteString(" — ")
	b.WriteString(dimStyle.Render("stored in .manigot/manigot.json, shared with your team"))
	b.WriteString("\n\n")

	// Base branch row (project): no example line — the placeholder ("main")
	// already communicates the blank default.
	b.WriteString(settingsField(v.focus == stFocusBranch, "Base branch", v.baseBranch.View()))
	b.WriteString("\n\n")

	// Job branch prefix row (project): the namespace job branches live under,
	// e.g. "jobs" makes a feature job's branch "jobs/feature/<id>_<slug>".
	b.WriteString(settingsField(v.focus == stFocusJobPrefix, "Job branch prefix", v.jobBranchPrefix.View()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  blank = feature/… · namespace for job branches"))
	b.WriteString("\n")

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
		return "tab/shift+tab recent activity · " + prefix
	case stFocusCount:
		return "tab/shift+tab profile · " + prefix
	case stFocusProfile:
		return "←/→ change profile · tab/shift+tab terminal · " + prefix
	case stFocusTerminal:
		return "tab/shift+tab theme · " + prefix
	case stFocusTheme:
		return "tab/shift+tab base branch · " + prefix
	case stFocusBranch:
		return "tab/shift+tab job branch prefix · " + prefix
	default: // stFocusJobPrefix
		return "tab/shift+tab editor · " + prefix
	}
}
