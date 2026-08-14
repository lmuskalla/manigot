package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/agentlist"
	"github.com/lmuskalla/manigot/internal/launch"
)

// writeGlobalAgent writes a fake manigot checkout's agents/<name>.md, for
// tests that need agentlist.Discover to succeed against a controlled
// $MANIGOT_HOME.
func writeGlobalAgent(t *testing.T, home, name, desc string) {
	t.Helper()
	dir := filepath.Join(home, "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- agentsPickerView (pure input component) --------------------------------

func testAgents() []agentlist.Agent {
	return []agentlist.Agent{
		{Name: "analyst", Description: "Break a brief into tasks."},
		{Name: "developer", Description: "Implement tasks."},
		{Name: "reviewer", Description: "Review implementations."},
	}
}

func TestAgentsPickerNavigation(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	if v.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", v.cursor)
	}
	v.update(keyMsg("up")) // already at top — must not go negative
	if v.cursor != 0 {
		t.Errorf("up at top moved cursor to %d, want 0", v.cursor)
	}
	v.update(keyMsg("down"))
	if v.cursor != 1 {
		t.Errorf("down = %d, want 1", v.cursor)
	}
	v.update(keyMsg("j"))
	if v.cursor != 2 {
		t.Errorf("j = %d, want 2", v.cursor)
	}
	v.update(keyMsg("down")) // already at bottom — must not overrun
	if v.cursor != 2 {
		t.Errorf("down at bottom moved cursor to %d, want 2", v.cursor)
	}
	v.update(keyMsg("k"))
	if v.cursor != 1 {
		t.Errorf("k = %d, want 1", v.cursor)
	}
	v.update(keyMsg("home"))
	if v.cursor != 0 {
		t.Errorf("home = %d, want 0", v.cursor)
	}
	v.update(keyMsg("end"))
	if v.cursor != 2 {
		t.Errorf("end = %d, want 2", v.cursor)
	}
}

func TestAgentsPickerActions(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	if v.update(keyMsg("esc")) != apCancel {
		t.Error("esc should cancel")
	}
	if v.update(keyMsg("enter")) != apSubmit {
		t.Error("enter should submit")
	}
}

func TestAgentsPickerSelected(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	v.cursor = 1
	got, ok := v.selected()
	if !ok || got.Name != "developer" {
		t.Errorf("selected() = %+v, %v, want developer", got, ok)
	}
}

func TestAgentsPickerRender(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	out := v.render()
	for _, want := range []string{"Launch an agent", "analyst", "Break a brief into tasks.", "developer", "reviewer", "enter launch", "esc cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
}

func TestAgentsPickerRenderCapsLongDescription(t *testing.T) {
	long := strings.Repeat("x", 200)
	v := newAgentsPickerView([]agentlist.Agent{{Name: "sysadmin", Description: long}}, 80, 24)
	out := v.render()
	if !strings.Contains(out, "…") {
		t.Errorf("long description should be truncated with an ellipsis:\n%s", out)
	}
	if strings.Contains(out, strings.Repeat("x", 60)) {
		t.Errorf("description should be capped to AgentDescriptionWidth:\n%s", out)
	}
}

func TestAgentsPickerRenderKeepsShortDescriptionWhole(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	out := v.render()
	for _, want := range []string{"Break a brief into tasks.", "Implement tasks.", "Review implementations."} {
		if !strings.Contains(out, want) {
			t.Errorf("short description %q should render untruncated:\n%s", want, out)
		}
	}
	if strings.Contains(out, "…") {
		t.Errorf("short descriptions should not be truncated:\n%s", out)
	}
}

func TestAgentsPickerRenderCapsToViewWidth(t *testing.T) {
	// At a narrow width the description is capped to the room left on the
	// row — 2-col marker + 16-col name + 2-col gap = 20 chars of prefix — not
	// just to AgentDescriptionWidth, so a row never spills past the edge.
	long := strings.Repeat("x", 200)
	v := newAgentsPickerView([]agentlist.Agent{{Name: "a", Description: long}}, 40, 24)
	out := v.render()
	if strings.Contains(out, strings.Repeat("x", 21)) {
		t.Errorf("description should be capped to the remaining row width:\n%s", out)
	}
}

// --- App wiring: "a" from the list opens the picker --------------------------

func TestUpdateListAKeyOpensPicker(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	writeGlobalAgent(t, home, "developer", "Implement tasks.")
	t.Setenv("MANIGOT_HOME", home)

	a := NewApp(dir, nil)
	a.width, a.height = 80, 24

	model, _ := a.updateList(keyMsg("a"))
	got := model.(*App)
	if got.state != stateAgents {
		t.Fatalf("state = %v, want stateAgents", got.state)
	}
	if got.agentsPicker == nil {
		t.Fatal("agentsPicker not set")
	}
	if len(got.agentsPicker.agents) != 1 || got.agentsPicker.agents[0].Name != "developer" {
		t.Errorf("agentsPicker.agents = %+v, want just the discovered developer agent", got.agentsPicker.agents)
	}
}

func TestUpdateListAKeyDiscoveryFailureShowsStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", t.TempDir()) // isolate from the real checkout's binaries too
	t.Setenv("MANIGOT_HOME", "")

	a := NewApp(dir, nil)
	a.width, a.height = 80, 24

	model, _ := a.updateList(keyMsg("a"))
	got := model.(*App)
	if got.state != stateList {
		t.Errorf("state = %v, want stateList (discovery failure must not open the picker)", got.state)
	}
	if got.status == "" {
		t.Error("status should report the discovery failure")
	}
}

func TestUpdateAgentsPickerCancelReturnsToList(t *testing.T) {
	dir := t.TempDir()
	a := NewApp(dir, nil)
	a.width, a.height = 80, 24
	a.agentsPicker = newAgentsPickerView(testAgents(), 80, 24)
	a.state = stateAgents

	model, _ := a.updateAgentsPicker(keyMsg("esc"))
	got := model.(*App)
	if got.state != stateList {
		t.Errorf("state = %v, want stateList after esc", got.state)
	}
	if got.agentsPicker != nil {
		t.Error("agentsPicker should be cleared after esc")
	}
}

// TestUpdateAgentsPickerSubmitReportsResolutionFailure verifies a failed
// resolution (manigot launcher not installed) surfaces as a footer status
// rather than panicking or silently doing nothing, mirroring
// TestJdiKeyReportsResolutionFailure's coverage of the same failure mode for
// the "j" key.
func TestUpdateAgentsPickerSubmitReportsResolutionFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIGOT_HOME", "")

	old := launch.ExeOverride
	t.Cleanup(func() { launch.ExeOverride = old })
	launch.ExeOverride = func() (string, error) { return "", errors.New("mg: not found") }

	a := NewApp(dir, nil)
	a.width, a.height = 80, 24
	a.agentsPicker = newAgentsPickerView(testAgents(), 80, 24)
	a.state = stateAgents

	model, _ := a.updateAgentsPicker(keyMsg("enter"))
	got := model.(*App)
	if got.state != stateList {
		t.Errorf("state = %v, want stateList after submit", got.state)
	}
	if got.agentsPicker != nil {
		t.Error("agentsPicker should be cleared after submit")
	}
	if !strings.Contains(got.status, "not found") {
		t.Errorf("status = %q, want it to explain the manigot launcher could not be resolved", got.status)
	}
}

func TestListFooterMentionsAgentKey(t *testing.T) {
	a := NewApp(t.TempDir(), nil)
	a.width, a.height = 80, 24
	if !strings.Contains(a.footer(), "a agent") {
		t.Errorf("footer missing the agent-picker key hint:\n%s", a.footer())
	}
}
