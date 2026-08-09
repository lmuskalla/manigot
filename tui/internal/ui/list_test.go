package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/lmuskalla/safecode/tui/internal/job"
)

// --- current-branch header line (TASK-3) ------------------------------------

// TestRenderListShowsCurrentBranch verifies the list header names the branch
// currently checked out, next to the "jobs in <root>" text.
func TestRenderListShowsCurrentBranch(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "aaaa01_a", "# Brief: A\n\nstatus: open\nid: aaaa01\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	got := a.renderList()
	if !strings.Contains(got, def) {
		t.Errorf("renderList output does not mention the current branch %q:\n%s", def, got)
	}
}

// TestRenderListOmitsBranchOnNonRepo verifies a project that isn't a git
// repository (job.Discover's working-tree-only fallback, currentBranch =
// "") renders no branch label at all rather than an empty/awkward one.
func TestRenderListOmitsBranchOnNonRepo(t *testing.T) {
	dir := t.TempDir() // not a git repo
	jobDir := dir + "/docs/jobs/nob01_n"
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobDir+"/brief.md", []byte("# Brief: NoBranch\n\nstatus: open\ndate: 2026-01-01\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	if a.currentBranch != "" {
		t.Fatalf("currentBranch = %q on a non-repo project, want empty", a.currentBranch)
	}

	got := a.renderList()
	if strings.Contains(got, "· on ") {
		t.Errorf("renderList should render no branch tag on a non-repo project:\n%s", got)
	}
}

// --- recent-activity strip (TASK-5) -----------------------------------------

// commitEmptyAt makes an empty commit with a deterministic, strictly
// increasing timestamp — mirrors git.recentCommitsFieldSep's own test helper
// but lives here since it's package ui, not package git.
func commitEmptyAt(t *testing.T, dir, msg string, secs int) {
	t.Helper()
	date := fmt.Sprintf("2026-01-01T00:00:%02dZ", secs)
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-q", "-m", msg)
	cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit -m %q in %s: %v\n%s", msg, dir, err, out)
	}
}

// TestRenderListRecentActivityShowsMostRecentAcrossBranches exercises the
// strip end to end through the UI layer. recentActivityCount is 1 (the
// brief's own named fallback for keeping the header's footprint from growing
// past its pre-existing size — see the const's doc comment), so this can't
// assert on multi-entry ordering the way the git-package-level
// git.RecentCommits tests do (TASK-2, recentcommits_test.go, is where the
// exhaustive dedup/ordering coverage lives). What it does assert: the one
// entry shown is the most recent commit across *all* local branches — not
// just the currently checked-out branch's own log — and a commit shared by
// an undiverged branch doesn't get double-counted into showing twice.
// Every commit gets an explicit, strictly increasing timestamp (via
// commitEmptyAt) rather than relying on wall-clock spacing, which would tie
// (or worse, invert order relative to gitInitRepo's real-time "init" commit).
func TestRenderListRecentActivityShowsMostRecentAcrossBranches(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	commitEmptyAt(t, dir, "init", 0)
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("read default branch: %v", err)
	}
	def := strings.TrimSpace(string(out))

	// "shared" never diverges from def — its tip commit ("init") must only
	// ever be able to render once, not once per branch that can reach it.
	gitRun(t, dir, "branch", "shared")

	gitRun(t, dir, "checkout", "-q", "-b", "feature")
	commitEmptyAt(t, dir, "ZZFEATURECOMMIT", 5)
	gitRun(t, dir, "checkout", "-q", def)

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	got := a.renderList()

	// def (the checked-out branch) alone doesn't have ZZFEATURECOMMIT in its
	// history — only its presence proves the strip looks across all local
	// branches, not just the current one.
	if n := strings.Count(got, "ZZFEATURECOMMIT"); n != 1 {
		t.Errorf("renderList shows the most recent cross-branch commit %d time(s), want exactly 1:\n%s", n, got)
	}
	// With recentActivityCount == 1, the older shared "init" commit must not
	// also appear.
	if strings.Contains(got, "init") {
		t.Errorf("renderList should show only the single most recent commit, not the older shared one too:\n%s", got)
	}
}

// TestRenderListRecentActivityEmptyOnFreshRepo verifies the strip disappears
// gracefully (no heading, no blank placeholder lines) when the repo has no
// commits yet.
func TestRenderListRecentActivityEmptyOnFreshRepo(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	if len(a.recentCommits) != 0 {
		t.Fatalf("recentCommits on a fresh empty repo = %+v, want empty", a.recentCommits)
	}
	got := a.renderList()
	if got == "" {
		t.Fatal("renderList returned nothing")
	}
}
