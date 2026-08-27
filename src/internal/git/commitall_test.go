package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommitAll verifies the happy path: a single CommitAll call sweeps a
// modified tracked file, a new untracked file, and a deleted tracked file
// into one commit, leaving the worktree clean.
func TestCommitAll(t *testing.T) {
	dir, _ := initRepo(t)
	writeFile(t, dir, "tracked.txt", "v1\n")
	writeFile(t, dir, "gone.txt", "bye\n")
	commitAll(t, dir, "setup")

	writeFile(t, dir, "tracked.txt", "v2\n")        // modified
	writeFile(t, dir, "new.txt", "fresh\n")         // new, untracked
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil { // deleted
		t.Fatal(err)
	}

	if err := CommitAll(dir, "[ab0001] chore: commit all"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	// The worktree is clean afterwards — nothing left for a clean-tree check
	// to trip on.
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("worktree not clean after CommitAll:\n%s", out)
	}

	// And the commit subject is the given message.
	subject, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(subject)); got != "[ab0001] chore: commit all" {
		t.Errorf("commit subject = %q, want %q", got, "[ab0001] chore: commit all")
	}
}

// TestCommitAllNothingToCommit verifies an already-clean worktree is a
// distinct, non-failure outcome: ErrNothingToCommit (errors.Is-matchable),
// and explicitly not ErrNotARepo.
func TestCommitAllNothingToCommit(t *testing.T) {
	dir, _ := initRepo(t)
	err := CommitAll(dir, "nothing here")
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("CommitAll on a clean repo: err = %v, want ErrNothingToCommit", err)
	}
	if errors.Is(err, ErrNotARepo) {
		t.Errorf("CommitAll on a clean repo misclassified as ErrNotARepo: %v", err)
	}
}

// TestCommitAllNotARepo mirrors the other functions' non-repo classification.
func TestCommitAllNotARepo(t *testing.T) {
	err := CommitAll(t.TempDir(), "msg")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("CommitAll on non-repo: err = %v, want ErrNotARepo", err)
	}
}

// TestCommitAllSweepsStagedChanges verifies git add -A semantics: content a
// prior partial `git add` left staged-but-uncommitted is swept into the
// CommitAll commit, not lost or duplicated.
func TestCommitAllSweepsStagedChanges(t *testing.T) {
	dir, _ := initRepo(t)
	writeFile(t, dir, "staged.txt", "v1\n")
	commitAll(t, dir, "setup")

	writeFile(t, dir, "staged.txt", "v2\n")
	runGit(t, dir, "add", "staged.txt")

	if err := CommitAll(dir, "sweep"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("worktree not clean after CommitAll:\n%s", out)
	}

	subject, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(subject)); got != "sweep" {
		t.Errorf("commit subject = %q, want %q", got, "sweep")
	}
}
