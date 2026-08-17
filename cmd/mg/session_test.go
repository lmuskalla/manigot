package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/session"
)

// pathWithRealGitOnly returns a PATH value containing git but nothing else,
// so the docker launch inside the session flow fails deterministically —
// never starting a real container on a machine that has docker, and failing
// with the same clear error on one that doesn't. The real git (not the
// manigot git shim, which may sit on PATH and refuses some session-flow git
// calls) is resolved explicitly.
func pathWithRealGitOnly(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	realGit := "/usr/bin/git"
	if _, err := os.Stat(realGit); err != nil {
		var lerr error
		realGit, lerr = exec.LookPath("git")
		if lerr != nil {
			t.Fatalf("could not locate a real git: %v", lerr)
		}
	}
	if err := os.Symlink(realGit, filepath.Join(binDir, "git")); err != nil {
		t.Fatal(err)
	}
	return binDir
}

// TestRunSessionPruneCalledBeforeRun pins the launch-path wiring: runSession
// calls pruneOrphans (stubbed — docker is not on the test PATH) before the
// docker run, and a successful prune is quiet. The run then proceeds to the
// docker launch, which fails on the git-only PATH — proving the prune
// happened before the run and did not replace it.
func TestRunSessionPruneCalledBeforeRun(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := mgCheckout(t)
	t.Chdir(dir)
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	var pruneCalls int
	old := pruneOrphans
	pruneOrphans = func(diag io.Writer) (session.PruneResult, error) {
		pruneCalls++
		return session.PruneResult{Removed: 1, Running: 0}, nil
	}
	t.Cleanup(func() { pruneOrphans = old })

	var out, errOut strings.Builder
	if code := runSession(nil, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1 (the docker launch fails on the git-only PATH)", code)
	}
	if pruneCalls != 1 {
		t.Errorf("pruneOrphans called %d times, want 1", pruneCalls)
	}
	if !strings.Contains(errOut.String(), "executable file not found") {
		t.Errorf("the docker launch must have been attempted after the prune:\n%s", errOut.String())
	}
	if strings.Contains(errOut.String(), "could not prune") {
		t.Errorf("a successful prune must not warn:\n%s", errOut.String())
	}
}

// TestRunSessionPruneFailureIsFailSoft pins the fail-soft contract: a prune
// failure only warns on stderr and never aborts the launch — the run proceeds
// to the docker launch (which fails on the git-only PATH) despite the prune
// error.
func TestRunSessionPruneFailureIsFailSoft(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := mgCheckout(t)
	t.Chdir(dir)
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	var pruneCalls int
	old := pruneOrphans
	pruneOrphans = func(diag io.Writer) (session.PruneResult, error) {
		pruneCalls++
		return session.PruneResult{}, errors.New("Cannot connect to the Docker daemon")
	}
	t.Cleanup(func() { pruneOrphans = old })

	var out, errOut strings.Builder
	if code := runSession(nil, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1 (the docker launch fails on the git-only PATH)", code)
	}
	if pruneCalls != 1 {
		t.Errorf("pruneOrphans called %d times, want 1", pruneCalls)
	}
	if !strings.Contains(errOut.String(), "mg: warning: could not prune orphaned containers") {
		t.Errorf("missing the prune warning:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "executable file not found") {
		t.Errorf("the launch must proceed past a prune failure (docker attempted):\n%s", errOut.String())
	}
}
