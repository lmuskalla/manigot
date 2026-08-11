package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// runGit runs git -C dir args, failing the test on any error. Every test in
// this package builds a throwaway repo this way rather than stubbing git: the
// whole point of this package is to shell out to real git, so the tests must
// exercise the same path.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// initRepo creates a fresh git repo at a temp dir with a deterministic identity
// and gpg signing disabled, then returns the default branch's short name (so
// tests don't assume "main" vs "master" across git versions).
func initRepo(t *testing.T) (dir, defaultBranch string) {
	t.Helper()
	dir = t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	// An initial commit is required before any branch ref exists.
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-q", "-m", "init")
	b, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("read default branch: %v", err)
	}
	return dir, b
}

// commitAll stages every change under dir and commits it.
func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", msg)
}

// writeFile writes a file under dir, creating parent dirs as needed.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalBranches(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/aaa_x")
	runGit(t, dir, "branch", "fix/bbb_y")

	got, err := LocalBranches(dir)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}
	want := []string{def, "feature/aaa_x", "fix/bbb_y"}
	sort.Strings(got)
	sort.Strings(want)
	// Compare element-wise in case git returns them in a different order.
	if len(got) != len(want) {
		t.Fatalf("LocalBranches = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("LocalBranches[%d] = %q, want %q (got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}

func TestLocalBranchesNoCommits(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	got, err := LocalBranches(dir)
	if err != nil {
		t.Fatalf("LocalBranches on unborn repo: unexpected error %v", err)
	}
	if len(got) != 0 {
		t.Errorf("LocalBranches on repo with no branches = %v, want empty", got)
	}
}

func TestLocalBranchesNotARepo(t *testing.T) {
	_, err := LocalBranches(t.TempDir())
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("LocalBranches on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestCurrentBranch(t *testing.T) {
	dir, def := initRepo(t)
	got, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != def {
		t.Errorf("CurrentBranch = %q, want %q", got, def)
	}

	// Checkout another branch and confirm it's reflected.
	runGit(t, dir, "branch", "other")
	runGit(t, dir, "checkout", "-q", "other")
	got, _ = CurrentBranch(dir)
	if got != "other" {
		t.Errorf("CurrentBranch after checkout = %q, want other", got)
	}
}

func TestCurrentBranchDetachedHEAD(t *testing.T) {
	dir, _ := initRepo(t)
	// Detach HEAD at the current commit.
	hash := strings.TrimSpace(func() string {
		out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}())
	runGit(t, dir, "checkout", "-q", "--detach", hash)

	got, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch on detached HEAD: unexpected error %v", err)
	}
	if got != "" {
		t.Errorf("CurrentBranch on detached HEAD = %q, want empty", got)
	}
}

func TestCurrentBranchNotARepo(t *testing.T) {
	_, err := CurrentBranch(t.TempDir())
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("CurrentBranch on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestCheckout(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/aaa")
	runGit(t, dir, "checkout", "-q", "feature/aaa")

	if err := Checkout(dir, def); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	got, _ := CurrentBranch(dir)
	if got != def {
		t.Errorf("after Checkout: CurrentBranch = %q, want %q", got, def)
	}
}

func TestCountVerdictCommits(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/aaaa01_x")
	runGit(t, dir, "checkout", "-q", "feature/aaaa01_x")
	writeFile(t, dir, "docs/jobs/aaaa01_x/tasks.md", "tasks\n")
	commitAll(t, dir, "[aaaa01] TASK-1: do a thing")
	writeFile(t, dir, "docs/jobs/aaaa01_x/verdict.md", "NEEDS WORK\n")
	commitAll(t, dir, "[aaaa01] verdict: needs work on TASK-1")
	writeFile(t, dir, "docs/jobs/aaaa01_x/tasks.md", "tasks fixed\n")
	commitAll(t, dir, "[aaaa01] TASK-1: address review feedback")
	writeFile(t, dir, "docs/jobs/aaaa01_x/verdict.md", "APPROVED\n")
	commitAll(t, dir, "[aaaa01] verdict: approved")

	got, err := CountVerdictCommits(dir, "feature/aaaa01_x", "aaaa01")
	if err != nil {
		t.Fatalf("CountVerdictCommits: %v", err)
	}
	if got != 2 {
		t.Errorf("CountVerdictCommits = %d, want 2", got)
	}

	// A different job's verdict commits on the same branch must not count.
	gotOther, err := CountVerdictCommits(dir, "feature/aaaa01_x", "zzzz99")
	if err != nil {
		t.Fatalf("CountVerdictCommits (other id): %v", err)
	}
	if gotOther != 0 {
		t.Errorf("CountVerdictCommits for a different job id = %d, want 0", gotOther)
	}

	runGit(t, dir, "checkout", "-q", def)
}

func TestCountVerdictCommitsZero(t *testing.T) {
	dir, def := initRepo(t)
	got, err := CountVerdictCommits(dir, def, "aaaa01")
	if err != nil {
		t.Fatalf("CountVerdictCommits on a branch with no verdict commits: %v", err)
	}
	if got != 0 {
		t.Errorf("CountVerdictCommits = %d, want 0", got)
	}
}

func TestCountVerdictCommitsIgnoresUnparseableMessage(t *testing.T) {
	dir, def := initRepo(t)
	writeFile(t, dir, "docs/jobs/aaaa01_x/verdict.md", "note\n")
	// Mentions "verdict" but doesn't follow the exact "[ID] verdict:" convention.
	commitAll(t, dir, "verdict was written by hand, sorry")
	commitAll2 := func(msg string) {
		writeFile(t, dir, "docs/jobs/aaaa01_x/verdict.md", msg+"\n")
		commitAll(t, dir, msg)
	}
	commitAll2("[aaaa0] verdict: wrong id length, should not match")

	got, err := CountVerdictCommits(dir, def, "aaaa01")
	if err != nil {
		t.Fatalf("CountVerdictCommits: %v", err)
	}
	if got != 0 {
		t.Errorf("CountVerdictCommits with only unparseable/mismatched messages = %d, want 0", got)
	}
}

func TestCountVerdictCommitsMissingBranch(t *testing.T) {
	dir, _ := initRepo(t)
	got, err := CountVerdictCommits(dir, "no-such-branch", "aaaa01")
	if err != nil {
		t.Fatalf("CountVerdictCommits on a missing branch: unexpected error %v", err)
	}
	if got != 0 {
		t.Errorf("CountVerdictCommits on a missing branch = %d, want 0", got)
	}
}

func TestCountVerdictCommitsNotARepo(t *testing.T) {
	_, err := CountVerdictCommits(t.TempDir(), "main", "aaaa01")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("CountVerdictCommits on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestLatestCommitIsVerdict(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/aaaa01_x")
	runGit(t, dir, "checkout", "-q", "feature/aaaa01_x")
	writeFile(t, dir, "docs/jobs/aaaa01_x/implementation.md", "impl\n")
	commitAll(t, dir, "[aaaa01] implementation: add summary")

	writeFile(t, dir, "docs/jobs/aaaa01_x/verdict.md", "NEEDS WORK\n")
	commitAll(t, dir, "[aaaa01] verdict: needs work on TASK-1")

	// Tip is the verdict commit — developer hasn't responded yet.
	got, err := LatestCommitIsVerdict(dir, "feature/aaaa01_x", "aaaa01")
	if err != nil {
		t.Fatalf("LatestCommitIsVerdict: %v", err)
	}
	if !got {
		t.Errorf("LatestCommitIsVerdict = false, want true (tip is the verdict commit)")
	}

	// A different job's id must not match this branch's verdict commit.
	gotOther, err := LatestCommitIsVerdict(dir, "feature/aaaa01_x", "zzzz99")
	if err != nil {
		t.Fatalf("LatestCommitIsVerdict (other id): %v", err)
	}
	if gotOther {
		t.Errorf("LatestCommitIsVerdict for a different job id = true, want false")
	}

	// Developer commits a fix on top — tip is no longer the verdict commit.
	writeFile(t, dir, "docs/jobs/aaaa01_x/implementation.md", "impl fixed\n")
	commitAll(t, dir, "[aaaa01] TASK-1: address review feedback")

	got2, err := LatestCommitIsVerdict(dir, "feature/aaaa01_x", "aaaa01")
	if err != nil {
		t.Fatalf("LatestCommitIsVerdict after fix commit: %v", err)
	}
	if got2 {
		t.Errorf("LatestCommitIsVerdict after a fix commit = true, want false")
	}

	runGit(t, dir, "checkout", "-q", def)
}

func TestLatestCommitIsVerdictNoMatchingCommits(t *testing.T) {
	dir, def := initRepo(t)
	got, err := LatestCommitIsVerdict(dir, def, "aaaa01")
	if err != nil {
		t.Fatalf("LatestCommitIsVerdict: %v", err)
	}
	if got {
		t.Errorf("LatestCommitIsVerdict on a branch with no verdict commit = true, want false")
	}
}

func TestLatestCommitIsVerdictMissingBranch(t *testing.T) {
	dir, _ := initRepo(t)
	got, err := LatestCommitIsVerdict(dir, "no-such-branch", "aaaa01")
	if err != nil {
		t.Fatalf("LatestCommitIsVerdict on a missing branch: unexpected error %v", err)
	}
	if got {
		t.Errorf("LatestCommitIsVerdict on a missing branch = true, want false")
	}
}

func TestLatestCommitIsVerdictNotARepo(t *testing.T) {
	_, err := LatestCommitIsVerdict(t.TempDir(), "main", "aaaa01")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("LatestCommitIsVerdict on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestHeadCommit(t *testing.T) {
	dir, def := initRepo(t)
	want, err := exec.Command("git", "-C", dir, "rev-parse", def).Output()
	if err != nil {
		t.Fatal(err)
	}
	got, err := HeadCommit(dir, def)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Errorf("HeadCommit = %q, want %q", got, strings.TrimSpace(string(want)))
	}

	// A new commit changes what HeadCommit reports.
	writeFile(t, dir, "extra.txt", "more\n")
	commitAll(t, dir, "extra")
	got2, err := HeadCommit(dir, def)
	if err != nil {
		t.Fatalf("HeadCommit after new commit: %v", err)
	}
	if got2 == got {
		t.Errorf("HeadCommit did not change after a new commit")
	}
}

func TestHeadCommitMissingBranch(t *testing.T) {
	dir, _ := initRepo(t)
	got, err := HeadCommit(dir, "no-such-branch")
	if err != nil {
		t.Fatalf("HeadCommit on a missing branch: unexpected error %v", err)
	}
	if got != "" {
		t.Errorf("HeadCommit on a missing branch = %q, want empty", got)
	}
}

func TestHeadCommitNotARepo(t *testing.T) {
	_, err := HeadCommit(t.TempDir(), "main")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("HeadCommit on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestWorktreeForBranch(t *testing.T) {
	dir, _ := initRepo(t)
	wtDir := t.TempDir()
	// git refuses to add a worktree into a non-empty directory; TempDir()
	// itself already exists, so give it a fresh subdirectory instead.
	wtPath := filepath.Join(wtDir, "job-a")
	runGit(t, dir, "worktree", "add", wtPath, "-b", "feature/aaaa01_a")

	got, ok, err := WorktreeForBranch(dir, "feature/aaaa01_a")
	if err != nil {
		t.Fatalf("WorktreeForBranch: %v", err)
	}
	if !ok {
		t.Fatal("WorktreeForBranch: ok = false, want true")
	}
	// Resolve symlinks on both sides (e.g. /tmp vs /private/tmp on macOS)
	// before comparing — a real worktree path comparison needs it.
	wantPath, err := filepath.EvalSymlinks(wtPath)
	if err != nil {
		t.Fatal(err)
	}
	gotPath, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", got, err)
	}
	if gotPath != wantPath {
		t.Errorf("WorktreeForBranch path = %q, want %q", gotPath, wantPath)
	}
}

func TestWorktreeForBranchNoWorktree(t *testing.T) {
	dir, _ := initRepo(t)
	runGit(t, dir, "branch", "feature/aaaa01_a")

	_, ok, err := WorktreeForBranch(dir, "feature/aaaa01_a")
	if err != nil {
		t.Fatalf("WorktreeForBranch: %v", err)
	}
	if ok {
		t.Error("WorktreeForBranch on a branch with no worktree: ok = true, want false")
	}
}

func TestWorktreeForBranchNoCrossMatchOnPrefix(t *testing.T) {
	dir, _ := initRepo(t)
	wtDir := t.TempDir()
	wtPath := filepath.Join(wtDir, "job-xy")
	// "feature/x" is a prefix of "feature/x-y" — only the latter gets a
	// worktree, and looking up the former must not match it.
	runGit(t, dir, "worktree", "add", wtPath, "-b", "feature/x-y")

	_, ok, err := WorktreeForBranch(dir, "feature/x")
	if err != nil {
		t.Fatalf("WorktreeForBranch: %v", err)
	}
	if ok {
		t.Error("WorktreeForBranch(\"feature/x\") matched \"feature/x-y\"'s worktree, want no match")
	}
}

func TestWorktreeForBranchMissingBranch(t *testing.T) {
	dir, _ := initRepo(t)
	_, ok, err := WorktreeForBranch(dir, "no-such-branch")
	if err != nil {
		t.Fatalf("WorktreeForBranch on a missing branch: unexpected error %v", err)
	}
	if ok {
		t.Error("WorktreeForBranch on a missing branch: ok = true, want false")
	}
}

func TestWorktreeForBranchNotARepo(t *testing.T) {
	_, ok, err := WorktreeForBranch(t.TempDir(), "main")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("WorktreeForBranch on non-repo: err = %v, want ErrNotARepo", err)
	}
	if ok {
		t.Error("WorktreeForBranch on non-repo: ok = true, want false")
	}
}

func TestCheckoutFailureSurfacesGitError(t *testing.T) {
	dir, _ := initRepo(t)
	// Uncommitted changes that conflict with the target branch's file.
	writeFile(t, dir, "README", "dirty\n")
	err := Checkout(dir, "no-such-branch")
	if err == nil {
		t.Fatal("Checkout of a missing branch: expected an error, got nil")
	}
	if errors.Is(err, ErrNotARepo) {
		t.Errorf("Checkout of a missing branch misclassified as ErrNotARepo: %v", err)
	}
}
