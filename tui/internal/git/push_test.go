package git

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// bareRepo creates a bare git repository at a temp dir, suitable for use as
// an `origin` remote in these tests — a real remote, not a mock, so Push is
// exercised against an actual `git push` round trip.
func bareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "--bare", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare %s: %v\n%s", dir, err, out)
	}
	return dir
}

// TestPush verifies the happy path: an existing origin remote and a branch
// with a real commit push successfully, and the pushed ref really lands on
// the remote.
func TestPush(t *testing.T) {
	dir, def := initRepo(t)
	origin := bareRepo(t)
	runGit(t, dir, "remote", "add", "origin", origin)
	runGit(t, dir, "branch", "feature/aaa")
	runGit(t, dir, "checkout", "-q", "feature/aaa")
	writeFile(t, dir, "feature.txt", "work\n")
	commitAll(t, dir, "feature work")

	if err := Push(dir, "feature/aaa"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// The branch ref really landed on the bare remote.
	localHash, err := exec.Command("git", "-C", dir, "rev-parse", "feature/aaa").Output()
	if err != nil {
		t.Fatal(err)
	}
	remoteHash, err := exec.Command("git", "-C", origin, "rev-parse", "feature/aaa").Output()
	if err != nil {
		t.Fatalf("rev-parse on remote after push: %v", err)
	}
	if strings.TrimSpace(string(localHash)) != strings.TrimSpace(string(remoteHash)) {
		t.Errorf("remote feature/aaa = %q, want %q (local)", remoteHash, localHash)
	}

	runGit(t, dir, "checkout", "-q", def)
}

// TestPushSetsUpstreamTracking verifies the -u flag actually configures
// tracking, so a later plain `git push` (no explicit remote/branch) against
// the same branch would work — this is the "-u sets upstream tracking on
// first push" behavior TASK-1 documents.
func TestPushSetsUpstreamTracking(t *testing.T) {
	dir, _ := initRepo(t)
	origin := bareRepo(t)
	runGit(t, dir, "remote", "add", "origin", origin)
	runGit(t, dir, "branch", "feature/bbb")
	runGit(t, dir, "checkout", "-q", "feature/bbb")

	if err := Push(dir, "feature/bbb"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "feature/bbb@{upstream}").Output()
	if err != nil {
		t.Fatalf("no upstream configured after Push: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "origin/feature/bbb" {
		t.Errorf("upstream = %q, want origin/feature/bbb", got)
	}
}

// TestPushNoOriginRemote verifies a missing origin remote degrades to a
// clear wrapped error rather than hanging or crashing.
func TestPushNoOriginRemote(t *testing.T) {
	dir, def := initRepo(t)
	err := Push(dir, def)
	if err == nil {
		t.Fatal("Push with no origin remote: expected an error, got nil")
	}
	if errors.Is(err, ErrNotARepo) {
		t.Errorf("Push with no origin remote misclassified as ErrNotARepo: %v", err)
	}
}

// TestPushRejectedNonFastForward verifies a diverged remote (someone else
// pushed to the same branch in the meantime) surfaces as a normal rejected
// -push error rather than being silently overwritten — Push never forces.
func TestPushRejectedNonFastForward(t *testing.T) {
	dir, def := initRepo(t)
	origin := bareRepo(t)
	runGit(t, dir, "remote", "add", "origin", origin)
	runGit(t, dir, "branch", "feature/ccc")
	runGit(t, dir, "checkout", "-q", "feature/ccc")
	writeFile(t, dir, "feature.txt", "v1\n")
	commitAll(t, dir, "v1")

	if err := Push(dir, "feature/ccc"); err != nil {
		t.Fatalf("initial Push: %v", err)
	}

	// Simulate another host pushing a divergent commit to the same branch:
	// clone the bare remote elsewhere, commit there, and push that.
	other := t.TempDir()
	runGit(t, other, "clone", "-q", origin, ".")
	runGit(t, other, "config", "user.email", "t@example.com")
	runGit(t, other, "config", "user.name", "Test")
	runGit(t, other, "config", "commit.gpgsign", "false")
	runGit(t, other, "checkout", "-q", "feature/ccc")
	writeFile(t, other, "feature.txt", "v2-from-other-host\n")
	commitAll(t, other, "v2 from other host")
	runGit(t, other, "push", "-q", "origin", "feature/ccc")

	// Back in dir, make a divergent local commit on the same branch.
	writeFile(t, dir, "feature.txt", "v2-from-this-host\n")
	commitAll(t, dir, "v2 from this host")

	err := Push(dir, "feature/ccc")
	if err == nil {
		t.Fatal("Push of a diverged branch: expected a rejected-push error, got nil")
	}
	if errors.Is(err, ErrNotARepo) {
		t.Errorf("Push of a diverged branch misclassified as ErrNotARepo: %v", err)
	}

	// And the remote must still hold the other host's commit, not have been
	// force-overwritten.
	remoteHash, rerr := exec.Command("git", "-C", origin, "rev-parse", "feature/ccc").Output()
	if rerr != nil {
		t.Fatal(rerr)
	}
	otherHash, oerr := exec.Command("git", "-C", other, "rev-parse", "feature/ccc").Output()
	if oerr != nil {
		t.Fatal(oerr)
	}
	if strings.TrimSpace(string(remoteHash)) != strings.TrimSpace(string(otherHash)) {
		t.Errorf("remote feature/ccc was overwritten by the rejected push: %q, want %q (other host's commit)", remoteHash, otherHash)
	}

	runGit(t, dir, "checkout", "-q", def)
}

// TestPushNotARepo mirrors the other functions' non-repo classification.
func TestPushNotARepo(t *testing.T) {
	err := Push(t.TempDir(), "main")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("Push on non-repo: err = %v, want ErrNotARepo", err)
	}
}
