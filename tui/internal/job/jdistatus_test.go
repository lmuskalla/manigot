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
