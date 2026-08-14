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
	for _, want := range []string{"Launch an agent", "analyst", "Break a brief into tasks.", "developer", "reviewer", "enter launch", "type to filter", "esc/q cancel"} {
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

// --- type-to-filter ---------------------------------------------------------

// TestAgentsPickerFilterNarrows covers the core filtering behaviour: typing
// against an agent's name/description narrows the list, and the cursor is
// clamped into the filtered list — mirroring TestPickerFilterNarrows.
func TestAgentsPickerFilterNarrows(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	v.cursor = 2 // start at the bottom so clamping is observable
	if got := v.update(keyMsg("dev")); got != apNone {
		t.Fatalf("typing should not return an action, got %v", got)
	}
	if v.filter != "dev" {
		t.Fatalf("filter = %q, want dev", v.filter)
	}
	if got := v.filtered(); len(got) != 1 || got[0].Name != "developer" {
		t.Errorf("filtered = %+v, want just developer", got)
	}
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped into the filtered list)", v.cursor)
	}
	// The filter is case-insensitive.
	v.update(keyMsg("Z")) // "devZ" — no match
	if got := v.filtered(); len(got) != 0 {
		t.Errorf("filter devZ = %+v, want no rows", got)
	}
}

// TestAgentsPickerFilterEscClearsBeforeCancel pins the two-stage esc: a first
// esc clears the filter (restoring the full list) and only a second one
// cancels — mirroring TestPickerFilterEscClearsBeforeCancel.
func TestAgentsPickerFilterEscClearsBeforeCancel(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	v.update(keyMsg("dev"))
	v.cursor = 0
	if got := v.update(keyMsg("esc")); got != apNone {
		t.Fatalf("esc with an active filter should not cancel, got %v", got)
	}
	if v.filter != "" || len(v.agents) != 3 || len(v.filtered()) != 3 {
		t.Errorf("after esc: filter = %q, filtered = %d, want cleared full list", v.filter, len(v.filtered()))
	}
	if got := v.update(keyMsg("esc")); got != apCancel {
		t.Fatalf("second esc (no filter) should cancel, got %v", got)
	}
}

// TestAgentsPickerFilterBackspace covers editing the filter: backspace deletes
// the last rune, widening the match set again — mirroring
// TestPickerFilterBackspace.
func TestAgentsPickerFilterBackspace(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	v.update(keyMsg("deve")) // matches developer only
	if got := v.filtered(); len(got) != 1 || got[0].Name != "developer" {
		t.Fatalf("filter deve = %+v, want just developer", got)
	}
	v.update(keyMsg("backspace")) // "dev" — still just developer
	if v.filter != "dev" {
		t.Errorf("filter = %q, want dev", v.filter)
	}
	if got := v.filtered(); len(got) != 1 || got[0].Name != "developer" {
		t.Errorf("filter dev = %+v, want just developer", got)
	}
	v.update(keyMsg("backspace")) // "de" — still just developer
	if v.filter != "de" {
		t.Errorf("filter = %q, want de", v.filter)
	}
	if len(v.filtered()) != 1 {
		t.Errorf("filtered = %d rows, want 1 (de only matches developer)", len(v.filtered()))
	}
	v.update(keyMsg("backspace")) // "d" — still just developer
	if v.filter != "d" {
		t.Errorf("filter = %q, want d", v.filter)
	}
	if len(v.filtered()) != 1 {
		t.Errorf("filtered = %d rows, want 1 (d only matches developer)", len(v.filtered()))
	}
	v.update(keyMsg("backspace")) // "" — full list back
	if v.filter != "" {
		t.Errorf("filter = %q, want empty", v.filter)
	}
	if len(v.filtered()) != 3 {
		t.Errorf("filtered = %d rows, want 3 (full list)", len(v.filtered()))
	}
	v.update(keyMsg("backspace")) // empty filter — backspace must be a no-op
	if v.filter != "" {
		t.Errorf("filter = %q after backspace on empty filter, want still empty", v.filter)
	}
}

// TestAgentsPickerFilterNavInterplay pins the navigation-vs-input resolution:
// with no filter, j/k navigate and q cancels; once a filter is active the
// same keys extend it, and navigation moves to the arrows. Clearing the
// filter (esc) flips the keys back — mirroring TestPickerFilterNavInterplay.
func TestAgentsPickerFilterNavInterplay(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	v.update(keyMsg("j")) // no filter yet — navigates
	if v.cursor != 1 {
		t.Errorf("j with no filter should navigate, cursor = %d, want 1", v.cursor)
	}
	if got := v.update(keyMsg("q")); got != apCancel {
		t.Errorf("q with no filter should cancel, got %v", got)
	}

	// With a filter active the same keys type.
	v = newAgentsPickerView(testAgents(), 80, 24)
	v.update(keyMsg("B")) // start the filter ("Break" — analyst only)
	if got := v.filtered(); len(got) != 1 || got[0].Name != "analyst" {
		t.Fatalf("filter B = %+v, want just analyst", got)
	}
	v.update(keyMsg("j")) // extends the filter, does not navigate
	if v.filter != "Bj" {
		t.Errorf("filter = %q, want Bj (j types while filtering)", v.filter)
	}
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (j must not navigate while filtering)", v.cursor)
	}
	v.update(keyMsg("q")) // q types while filtering, does not cancel
	if v.filter != "Bjq" {
		t.Errorf("filter = %q, want Bjq (q types while filtering)", v.filter)
	}
	// "Bjq" matches nothing, so arrows have no row to move to.
	v.update(keyMsg("down"))
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (no matches to navigate)", v.cursor)
	}

	// esc flips the keys back to navigation; a fresh filter matching two
	// rows ("v" — developer and reviewer) lets the arrows move within it.
	v.update(keyMsg("esc"))
	if v.filter != "" {
		t.Fatalf("filter = %q, want empty after esc", v.filter)
	}
	v.update(keyMsg("down"))
	if v.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (keys back after clearing)", v.cursor)
	}
	v.update(keyMsg("v"))
	if len(v.filtered()) != 2 {
		t.Fatalf("filter v = %+v, want two rows", v.filtered())
	}
	v.update(keyMsg("down"))
	if v.cursor != 1 {
		t.Errorf("down while filtering = %d, want 1 (arrows navigate the filtered list)", v.cursor)
	}
}

// TestAgentsPickerFilterSubmit verifies enter reports the filtered row's
// agent, and that a filter matching nothing cannot be submitted — mirroring
// TestPickerFilterSubmit.
func TestAgentsPickerFilterSubmit(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	v.update(keyMsg("w")) // matches reviewer only ("Review")
	if got := v.filtered(); len(got) != 1 || got[0].Name != "reviewer" {
		t.Fatalf("filter w = %+v, want just reviewer", got)
	}
	if got := v.update(keyMsg("enter")); got != apSubmit {
		t.Fatalf("enter should submit, got %v", got)
	}
	if ag, ok := v.selected(); !ok || ag.Name != "reviewer" {
		t.Errorf("selected() = %+v, %v, want reviewer", ag, ok)
	}

	v = newAgentsPickerView(testAgents(), 80, 24)
	v.update(keyMsg("z")) // no agent contains z
	if len(v.filtered()) != 0 {
		t.Fatalf("filter z = %+v, want no rows", v.filtered())
	}
	if got := v.update(keyMsg("enter")); got != apNone {
		t.Errorf("enter with an empty filtered list should be a no-op, got %v", got)
	}
}

// TestAgentsPickerFilterRender pins the filtered render: the filter line and
// the changed footer appear, and a matching-nothing filter renders "no
// matches" — mirroring TestPickerFilterRender.
func TestAgentsPickerFilterRender(t *testing.T) {
	v := newAgentsPickerView(testAgents(), 80, 24)
	v.update(keyMsg("impl"))
	out := v.render()
	for _, want := range []string{"filter: impl", "developer", "Implement tasks.", "esc clear filter"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "analyst") {
		t.Errorf("filtered-out row must not render:\n%s", out)
	}

	v.update(keyMsg("z")) // "implz" — no matches
	out = v.render()
	if !strings.Contains(out, "no matches") {
		t.Errorf("missing no-matches hint:\n%s", out)
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
