package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/project"
)

func TestSettingsInitialFocus(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	if v.focus != stFocusEditor {
		t.Errorf("initial focus = %d, want %d (editor)", v.focus, stFocusEditor)
	}
}

func TestSettingsActions(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	if v.update(key(t, tea.KeyEscape)) != stCancel {
		t.Error("esc should cancel")
	}
	if v.update(key(t, tea.KeyEnter)) != stSubmit {
		t.Error("enter should submit")
	}
}

func TestSettingsTabCyclesFocus(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	if v.focus != stFocusEditor {
		t.Fatal("expected initial focus editor")
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != stFocusBranch {
		t.Errorf("after tab, focus = %d, want %d (base branch)", v.focus, stFocusBranch)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != stFocusJobPrefix {
		t.Errorf("after second tab, focus = %d, want %d (job branch prefix)", v.focus, stFocusJobPrefix)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != stFocusCount {
		t.Errorf("after third tab, focus = %d, want %d (recent activity count)", v.focus, stFocusCount)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != stFocusProfile {
		t.Errorf("after fourth tab, focus = %d, want %d (profile)", v.focus, stFocusProfile)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != stFocusTerminal {
		t.Errorf("after fifth tab, focus = %d, want %d (terminal)", v.focus, stFocusTerminal)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != stFocusEditor {
		t.Errorf("after sixth tab (wrap), focus = %d, want %d (editor)", v.focus, stFocusEditor)
	}
}

func TestSettingsShiftTabCyclesFocusBackward(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	if v.focus != stFocusEditor {
		t.Fatal("expected initial focus editor")
	}
	// shift+tab from editor wraps back to terminal.
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusTerminal {
		t.Errorf("after shift+tab, focus = %d, want %d (terminal)", v.focus, stFocusTerminal)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusProfile {
		t.Errorf("after second shift+tab, focus = %d, want %d (profile)", v.focus, stFocusProfile)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusCount {
		t.Errorf("after third shift+tab, focus = %d, want %d (recent activity count)", v.focus, stFocusCount)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusJobPrefix {
		t.Errorf("after fourth shift+tab, focus = %d, want %d (job branch prefix)", v.focus, stFocusJobPrefix)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusBranch {
		t.Errorf("after fifth shift+tab, focus = %d, want %d (base branch)", v.focus, stFocusBranch)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusEditor {
		t.Errorf("after sixth shift+tab, focus = %d, want %d (editor)", v.focus, stFocusEditor)
	}
}

func TestSettingsProfileCycleOnlyWhenProfileFocused(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	// focus on editor: left/right must not change the profile.
	before := v.profile
	v.update(key(t, tea.KeyRight))
	if v.profile != before {
		t.Errorf("left/right while editor focused changed profile: %d -> %d", before, v.profile)
	}

	// focus on profile (four tabs away now): right cycles claude-pro -> zai -> opencode-go -> claude-pro.
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyTab)) // base branch -> job branch prefix
	v.update(key(t, tea.KeyTab)) // job branch prefix -> recent activity count
	v.update(key(t, tea.KeyTab)) // recent activity count -> profile
	// left/right must not mutate the text inputs while profile is focused either.
	branchBefore := v.baseBranch.Value()
	v.update(key(t, tea.KeyRight))
	if v.baseBranch.Value() != branchBefore {
		t.Errorf("right while profile focused edited the base branch field: %q -> %q", branchBefore, v.baseBranch.Value())
	}
	if got := v.settingsValue().Profile; got != config.ProfileZAI {
		t.Errorf("after right: profile = %q, want %q", got, config.ProfileZAI)
	}
	v.update(key(t, tea.KeyRight))
	if got := v.settingsValue().Profile; got != config.ProfileOpenCodeGo {
		t.Errorf("after right: profile = %q, want %q", got, config.ProfileOpenCodeGo)
	}
	v.update(key(t, tea.KeyRight))
	if got := v.settingsValue().Profile; got != config.ProfileClaudePro {
		t.Errorf("after right wrap: profile = %q, want %q", got, config.ProfileClaudePro)
	}
	// left cycles backwards.
	v.update(key(t, tea.KeyLeft))
	if got := v.settingsValue().Profile; got != config.ProfileOpenCodeGo {
		t.Errorf("after left: profile = %q, want %q", got, config.ProfileOpenCodeGo)
	}
}

func TestSettingsEditorEdits(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	v.update(key(t, tea.KeyRunes, 'h', 'i'))
	if got := v.editor.Value(); got != "hi" {
		t.Errorf("editor after typing = %q, want hi", got)
	}
}

func TestSettingsBaseBranchEdits(t *testing.T) {
	v := newSettingsView(config.Settings{Editor: "x"}, project.Settings{BaseBranch: "main"}, 80, 24)
	// Focus the base branch field, then type into it.
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyRunes, 'd', 'e', 'v'))
	if got := v.baseBranch.Value(); got != "maindev" {
		t.Errorf("base branch after typing = %q, want maindev", got)
	}
	// Typing into base branch must not leak into the editor.
	if got := v.editor.Value(); got != "x" {
		t.Errorf("editor leaked = %q, want x", got)
	}
}

func TestSettingsSeededFromProjectSettings(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{BaseBranch: "develop"}, 80, 24)
	if got := v.baseBranch.Value(); got != "develop" {
		t.Errorf("base branch seed = %q, want develop", got)
	}
	if got := v.projectValue().BaseBranch; got != "develop" {
		t.Errorf("projectValue after seed = %q, want develop", got)
	}
}

func TestSettingsJobPrefixEdits(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	// Focus the job branch prefix field (two tabs), then type into it.
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyTab)) // base branch -> job branch prefix
	v.update(key(t, tea.KeyRunes, 'j', 'o', 'b', 's'))
	if got := v.jobBranchPrefix.Value(); got != "jobs" {
		t.Errorf("job branch prefix after typing = %q, want jobs", got)
	}
	// projectValue carries it through, trimmed.
	if got := v.projectValue().JobBranchPrefix; got != "jobs" {
		t.Errorf("projectValue after typing = %q, want jobs", got)
	}
	// Typing into job prefix must not leak into the other fields.
	if got := v.editor.Value(); got != "" {
		t.Errorf("editor leaked = %q, want empty", got)
	}
	if got := v.baseBranch.Value(); got != "" {
		t.Errorf("base branch leaked = %q, want empty", got)
	}
}

func TestSettingsJobPrefixSeededFromProjectSettings(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{JobBranchPrefix: "mg"}, 80, 24)
	if got := v.jobBranchPrefix.Value(); got != "mg" {
		t.Errorf("job branch prefix seed = %q, want mg", got)
	}
	if got := v.projectValue().JobBranchPrefix; got != "mg" {
		t.Errorf("projectValue after seed = %q, want mg", got)
	}
}

func TestSettingsProjectValueTrimsAndStaysSeparateFromGlobal(t *testing.T) {
	v := newSettingsView(config.Settings{Editor: "vim", Profile: config.ProfileZAI}, project.Settings{}, 80, 24)
	// Set a base branch with surrounding whitespace; projectValue should trim it.
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyRunes, ' ', 't', 'r', 'u', 'n', 'k', ' '))
	if got := v.projectValue(); got != (project.Settings{BaseBranch: "trunk"}) {
		t.Errorf("projectValue = %+v, want {trunk}", got)
	}
	// Global settings are unaffected by base branch edits (the recent-activity
	// count field carries its seeded default).
	if got := v.settingsValue(); got != (config.Settings{Editor: "vim", Profile: config.ProfileZAI, RecentActivityCount: config.DefaultRecentActivityCount}) {
		t.Errorf("settingsValue leaked from base branch = %+v, want {vim zai count:%d}", got, config.DefaultRecentActivityCount)
	}
}

func TestSettingsRender(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	v.update(key(t, tea.KeyRunes, 'a', 'b', 'c'))
	out := v.render()
	for _, want := range []string{"Editor:", "Base branch", "Job branch prefix", "Recent activity:", "claude-pro", "zai", "opencode-go", "abc", "Profile", "recent activity strip", "Terminal:", "auto-detect"} {
		if !contains(out, want) {
			t.Errorf("render missing %q", want)
		}
	}
}

func TestSettingsTerminalSeeded(t *testing.T) {
	v := newSettingsView(config.Settings{Terminal: "kitty"}, project.Settings{}, 80, 24)
	if got := v.terminal.Value(); got != "kitty" {
		t.Errorf("terminal seed = %q, want kitty", got)
	}
	if got := v.settingsValue().Terminal; got != "kitty" {
		t.Errorf("settingsValue().Terminal = %q, want kitty", got)
	}
}

func TestSettingsTerminalEdits(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyTab)) // base branch -> job branch prefix
	v.update(key(t, tea.KeyTab)) // job branch prefix -> recent activity count
	v.update(key(t, tea.KeyTab)) // recent activity count -> profile
	v.update(key(t, tea.KeyTab)) // profile -> terminal
	if v.focus != stFocusTerminal {
		t.Fatalf("focus = %d, want %d (terminal)", v.focus, stFocusTerminal)
	}
	v.update(key(t, tea.KeyRunes, 'a', 'l', 'a', 'c', 'r', 'i', 't', 't', 'y'))
	if got := v.terminal.Value(); got != "alacritty" {
		t.Errorf("terminal after typing = %q, want alacritty", got)
	}
	// Typing into terminal must not leak into the other fields.
	if got := v.editor.Value(); got != "" {
		t.Errorf("editor leaked = %q, want empty", got)
	}
}

func TestSettingsTerminalValueTrims(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	v.terminal.SetValue("  kitty  ")
	if got := v.settingsValue().Terminal; got != "kitty" {
		t.Errorf("settingsValue().Terminal = %q, want trimmed kitty", got)
	}
}

func TestSettingsRecentCountSeeded(t *testing.T) {
	// An explicit value seeds the field with the resolved number.
	v := newSettingsView(config.Settings{RecentActivityCount: 12}, project.Settings{}, 80, 24)
	if got := v.recentCount.Value(); got != "12" {
		t.Errorf("recent count seed = %q, want 12", got)
	}
	// Unset (0) seeds with the default, so the user always sees the effective
	// value.
	v = newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	if got := v.recentCount.Value(); got != "5" {
		t.Errorf("recent count default seed = %q, want 5", got)
	}
}

func TestSettingsRecentCountEdits(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyTab)) // base branch -> job branch prefix
	v.update(key(t, tea.KeyTab)) // job branch prefix -> recent activity count
	v.recentCount.SetValue("")   // clear the seeded default
	v.update(key(t, tea.KeyRunes, '3'))
	if got := v.recentCount.Value(); got != "3" {
		t.Errorf("recent count after typing = %q, want 3", got)
	}
	// Typing into recent count must not leak into the other fields.
	if got := v.editor.Value(); got != "" {
		t.Errorf("editor leaked = %q, want empty", got)
	}
	if got := v.baseBranch.Value(); got != "" {
		t.Errorf("base branch leaked = %q, want empty", got)
	}
}

func TestSettingsRecentCountValidation(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)

	// Blank -> default.
	v.recentCount.SetValue("")
	if n, err := v.recentActivityCount(); err != nil || n != config.DefaultRecentActivityCount {
		t.Errorf("blank count = (%d, %v), want (%d, nil)", n, err, config.DefaultRecentActivityCount)
	}

	// Valid integer in range -> parsed, and reflected in settingsValue.
	v.recentCount.SetValue("12")
	if n, err := v.recentActivityCount(); err != nil || n != 12 {
		t.Errorf("12 = (%d, %v), want (12, nil)", n, err)
	}
	if got := v.settingsValue().RecentActivityCount; got != 12 {
		t.Errorf("settingsValue().RecentActivityCount = %d, want 12", got)
	}

	// Anything else -> error, and settingsValue must not smuggle it out.
	for _, bad := range []string{"abc", "0", "-1", "101", "12.5"} {
		v.recentCount.SetValue(bad)
		if _, err := v.recentActivityCount(); err == nil {
			t.Errorf("count %q accepted, want an error", bad)
		}
		if got := v.settingsValue().RecentActivityCount; got != 0 {
			t.Errorf("invalid count %q leaked into settingsValue as %d, want 0", bad, got)
		}
	}
}

func TestSettingsSubmitInvalidCountKeepsFormOpen(t *testing.T) {
	root := t.TempDir()
	a := NewApp(root, nil)
	a.width, a.height = 80, 24
	a.settings.RecentActivityCount = 7 // pin regardless of on-disk settings
	a.updateList(keyMsg("s"))          // open the settings form
	if a.settingsView == nil {
		t.Fatal("settings view did not open")
	}

	a.settingsView.recentCount.SetValue("abc")
	a.updateSettings(key(t, tea.KeyEnter)) // submit

	// The form must stay open with a status message, and nothing persisted.
	if a.settingsView == nil {
		t.Fatal("settings form closed despite an invalid recent activity count")
	}
	if a.settingsView.status == "" {
		t.Error("status not set for an invalid recent activity count")
	}
	if a.settings.RecentActivityCount != 7 {
		t.Errorf("in-memory settings changed on invalid submit: RecentActivityCount = %d, want 7", a.settings.RecentActivityCount)
	}
}
