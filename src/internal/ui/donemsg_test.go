package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
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

	// Setting a non-empty status arms the blink-then-expire timer (see
	// App.setStatus/statusExpireMsg), so a follow-up cmd is now expected —
	// unlike before that mechanism existed.
	model, cmd := a.Update(doneMsg{err: nil})
	if cmd == nil {
		t.Errorf("expected a follow-up cmd arming the status-expiry timer, got nil")
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

	// Setting a non-empty status arms the blink-then-expire timer (see
	// App.setStatus/statusExpireMsg), so a follow-up cmd is now expected —
	// unlike before that mechanism existed.
	wantErr := errors.New("exit status 1")
	model, cmd := a.Update(doneMsg{err: wantErr})
	if cmd == nil {
		t.Errorf("expected a follow-up cmd arming the status-expiry timer, got nil")
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

// TestDoneMsgGitSolverHandoffShowsHandoffStatus verifies a doneMsg carrying
// ErrGitSolverHandoff — the TUI's pre-approved yesConfirm always accepts the
// git-solver offer on a failed merge, so this is the common TUI outcome of a
// conflicted `mg done` — renders a friendly handoff status instead of an
// error.
func TestDoneMsgGitSolverHandoffShowsHandoffStatus(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0007_h")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: H\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	model, _ := a.Update(doneMsg{err: job.ErrGitSolverHandoff})
	got := model.(*App)

	if got.state != stateList {
		t.Errorf("state = %v, want stateList", got.state)
	}
	if !strings.Contains(got.status, "@git-solver") {
		t.Errorf("status = %q, want a @git-solver handoff message", got.status)
	}
	if strings.HasPrefix(got.status, "error: ") {
		t.Errorf("handoff rendered as an error: %q", got.status)
	}
}

// TestDoneKeyOpensConfirmView verifies the "D" key in the detail view opens
// the TUI-side confirmation (the in-process replacement for the finish-job.sh
// subprocess) rather than spawning anything.
func TestDoneKeyOpensConfirmView(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0004_w")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: W\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	model, cmd := a.updateDetail(keyMsg("D"))
	if cmd != nil {
		t.Errorf("expected no follow-up cmd from opening the confirm view, got one")
	}
	got := model.(*App)
	if got.state != stateConfirm || got.confirm == nil {
		t.Fatalf("state = %v, confirm nil = %v — want stateConfirm with a confirm view", got.state, got.confirm == nil)
	}
	if got.confirm.action != confirmDone {
		t.Errorf("confirm action = %v, want confirmDone", got.confirm.action)
	}
}

// TestConfirmViewCancelReturnsToDetail verifies n/esc in the confirm view
// returns to the detail view without running anything.
func TestConfirmViewCancelReturnsToDetail(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0005_v")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: V\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.openConfirm(confirmDelete)

	for _, k := range []string{"n", "N", "esc", "q"} {
		t.Run(k, func(t *testing.T) {
			a.openConfirm(confirmDelete)
			model, cmd := a.Update(keyMsg(k))
			if cmd != nil {
				t.Errorf("cancel should not run a lifecycle command, got one")
			}
			got := model.(*App)
			if got.state != stateDetail || got.confirm != nil {
				t.Errorf("after %q: state = %v, confirm nil = %v — want back in the detail view", k, got.state, got.confirm == nil)
			}
		})
	}
}

// TestConfirmViewYesRunsLifecycle verifies y/enter in the confirm view returns
// to the detail view and returns the lifecycle command (without executing it —
// the job package's own tests cover the git behavior).
func TestConfirmViewYesRunsLifecycle(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0006_u")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: U\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)

	a.openConfirm(confirmDone)
	model, cmd := a.Update(keyMsg("y"))
	if cmd == nil {
		t.Fatal("confirming done should return a lifecycle command")
	}
	got := model.(*App)
	if got.state != stateDetail || got.confirm != nil {
		t.Errorf("after y: state = %v, confirm nil = %v — want back in the detail view", got.state, got.confirm == nil)
	}

	// The delete action returns its own lifecycle command too.
	a.openConfirm(confirmDelete)
	_, cmd = a.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("confirming delete should return a lifecycle command")
	}
}
