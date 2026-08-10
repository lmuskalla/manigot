package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/job"
)

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
// gitRun / gitInitRepo / gitCommitJob are local copies of the job-package
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
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644)
	gitRun(t, dir, "add", "README")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	b, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("read default branch: %v", err)
	}
	return dir, strings.TrimSpace(string(b))
}

func gitCommitJob(t *testing.T, dir, name, brief string) {
	t.Helper()
	jobDir := filepath.Join(dir, "docs", "jobs", name)
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "job "+name)
}

// TestDetailViewReadsOffBranchJobViaGit confirms the detail view shows an
// off-branch job's files even though they aren't checked out into the working
// tree at all. Without TASK-4 every tab would show the "(label)" placeholder
// and a blank body.
func TestDetailViewReadsOffBranchJobViaGit(t *testing.T) {
	dir, def := gitInitRepo(t)

	// Build a job with real brief + tasks content on a feature branch.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/off01_o")
	gitCommitJob(t, dir, "off01_o",
		"# Brief: OffBranch\n\nstatus: open\nid: off01\nbranch: feature/off01_o\ndate: 2026-01-01\n\n## What\n\nZZOFFBRANCHBODY\n")
	tasks := "# Tasks: OffBranch\n\nid: off01\nstatus: open\n\n## Task breakdown\n\nTASK-1: real off-branch work here\n"
	os.WriteFile(filepath.Join(dir, "docs", "jobs", "off01_o", "tasks.md"), []byte(tasks), 0o644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "tasks")
	gitRun(t, dir, "checkout", "-q", def)

	// The job dir is absent from the default branch's working tree.
	if _, err := os.Stat(filepath.Join(dir, "docs", "jobs", "off01_o")); !os.IsNotExist(err) {
		t.Fatalf("off01_o unexpectedly checked out on default branch: %v", err)
	}

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}
	if jobs[0].OnCurrentBranch {
		t.Fatal("expected an off-branch job (OnCurrentBranch=false)")
	}

	d := newDetailView(jobs[0], 80, 24)

	// Brief tab: exists, body content read via git show.
	if !d.tabs[0].exists {
		t.Error("brief tab should exist for an off-branch job (read via git show)")
	}
	if !strings.Contains(d.tabs[0].viewer.View(), "ZZOFFBRANCHBODY") {
		t.Errorf("brief body not loaded for off-branch job; view:\n%s", d.tabs[0].viewer.View())
	}

	// Tasks tab: exists with the TASK- marker.
	d.cur = 1
	d.render()
	if !d.tabs[1].exists {
		t.Error("tasks tab should exist for an off-branch job (read via git show)")
	}
	if !strings.Contains(d.tabs[1].viewer.View(), "TASK-1") {
		t.Errorf("tasks body not loaded for off-branch job; view:\n%s", d.tabs[1].viewer.View())
	}

	// Tabs whose files don't exist on the branch (implementation, verdict)
	// still fall back to the "(label)" placeholder, same as the working-tree
	// path's missing-file behaviour. (The inactive tab's viewer isn't
	// rendered yet — loadTab only sets t.content for non-active tabs — so we
	// assert on content, then confirm activating it renders the placeholder.)
	if d.tabs[2].exists {
		t.Error("implementation tab should be the placeholder (no implementation.md on the branch)")
	}
	if !strings.Contains(d.tabs[2].content, "has not been written yet") {
		t.Errorf("implementation tab content should be the placeholder; content:\n%s", d.tabs[2].content)
	}
	d.cur = 2
	d.render()
	if !strings.Contains(d.tabs[2].viewer.View(), "has not been written yet") {
		t.Errorf("implementation tab should render the placeholder once active; view:\n%s", d.tabs[2].viewer.View())
	}
}

// TestDetailViewCurrentBranchStillReadsWorkingTree confirms the current-branch
// path did not regress: a job checked out into the working tree still reflects
// uncommitted edits (TASK-4's "current-branch jobs keep working-tree reads").
func TestDetailViewCurrentBranchStillReadsWorkingTree(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "cur01_c", "# Brief: CurBranch\n\nstatus: open\nid: cur01\nbranch: "+def+"\ndate: 2026-01-01\n\n## What\n\noriginal\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 || !jobs[0].OnCurrentBranch {
		t.Fatalf("expected one current-branch job; got %+v", jobs)
	}
	d := newDetailView(jobs[0], 80, 24)
	if !strings.Contains(d.tabs[0].viewer.View(), "original") {
		t.Errorf("current-branch brief not loaded; view:\n%s", d.tabs[0].viewer.View())
	}

	// Edit brief in the working tree WITHOUT committing — it must show up.
	briefPath := filepath.Join(dir, "docs", "jobs", "cur01_c", "brief.md")
	os.WriteFile(briefPath, []byte("# Brief: CurBranch\n\nstatus: open\nid: cur01\nbranch: "+def+"\ndate: 2026-01-01\n\n## What\n\nZZUNCOMMITTED\n"), 0o644)
	d.reload()
	if !strings.Contains(d.tabs[0].viewer.View(), "ZZUNCOMMITTED") {
		t.Errorf("uncommitted edit not reflected for current-branch job; view:\n%s", d.tabs[0].viewer.View())
	}
}

// --- branch surfacing (TASK-5) ----------------------------------------------

// TestDetailViewMetaLineShowsBranch confirms the detail view's meta line names
// the job's branch, and flags it as "other branch" when the job isn't on the
// currently-checked-out branch.
func TestDetailViewMetaLineShowsBranch(t *testing.T) {
	dir, def := gitInitRepo(t)
	// Off-branch job.
	gitRun(t, dir, "checkout", "-q", "-b", "feature/off05_o")
	gitCommitJob(t, dir, "off05_o", "# Brief: Off\n\nstatus: open\nid: off05\nbranch: feature/off05_o\ndate: 2026-01-01\n")
	gitRun(t, dir, "checkout", "-q", def)
	// Current-branch job.
	gitCommitJob(t, dir, "cur05_c", "# Brief: Cur\n\nstatus: open\nid: cur05\nbranch: "+def+"\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	byID := map[string]job.Job{}
	for _, j := range jobs {
		byID[j.ID] = j
	}

	// Off-branch job: branch shown + flagged as other-branch. The chrome's
	// off-branch format doesn't repeat the "branch: " label prefix (see
	// detailView.render's branchMeta) — it reads "<branch> (other branch —
	// press b to switch)" instead, so this asserts on the branch name alone.
	// (Before TASK-6's dedup stripping, this also matched the raw frontmatter
	// line rendered a second time in the body — that was a coincidental
	// pass, not what this test means to cover.)
	dOff := newDetailView(byID["off05"], 80, 24)
	out := dOff.render()
	if !strings.Contains(out, "feature/off05_o") {
		t.Errorf("off-branch meta line missing the branch:\n%s", out)
	}
	if !strings.Contains(out, "other branch") {
		t.Errorf("off-branch meta line missing the 'other branch' flag:\n%s", out)
	}

	// Current-branch job: branch shown, no other-branch flag.
	dCur := newDetailView(byID["cur05"], 80, 24)
	out = dCur.render()
	if !strings.Contains(out, "branch: "+def) {
		t.Errorf("current-branch meta line missing the branch:\n%s", out)
	}
	if strings.Contains(out, "other branch") {
		t.Errorf("current-branch meta line should NOT show the other-branch flag:\n%s", out)
	}
}

// TestJobListMarksOffBranchRows confirms the job list appends a "· <branch>"
// tag to rows whose job isn't on the current branch, and leaves current-branch
// rows untouched (no new column introduced).
func TestJobListMarksOffBranchRows(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/off06_o")
	gitCommitJob(t, dir, "off06_o", "# Brief: Off\n\nstatus: open\nid: off06\nbranch: feature/off06_o\ndate: 2026-01-01\n")
	gitRun(t, dir, "checkout", "-q", def)
	gitCommitJob(t, dir, "cur06_c", "# Brief: Cur\n\nstatus: open\nid: cur06\nbranch: "+def+"\ndate: 2026-02-02\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	cols := listColumns()

	byID := map[string]job.Job{}
	for _, j := range jobs {
		byID[j.ID] = j
	}

	// Off-branch row carries the branch tag.
	offRow := a.renderJobRow(byID["off06"], cols, false)
	if !strings.Contains(offRow, "feature/off06_o") {
		t.Errorf("off-branch row missing the branch tag:\n%s", offRow)
	}

	// Current-branch row carries no branch tag (its Branch equals the current).
	curRow := a.renderJobRow(byID["cur06"], cols, false)
	if strings.Contains(curRow, def) {
		t.Errorf("current-branch row should not carry a branch tag (Branch == current); got:\n%s", curRow)
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

// TestDetailViewReadsOffBranchJobViaGit (existing, off-branch fixture) already
// asserts "TASK-1" survives inside "## Task breakdown" for the tasks tab —
// re-confirmed here isn't necessary, just noted: that assertion is the
// over-strip regression guard for the git-show read path specifically, this
// test's is for the working-tree read path.

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

func TestDetailViewHasFiveTabsIncludingLog(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa01_x")

	d := newDetailView(j, 80, 24)
	if len(d.tabs) != 5 {
		t.Fatalf("len(d.tabs) = %d, want 5", len(d.tabs))
	}
	last := d.tabs[4]
	if last.label != "log" {
		t.Errorf("tabs[4].label = %q, want %q", last.label, "log")
	}
	if !last.isLog {
		t.Error("tabs[4].isLog = false, want true")
	}
	if last.editable {
		t.Error("the log tab must never be editable")
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
	if !strings.Contains(d.tabs[4].content, "no mg-jdi run has happened") {
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
// bar renders the same "[mg-jdi: running @<agent>]" badge the job-list row
// already shows, right alongside the "[j] mg-jdi" button, when the sidecar
// reports job.JDIRunning.
func TestDetailActionBarShowsJDIRunningBadge(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa02_r")

	if err := job.WriteJDIStatus(root, j.Name, job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()
	if !strings.Contains(bar, "mg-jdi: running @developer") {
		t.Errorf("action bar missing the running badge:\n%s", bar)
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
	if !strings.Contains(bar, "mg-jdi: finished") {
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
	if !strings.Contains(bar, "mg-jdi: needs human") {
		t.Errorf("action bar missing the needs-human badge:\n%s", bar)
	}
}

// TestDetailActionBarOmitsJDIBadgeWhenNoStatus confirms there is no badge at
// all — not even an empty one — when there is no sidecar status file yet for
// the job. The plain "[j] mg-jdi" button label (no colon) is expected to
// still be present; only the colon-bearing "mg-jdi:" badge text must be
// absent.
func TestDetailActionBarOmitsJDIBadgeWhenNoStatus(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa05_n")

	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()
	if strings.Contains(bar, "mg-jdi:") {
		t.Errorf("action bar should not mention mg-jdi: with no status sidecar:\n%s", bar)
	}
	if !strings.Contains(bar, "mg-jdi") {
		t.Errorf("action bar should still show the [j] mg-jdi button itself:\n%s", bar)
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
