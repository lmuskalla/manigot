package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmuskalla/manigot/tui/internal/job"
)

// TestJdiKeyLaunchesDetachedAndSeedsBellDedup exercises the full "j" flow on
// a job in its own worktree: it resolves and starts the stub mg-jdi (no
// spawned terminal — Decision 7a), reports a status message, and seeds
// a.jdiSeen so the stop-notification dedup (TASK-11) starts at "running"
// immediately rather than waiting for the first poll to discover it.
func TestJdiKeyLaunchesDetachedAndSeedsBellDedup(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur21_c", "cur21_c", "# Brief: Cur\n\nstatus: open\nid: cur21\nbranch: feature/cur21_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	// Stub mg-jdi so no real container/process is spawned. PATH is left
	// alone (unlike hostcmd's isolate() pattern) — the env-var override
	// below wins resolution regardless of what's on PATH anyway.
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

	_, cmd := a.updateDetail(keyMsg("j"))
	if cmd == nil {
		t.Errorf("updateDetail(j) returned a nil cmd, want the activity-indicator tick (the launch seeds jdiSeen=running, so the spinner must start)")
	}
	if !strings.Contains(a.detail.status, "mg jdi") {
		t.Errorf("status = %q, want it to mention mg jdi starting", a.detail.status)
	}
	if got := a.jdiSeen[jobs[0].Name]; got != job.JDIRunning {
		t.Errorf("jdiSeen[%q] = %q, want %q (seeded immediately on launch)", jobs[0].Name, got, job.JDIRunning)
	}
}

// TestJdiKeyReportsResolutionFailure verifies a failed resolution (mg-jdi
// not installed) surfaces as a footer status rather than panicking or
// silently doing nothing.
func TestJdiKeyReportsResolutionFailure(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur22_c", "cur22_c", "# Brief: Cur\n\nstatus: open\nid: cur22\nbranch: feature/cur22_c\ndate: 2026-01-01\n")

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

	a.updateDetail(keyMsg("j"))
	if !strings.Contains(a.detail.status, "not found") {
		t.Errorf("status = %q, want it to explain mg-jdi could not be resolved", a.detail.status)
	}
	if _, ok := a.jdiSeen[jobs[0].Name]; ok {
		t.Error("jdiSeen should not be seeded when the launch itself failed")
	}
}

// --- launch block (TASK-1 of the "multiple jdi instances" job) -------------

// markerStub writes a stub mg-jdi script under t.TempDir() that appends one
// line to markerPath every time it runs, so tests below can assert on how
// many times (if any) launch.Jdi actually started it — the thing the launch
// block must prevent from happening a second time.
func markerStub(t *testing.T, markerPath string) string {
	t.Helper()
	stub := filepath.Join(t.TempDir(), "stub.sh")
	script := "#!/bin/sh\necho ran >> " + markerPath + "\nexit 0\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return stub
}

// countMarkerRuns reports how many times the stub written by markerStub has
// actually run so far. 0 (not an error) when the marker file doesn't exist
// yet — the common case of a blocked launch never having started anything.
func countMarkerRuns(t *testing.T, markerPath string) int {
	t.Helper()
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "ran")
}

// waitForMarkerRuns polls countMarkerRuns until it reaches want or a short
// deadline passes — launch.Jdi only starts the stub asynchronously
// (cmd.Start(), reaped in a goroutine), so a launch that should run the
// stub needs a moment to actually write its marker line rather than being
// checked the instant updateDetail returns.
func waitForMarkerRuns(t *testing.T, markerPath string, want int) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	got := countMarkerRuns(t, markerPath)
	for got < want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		got = countMarkerRuns(t, markerPath)
	}
	return got
}

// TestJdiKeyBlocksSecondLaunchWhenSidecarSaysRunning is TASK-1's core case:
// the on-disk sidecar already reporting job.JDIRunning (e.g. mg-jdi is
// mid-run from a previous "j" press, possibly even a previous TUI session)
// must block a second launch outright and explain why, naming the running
// agent per the list badge's own wording.
func TestJdiKeyBlocksSecondLaunchWhenSidecarSaysRunning(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur23_c", "cur23_c", "# Brief: Cur\n\nstatus: open\nid: cur23\nbranch: feature/cur23_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	if err := job.WriteJDIStatus(dir, jobs[0].Name, job.JDIRunning, "developer"); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("MANIGOT_JDI_BIN", markerStub(t, marker))

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("j"))

	if !strings.Contains(a.detail.status, "already running") {
		t.Errorf("status = %q, want it to explain mg-jdi is already running", a.detail.status)
	}
	if !strings.Contains(a.detail.status, "@developer") {
		t.Errorf("status = %q, want it to name the running agent (@developer)", a.detail.status)
	}
	if got := countMarkerRuns(t, marker); got != 0 {
		t.Errorf("stub mg-jdi ran %d time(s), want 0 (the second launch should have been blocked)", got)
	}
}

// TestJdiKeyBlocksSecondLaunchViaInSessionDedup covers the race TASK-1 calls
// out: a press landing before mg-jdi has written its own first status file
// (so the sidecar is still absent) must still be caught, via a.jdiSeen —
// the same in-memory map updateDetail's "j" handler seeds on a successful
// launch.
func TestJdiKeyBlocksSecondLaunchViaInSessionDedup(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur24_c", "cur24_c", "# Brief: Cur\n\nstatus: open\nid: cur24\nbranch: feature/cur24_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("MANIGOT_JDI_BIN", markerStub(t, marker))

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// First press: no sidecar exists yet (job.ReadJDIStatus's ok=false), so
	// this launches and seeds jdiSeen — mirroring TestJdiKeyLaunchesDetached
	// AndSeedsBellDedup above, but without waiting for the sidecar the real
	// mg-jdi binary would eventually write.
	a.updateDetail(keyMsg("j"))
	if got := waitForMarkerRuns(t, marker, 1); got != 1 {
		t.Fatalf("first press: stub mg-jdi ran %d time(s), want exactly 1", got)
	}
	if got := a.jdiSeen[jobs[0].Name]; got != job.JDIRunning {
		t.Fatalf("jdiSeen not seeded after first press: got %q", got)
	}

	// Second press, immediately after: still no sidecar file (the stub never
	// writes one), so only a.jdiSeen can catch this — and must.
	a.updateDetail(keyMsg("j"))
	if !strings.Contains(a.detail.status, "already running") {
		t.Errorf("status = %q, want it to explain mg-jdi is already running", a.detail.status)
	}
	if got := countMarkerRuns(t, marker); got != 1 {
		t.Errorf("stub mg-jdi ran %d time(s) after the second press, want still 1 (blocked)", got)
	}
}

// TestJdiKeyAllowsLaunchWhenSidecarStoppedDespiteStaleSession confirms the
// on-disk sidecar always wins over a.jdiSeen: once mg-jdi itself has written
// a stopped:* status, a fresh launch is allowed even if this session's
// jdiSeen still remembers having launched it earlier (e.g. the TUI never
// polled/refreshed since) — TASK-1's "too strict permanently blocks
// re-running a job" failure mode.
func TestJdiKeyAllowsLaunchWhenSidecarStoppedDespiteStaleSession(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur25_c", "cur25_c", "# Brief: Cur\n\nstatus: open\nid: cur25\nbranch: feature/cur25_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	if err := job.WriteJDIStatus(dir, jobs[0].Name, job.JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("MANIGOT_JDI_BIN", markerStub(t, marker))

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail
	// Simulate a stale in-session memory of an earlier run that the TUI
	// never got a chance to observe stopping (no refreshJobs/pollJDIBell in
	// between).
	a.jdiSeen[jobs[0].Name] = job.JDIRunning

	a.updateDetail(keyMsg("j"))

	if strings.Contains(a.detail.status, "already running") {
		t.Errorf("status = %q, launch should not have been blocked (sidecar says stopped)", a.detail.status)
	}
	if got := waitForMarkerRuns(t, marker, 1); got != 1 {
		t.Errorf("stub mg-jdi ran %d time(s), want exactly 1 (the on-disk stopped status should have allowed the launch)", got)
	}
}

// TestJdiKeyAllowsLaunchAfterInSessionDedupExpiresWithNoSidecar covers the
// gap the reviewer found in the first pass: a jdiSeen entry left behind by a
// launch whose mg-jdi process crashed or was killed before ever writing a
// sidecar file (so, unlike the test above, there is no on-disk status at
// all — not "exists and says stopped") must not block a re-launch forever.
// Once it's older than jdiSeenFallbackTTL, jdiAlreadyRunning must stop
// trusting it and allow a fresh launch, exactly as it already does for a
// stale-but-present on-disk status.
func TestJdiKeyAllowsLaunchAfterInSessionDedupExpiresWithNoSidecar(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur26_c", "cur26_c", "# Brief: Cur\n\nstatus: open\nid: cur26\nbranch: feature/cur26_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("MANIGOT_JDI_BIN", markerStub(t, marker))

	// Freeze jdiNow so the TTL can be crossed deterministically without an
	// actual sleep.
	now := time.Now()
	origNow := jdiNow
	jdiNow = func() time.Time { return now }
	defer func() { jdiNow = origNow }()

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// Simulate a launch whose mg-jdi died before writing any status file at
	// all: jdiSeen still says JDIRunning, but jdiSeenAt is older than
	// jdiSeenFallbackTTL, and there is no sidecar on disk (job.WriteJDIStatus
	// was never called).
	a.jdiSeen[jobs[0].Name] = job.JDIRunning
	a.jdiSeenAt[jobs[0].Name] = now.Add(-jdiSeenFallbackTTL - time.Second)

	a.updateDetail(keyMsg("j"))

	if strings.Contains(a.detail.status, "already running") {
		t.Errorf("status = %q, launch should not have been blocked (stale jdiSeen entry with no sidecar must expire)", a.detail.status)
	}
	if got := waitForMarkerRuns(t, marker, 1); got != 1 {
		t.Errorf("stub mg-jdi ran %d time(s), want exactly 1 (the expired jdiSeen entry should have allowed the launch)", got)
	}
}
