package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/internal/job"
)

// TestDeleteMsgSuccessReturnsToList verifies that a clean (err == nil)
// deleteMsg always refreshes the job list from disk and returns to the list
// view — mirrors TestDoneMsgSuccessReturnsToList's reasoning: a nil error is
// not itself proof the job was deleted, but re-reading from disk shows the
// true state either way.
func TestDeleteMsgSuccessReturnsToList(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_x")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: X\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	// Simulate the job having been deleted by the delete-job.sh run.
	os.RemoveAll(jobDir)

	// Setting a non-empty status arms the blink-then-expire timer (see
	// App.setStatus/statusExpireMsg), so a follow-up cmd is now expected —
	// unlike before that mechanism existed.
	model, cmd := a.Update(deleteMsg{err: nil})
	if cmd == nil {
		t.Errorf("expected a follow-up cmd arming the status-expiry timer, got nil")
	}
	got := model.(*App)

	if got.state != stateList {
		t.Errorf("state = %v, want stateList", got.state)
	}
	if got.detail != nil {
		t.Errorf("detail should be nil after a deleteMsg, success or not")
	}
	if len(got.jobs) != 0 {
		t.Errorf("job list should be empty after refresh picks up the deletion; got %d job(s)", len(got.jobs))
	}
	if got.status != "refreshed" {
		t.Errorf("status = %q, want %q", got.status, "refreshed")
	}
}

// TestDeleteMsgDeclinedStillReturnsToList mirrors
// TestDoneMsgDeclinedStillReturnsToList: delete-job.sh's own read -rp prompt
// exits 0 on decline too, so a declined delete just leaves the job present
// in the re-read list.
func TestDeleteMsgDeclinedStillReturnsToList(t *testing.T) {
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
	model, _ := a.Update(deleteMsg{err: nil})
	got := model.(*App)

	if got.state != stateList {
		t.Errorf("state = %v, want stateList", got.state)
	}
	if len(got.jobs) != 1 {
		t.Errorf("declined job should still be present after refresh; got %d job(s)", len(got.jobs))
	}
}

// TestDeleteMsgErrorSurfacesAndReturnsToList verifies a non-zero exit still
// surfaces through cmdErrorText, but the view falls back to the list exactly
// as the success path does.
func TestDeleteMsgErrorSurfacesAndReturnsToList(t *testing.T) {
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
	model, cmd := a.Update(deleteMsg{err: wantErr})
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

// TestDeleteKeyOpensConfirmView verifies the "delete"/"x" keys in the detail
// view open the TUI-side confirmation (the in-process replacement for the
// delete-job.sh subprocess). Both triggers are checked — the physical
// Delete/Entf key and its "x" fallback — since the whole point of the
// fallback is that it must dispatch identically when the real key's escape
// sequence isn't decoded by a given terminal.
func TestDeleteKeyOpensConfirmView(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"delete key": {Type: tea.KeyDelete},
		"x fallback": keyMsg("x"),
	}
	for name, key := range keys {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			jobDir := filepath.Join(root, "docs", "jobs", "ab0004_w")
			os.MkdirAll(jobDir, 0o755)
			os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: W\n\nstatus: open\n"), 0o644)

			jobs, _ := job.Discover(root)
			a := NewApp(root, jobs)
			a.width, a.height = 80, 24
			a.detail = newDetailView(a.jobs[0], 80, 24)
			a.state = stateDetail

			model, cmd := a.updateDetail(key)
			if cmd != nil {
				t.Errorf("expected no follow-up cmd from opening the confirm view, got one")
			}
			got := model.(*App)
			if got.state != stateConfirm || got.confirm == nil {
				t.Fatalf("state = %v, confirm nil = %v — want stateConfirm with a confirm view", got.state, got.confirm == nil)
			}
			if got.confirm.action != confirmDelete {
				t.Errorf("confirm action = %v, want confirmDelete", got.confirm.action)
			}
		})
	}
}

// TestConfirmViewRendersDeleteInfo verifies the delete confirmation shows the
// script's summary lines (title, worktree, branch, the cannot-be-undone
// warning).
func TestConfirmViewRendersDeleteInfo(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0005_v")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: V\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.openConfirm(confirmDelete)

	out := a.confirm.render()
	for _, want := range []string{"Delete job", "Title", "Worktree", "This cannot be undone.", "y/enter proceed"} {
		if !contains(out, want) {
			t.Errorf("confirm render missing %q:\n%s", want, out)
		}
	}
}
