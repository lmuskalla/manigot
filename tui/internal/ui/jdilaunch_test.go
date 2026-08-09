package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/job"
)

// TestJdiKeyLaunchesDetachedAndSeedsBellDedup exercises the full "J" flow on
// a current-branch job: it resolves and starts the stub mg-jdi (no spawned
// terminal — Decision 7a), reports a status message, and seeds a.jdiSeen so
// the stop-notification dedup (TASK-11) starts at "running" immediately
// rather than waiting for the first poll to discover it.
func TestJdiKeyLaunchesDetachedAndSeedsBellDedup(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "cur21_c", "# Brief: Cur\n\nstatus: open\nid: cur21\nbranch: "+def+"\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 || !jobs[0].OnCurrentBranch {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	// Stub mg-jdi so no real container/process is spawned. PATH is left
	// alone (unlike hostcmd's isolate() pattern) — branchGuard shells out to
	// the real git binary as part of this same "J" flow, and the env-var
	// override below wins resolution regardless of what's on PATH anyway.
	t.Setenv("MANIGOT_HOME", "")
	stub := filepath.Join(t.TempDir(), "stub.sh")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_JDI_BIN", stub)

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	_, cmd := a.updateDetail(keyMsg("J"))
	if cmd != nil {
		t.Errorf("updateDetail(J) returned a non-nil cmd, want nil (Jdi runs synchronously to Start(), no tea.Cmd needed)")
	}
	if !strings.Contains(a.detail.status, "mg-jdi") {
		t.Errorf("status = %q, want it to mention mg-jdi starting", a.detail.status)
	}
	if got := a.jdiSeen[jobs[0].Name]; got != job.JDIRunning {
		t.Errorf("jdiSeen[%q] = %q, want %q (seeded immediately on launch)", jobs[0].Name, got, job.JDIRunning)
	}
}

// TestJdiKeyReportsResolutionFailure verifies a failed resolution (mg-jdi
// not installed) surfaces as a footer status rather than panicking or
// silently doing nothing.
func TestJdiKeyReportsResolutionFailure(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "cur22_c", "# Brief: Cur\n\nstatus: open\nid: cur22\nbranch: "+def+"\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	// PATH is left alone (see the comment in the test above) — mg-jdi is
	// simply not installed under that name in this test environment, and
	// the env var override is what's actually under test here.
	t.Setenv("MANIGOT_JDI_BIN", "")
	t.Setenv("MANIGOT_HOME", "")

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("J"))
	if !strings.Contains(a.detail.status, "not found") {
		t.Errorf("status = %q, want it to explain mg-jdi could not be resolved", a.detail.status)
	}
	if _, ok := a.jdiSeen[jobs[0].Name]; ok {
		t.Error("jdiSeen should not be seeded when the launch itself failed")
	}
}
