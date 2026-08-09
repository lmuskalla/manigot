package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lmuskalla/safecode/tui/internal/job"
)

// TestDoneMsgSuccessReturnsToList verifies that a clean (err == nil) doneMsg
// always refreshes the job list from disk and returns to the list view — per
// Q2 in tasks.md, a nil error is not itself proof the job was archived, but
// re-reading from disk shows the true state either way.
func TestDoneMsgSuccessReturnsToList(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_x")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: X\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	// Simulate the job having been archived by the finish-job.sh run.
	os.RemoveAll(jobDir)

	model, cmd := a.Update(doneMsg{err: nil})
	if cmd != nil {
		t.Errorf("expected no follow-up cmd, got one")
	}
	got := model.(*App)

	if got.state != stateList {
		t.Errorf("state = %v, want stateList", got.state)
	}
	if got.detail != nil {
		t.Errorf("detail should be nil after a doneMsg, success or not")
	}
	if len(got.jobs) != 0 {
		t.Errorf("job list should be empty after refresh picks up the archive; got %d job(s)", len(got.jobs))
	}
	if got.status != "refreshed" {
		t.Errorf("status = %q, want %q", got.status, "refreshed")
	}
}

// TestDoneMsgDeclinedStillReturnsToList mirrors the "user answered N to a
// finish-job.sh prompt" case from Q2: the process still exits 0, but the job
// was never archived. The doneMsg handler cannot tell the difference from a
// real success — it must fall back to the list either way, and the still-
// present job proves the true (unarchived) state to the user.
func TestDoneMsgDeclinedStillReturnsToList(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0002_y")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: Y\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	// Job dir untouched — as if the user declined a confirmation.
	model, _ := a.Update(doneMsg{err: nil})
	got := model.(*App)

	if got.state != stateList {
		t.Errorf("state = %v, want stateList", got.state)
	}
	if len(got.jobs) != 1 {
		t.Errorf("declined job should still be present after refresh; got %d job(s)", len(got.jobs))
	}
}

// TestDoneMsgErrorSurfacesAndReturnsToList verifies a non-zero exit still
// surfaces through cmdErrorText, but the view falls back to the list exactly
// as the success path does (per TASK-7).
func TestDoneMsgErrorSurfacesAndReturnsToList(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0003_z")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: Z\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	wantErr := errors.New("exit status 1")
	model, cmd := a.Update(doneMsg{err: wantErr})
	if cmd != nil {
		t.Errorf("expected no follow-up cmd, got one")
	}
	got := model.(*App)

	if got.state != stateList {
		t.Errorf("state = %v, want stateList", got.state)
	}
	want := cmdErrorText(wantErr)
	if got.status != want {
		t.Errorf("status = %q, want %q", got.status, want)
	}
}

// TestDoneCmdResolutionFailureSurfacesNotFoundError verifies the "D" key
// handler surfaces a resolution failure through cmdErrorText's NotFoundError
// branch, the same way agent launch and "e" edit failures do — exercised via
// a.doneCmd() directly (mirroring hostcmd's own resolution-failure test)
// rather than through a real tea.ExecProcess run.
func TestDoneCmdResolutionFailureSurfacesNotFoundError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("SAFECODE_DONE_BIN", "")
	t.Setenv("SAFECODE_HOME", "")

	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0004_w")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: W\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)

	_, err := a.doneCmd()
	if err == nil {
		t.Fatal("expected a resolution error when sc-done cannot be found")
	}
	if got := cmdErrorText(err); got == "error: "+err.Error() {
		t.Errorf("expected the NotFoundError diagnosis branch of cmdErrorText, got the plain one: %q", got)
	}
}
