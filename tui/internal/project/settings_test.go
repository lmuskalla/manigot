package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsZeroValue(t *testing.T) {
	root := t.TempDir()
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s != (Settings{}) {
		t.Errorf("Load on missing file = %+v, want zero value", s)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	root := t.TempDir()
	want := Settings{BaseBranch: "develop"}
	if err := Save(root, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(root, "docs", "manigot.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file not written at %s: %v", path, err)
	}

	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("Load after Save = %+v, want %+v", got, want)
	}
}

func TestBaseBranchValueDefaultsToMain(t *testing.T) {
	if got := (Settings{}).BaseBranchValue(); got != "main" {
		t.Errorf("zero-value BaseBranchValue = %q, want %q", got, "main")
	}
	if got := (Settings{BaseBranch: "trunk"}).BaseBranchValue(); got != "trunk" {
		t.Errorf("BaseBranchValue = %q, want %q", got, "trunk")
	}
}
