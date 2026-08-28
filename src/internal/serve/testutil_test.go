package serve

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
)

// entryFor turns a project root into a registry Entry named after the
// root's base name — the shared shape most tests use, since URL construction
// throughout this package already spells out filepath.Base(root) (a holdover
// from when Project matched by base name). Resolution is strictly by
// configured Name now — see TestRegistryProjectNameOnlyResolution in
// registry_test.go for the pin that a differently-named entry does NOT
// resolve by its directory's base name.
func entryFor(root string) Entry {
	return Entry{Name: filepath.Base(root), Path: filepath.Clean(root)}
}

// netListen is net.Listen's tcp form, split out so tests read cleanly.
func netListen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// httpGet is http.Get, split out for the same reason.
func httpGet(url string) (*http.Response, error) {
	return http.Get(url)
}

// writeJDIStatusForTest writes a stopped:finished reviewer sidecar for the
// named job under root, so job-reader tests have a live status to read.
func writeJDIStatusForTest(t *testing.T, root, jobName string) error {
	t.Helper()
	return job.WriteJDIStatus(root, jobName, job.JDIStoppedFinished, "reviewer")
}

// pathWithRealGitOnly returns a PATH value containing the real git binary and
// nothing else, so tests that need `git init` (to build throwaway repos) are
// not blocked by the manigot session git shim, which may sit on PATH and
// refuses git init. Mirrors cmd/mg/session_test.go's same-named helper.
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

// runGitT runs git -C dir args, failing the test on any error — the throwaway
// repo builder for tests that exercise real git (the diff endpoint).
func runGitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// initGitRepo creates a fresh git repo at a temp dir (default branch main, a
// deterministic identity, gpg signing off) with one initial commit, returning
// the dir. The caller must set PATH to pathWithRealGitOnly first.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitT(t, dir, "init", "-q", "-b", "main")
	runGitT(t, dir, "config", "user.email", "t@example.com")
	runGitT(t, dir, "config", "user.name", "Test")
	runGitT(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitT(t, dir, "add", "README")
	runGitT(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// fakeCheckout creates a minimal manigot checkout (scripts/entrypoint.sh, the
// file home.looksLikeCheckout keys off, plus agents/ with the named agent
// files) and points $MANIGOT_HOME at it. Returns the checkout dir.
func fakeCheckout(t *testing.T, agents map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if len(agents) > 0 {
		if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range agents {
			if err := os.WriteFile(filepath.Join(dir, "agents", name+".md"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("MANIGOT_HOME", dir)
	return dir
}
