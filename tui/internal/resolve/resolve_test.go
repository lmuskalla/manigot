package resolve

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExec creates an executable stub file at dir/name and returns its path.
func writeExec(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// isolate points $PATH at a single empty temp dir and clears the env vars this
// package reads, so a resolver test cannot be influenced by what the host
// machine happens to have installed. It returns that dir.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("TEST_BIN_OVERRIDE", "")
	return dir
}

func testSpec() Spec {
	return Spec{
		Label:  "manigot-test",
		EnvVar: "TEST_BIN_OVERRIDE",
		Names:  []string{"manigot-test", "sc-test", "legacy-test"},
		Script: "scripts/test.sh",
	}
}

func TestResolveEnvOverrideWithPath(t *testing.T) {
	isolate(t)
	other := t.TempDir()
	want := writeExec(t, other, "anything-at-all")
	t.Setenv("TEST_BIN_OVERRIDE", want)

	got, err := Resolve(testSpec())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
	if got.How != "$TEST_BIN_OVERRIDE" {
		t.Errorf("How = %q, want $TEST_BIN_OVERRIDE", got.How)
	}
}

func TestResolveEnvOverrideWithBareName(t *testing.T) {
	dir := isolate(t)
	want := writeExec(t, dir, "some-other-name")
	t.Setenv("TEST_BIN_OVERRIDE", "some-other-name")

	got, err := Resolve(testSpec())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
}

// A broken override must fail loudly instead of silently falling through to
// $PATH — otherwise a typo in the env var is invisible.
func TestResolveEnvOverrideBrokenDoesNotFallThrough(t *testing.T) {
	dir := isolate(t)
	writeExec(t, dir, "manigot-test") // would resolve if we fell through
	t.Setenv("TEST_BIN_OVERRIDE", filepath.Join(dir, "does-not-exist"))

	_, err := Resolve(testSpec())
	if err == nil {
		t.Fatal("expected an error for a broken override")
	}
	if !strings.Contains(err.Error(), "TEST_BIN_OVERRIDE") {
		t.Errorf("error should name the env var; got: %v", err)
	}
}

func TestResolveEnvOverrideIgnoresNonExecutable(t *testing.T) {
	dir := isolate(t)
	plain := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(plain, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_BIN_OVERRIDE", plain)

	if _, err := Resolve(testSpec()); err == nil {
		t.Fatal("expected an error for a non-executable override")
	}
}

func TestResolveNameOnPath(t *testing.T) {
	dir := isolate(t)
	want := writeExec(t, dir, "manigot-test")

	got, err := Resolve(testSpec())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
	if got.How != "manigot-test on $PATH" {
		t.Errorf("How = %q", got.How)
	}
}

func TestResolveScriptFallbackViamanigotHome(t *testing.T) {
	isolate(t)
	home := t.TempDir()
	want := writeExec(t, filepath.Join(home, "scripts"), "test.sh")
	t.Setenv("MANIGOT_HOME", home)

	got, err := Resolve(testSpec())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
}

func TestResolveNotFound(t *testing.T) {
	isolate(t)

	_, err := Resolve(testSpec())
	if err == nil {
		t.Fatal("expected an error when nothing is installed")
	}
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("want *NotFoundError, got %T", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say 'not found'; got: %v", err)
	}
	// Every strategy must be reported, so the user can see what was checked.
	for _, want := range []string{"TEST_BIN_OVERRIDE", "manigot-test", "sc-test", "legacy-test", "MANIGOT_HOME"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q; got: %v", want, err)
		}
	}
}

// makeCheckout creates a directory that looksLikeCheckout accepts.
func makeCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeExec(t, filepath.Join(root, "scripts"), "run.sh")
	return root
}

func TestLooksLikeCheckout(t *testing.T) {
	root := makeCheckout(t)
	if !looksLikeCheckout(root) {
		t.Errorf("looksLikeCheckout(%q) = false, want true", root)
	}
	// A bare temp dir — what `go run` produces — must be rejected.
	if looksLikeCheckout(t.TempDir()) {
		t.Error("a directory without scripts/run.sh must not count as a checkout")
	}
	for _, dir := range []string{"", ".", string(filepath.Separator)} {
		if looksLikeCheckout(dir) {
			t.Errorf("looksLikeCheckout(%q) = true, want false", dir)
		}
	}
}

// The test binary itself does not live in a checkout, so executableRoots must
// come up empty rather than inventing a root — this is the `go run` / `go test`
// case.
func TestExecutableRootsOutsideCheckout(t *testing.T) {
	if got := executableRoots(); len(got) != 0 {
		t.Errorf("executableRoots() = %v, want empty for a binary outside a checkout", got)
	}
}

func TestSeedHomeRespectsExistingValue(t *testing.T) {
	isolate(t)
	t.Setenv("MANIGOT_HOME", "/somewhere/explicit")
	if got := SeedHome(); got != "/somewhere/explicit" {
		t.Errorf("SeedHome() = %q, want the pre-set value", got)
	}
}

func TestSeedHomeNoCheckout(t *testing.T) {
	isolate(t)
	if got := SeedHome(); got != "" {
		t.Errorf("SeedHome() = %q, want empty when no checkout can be derived", got)
	}
}

func TestHome(t *testing.T) {
	isolate(t)
	if got := Home(); got != "" {
		t.Errorf("Home() = %q, want empty when nothing is known", got)
	}
	root := makeCheckout(t)
	t.Setenv("MANIGOT_HOME", root)
	if got := Home(); got != root {
		t.Errorf("Home() = %q, want %q", got, root)
	}
}

// TestResolutionOrderEndToEnd is the guard against a silent regression in the
// lookup order. Everything is installed at once — override, canonical, short and
// legacy names on $PATH, and the script in a checkout — and then the winner is
// removed one step at a time. Each stage must fall through to exactly the next
// strategy, ending at the script fallback.
func TestResolutionOrderEndToEnd(t *testing.T) {
	spec := testSpec()

	bin := isolate(t)
	home := t.TempDir()

	override := writeExec(t, t.TempDir(), "override-bin")
	canonical := writeExec(t, bin, spec.Names[0])
	short := writeExec(t, bin, spec.Names[1])
	legacy := writeExec(t, bin, spec.Names[2])
	script := writeExec(t, filepath.Join(home, filepath.Dir(spec.Script)), filepath.Base(spec.Script))

	t.Setenv("MANIGOT_HOME", home)
	t.Setenv(spec.EnvVar, override)

	stages := []struct {
		name     string
		wantPath string
		wantHow  string
		// disable makes the current winner unavailable for the next stage.
		disable func()
	}{
		{"env override", override, "$" + spec.EnvVar, func() { t.Setenv(spec.EnvVar, "") }},
		{"canonical name", canonical, spec.Names[0] + " on $PATH", func() { os.Remove(canonical) }},
		{"short name", short, spec.Names[1] + " on $PATH", func() { os.Remove(short) }},
		{"legacy name", legacy, spec.Names[2] + " on $PATH", func() { os.Remove(legacy) }},
		{"script in $MANIGOT_HOME", script, script, func() { os.Remove(script) }},
	}

	for _, stage := range stages {
		got, err := Resolve(spec)
		if err != nil {
			t.Fatalf("%s: Resolve: %v", stage.name, err)
		}
		if got.Path != stage.wantPath {
			t.Errorf("%s: Path = %q, want %q", stage.name, got.Path, stage.wantPath)
		}
		if got.How != stage.wantHow {
			t.Errorf("%s: How = %q, want %q", stage.name, got.How, stage.wantHow)
		}
		stage.disable()
	}

	// With every strategy exhausted, resolution must fail — not fall back to
	// something surprising.
	if got, err := Resolve(spec); err == nil {
		t.Fatalf("expected failure once nothing is installed, got %q", got.Path)
	}
}

func TestResolveIgnoresDirectories(t *testing.T) {
	isolate(t)
	home := t.TempDir()
	// A directory named like the script must not be mistaken for it.
	if err := os.MkdirAll(filepath.Join(home, "scripts", "test.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", home)

	if _, err := Resolve(testSpec()); err == nil {
		t.Fatal("expected an error when the script path is a directory")
	}
}
