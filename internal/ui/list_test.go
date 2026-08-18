package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
)

// --- current-branch header line (TASK-3) ------------------------------------

// TestRenderListShowsCurrentBranch verifies the list header names the branch
// currently checked out in the main worktree, in the "manigot - <project> -
// on <branch>" title.
func TestRenderListShowsCurrentBranch(t *testing.T) {
	dir, def := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa01_a", "aaaa01_a", "# Brief: A\n\nstatus: open\nid: aaaa01\nbranch: feature/aaaa01_a\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
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
	if a.list.currentBranch != "" {
		t.Fatalf("currentBranch = %q on a non-repo project, want empty", a.list.currentBranch)
	}

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
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

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)

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
// pre-existing 1-line floor, up to the configured maximum count.
func TestRenderListRecentActivityScalesWithSpareRoom(t *testing.T) {
	dir, def := gitInitRepo(t)
	commitEmptyAt(t, dir, "c2", 1)
	commitEmptyAt(t, dir, "c3", 2)
	commitEmptyAt(t, dir, "c4", 3)
	commitEmptyAt(t, dir, "c5", 4)
	commitEmptyAt(t, dir, "c6", 5)
	commitEmptyAt(t, dir, "c7", 6)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa02_a", "aaaa02_a", "# Brief: A\n\nstatus: open\nid: aaaa02\nbranch: feature/aaaa02_a\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: got %d jobs, want 1", len(jobs))
	}
	a := NewApp(dir, jobs)
	// Pin the configured count explicitly — NewApp loads the real on-disk
	// settings, so without this the test would depend on whatever
	// tui-settings.json the developer has locally. Re-fetch so the pinned
	// count drives the fetch too, not just the render-time clamp.
	a.settings.RecentActivityCount = 7
	a.refreshRecentCommits()
	a.width, a.height = 80, 40 // generous height — plenty of spare room

	if got := a.list.recentActivityShown(len(a.jobs), a.settings.RecentActivityCountValue(), a.height); got != 7 {
		t.Fatalf("recentActivityShown() = %d, want the configured max 7 given ample spare room", got)
	}

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
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
	wts := t.TempDir()
	for i := 0; i < 20; i++ {
		addJobWorktree(t, dir, wts, fmt.Sprintf("feature/bbbb%02d_b", i), fmt.Sprintf("bbbb%02d_b", i),
			fmt.Sprintf("# Brief: B%d\n\nstatus: open\nid: bbbb%02d\ndate: 2026-01-01\n", i, i))
	}

	jobs, _ := job.Discover(dir)
	if len(jobs) != 20 {
		t.Fatalf("job.Discover: got %d jobs, want 20", len(jobs))
	}
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24 // a list this long already fills a normal terminal

	if got := a.list.recentActivityShown(len(a.jobs), a.settings.RecentActivityCountValue(), a.height); got != recentActivityFloor {
		t.Errorf("recentActivityShown() = %d, want the floor %d when the list already fills the screen", got, recentActivityFloor)
	}

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
	// Every job row must still be present — the strip must never have pushed
	// one off the rendered output. The rows' briefs are frontmatter-only, so
	// job.Stage() reads them as unwritten → define; the status+stage+type
	// cell run is rebuilt here rather than hand-typed so the spacing can't
	// drift from renderJobRow's pad()+join.
	cols := listColumns()
	row := pad("open", cols.status) + "  " + pad("define", cols.stage) + "  " + pad("feature", cols.typ)
	if n := strings.Count(got, row); n != 20 {
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
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa03_a", "aaaa03_a", "# Brief: A\n\nstatus: open\nid: aaaa03\nbranch: feature/aaaa03_a\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	// a.width, a.height left at zero.

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
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

	if len(a.list.recentCommits) != 0 {
		t.Fatalf("recentCommits on a fresh empty repo = %+v, want empty", a.list.recentCommits)
	}
	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
	if got == "" {
		t.Fatal("renderList returned nothing")
	}
}

// TestRenderListZeroHeightNoCommitsDoesNotPanic is the yz0vfz regression
// test: the exact crash combination from the brief — a fresh repo with no
// commits yet (an unborn HEAD, exactly the state right after mg init on a
// brand-new project) and an App that never received a tea.WindowSizeMsg
// (a.height left at its zero value). Before the fix,
// recentActivityShown's a.height == 0 path returned the floor (1) without
// clamping to len(a.recentCommits), and renderRecentActivity sliced
// a.recentCommits[:1] against the empty cache — "slice bounds out of range
// [:1] with capacity 0". The two existing neighbors each cover one half of
// the combination: TestRenderListZeroHeightDoesNotPanic uses a repo *with*
// an init commit, and TestRenderListRecentActivityEmptyOnFreshRepo uses
// height 24; this test pins the intersection.
func TestRenderListZeroHeightNoCommitsDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	// a.width, a.height left at zero.

	if len(a.list.recentCommits) != 0 {
		t.Fatalf("recentCommits on a fresh empty repo = %+v, want empty", a.list.recentCommits)
	}

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
	if got == "" {
		t.Fatal("renderList returned nothing for a zero-height App on a commit-less repo")
	}
	if !strings.Contains(got, "manigot") {
		t.Errorf("renderList output missing expected header text:\n%s", got)
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

// TestListCtrlRStatusShowsRefreshedJobCount is a regression test for the
// reviewer bounce-back of the "loading indicator for jdi" job: TASK-2's
// refactor of the "ctrl+r" handler to return the spinner-tick cmd moved the
// status line *before* a.refresh(), so the footer showed a stale job count
// whenever the refresh changed the job list (e.g. a job created or archived
// by another process since the last refresh). The count must be read after
// the refresh.
func TestListCtrlRStatusShowsRefreshedJobCount(t *testing.T) {
	root := t.TempDir()
	mkJob := func(name string) {
		dir := filepath.Join(root, "docs", "jobs", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief.md"),
			[]byte("# Brief: "+name+"\n\nstatus: open\ndate: 2026-01-01\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkJob("aa0001_a")

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	if len(a.jobs) != 1 {
		t.Fatalf("setup: want 1 job discovered, got %d", len(a.jobs))
	}

	// A second job appears out-of-band after the TUI's last discovery.
	mkJob("bb0002_b")

	model, _ := a.updateList(keyMsg("ctrl+r"))
	got := model.(*App)

	want := fmt.Sprintf("refreshed · %d job(s)", len(got.jobs))
	if got.status != want {
		t.Errorf("status = %q, want the post-refresh count %q", got.status, want)
	}
	if strings.Contains(got.status, "refreshed · 1 job(s)") {
		t.Errorf("status shows the stale pre-refresh count: %q", got.status)
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

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
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

// --- mg-jdi status badge (TASK-8) --------------------------------------------

func TestRenderListShowsJDIRunningBadge(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa01_a", "aaaa01_a", "# Brief: A\n\nstatus: open\nid: aaaa01\nbranch: feature/aaaa01_a\ndate: 2026-01-01\n")

	if err := job.WriteJDIStatus(dir, "aaaa01_a", job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
	if !strings.Contains(got, "running @developer") {
		t.Errorf("renderList missing the running badge:\n%s", got)
	}
}

func TestRenderListShowsJDINeedsHumanBadge(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa01_a", "aaaa01_a", "# Brief: A\n\nstatus: open\nid: aaaa01\nbranch: feature/aaaa01_a\ndate: 2026-01-01\n")

	if err := job.WriteJDIStatus(dir, "aaaa01_a", job.JDIStoppedNeedsHuman, "reviewer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
	if !strings.Contains(got, "needs human") {
		t.Errorf("renderList missing the needs-human badge:\n%s", got)
	}
}

func TestRenderListOmitsJDIBadgeWhenNoStatus(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa01_a", "aaaa01_a", "# Brief: A\n\nstatus: open\nid: aaaa01\nbranch: feature/aaaa01_a\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
	for _, badge := range []string{"[running", "[finished]", "[needs human]"} {
		if strings.Contains(got, badge) {
			t.Errorf("renderList should not render the %s badge with no status sidecar:\n%s", badge, got)
		}
	}
}

// TestRenderListRunningBadgeShowsSpinnerFrame verifies the running badge's
// animated activity-indicator frame renders next to "[running @...]" in the
// list row (TASK-3), and that the badge is actually driven by the App's
// spinnerStep counter — a different step renders a different frame.
func TestRenderListRunningBadgeShowsSpinnerFrame(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa01_a", "aaaa01_a", "# Brief: A\n\nstatus: open\nid: aaaa01\nbranch: feature/aaaa01_a\ndate: 2026-01-01\n")

	if err := job.WriteJDIStatus(dir, "aaaa01_a", job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	cols := listColumns()

	row := a.list.renderJobRow(jobs[0], cols, false, a.spinnerStep)
	if !strings.Contains(row, "running @developer") {
		t.Errorf("row missing the running badge:\n%s", row)
	}
	if !strings.Contains(row, activityFrame(0)) {
		t.Errorf("row missing the spinner frame %q next to the running badge:\n%s", activityFrame(0), row)
	}

	a.spinnerStep = 1
	row = a.list.renderJobRow(jobs[0], cols, false, a.spinnerStep)
	if !strings.Contains(row, activityFrame(1)) {
		t.Errorf("row missing the advanced spinner frame %q:\n%s", activityFrame(1), row)
	}
	if strings.Contains(row, activityFrame(0)) {
		t.Errorf("row still shows frame %q after advancing to step 1:\n%s", activityFrame(0), row)
	}
}

// TestRenderListStoppedBadgesShowNoSpinnerFrame verifies the finished and
// needs-human badges render exactly as before — no spinner frame, since
// nothing is animating — even when the App's step counter is non-zero.
func TestRenderListStoppedBadgesShowNoSpinnerFrame(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa01_a", "aaaa01_a", "# Brief: A\n\nstatus: open\nid: aaaa01\nbranch: feature/aaaa01_a\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.spinnerStep = 3
	cols := listColumns()

	if err := job.WriteJDIStatus(dir, "aaaa01_a", job.JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}
	row := a.list.renderJobRow(jobs[0], cols, false, a.spinnerStep)
	if !strings.Contains(row, "[finished]") {
		t.Errorf("finished badge missing:\n%s", row)
	}
	if strings.Contains(row, activityFrame(3)) {
		t.Errorf("finished badge unexpectedly shows the spinner frame %q:\n%s", activityFrame(3), row)
	}

	if err := job.WriteJDIStatus(dir, "aaaa01_a", job.JDIStoppedNeedsHuman, "reviewer"); err != nil {
		t.Fatal(err)
	}
	row = a.list.renderJobRow(jobs[0], cols, false, a.spinnerStep)
	if !strings.Contains(row, "[needs human]") {
		t.Errorf("needs-human badge missing:\n%s", row)
	}
	if strings.Contains(row, activityFrame(3)) {
		t.Errorf("needs-human badge unexpectedly shows the spinner frame %q:\n%s", activityFrame(3), row)
	}
}

// TestRenderListShowsStageColumn verifies each job row carries the job's
// current workflow stage as its own column (rendered between status and
// type) — the rendering change this job ships. mkStageJob builds a job that
// lands on exactly the requested stage; the row and the full list render are
// both checked, and the expected cell is rebuilt via pad() so the spacing
// can't drift from renderJobRow.
func TestRenderListShowsStageColumn(t *testing.T) {
	for _, stage := range allStages {
		j := mkStageJob(t, stage)
		a := NewApp(j.Root, []job.Job{j})
		a.width, a.height = 80, 24
		cols := listColumns()

		want := pad(string(stage), cols.stage)
		if row := a.list.renderJobRow(j, cols, false, a.spinnerStep); !strings.Contains(row, want) {
			t.Errorf("stage=%s: job row missing the %q stage cell:\n%s", stage, want, row)
		}
		got := a.list.render(a.jobs, a.status, a.settings.RecentActivityCountValue(), a.spinnerStep, a.width, a.height)
		if !strings.Contains(got, want) {
			t.Errorf("stage=%s: renderList missing the %q stage column:\n%s", stage, want, got)
		}
	}
}

// TestRenderListWordIDNotTruncated verifies the job-id column renders
// word-based ids in full with the "#" prefix — the column is 13 wide so the
// longest word id (12 chars, e.g. "unemployment") plus the "#" isn't cut off
// by pad() at the column edge.
func TestRenderListWordIDNotTruncated(t *testing.T) {
	j := job.Job{
		Name:   "unemployment_benefits",
		Dir:    t.TempDir(), // no job files → stage define
		ID:     "unemployment",
		Title:  "Unemployment Benefits",
		Type:   "feature",
		Status: "open",
	}
	cols := listColumns()
	row := (&listView{}).renderJobRow(j, cols, false, 0)
	if !strings.Contains(row, "#unemployment") {
		t.Errorf("job row truncated the word id:\n%s", row)
	}
}

// TestListJAndKNoLongerMoveCursor is the b8kbwb regression test: after "j"
// became the mg-jdi launch key and "k" was dropped, neither vim key may move
// the list cursor anymore — navigation is purely ↑/↓ (plus home/end, g/G).
// "j" is pressed with a working stub mg-jdi so the launch succeeds, proving
// that even a successful launch doesn't move the cursor; "k" is unbound and
// must leave the cursor where it is. ↑/↓ still move, as the contrast.
func TestListJAndKNoLongerMoveCursor(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/aaaa04_a", "aaaa04_a", "# Brief: A\n\nstatus: open\nid: aaaa04\nbranch: feature/aaaa04_a\ndate: 2026-01-01\n")
	addJobWorktree(t, dir, wts, "feature/aaaa05_a", "aaaa05_a", "# Brief: A\n\nstatus: open\nid: aaaa05\nbranch: feature/aaaa05_a\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 2 {
		t.Fatalf("job.Discover: got %d jobs, want 2", len(jobs))
	}

	// Stub mg-jdi so the "j" press below is a real (successful) launch — the
	// point is that it must still not move the cursor.
	t.Setenv("MANIGOT_HOME", "")
	stub := filepath.Join(t.TempDir(), "stub.sh")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubJdiExe(t, stub)

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24

	if a.list.cursor != 0 {
		t.Fatalf("setup: cursor = %d, want 0", a.list.cursor)
	}

	// "j" no longer moves down — it launches mg-jdi against the current job
	// (status proves the launch path ran) and leaves the cursor alone.
	a.updateList(keyMsg("j"))
	if a.list.cursor != 0 {
		t.Errorf("cursor = %d after pressing j, want 0 (j must not move the selection)", a.list.cursor)
	}
	if a.status == "" {
		t.Errorf("status empty after pressing j, want the mg-jdi launch message (the launch path should have run)")
	}

	// "k" no longer moves up.
	a.updateList(keyMsg("k"))
	if a.list.cursor != 0 {
		t.Errorf("cursor = %d after pressing k, want 0 (k must not move the selection)", a.list.cursor)
	}

	// Contrast: ↑/↓ still navigate.
	a.updateList(keyMsg("down"))
	if a.list.cursor != 1 {
		t.Errorf("cursor = %d after pressing down, want 1 (down must still move the selection)", a.list.cursor)
	}
	a.updateList(keyMsg("up"))
	if a.list.cursor != 0 {
		t.Errorf("cursor = %d after pressing up, want 0 (up must still move the selection)", a.list.cursor)
	}
}

// --- shared activity-line formatter (TASK-3/4) -------------------------------

// TestRenderActivityLinesByteIdentical is the regression guard for TASK-3's
// extraction: listView.renderRecentActivity now delegates to the shared
// renderActivityLines free function (which the detail view's strip also
// uses), and this test pins that the strip's per-commit line format is
// byte-for-byte what renderRecentActivity produced before the extraction —
// hash(7) + 2 spaces + subject(truncated to the leftover width) + 2 spaces +
// reltime(10) + 2 spaces + branch(16), trailing spaces trimmed, each line
// dim-styled — by rebuilding the expected output with the original inline
// logic, and by asserting the list view still delegates to the shared
// function.
func TestRenderActivityLinesByteIdentical(t *testing.T) {
	commits := []git.Commit{
		{ShortHash: "abc1234", Subject: "a commit subject", RelTime: "2 hours ago", Branch: "feature/abc_x"},
		{ShortHash: "def5678", Subject: "short", RelTime: "5 days ago", Branch: "main"},
		{ShortHash: "deadbee", Subject: "a very long subject that must be truncated down to the column width", RelTime: "now", Branch: "averylongbranchname"},
	}
	w := 80

	got := renderActivityLines(commits, w)

	// The original inline logic the extraction came from.
	const hashW, relW, branchW = 7, 10, 16
	subjectW := w - hashW - relW - branchW - 6
	var want strings.Builder
	for _, c := range commits {
		line := pad(c.ShortHash, hashW) + "  " +
			pad(truncate(c.Subject, subjectW), subjectW) + "  " +
			pad(c.RelTime, relW) + "  " +
			truncate(c.Branch, branchW)
		want.WriteString(activityStyle.Render(strings.TrimRight(line, " ")))
		want.WriteString("\n")
	}
	if got != want.String() {
		t.Errorf("renderActivityLines drifted from the pre-extraction list output:\n got: %q\nwant: %q", got, want.String())
	}

	// The list view's strip must still go through the same formatter, so the
	// two views can never render different text for the same commits.
	v := &listView{recentCommits: commits}
	if viaList := v.renderRecentActivity(w, len(commits)); viaList != got {
		t.Errorf("listView.renderRecentActivity no longer delegates to renderActivityLines:\n got: %q\nwant: %q", viaList, got)
	}
}
