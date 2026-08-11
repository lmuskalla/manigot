package job

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the worktree-backed discovery path
// (207bfu_git-worktrees). They build real throwaway git repos (with real git
// worktrees) so the git package's exec path is exercised end-to-end.

// gitRun runs git -C dir args, failing the test on any error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// gitInitRepo creates a fresh repo with identity + an initial commit and
// returns (dir, defaultBranch).
func gitInitRepo(t *testing.T) (dir, defaultBranch string) {
	t.Helper()
	dir = t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "README")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	// Read the default branch back (don't assume "main" vs "master").
	b, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("read default branch: %v", err)
	}
	return dir, strings.TrimSpace(string(b))
}

// briefWith is a tiny helper to build a brief.md body with frontmatter.
func briefWith(title, id, branch, date string) string {
	return "# Brief: " + title + "\n\n" +
		"status: open\ntype: feature\nid: " + id + "\nbranch: " + branch + "\ndate: " + date + "\n"
}

// addJobWorktree creates a new worktree at <worktreesDir>/<name> on a new
// branch, writes docs/jobs/<name>/brief.md inside it, and commits it there —
// mirroring what scripts/new-job.sh does (git worktree add + scaffold
// commit), so tests exercise the exact shape Discover expects.
func addJobWorktree(t *testing.T, repoDir, worktreesDir, branch, name, brief string) (worktreePath string) {
	t.Helper()
	worktreePath = filepath.Join(worktreesDir, name)
	gitRun(t, repoDir, "worktree", "add", worktreePath, "-b", branch)

	jobDir := filepath.Join(worktreePath, "docs", "jobs", name)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreePath, "add", "-A")
	gitRun(t, worktreePath, "commit", "-q", "-m", "job "+name)
	return worktreePath
}

// TestDiscoverListsJobsAcrossWorktrees is the worktree-era version of the
// original cross-branch brief's symptom check: create a job in its own
// worktree for three separate branches, leave the main worktree on the base
// branch (which has no jobs of its own), and confirm Discover still lists
// all three.
func TestDiscoverListsJobsAcrossWorktrees(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()

	addJobWorktree(t, dir, wts, "feature/aaa01_a", "aaa01_a", briefWith("Aaa", "aaa01", "feature/aaa01_a", "2026-01-01"))
	addJobWorktree(t, dir, wts, "fix/bbb02_b", "bbb02_b", briefWith("Bbb", "bbb02", "fix/bbb02_b", "2026-02-02"))
	addJobWorktree(t, dir, wts, "chore/ccc03_c", "ccc03_c", briefWith("Ccc", "ccc03", "chore/ccc03_c", "2026-03-03"))

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("len(jobs) = %d, want 3; got %+v", len(jobs), jobs)
	}

	byID := map[string]Job{}
	for _, j := range jobs {
		byID[j.ID] = j
	}
	for _, want := range []string{"aaa01", "bbb02", "ccc03"} {
		if _, ok := byID[want]; !ok {
			t.Errorf("job %q missing from Discover result: %+v", want, jobs)
		}
	}
}

// TestDiscoverMainWorktreeBaseBranchContributesNothing confirms the
// steady-state case: when the main worktree sits on the base branch (as it
// does once every open job lives in its own worktree), scanning it adds
// nothing — its docs/jobs/ holds only archive/ (excluded) — so only the job
// worktrees' jobs are listed. The main worktree is scanned like any other
// worktree now (see Discover's doc), it just has nothing to contribute here.
func TestDiscoverMainWorktreeBaseBranchContributesNothing(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaa01_a", "aaa01_a", briefWith("Aaa", "aaa01", "feature/aaa01_a", "2026-01-01"))

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1 (only the job worktree's job); got %+v", len(jobs), jobs)
	}
}

// TestDiscoverListsTransitionalMainWorktreeJob confirms a pre-worktree job —
// one whose branch is checked out in the *main* worktree rather than in its
// own worktree (the transitional case this very job is in: /workspace is the
// only worktree, on feature/207bfu_git-worktrees) — IS listed, attributed to
// the main worktree's checked-out branch. This is what keeps the TUI and
// mg-jdi working on such a job until it is finished or migrated.
func TestDiscoverListsTransitionalMainWorktreeJob(t *testing.T) {
	dir, _ := gitInitRepo(t)

	// Check out a job branch in the main worktree (no separate worktree),
	// write its docs/jobs/<name>/brief.md, and commit — exactly the
	// pre-worktree job layout this change itself ships in.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/aaa01_a")
	jobDir := filepath.Join(dir, "docs", "jobs", "aaa01_a")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(briefWith("Aaa", "aaa01", "feature/aaa01_a", "2026-01-01")), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "job aaa01_a")

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1 (the transitional main-worktree job); got %+v", len(jobs), jobs)
	}
	if jobs[0].ID != "aaa01" || jobs[0].Branch != "feature/aaa01_a" {
		t.Errorf("Discover = %+v, want job aaa01 attributed to branch feature/aaa01_a", jobs)
	}
}

// TestDiscoverIgnoresNonJobDirsInMainWorktree confirms the main worktree's
// docs/jobs/ content that is not a job — the .jdi-status sidecar directory,
// or a stray empty directory — is not mislisted as a job. The pre-worktree
// enumeration never saw these (git ls-tree only lists tracked content); the
// brief.md requirement reproduces that for the disk-based worktree scan.
func TestDiscoverIgnoresNonJobDirsInMainWorktree(t *testing.T) {
	dir, _ := gitInitRepo(t)

	// Main worktree on a job branch (transitional layout), with the job plus
	// a .jdi-status sidecar dir and a stray empty dir sitting next to it.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/aaa01_a")
	jobDir := filepath.Join(dir, "docs", "jobs", "aaa01_a")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(briefWith("Aaa", "aaa01", "feature/aaa01_a", "2026-01-01")), 0o644)
	os.MkdirAll(filepath.Join(dir, "docs", "jobs", ".jdi-status", "aaa01_a"), 0o755)
	os.MkdirAll(filepath.Join(dir, "docs", "jobs", "stray_empty_dir"), 0o755)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "job aaa01_a + sidecar + stray")

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1 (sidecar and stray dirs are not jobs); got %+v", len(jobs), jobs)
	}
	if jobs[0].ID != "aaa01" {
		t.Errorf("Discover = %+v, want only job aaa01", jobs)
	}
}

// TestDiscoverBranchPopulated confirms each Job.Branch is the worktree's own
// branch.
func TestDiscoverBranchPopulated(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/ggg02_g", "ggg02_g", briefWith("Ggg", "ggg02", "feature/ggg02_g", "2026-02-02"))

	jobs, _ := Discover(dir)
	if len(jobs) != 1 || jobs[0].Branch != "feature/ggg02_g" {
		t.Errorf("Discover = %+v, want one job with Branch feature/ggg02_g", jobs)
	}
}

// TestDiscoverReadsWorktreeWorkingTree confirms a job's brief reflects its
// own worktree's working tree, so an uncommitted edit there shows up without
// a commit — the same "no branch check needed" guarantee TASK-7 gives every
// caller of Job.Dir.
func TestDiscoverReadsWorktreeWorkingTree(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/hhh01_h", "hhh01_h", briefWith("Old", "hhh01", "feature/hhh01_h", "2026-01-01"))

	briefPath := filepath.Join(wtPath, "docs", "jobs", "hhh01_h", "brief.md")
	if err := os.WriteFile(briefPath, []byte(briefWith("NewUncommitted", "hhh01", "feature/hhh01_h", "2026-01-01")), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := Discover(dir)
	var h Job
	for _, j := range jobs {
		if j.ID == "hhh01" {
			h = j
		}
	}
	if h.Title != "NewUncommitted" {
		t.Errorf("job Title = %q, want %q (uncommitted worktree edit must show)", h.Title, "NewUncommitted")
	}
}

// TestDiscoverExcludesArchive confirms a job worktree's own archive/
// subdirectory is excluded, mirroring the working-tree fallback's behaviour.
func TestDiscoverExcludesArchive(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/jjj01_j", "jjj01_j", briefWith("Jjj", "jjj01", "feature/jjj01_j", "2026-01-01"))

	archDir := filepath.Join(wtPath, "docs", "jobs", "archive", "zzz_arch")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archDir, "brief.md"), []byte(briefWith("Archived", "zzzarc", "feature/jjj01_j", "2020-01-01")), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := Discover(dir)
	for _, j := range jobs {
		if strings.Contains(j.Name, "archive") || j.ID == "zzzarc" {
			t.Errorf("archive/ job leaked into results: %+v", j)
		}
	}
}

// TestDiscoverFreshRepoFallsBack confirms a repo with no commits (no branch
// refs yet, so no worktrees are even possible) degrades to the working-tree
// enumeration rather than failing.
func TestDiscoverFreshRepoFallsBack(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")

	// Create a job directly in the working tree (unborn branch, no commits).
	jd := filepath.Join(dir, "docs", "jobs", "kkk01_k")
	os.MkdirAll(jd, 0o755)
	os.WriteFile(filepath.Join(jd, "brief.md"), []byte(briefWith("Kkk", "kkk01", "", "2026-01-01")), 0o644)

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover on unborn repo: unexpected error %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "kkk01" {
		t.Errorf("Discover on unborn repo = %+v, want one job kkk01", jobs)
	}
}

// TestDiscoverSortsByDateDesc confirms the worktree-backed path keeps the
// same date-desc / name-tiebreak sort the working-tree path has always used.
func TestDiscoverSortsByDateDesc(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/older", "aaa01_a", briefWith("Older", "aaa01", "feature/older", "2025-01-01"))
	addJobWorktree(t, dir, wts, "feature/newer", "bbb02_b", briefWith("Newer", "bbb02", "feature/newer", "2026-12-31"))

	jobs, _ := Discover(dir)
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}
	if jobs[0].ID != "bbb02" {
		t.Errorf("jobs[0].ID = %q, want bbb02 (newest first)", jobs[0].ID)
	}
	if jobs[1].ID != "aaa01" {
		t.Errorf("jobs[1].ID = %q, want aaa01", jobs[1].ID)
	}
}

// TestDiscoverIsReadOnly confirms Discover never switches the checked-out
// branch in any worktree as a side effect.
func TestDiscoverIsReadOnly(t *testing.T) {
	dir, def := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/xyz01_x", "xyz01_x", briefWith("Xyz", "xyz01", "feature/xyz01_x", "2026-01-01"))

	beforeCur := strings.TrimSpace(func() string {
		out, _ := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
		return string(out)
	}())

	if _, err := Discover(dir); err != nil {
		t.Fatalf("Discover: %v", err)
	}

	afterCur := strings.TrimSpace(func() string {
		out, _ := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
		return string(out)
	}())
	if afterCur != beforeCur || afterCur != def {
		t.Errorf("Discover changed the main worktree's checked-out branch: %q → %q", beforeCur, afterCur)
	}
}
