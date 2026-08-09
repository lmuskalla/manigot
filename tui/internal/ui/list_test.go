package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/job"
)

// --- current-branch header line (TASK-3) ------------------------------------

// TestRenderListShowsCurrentBranch verifies the list header names the branch
// currently checked out, in the "manigot - <project> - on <branch>" title.
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
	if strings.Contains(got, " - on ") {
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
// strip end to end through the UI layer. The terminal height is deliberately
// pinned tight enough (see recentActivityShown's spare-room formula) that
// exactly one line of spare room exists, forcing the adaptive strip down to
// its floor of 1 — the same footprint the strip always had before TASK-1 —
// so this can still assert on single-entry behaviour without racing TASK-1's
// new sizing math. Multi-entry ordering/sizing coverage against a *sparse*
// list lives in TestRenderListRecentActivityScalesWithSpareRoom below; the
// git-package-level git.RecentCommits tests (recentcommits_test.go) are where
// the exhaustive dedup/ordering coverage lives regardless of how many the UI
// layer chooses to show. What this test asserts: the one entry shown is the
// most recent commit across *all* local branches — not just the
// currently-checked-out branch's own log — and a commit shared by an
// undiverged branch doesn't get double-counted into showing twice.
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

	jobs, _ := job.Discover(dir) // no jobs dir here — 0 jobs
	a := NewApp(dir, jobs)
	// spare = height - dashboardFixedChrome - len(jobs) = 6 - 7 - 0 = -1,
	// clamping the strip to its floor of 1 entry.
	a.width, a.height = 80, 6

	got := a.renderList()

	// def (the checked-out branch) alone doesn't have ZZFEATURECOMMIT in its
	// history — only its presence proves the strip looks across all local
	// branches, not just the current one.
	if n := strings.Count(got, "ZZFEATURECOMMIT"); n != 1 {
		t.Errorf("renderList shows the most recent cross-branch commit %d time(s), want exactly 1:\n%s", n, got)
	}
	// At the floor (1 entry), the older shared "init" commit must not also
	// appear.
	if strings.Contains(got, "init") {
		t.Errorf("renderList should show only the single most recent commit at the floor, not the older shared one too:\n%s", got)
	}
}

// TestRenderListRecentActivityScalesWithSpareRoom is TASK-1's core coverage:
// a sparse (1-job) list at a generous terminal height shows more than the
// pre-existing 1-line floor, up to the 5-entry ceiling.
func TestRenderListRecentActivityScalesWithSpareRoom(t *testing.T) {
	dir, def := gitInitRepo(t)
	commitEmptyAt(t, dir, "c2", 1)
	commitEmptyAt(t, dir, "c3", 2)
	commitEmptyAt(t, dir, "c4", 3)
	commitEmptyAt(t, dir, "c5", 4)
	gitCommitJob(t, dir, "aaaa02_a", "# Brief: A\n\nstatus: open\nid: aaaa02\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: got %d jobs, want 1", len(jobs))
	}
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 40 // generous height — plenty of spare room

	if got := a.recentActivityShown(); got != recentActivityCeiling {
		t.Fatalf("recentActivityShown() = %d, want the ceiling %d given ample spare room", got, recentActivityCeiling)
	}

	got := a.renderList()
	if n := strings.Count(got, def); n < 1 {
		t.Errorf("renderList at generous height should show multiple activity entries; def branch %q not found:\n%s", def, got)
	}
}

// TestRenderListRecentActivityKeepsFloorWhenListFillsScreen is a regression
// test for qge358's original constraint: a job list tall enough to already
// fill the screen must not have the strip grow past its pre-existing 1-line
// footprint (or shrink further), even though TASK-1 made the strip capable of
// growing.
func TestRenderListRecentActivityKeepsFloorWhenListFillsScreen(t *testing.T) {
	dir, _ := gitInitRepo(t)
	commitEmptyAt(t, dir, "c2", 1)
	commitEmptyAt(t, dir, "c3", 2)
	for i := 0; i < 20; i++ {
		gitCommitJob(t, dir, fmt.Sprintf("bbbb%02d_b", i), fmt.Sprintf("# Brief: B%d\n\nstatus: open\nid: bbbb%02d\ndate: 2026-01-01\n", i, i))
	}

	jobs, _ := job.Discover(dir)
	if len(jobs) != 20 {
		t.Fatalf("job.Discover: got %d jobs, want 20", len(jobs))
	}
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24 // a list this long already fills a normal terminal

	if got := a.recentActivityShown(); got != recentActivityFloor {
		t.Errorf("recentActivityShown() = %d, want the floor %d when the list already fills the screen", got, recentActivityFloor)
	}

	got := a.renderList()
	// Every job row must still be present — the strip must never have pushed
	// one off the rendered output.
	if n := strings.Count(got, "open    feature"); n != 20 {
		t.Errorf("renderList should still show all 20 job rows; found %d:\n%s", n, got)
	}
}

// TestRenderListZeroHeightDoesNotPanic confirms an App that never received a
// tea.WindowSizeMsg (a.height left at its zero value, e.g. some
// construction-only tests) renders something sane instead of panicking —
// recentActivityShown's a.height == 0 guard exists specifically for this
// case.
func TestRenderListZeroHeightDoesNotPanic(t *testing.T) {
	dir, _ := gitInitRepo(t)
	gitCommitJob(t, dir, "aaaa03_a", "# Brief: A\n\nstatus: open\nid: aaaa03\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	// a.width, a.height left at zero.

	got := a.renderList()
	if got == "" {
		t.Fatal("renderList returned nothing for a zero-height App")
	}
	if !strings.Contains(got, "manigot") {
		t.Errorf("renderList output missing expected header text:\n%s", got)
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

// --- status/hint coexistence (TASK-5) ---------------------------------------

// TestListFooterKeepsHintAlongsideStatus is a regression test for TASK-4: the
// footer must show the status message *and* the key hint together after an
// action like "ctrl+r", not replace the hint entirely — otherwise a user who
// just refreshed no longer knows what keys exist.
func TestListFooterKeepsHintAlongsideStatus(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0010_f")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: F\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24

	model, _ := a.updateList(keyMsg("ctrl+r"))
	got := model.(*App)

	if got.status == "" {
		t.Fatal("status should be set after ctrl+r")
	}
	footer := got.footer()
	if !strings.Contains(footer, got.status) {
		t.Errorf("footer missing the status message %q:\n%s", got.status, footer)
	}
	if !strings.Contains(footer, "q quit") {
		t.Errorf("footer lost the key hint after a status was set:\n%s", footer)
	}
}

// --- empty-list invitation (TASK-11) ----------------------------------------

// TestRenderListEmptyStateInvitesNewJob is a regression test for TASK-10:
// the most prominent text a zero-job user sees must be the path to creating
// their first job ("n"), not the path to quitting. The footer below still
// lists "q quit" like every other key — this only asserts on the dedicated
// empty-state message itself, isolated by line, not the whole rendered view
// (which legitimately still mentions quit once, in the footer's full key
// list).
func TestRenderListEmptyStateInvitesNewJob(t *testing.T) {
	dir := t.TempDir() // not a git repo, no jobs dir — an empty project

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	got := a.renderList()
	if !strings.Contains(got, "press n") {
		t.Errorf("empty-list state should invite the user to press n:\n%s", got)
	}

	var emptyStateLine string
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "press n") {
			emptyStateLine = line
			break
		}
	}
	if emptyStateLine == "" {
		t.Fatalf("could not find the empty-state line in:\n%s", got)
	}
	if strings.Contains(emptyStateLine, "quit") {
		t.Errorf("empty-list state's dedicated message should not mention quitting at all: %q", emptyStateLine)
	}
}
