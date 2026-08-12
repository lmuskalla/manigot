package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	code := runAgents(nil, strings.NewReader(""), &out, &strings.Builder{}, false)
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
	code := runAgents(nil, strings.NewReader(""), &out, &errOut, false)
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
	code := runAgents(nil, strings.NewReader(""), &strings.Builder{}, &errOut, false)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: no agents/ directory found at") {
		t.Errorf("missing no-agents-dir error:\n%s", errOut.String())
	}
}

func TestAgentsSelectWritesChosenAndLaunches(t *testing.T) {
	agentsCheckout(t, map[string]string{"analyst": "a", "reviewer": "r"}, "", nil)
	var out strings.Builder
	code := runAgents([]string{"--profile", "zai"}, strings.NewReader("2\n"), &out, &strings.Builder{}, true)
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
