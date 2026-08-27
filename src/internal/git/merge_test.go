package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeBranch verifies the happy path with a diverged-branch fixture: the
// base branch moves on while the job branch is out, and merging it back in
// produces a real merge commit (the `--no-edit` case — git would otherwise
// open an editor for the auto-generated merge-commit message) and brings the
// base's changes into the job worktree.
func TestMergeBranch(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/aaa")
	runGit(t, dir, "checkout", "-q", "feature/aaa")
	writeFile(t, dir, "feature.txt", "feature work\n")
	commitAll(t, dir, "feature work")

	// Divergence: the base branch moves on while the job branch is out.
	runGit(t, dir, "checkout", "-q", def)
	writeFile(t, dir, "base.txt", "base work\n")
	commitAll(t, dir, "base work")

	// Back on the job branch: merging the base brings its commit in.
	runGit(t, dir, "checkout", "-q", "feature/aaa")
	if err := MergeBranch(dir, def); err != nil {
		t.Fatalf("MergeBranch: %v", err)
	}

	// The merged-in file really landed in the job worktree...
	if _, err := os.Stat(filepath.Join(dir, "base.txt")); err != nil {
		t.Errorf("base.txt missing after merge: %v", err)
	}
	// ...and the diverged merge produced a merge commit (two parents) on the
	// job branch.
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%P").Output()
	if err != nil {
		t.Fatal(err)
	}
	if parents := strings.Fields(string(out)); len(parents) != 2 {
		t.Errorf("tip of feature/aaa has %d parents, want 2 (a merge commit)", len(parents))
	}

	runGit(t, dir, "checkout", "-q", def)
}

// TestMergeBranchConflict verifies a merge that hits conflicting changes
// surfaces as a normal wrapped error (mentioning CONFLICT) rather than
// crashing or misclassifying as ErrNotARepo — the tree is left in the
// conflicted state for the user to resolve manually.
func TestMergeBranchConflict(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/bbb")
	runGit(t, dir, "checkout", "-q", "feature/bbb")
	writeFile(t, dir, "shared.txt", "feature version\n")
	commitAll(t, dir, "feature work")

	runGit(t, dir, "checkout", "-q", def)
	writeFile(t, dir, "shared.txt", "base version\n")
	commitAll(t, dir, "base work")

	runGit(t, dir, "checkout", "-q", "feature/bbb")
	err := MergeBranch(dir, def)
	if err == nil {
		t.Fatal("MergeBranch with conflicting changes: expected an error, got nil")
	}
	if errors.Is(err, ErrNotARepo) {
		t.Errorf("MergeBranch conflict misclassified as ErrNotARepo: %v", err)
	}
	if !strings.Contains(err.Error(), "CONFLICT") {
		t.Errorf("MergeBranch conflict error should mention CONFLICT, got: %v", err)
	}
	// The tree is left in the conflicted state (unmerged index entries) —
	// switching branches now would fail, so the test just ends here.
}

// TestMergeBranchDirtyWorktreeRefused verifies git's refusal to merge into a
// worktree whose local changes would be overwritten surfaces as a wrapped
// error, not a crash — the TUI's status-line degradation for the same case.
func TestMergeBranchDirtyWorktreeRefused(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/ccc")
	runGit(t, dir, "checkout", "-q", "feature/ccc")
	writeFile(t, dir, "shared.txt", "feature version\n")
	commitAll(t, dir, "feature work")

	runGit(t, dir, "checkout", "-q", def)
	writeFile(t, dir, "shared.txt", "base version\n")
	commitAll(t, dir, "base work")

	runGit(t, dir, "checkout", "-q", "feature/ccc")
	// Dirty the tracked file the merge would have to overwrite.
	writeFile(t, dir, "shared.txt", "dirty local edit\n")

	err := MergeBranch(dir, def)
	if err == nil {
		t.Fatal("MergeBranch into a dirty worktree: expected an error, got nil")
	}
	if errors.Is(err, ErrNotARepo) {
		t.Errorf("MergeBranch into a dirty worktree misclassified as ErrNotARepo: %v", err)
	}
}

// TestMergeBranchAlreadyUpToDate verifies an undiverged branch is a successful
// no-op ("Already up to date.") rather than an error.
func TestMergeBranchAlreadyUpToDate(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/ddd")
	runGit(t, dir, "checkout", "-q", "feature/ddd")
	// No divergence at all — merging the base must be a no-op success.
	if err := MergeBranch(dir, def); err != nil {
		t.Fatalf("MergeBranch of an up-to-date branch: %v", err)
	}
	runGit(t, dir, "checkout", "-q", def)
}

// TestMergeBranchMissingBranch verifies a branch that doesn't exist surfaces
// as a wrapped error (git's "not something we can merge"), not ErrNotARepo.
func TestMergeBranchMissingBranch(t *testing.T) {
	dir, _ := initRepo(t)
	err := MergeBranch(dir, "no-such-branch")
	if err == nil {
		t.Fatal("MergeBranch of a missing branch: expected an error, got nil")
	}
	if errors.Is(err, ErrNotARepo) {
		t.Errorf("MergeBranch of a missing branch misclassified as ErrNotARepo: %v", err)
	}
}

// TestMergeBranchNotARepo mirrors the other functions' non-repo classification.
func TestMergeBranchNotARepo(t *testing.T) {
	err := MergeBranch(t.TempDir(), "main")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("MergeBranch on non-repo: err = %v, want ErrNotARepo", err)
	}
}
