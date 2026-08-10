package config

import (
	"os"
	"path/filepath"
	"testing"
)

// checkout builds a minimal fake manigot checkout (just the one file
// resolve.looksLikeCheckout keys off) and points $MANIGOT_HOME at it, so
// Dir/Load/Save are exercised without depending on where `go test` happens to
// run from.
func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", dir)
	return dir
}

func TestLoadMissingFileReturnsZeroValue(t *testing.T) {
	checkout(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s != (Settings{}) {
		t.Errorf("Load on missing file = %+v, want zero value", s)
	}
}

func TestLoadUnresolvableHomeReturnsZeroValue(t *testing.T) {
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("PATH", t.TempDir()) // hide any real manigot checkout on $PATH
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s != (Settings{}) {
		t.Errorf("Load with no resolvable home = %+v, want zero value", s)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := checkout(t)
	want := Settings{Editor: "vim", Profile: ProfileZAI}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(dir, "config", "tui-settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file not written at %s: %v", path, err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("Load after Save = %+v, want %+v", got, want)
	}
}

func TestSaveUnresolvableHomeErrors(t *testing.T) {
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("PATH", t.TempDir())
	if err := Save(Settings{Editor: "vim"}); err == nil {
		t.Fatal("expected an error when no checkout can be located")
	}
}

func TestProfileValueDefaultsToClaudePro(t *testing.T) {
	if got := (Settings{}).ProfileValue(); got != ProfileClaudePro {
		t.Errorf("zero-value ProfileValue = %q, want %q", got, ProfileClaudePro)
	}
	if got := (Settings{Profile: ProfileOpenCodeGo}).ProfileValue(); got != ProfileOpenCodeGo {
		t.Errorf("ProfileValue = %q, want %q", got, ProfileOpenCodeGo)
	}
}

// writeSettings writes a tui-settings.json into the fake checkout.
func writeSettings(t *testing.T, data string) {
	t.Helper()
	dir := checkout(t)
	path := filepath.Join(dir, "config", "tui-settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMigratesLegacyToolField(t *testing.T) {
	// claude-code maps to claude-pro.
	writeSettings(t, `{"editor":"vim","tool":"claude-code"}`)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileClaudePro {
		t.Errorf("legacy tool claude-code -> profile %q, want %q", s.Profile, ProfileClaudePro)
	}

	// opencode maps to zai (the opencode subscription manigot configured first).
	writeSettings(t, `{"tool":"opencode"}`)
	s, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileZAI {
		t.Errorf("legacy tool opencode -> profile %q, want %q", s.Profile, ProfileZAI)
	}
}

func TestLoadKeepsExplicitProfile(t *testing.T) {
	// A settings file that already has a profile wins over any legacy tool
	// field, and the legacy field is dropped.
	writeSettings(t, `{"profile":"opencode-go","tool":"opencode"}`)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileOpenCodeGo {
		t.Errorf("profile = %q, want %q", s.Profile, ProfileOpenCodeGo)
	}
	if s.Tool != "" {
		t.Errorf("legacy tool field not cleared, got %q", s.Tool)
	}
}
