package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAddRegistryEntryCreatesFile: adding to a registry that does not exist
// yet (a fresh checkout) creates the file with the one entry, in the
// daemon-readable shape.
func TestAddRegistryEntryCreatesFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "config", "serve.json")

	entry, err := AddRegistryEntry(path, "proj", root)
	if err != nil {
		t.Fatalf("AddRegistryEntry on missing file: %v", err)
	}
	if entry.Name != "proj" || entry.Path != filepath.Clean(root) {
		t.Errorf("returned entry = %+v, want {proj %s}", entry, filepath.Clean(root))
	}

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry after add: %v", err)
	}
	if got := reg.Entries(); len(got) != 1 || got[0].Name != "proj" || got[0].Path != filepath.Clean(root) {
		t.Errorf("Entries() = %+v, want the added entry", got)
	}
}

// TestAddRegistryEntryAppendsInOrder: a second add keeps the first entry and
// the file order (append, never prepend or reorder).
func TestAddRegistryEntryAppendsInOrder(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"first", a}))

	if _, err := AddRegistryEntry(path, "second", b); err != nil {
		t.Fatalf("AddRegistryEntry: %v", err)
	}
	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry after add: %v", err)
	}
	names := []string{reg.Entries()[0].Name, reg.Entries()[1].Name}
	if names[0] != "first" || names[1] != "second" {
		t.Errorf("entry order after add = %v, want [first second]", names)
	}
}

// TestAddRegistryEntryRejectsInvalidNames: the add path enforces the same
// name rule the daemon enforces at startup.
func TestAddRegistryEntryRejectsInvalidNames(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"", "solyto/api", `solyto\api`, ".", "..", "solyto api", "sölyto"} {
		path := filepath.Join(t.TempDir(), "serve.json")
		if _, err := AddRegistryEntry(path, bad, root); err == nil {
			t.Errorf("AddRegistryEntry with name %q = nil error, want an error", bad)
		}
	}
}

// TestAddRegistryEntryRejectsDuplicateName: a name already registered is an
// error, and the file is left unchanged.
func TestAddRegistryEntryRejectsDuplicateName(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"dup", a}))

	if _, err := AddRegistryEntry(path, "dup", b); err == nil {
		t.Fatal("AddRegistryEntry with a duplicate name = nil error, want an error")
	}
	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry after the refused add: %v", err)
	}
	if got := reg.Entries(); len(got) != 1 {
		t.Errorf("Entries() after the refused add = %+v, want the original single entry", got)
	}
}

// TestAddRegistryEntryRejectsDuplicatePath: a path already registered under
// another name is an error — including when spelled differently (relative vs
// absolute), since entries are compared on the resolved absolute path.
func TestAddRegistryEntryRejectsDuplicatePath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"name-a", root}))

	// Same path, different name, absolute spelling.
	if _, err := AddRegistryEntry(path, "name-b", root); err == nil {
		t.Fatal("AddRegistryEntry with a duplicate path = nil error, want an error")
	}
	// Same path via a relative spelling (relative to root's parent).
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Dir(root)); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Chdir(cwd) }()
	if _, err := AddRegistryEntry(path, "name-c", "."+string(filepath.Separator)+filepath.Base(root)); err == nil {
		t.Fatal("AddRegistryEntry with a relative spelling of a registered path = nil error, want an error")
	}
}

// TestAddRegistryEntryRejectsMissingRootAndNonDirectory: the root being
// registered must be an existing directory — a missing path and a file are
// both errors, with the daemon's wording.
func TestAddRegistryEntryRejectsMissingRootAndNonDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "some-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	for _, bad := range []string{file, missing} {
		path := filepath.Join(t.TempDir(), "serve.json")
		_, err := AddRegistryEntry(path, "proj", bad)
		if err == nil {
			t.Errorf("AddRegistryEntry with root %q = nil error, want an error", bad)
			continue
		}
		if !strings.Contains(err.Error(), "is not an existing directory") && !strings.Contains(err.Error(), "is not a directory") {
			t.Errorf("error for root %q = %v, want the not-a-directory wording", bad, err)
		}
	}
}

// TestAddRegistryEntryRefusesCorruptRegistry: a present-but-garbage config is
// an error (the LoadRegistry reuse), and the file is never touched.
func TestAddRegistryEntryRefusesCorruptRegistry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, `{not json`)

	if _, err := AddRegistryEntry(path, "proj", root); err == nil {
		t.Fatal("AddRegistryEntry on a corrupt registry = nil error, want an error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{not json` {
		t.Errorf("registry after the refused add = %q, want it untouched", data)
	}
}

// TestRemoveRegistryEntryRemovesByName: rm keeps the other entries in order
// and the daemon still loads the file afterwards.
func TestRemoveRegistryEntryRemovesByName(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"proj-a", a}, [2]string{"proj-b", b}))

	removed, err := RemoveRegistryEntry(path, "proj-a")
	if err != nil {
		t.Fatalf("RemoveRegistryEntry: %v", err)
	}
	if removed.Name != "proj-a" || removed.Path != filepath.Clean(a) {
		t.Errorf("removed entry = %+v, want {proj-a %s}", removed, filepath.Clean(a))
	}

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry after remove: %v", err)
	}
	got := reg.Entries()
	if len(got) != 1 || got[0].Name != "proj-b" {
		t.Errorf("Entries() after remove = %+v, want only proj-b", got)
	}
}

// TestRemoveRegistryEntryToEmptyWritesEmptyList: removing the last entry
// keeps the file on disk with an empty projects list (never `null`, never a
// deleted file) — the missing-file and empty-list shapes must both stay
// daemon-loadable, and `null` would be a hand-hostile shape.
func TestRemoveRegistryEntryToEmptyWritesEmptyList(t *testing.T) {
	a := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"only", a}))

	if _, err := RemoveRegistryEntry(path, "only"); err != nil {
		t.Fatalf("RemoveRegistryEntry: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("registry file after removing the last entry: %v (want it kept)", err)
	}
	if strings.Contains(string(data), "null") {
		t.Errorf("registry after removing the last entry = %q, want an empty list (never null)", data)
	}
	if reg, err := LoadRegistry(path); err != nil || len(reg.Projects()) != 0 {
		t.Errorf("LoadRegistry after removing the last entry = %v, %v; want an empty registry, nil", reg, err)
	}
}

// TestRemoveRegistryEntryUnknownName: removing a name that is not registered
// is an error.
func TestRemoveRegistryEntryUnknownName(t *testing.T) {
	a := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")
	writeRegistry(t, path, namedProjectsJSON([2]string{"proj", a}))

	if _, err := RemoveRegistryEntry(path, "nope"); err == nil {
		t.Fatal("RemoveRegistryEntry with an unknown name = nil error, want an error")
	}
}

// TestValidProjectName pins the exported wrapper against validProjectName's
// rule (single URL-safe segment, not "." or "..").
func TestValidProjectName(t *testing.T) {
	for _, good := range []string{"a", "My-Project_2", "x.y", "0"} {
		if !ValidProjectName(good) {
			t.Errorf("ValidProjectName(%q) = false, want true", good)
		}
	}
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "a b", "a:b"} {
		if ValidProjectName(bad) {
			t.Errorf("ValidProjectName(%q) = true, want false", bad)
		}
	}
}
