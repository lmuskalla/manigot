package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mgGit runs git -C dir args, failing the test on error, and returns trimmed
// stdout.
func mgGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// mgCheckout builds a scratch git repo with a docs/ dir and an initial commit.
func mgCheckout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mgGit(t, dir, "init", "-q", "-b", "main")
	mgGit(t, dir, "config", "user.name", "Test")
	mgGit(t, dir, "config", "user.email", "test@x.io")
	if err := os.WriteFile(filepath.Join(dir, "docs", "jobs", ".gitkeep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	mgGit(t, dir, "add", "-A")
	mgGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func TestRunJobUsage(t *testing.T) {
	var out, errOut strings.Builder
	if code := runJob(nil, &out, &errOut); code != 1 {
		t.Errorf("no-args exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Usage: mg-job") {
		t.Errorf("missing usage:\n%s", errOut.String())
	}
}

func TestRunJobUnknownArg(t *testing.T) {
	var out, errOut strings.Builder
	if code := runJob([]string{"title", "--bogus"}, &out, &errOut); code != 1 {
		t.Errorf("unknown-arg exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Unknown argument: --bogus") {
		t.Errorf("missing unknown-arg error:\n%s", errOut.String())
	}
}

func TestRunJobCreatesJob(t *testing.T) {
	dir := mgCheckout(t)
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runJob([]string{"Add Gallery", "--type", "fix"}, &out, &errOut); code != 0 {
		t.Fatalf("runJob exit code = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "✓ Job created:") {
		t.Errorf("missing creation summary:\n%s", out.String())
	}
	// The job branch exists and the job directory lives in its own worktree.
	branches := mgGit(t, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	var jobBranch string
	for _, b := range strings.Split(branches, "\n") {
		if strings.HasPrefix(b, "fix/") && strings.HasSuffix(b, "_add-gallery") {
			jobBranch = b
			break
		}
	}
	if jobBranch == "" {
		t.Fatalf("no fix/*_add-gallery branch created:\n%s", branches)
	}
	wt := mgGit(t, dir, "worktree", "list", "--porcelain")
	if !strings.Contains(wt, jobBranch) {
		t.Errorf("no worktree for %s:\n%s", jobBranch, wt)
	}
}

func TestRunDoneUsageAndCancel(t *testing.T) {
	dir := mgCheckout(t)
	t.Chdir(dir)

	// No args → usage error.
	var out, errOut strings.Builder
	if code := runDone(nil, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Errorf("no-args exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Usage: mg-done") {
		t.Errorf("missing usage:\n%s", errOut.String())
	}

	// Create a real job, then decline its "Proceed?" confirmation — the
	// script's `exit 0`, not an error.
	var jobOut strings.Builder
	if code := runJob([]string{"Cancel Job"}, &jobOut, &errOut); code != 0 {
		t.Fatalf("runJob: %d %s", code, errOut.String())
	}
	jobName := strings.TrimPrefix(mgGit(t, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/feature/"), "feature/")
	var out2, errOut2 strings.Builder
	if code := runDone([]string{jobName}, strings.NewReader("n\n"), &out2, &errOut2); code != 0 {
		t.Errorf("declined exit code = %d, want 0 (script exits 0 on decline)", code)
	}
}

func TestRunDeleteUsage(t *testing.T) {
	var out, errOut strings.Builder
	if code := runDelete(nil, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Errorf("no-args exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Usage: mg-delete") {
		t.Errorf("missing usage:\n%s", errOut.String())
	}
}
