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

func TestListJobDirs(t *testing.T) {
	dir, def := initRepo(t)
	writeFile(t, dir, "docs/jobs/aaaa01_a/brief.md", "# a\n")
	writeFile(t, dir, "docs/jobs/bbbb02_b/brief.md", "# b\n")
	writeFile(t, dir, "docs/jobs/archive/zzz_archived/brief.md", "# z\n")
	writeFile(t, dir, "docs/jobs/README.md", "not a job")
	commitAll(t, dir, "jobs")

	got, err := ListJobDirs(dir, def)
	if err != nil {
		t.Fatalf("ListJobDirs: %v", err)
	}
	sort.Strings(got)
	want := []string{"aaaa01_a", "bbbb02_b"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ListJobDirs = %v, want %v", got, want)
	}
}

func TestListJobDirsNoJobsDir(t *testing.T) {
	dir, def := initRepo(t)
	// No docs/jobs at all on the branch.
	got, err := ListJobDirs(dir, def)
	if err != nil {
		t.Errorf("ListJobDirs with no docs/jobs: unexpected error %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListJobDirs with no docs/jobs = %v, want empty", got)
	}
}

func TestListJobDirsOnBranchWithoutJobs(t *testing.T) {
	dir, def := initRepo(t)
	// Branch off BEFORE any job exists, so the branch genuinely has no
	// docs/jobs of its own (a branch created after the job would inherit it).
	runGit(t, dir, "branch", "empty")
	writeFile(t, dir, "docs/jobs/aaaa01_a/brief.md", "# a\n")
	commitAll(t, dir, "jobs on default")

	got, err := ListJobDirs(dir, "empty")
	if err != nil {
		t.Errorf("ListJobDirs on branch without jobs: unexpected error %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListJobDirs on branch without jobs = %v, want empty", got)
	}
	// And confirm the default branch still lists it.
	gotDef, _ := ListJobDirs(dir, def)
	if len(gotDef) != 1 || gotDef[0] != "aaaa01_a" {
		t.Errorf("ListJobDirs on default = %v, want [aaaa01_a]", gotDef)
	}
}

func TestListJobDirsNotARepo(t *testing.T) {
	_, err := ListJobDirs(t.TempDir(), "main")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("ListJobDirs on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestShowFile(t *testing.T) {
	dir, def := initRepo(t)
	const body = "# Brief: X\n\nstatus: open\nid: aaaa01\n"
	writeFile(t, dir, "docs/jobs/aaaa01_a/brief.md", body)
	commitAll(t, dir, "job")

	got, err := ShowFile(dir, def, "docs/jobs/aaaa01_a/brief.md")
	if err != nil {
		t.Fatalf("ShowFile: %v", err)
	}
	if string(got) != body {
		t.Errorf("ShowFile = %q, want %q", string(got), body)
	}
}

func TestShowFileReflectsBranchContents(t *testing.T) {
	dir, def := initRepo(t)
	writeFile(t, dir, "docs/jobs/aaaa01_a/brief.md", "on default\n")
	commitAll(t, dir, "default job")

	// Create a branch where the file has different content.
	runGit(t, dir, "branch", "feature/aaa")
	runGit(t, dir, "checkout", "-q", "feature/aaa")
	writeFile(t, dir, "docs/jobs/aaaa01_a/brief.md", "on feature\n")
	commitAll(t, dir, "feature edit")
	runGit(t, dir, "checkout", "-q", def)

	gotDef, _ := ShowFile(dir, def, "docs/jobs/aaaa01_a/brief.md")
	if string(gotDef) != "on default\n" {
		t.Errorf("ShowFile(default) = %q, want %q", string(gotDef), "on default\n")
	}
	gotFea, _ := ShowFile(dir, "feature/aaa", "docs/jobs/aaaa01_a/brief.md")
	if string(gotFea) != "on feature\n" {
		t.Errorf("ShowFile(feature) = %q, want %q", string(gotFea), "on feature\n")
	}
}

func TestShowFileMissingPath(t *testing.T) {
	dir, def := initRepo(t)
	// initRepo already made the initial commit, so docs/jobs simply doesn't
	// exist on the branch yet.
	_, err := ShowFile(dir, def, "docs/jobs/nope/brief.md")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ShowFile on missing path: err = %v, want os.ErrNotExist", err)
	}
}

func TestShowFileMissingBranch(t *testing.T) {
	dir, _ := initRepo(t)
	_, err := ShowFile(dir, "no-such-branch", "docs/jobs/x/brief.md")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ShowFile on missing branch: err = %v, want os.ErrNotExist", err)
	}
}

func TestShowFileNotARepo(t *testing.T) {
	_, err := ShowFile(t.TempDir(), "main", "docs/jobs/x/brief.md")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("ShowFile on non-repo: err = %v, want ErrNotARepo", err)
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
