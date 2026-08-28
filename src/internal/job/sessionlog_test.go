package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- session.log live-stream helper (OpenSessionLog + SessionLog) ----------

// writeAndClose streams raw into a freshly opened session.log section and
// closes it, failing the test on either error.
func writeAndClose(t *testing.T, path, agent string, attempt int, raw []byte) {
	t.Helper()
	s, err := OpenSessionLog(path, agent, attempt)
	if err != nil {
		t.Fatalf("OpenSessionLog: %v", err)
	}
	if _, err := s.Write(raw); err != nil {
		t.Fatalf("SessionLog.Write: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("SessionLog.Close: %v", err)
	}
}

func TestOpenSessionLogCreatesOnFirstUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.log")
	writeAndClose(t, path, "analyst", 1, []byte("Wrote tasks.md.\n"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading session.log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "analyst (attempt 1)") {
		t.Errorf("session.log = %q, want an analyst (attempt 1) header", got)
	}
	if !strings.Contains(got, "Wrote tasks.md.") {
		t.Errorf("session.log = %q, want the raw output preserved", got)
	}
}

func TestOpenSessionLogAppendsAcrossSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.log")
	writeAndClose(t, path, "analyst", 1, []byte("first output\n"))
	writeAndClose(t, path, "developer", 2, []byte("second output\n"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if i, j := strings.Index(got, "analyst (attempt 1)"), strings.Index(got, "developer (attempt 2)"); i == -1 || j == -1 || i > j {
		t.Errorf("session.log sections out of order or missing, got:\n%s", got)
	}
	if i, j := strings.Index(got, "first output"), strings.Index(got, "second output"); i == -1 || j == -1 || i > j {
		t.Errorf("session.log raw outputs out of order or missing, got:\n%s", got)
	}
}

func TestOpenSessionLogHeaderCarriesAgentAttemptTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.log")
	writeAndClose(t, path, "reviewer", 3, []byte("ok\n"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The header is the first line: "=== <RFC3339 timestamp> <agent>
	// (attempt N) ===".
	line, _, _ := strings.Cut(string(data), "\n")
	if !strings.HasPrefix(line, "=== ") || !strings.HasSuffix(line, " reviewer (attempt 3) ===") {
		t.Fatalf("header line = %q, want %q shape", line, "=== <timestamp> reviewer (attempt 3) ===")
	}
	ts := strings.TrimSuffix(strings.TrimPrefix(line, "=== "), " reviewer (attempt 3) ===")
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("header timestamp %q is not RFC3339: %v", ts, err)
	}
}

func TestOpenSessionLogPreservesRawVerbatim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.log")
	raw := []byte("line one\nline two\nline three\n")
	writeAndClose(t, path, "analyst", 1, raw)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "line one\nline two\nline three\n") {
		t.Errorf("session.log = %q, want the raw bytes preserved verbatim", string(data))
	}
}

func TestOpenSessionLogSectionsSeparated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.log")
	// Two sections where the first's raw output deliberately lacks a
	// trailing newline: the blank-line separator plus the trailing-newline
	// guarantee must keep the second section's header from gluing to it.
	writeAndClose(t, path, "analyst", 1, []byte("no trailing newline"))
	writeAndClose(t, path, "developer", 1, []byte("second\n"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "no trailing newline\n\n=== ") {
		t.Errorf("sections not separated by a blank line, got:\n%q", got)
	}
}

func TestOpenSessionLogTrailingNewlineGuarantee(t *testing.T) {
	// Raw output without a trailing newline gets one appended on Close...
	path := filepath.Join(t.TempDir(), "session.log")
	writeAndClose(t, path, "analyst", 1, []byte("no newline"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "no newline\n") {
		t.Errorf("session.log = %q, want the raw output to end with a newline", string(data))
	}
	// ...and output that already ends with one is not double-newlined.
	path2 := filepath.Join(t.TempDir(), "session.log")
	writeAndClose(t, path2, "analyst", 1, []byte("has newline\n"))
	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(string(data2), "has newline\n\n") {
		t.Errorf("session.log = %q, want exactly one trailing newline", string(data2))
	}
}

func TestOpenSessionLogEmptyRawStillWritesHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.log")
	// An invocation that produces no output still opens + closes its
	// section: the header (with its own trailing newline) is the section.
	s, err := OpenSessionLog(path, "analyst", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.Contains(got, "analyst (attempt 1)") {
		t.Errorf("session.log = %q, want the header even for empty output", got)
	}
	if !strings.HasSuffix(string(data), "===\n") {
		t.Errorf("session.log = %q, want the header's own newline as the section end", string(data))
	}
}
