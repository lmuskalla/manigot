package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lmuskalla/manigot/internal/job"
)

// mkStatusTestJob scaffolds a minimal open job (brief.md only) under root and
// returns the discovered job.Job list — the shared setup for this file's
// detail-status regression tests, which don't care about a real git repo.
func mkStatusTestJob(t *testing.T, root, dirName string) []job.Job {
	t.Helper()
	jobDir := filepath.Join(root, "docs", "jobs", dirName)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: S\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}
	return jobs
}

// TestDetailSetStatusArmsExpiryDeadline is a regression test for the bug this
// job fixes: detailView.setStatus used to set only d.status, never
// d.statusUntil — leaving it at its zero value, which the statusExpireMsg
// handler treats as "already past deadline" (see statusVisible). The result
// was that any detail-view status (an agent launch confirmation, a git
// action's result, an error) was cleared on the very next blink tick — around
// 200ms after being set — instead of the intended ~3s statusLifetime. This
// pins that setStatus now arms a real future deadline, mirroring App.setStatus
// for the list footer.
func TestDetailSetStatusArmsExpiryDeadline(t *testing.T) {
	root := t.TempDir()
	jobs := mkStatusTestJob(t, root, "stat01_s")

	now := time.Now()
	origNow := statusNow
	statusNow = func() time.Time { return now }
	defer func() { statusNow = origNow }()

	d := newDetailView(jobs[0], 80, 24)
	d.setStatus("→ something happened")

	if d.status != "→ something happened" {
		t.Fatalf("status = %q, want the set text", d.status)
	}
	if !d.statusUntil.After(now) {
		t.Fatalf("statusUntil = %v, want a deadline after now (%v) — got the zero-value bug back", d.statusUntil, now)
	}
	wantUntil := now.Add(statusLifetime)
	if !d.statusUntil.Equal(wantUntil) {
		t.Errorf("statusUntil = %v, want now + statusLifetime = %v", d.statusUntil, wantUntil)
	}

	d.setStatus("")
	if !d.statusUntil.IsZero() {
		t.Errorf("statusUntil = %v after clearing status, want zero", d.statusUntil)
	}
}

// TestDetailStatusSurvivesUntilItsDeadlineThroughTickChain is the end-to-end
// regression test: it drives a detail-view status through the same
// statusExpireMsg tick chain the running app uses (via a commitMsg, one of
// the ~23 call sites that sets a.detail's status) and verifies the status
// text stays put — and visible — for the ticks inside its statusLifetime
// window, not just the first one. Before this fix, the very first tick
// (armed by any a.detail.setStatus caller's now-correctly-returned
// armStatusExpiry cmd) cleared the status immediately, regardless of how much
// of its 3s lifetime had actually elapsed.
func TestDetailStatusSurvivesUntilItsDeadlineThroughTickChain(t *testing.T) {
	root := t.TempDir()
	jobs := mkStatusTestJob(t, root, "stat02_s")

	now := time.Now()
	origNow := statusNow
	statusNow = func() time.Time { return now }
	defer func() { statusNow = origNow }()

	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	model, cmd := a.Update(commitMsg{err: nil})
	a = model.(*App)
	if cmd == nil {
		t.Fatal("expected commitMsg handling to arm the status-expiry tick chain, got nil cmd")
	}
	if a.detail.status != "edited and committed brief.md" {
		t.Fatalf("status = %q, want the commit confirmation", a.detail.status)
	}

	// Well inside the lifetime, before the blink window: a tick must leave
	// the status text and its visibility alone.
	now = now.Add(500 * time.Millisecond)
	model, cmd = a.Update(statusExpireMsg{})
	a = model.(*App)
	if a.detail.status == "" {
		t.Fatal("detail status was cleared well before its statusLifetime elapsed — the instant-clear regression")
	}
	if !statusVisible(a.detail.status, a.detail.statusUntil, a.detail.statusBlinkOn, statusNow()) {
		t.Errorf("detail status should still render as visible before the blink window")
	}
	if cmd == nil {
		t.Fatal("expected the tick chain to continue while a status remains")
	}

	// Past the full lifetime: the next tick clears it.
	now = now.Add(statusLifetime)
	model, _ = a.Update(statusExpireMsg{})
	a = model.(*App)
	if a.detail.status != "" {
		t.Errorf("status = %q after its lifetime elapsed, want cleared", a.detail.status)
	}
}
