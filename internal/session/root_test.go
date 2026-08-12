package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// projectCheckout builds a scratch git repo at dir with a docs/ dir (when
// withDocs) and an initial commit, returning its path.
func projectCheckout(t *testing.T, dir string, withDocs bool) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file at the root so the initial commit always has something to stage.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withDocs {
		if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs"), 0o755); err != nil {
			t.Fatal(err)
		}
		// Empty dirs aren't tracked by git; give the commit something to stage.
		if err := os.WriteFile(filepath.Join(dir, "docs", "jobs", ".gitkeep"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "config", "user.name", "Test")
	gitCmd(t, dir, "config", "user.email", "test@x.io")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestResolveRootWithDocs(t *testing.T) {
	dir := projectCheckout(t, t.TempDir(), true)
	t.Chdir(dir)
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if !r.DocsInitialized || r.ProjectRoot != filepath.Clean(dir) || r.DocsDir != filepath.Join(filepath.Clean(dir), "docs") {
		t.Errorf("ResolveRoot = %+v", r)
	}
	if r.Job != "" || r.GitCommonDir != "" {
		t.Errorf("plain session should have no job/worktree: %+v", r)
	}
}

func TestResolveRootNoDocsFallsBackToGitRoot(t *testing.T) {
	dir := projectCheckout(t, t.TempDir(), false)
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if r.DocsInitialized {
		t.Error("no docs/ should mean not initialized")
	}
	if r.ProjectRoot != filepath.Clean(dir) {
		t.Errorf("fallback root = %q, want %q", r.ProjectRoot, dir)
	}
}

func TestResolveRootNoDocsNoGitFallsBackToPWD(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if r.DocsInitialized || r.ProjectRoot != filepath.Clean(dir) {
		t.Errorf("ResolveRoot = %+v", r)
	}
}

func TestResolveJobRequiresInitializedProject(t *testing.T) {
	dir := projectCheckout(t, t.TempDir(), false)
	t.Chdir(dir)
	_, err := ResolveRoot(Options{Job: "abc123_x"})
	if err == nil || !strings.Contains(err.Error(), "Error: --job requires an initialized project (no docs/ found).") {
		t.Errorf("uninitialized --job error = %v", err)
	}
}

func TestResolveJobNoBranchesFlatScan(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs", "abc123_hello"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	// Not a git repo at all → no branches → flat scan.
	r, err := ResolveRoot(Options{Job: "abc123"})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if r.Job != "abc123_hello" {
		t.Errorf("flat-scan job = %q, want abc123_hello", r.Job)
	}
	if r.ProjectRoot != filepath.Clean(dir) {
		t.Errorf("flat-scan must not reassign the root: %q", r.ProjectRoot)
	}
}

func TestResolveJobFlatScanNotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	_, err := ResolveRoot(Options{Job: "nope"})
	if err == nil || !strings.Contains(err.Error(), "Error: job 'nope' not found under docs/jobs/") {
		t.Errorf("flat-scan not-found error = %v", err)
	}
}

func TestResolveJobFlatScanExcludesArchive(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs", "abc123_hello"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs", "archive", "abc123_done"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	r, err := ResolveRoot(Options{Job: "abc123"})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if r.Job != "abc123_hello" {
		t.Errorf("archive job must be excluded, got %q", r.Job)
	}
}

// worktreeProject sets up a repo with a job branch + worktree (the shape
// new-job.sh creates) and returns (root, jobName, worktreePath).
func worktreeProject(t *testing.T) (root, jobName, wtPath string) {
	t.Helper()
	root = projectCheckout(t, t.TempDir(), true)
	jobName = "abc123_hello"
	branch := "feature/" + jobName
	gitCmd(t, root, "checkout", "-q", "-b", branch)
	gitCmd(t, root, "checkout", "-q", "main")
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	if err := os.MkdirAll(wtParent, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath = filepath.Join(wtParent, jobName)
	gitCmd(t, root, "worktree", "add", "-q", wtPath, branch)
	if err := os.MkdirAll(filepath.Join(wtPath, "docs", "jobs", jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, jobName, wtPath
}

func TestResolveJobWorktreeExactMatch(t *testing.T) {
	root, jobName, wtPath := worktreeProject(t)
	t.Chdir(root)
	r, err := ResolveRoot(Options{Job: jobName})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if r.Job != jobName {
		t.Errorf("job = %q, want %q", r.Job, jobName)
	}
	if r.ProjectRoot != filepath.Clean(wtPath) {
		t.Errorf("root reassigned to %q, want %q", r.ProjectRoot, wtPath)
	}
	if r.GitCommonDir == "" {
		t.Error("GitCommonDir should be resolved for a worktree")
	}
}

func TestResolveJobWorktreePrefixMatch(t *testing.T) {
	root, jobName, _ := worktreeProject(t)
	t.Chdir(root)
	r, err := ResolveRoot(Options{Job: "abc123"})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if r.Job != jobName {
		t.Errorf("prefix job = %q, want %q", r.Job, jobName)
	}
}

func TestResolveJobAmbiguous(t *testing.T) {
	root, _, _ := worktreeProject(t)
	// Add a second branch sharing the prefix.
	gitCmd(t, root, "checkout", "-q", "-b", "feature/abc123_other")
	gitCmd(t, root, "checkout", "-q", "main")
	t.Chdir(root)
	_, err := ResolveRoot(Options{Job: "abc123"})
	if err == nil || !strings.Contains(err.Error(), "Error: job 'abc123' is ambiguous — matches branches:") {
		t.Errorf("ambiguity error = %v", err)
	}
}

func TestResolveJobBranchNotFound(t *testing.T) {
	root := projectCheckout(t, t.TempDir(), true)
	t.Chdir(root)
	_, err := ResolveRoot(Options{Job: "zzz999_nope"})
	if err == nil || !strings.Contains(err.Error(), "Error: job 'zzz999_nope' not found among local branches.") {
		t.Errorf("branch-not-found error = %v", err)
	}
}

func TestResolveJobBranchWithoutWorktreeHardError(t *testing.T) {
	root := projectCheckout(t, t.TempDir(), true)
	jobName := "abc123_noworktree"
	gitCmd(t, root, "checkout", "-q", "-b", "feature/"+jobName)
	gitCmd(t, root, "checkout", "-q", "main")
	t.Chdir(root)
	_, err := ResolveRoot(Options{Job: jobName})
	if err == nil {
		t.Fatal("branch-without-worktree must be a hard error")
	}
	for _, want := range []string{
		"Error: branch 'feature/" + jobName + "' has no git worktree.",
		"Refusing to fall back to mounting " + filepath.Clean(root) + " instead: that would show the wrong job's content.",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("hard-error wording missing %q:\n%v", want, err)
		}
	}
}

func TestResolveJobWorktreeMissingJobDir(t *testing.T) {
	root, jobName, wtPath := worktreeProject(t)
	// Remove the job dir from the worktree → inconsistent state.
	if err := os.RemoveAll(filepath.Join(wtPath, "docs", "jobs", jobName)); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	_, err := ResolveRoot(Options{Job: jobName})
	if err == nil || !strings.Contains(err.Error(), "inconsistent worktree state") {
		t.Errorf("missing job dir error = %v", err)
	}
}
