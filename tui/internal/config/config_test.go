package config

import (
	"os"
	"path/filepath"
	"testing"
)

// checkout builds a minimal fake safecode checkout (just the one file
// resolve.looksLikeCheckout keys off) and points $SAFECODE_HOME at it, so
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
	t.Setenv("SAFECODE_HOME", dir)
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
	t.Setenv("SAFECODE_HOME", "")
	t.Setenv("PATH", t.TempDir()) // hide any real safecode checkout on $PATH
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
	want := Settings{Editor: "vim", Tool: ToolOpenCode}
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
	t.Setenv("SAFECODE_HOME", "")
	t.Setenv("PATH", t.TempDir())
	if err := Save(Settings{Editor: "vim"}); err == nil {
		t.Fatal("expected an error when no checkout can be located")
	}
}

func TestToolValueDefaultsToClaudeCode(t *testing.T) {
	if got := (Settings{}).ToolValue(); got != ToolClaudeCode {
		t.Errorf("zero-value ToolValue = %q, want %q", got, ToolClaudeCode)
	}
	if got := (Settings{Tool: ToolOpenCode}).ToolValue(); got != ToolOpenCode {
		t.Errorf("ToolValue = %q, want %q", got, ToolOpenCode)
	}
}
