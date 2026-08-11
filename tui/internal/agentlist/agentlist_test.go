package agentlist

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate points $MANIGOT_HOME/$PATH at a fresh, empty temp dir/checkout so
// resolve.Home() can't be influenced by whatever the test host happens to
// have installed, mirroring resolve package's own test helper.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("MANIGOT_HOME", "")
}

// writeAgent writes an agent markdown file with the given frontmatter
// description (or no description: line at all when desc == "").
func writeAgent(t *testing.T, dir, name, desc string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body := "---\nname: " + name + "\n"
	if desc != "" {
		body += "description: " + desc + "\n"
	}
	body += "---\nbody\n"
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDiscoverGlobalOnly(t *testing.T) {
	isolate(t)
	home := t.TempDir()
	writeAgent(t, filepath.Join(home, "agents"), "analyst", "Break a brief into tasks.")
	writeAgent(t, filepath.Join(home, "agents"), "developer", "Implement tasks.")
	t.Setenv("MANIGOT_HOME", home)

	got, err := Discover(t.TempDir())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := []Agent{
		{Name: "analyst", Description: "Break a brief into tasks."},
		{Name: "developer", Description: "Implement tasks."},
	}
	if len(got) != len(want) {
		t.Fatalf("Discover = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Discover[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDiscoverProjectOverride(t *testing.T) {
	isolate(t)
	home := t.TempDir()
	writeAgent(t, filepath.Join(home, "agents"), "developer", "Implement tasks.")
	t.Setenv("MANIGOT_HOME", home)

	project := t.TempDir()
	writeAgent(t, filepath.Join(project, "docs", "agents"), "developer", "Custom project developer.")

	got, err := Discover(project)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].Description != "Custom project developer." {
		t.Errorf("Discover = %+v, want the project override description", got)
	}
}

func TestDiscoverProjectOnlyAddition(t *testing.T) {
	isolate(t)
	home := t.TempDir()
	writeAgent(t, filepath.Join(home, "agents"), "developer", "Implement tasks.")
	t.Setenv("MANIGOT_HOME", home)

	project := t.TempDir()
	writeAgent(t, filepath.Join(project, "docs", "agents"), "custom", "Project-only agent.")

	got, err := Discover(project)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := []Agent{
		{Name: "developer", Description: "Implement tasks."},
		{Name: "custom", Description: "Project-only agent."},
	}
	if len(got) != len(want) {
		t.Fatalf("Discover = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Discover[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDiscoverNoDescriptionFallback(t *testing.T) {
	isolate(t)
	home := t.TempDir()
	writeAgent(t, filepath.Join(home, "agents"), "mystery", "")
	t.Setenv("MANIGOT_HOME", home)

	got, err := Discover(t.TempDir())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].Description != "(no description)" {
		t.Errorf("Discover = %+v, want the no-description fallback", got)
	}
}

func TestDiscoverEmptyProjectRoot(t *testing.T) {
	// projectRoot == "" (e.g. a non-git/uninitialized project fallback) must
	// still return the global agent list, just with no docs/agents/ lookup.
	isolate(t)
	home := t.TempDir()
	writeAgent(t, filepath.Join(home, "agents"), "analyst", "Break a brief into tasks.")
	t.Setenv("MANIGOT_HOME", home)

	got, err := Discover("")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].Name != "analyst" {
		t.Errorf("Discover = %+v, want just the global analyst agent", got)
	}
}

func TestDiscoverNoCheckoutFound(t *testing.T) {
	isolate(t)
	if _, err := Discover(t.TempDir()); err == nil {
		t.Error("Discover with no resolvable manigot checkout should error, got nil")
	}
}

func TestDiscoverNoGlobalAgentsDir(t *testing.T) {
	isolate(t)
	home := t.TempDir() // no agents/ subdir at all
	t.Setenv("MANIGOT_HOME", home)
	if _, err := Discover(t.TempDir()); err == nil {
		t.Error("Discover with no global agents dir should error, got nil")
	}
}
