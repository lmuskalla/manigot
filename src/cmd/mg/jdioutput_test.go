package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/orchestrate"
)

func TestLogStarted(t *testing.T) {
	var buf bytes.Buffer
	logStarted(&buf, "aaaa01_test-job", "claude-pro")
	got := buf.String()
	if !strings.Contains(got, "mg jdi started") {
		t.Errorf("logStarted output missing 'mg jdi started', got:\n%s", got)
	}
	if !strings.Contains(got, "aaaa01_test-job") {
		t.Errorf("logStarted output missing job name, got:\n%s", got)
	}
	if !strings.Contains(got, "claude-pro") {
		t.Errorf("logStarted output missing profile, got:\n%s", got)
	}
}

func TestLogAgentInvoked(t *testing.T) {
	var buf bytes.Buffer
	logAgentInvoked(&buf, "developer", 2)
	got := buf.String()
	if !strings.Contains(got, "developer invoked (attempt 2)") {
		t.Errorf("logAgentInvoked output missing header, got:\n%s", got)
	}
}

func TestLogJobFinishedIncludesReasonByDefault(t *testing.T) {
	var buf bytes.Buffer
	logJobFinished(&buf, orchestrate.StopFinished, "verdict.md's Overall verdict is APPROVED", true)
	got := buf.String()
	if !strings.Contains(got, "mg jdi finished") {
		t.Errorf("logJobFinished output missing 'mg jdi finished', got:\n%s", got)
	}
	if !strings.Contains(got, "stop-finished") {
		t.Errorf("logJobFinished output missing the outcome kind, got:\n%s", got)
	}
	if !strings.Contains(got, "verdict.md's Overall verdict is APPROVED") {
		t.Errorf("logJobFinished output missing reason, got:\n%s", got)
	}
}

func TestLogJobFinishedOmitsReasonWhenAlreadyLogged(t *testing.T) {
	var buf bytes.Buffer
	logJobFinished(&buf, orchestrate.StopNeedsHuman, "brief.md is not written yet", false)
	got := buf.String()
	if !strings.Contains(got, "mg jdi finished") {
		t.Errorf("logJobFinished output missing 'mg jdi finished', got:\n%s", got)
	}
	if strings.Contains(got, "brief.md is not written yet") {
		t.Errorf("logJobFinished output repeats reason despite includeReason=false, got:\n%s", got)
	}
}

func TestLogInvocationPlainText(t *testing.T) {
	var buf bytes.Buffer
	logInvocation(&buf, "developer", 1, []byte("did the thing\n"), "", "")
	got := buf.String()
	if !strings.Contains(got, "developer finished (attempt 1)") {
		t.Errorf("logInvocation output missing header, got:\n%s", got)
	}
	if !strings.Contains(got, "did the thing") {
		t.Errorf("logInvocation output missing agent text, got:\n%s", got)
	}
}

func TestLogInvocationExtractsJSONResult(t *testing.T) {
	var buf bytes.Buffer
	raw := []byte(`{"type":"result","result":"All done, committed TASK-1."}`)
	logInvocation(&buf, "developer", 2, raw, "", "")
	got := buf.String()
	if !strings.Contains(got, "All done, committed TASK-1.") {
		t.Errorf("logInvocation did not extract the JSON result field, got:\n%s", got)
	}
	if strings.Contains(got, `"type":"result"`) {
		t.Errorf("logInvocation leaked raw JSON into the log, got:\n%s", got)
	}
}

func TestLogInvocationEmptyOutput(t *testing.T) {
	var buf bytes.Buffer
	logInvocation(&buf, "analyst", 1, nil, "", "")
	got := buf.String()
	if !strings.Contains(got, "(no output)") {
		t.Errorf("logInvocation on empty output = %q, want it to note there was no output", got)
	}
}

// --- TASK-5: dedup already-committed file content -----------------------

func TestLogInvocationOmitsDuplicateOutput(t *testing.T) {
	var buf bytes.Buffer
	fileContent := "# Tasks\n\nTASK-1: do the thing\n"
	raw := []byte("Sure, here's what I wrote:\n\n" + fileContent + "\nAll done!")
	logInvocation(&buf, "analyst", 1, raw, "tasks.md", fileContent)
	got := buf.String()
	if !strings.Contains(got, "(output matches tasks.md, omitted)") {
		t.Errorf("logInvocation did not dedup matching output, got:\n%s", got)
	}
	if strings.Contains(got, "TASK-1: do the thing") {
		t.Errorf("logInvocation printed the full duplicated text instead of omitting it, got:\n%s", got)
	}
}

func TestLogInvocationKeepsDistinctOutput(t *testing.T) {
	var buf bytes.Buffer
	fileContent := "# Tasks\n\nTASK-1: do the thing\n"
	raw := []byte("I analyzed the brief and decided on a different approach entirely.")
	logInvocation(&buf, "analyst", 1, raw, "tasks.md", fileContent)
	got := buf.String()
	if strings.Contains(got, "omitted") {
		t.Errorf("logInvocation dedup'd distinct output, got:\n%s", got)
	}
	if !strings.Contains(got, "I analyzed the brief") {
		t.Errorf("logInvocation output missing agent text, got:\n%s", got)
	}
}

func TestLogInvocationSkipsDedupWhenTargetFileEmpty(t *testing.T) {
	var buf bytes.Buffer
	raw := []byte("some response text")
	logInvocation(&buf, "analyst", 1, raw, "", "")
	got := buf.String()
	if strings.Contains(got, "omitted") {
		t.Errorf("logInvocation dedup'd with no target file, got:\n%s", got)
	}
	if !strings.Contains(got, "some response text") {
		t.Errorf("logInvocation output missing agent text, got:\n%s", got)
	}
}

func TestIsDuplicateOutput(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		fileContent string
		want        bool
	}{
		{"exact substring", "prose\n# Tasks\nTASK-1\nmore prose", "# Tasks\nTASK-1", true},
		{"whitespace differs", "prose\n#  Tasks\n\nTASK-1  \nmore prose", "# Tasks\nTASK-1", true},
		{"not present", "totally different text", "# Tasks\nTASK-1", false},
		{"empty file content never matches", "anything at all", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isDuplicateOutput(c.text, c.fileContent); got != c.want {
				t.Errorf("isDuplicateOutput(%q, %q) = %v, want %v", c.text, c.fileContent, got, c.want)
			}
		})
	}
}

func TestOpenRunLogCreatesSidecarAndAppends(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"

	f, _, err := openRunLog(root, jobName)
	if err != nil {
		t.Fatalf("openRunLog: %v", err)
	}
	if _, err := f.WriteString("first run\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// A second open must append, not truncate — a fresh mg-jdi run
	// continues the same job's transcript.
	f2, hadContent, err := openRunLog(root, jobName)
	if err != nil {
		t.Fatalf("openRunLog (second): %v", err)
	}
	if !hadContent {
		t.Error("second open of a non-empty run.log reported hadContent=false, want true")
	}
	if _, err := f2.WriteString("second run\n"); err != nil {
		t.Fatal(err)
	}
	f2.Close()

	data, err := os.ReadFile(job.JDIRunLogPath(root, jobName))
	if err != nil {
		t.Fatalf("reading run.log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "first run") || !strings.Contains(got, "second run") {
		t.Errorf("run.log = %q, want both writes present (append, not truncate)", got)
	}
}

func TestOpenRunLogReportsEmptyFreshLog(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa02_test-job"

	f, hadContent, err := openRunLog(root, jobName)
	if err != nil {
		t.Fatalf("openRunLog: %v", err)
	}
	if hadContent {
		t.Error("first open of a fresh run.log reported hadContent=true, want false")
	}
	f.Close()
}

// TestRunLogSeparationAcrossRuns wires openRunLog and sectionWriter together
// exactly as main() does, across two runs of the same job: the first run
// ever written to the log starts it at the top (no leading blank line), and
// the second run's first header picks up the separating blank line.
func TestRunLogSeparationAcrossRuns(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa03_test-job"

	run := func() {
		f, hadContent, err := openRunLog(root, jobName)
		if err != nil {
			t.Fatalf("openRunLog: %v", err)
		}
		defer f.Close()
		w := &sectionWriter{w: f, wroteSection: hadContent}
		logStarted(w, jobName, "claude-pro")
		logJobFinished(w, orchestrate.StopFinished, "verdict APPROVED", true)
	}

	run()
	data, err := os.ReadFile(job.JDIRunLogPath(root, jobName))
	if err != nil {
		t.Fatalf("reading run.log: %v", err)
	}
	first := string(data)
	if strings.HasPrefix(first, "\n") {
		t.Errorf("first run ever must not open the log with a blank line, got:\n%q", first)
	}
	if n := sectionHeaderCount(first); n != 2 {
		t.Fatalf("first run wrote %d headers, want 2:\n%q", n, first)
	}
	if n := strings.Count(first, "\n\n\n==="); n != 1 {
		t.Errorf("first run should have exactly 1 separator (between its two headers), got %d:\n%q", n, first)
	}

	run()
	data, err = os.ReadFile(job.JDIRunLogPath(root, jobName))
	if err != nil {
		t.Fatalf("reading run.log: %v", err)
	}
	both := string(data)
	if n := sectionHeaderCount(both); n != 4 {
		t.Fatalf("after two runs got %d headers, want 4:\n%q", n, both)
	}
	if n := strings.Count(both, "\n\n\n==="); n != 3 {
		t.Errorf("second run should add 2 separators (1 between its own headers + 1 from the prior run's last header), got %d:\n%q", n, both)
	}
}

// --- sectionWriter -----------------------------------------------------------

// TestSectionWriterSeparatesHeadersButNotFirstRunEver covers the two
// sectionWriter behaviors together: within a single writer, the very first
// section header (a fresh log — the first run ever written to it) gets no
// leading blank line, while every subsequent header gets "\n\n", and
// non-header writes (an agent-text or reason body following its header)
// pass through untouched — sectionWriter never inserts blank lines inside a
// section. The invocation body's two blank lines of separation (brief: jdi
// logs new line) are logInvocation's own writes, asserted below.
func TestSectionWriterSeparatesHeadersButNotFirstRunEver(t *testing.T) {
	var buf bytes.Buffer
	w := &sectionWriter{w: &buf, wroteSection: false}

	logStarted(w, "aaaa01_test-job", "claude-pro")
	logAgentInvoked(w, "analyst", 1)
	logInvocation(w, "analyst", 1, []byte("wrote tasks.md"), "", "")
	logJobFinished(w, orchestrate.StopFinished, "verdict APPROVED", true)

	got := buf.String()
	if strings.HasPrefix(got, "\n") {
		t.Errorf("fresh log must start at the top with no leading blank line, got:\n%q", got)
	}
	// Every header line after the first must be preceded by a blank line —
	// the separator is "\n\n" on top of the preceding header's own trailing
	// newline, so between two headers the text reads "\n\n\n===", and that
	// separator must appear exactly once per header after the first.
	if n := sectionHeaderCount(got); n != 4 {
		t.Fatalf("got %d section headers, want 4:\n%q", n, got)
	}
	if n := strings.Count(got, "\n\n\n==="); n != 3 {
		t.Errorf("got %d blank-line separators, want 3 (one before every header but the first):\n%q", n, got)
	}
	// The invocation body is deliberately separated from its "finished"
	// header by two blank lines (brief: jdi logs new line): the header's own
	// trailing newline plus the two blank lines reads as
	// "finished (attempt 1) ===\n\n\nwrote tasks.md", so the agent's output
	// no longer looks glued to the finished statement.
	if !strings.Contains(got, "finished (attempt 1) ===\n\n\nwrote tasks.md") {
		t.Errorf("expected two blank lines between the finished header and its body, got:\n%q", got)
	}
	if strings.Contains(got, "finished (attempt 1) ===\n\n\n\nwrote tasks.md") {
		t.Errorf("more than two blank lines between the finished header and its body, got:\n%q", got)
	}
}

// TestSectionWriterJoinsPriorRunWithBlankLine covers the "second run" case:
// a run.log that already has content (wroteSection seeded true) must get the
// separating blank line before its very first header too.
func TestSectionWriterJoinsPriorRunWithBlankLine(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("=== 2026-01-01T00:00:00Z mg jdi started: job aaaa01, profile claude-pro ===\n")
	w := &sectionWriter{w: &buf, wroteSection: true}

	logStarted(w, "aaaa01_test-job", "claude-pro")
	logAgentInvoked(w, "reviewer", 1)

	got := buf.String()
	if n := sectionHeaderCount(got); n != 3 {
		t.Fatalf("got %d section headers, want 3:\n%q", n, got)
	}
	if n := strings.Count(got, "\n\n\n==="); n != 2 {
		t.Errorf("got %d blank-line separators, want 2 (before both of this run's headers — the prior run's own header was the first ever written, so it correctly has none):\n%q", n, got)
	}
}

// sectionHeaderCount counts the "=== ... ===" state-log header lines in s —
// lines that begin (at the very start, or right after a newline) with "===".
// Counting occurrences of the "===" literal itself would overcount, since
// each header line both opens and closes with it.
func sectionHeaderCount(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "===") {
			n++
		}
	}
	return n
}

// TestSectionWriterPassesThroughNonHeaders ensures a write that does not
// begin with the "===" literal — e.g. an invocation's agent text or a stop
// reason — is never prefixed, so blank lines only ever separate sections.
func TestSectionWriterPassesThroughNonHeaders(t *testing.T) {
	var buf bytes.Buffer
	w := &sectionWriter{w: &buf, wroteSection: true}

	// A reason line written straight through must not pick up a prefix.
	if _, err := w.Write([]byte("brief.md is not written yet\n")); err != nil {
		t.Fatal(err)
	}
	// A subsequent real header still gets separated from it.
	logImmediateStop(w, "brief.md is not written yet")

	got := buf.String()
	if !strings.HasPrefix(got, "brief.md is not written yet\n") {
		t.Errorf("non-header write was not passed through verbatim, got:\n%q", got)
	}
	if n := strings.Count(got, "\n\n\n==="); n != 1 {
		t.Errorf("got %d separators, want exactly 1 (only before the header):\n%q", n, got)
	}
}

// --- ensureSidecarIgnored ----------------------------------------------------
//
// Regression coverage for a bug a real end-to-end mg-jdi run (TASK-14) caught:
// without this, an agent's own `git add -A` (run inside the container against
// the *target project*, not manigot's own checkout) swept the status/run.log
// sidecar straight into a real job-branch commit.

func gitRunForOutputTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func initGitRepoForOutputTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRunForOutputTest(t, dir, "init", "-q")
	gitRunForOutputTest(t, dir, "config", "user.email", "t@example.com")
	gitRunForOutputTest(t, dir, "config", "user.name", "Test")
	gitRunForOutputTest(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunForOutputTest(t, dir, "add", "README")
	gitRunForOutputTest(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func TestEnsureSidecarIgnoredWritesExcludeFile(t *testing.T) {
	root := initGitRepoForOutputTest(t)

	if err := ensureSidecarIgnored(root); err != nil {
		t.Fatalf("ensureSidecarIgnored: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("reading .git/info/exclude: %v", err)
	}
	if !strings.Contains(string(data), ".manigot/jdi-status/") {
		t.Errorf(".git/info/exclude = %q, want it to contain .manigot/jdi-status/", string(data))
	}
}

func TestEnsureSidecarIgnoredIsIdempotent(t *testing.T) {
	root := initGitRepoForOutputTest(t)

	if err := ensureSidecarIgnored(root); err != nil {
		t.Fatal(err)
	}
	if err := ensureSidecarIgnored(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(string(data), ".manigot/jdi-status/")
	if count != 1 {
		t.Errorf("pattern appears %d times after two calls, want exactly 1 (idempotent)", count)
	}
}

func TestEnsureSidecarIgnoredPreservesExistingContent(t *testing.T) {
	root := initGitRepoForOutputTest(t)
	excludeDir := filepath.Join(root, ".git", "info")
	if err := os.MkdirAll(excludeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte("*.local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureSidecarIgnored(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(excludeDir, "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "*.local") {
		t.Errorf(".git/info/exclude lost its pre-existing content: %q", got)
	}
	if !strings.Contains(got, ".manigot/jdi-status/") {
		t.Errorf(".git/info/exclude missing the sidecar pattern: %q", got)
	}
}

// TestEnsureSidecarIgnoredActuallyWorksWithGit is the real regression test:
// after ensureSidecarIgnored, `git add -A` inside the project must not pick
// up the sidecar directory at all — the exact scenario the TASK-14 manual
// run caught.
func TestEnsureSidecarIgnoredActuallyWorksWithGit(t *testing.T) {
	root := initGitRepoForOutputTest(t)
	if err := ensureSidecarIgnored(root); err != nil {
		t.Fatal(err)
	}

	sidecarFile := filepath.Join(root, ".manigot", "jdi-status", "aaaa01_x", "status")
	if err := os.MkdirAll(filepath.Dir(sidecarFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecarFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A normal job file, tracked as usual, to prove add -A isn't just a
	// no-op — it should still pick this up.
	jobFile := filepath.Join(root, "docs", "jobs", "aaaa01_x", "tasks.md")
	if err := os.MkdirAll(filepath.Dir(jobFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobFile, []byte("# Tasks\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gitRunForOutputTest(t, root, "add", "-A")

	out, err := exec.Command("git", "-C", root, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatal(err)
	}
	staged := string(out)
	if strings.Contains(staged, "jdi-status") {
		t.Errorf("git add -A staged the sidecar despite ensureSidecarIgnored:\n%s", staged)
	}
	if !strings.Contains(staged, "docs/jobs/aaaa01_x/tasks.md") {
		t.Errorf("git add -A did not stage the real job file — test itself is broken:\n%s", staged)
	}
}

// --- session.log live-stream helper (openSessionLog + sessionLog) ----------

// writeAndClose streams raw into a freshly opened session.log section and
// closes it, failing the test on either error.
func writeAndClose(t *testing.T, path, agent string, attempt int, raw []byte) {
	t.Helper()
	s, err := openSessionLog(path, agent, attempt)
	if err != nil {
		t.Fatalf("openSessionLog: %v", err)
	}
	if _, err := s.Write(raw); err != nil {
		t.Fatalf("sessionLog.Write: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("sessionLog.Close: %v", err)
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
	s, err := openSessionLog(path, "analyst", 1)
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
