package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmuskalla/manigot/internal/job"
)

// These tests exercise the app's permanent auto-refresh tick (autoRefreshMsg,
// TASK-1/TASK-2 of the auto-refresh job): the tick triggers the same full
// refresh ctrl+r does, is silent (no status line), never starts a second
// concurrent chain, re-arms from every state, and skips the refresh work in
// the form/overlay states.

// TestAutoRefreshTickTriggersFullRefresh feeds one tick with a detail view
// open and state set to stateDetail: an out-of-band file edit must appear in
// the detail view, a job created on disk must appear in the job list, and a
// project-settings change must be re-read — all without any keypress. The
// returned cmd must be non-nil (the chain re-arms).
func TestAutoRefreshTickTriggersFullRefresh(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_x")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(jobDir, "brief.md")
	if err := os.WriteFile(brief, []byte("# Brief: X\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail

	if strings.Contains(a.detail.tabs[0].viewer.View(), "ZZAUTOREFRESH") {
		t.Fatal("marker already present before the tick")
	}

	// Out-of-band changes while the TUI sits idle: an agent edits brief.md, a
	// second job appears, and the project's base branch setting changes.
	if err := os.WriteFile(brief, []byte("# Brief: X\n\nstatus: open\n\nZZAUTOREFRESH new paragraph.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mkJob := func(name string) {
		dir := filepath.Join(root, "docs", "jobs", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief.md"),
			[]byte("# Brief: "+name+"\n\nstatus: open\ndate: 2026-01-01\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkJob("aa0002_y")
	if err := os.MkdirAll(filepath.Join(root, ".manigot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".manigot", "manigot.json"),
		[]byte(`{"baseBranch": "trunk"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	model, cmd := a.Update(autoRefreshMsg{})
	got := model.(*App)

	if !strings.Contains(got.detail.tabs[0].viewer.View(), "ZZAUTOREFRESH") {
		t.Error("tick did not reload the open detail view; out-of-band edit missing")
	}
	if len(got.jobs) != 2 {
		t.Errorf("jobs = %d after the tick, want 2 (job list must be re-read)", len(got.jobs))
	}
	if got.projectSettings.BaseBranch != "trunk" {
		t.Errorf("projectSettings.BaseBranch = %q after the tick, want %q (project settings must be re-read)", got.projectSettings.BaseBranch, "trunk")
	}
	if cmd == nil {
		t.Error("tick returned nil cmd, want the re-armed next tick")
	}
}

// TestAutoRefreshTickIsSilent confirms the tick never leaks a status line:
// no list status, no detail status, and no status-expiry arming (which would
// fight the blink chain and blink every second).
func TestAutoRefreshTickIsSilent(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_s")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: S\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)
	a.state = stateDetail
	// Baseline: no status anywhere, no expiry chain.
	a.status = ""
	a.statusBlinkOn = false
	a.statusUntil = time.Time{}
	a.statusExpiryTicking = false

	model, cmd := a.Update(autoRefreshMsg{})
	got := model.(*App)

	if got.status != "" {
		t.Errorf("list status = %q after the tick, want empty (refresh must be silent)", got.status)
	}
	if got.detail.status != "" {
		t.Errorf("detail status = %q after the tick, want empty (refresh must be silent)", got.detail.status)
	}
	if got.statusExpiryTicking {
		t.Error("statusExpiryTicking set by the tick, want false (no status-expiry arming from a silent refresh)")
	}
	if got.statusBlinkOn {
		t.Error("statusBlinkOn set by the tick, want false")
	}
	if cmd == nil {
		t.Error("tick returned nil cmd, want the re-armed next tick")
	}
}

// TestAutoRefreshGuardPreventsSecondChain verifies the autoRefreshTicking
// guard: while a chain is already live, a further start attempt is a no-op —
// no duplicate concurrent tick chains — and the chain persists (still armed,
// guard still set) after a tick is handled.
func TestAutoRefreshGuardPreventsSecondChain(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_g")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: G\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)

	first := a.startAutoRefresh()
	if first == nil {
		t.Fatal("startAutoRefresh returned nil, want the first tick cmd")
	}
	if !a.autoRefreshTicking {
		t.Fatal("autoRefreshTicking not set after the first start")
	}
	if second := a.startAutoRefresh(); second != nil {
		t.Error("startAutoRefresh returned a second cmd while already ticking, want nil (no duplicate chains)")
	}

	// The chain persists: a handled tick keeps the guard set and re-arms.
	model, cmd := a.Update(autoRefreshMsg{})
	got := model.(*App)
	if cmd == nil {
		t.Error("tick returned nil cmd, want the re-armed next tick")
	}
	if !got.autoRefreshTicking {
		t.Error("autoRefreshTicking cleared by the tick handler, want still set (the chain never self-terminates)")
	}
}

// TestAutoRefreshSkipsWorkInFormStates confirms the tick re-arms from every
// state but only performs the refresh work in stateList/stateDetail: in the
// form/overlay states (stateNewJob, stateSettings, stateAgents,
// stateConfirm, stateGitPanel) the returned cmd is the next tick while the
// job list is left untouched even when disk has changed underneath.
func TestAutoRefreshSkipsWorkInFormStates(t *testing.T) {
	root := t.TempDir()
	mkJob := func(name string) {
		dir := filepath.Join(root, "docs", "jobs", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief.md"),
			[]byte("# Brief: "+name+"\n\nstatus: open\ndate: 2026-01-01\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkJob("aa0001_a")
	mkJob("bb0002_b")

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	if len(a.jobs) != 2 {
		t.Fatalf("setup: want 2 jobs, got %d", len(a.jobs))
	}

	// Remove a job from disk out-of-band — only a refresh would notice.
	if err := os.RemoveAll(filepath.Join(root, "docs", "jobs", "bb0002_b")); err != nil {
		t.Fatal(err)
	}

	for _, st := range []appState{stateNewJob, stateSettings, stateAgents, stateConfirm, stateGitPanel} {
		a.state = st
		model, cmd := a.Update(autoRefreshMsg{})
		got := model.(*App)
		if cmd == nil {
			t.Errorf("state %v: tick returned nil cmd, want the re-armed next tick", st)
		}
		if len(got.jobs) != 2 {
			t.Errorf("state %v: jobs = %d, want 2 (refresh must be skipped in form/overlay states)", st, len(got.jobs))
		}
	}

	// Contrast: in the list state the same tick refreshes and picks up the
	// removal.
	a.state = stateList
	model, cmd := a.Update(autoRefreshMsg{})
	got := model.(*App)
	if cmd == nil {
		t.Error("stateList: tick returned nil cmd, want the re-armed next tick")
	}
	if len(got.jobs) != 1 {
		t.Errorf("stateList: jobs = %d after the tick, want 1 (refresh must run in the list state)", len(got.jobs))
	}
}

// TestAutoRefreshIntervalPinned pins the cadence decision from TASK-3: the
// brief prefers every 1s, and the performance check confirmed the full
// refresh (with a detail view open on the diff tab) costs well under 1% of
// each second — so the constant stays 1s, not the 5s fallback.
func TestAutoRefreshIntervalPinned(t *testing.T) {
	if autoRefreshInterval != 1*time.Second {
		t.Errorf("autoRefreshInterval = %v, want 1s (TASK-3 confirmed 1s is performance-feasible)", autoRefreshInterval)
	}
}