package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/project"
)

// keyMsg builds a tea.KeyMsg for a single rune, the standard way these tests
// press a key.
func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestDetailBodyHeightShrinksForMultiLineStatus is a regression test for the
// TASK-7 review finding: a multi-line footer (e.g. cmdErrorText's resolution
// diagnosis) must shrink the body viewport, not just get appended on top of
// it — otherwise the rendered view is taller than the terminal and the
// alt-screen clips the bottom of the status (the "fix:" line).
func TestDetailBodyHeightShrinksForMultiLineStatus(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_x")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Enough lines that the body viewer always fills the full viewport,
	// regardless of how tall it is allowed to be.
	var sb strings.Builder
	sb.WriteString("# Brief: X\n\nstatus: open\n\n")
	for i := 0; i < 200; i++ {
		sb.WriteString("line of body text to force scrolling\n")
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	const height = 24
	d := newDetailView(jobs[0], 80, height)

	oneLineLines := strings.Split(d.render(), "\n")
	if len(oneLineLines) > height {
		t.Fatalf("render with 1-line footer used %d rows, want <= %d", len(oneLineLines), height)
	}

	// Simulate a failed agent-launch resolution, which produces a 3-line
	// status (see cmdErrorText / resolve.NotFoundError).
	d.setStatus("error: mg-job not found\ntried: a, b\nfix:   set $MANIGOT_JOB_BIN")

	multiLineLines := strings.Split(d.render(), "\n")
	if len(multiLineLines) > height {
		t.Errorf("render with 3-line footer used %d rows, want <= %d (footer got clipped by the alt-screen viewport)", len(multiLineLines), height)
	}

	// The fix line must actually be present in the rendered output, not just
	// in d.status — i.e. it must not have been truncated by the viewport.
	if !strings.Contains(d.render(), "fix:   set $MANIGOT_JOB_BIN") {
		t.Errorf("rendered view is missing the fix line entirely")
	}
}

// TestDetailDefersResizeForInactiveTabs is a regression test for TASK-3: a
// body-size change (e.g. from setStatus's multi-line-footer handling) must
// resize only the active tab's viewer immediately; the other three should be
// marked stale and only actually re-rendered once they become active.
func TestDetailDefersResizeForInactiveTabs(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0003_z")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("word ", 200)
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief\n\n"+long+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "tasks.md"), []byte("# Tasks\n\n"+long+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	d := newDetailView(jobs[0], 100, 24) // wide viewport

	// Activate tab 1 once so it has an initial wide-width render to diff
	// against below — loadTabs no longer renders inactive tabs eagerly (see
	// TestDetailViewOnlyRendersActiveTabOnLoad), so it starts out unrendered.
	d.cur = 1
	d.render()
	wideLines := d.tabs[1].viewer.LineCount()
	if wideLines == 0 {
		t.Fatal("tab 1 should have rendered content once activated")
	}

	// Back to tab 0 before resizing, so the resize below marks tab 1 stale
	// instead of resizing it directly.
	d.cur = 0
	d.render()

	d.resize(30, 24) // much narrower — would re-wrap into more lines

	// The active tab (0) is resized immediately.
	if d.tabs[0].stale {
		t.Errorf("active tab (0) should not be marked stale after resize")
	}

	// The inactive tab (1) must not have been re-rendered yet: it should
	// still be marked stale and still report the wide-width line count.
	if !d.tabs[1].stale {
		t.Errorf("inactive tab (1) should be marked stale after resize")
	}
	if got := d.tabs[1].viewer.LineCount(); got != wideLines {
		t.Errorf("inactive tab (1) was re-rendered eagerly: LineCount = %d, want unchanged %d", got, wideLines)
	}

	// Switching to it and rendering should resize/re-wrap it and clear the
	// stale flag.
	d.cur = 1
	d.render()
	if d.tabs[1].stale {
		t.Errorf("tab 1 should no longer be stale after becoming active and rendering")
	}
	if got := d.tabs[1].viewer.LineCount(); got == wideLines {
		t.Errorf("tab 1 was not re-wrapped after becoming active (LineCount still %d)", got)
	}
}

// TestDetailViewOnlyRendersActiveTabOnLoad is a regression test: opening a
// job (newDetailView) used to call loadTabs, which eagerly rendered all four
// tabs' markdown via glamour up front — non-trivial cost that made selecting
// a job (and, via detailView.reload, leaving one) feel laggy. Only the active
// tab should render immediately; the rest stay unrendered until switched to.
func TestDetailViewOnlyRendersActiveTabOnLoad(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0004_w")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief\n\nhi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "tasks.md"), []byte("# Tasks\n\nhi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	d := newDetailView(jobs[0], 80, 24)

	if d.tabs[0].stale {
		t.Error("active tab (0, brief) should not be marked stale after load")
	}
	if d.tabs[0].viewer.LineCount() == 0 {
		t.Error("active tab (0, brief) should be rendered immediately on load")
	}
	for i := 1; i < len(d.tabs); i++ {
		if !d.tabs[i].stale {
			t.Errorf("inactive tab %d should be marked stale after load, deferring its render", i)
		}
		if got := d.tabs[i].viewer.LineCount(); got != 0 {
			t.Errorf("inactive tab %d was rendered eagerly on load (LineCount=%d, want 0)", i, got)
		}
	}

	// Switching to a deferred tab renders it lazily.
	d.cur = 1
	d.render()
	if d.tabs[1].stale {
		t.Error("tab 1 should no longer be stale once rendered")
	}
	if d.tabs[1].viewer.LineCount() == 0 {
		t.Error("tab 1 should be rendered once it becomes active")
	}
}

// TestDetailReloadOnlyRendersActiveTab is a regression test for the same bug
// as TestDetailViewOnlyRendersActiveTabOnLoad, but for the reload path
// (detailView.reload, driven by App.refresh — "ctrl+r" and, formerly, going
// back to the list): it must not re-render every tab either, only the active
// one, and it must pick up on-disk changes to the active tab's content.
func TestDetailReloadOnlyRendersActiveTab(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0006_r")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief\n\nv1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "tasks.md"), []byte("# Tasks\n\nv1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}
	d := newDetailView(jobs[0], 80, 24)

	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief\n\nZZUPDATED\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d.reload()

	if !strings.Contains(d.tabs[0].viewer.View(), "ZZUPDATED") {
		t.Error("active tab was not re-rendered with the updated content on reload")
	}
	if d.tabs[1].stale != true {
		t.Error("inactive tab should be marked stale after reload, not re-rendered")
	}
	if got := d.tabs[1].viewer.LineCount(); got != 0 {
		t.Errorf("inactive tab was rendered eagerly on reload (LineCount=%d, want 0)", got)
	}
}

// TestDetailFooterEditHintOnlyOnEditableTab is a regression test for TASK-4:
// the "e edit" footer hint should only show up for tabs the shortcut
// actually does something on (brief.md today), not for the agent-authored
// tabs where pressing "e" is a no-op.
func TestDetailFooterEditHintOnlyOnEditableTab(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0002_y")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: Y\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	d := newDetailView(jobs[0], 80, 24)

	// Tab 0 is brief — editable.
	if !strings.Contains(d.renderFooter(), "e edit") {
		t.Errorf("footer on the brief tab is missing the edit hint:\n%s", d.renderFooter())
	}

	// Tab 1 is tasks — not editable, so no hint.
	d.cur = 1
	if strings.Contains(d.renderFooter(), "e edit") {
		t.Errorf("footer on the tasks tab unexpectedly shows the edit hint:\n%s", d.renderFooter())
	}
}

// --- cross-branch detail view (TASK-4) --------------------------------------
//
// gitRun / gitInitRepo / addJobWorktree are local copies of the job-package
// helpers so this test stays self-contained.

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func gitInitRepo(t *testing.T) (dir, defaultBranch string) {
	t.Helper()
	dir = t.TempDir()
	// -b main pins the default branch: the diff tab resolves the base via
	// git.SymbolicRefHead (no origin/HEAD in a scratch repo → "main"), so a
	// "master"-defaulted repo would make every diff-tab test hit the
	// git-error placeholder instead of the happy path. The helper still reads
	// the branch back (don't assume), keeping its contract unchanged.
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "README")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	b, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("read default branch: %v", err)
	}
	return dir, strings.TrimSpace(string(b))
}

// addJobWorktree creates a new worktree at <worktreesDir>/<name> on a new
// branch, writes docs/jobs/<name>/brief.md inside it, and commits it there —
// mirroring what scripts/new-job.sh does (git worktree add + scaffold
// commit), so tests exercise the exact shape Discover expects (one worktree
// per job branch, main worktree left alone).
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

// TestDetailViewCurrentBranchStillReadsWorkingTree confirms a job's files are
// read straight off its own worktree's working tree, so uncommitted edits
// (made in that worktree) show up without a commit — the same "no branch
// check needed" guarantee TASK-7 gives every caller of Job.Dir. Every open
// job lives in its own worktree (207bfu_git-worktrees), which is
// unconditionally the live, correct place to read its files from.
func TestDetailViewCurrentBranchStillReadsWorkingTree(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/cur01_c", "cur01_c", "# Brief: CurBranch\n\nstatus: open\nid: cur01\nbranch: feature/cur01_c\ndate: 2026-01-01\n\n## What\n\noriginal\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("expected one job in its worktree; got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	if !strings.Contains(d.tabs[0].viewer.View(), "original") {
		t.Errorf("job brief not loaded; view:\n%s", d.tabs[0].viewer.View())
	}

	// Edit brief in the job's own worktree WITHOUT committing — it must show up.
	briefPath := filepath.Join(wtPath, "docs", "jobs", "cur01_c", "brief.md")
	os.WriteFile(briefPath, []byte("# Brief: CurBranch\n\nstatus: open\nid: cur01\nbranch: feature/cur01_c\ndate: 2026-01-01\n\n## What\n\nZZUNCOMMITTED\n"), 0o644)
	d.reload()
	if !strings.Contains(d.tabs[0].viewer.View(), "ZZUNCOMMITTED") {
		t.Errorf("uncommitted edit not reflected for job; view:\n%s", d.tabs[0].viewer.View())
	}
}

// --- branch surfacing (TASK-5) ----------------------------------------------

// TestDetailViewMetaLineShowsBranch confirms the detail view's meta line names
// the job's branch. There is no "other branch" flag anymore — every job has
// its own worktree (207bfu_git-worktrees), so the branch is purely
// informational.
func TestDetailViewMetaLineShowsBranch(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/off05_o", "off05_o", "# Brief: Off\n\nstatus: open\nid: off05\nbranch: feature/off05_o\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}

	d := newDetailView(jobs[0], 80, 24)
	out := d.render()
	if !strings.Contains(out, "branch: feature/off05_o") {
		t.Errorf("meta line missing the branch:\n%s", out)
	}
	if strings.Contains(out, "other branch") {
		t.Errorf("meta line should NOT show the other-branch flag anymore:\n%s", out)
	}
}

// --- status/hint coexistence (TASK-5) ---------------------------------------

// TestDetailFooterKeepsHintAlongsideSingleLineStatus is a regression test for
// TASK-4: a single-line status (e.g. "refreshed" after ctrl+r, or an agent
// launch confirmation) must show alongside the scroll-position/key hint, not
// replace it.
func TestDetailFooterKeepsHintAlongsideSingleLineStatus(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0011_g")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: G\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	d := newDetailView(jobs[0], 80, 24)
	d.setStatus("refreshed")

	footer := d.renderFooter()
	if !strings.Contains(footer, "refreshed") {
		t.Errorf("footer missing the status message:\n%s", footer)
	}
	if !strings.Contains(footer, "q quit") {
		t.Errorf("footer lost the key hint after a single-line status was set:\n%s", footer)
	}
}

// TestDetailFooterMultiLineStatusStillReplacesHint confirms the deliberate
// exception TASK-4 carves out: a multi-line status (cmdErrorText's
// resolution diagnosis) keeps fully replacing the hint, since appending the
// hint to an already multi-line diagnostic risks overflowing narrow
// terminals — this is a distinct, already-tested case (see
// TestDetailBodyHeightShrinksForMultiLineStatus), not the "lost the legend"
// problem TASK-4 targets.
func TestDetailFooterMultiLineStatusStillReplacesHint(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0012_h")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: H\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	d := newDetailView(jobs[0], 80, 24)
	d.setStatus("error: mg-job not found\ntried: a, b\nfix:   set $MANIGOT_JOB_BIN")

	footer := d.renderFooter()
	if strings.Contains(footer, "q quit") {
		t.Errorf("multi-line status footer should not have the hint appended:\n%s", footer)
	}
}

// --- de-duplicated identity/metadata (TASK-7) -------------------------------

// TestDetailTabsDoNotRepeatTitleOrFrontmatter is the main proof for TASK-6:
// for each of the four job files (in new-job.sh's exact scaffold shape), the
// rendered tab body must not repeat the file's own "# <Label>: <title>"
// heading or its "key: value" frontmatter lines — the chrome's title+meta
// line already shows all of that — while real body content, including a
// TASK-N-shaped line inside a real section, still renders.
func TestDetailTabsDoNotRepeatTitleOrFrontmatter(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0020_dedup")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"brief.md":          "# Brief: Dedup\n\nstatus: open\ntype: feature\nid: ab0020\nbranch: feature/ab0020_dedup\ndate: 2026-01-01\nauthor: Test\n\n## What\n\nZZBRIEFBODY real content here.\n",
		"tasks.md":          "# Tasks: Dedup\n\nid: ab0020\nstatus: open\nanalyst: Test\ndate: 2026-01-01\n\n## Task breakdown\n\nTASK-1: real work here, colon-shaped like frontmatter but deep in the body.\n",
		"implementation.md": "# Implementation: Dedup\n\nid: ab0020\nstatus: open\ndeveloper: Test\ndate: 2026-01-01\n\n## Summary\n\nZZIMPLBODY TASK-1 implemented.\n",
		"verdict.md":        "# Verdict: Dedup\n\nid: ab0020\nstatus: open\nreviewer: Test\ndate: 2026-01-01\n\n## Review\n\nTASK-1: PASS, ZZVERDICTBODY.\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(jobDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	cases := []struct {
		tab            int
		heading        string // the file's own H1 text, must NOT appear
		frontmatterHit string // a frontmatter value, must NOT appear
		realContent    string // real body content, MUST appear
	}{
		{0, "Brief: Dedup", "author: Test", "ZZBRIEFBODY"},
		{1, "Tasks: Dedup", "analyst: Test", "TASK-1: real work here"},
		{2, "Implementation: Dedup", "developer: Test", "ZZIMPLBODY"},
		{3, "Verdict: Dedup", "reviewer: Test", "ZZVERDICTBODY"},
	}
	for _, c := range cases {
		d := newDetailView(jobs[0], 80, 24)
		d.cur = c.tab
		d.render()
		view := d.tabs[c.tab].viewer.View()

		if strings.Contains(view, c.heading) {
			t.Errorf("tab %d: rendered body repeats its own heading %q:\n%s", c.tab, c.heading, view)
		}
		if strings.Contains(view, c.frontmatterHit) {
			t.Errorf("tab %d: rendered body repeats frontmatter %q the chrome already shows:\n%s", c.tab, c.frontmatterHit, view)
		}
		if !strings.Contains(view, c.realContent) {
			t.Errorf("tab %d: rendered body missing real content %q:\n%s", c.tab, c.realContent, view)
		}
	}
}

// TestDetailTabsDoNotRepeatTitleOrFrontmatter's "TASK-1: real work here"
// case asserts "TASK-1" survives inside "## Task breakdown" for the tasks tab —
// the over-strip regression guard for a TASK-shaped line in the body.

// TestFilePlaceholderHasNoOwnHeading confirms filePlaceholder no longer
// renders its own "# <label>" heading (TASK-6): the chrome's title line
// already names the job regardless of which tab/file is showing, so a
// missing file's placeholder would otherwise be a second (empty-ish) title.
func TestFilePlaceholderHasNoOwnHeading(t *testing.T) {
	got := filePlaceholder("implementation.md")
	if strings.HasPrefix(strings.TrimSpace(got), "#") {
		t.Errorf("filePlaceholder still renders its own heading: %q", got)
	}
	if !strings.Contains(got, "implementation.md") {
		t.Errorf("filePlaceholder missing the filename: %q", got)
	}
}

// --- log tab (TASK-9) --------------------------------------------------------

// discoverOneJob is a small helper for the log-tab tests below: a non-repo
// project (job.Discover's working-tree-only fallback) with a single job
// whose brief.md is real enough to be "written".
func discoverOneJob(t *testing.T, root, name string) job.Job {
	t.Helper()
	jobDir := filepath.Join(root, "docs", "jobs", name)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "# Brief: Log tab test\n\nstatus: open\nid: " + name + "\n\n" +
		"## What\n\nsomething substantial\nand a second line\n"
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}
	return jobs[0]
}

func TestDetailViewHasSixTabsIncludingLogAndDiff(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa01_x")

	d := newDetailView(j, 80, 24)
	if len(d.tabs) != 6 {
		t.Fatalf("len(d.tabs) = %d, want 6", len(d.tabs))
	}
	last := d.tabs[5]
	if last.label != "diff" {
		t.Errorf("tabs[5].label = %q, want %q", last.label, "diff")
	}
	if !last.isDiff {
		t.Error("tabs[5].isDiff = false, want true")
	}
	if last.editable {
		t.Error("the diff tab must never be editable")
	}
}

func TestDetailViewLogTabPlaceholderWhenNoRun(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa01_x")

	d := newDetailView(j, 80, 24)
	d.cur = 4 // switch to the log tab so render() catches it up
	d.ensureCurrentSized()

	if d.tabs[4].exists {
		t.Error("log tab exists=true with no run.log at all, want false")
	}
	if !strings.Contains(d.tabs[4].content, "no mg jdi run has happened") {
		t.Errorf("log tab placeholder = %q, want it to explain no run has happened yet", d.tabs[4].content)
	}
}

func TestDetailViewLogTabShowsRunLogContent(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa01_x")

	logDir := job.JDIStatusDir(root, j.Name)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "=== 2026-08-09T00:00:00Z analyst (attempt 1) ===\nwrote tasks.md, all good\n"
	if err := os.WriteFile(job.JDIRunLogPath(root, j.Name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	d := newDetailView(j, 80, 24)
	d.cur = 4
	d.ensureCurrentSized()

	if !d.tabs[4].exists {
		t.Error("log tab exists=false with a real run.log present, want true")
	}
	if !strings.Contains(d.tabs[4].content, "wrote tasks.md, all good") {
		t.Errorf("log tab content = %q, want it to contain the run.log body", d.tabs[4].content)
	}
	rendered := d.render()
	if !strings.Contains(rendered, "wrote tasks.md, all good") {
		t.Errorf("rendered detail view does not show the log tab's content:\n%s", rendered)
	}
}

func TestDetailViewLogTabKeyBindingSwitchesTab(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa01_x")

	d := newDetailView(j, 80, 24)
	d.update(keyMsg("5"))
	if d.cur != 4 {
		t.Errorf("after pressing 5, cur = %d, want 4 (the log tab)", d.cur)
	}
}

func TestDetailViewDiffTabKeyBindingSwitchesTab(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa01_x")

	d := newDetailView(j, 80, 24)
	d.update(keyMsg("6"))
	if d.cur != 5 {
		t.Errorf("after pressing 6, cur = %d, want 5 (the diff tab)", d.cur)
	}
}

// --- diff tab content (TASK-1) ----------------------------------------------
//
// These build real scratch repos (the gitInitRepo / addJobWorktree helpers
// above) so the diff tab's git.LogOneline / git.DiffStat calls run end to
// end. The scratch repos are inited with default branch "main" (see
// gitInitRepo), matching the diff tab's SymbolicRefHead fallback, so the
// happy paths resolve.

// TestDetailDiffTabShowsLogAndStatForDivergedBranch is the core content
// test: a job branch with its own commits renders the quick eyeball — the
// branch's own commits (git log --oneline) plus the files it changed
// (git diff --stat), the same output `mg diff` prints.
func TestDetailDiffTabShowsLogAndStatForDivergedBranch(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/df01_d", "df01_d", "# Brief: Diff\n\nstatus: open\nid: df01\nbranch: feature/df01_d\ndate: 2026-01-01\n")
	// A real change on the job branch, committed after the scaffold commit.
	if err := os.WriteFile(filepath.Join(wtPath, "jobfile.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "jobfile.txt")
	gitRun(t, wtPath, "commit", "-q", "-m", "ZZDIFFCOMMIT")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	d.cur = 5
	d.ensureCurrentSized()

	if !d.tabs[5].exists {
		t.Error("diff tab exists=false for a diverged branch, want true")
	}
	// The log half: the branch's own commit subject.
	if !strings.Contains(d.tabs[5].content, "ZZDIFFCOMMIT") {
		t.Errorf("diff tab missing the branch commit in the log half:\n%s", d.tabs[5].content)
	}
	// The stat half: the changed file.
	if !strings.Contains(d.tabs[5].content, "jobfile.txt") {
		t.Errorf("diff tab missing the changed file in the stat half:\n%s", d.tabs[5].content)
	}
	// And the rendered detail view shows it once the tab is active.
	if out := d.render(); !strings.Contains(out, "ZZDIFFCOMMIT") {
		t.Errorf("rendered detail view missing the diff tab's log content:\n%s", out)
	}
}

// TestDetailDiffTabOneChangePerLine is the regression test for the "diff on
// new lines" job: the rendered diff tab must show each commit subject on its
// own rendered line and each diff --stat entry on its own line. Today the
// glamour paragraph reflow joins consecutive non-blank lines into one
// paragraph, so on a wide terminal several commits — and several stat
// entries — land on the same rendered line. A wide viewport is exactly what
// exposes the merge (a narrow one wraps the joined paragraph apart, masking
// it), so this renders wide and asserts per-line structure.
func TestDetailDiffTabOneChangePerLine(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/df06_l", "df06_l", "# Brief: L\n\nstatus: open\nid: df06\nbranch: feature/df06_l\ndate: 2026-01-01\n")
	// Two commits touching two different files: two log subjects and two
	// stat entries — the shape the paragraph reflow used to merge.
	for _, f := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(wtPath, f), []byte(f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, wtPath, "add", f)
		gitRun(t, wtPath, "commit", "-q", "-m", "ZZCHANGE "+f)
	}

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}
	d := newDetailView(jobs[0], 160, 40) // wide + tall
	d.cur = 5
	d.ensureCurrentSized()

	lines := strings.Split(d.tabs[5].viewer.View(), "\n")
	// Each commit subject on its own rendered line.
	assertEachEntryOnOwnLine(t, lines, []string{"ZZCHANGE one.txt", "ZZCHANGE two.txt"})
	// Each diff --stat entry on its own rendered line. git pads the path
	// column ("one.txt                   | 1 +"), so match the path followed
	// by the line-count pipe with flexible whitespace; the pipe never
	// appears in the log half, so it identifies stat lines unambiguously.
	for i, a := range []string{"one.txt", "two.txt"} {
		for _, b := range []string{"one.txt", "two.txt"}[i+1:] {
			reA := regexp.MustCompile(regexp.QuoteMeta(a) + `\s+\|`)
			reB := regexp.MustCompile(regexp.QuoteMeta(b) + `\s+\|`)
			for _, l := range lines {
				if reA.MatchString(l) && reB.MatchString(l) {
					t.Errorf("stat entries %q and %q share one rendered line:\n%q\nfull view:\n%s", a, b, l, strings.Join(lines, "\n"))
				}
			}
		}
	}
	// And each stat entry must actually be present.
	for _, f := range []string{"one.txt", "two.txt"} {
		re := regexp.MustCompile(regexp.QuoteMeta(f) + `\s+\|`)
		if !re.MatchString(strings.Join(lines, "\n")) {
			t.Errorf("stat entry for %q missing from the rendered view:\n%s", f, strings.Join(lines, "\n"))
		}
	}
}

// assertEachEntryOnOwnLine fails when any entry is missing from the given
// lines, or when any two entries share one line — the "one change per line"
// guarantee the diff tab must provide.
func assertEachEntryOnOwnLine(t *testing.T, lines, entries []string) {
	t.Helper()
	seen := make([]bool, len(entries))
	shared := map[string][]string{} // line -> the entries sharing it
	for _, l := range lines {
		var on []string
		for i, e := range entries {
			if strings.Contains(l, e) {
				seen[i] = true
				on = append(on, e)
			}
		}
		if len(on) > 1 {
			shared[l] = on
		}
	}
	for i, e := range entries {
		if !seen[i] {
			t.Errorf("entry %q missing from the rendered view:\n%s", e, strings.Join(lines, "\n"))
		}
	}
	for l, on := range shared {
		t.Errorf("rendered line holds %d entries on one line (%s):\n%q\nfull view:\n%s", len(on), strings.Join(on, ", "), l, strings.Join(lines, "\n"))
	}
}

// TestDetailDiffTabNoChangesPlaceholderForUndivergedBranch: a branch that has
// not diverged from base (no commits of its own) gets `mg diff`'s exact
// "No changes on <branch> relative to <base>." line, not an error — the
// three-dot range is empty, which git reports as success with no output.
func TestDetailDiffTabNoChangesPlaceholderForUndivergedBranch(t *testing.T) {
	dir, base := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := filepath.Join(wts, "df02_u")
	gitRun(t, dir, "worktree", "add", wtPath, "-b", "feature/df02_u")
	// The job dir is written but NOT committed: the branch still points at
	// the same commit as base, so the three-dot range is empty.
	jobDir := filepath.Join(wtPath, "docs", "jobs", "df02_u")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: U\n\nstatus: open\nid: df02\nbranch: feature/df02_u\ndate: 2026-01-01\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	d.cur = 5
	d.ensureCurrentSized()

	if !d.tabs[5].exists {
		t.Error("diff tab exists=false for an undiverged branch, want true (\"no changes\" is a real result)")
	}
	want := "No changes on feature/df02_u relative to " + base + "."
	if !strings.Contains(d.tabs[5].content, want) {
		t.Errorf("diff tab content = %q, want it to contain %q", d.tabs[5].content, want)
	}
}

// TestDetailDiffTabNoBranchPlaceholderForWorkingTreeFallback: a job with no
// branch (job.Discover's non-repo working-tree-only fallback) gets a
// plain-text placeholder and exists=false — there is nothing to diff, so the
// tab bar dims it like the log tab's no-run-yet state.
func TestDetailDiffTabNoBranchPlaceholderForWorkingTreeFallback(t *testing.T) {
	root := t.TempDir() // not a git repo
	j := discoverOneJob(t, root, "aaaa07_n")
	if j.Branch != "" {
		t.Fatalf("setup: expected a branchless job on a non-repo project, got %q", j.Branch)
	}

	d := newDetailView(j, 80, 24)
	d.cur = 5
	d.ensureCurrentSized()

	if d.tabs[5].exists {
		t.Error("diff tab exists=true with no branch, want false")
	}
	if !strings.Contains(d.tabs[5].content, "no branch to diff") {
		t.Errorf("diff tab placeholder = %q, want it to explain there is no branch to diff", d.tabs[5].content)
	}
}

// TestDetailDiffTabRespectsConfiguredBaseBranch proves the diff tab resolves
// the base from .manigot/manigot.json exactly like `mg diff` does: with a
// configured baseBranch that does not exist, the diff must fail against it
// (degrading to the error placeholder that names it) rather than silently
// falling back to the default-branch resolution.
func TestDetailDiffTabRespectsConfiguredBaseBranch(t *testing.T) {
	dir, _ := gitInitRepo(t)
	if err := project.Save(dir, project.Settings{BaseBranch: "trunk"}); err != nil {
		t.Fatal(err)
	}
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/df04_t", "df04_t", "# Brief: T\n\nstatus: open\nid: df04\nbranch: feature/df04_t\ndate: 2026-01-01\n")
	if err := os.WriteFile(filepath.Join(wtPath, "jobfile.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "jobfile.txt")
	gitRun(t, wtPath, "commit", "-q", "-m", "ZZDIFFCOMMIT")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	d.cur = 5
	d.ensureCurrentSized()

	if d.tabs[5].exists {
		t.Error("diff tab exists=true against a missing base branch, want false (degrade)")
	}
	if !strings.Contains(d.tabs[5].content, "trunk") {
		t.Errorf("diff tab did not resolve the configured base branch 'trunk':\n%s", d.tabs[5].content)
	}
}

// TestDetailDiffTabRefreshPicksUpNewCommits is the ctrl+r path: reloading the
// detail view (App.refresh → detail.reload → loadTab) must re-compute the
// diff from git, so commits made after the view opened show up.
func TestDetailDiffTabRefreshPicksUpNewCommits(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/df05_r", "df05_r", "# Brief: R\n\nstatus: open\nid: df05\nbranch: feature/df05_r\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	d.cur = 5
	d.ensureCurrentSized()
	if strings.Contains(d.tabs[5].content, "ZZREFRESHCOMMIT") {
		t.Fatal("setup: refresh commit already present before it was made")
	}

	// Commit a new change on the job branch out-of-band, then refresh.
	if err := os.WriteFile(filepath.Join(wtPath, "jobfile.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "jobfile.txt")
	gitRun(t, wtPath, "commit", "-q", "-m", "ZZREFRESHCOMMIT")

	d.reload()
	if !strings.Contains(d.tabs[5].content, "ZZREFRESHCOMMIT") {
		t.Errorf("reload did not pick up the new commit in the diff tab:\n%s", d.tabs[5].content)
	}
	if !strings.Contains(d.tabs[5].content, "jobfile.txt") {
		t.Errorf("reload did not pick up the new file in the diff tab's stat:\n%s", d.tabs[5].content)
	}
}

// --- vim keybindings removed (TASK-5) ---------------------------------------

// TestDetailVimKeysAreInert confirms j/k/h/l no longer do anything in the
// file body/tab context — no scroll, no tab switch — now that TASK-1 has
// dropped them as aliases for down/up/tab-right/tab-left. up/down/left/right/
// tab/shift+tab are asserted alongside as the contrasting "these still work"
// case, so a regression that broke real navigation wouldn't be masked by only
// checking the vim keys are gone.
func TestDetailVimKeysAreInert(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0030_vim")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	sb.WriteString("# Brief: Vim\n\nstatus: open\n\n")
	for i := 0; i < 200; i++ {
		sb.WriteString("line of body text to force scrolling\n")
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "tasks.md"), []byte("# Tasks: Vim\n\nstatus: open\n\nhi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	d := newDetailView(jobs[0], 80, 24)
	d.update(keyMsg("down")) // move off the top so ScrollUp/j vs k are distinguishable
	d.update(keyMsg("down"))

	wantPos := d.current().viewer.Position()
	wantCur := d.cur

	for _, k := range []string{"j", "k", "h", "l"} {
		d.update(keyMsg(k))
		if got := d.current().viewer.Position(); got != wantPos {
			t.Errorf("after key %q, scroll position = %q, want unchanged %q", k, got, wantPos)
		}
		if d.cur != wantCur {
			t.Errorf("after key %q, d.cur = %d, want unchanged %d", k, d.cur, wantCur)
		}
	}

	// Contrasting case: the real bindings still work.
	d.update(keyMsg("up"))
	if got := d.current().viewer.Position(); got == wantPos {
		t.Errorf("\"up\" should have changed the scroll position, still %q", got)
	}

	d.update(keyMsg("tab"))
	if d.cur != wantCur+1 {
		t.Errorf("\"tab\" should switch to the next tab: cur = %d, want %d", d.cur, wantCur+1)
	}
	d.update(keyMsg("shift+tab"))
	if d.cur != wantCur {
		t.Errorf("\"shift+tab\" should switch back: cur = %d, want %d", d.cur, wantCur)
	}
	d.update(keyMsg("right"))
	if d.cur != wantCur+1 {
		t.Errorf("\"right\" should switch to the next tab: cur = %d, want %d", d.cur, wantCur+1)
	}
	d.update(keyMsg("left"))
	if d.cur != wantCur {
		t.Errorf("\"left\" should switch back: cur = %d, want %d", d.cur, wantCur)
	}
}

// --- mg-jdi status indicator in the detail view (TASK-2/4 of the "multiple
// jdi instances" job) -----------------------------------------------------

// TestDetailActionBarShowsJDIRunningBadge confirms the detail view's action
// bar renders the same "[running @<agent>]" badge the job-list row
// already shows, right alongside the "[j] just do it" button, when the
// sidecar reports job.JDIRunning.
func TestDetailActionBarShowsJDIRunningBadge(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa02_r")

	if err := job.WriteJDIStatus(root, j.Name, job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()
	if !strings.Contains(bar, "running @developer") {
		t.Errorf("action bar missing the running badge:\n%s", bar)
	}
}

// TestDetailActionBarRunningBadgeShowsSpinnerFrame verifies the action bar's
// running badge shows the animated activity-indicator frame next to
// "[running @...]" (TASK-3), driven by the detail view's threaded
// spinnerStep — a different step renders a different frame.
func TestDetailActionBarRunningBadgeShowsSpinnerFrame(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa02_r")

	if err := job.WriteJDIStatus(root, j.Name, job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()
	if !strings.Contains(bar, "running @developer") {
		t.Errorf("action bar missing the running badge:\n%s", bar)
	}
	if !strings.Contains(bar, activityFrame(0)) {
		t.Errorf("action bar missing the spinner frame %q next to the running badge:\n%s", activityFrame(0), bar)
	}

	d.spinnerStep = 2
	bar = d.renderActionBar()
	if !strings.Contains(bar, activityFrame(2)) {
		t.Errorf("action bar missing the advanced spinner frame %q:\n%s", activityFrame(2), bar)
	}
	if strings.Contains(bar, activityFrame(0)) {
		t.Errorf("action bar still shows frame %q after advancing to step 2:\n%s", activityFrame(0), bar)
	}
}

// TestDetailActionBarStoppedBadgesShowNoSpinnerFrame verifies the finished
// and needs-human action-bar badges render no spinner frame — nothing is
// animating — even when the view's step counter is non-zero.
func TestDetailActionBarStoppedBadgesShowNoSpinnerFrame(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa03_f")

	d := newDetailView(j, 80, 24)
	d.spinnerStep = 4

	if err := job.WriteJDIStatus(root, j.Name, job.JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}
	bar := d.renderActionBar()
	if !strings.Contains(bar, "[finished]") {
		t.Errorf("action bar missing the finished badge:\n%s", bar)
	}
	if strings.Contains(bar, activityFrame(4)) {
		t.Errorf("finished badge unexpectedly shows the spinner frame %q:\n%s", activityFrame(4), bar)
	}

	if err := job.WriteJDIStatus(root, j.Name, job.JDIStoppedNeedsHuman, "reviewer"); err != nil {
		t.Fatal(err)
	}
	bar = d.renderActionBar()
	if !strings.Contains(bar, "[needs human]") {
		t.Errorf("action bar missing the needs-human badge:\n%s", bar)
	}
	if strings.Contains(bar, activityFrame(4)) {
		t.Errorf("needs-human badge unexpectedly shows the spinner frame %q:\n%s", activityFrame(4), bar)
	}
}

// TestDetailActionBarShowsJDIFinishedBadge confirms the finished variant
// renders too.
func TestDetailActionBarShowsJDIFinishedBadge(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa03_f")

	if err := job.WriteJDIStatus(root, j.Name, job.JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}

	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()
	if !strings.Contains(bar, "[finished]") {
		t.Errorf("action bar missing the finished badge:\n%s", bar)
	}
}

// TestDetailActionBarShowsJDINeedsHumanBadge confirms the needs-human
// variant renders too.
func TestDetailActionBarShowsJDINeedsHumanBadge(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa04_h")

	if err := job.WriteJDIStatus(root, j.Name, job.JDIStoppedNeedsHuman, "reviewer"); err != nil {
		t.Fatal(err)
	}

	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()
	if !strings.Contains(bar, "[needs human]") {
		t.Errorf("action bar missing the needs-human badge:\n%s", bar)
	}
}

// TestDetailActionBarOmitsJDIBadgeWhenNoStatus confirms there is no badge at
// all — not even an empty one — when there is no sidecar status file yet for
// the job. The plain "[j] just do it" button label is expected to still be
// present; only the badge's own bracket-wrapped text must be absent.
func TestDetailActionBarOmitsJDIBadgeWhenNoStatus(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa05_n")

	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()
	for _, badge := range []string{"[running", "[finished]", "[needs human]"} {
		if strings.Contains(bar, badge) {
			t.Errorf("action bar should not render the %s badge with no status sidecar:\n%s", badge, bar)
		}
	}
	if !strings.Contains(bar, "just do it") {
		t.Errorf("action bar should still show the [j] just do it button itself:\n%s", bar)
	}
}

// TestStripLeadingFrontmatterLeavesNonScaffoldContentAlone confirms a file
// that doesn't start with an H1, or whose line right after it isn't
// frontmatter-shaped, is returned unchanged rather than guessed at.
func TestStripLeadingFrontmatterLeavesNonScaffoldContentAlone(t *testing.T) {
	cases := []string{
		"no heading here, just prose\n\nmore prose\n",
		"# Just a heading\n\nStraight into prose, no frontmatter at all.\n",
	}
	for _, in := range cases {
		if got := stripLeadingFrontmatter(in); got != in {
			t.Errorf("stripLeadingFrontmatter(%q) = %q, want unchanged", in, got)
		}
	}
}

// --- job-branch git-log strip (TASK-3/4) -------------------------------------

// TestDetailCommitStripShowsOnlyJobBranchCommits verifies the detail view's
// bottom git-log strip is scoped to the job's own branch (git.BranchCommits
// against d.job.Branch): a commit made on the base branch after the job
// branched off must not appear, while the job branch's own commits (and the
// shared history it was cut from) do.
func TestDetailCommitStripShowsOnlyJobBranchCommits(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/st01_s", "st01_s", "# Brief: Strip\n\nstatus: open\nid: st01\nbranch: feature/st01_s\ndate: 2026-01-01\n")

	// A commit on the job branch itself, after the scaffold commit.
	if err := os.WriteFile(filepath.Join(wtPath, "jobfile.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "jobfile.txt")
	gitRun(t, wtPath, "commit", "-q", "-m", "ZZJOBCOMMIT")

	// A commit on the base branch made after the job branched — reachable
	// from the base branch only, so it must never appear in the job's strip.
	if err := os.WriteFile(filepath.Join(dir, "mainfile.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "mainfile.txt")
	gitRun(t, dir, "commit", "-q", "-m", "ZZMAINCOMMIT")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	d.refreshCommits(5)

	out := d.render()
	if !strings.Contains(out, "ZZJOBCOMMIT") {
		t.Errorf("job strip missing the job branch's own commit:\n%s", out)
	}
	if strings.Contains(out, "ZZMAINCOMMIT") {
		t.Errorf("job strip shows the base-branch-only commit:\n%s", out)
	}
}

// TestDetailCommitStripEmptyWhenNoBranchOrNonRepo verifies a job with no
// branch at all (job.Discover's non-repo working-tree-only fallback) gets no
// strip: refreshCommits degrades to an empty cache, and the render shows
// neither commit lines nor an extra spacer, staying within the viewport.
func TestDetailCommitStripEmptyWhenNoBranchOrNonRepo(t *testing.T) {
	root := t.TempDir() // not a git repo
	j := discoverOneJob(t, root, "aaaa06_n")
	if j.Branch != "" {
		t.Fatalf("setup: expected a branchless job on a non-repo project, got %q", j.Branch)
	}

	d := newDetailView(j, 80, 24)
	d.refreshCommits(5)
	if len(d.recentCommits) != 0 {
		t.Fatalf("recentCommits = %+v, want empty for a branchless/non-repo job", d.recentCommits)
	}
	out := d.render()
	if len(strings.Split(out, "\n")) > 24 {
		t.Errorf("render grew past the viewport without a strip:\n%s", out)
	}
}

// TestDetailCommitStripDegradesOnNonRepo covers the other half of the
// degrade: a job that does carry a Branch field but whose Root isn't a git
// repo — git.BranchCommits errors, refreshCommits degrades to an empty cache,
// and no strip renders.
func TestDetailCommitStripDegradesOnNonRepo(t *testing.T) {
	root := t.TempDir() // not a git repo
	j := job.Job{Name: "x_a", Dir: filepath.Join(root, "docs", "jobs", "x_a"), Root: root, Branch: "feature/x_a"}
	d := newDetailView(j, 80, 24)
	d.refreshCommits(5)
	if len(d.recentCommits) != 0 {
		t.Fatalf("recentCommits = %+v, want empty when BranchCommits errors", d.recentCommits)
	}
}

// TestDetailCommitStripFitsViewportAndCoexistsWithFooter verifies the strip
// renders below the footer and the whole view still fits the terminal: the
// body must shrink (bodyHeight's commitStripRows budget) so the strip lines
// don't clip off the alt-screen — the same overflow
// TestDetailBodyHeightShrinksForMultiLineStatus guards for the footer.
func TestDetailCommitStripFitsViewportAndCoexistsWithFooter(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/st02_s", "st02_s", "# Brief: Strip\n\nstatus: open\nid: st02\nbranch: feature/st02_s\ndate: 2026-01-01\n")
	// Enough commits to exercise a multi-line strip at the configured max.
	for _, subj := range []string{"ZZC3", "ZZC2", "ZZC1", "ZZC0"} {
		if err := os.WriteFile(filepath.Join(wtPath, "f.txt"), []byte(subj), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, wtPath, "add", "-A")
		gitRun(t, wtPath, "commit", "-q", "-m", subj)
	}

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 {
		t.Fatalf("job.Discover: want 1 job, got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	d.refreshCommits(5)

	const height = 24
	out := d.render()
	if len(strings.Split(out, "\n")) > height {
		t.Errorf("render with the commit strip used %d rows, want <= %d (strip clipped by the alt-screen viewport)", len(strings.Split(out, "\n")), height)
	}

	// Footer and strip must both be present, footer above the strip.
	if !strings.Contains(out, "q quit") {
		t.Errorf("footer missing from the render with the strip:\n%s", out)
	}
	if !strings.Contains(out, "ZZC3") {
		t.Errorf("strip missing the job branch's most recent commit:\n%s", out)
	}
	if !strings.Contains(out, "ZZC0") {
		t.Errorf("strip missing the oldest cached commit (all cached entries should render when room allows):\n%s", out)
	}
	lines := strings.Split(out, "\n")
	footerIdx, stripIdx := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "q quit") {
			footerIdx = i
		}
		if strings.Contains(l, "ZZC3") {
			stripIdx = i
		}
	}
	if footerIdx < 0 || stripIdx < 0 || stripIdx <= footerIdx {
		t.Errorf("strip should render below the footer (footer line %d, strip line %d):\n%s", footerIdx, stripIdx, out)
	}
}

// TestDetailCommitStripSizesAdaptively verifies the strip mirrors the list
// view's adaptive sizing (listView.recentActivityShown): with an empty
// height guard falling back to the floor, and with a tall viewport showing
// the configured maximum — clamped to the available commits.
func TestDetailCommitStripSizesAdaptively(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/st03_s", "st03_s", "# Brief: Strip\n\nstatus: open\nid: st03\nbranch: feature/st03_s\ndate: 2026-01-01\n")
	for _, subj := range []string{"c1", "c2", "c3", "c4", "c5", "c6"} {
		if err := os.WriteFile(filepath.Join(wtPath, "f.txt"), []byte(subj), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, wtPath, "add", "-A")
		gitRun(t, wtPath, "commit", "-q", "-m", subj)
	}

	jobs, _ := job.Discover(dir)
	d := newDetailView(jobs[0], 80, 40) // generous height — plenty of spare room
	d.refreshCommits(7)

	if got := d.commitStripShown(); got != 7 {
		t.Errorf("commitStripShown() = %d, want the configured max 7 given ample spare room", got)
	}
	// All 7 cached entries (6 job commits + the scaffold commit) render.
	out := d.render()
	if n := strings.Count(out, "\n"); n > 40 {
		t.Errorf("render at height 40 with a 7-line strip used %d rows, want <= 40", n)
	}

	// A view that never received a WindowSizeMsg falls back to the floor.
	d2 := newDetailView(jobs[0], 80, 0)
	d2.refreshCommits(7)
	if got := d2.commitStripShown(); got != recentActivityFloor {
		t.Errorf("commitStripShown() with height 0 = %d, want the floor %d", got, recentActivityFloor)
	}
}
