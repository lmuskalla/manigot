package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWithContextTimeoutSurfacesDeadline exercises the context-aware exec
// path with a stubbed slow git: a fake `git` on PATH that sleeps far longer
// than the context's deadline, so the call must be cut off by the timeout
// and surface it as an ordinary wrapped error — never hang the test (or a
// real caller) and never panic.
func TestWithContextTimeoutSurfacesDeadline(t *testing.T) {
	// Fake git on PATH that outlives any sane timeout.
	dir := t.TempDir()
	stub := filepath.Join(dir, "git")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexec sleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := PushWithContext(ctx, "/nonexistent-root", "main")
	if err == nil {
		t.Fatal("PushWithContext with a slow git: expected a timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("call took %v — the timeout did not cut the slow git off", elapsed)
	}
	// The deadline must be reachable through the wrap chain (wrapErr uses %w).
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

// TestPlainRunHasNoTimeout confirms the context-free variants keep the
// original no-timeout behavior: the slow fake git makes a plain call take as
// long as the stub itself, not the ~instant cut-off of the ctx variants.
func TestPlainRunHasNoTimeout(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "git")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexec sleep 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	start := time.Now()
	_ = Push("/nonexistent-root", "main")
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Errorf("plain Push returned after %v — the no-timeout variant must wait for the stub, not cut it off", elapsed)
	}
}
