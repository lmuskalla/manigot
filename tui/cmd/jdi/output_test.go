package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/job"
)

func TestLogInvocationPlainText(t *testing.T) {
	var buf bytes.Buffer
	logInvocation(&buf, "developer", 1, []byte("did the thing\n"))
	got := buf.String()
	if !strings.Contains(got, "developer (attempt 1)") {
		t.Errorf("logInvocation output missing header, got:\n%s", got)
	}
	if !strings.Contains(got, "did the thing") {
		t.Errorf("logInvocation output missing agent text, got:\n%s", got)
	}
}

func TestLogInvocationExtractsJSONResult(t *testing.T) {
	var buf bytes.Buffer
	raw := []byte(`{"type":"result","result":"All done, committed TASK-1."}`)
	logInvocation(&buf, "developer", 2, raw)
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
	logInvocation(&buf, "analyst", 1, nil)
	got := buf.String()
	if !strings.Contains(got, "(no output)") {
		t.Errorf("logInvocation on empty output = %q, want it to note there was no output", got)
	}
}

func TestOpenRunLogCreatesSidecarAndAppends(t *testing.T) {
	root := t.TempDir()
	const jobName = "aaaa01_test-job"

	f, err := openRunLog(root, jobName)
	if err != nil {
		t.Fatalf("openRunLog: %v", err)
	}
	if _, err := f.WriteString("first run\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// A second open must append, not truncate — a fresh mg-jdi run
	// continues the same job's transcript.
	f2, err := openRunLog(root, jobName)
	if err != nil {
		t.Fatalf("openRunLog (second): %v", err)
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
	if !strings.Contains(string(data), "docs/jobs/.jdi-status/") {
		t.Errorf(".git/info/exclude = %q, want it to contain docs/jobs/.jdi-status/", string(data))
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
	count := strings.Count(string(data), "docs/jobs/.jdi-status/")
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
	if !strings.Contains(got, "docs/jobs/.jdi-status/") {
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

	sidecarFile := filepath.Join(root, "docs", "jobs", ".jdi-status", "aaaa01_x", "status")
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
	if strings.Contains(staged, ".jdi-status") {
		t.Errorf("git add -A staged the sidecar despite ensureSidecarIgnored:\n%s", staged)
	}
	if !strings.Contains(staged, "docs/jobs/aaaa01_x/tasks.md") {
		t.Errorf("git add -A did not stage the real job file — test itself is broken:\n%s", staged)
	}
}
