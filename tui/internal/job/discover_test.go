package job

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the cross-branch, git-backed discovery path (TASK-2).
// They build real throwaway git repos so the git package's exec path is
// exercised end-to-end. The shared helpers live in git_test.go in the sibling
// package; here we re-declare a couple minimal ones to keep this package
// self-contained.

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

// gitCommitJob writes brief.md under docs/jobs/<name>/ and commits it.
func gitCommitJob(t *testing.T, dir, name, brief string) {
	t.Helper()
	jobDir := filepath.Join(dir, "docs", "jobs", name)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "job "+name)
}

// briefWith is a tiny helper to build a brief.md body with frontmatter.
func briefWith(title, id, branch, date string) string {
	return "# Brief: " + title + "\n\n" +
		"status: open\ntype: feature\nid: " + id + "\nbranch: " + branch + "\ndate: " + date + "\n"
}

// TestDiscoverListsJobsAcrossBranches is the brief's exact symptom: create a
// job on each of three separate feature branches, check out main (which has no
// jobs of its own), and confirm Discover still lists all three — where today's
// working-tree-only Discover lists zero.
func TestDiscoverListsJobsAcrossBranches(t *testing.T) {
	dir, def := gitInitRepo(t)

	// One job per feature branch, each branched off main so main itself has
	// no jobs. The currently-checked-out branch stays main throughout.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/aaa01_a")
	gitCommitJob(t, dir, "aaa01_a", briefWith("Aaa", "aaa01", "feature/aaa01_a", "2026-01-01"))
	gitRun(t, dir, "checkout", "-q", def)

	gitRun(t, dir, "checkout", "-q", "-b", "fix/bbb02_b")
	gitCommitJob(t, dir, "bbb02_b", briefWith("Bbb", "bbb02", "fix/bbb02_b", "2026-02-02"))
	gitRun(t, dir, "checkout", "-q", def)

	gitRun(t, dir, "checkout", "-q", "-b", "chore/ccc03_c")
	gitCommitJob(t, dir, "ccc03_c", briefWith("Ccc", "ccc03", "chore/ccc03_c", "2026-03-03"))
	gitRun(t, dir, "checkout", "-q", def)

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("len(jobs) = %d, want 3 (the brief's symptom was zero); got %+v", len(jobs), jobs)
	}

	byID := map[string]Job{}
	for _, j := range jobs {
		byID[j.ID] = j
	}
	for _, want := range []string{"aaa01", "bbb02", "ccc03"} {
		j, ok := byID[want]
		if !ok {
			t.Errorf("job %q missing from Discover result: %+v", want, jobs)
			continue
		}
		// None of these jobs live on the checked-out (default) branch.
		if j.OnCurrentBranch {
			t.Errorf("job %q OnCurrentBranch = true, want false (it lives on its own branch)", want)
		}
	}
}

// TestDiscoverOnCurrentBranchFlagged confirms the one job on the checked-out
// branch is flagged OnCurrentBranch, and the rest are not.
func TestDiscoverOnCurrentBranchFlagged(t *testing.T) {
	dir, def := gitInitRepo(t)

	// A job on the default branch (currently checked out).
	gitCommitJob(t, dir, "ddd01_d", briefWith("Ddd", "ddd01", def, "2026-01-01"))

	// A job on another branch.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/eee02_e")
	gitCommitJob(t, dir, "eee02_e", briefWith("Eee", "eee02", "feature/eee02_e", "2026-02-02"))
	gitRun(t, dir, "checkout", "-q", def)

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	byID := map[string]Job{}
	for _, j := range jobs {
		byID[j.ID] = j
	}
	if d, ok := byID["ddd01"]; !ok {
		t.Errorf("ddd01 missing: %+v", jobs)
	} else if !d.OnCurrentBranch {
		t.Errorf("ddd01 OnCurrentBranch = false, want true (it's on the checked-out %q)", def)
	}
	if e, ok := byID["eee02"]; !ok {
		t.Errorf("eee02 missing: %+v", jobs)
	} else if e.OnCurrentBranch {
		t.Errorf("eee02 OnCurrentBranch = true, want false")
	}
}

// TestDiscoverBranchPopulated confirms each Job.Branch is the discovery branch.
func TestDiscoverBranchPopulated(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "fff01_f", briefWith("Fff", "fff01", def, "2026-01-01"))
	gitRun(t, dir, "checkout", "-q", "-b", "feature/ggg02_g")
	gitCommitJob(t, dir, "ggg02_g", briefWith("Ggg", "ggg02", "feature/ggg02_g", "2026-02-02"))
	gitRun(t, dir, "checkout", "-q", def)

	jobs, _ := Discover(dir)
	byID := map[string]Job{}
	for _, j := range jobs {
		byID[j.ID] = j
	}
	if got := byID["fff01"].Branch; got != def {
		t.Errorf("fff01.Branch = %q, want %q", got, def)
	}
	if got := byID["ggg02"].Branch; got != "feature/ggg02_g" {
		t.Errorf("ggg02.Branch = %q, want feature/ggg02_g", got)
	}
}

// TestDiscoverCurrentBranchReadsWorkingTree confirms the brief's last edge
// case: a job on the currently-checked-out branch reflects the working tree,
// so uncommitted brief edits show up without a commit.
func TestDiscoverCurrentBranchReadsWorkingTree(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "hhh01_h", briefWith("Old", "hhh01", def, "2026-01-01"))

	// Edit the brief in the working tree WITHOUT committing.
	briefPath := filepath.Join(dir, "docs", "jobs", "hhh01_h", "brief.md")
	if err := os.WriteFile(briefPath, []byte(briefWith("NewUncommitted", "hhh01", def, "2026-01-01")), 0o644); err != nil {
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
		t.Errorf("current-branch job Title = %q, want %q (working-tree edit must show)", h.Title, "NewUncommitted")
	}
}

// TestDiscoverOtherBranchReadsCommitted confirms a job on a non-current branch
// is read via git show — i.e. it reflects the committed state, not whatever
// may or may not be in the working tree at its dir path.
func TestDiscoverOtherBranchReadsCommitted(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/iii01_i")
	gitCommitJob(t, dir, "iii01_i", briefWith("Iii", "iii01", "feature/iii01_i", "2026-01-01"))
	gitRun(t, dir, "checkout", "-q", def)

	// The job's dir doesn't exist on the default branch's working tree at
	// all — proving the brief was read from the other branch via git.
	if _, err := os.Stat(filepath.Join(dir, "docs", "jobs", "iii01_i")); !os.IsNotExist(err) {
		t.Fatalf("iii01_i unexpectedly present on default working tree: %v", err)
	}

	jobs, _ := Discover(dir)
	var i Job
	for _, j := range jobs {
		if j.ID == "iii01" {
			i = j
		}
	}
	if i.Title != "Iii" {
		t.Errorf("other-branch job Title = %q, want %q (read via git show)", i.Title, "Iii")
	}
}

// TestDiscoverExcludesArchive confirms the archive/ directory is excluded on
// every branch in the cross-branch path, mirroring the working-tree behaviour.
func TestDiscoverExcludesArchive(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "jjj01_j", briefWith("Jjj", "jjj01", def, "2026-01-01"))

	// An archived job under docs/jobs/archive/.
	archDir := filepath.Join(dir, "docs", "jobs", "archive", "zzz_arch")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archDir, "brief.md"), []byte(briefWith("Archived", "zzzarc", def, "2020-01-01")), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "archive an old job")

	jobs, _ := Discover(dir)
	for _, j := range jobs {
		if strings.Contains(j.Name, "archive") || j.ID == "zzzarc" {
			t.Errorf("archive/ job leaked into results: %+v", j)
		}
	}
}

// TestDiscoverFreshRepoFallsBack confirms a repo with no commits (no branch
// refs yet) degrades to the working-tree enumeration rather than failing.
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

// TestDiscoverSortsByDateDesc confirms the cross-branch path keeps the same
// date-desc / name-tiebreak sort the working-tree path has always used.
func TestDiscoverSortsByDateDesc(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/older")
	gitCommitJob(t, dir, "aaa01_a", briefWith("Older", "aaa01", "feature/older", "2025-01-01"))
	gitRun(t, dir, "checkout", "-q", def)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/newer")
	gitCommitJob(t, dir, "bbb02_b", briefWith("Newer", "bbb02", "feature/newer", "2026-12-31"))
	gitRun(t, dir, "checkout", "-q", def)

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

// --- dedup (TASK-3) ---------------------------------------------------------
//
// A job appears on more than one branch when one branch descends from a commit
// that already contained it (e.g. a stale branch after a merge). dedupByID
// collapses those copies by job ID, keeping the copy whose discovery branch
// equals the brief's own `branch:` frontmatter field — so the stale copy on
// the other branch is dropped and the job is listed once.

// TestDiscoverDedupsJobOnMultipleBranches: a job whose home branch is
// feature/x, but whose directory also exists on main (because main was
// branched from a commit that already had it), must be listed exactly once,
// attributed to feature/x.
func TestDiscoverDedupsJobOnMultipleBranches(t *testing.T) {
	dir, def := gitInitRepo(t)

	// Create the job on the default branch first, then branch off carrying
	// it, then delete it on the default branch — so the job's home is the
	// feature branch while a copy still lingers on... actually here it lingers
	// on the default branch. The brief's `branch:` field names the feature
	// branch, which is the authoritative one.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/abc01_a")
	gitCommitJob(t, dir, "abc01_a", briefWith("Abc", "abc01", "feature/abc01_a", "2026-01-01"))
	// Bring the job onto the default branch too (it's now on both).
	gitRun(t, dir, "checkout", "-q", def)
	gitRun(t, dir, "merge", "-q", "--no-ff", "-m", "merge feature", "feature/abc01_a")

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// Must be deduped to a single entry despite living on two branches.
	count := 0
	var got Job
	for _, j := range jobs {
		if j.ID == "abc01" {
			count++
			got = j
		}
	}
	if count != 1 {
		t.Errorf("abc01 appears %d times, want exactly 1 (deduped); jobs=%+v", count, jobs)
	}
	// The surviving copy must be the one on the brief's own branch.
	if got.Branch != "feature/abc01_a" {
		t.Errorf("deduped abc01.Branch = %q, want feature/abc01_a (the brief's own branch)", got.Branch)
	}
}

// TestDiscoverDedupsPrefersFrontmatterBranchAcrossStaleBranches: the brief's
// `branch:` field points at feature/x, but the job dir also exists on a stale
// branch (branched from the same commit, so it carries the job too). Dedup
// must keep the feature/x copy regardless of how many stale copies exist, and
// Discover must not switch branches as a side effect.
func TestDiscoverDedupsPrefersFrontmatterBranchAcrossStaleBranches(t *testing.T) {
	dir, def := gitInitRepo(t)

	// Job's home branch.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/xyz01_x")
	gitCommitJob(t, dir, "xyz01_x", briefWith("Xyz", "xyz01", "feature/xyz01_x", "2026-01-01"))
	// A stale branch that carries the same commit (job dir exists on both refs).
	gitRun(t, dir, "branch", "stale/xyz01_x")
	// Back to the default branch before discovering.
	gitRun(t, dir, "checkout", "-q", def)

	// Discover must not switch branches as a side effect.
	beforeCur := strings.TrimSpace(func() string {
		out, _ := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
		return string(out)
	}())

	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	afterCur := strings.TrimSpace(func() string {
		out, _ := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
		return string(out)
	}())
	if afterCur != beforeCur {
		t.Errorf("Discover changed the checked-out branch: %q → %q (must be a read-only operation)", beforeCur, afterCur)
	}

	count := 0
	var got Job
	for _, j := range jobs {
		if j.ID == "xyz01" {
			count++
			got = j
		}
	}
	if count != 1 {
		t.Errorf("xyz01 appears %d times, want exactly 1; jobs=%+v", count, jobs)
	}
	// Two branches carry it (feature/xyz01_x and stale/xyz01_x); dedup keeps
	// the frontmatter one.
	if got.Branch != "feature/xyz01_x" {
		t.Errorf("deduped xyz01.Branch = %q, want feature/xyz01_x", got.Branch)
	}
}

// TestDedupByIDTable exercises dedupByID directly with constructed Job slices,
// covering the table of edge cases without the git scaffolding overhead.
func TestDedupByIDTable(t *testing.T) {
	// Helper to make a job copy.
	mk := func(id, branch, frontmatterBranch string) Job {
		return Job{ID: id, Branch: branch, briefBranch: frontmatterBranch}
	}

	tests := []struct {
		name string
		in   []Job
		want []Job // by (ID, Branch)
	}{
		{
			name: "no duplicates passes through",
			in:   []Job{mk("a", "feature/a", "feature/a"), mk("b", "feature/b", "feature/b")},
			want: []Job{{ID: "a", Branch: "feature/a"}, {ID: "b", Branch: "feature/b"}},
		},
		{
			name: "duplicate keeps frontmatter-branch copy",
			in: []Job{
				mk("a", "main", "feature/a"),      // stale copy on main
				mk("a", "feature/a", "feature/a"), // home copy
			},
			want: []Job{{ID: "a", Branch: "feature/a"}},
		},
		{
			name: "duplicate order does not matter",
			in: []Job{
				mk("a", "feature/a", "feature/a"), // home copy first
				mk("a", "main", "feature/a"),      // stale copy second
			},
			want: []Job{{ID: "a", Branch: "feature/a"}},
		},
		{
			name: "no frontmatter branch keeps first-seen",
			in: []Job{
				mk("a", "main", ""), // no authoritative branch
				mk("a", "feature/a", ""),
			},
			want: []Job{{ID: "a", Branch: "main"}},
		},
		{
			name: "three copies, one on the home branch",
			in: []Job{
				mk("a", "main", "feature/a"),
				mk("a", "stale", "feature/a"),
				mk("a", "feature/a", "feature/a"),
			},
			want: []Job{{ID: "a", Branch: "feature/a"}},
		},
		{
			name: "empty and single inputs are no-ops",
			in:   []Job{mk("a", "feature/a", "feature/a")},
			want: []Job{{ID: "a", Branch: "feature/a"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupByID(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("dedupByID len = %d, want %d (got %+v)", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i].ID != tc.want[i].ID || got[i].Branch != tc.want[i].Branch {
					t.Errorf("dedupByID[%d] = {ID:%q Branch:%q}, want {ID:%q Branch:%q}",
						i, got[i].ID, got[i].Branch, tc.want[i].ID, tc.want[i].Branch)
				}
			}
		})
	}
}
