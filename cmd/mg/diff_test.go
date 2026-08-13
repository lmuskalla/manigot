package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for `mg diff` (TASK-4 of the mg diff job). Same real-git scratch-repo
// style as the lifecycle tests: mgCheckout builds a repo, runJob creates a
// real job (branch + worktree + scaffold commit), and runDiff is exercised
// against it.

func TestRunDiffUsage(t *testing.T) {
	var out, errOut strings.Builder
	if code := runDiff(nil, &out, &errOut); code != 1 {
		t.Errorf("no-args exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Usage: mg-diff") {
		t.Errorf("missing usage:\n%s", errOut.String())
	}
}

// TestRunDiffUnknownArg pins the single-line unknown-argument error: --help
// (no help case, mirroring mg job), unknown flags, and stray positionals all
// print exactly one "Unknown argument: <token>" line and nothing else.
func TestRunDiffUnknownArg(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"help", []string{"--help"}, "Unknown argument: --help\n"},
		{"unknown-flag", []string{"--bogus"}, "Unknown argument: --bogus\n"},
		{"extra-positional", []string{"somejob", "stray"}, "Unknown argument: stray\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut strings.Builder
			if code := runDiff(tc.args, &out, &errOut); code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if errOut.String() != tc.want {
				t.Errorf("stderr = %q, want exactly %q", errOut.String(), tc.want)
			}
		})
	}
}

func TestRunDiffUnknownJob(t *testing.T) {
	dir := mgCheckout(t)
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runDiff([]string{"nosuch"}, &out, &errOut); code != 1 {
		t.Errorf("unknown-job exit code = %d, want 1", code)
	}
	// done/delete's exact not-found wording, with the active-branch listing.
	if !strings.Contains(errOut.String(), "job 'nosuch' not found among local branches.") {
		t.Errorf("missing not-found wording:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Active job branches:") {
		t.Errorf("missing active-branches header:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "main") {
		t.Errorf("missing branch listing:\n%s", errOut.String())
	}
}

func TestRunDiffAmbiguous(t *testing.T) {
	dir := mgCheckout(t)
	t.Chdir(dir)
	mgGit(t, dir, "branch", "feature/abc123_x")
	mgGit(t, dir, "branch", "feature/abc123_y")

	var out, errOut strings.Builder
	if code := runDiff([]string{"abc12"}, &out, &errOut); code != 1 {
		t.Errorf("ambiguous exit code = %d, want 1", code)
	}
	if want := "job 'abc12' is ambiguous — matches branches: feature/abc123_x feature/abc123_y"; !strings.Contains(errOut.String(), want) {
		t.Errorf("missing ambiguity wording (want %q):\n%s", want, errOut.String())
	}
}

// TestRunDiffOutput exercises the successful output paths on a real job:
// the default quick eyeball (log + stat), --name-only, and --full, with flags
// accepted both before and after the job id.
func TestRunDiffOutput(t *testing.T) {
	dir := mgCheckout(t)
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runJob([]string{"Add Gallery"}, &out, &errOut); code != 0 {
		t.Fatalf("runJob: %d %s", code, errOut.String())
	}
	branch := mgJobBranch(t, dir)
	jobName := strings.TrimPrefix(branch, "feature/")

	// A second commit on the branch, so the log shows two commits.
	wt := mgWorktreePath(t, dir, branch)
	if err := os.WriteFile(filepath.Join(wt, "docs", "jobs", jobName, "implementation.md"), []byte("impl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mgGit(t, wt, "add", "-A")
	mgGit(t, wt, "commit", "-q", "-m", "[job] TASK-1: add the gallery")

	briefStat := "docs/jobs/" + jobName + "/brief.md"

	// Default: the commit log, then the diff --stat.
	var out1, err1 strings.Builder
	if code := runDiff([]string{jobName}, &out1, &err1); code != 0 {
		t.Fatalf("runDiff exit code = %d, stderr: %s", code, err1.String())
	}
	if !strings.Contains(out1.String(), "Scaffold job") {
		t.Errorf("default output missing scaffold commit:\n%s", out1.String())
	}
	if !strings.Contains(out1.String(), "[job] TASK-1: add the gallery") {
		t.Errorf("default output missing implementation commit:\n%s", out1.String())
	}
	if !strings.Contains(out1.String(), briefStat+" ") {
		t.Errorf("default output missing stat entry for %s:\n%s", briefStat, out1.String())
	}

	// --name-only: filenames, no stat line counts. Flag after the id here.
	var out2, err2 strings.Builder
	if code := runDiff([]string{jobName, "--name-only"}, &out2, &err2); code != 0 {
		t.Fatalf("runDiff --name-only exit code = %d, stderr: %s", code, err2.String())
	}
	if !strings.Contains(out2.String(), briefStat) {
		t.Errorf("name-only output missing %s:\n%s", briefStat, out2.String())
	}
	if !strings.Contains(out2.String(), "docs/jobs/"+jobName+"/implementation.md") {
		t.Errorf("name-only output missing implementation.md:\n%s", out2.String())
	}
	// The stat form pads the path column before a "|" (diff --stat) — its
	// absence is what tells name-only apart from the default output.
	if strings.Contains(out2.String(), " | ") {
		t.Errorf("name-only output contains diff --stat line counts:\n%s", out2.String())
	}

	// --full: the complete patch. Flag before the id here, to pin that
	// splitFlags accepts either position.
	var out3, err3 strings.Builder
	if code := runDiff([]string{"--full", jobName}, &out3, &err3); code != 0 {
		t.Fatalf("runDiff --full exit code = %d, stderr: %s", code, err3.String())
	}
	if !strings.Contains(out3.String(), "diff --git") {
		t.Errorf("full output missing patch header:\n%s", out3.String())
	}
	if !strings.Contains(out3.String(), "+impl") {
		t.Errorf("full output missing the added content:\n%s", out3.String())
	}
}

// TestRunDiffTigMissing pins the --tig flag parse (in both positions) and the
// clear error when tig isn't on the host — tigLookPath is stubbed so the test
// never needs a real tig, mirroring the mg host CLI-missing test pattern.
func TestRunDiffTigMissing(t *testing.T) {
	dir := mgCheckout(t)
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runJob([]string{"Add Gallery"}, &out, &errOut); code != 0 {
		t.Fatalf("runJob: %d %s", code, errOut.String())
	}
	jobName := strings.TrimPrefix(mgJobBranch(t, dir), "feature/")

	old := tigLookPath
	tigLookPath = func(name string) (string, error) { return "", errors.New("exec: not found") }
	t.Cleanup(func() { tigLookPath = old })

	for _, args := range [][]string{
		{jobName, "--tig"},
		{"--tig", jobName},
	} {
		var outT, errT strings.Builder
		if code := runDiff(args, &outT, &errT); code != 1 {
			t.Errorf("runDiff(%v) exit code = %d, want 1", args, code)
		}
		if !strings.Contains(errT.String(), "tig is not installed on the host") {
			t.Errorf("runDiff(%v) missing tig-missing error:\n%s", args, errT.String())
		}
	}
}

// TestRunDiffNotARepo pins the non-repo degrade: a project (docs/ present)
// that isn't under git reports the package's ErrNotARepo wording.
func TestRunDiffNotARepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runDiff([]string{"somejob"}, &out, &errOut); code != 1 {
		t.Errorf("non-repo exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "not a git repository") {
		t.Errorf("missing not-a-repo error:\n%s", errOut.String())
	}
}

// mgJobBranch returns the single feature/* branch in the scratch repo.
func mgJobBranch(t *testing.T, dir string) string {
	t.Helper()
	branches := mgGit(t, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/feature/")
	for _, b := range strings.Split(branches, "\n") {
		if strings.HasPrefix(b, "feature/") {
			return b
		}
	}
	t.Fatalf("no feature/* branch created:\n%s", branches)
	return ""
}

// mgWorktreePath returns the worktree path for branch from `git worktree
// list --porcelain` — the scratch-repo twin of internal/git's
// WorktreeForBranch.
func mgWorktreePath(t *testing.T, dir, branch string) string {
	t.Helper()
	out := mgGit(t, dir, "worktree", "list", "--porcelain")
	var current string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			if strings.TrimPrefix(line, "branch ") == "refs/heads/"+branch {
				return current
			}
		}
	}
	t.Fatalf("no worktree for branch %s:\n%s", branch, out)
	return ""
}
