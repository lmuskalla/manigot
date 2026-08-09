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
