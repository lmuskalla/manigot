package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/serve"
)

// runServeProjectsCmd runs the command with captured streams, returning the
// exit code, stdout and stderr.
func runServeProjectsCmd(args ...string) (int, string, string) {
	var stdout, stderr strings.Builder
	code := runServeProjects(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestServeProjectsListEmpty: with no registry file yet, list is the empty
// state plus the add hint, exit 0 (a missing file is an empty registry, not
// an error — the same degrade as mg serve).
func TestServeProjectsListEmpty(t *testing.T) {
	code, stdout, stderr := runServeProjectsCmd("--registry", filepath.Join(t.TempDir(), "serve.json"))
	if code != 0 {
		t.Fatalf("exit = %d (stdout %q, stderr %q), want 0", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "No projects registered") || !strings.Contains(stdout, "mg serve-projects add") {
		t.Errorf("stdout = %q, want the empty state + add hint", stdout)
	}
}

// TestServeProjectsAddExplicitThenList: add with an explicit path and name
// writes a daemon-loadable entry, and list shows it.
func TestServeProjectsAddExplicitThenList(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")

	code, stdout, stderr := runServeProjectsCmd("--registry", path, "add", root, "my-project")
	if code != 0 {
		t.Fatalf("add: exit = %d (stdout %q, stderr %q), want 0", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Registered 'my-project'") || !strings.Contains(stdout, "Restart a running mg serve") {
		t.Errorf("add stdout = %q, want the registered + restart lines", stdout)
	}
	// The added root has no docs/ — the warning the daemon would print.
	if !strings.Contains(stdout, "no docs/ directory") {
		t.Errorf("add stdout = %q, want the missing-docs warning", stdout)
	}

	reg, err := serve.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry after add: %v", err)
	}
	entries := reg.Entries()
	if len(entries) != 1 || entries[0].Name != "my-project" || entries[0].Path != filepath.Clean(root) {
		t.Fatalf("Entries() = %+v, want {my-project %s}", entries, filepath.Clean(root))
	}

	code, stdout, stderr = runServeProjectsCmd("--registry", path)
	if code != 0 {
		t.Fatalf("list: exit = %d (stdout %q, stderr %q), want 0", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "my-project") || !strings.Contains(stdout, filepath.Clean(root)) {
		t.Errorf("list stdout = %q, want the registered row", stdout)
	}
	// list carries the daemon's missing-docs warning for the entry.
	if !strings.Contains(stderr, "no docs/ directory") {
		t.Errorf("list stderr = %q, want the missing-docs warning", stderr)
	}
}

// TestServeProjectsAddDefaults: with no arguments the path defaults to the
// current directory and the name to its base name.
func TestServeProjectsAddDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "my-app"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "serve.json")

	t.Chdir(filepath.Join(root, "my-app"))
	code, stdout, stderr := runServeProjectsCmd("--registry", path, "add")
	if code != 0 {
		t.Fatalf("add: exit = %d (stdout %q, stderr %q), want 0", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Registered 'my-app'") {
		t.Errorf("add stdout = %q, want the name defaulted to the base name", stdout)
	}

	reg, err := serve.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry after add: %v", err)
	}
	entries := reg.Entries()
	if len(entries) != 1 || entries[0].Name != "my-app" || entries[0].Path != filepath.Join(root, "my-app") {
		t.Errorf("Entries() = %+v, want {my-app %s}", entries, filepath.Join(root, "my-app"))
	}
}

// TestServeProjectsAddDuplicateName: a second add under a taken name fails
// with the duplicate error.
func TestServeProjectsAddDuplicateName(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")

	if code, _, _ := runServeProjectsCmd("--registry", path, "add", root, "dup"); code != 0 {
		t.Fatal("first add failed")
	}
	code, _, stderr := runServeProjectsCmd("--registry", path, "add", other, "dup")
	if code == 0 {
		t.Fatal("second add with a duplicate name = exit 0, want an error")
	}
	if !strings.Contains(stderr, "already registered") {
		t.Errorf("stderr = %q, want the already-registered error", stderr)
	}
}

// TestServeProjectsAddMissingRoot: adding a path that is not an existing
// directory fails.
func TestServeProjectsAddMissingRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.json")
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	code, _, stderr := runServeProjectsCmd("--registry", path, "add", missing, "proj")
	if code == 0 {
		t.Fatal("add of a missing path = exit 0, want an error")
	}
	if !strings.Contains(stderr, "is not an existing directory") {
		t.Errorf("stderr = %q, want the not-a-directory error", stderr)
	}
}

// TestServeProjectsRmThenListEmpty: rm by name unregisters the entry and the
// list returns to the empty state.
func TestServeProjectsRmThenListEmpty(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "serve.json")

	if code, _, _ := runServeProjectsCmd("--registry", path, "add", root, "my-project"); code != 0 {
		t.Fatal("add failed")
	}
	code, stdout, stderr := runServeProjectsCmd("--registry", path, "rm", "my-project")
	if code != 0 {
		t.Fatalf("rm: exit = %d (stdout %q, stderr %q), want 0", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Unregistered 'my-project'") || !strings.Contains(stdout, "Restart a running mg serve") {
		t.Errorf("rm stdout = %q, want the unregistered + restart lines", stdout)
	}

	code, stdout, _ = runServeProjectsCmd("--registry", path, "list")
	if code != 0 || !strings.Contains(stdout, "No projects registered") {
		t.Errorf("list after rm: exit %d, stdout %q; want 0 + the empty state", code, stdout)
	}
}

// TestServeProjectsRmUnknown: removing an unregistered name is an error.
func TestServeProjectsRmUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.json")
	code, _, stderr := runServeProjectsCmd("--registry", path, "rm", "nope")
	if code == 0 {
		t.Fatal("rm of an unknown name = exit 0, want an error")
	}
	if !strings.Contains(stderr, "no project named") {
		t.Errorf("stderr = %q, want the unknown-name error", stderr)
	}
}

// TestServeProjectsUnknownSubcommandAndUsage: an unknown subcommand and
// wrong arities are clear usage errors (exit 1); `help` prints the usage to
// stdout (exit 0).
func TestServeProjectsUnknownSubcommandAndUsage(t *testing.T) {
	for _, args := range [][]string{
		{"frobnicate"},
		{"rm"},
		{"rm", "a", "b"},
		{"add", "p", "n", "extra"},
	} {
		path := filepath.Join(t.TempDir(), "serve.json")
		full := append([]string{"--registry", path}, args...)
		code, _, stderr := runServeProjectsCmd(full...)
		if code != 1 {
			t.Errorf("runServeProjects(%v) = exit %d, want 1", args, code)
		}
		if !strings.Contains(stderr, "Usage") {
			t.Errorf("runServeProjects(%v) stderr = %q, want a Usage line", args, stderr)
		}
	}

	code, stdout, _ := runServeProjectsCmd("help")
	if code != 0 || !strings.Contains(stdout, "mg serve-projects") {
		t.Errorf("help: exit %d, stdout %q; want 0 + the usage text", code, stdout)
	}
}

// TestServeProjectsNoRegistryLocation: with no --registry and no resolvable
// checkout, the command fails with the shared ErrNoRegistryPath error —
// mirroring mg serve's behavior in the same shape.
func TestServeProjectsNoRegistryLocation(t *testing.T) {
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("PATH", t.TempDir())
	code, _, stderr := runServeProjectsCmd()
	if code == 0 {
		t.Fatal("runServeProjects with no registry location = exit 0, want an error")
	}
	if !strings.Contains(stderr, "registry") {
		t.Errorf("stderr = %q, want the no-registry-location error", stderr)
	}
}
