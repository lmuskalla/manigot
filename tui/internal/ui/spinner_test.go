package ui

import (
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/job"
)

// These tests exercise the activity indicator's timer-driven tick chain
// (TASK-2): the App's spinnerTickMsg handler advances the step counter and
// keeps the chain alive only while a job is actually running, ends it the
// moment the run stops, and never starts one without a run at all.

// TestSpinnerTickAdvancesStepAndContinuesWhileRunning feeds one tick while
// the sidecar reports JDIRunning: the step must advance, the handler must
// return the next tick cmd (the chain continues), the guard must stay set,
// and the step must be threaded into an open detail view.
func TestSpinnerTickAdvancesStepAndContinuesWhileRunning(t *testing.T) {
	root := t.TempDir()
	mkJDIJob(t, root, "aaaa01_a")
	if err := job.WriteJDIStatus(root, "aaaa01_a", job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	model, cmd := a.Update(spinnerTickMsg{})
	got := model.(*App)
	if got.spinnerStep != 1 {
		t.Errorf("spinnerStep = %d after one tick, want 1", got.spinnerStep)
	}
	if cmd == nil {
		t.Error("tick while a run is active returned nil, want the next tick cmd")
	}
	if !got.spinnerTicking {
		t.Error("spinnerTicking = false while a run is active, want true")
	}
	if got.detail.spinnerStep != 1 {
		t.Errorf("detail.spinnerStep = %d after one tick, want 1 (threaded from the App)", got.detail.spinnerStep)
	}
}

// TestSpinnerTickChainEndsWhenRunStops verifies the chain's termination: once
// the sidecar flips to a stopped state (as mg-jdi does at loop exit), the
// next tick returns nil and clears the guard — otherwise the spinner would
// spin forever against a dead run.
func TestSpinnerTickChainEndsWhenRunStops(t *testing.T) {
	root := t.TempDir()
	mkJDIJob(t, root, "aaaa01_a")
	if err := job.WriteJDIStatus(root, "aaaa01_a", job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.spinnerTicking = true // simulate a live chain

	if err := job.WriteJDIStatus(root, "aaaa01_a", job.JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}

	model, cmd := a.Update(spinnerTickMsg{})
	got := model.(*App)
	if cmd != nil {
		t.Error("tick after the run stopped returned a cmd, want nil (the chain must end)")
	}
	if got.spinnerTicking {
		t.Error("spinnerTicking still true after the run stopped, want false")
	}
}

// TestSpinnerTickNoCmdWithoutRun feeds a stray tick with no run at all (no
// sidecar, nothing in jdiSeen): the handler must produce no next-tick cmd and
// must not set the guard — the chain cannot start from nowhere.
func TestSpinnerTickNoCmdWithoutRun(t *testing.T) {
	root := t.TempDir()
	mkJDIJob(t, root, "aaaa01_a")

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)

	_, cmd := a.Update(spinnerTickMsg{})
	if cmd != nil {
		t.Error("tick with no run returned a cmd, want nil")
	}
	if a.spinnerTicking {
		t.Error("spinnerTicking set with no run at all, want false")
	}
}

// TestStartSpinnerIfRunningDoesNotDoubleStart verifies the spinnerTicking
// guard: while a chain is already live, a further start attempt is a no-op —
// no duplicate concurrent tick chains (TASK-2's stated risk).
func TestStartSpinnerIfRunningDoesNotDoubleStart(t *testing.T) {
	root := t.TempDir()
	mkJDIJob(t, root, "aaaa01_a")
	if err := job.WriteJDIStatus(root, "aaaa01_a", job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)

	first := a.startSpinnerIfRunning()
	if first == nil {
		t.Fatal("startSpinnerIfRunning with a live run returned nil, want the first tick cmd")
	}
	if !a.spinnerTicking {
		t.Fatal("spinnerTicking not set after the first start")
	}
	if second := a.startSpinnerIfRunning(); second != nil {
		t.Error("startSpinnerIfRunning returned a second cmd while already ticking, want nil (no duplicate chains)")
	}
}
