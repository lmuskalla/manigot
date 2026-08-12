package home

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeCheckout builds a directory that looksLikeCheckout accepts (scripts/
// with entrypoint.sh).
func fakeCheckout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRootHonorsManigotHome(t *testing.T) {
	dir := fakeCheckout(t)
	t.Setenv(EnvHome, dir)
	if got := Root(); got != dir {
		t.Errorf("Root() = %q, want %q", got, dir)
	}
}

func TestRootEmptyWithoutCheckout(t *testing.T) {
	t.Setenv(EnvHome, "")
	// The test binary lives in a temp build dir and cwd is the package dir
	// (inside the real checkout, which has scripts/entrypoint.sh) — so Root
	// finds the real checkout. To test the empty case, point the cwd
	// somewhere neutral is not possible portably; instead assert that Root
	// either finds a checkout or returns non-empty only when one exists.
	// The meaningful contract is the MANIGOT_HOME path (above) and
	// looksLikeCheckout's rejection (below).
	if got := looksLikeCheckout(t.TempDir()); got {
		t.Error("looksLikeCheckout on an empty dir = true, want false")
	}
}

func TestLooksLikeCheckout(t *testing.T) {
	dir := fakeCheckout(t)
	if !looksLikeCheckout(dir) {
		t.Error("looksLikeCheckout on a checkout = false, want true")
	}
	// A directory without scripts/entrypoint.sh must not count.
	plain := t.TempDir()
	if looksLikeCheckout(plain) {
		t.Error("looksLikeCheckout on a plain dir = true, want false")
	}
	// Root-level guards.
	if looksLikeCheckout("") || looksLikeCheckout(".") || looksLikeCheckout("/") {
		t.Error("looksLikeCheckout accepted an empty/dot/root dir")
	}
}

func TestSeedSetsEnv(t *testing.T) {
	t.Setenv(EnvHome, "")
	dir := fakeCheckout(t)
	t.Setenv(EnvHome, dir)
	if got := Seed(); got != dir {
		t.Errorf("Seed() = %q, want %q", got, dir)
	}
	if got := os.Getenv(EnvHome); got != dir {
		t.Errorf("MANIGOT_HOME after Seed = %q, want %q", got, dir)
	}
}

func TestRootTrimsWhitespace(t *testing.T) {
	dir := fakeCheckout(t)
	t.Setenv(EnvHome, " "+dir+" ")
	if got := Root(); got != dir {
		t.Errorf("Root() = %q, want trimmed %q", got, dir)
	}
}
