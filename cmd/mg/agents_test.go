package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/ui"
)

// agentsCheckout builds a fake manigot checkout with an agents/ dir (plus an
// optional docs/ project at projRoot), so the listing is hermetic.
func agentsCheckout(t *testing.T, globalAgents map[string]string, projRoot string, projAgents map[string]string) {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, desc := range globalAgents {
		if err := os.WriteFile(filepath.Join(home, "agents", name+".md"), []byte("---\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if projRoot != "" {
		if err := os.MkdirAll(filepath.Join(projRoot, "docs", "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		for name, desc := range projAgents {
			if err := os.WriteFile(filepath.Join(projRoot, "docs", "agents", name+".md"), []byte("---\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("MANIGOT_HOME", home)
}

func TestAgentsListsWithTags(t *testing.T) {
	proj := t.TempDir()
	agentsCheckout(t,
		map[string]string{"analyst": "breaks work down", "reviewer": "checks work"},
		proj,
		map[string]string{"analyst": "project version", "custom": "project only"},
	)
	t.Chdir(proj)
	var out strings.Builder
	code := runAgents(nil, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	// Non-TTY: after listing it must refuse to pick, like agents.sh.
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (non-TTY refusal)", code)
	}
	got := out.String()
	for _, want := range []string{
		"Available agents:",
		"1) analyst", "(project override)",
		"2) reviewer", // global, no tag
		"3) custom", "(project)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("listing missing %q:\n%s", want, got)
		}
	}
}

func TestAgentsNonTTYRefusal(t *testing.T) {
	agentsCheckout(t, map[string]string{"analyst": "x"}, "", nil)
	var out, errOut strings.Builder
	code := runAgents(nil, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: mg agents needs an interactive terminal to select an agent.") {
		t.Errorf("missing non-TTY refusal:\n%s", errOut.String())
	}
}

func TestAgentsNoGlobalDir(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", home)
	var errOut strings.Builder
	code := runAgents(nil, strings.NewReader(""), &strings.Builder{}, &errOut, false, pickerStub(t))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: no agents/ directory found at") {
		t.Errorf("missing no-agents-dir error:\n%s", errOut.String())
	}
}

// TestAgentsSelectWritesChosenAndLaunches covers the TTY submit path with an
// injected picker (the seam — no real Bubble Tea program): the chosen agent
// is passed through to the launch line and the re-exec of os.Executable().
func TestAgentsSelectWritesChosenAndLaunches(t *testing.T) {
	agentsCheckout(t, map[string]string{"analyst": "a", "reviewer": "r"}, "", nil)
	var out strings.Builder
	code := runAgents([]string{"--profile", "zai"}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("reviewer", true))
	// The launch re-execs os.Executable() — the go test binary — with
	// --agent reviewer --profile zai, which it rejects as unknown flags and
	// exits non-zero. What matters here is the menu output up to the launch.
	if code == 0 {
		t.Fatalf("unexpected success; the re-exec should not accept the flags")
	}
	if !strings.Contains(out.String(), "→ Starting a session in @reviewer...") {
		t.Errorf("missing launch line:\n%s", out.String())
	}
}

// TestAgentsPickerGetsAgentRows pins the picker wiring: on a TTY the picker
// is fed a title plus one pre-rendered row per agent (name/description +
// source tag, search key name + description), and a cancelled picker exits 0
// quietly.
func TestAgentsPickerGetsAgentRows(t *testing.T) {
	proj := t.TempDir()
	agentsCheckout(t,
		map[string]string{"analyst": "breaks work down", "reviewer": "checks work"},
		proj,
		map[string]string{"analyst": "project version", "custom": "project only"},
	)
	t.Chdir(proj)

	var gotTitle string
	var gotRows []ui.PickerRow
	pick := func(title string, rows []ui.PickerRow) (string, bool, error) {
		gotTitle, gotRows = title, rows
		return "", false, nil // cancelled
	}
	var out strings.Builder
	code := runAgents(nil, strings.NewReader(""), &out, &strings.Builder{}, true, pick)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (cancel exits quietly)", code)
	}
	if gotTitle != "Select an agent" {
		t.Errorf("picker title = %q, want %q", gotTitle, "Select an agent")
	}
	if len(gotRows) != 3 {
		t.Fatalf("picker rows = %d, want 3", len(gotRows))
	}
	if gotRows[0].ID != "analyst" || gotRows[0].SearchKey != "analyst project version" {
		t.Errorf("row 0 = %+v, want analyst with name+description search key", gotRows[0])
	}
	if !strings.Contains(gotRows[0].Label, "analyst") || !strings.Contains(gotRows[0].Label, "project version") ||
		!strings.Contains(gotRows[0].Label, "(project override)") {
		t.Errorf("row 0 label missing name/description/tag: %q", gotRows[0].Label)
	}
	if !strings.Contains(gotRows[1].Label, "reviewer") || strings.Contains(gotRows[1].Label, "(project") {
		t.Errorf("row 1 (global) label should carry no source tag: %q", gotRows[1].Label)
	}
	if !strings.Contains(gotRows[2].Label, "custom") || !strings.Contains(gotRows[2].Label, "(project)") {
		t.Errorf("row 2 label missing project-only tag: %q", gotRows[2].Label)
	}
	// Short descriptions pass through whole — no ellipsis anywhere.
	for i, row := range gotRows {
		if strings.Contains(row.Label, "…") {
			t.Errorf("row %d label truncated a short description: %q", i, row.Label)
		}
	}
}

// TestAgentsListingCapsDescription pins the plain non-TTY listing's
// description capping: a long description renders truncated with an ellipsis,
// so each row stays one readable line with the name column intact.
func TestAgentsListingCapsDescription(t *testing.T) {
	long := strings.Repeat("z", 150)
	agentsCheckout(t, map[string]string{"analyst": long}, "", nil)
	var out strings.Builder
	code := runAgents(nil, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (non-TTY refusal)", code)
	}
	got := out.String()
	if !strings.Contains(got, "1) analyst") {
		t.Errorf("listing missing the name column:\n%s", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("capped listing should show an ellipsis:\n%s", got)
	}
	if strings.Contains(got, strings.Repeat("z", 60)) {
		t.Errorf("listing description not capped to AgentDescriptionWidth:\n%s", got)
	}
}

// TestAgentsPickerRowsCapDescription pins the TTY picker row formatting: the
// label carries name + source tag + truncated description (ellipsis on long
// ones), and the SearchKey keeps the full description so type-to-filter still
// matches on it.
func TestAgentsPickerRowsCapDescription(t *testing.T) {
	long := strings.Repeat("y", 200)
	proj := t.TempDir()
	agentsCheckout(t, map[string]string{"analyst": "global analyst"}, proj, map[string]string{"analyst": long})
	t.Chdir(proj)

	var gotRows []ui.PickerRow
	pick := func(title string, rows []ui.PickerRow) (string, bool, error) {
		gotRows = rows
		return "", false, nil // cancelled
	}
	var out strings.Builder
	code := runAgents(nil, strings.NewReader(""), &out, &strings.Builder{}, true, pick)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (cancel exits quietly)", code)
	}
	if len(gotRows) != 1 {
		t.Fatalf("picker rows = %d, want 1", len(gotRows))
	}
	row := gotRows[0]
	if row.SearchKey != "analyst "+long {
		t.Errorf("SearchKey = %q..., want the full description preserved", row.SearchKey[:30])
	}
	if !strings.Contains(row.Label, "analyst") || !strings.Contains(row.Label, "(project override)") {
		t.Errorf("label missing name/source tag: %q", row.Label)
	}
	if !strings.Contains(row.Label, "…") {
		t.Errorf("label should end the long description with an ellipsis: %q", row.Label)
	}
	if strings.Contains(row.Label, strings.Repeat("y", 60)) {
		t.Errorf("label description not capped to AgentDescriptionWidth: %q", row.Label)
	}
}
