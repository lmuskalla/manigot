package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lmuskalla/manigot/tui/internal/config"
	"github.com/lmuskalla/manigot/tui/internal/project"
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
	if v.focus != stFocusProfile {
		t.Errorf("after second tab, focus = %d, want %d (profile)", v.focus, stFocusProfile)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != stFocusEditor {
		t.Errorf("after third tab (wrap), focus = %d, want %d (editor)", v.focus, stFocusEditor)
	}
}

func TestSettingsShiftTabCyclesFocusBackward(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	if v.focus != stFocusEditor {
		t.Fatal("expected initial focus editor")
	}
	// shift+tab from editor wraps back to profile.
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusProfile {
		t.Errorf("after shift+tab, focus = %d, want %d (profile)", v.focus, stFocusProfile)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusBranch {
		t.Errorf("after second shift+tab, focus = %d, want %d (base branch)", v.focus, stFocusBranch)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != stFocusEditor {
		t.Errorf("after third shift+tab, focus = %d, want %d (editor)", v.focus, stFocusEditor)
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

	// focus on profile (two tabs away now): right cycles claude-pro -> zai -> opencode-go -> claude-pro.
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyTab)) // base branch -> profile
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

func TestSettingsProjectValueTrimsAndStaysSeparateFromGlobal(t *testing.T) {
	v := newSettingsView(config.Settings{Editor: "vim", Profile: config.ProfileZAI}, project.Settings{}, 80, 24)
	// Set a base branch with surrounding whitespace; projectValue should trim it.
	v.update(key(t, tea.KeyTab)) // editor -> base branch
	v.update(key(t, tea.KeyRunes, ' ', 't', 'r', 'u', 'n', 'k', ' '))
	if got := v.projectValue(); got != (project.Settings{BaseBranch: "trunk"}) {
		t.Errorf("projectValue = %+v, want {trunk}", got)
	}
	// Global settings are unaffected by base branch edits.
	if got := v.settingsValue(); got != (config.Settings{Editor: "vim", Profile: config.ProfileZAI}) {
		t.Errorf("settingsValue leaked from base branch = %+v, want {vim zai}", got)
	}
}

func TestSettingsRender(t *testing.T) {
	v := newSettingsView(config.Settings{}, project.Settings{}, 80, 24)
	v.update(key(t, tea.KeyRunes, 'a', 'b', 'c'))
	out := v.render()
	for _, want := range []string{"Editor:", "Base branch", "claude-pro", "zai", "opencode-go", "abc", "Profile"} {
		if !contains(out, want) {
			t.Errorf("render missing %q", want)
		}
	}
}
