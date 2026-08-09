package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/lmuskalla/safecode/tui/internal/job"
)

// TestRefreshPicksUpFileEdits verifies that App.refresh re-reads the detail
// view's files after an agent edits them on disk.
func TestRefreshPicksUpFileEdits(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "processes", "ab0001_x")
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
		dir := filepath.Join(root, "docs", "processes", name)
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
	os.RemoveAll(filepath.Join(root, "docs", "processes", "bb0002_b"))

	a.refresh()
	if a.cursor >= len(a.jobs) {
		t.Errorf("cursor = %d after refresh, but only %d jobs remain", a.cursor, len(a.jobs))
	}
	if a.cursor < 0 {
		t.Errorf("cursor went negative: %d", a.cursor)
	}
}
