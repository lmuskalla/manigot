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

// TestRunJobUnknownArg pins the single-line unknown-flag error: the old
// hand-rolled loop printed exactly one "Unknown argument: <flag>" line and
// nothing else, and the flag package's own diagnostic must not leak ahead of
// it (fs.SetOutput(io.Discard)).
func TestRunJobUnknownArg(t *testing.T) {
	var out, errOut strings.Builder
	if code := runJob([]string{"title", "--bogus"}, &out, &errOut); code != 1 {
		t.Errorf("unknown-arg exit code = %d, want 1", code)
	}
	if want := "Unknown argument: --bogus\n"; errOut.String() != want {
		t.Errorf("stderr = %q, want exactly %q", errOut.String(), want)
	}
}

// TestRunJobMissingValue pins the single-line missing-value error the same
// way: "mg job T --type" must print exactly one "Unknown argument: --type"
// line, not the flag package's "flag needs an argument" diagnostic.
func TestRunJobMissingValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"type", []string{"title", "--type"}, "Unknown argument: --type\n"},
		{"base-branch", []string{"title", "--base-branch"}, "Unknown argument: --base-branch\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut strings.Builder
			if code := runJob(tc.args, &out, &errOut); code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if errOut.String() != tc.want {
				t.Errorf("stderr = %q, want exactly %q", errOut.String(), tc.want)
			}
		})
	}
}

// TestRunJobRejectsPositionals pins the script's hand-rolled-loop behavior:
// any positional after the title (flag.FlagSet leaves them in fs.Args()) is
// an unknown argument, not a silently-ignored extra.
func TestRunJobRejectsPositionals(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"unquoted-title-word", []string{"Add", "Gallery"}, "Unknown argument: Gallery"},
		{"after-type", []string{"Title", "--type", "fix", "stray"}, "Unknown argument: stray"},
		{"after-base-branch", []string{"Title", "--base-branch", "main", "stray"}, "Unknown argument: stray"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut strings.Builder
			if code := runJob(tc.args, &out, &errOut); code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if !strings.Contains(errOut.String(), tc.want) {
				t.Errorf("missing %q:\n%s", tc.want, errOut.String())
			}
		})
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

// mgOrphanWorktree builds a dead worktree dir under root's .manigot-worktrees
// sibling parent (the orphan shape job.DiscoverOrphans detects) and returns
// the orphan's dir path. The orphan has no branch and no worktree registration,
// mirroring a job scaffolded and then abandoned.
func mgOrphanWorktree(t *testing.T, root, name string) string {
	t.Helper()
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	wtPath := filepath.Join(wtParent, name)
	if err := os.MkdirAll(wtParent, 0o755); err != nil {
		t.Fatal(err)
	}
	mgGit(t, root, "worktree", "add", wtPath, "-b", "feature/"+name)
	data, err := os.ReadFile(filepath.Join(wtPath, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir:"))
	if err := os.RemoveAll(gitdir); err != nil {
		t.Fatal(err)
	}
	mgGit(t, root, "branch", "-D", "feature/"+name)
	return wtPath
}

func TestRunDeleteOrphan(t *testing.T) {
	dir := mgCheckout(t)
	orphanDir := mgOrphanWorktree(t, dir, "o3kk3n_jdi-is-broken")
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runDelete([]string{"o3kk3n_jdi-is-broken"}, strings.NewReader("y\n"), &out, &errOut); code != 0 {
		t.Fatalf("runDelete orphan exit code = %d, stderr: %s", code, errOut.String())
	}
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Errorf("orphan dir still exists after delete: %v", err)
	}
	if !strings.Contains(out.String(), "✓ Orphan removed: o3kk3n_jdi-is-broken") {
		t.Errorf("missing orphan-removed line:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "This cannot be undone.") {
		t.Errorf("missing 'This cannot be undone.':\n%s", out.String())
	}
}

func TestRunDeleteOrphanPrefix(t *testing.T) {
	dir := mgCheckout(t)
	orphanDir := mgOrphanWorktree(t, dir, "o3kk3n_jdi-is-broken")
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runDelete([]string{"o3kk3n"}, strings.NewReader("y\n"), &out, &errOut); code != 0 {
		t.Fatalf("runDelete orphan-prefix exit code = %d, stderr: %s", code, errOut.String())
	}
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Errorf("orphan dir still exists after prefix delete: %v", err)
	}
}

func TestRunDeleteOrphanDeclined(t *testing.T) {
	dir := mgCheckout(t)
	orphanDir := mgOrphanWorktree(t, dir, "o3kk3n_jdi-is-broken")
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runDelete([]string{"o3kk3n"}, strings.NewReader("n\n"), &out, &errOut); code != 0 {
		t.Fatalf("declined orphan delete exit code = %d, want 0 (script exits 0 on decline)", code)
	}
	if _, err := os.Stat(orphanDir); err != nil {
		t.Errorf("orphan dir removed despite a declined confirmation: %v", err)
	}
}
