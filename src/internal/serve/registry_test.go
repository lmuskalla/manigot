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

// namedProjectsJSON renders a {"projects": [{"name":..., "path":...}, ...]}
// registry config body from name/path pairs.
func namedProjectsJSON(pairs ...[2]string) string {
	var b strings.Builder
	b.WriteString(`{"projects": [`)
	for i, p := range pairs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"name": "` + p[0] + `", "path": "` + p[1] + `"}`)
	}
	b.WriteString(`]}`)
	return b.String()
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

// TestLoadRegistryParsesAndValidatesRoots: a valid config of named entries
// yields the registered roots in order, each cleaned to an absolute path.
func TestLoadRegistryParsesAndValidatesRoots(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"proj-a", a}, [2]string{"proj-b", b}))

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	got := reg.Projects()
	if len(got) != 2 {
		t.Fatalf("Projects() = %v, want 2 roots", got)
	}
	if got[0] != filepath.Clean(a) || got[1] != filepath.Clean(b) {
		t.Errorf("Projects() = %v, want [%s %s]", got, a, b)
	}

	entries := reg.Entries()
	if len(entries) != 2 || entries[0].Name != "proj-a" || entries[1].Name != "proj-b" {
		t.Errorf("Entries() = %+v, want proj-a then proj-b", entries)
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
		writeRegistry(t, path, namedProjectsJSON([2]string{"proj", bad}))
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

// TestLoadRegistryRejectsFlatStringForm pins the "no backwards compatibility"
// requirement: the old flat-string schema ("projects": ["/path", ...]) fails
// to parse under the object schema.
func TestLoadRegistryRejectsFlatStringForm(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, `{"projects": ["`+root+`"]}`)
	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("LoadRegistry on the flat-string form = nil error, want an error")
	}
}

// TestLoadRegistryRejectsEmptyName: an entry with no name is refused.
func TestLoadRegistryRejectsEmptyName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"", root}))
	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("LoadRegistry with an empty name = nil error, want an error")
	}
}

// TestLoadRegistryRejectsNameWithSeparators: a name containing a forward or
// backward slash is refused — a name must be a single URL path segment.
func TestLoadRegistryRejectsNameWithSeparators(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"solyto/api", `solyto\api`} {
		path := filepath.Join(t.TempDir(), "serve.json")
		writeRegistry(t, path, namedProjectsJSON([2]string{bad, root}))
		if _, err := LoadRegistry(path); err == nil {
			t.Errorf("LoadRegistry with name %q = nil error, want an error", bad)
		}
	}
}

// TestLoadRegistryRejectsDotNames: names "." and ".." are refused (traversal
// segments, even though they'd otherwise pass the charset check).
func TestLoadRegistryRejectsDotNames(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{".", ".."} {
		path := filepath.Join(t.TempDir(), "serve.json")
		writeRegistry(t, path, namedProjectsJSON([2]string{bad, root}))
		if _, err := LoadRegistry(path); err == nil {
			t.Errorf("LoadRegistry with name %q = nil error, want an error", bad)
		}
	}
}

// TestLoadRegistryRejectsNonURLSafeName: a name outside the conservative
// [A-Za-z0-9._-] charset is refused.
func TestLoadRegistryRejectsNonURLSafeName(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"solyto api", "solyto@api", "solyto:api", "sölyto"} {
		path := filepath.Join(t.TempDir(), "serve.json")
		writeRegistry(t, path, namedProjectsJSON([2]string{bad, root}))
		if _, err := LoadRegistry(path); err == nil {
			t.Errorf("LoadRegistry with name %q = nil error, want an error", bad)
		}
	}
}

// TestLoadRegistryRejectsDuplicateNames: two entries with the same name are
// refused, regardless of their paths.
func TestLoadRegistryRejectsDuplicateNames(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"dup", a}, [2]string{"dup", b}))
	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("LoadRegistry with duplicate names = nil error, want an error")
	}
}

// TestLoadRegistryRejectsDuplicatePaths: two entries with different names
// registering the same path are refused — the pinned duplicate-path
// behavior (see LoadRegistry's doc comment).
func TestLoadRegistryRejectsDuplicatePaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"name-a", root}, [2]string{"name-b", root}))
	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("LoadRegistry with duplicate paths = nil error, want an error")
	}
}

// TestRegistryProjectNameOnlyResolution: Project resolves strictly by
// configured name — an entry registered under a name different from its
// directory's base name resolves by the configured name and NOT by the base
// name, and an unknown segment is not-found.
func TestRegistryProjectNameOnlyResolution(t *testing.T) {
	root := t.TempDir() // base name is whatever t.TempDir() picked
	reg := &Registry{entries: []Entry{{Name: "custom-name", Path: filepath.Clean(root)}}}

	if root2, ok := reg.Project("custom-name"); !ok || root2 != filepath.Clean(root) {
		t.Errorf("Project(configured name) = %q, %v; want %q, true", root2, ok, filepath.Clean(root))
	}
	if _, ok := reg.Project(filepath.Base(root)); ok {
		t.Errorf("Project(base name) resolved, want not-found — resolution is by configured name only")
	}
	if _, ok := reg.Project(filepath.Clean(root)); ok {
		t.Errorf("Project(exact path) resolved, want not-found — resolution is by configured name only")
	}
	if _, ok := reg.Project("no-such-project"); ok {
		t.Errorf("Project(unknown) resolved, want not-found")
	}
}

// TestLoadRegistryAcceptsRootWithoutDocs: a registered root without docs/ is
// accepted (it lists no jobs rather than failing startup), and
// WarnMissingDocs flags it.
func TestLoadRegistryAcceptsRootWithoutDocs(t *testing.T) {
	root := t.TempDir() // no docs/ inside
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"docsless", root}))

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
	reg2 := &Registry{entries: []Entry{{Name: "withdocs", Path: filepath.Clean(withDocs)}}}
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
