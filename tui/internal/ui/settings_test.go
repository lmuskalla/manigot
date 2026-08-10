package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lmuskalla/manigot/tui/internal/config"
)

func TestSettingsInitialFocus(t *testing.T) {
	v := newSettingsView(config.Settings{}, 80, 24)
	if v.focus != 0 {
		t.Errorf("initial focus = %d, want 0 (editor)", v.focus)
	}
}

func TestSettingsActions(t *testing.T) {
	v := newSettingsView(config.Settings{}, 80, 24)
	if v.update(key(t, tea.KeyEscape)) != stCancel {
		t.Error("esc should cancel")
	}
	if v.update(key(t, tea.KeyEnter)) != stSubmit {
		t.Error("enter should submit")
	}
}

func TestSettingsTabTogglesFocus(t *testing.T) {
	v := newSettingsView(config.Settings{}, 80, 24)
	if v.focus != 0 {
		t.Fatal("expected initial focus editor")
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != 1 {
		t.Errorf("after tab, focus = %d, want 1 (tool)", v.focus)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != 0 {
		t.Errorf("after second tab, focus = %d, want 0 (editor)", v.focus)
	}
}

func TestSettingsShiftTabTogglesFocusBackward(t *testing.T) {
	v := newSettingsView(config.Settings{}, 80, 24)
	if v.focus != 0 {
		t.Fatal("expected initial focus editor")
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != 1 {
		t.Errorf("after shift+tab, focus = %d, want 1 (tool)", v.focus)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != 0 {
		t.Errorf("after second shift+tab, focus = %d, want 0 (editor)", v.focus)
	}
}

func TestSettingsToolCycleOnlyWhenToolFocused(t *testing.T) {
	v := newSettingsView(config.Settings{}, 80, 24)
	// focus on editor: left/right must not change the tool.
	before := v.tool
	v.update(key(t, tea.KeyRight))
	if v.tool != before {
		t.Errorf("left/right while editor focused changed tool: %d -> %d", before, v.tool)
	}

	// focus on tool: right cycles claude-code -> opencode -> claude-code.
	v.update(key(t, tea.KeyTab)) // now focus == tool
	v.update(key(t, tea.KeyRight))
	if got := v.settingsValue().Tool; got != config.ToolOpenCode {
		t.Errorf("after right: tool = %q, want %q", got, config.ToolOpenCode)
	}
	v.update(key(t, tea.KeyRight))
	if got := v.settingsValue().Tool; got != config.ToolClaudeCode {
		t.Errorf("after right wrap: tool = %q, want %q", got, config.ToolClaudeCode)
	}
	// left cycles backwards.
	v.update(key(t, tea.KeyLeft))
	if got := v.settingsValue().Tool; got != config.ToolOpenCode {
		t.Errorf("after left: tool = %q, want %q", got, config.ToolOpenCode)
	}
}

func TestSettingsEditorEdits(t *testing.T) {
	v := newSettingsView(config.Settings{}, 80, 24)
	v.update(key(t, tea.KeyRunes, 'h', 'i'))
	if got := v.editor.Value(); got != "hi" {
		t.Errorf("editor after typing = %q, want hi", got)
	}
}

func TestSettingsRender(t *testing.T) {
	v := newSettingsView(config.Settings{}, 80, 24)
	v.update(key(t, tea.KeyRunes, 'a', 'b', 'c'))
	out := v.render()
	for _, want := range []string{"Editor:", "claude-code", "opencode", "abc"} {
		if !contains(out, want) {
			t.Errorf("render missing %q", want)
		}
	}
}
