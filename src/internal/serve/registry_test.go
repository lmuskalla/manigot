package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRegistry writes a registry config file at path with the given
// projects JSON.
func writeRegistry(t *testing.T, path, projectsJSON string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(projectsJSON), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadRegistryMissingFileIsEmptyRegistry pins the degrade: a registry
// config that does not exist yet (a fresh checkout) is an empty registry, not
// an error — the daemon starts with nothing registered.
func TestLoadRegistryMissingFileIsEmptyRegistry(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("LoadRegistry on missing file: %v", err)
	}
	if got := reg.Projects(); len(got) != 0 {
		t.Errorf("Projects() = %v, want empty registry", got)
	}
	if _, ok := reg.Project("anything"); ok {
		t.Errorf("Project on empty registry resolved, want not-found")
	}
}

// TestLoadRegistryParsesAndValidatesRoots: a valid config yields the
// registered roots in order, each cleaned to an absolute path, with
// duplicates collapsed.
func TestLoadRegistryParsesAndValidatesRoots(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, `{"projects": ["`+a+`", "`+b+`", "`+a+`"]}`)

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	got := reg.Projects()
	if len(got) != 2 {
		t.Fatalf("Projects() = %v, want 2 roots (duplicate collapsed)", got)
	}
	if got[0] != filepath.Clean(a) || got[1] != filepath.Clean(b) {
		t.Errorf("Projects() = %v, want [%s %s]", got, a, b)
	}
}

// TestLoadRegistryRejectsNonDirectoryEntries: an entry that exists but is not
// a directory (a file), and an entry that does not exist at all, are both
// errors — a broken registry must never silently serve a subset.
func TestLoadRegistryRejectsNonDirectoryEntries(t *testing.T) {
	file := filepath.Join(t.TempDir(), "some-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	for _, bad := range []string{file, missing} {
		path := filepath.Join(t.TempDir(), "serve.json")
		writeRegistry(t, path, `{"projects": ["`+bad+`"]}`)
		if _, err := LoadRegistry(path); err == nil {
			t.Errorf("LoadRegistry with entry %q = nil error, want an error", bad)
		}
	}
}

// TestLoadRegistryRejectsUnparseableFile: a present-but-garbage config is an
// error (distinct from the missing-file empty-registry degrade).
func TestLoadRegistryRejectsUnparseableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, `{not json`)
	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("LoadRegistry on garbage = nil error, want an error")
	}
}

// TestRegistryProjectLookupExactPathThenBaseName: Project resolves an exact
// registered path first, then a unique base name — and never invents a root.
func TestRegistryProjectLookupExactPathThenBaseName(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	reg := &Registry{projects: []string{filepath.Clean(a), filepath.Clean(b)}}

	// Exact path match.
	if root, ok := reg.Project(filepath.Clean(a)); !ok || root != filepath.Clean(a) {
		t.Errorf("Project(exact path) = %q, %v; want %q, true", root, ok, filepath.Clean(a))
	}
	// Base name match.
	if root, ok := reg.Project(filepath.Base(b)); !ok || root != filepath.Clean(b) {
		t.Errorf("Project(base name) = %q, %v; want %q, true", root, ok, filepath.Clean(b))
	}
	// Unknown segment: not-found.
	if _, ok := reg.Project("no-such-project"); ok {
		t.Errorf("Project(unknown) resolved, want not-found")
	}
}

// TestRegistryProjectAmbiguousBaseName: two roots sharing a base name resolve
// to not-found rather than guessing.
func TestRegistryProjectAmbiguousBaseName(t *testing.T) {
	parent := t.TempDir()
	a := filepath.Join(parent, "same")
	b := filepath.Join(parent, "other", "same")
	for _, d := range []string{a, filepath.Dir(b)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reg := &Registry{projects: []string{filepath.Clean(a), filepath.Clean(b)}}
	if _, ok := reg.Project("same"); ok {
		t.Errorf("Project with ambiguous base name resolved, want not-found")
	}
}

// TestLoadRegistryAcceptsRootWithoutDocs: a registered root without docs/ is
// accepted (it lists no jobs rather than failing startup), and
// WarnMissingDocs flags it.
func TestLoadRegistryAcceptsRootWithoutDocs(t *testing.T) {
	root := t.TempDir() // no docs/ inside
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, `{"projects": ["`+root+`"]}`)

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry with docs-less root: %v", err)
	}
	if len(reg.Projects()) != 1 {
		t.Fatalf("Projects() = %v, want the docs-less root registered", reg.Projects())
	}

	var warnings strings.Builder
	reg.WarnMissingDocs(&warnings)
	if !strings.Contains(warnings.String(), "no docs/ directory") {
		t.Errorf("WarnMissingDocs = %q, want a no-docs warning", warnings.String())
	}

	// A root WITH docs/ produces no warning.
	withDocs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withDocs, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg2 := &Registry{projects: []string{filepath.Clean(withDocs)}}
	var w2 strings.Builder
	reg2.WarnMissingDocs(&w2)
	if w2.Len() != 0 {
		t.Errorf("WarnMissingDocs for a root with docs/ = %q, want empty", w2.String())
	}
}

// TestDefaultRegistryPathPinsConfigLocation: the default registry path is
// <checkout>/config/serve.json, derived via config.Dir like every other
// checkout data file.
func TestDefaultRegistryPathPinsConfigLocation(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", dir)

	want := filepath.Join(dir, "config", RegistryFileName)
	if got := DefaultRegistryPath(); got != want {
		t.Errorf("DefaultRegistryPath() = %q, want %q", got, want)
	}
}
