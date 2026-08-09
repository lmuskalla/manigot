package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/tui/internal/job"
)

// TestRefreshPicksUpFileEdits verifies that App.refresh re-reads the detail
// view's files after an agent edits them on disk.
func TestRefreshPicksUpFileEdits(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_x")
	os.MkdirAll(jobDir, 0o755)
	brief := filepath.Join(jobDir, "brief.md")
	os.WriteFile(brief, []byte("# Brief: X\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)

	// Sanity: original brief has no marker.
	if strings.Contains(a.detail.tabs[0].viewer.View(), "ZZREFRESHMARKER") {
		t.Fatal("marker already present before edit")
	}

	// Simulate an agent writing new content to brief.md out-of-band.
	os.WriteFile(brief, []byte("# Brief: X\n\nstatus: open\n\nZZREFRESHMARKER new paragraph here.\n"), 0o644)

	a.refresh()

	got := a.detail.tabs[0].viewer.View()
	if !strings.Contains(got, "ZZREFRESHMARKER") {
		t.Errorf("refresh did not reload brief.md; marker missing from view")
	}
}

// TestRefreshClampsCursor verifies the cursor is clamped if a job disappears
// (e.g. archived) between refreshes.
func TestRefreshClampsCursor(t *testing.T) {
	root := t.TempDir()
	mkJob := func(name string) {
		dir := filepath.Join(root, "docs", "jobs", name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "brief.md"),
			[]byte("# Brief: "+name+"\n\nstatus: open\ndate: 2026-01-01\n"), 0o644)
	}
	mkJob("aa0001_a")
	mkJob("bb0002_b")

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.cursor = len(a.jobs) - 1 // point at the last job

	// Remove the job under the cursor (simulate archiving).
	os.RemoveAll(filepath.Join(root, "docs", "jobs", "bb0002_b"))

	a.refresh()
	if a.cursor >= len(a.jobs) {
		t.Errorf("cursor = %d after refresh, but only %d jobs remain", a.cursor, len(a.jobs))
	}
	if a.cursor < 0 {
		t.Errorf("cursor went negative: %d", a.cursor)
	}
}

// TestEscFromDetailDoesNotReloadDetailFiles is a regression test: going back
// to the list used to call the full refresh() — which reloads every open
// detail-view tab (detailView.reload, itself now cheap per the lazy-tab-load
// fix, but pointless here regardless) — right before discarding that detail
// view. It must use refreshJobs (list only) instead, since the view being
// reloaded is thrown away in the very next line anyway.
func TestEscFromDetailDoesNotReloadDetailFiles(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0007_e")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: E\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := model.(*App)

	if got.state != stateList {
		t.Errorf("state = %v, want stateList", got.state)
	}
	if got.detail != nil {
		t.Errorf("detail should be nil after returning to the list")
	}
	if got.status != "refreshed" {
		t.Errorf("status = %q, want %q", got.status, "refreshed")
	}
}

// TestRefreshJobsClampsCursorWithoutTouchingDetail verifies refreshJobs (the
// list-only half of refresh, used by the esc/backspace back-to-list path)
// updates a.jobs and clamps the cursor exactly like refresh, without
// requiring or touching a.detail.
func TestRefreshJobsClampsCursorWithoutTouchingDetail(t *testing.T) {
	root := t.TempDir()
	mkJob := func(name string) {
		dir := filepath.Join(root, "docs", "jobs", name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "brief.md"),
			[]byte("# Brief: "+name+"\n\nstatus: open\ndate: 2026-01-01\n"), 0o644)
	}
	mkJob("aa0002_a")
	mkJob("bb0003_b")

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.cursor = len(a.jobs) - 1 // a.detail stays nil throughout

	os.RemoveAll(filepath.Join(root, "docs", "jobs", "bb0003_b"))

	a.refreshJobs()
	if a.cursor >= len(a.jobs) {
		t.Errorf("cursor = %d after refreshJobs, but only %d jobs remain", a.cursor, len(a.jobs))
	}
	if a.cursor < 0 {
		t.Errorf("cursor went negative: %d", a.cursor)
	}
}

// --- mg-jdi stop-notification bell (TASK-11) ---------------------------------

// withFakeBell swaps out ringBell for the duration of the test and returns a
// pointer to a counter tracking how many times it was called.
func withFakeBell(t *testing.T) *int {
	t.Helper()
	count := 0
	orig := ringBell
	ringBell = func() { count++ }
	t.Cleanup(func() { ringBell = orig })
	return &count
}

func mkJDIJob(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "jobs", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief.md"),
		[]byte("# Brief: "+name+"\n\nstatus: open\ndate: 2026-01-01\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPollJDIBellNeverRingsOnFirstObservation confirms a job already
// stopped before the TUI ever polled it does not ring on the very first
// refreshJobs — only a *transition* observed after that first poll should.
func TestPollJDIBellNeverRingsOnFirstObservation(t *testing.T) {
	root := t.TempDir()
	mkJDIJob(t, root, "aaaa01_a")
	if err := job.WriteJDIStatus(root, "aaaa01_a", job.JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	count := withFakeBell(t)

	a.refreshJobs()
	if *count != 0 {
		t.Errorf("bell rang %d times on the first observation of an already-stopped job, want 0", *count)
	}
}

// TestPollJDIBellRingsOnceOnFreshTransition confirms a job observed as
// running, then observed as stopped on a later poll, rings exactly once —
// not again on a third poll where the state hasn't changed.
func TestPollJDIBellRingsOnceOnFreshTransition(t *testing.T) {
	root := t.TempDir()
	mkJDIJob(t, root, "aaaa01_a")
	if err := job.WriteJDIStatus(root, "aaaa01_a", job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	count := withFakeBell(t)

	a.refreshJobs() // seeds jdiSeen[aaaa01_a] = running, no ring
	if *count != 0 {
		t.Fatalf("bell rang %d times while status was still running, want 0", *count)
	}

	if err := job.WriteJDIStatus(root, "aaaa01_a", job.JDIStoppedNeedsHuman, "developer"); err != nil {
		t.Fatal(err)
	}
	a.refreshJobs() // fresh transition running -> stopped:needs-human
	if *count != 1 {
		t.Errorf("bell rang %d times on the fresh stop transition, want 1", *count)
	}

	a.refreshJobs() // same stopped state again, no new transition
	if *count != 1 {
		t.Errorf("bell rang %d times after a second poll with no state change, want still 1", *count)
	}
}

// TestPollJDIBellIgnoresJobsWithNoStatus confirms a job with no mg-jdi
// status sidecar at all (the common case) never rings, and doesn't panic.
func TestPollJDIBellIgnoresJobsWithNoStatus(t *testing.T) {
	root := t.TempDir()
	mkJDIJob(t, root, "aaaa01_a")

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	count := withFakeBell(t)

	a.refreshJobs()
	a.refreshJobs()
	if *count != 0 {
		t.Errorf("bell rang %d times for a job with no status sidecar, want 0", *count)
	}
}
