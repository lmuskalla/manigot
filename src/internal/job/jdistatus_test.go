package job

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWriteThenReadJDIStatus(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"

	if err := WriteJDIStatus(root, jobName, JDIRunning, "developer"); err != nil {
		t.Fatalf("WriteJDIStatus: %v", err)
	}

	got, ok := ReadJDIStatus(root, jobName)
	if !ok {
		t.Fatal("ReadJDIStatus: expected ok=true right after a fresh write")
	}
	if got.State != JDIRunning {
		t.Errorf("State = %q, want %q", got.State, JDIRunning)
	}
	if got.Agent != "developer" {
		t.Errorf("Agent = %q, want %q", got.Agent, "developer")
	}
	if time.Since(got.Updated) > time.Minute {
		t.Errorf("Updated = %v, want close to now", got.Updated)
	}
}

func TestReadJDIStatusMissingFile(t *testing.T) {
	root := t.TempDir()
	_, ok := ReadJDIStatus(root, "no-such-job")
	if ok {
		t.Error("ReadJDIStatus with no sidecar file: expected ok=false")
	}
}

func TestRemoveJDIStatus(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"

	// No sidecar ever existed: nothing to remove, and no error.
	removed, err := RemoveJDIStatus(root, jobName)
	if err != nil {
		t.Fatalf("RemoveJDIStatus (absent): %v", err)
	}
	if removed {
		t.Error("RemoveJDIStatus (absent) reported removed=true")
	}

	// A real sidecar (status + run.log) is removed wholesale.
	if err := WriteJDIStatus(root, jobName, JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatalf("WriteJDIStatus: %v", err)
	}
	if err := os.WriteFile(JDIRunLogPath(root, jobName), []byte("log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err = RemoveJDIStatus(root, jobName)
	if err != nil {
		t.Fatalf("RemoveJDIStatus: %v", err)
	}
	if !removed {
		t.Error("RemoveJDIStatus (present) reported removed=false")
	}
	if _, err := os.Stat(JDIStatusDir(root, jobName)); !os.IsNotExist(err) {
		t.Errorf("sidecar dir still exists after RemoveJDIStatus: %v", err)
	}

	// Second removal of an already-gone sidecar: absent, not an error.
	removed, err = RemoveJDIStatus(root, jobName)
	if err != nil || removed {
		t.Errorf("RemoveJDIStatus (after removal) = removed=%v err=%v, want false, nil", removed, err)
	}
}

func TestReadJDIStatusUnparseableFile(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(JDIStatusPath(root, jobName), []byte("not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok := ReadJDIStatus(root, jobName)
	if ok {
		t.Error("ReadJDIStatus on an unparseable file: expected ok=false")
	}
}

func TestReadJDIStatusUnknownState(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"state":"bogus","agent":"","updated":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
	if err := os.WriteFile(JDIStatusPath(root, jobName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok := ReadJDIStatus(root, jobName)
	if ok {
		t.Error("ReadJDIStatus with an unknown state value: expected ok=false")
	}
}

func TestReadJDIStatusStaleRunningDegradesToNoRun(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-2 * jdiRunningStaleAfter).UTC().Format(time.RFC3339)
	body := `{"state":"running","agent":"developer","updated":"` + stale + `"}`
	if err := os.WriteFile(JDIStatusPath(root, jobName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok := ReadJDIStatus(root, jobName)
	if ok {
		t.Error("ReadJDIStatus with a stale 'running' status: expected ok=false (degrade to no run in progress)")
	}
}

func TestStaleRunningJDIReportsStaleRunning(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-2 * jdiRunningStaleAfter).UTC().Format(time.RFC3339)
	body := `{"state":"running","agent":"developer","updated":"` + stale + `"}`
	if err := os.WriteFile(JDIStatusPath(root, jobName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	st, ok := StaleRunningJDI(root, jobName)
	if !ok {
		t.Fatal("StaleRunningJDI with a stale 'running' status: expected ok=true (the run died without reporting a stop)")
	}
	if st.State != JDIRunning || st.Agent != "developer" {
		t.Errorf("got %+v, want State=%q Agent=%q", st, JDIRunning, "developer")
	}
}

func TestStaleRunningJDINotStale(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := WriteJDIStatus(root, jobName, JDIRunning, "analyst"); err != nil {
		t.Fatal(err)
	}
	if _, ok := StaleRunningJDI(root, jobName); ok {
		t.Error("StaleRunningJDI with a fresh 'running' status: expected ok=false (a live run must not look crashed)")
	}
}

func TestStaleRunningJDIStoppedOrMissingIsNotCrash(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"

	// An old stopped:* status is a terminal state, not a crash.
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * jdiRunningStaleAfter).UTC().Format(time.RFC3339)
	body := `{"state":"stopped:finished","agent":"reviewer","updated":"` + old + `"}`
	if err := os.WriteFile(JDIStatusPath(root, jobName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := StaleRunningJDI(root, jobName); ok {
		t.Error("StaleRunningJDI with an old stopped:finished status: expected ok=false")
	}

	// No sidecar at all: nothing crashed.
	if _, ok := StaleRunningJDI(root, "no-such-job"); ok {
		t.Error("StaleRunningJDI with no sidecar file: expected ok=false")
	}
}

// A stopped:* status is a terminal state and must never be treated as
// stale, no matter how old it is — the job simply finished a while ago.
func TestReadJDIStatusOldStoppedStatusStillReported(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * jdiRunningStaleAfter).UTC().Format(time.RFC3339)
	body := `{"state":"stopped:finished","agent":"reviewer","updated":"` + old + `"}`
	if err := os.WriteFile(JDIStatusPath(root, jobName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadJDIStatus(root, jobName)
	if !ok {
		t.Fatal("ReadJDIStatus with an old stopped:finished status: expected ok=true")
	}
	if got.State != JDIStoppedFinished {
		t.Errorf("State = %q, want %q", got.State, JDIStoppedFinished)
	}
}

func TestWriteJDIStatusOverwritesPreviousState(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"

	if err := WriteJDIStatus(root, jobName, JDIRunning, "analyst"); err != nil {
		t.Fatal(err)
	}
	if err := WriteJDIStatus(root, jobName, JDIStoppedNeedsHuman, "developer"); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadJDIStatus(root, jobName)
	if !ok {
		t.Fatal("ReadJDIStatus: expected ok=true")
	}
	if got.State != JDIStoppedNeedsHuman || got.Agent != "developer" {
		t.Errorf("got %+v, want State=%q Agent=%q", got, JDIStoppedNeedsHuman, "developer")
	}
}

func TestReadJDIRunLogTailNoFile(t *testing.T) {
	root := t.TempDir()
	_, ok := ReadJDIRunLogTail(root, "no-such-job")
	if ok {
		t.Error("ReadJDIRunLogTail with no run.log at all: expected ok=false")
	}
}

func TestReadJDIRunLogTailReadsContent(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "=== 2026-08-09T00:00:00Z analyst (attempt 1) ===\nwrote tasks.md\n"
	if err := os.WriteFile(JDIRunLogPath(root, jobName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := ReadJDIRunLogTail(root, jobName)
	if !ok {
		t.Fatal("ReadJDIRunLogTail: expected ok=true")
	}
	if got != body {
		t.Errorf("ReadJDIRunLogTail = %q, want %q", got, body)
	}
}

func TestReadJDIRunLogTailEmptyFile(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(JDIRunLogPath(root, jobName), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := ReadJDIRunLogTail(root, jobName)
	if !ok {
		t.Fatal("ReadJDIRunLogTail on an empty file: expected ok=true")
	}
	if got != "" {
		t.Errorf("ReadJDIRunLogTail on an empty file = %q, want empty", got)
	}
}

func TestReadJDIRunLogTailTruncatesLargeFile(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := os.MkdirAll(JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	for i := 0; i < 20000; i++ {
		fmt.Fprintf(&b, "line %d filler filler filler filler filler\n", i)
	}
	b.WriteString("=== the very last line ===\n")
	full := b.String()
	if len(full) <= jdiRunLogTailBytes {
		t.Fatalf("test fixture too small to exercise truncation: %d bytes", len(full))
	}
	if err := os.WriteFile(JDIRunLogPath(root, jobName), []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := ReadJDIRunLogTail(root, jobName)
	if !ok {
		t.Fatal("ReadJDIRunLogTail: expected ok=true")
	}
	if len(got) >= len(full) {
		t.Errorf("ReadJDIRunLogTail did not truncate: got %d bytes, full file was %d bytes", len(got), len(full))
	}
	if !strings.Contains(got, "the very last line") {
		t.Errorf("ReadJDIRunLogTail should keep the tail of the file, missing the last line; got:\n%s", got[:200])
	}
	if !strings.HasPrefix(got, "… (log truncated") {
		t.Errorf("ReadJDIRunLogTail should note truncation at the top, got prefix: %q", got[:min(60, len(got))])
	}
}
