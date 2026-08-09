package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/safecode/tui/internal/job"
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
	d.setStatus("error: sc-job not found\ntried: a, b\nfix:   set $SAFECODE_JOB_BIN")

	multiLineLines := strings.Split(d.render(), "\n")
	if len(multiLineLines) > height {
		t.Errorf("render with 3-line footer used %d rows, want <= %d (footer got clipped by the alt-screen viewport)", len(multiLineLines), height)
	}

	// The fix line must actually be present in the rendered output, not just
	// in d.status — i.e. it must not have been truncated by the viewport.
	if !strings.Contains(d.render(), "fix:   set $SAFECODE_JOB_BIN") {
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
	wideLines := d.tabs[1].viewer.LineCount()

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
