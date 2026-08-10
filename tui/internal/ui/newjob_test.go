package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(t testing.TB, kt tea.KeyType, runes ...rune) tea.KeyMsg {
	t.Helper()
	return tea.KeyMsg{Type: kt, Runes: runes}
}

func TestNewJobInitialFocus(t *testing.T) {
	v := newNewJobView(80, 24)
	if v.focus != 0 {
		t.Errorf("initial focus = %d, want 0 (title)", v.focus)
	}
	if v.typeValue() != "feature" {
		t.Errorf("initial type = %q, want feature", v.typeValue())
	}
}

func TestNewJobActions(t *testing.T) {
	v := newNewJobView(80, 24)
	if v.update(key(t, tea.KeyEscape)) != njCancel {
		t.Error("esc should cancel")
	}
	if v.update(key(t, tea.KeyEnter)) != njSubmit {
		t.Error("enter should submit")
	}
}

func TestNewJobTabTogglesFocus(t *testing.T) {
	v := newNewJobView(80, 24)
	if v.focus != 0 {
		t.Fatal("expected initial focus title")
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != 1 {
		t.Errorf("after tab, focus = %d, want 1 (type)", v.focus)
	}
	v.update(key(t, tea.KeyTab))
	if v.focus != 0 {
		t.Errorf("after second tab, focus = %d, want 0 (title)", v.focus)
	}
}

func TestNewJobShiftTabTogglesFocusBackward(t *testing.T) {
	v := newNewJobView(80, 24)
	if v.focus != 0 {
		t.Fatal("expected initial focus title")
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != 1 {
		t.Errorf("after shift+tab, focus = %d, want 1 (type)", v.focus)
	}
	v.update(keyMsg("shift+tab"))
	if v.focus != 0 {
		t.Errorf("after second shift+tab, focus = %d, want 0 (title)", v.focus)
	}
}

func TestNewJobTypeCycleOnlyWhenTypeFocused(t *testing.T) {
	v := newNewJobView(80, 24)
	// focus on title: left/right must not change the type.
	before := v.typ
	v.update(key(t, tea.KeyRight))
	if v.typ != before {
		t.Errorf("left/right while title focused changed type: %d -> %d", before, v.typ)
	}

	// focus on type: right cycles feature -> fix -> chore -> feature.
	v.update(key(t, tea.KeyTab)) // now focus == type
	v.update(key(t, tea.KeyRight))
	if v.typeValue() != "fix" {
		t.Errorf("after right: type = %q, want fix", v.typeValue())
	}
	v.update(key(t, tea.KeyRight))
	if v.typeValue() != "chore" {
		t.Errorf("after right: type = %q, want chore", v.typeValue())
	}
	v.update(key(t, tea.KeyRight))
	if v.typeValue() != "feature" {
		t.Errorf("after right wrap: type = %q, want feature", v.typeValue())
	}
	// left cycles backwards.
	v.update(key(t, tea.KeyLeft))
	if v.typeValue() != "chore" {
		t.Errorf("after left: type = %q, want chore", v.typeValue())
	}
}

func TestNewJobTitleEdits(t *testing.T) {
	v := newNewJobView(80, 24)
	v.update(key(t, tea.KeyRunes, 'h', 'i'))
	if got := v.title.Value(); got != "hi" {
		t.Errorf("title after typing = %q, want hi", got)
	}
}

func TestNewJobRender(t *testing.T) {
	v := newNewJobView(80, 24)
	v.update(key(t, tea.KeyRunes, 'a', 'b', 'c'))
	out := v.render()
	// Should mention the three job types and the title label.
	for _, want := range []string{"Title:", "feature", "fix", "chore", "abc"} {
		if !contains(out, want) {
			t.Errorf("render missing %q", want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
