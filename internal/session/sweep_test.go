package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGit runs git -C dir args, failing the test on any error — mirrors
// internal/git's own test helper (unexported there), duplicated here since
// the sweep tests exercise real git against a throwaway repo.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// initRepo creates a fresh git repo at a temp dir with a deterministic
// identity and gpg signing disabled, plus an initial commit.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// writeFile writes a file under dir, creating parent dirs as needed.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitAll stages every change under dir and commits it.
func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", msg)
}

// headSubject returns the most recent commit's subject in dir.
func headSubject(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// statusPorcelain returns `git status --porcelain` output in dir.
func statusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// hasCommits reports whether the repo at dir has at least one commit (a fresh
// git init before its first commit has an unborn HEAD).
func hasCommits(t *testing.T, dir string) bool {
	t.Helper()
	return exec.Command("git", "-C", dir, "rev-parse", "--verify", "HEAD").Run() == nil
}

// TestSweepJobWorktreeCommitsLeftovers verifies the happy path: a job
// worktree holding a modified tracked file, a new untracked file, and a
// deleted tracked file is swept into one [<id>] chore: commit all commit and
// left clean, with a short committed note on diag.
func TestSweepJobWorktreeCommitsLeftovers(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "tracked.txt", "v1\n")
	writeFile(t, dir, "gone.txt", "bye\n")
	commitAll(t, dir, "setup")

	writeFile(t, dir, "tracked.txt", "v2\n")                          // modified
	writeFile(t, dir, "new.txt", "fresh\n")                           // new, untracked
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil { // deleted
		t.Fatal(err)
	}

	var diag strings.Builder
	SweepJobWorktree(Root{Job: "ab12cd_x", ProjectRoot: dir, InvocationRoot: t.TempDir(), GitCommonDir: filepath.Join(dir, ".git")}, &diag)

	if got := statusPorcelain(t, dir); got != "" {
		t.Errorf("worktree not clean after sweep:\n%s", got)
	}
	if got := headSubject(t, dir); got != "[ab12cd] chore: commit all" {
		t.Errorf("commit subject = %q, want %q", got, "[ab12cd] chore: commit all")
	}
	if !strings.Contains(diag.String(), "mg: committed leftover changes") {
		t.Errorf("diag missing the committed note:\n%s", diag.String())
	}
}

// TestSweepJobWorktreeCleanTree verifies a clean worktree produces no commit
// and no error — ErrNothingToCommit is swallowed, diag stays empty.
func TestSweepJobWorktreeCleanTree(t *testing.T) {
	dir := initRepo(t)
	var diag strings.Builder
	SweepJobWorktree(Root{Job: "ab12cd_x", ProjectRoot: dir, InvocationRoot: t.TempDir(), GitCommonDir: filepath.Join(dir, ".git")}, &diag)
	if got := headSubject(t, dir); got != "init" {
		t.Errorf("clean tree must not produce a commit (head = %q)", got)
	}
	if diag.Len() != 0 {
		t.Errorf("diag = %q, want empty for a clean tree", diag.String())
	}
}

// TestSweepJobWorktreeNonJobRoot verifies a plain session (root.Job == "") is
// a no-op: the user's own uncommitted work is never swept.
func TestSweepJobWorktreeNonJobRoot(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "user-work.txt", "mine\n")
	var diag strings.Builder
	SweepJobWorktree(Root{Job: "", ProjectRoot: dir}, &diag)
	if got := headSubject(t, dir); got != "init" {
		t.Errorf("non-job root must not be swept (head = %q)", got)
	}
	if diag.Len() != 0 {
		t.Errorf("diag = %q, want empty for a non-job root", diag.String())
	}
}

// TestSweepJobWorktreeNonRepo verifies a CommitAll failure of the ErrNotARepo
// kind is swallowed even when the worktree gate passes (a job worktree whose
// gitdir vanished mid-session) — diag stays empty, no abort.
func TestSweepJobWorktreeNonRepo(t *testing.T) {
	dir := t.TempDir()
	var diag strings.Builder
	// ProjectRoot != InvocationRoot (the worktree gate passes) but ProjectRoot
	// is not a repo — CommitAll returns ErrNotARepo, which is swallowed.
	SweepJobWorktree(Root{Job: "ab12cd_x", ProjectRoot: dir, InvocationRoot: t.TempDir(), GitCommonDir: "/nonexistent-gitdir"}, &diag)
	if diag.Len() != 0 {
		t.Errorf("diag = %q, want empty for a non-repo root", diag.String())
	}
}

// TestSweepJobWorktreeFlatScanFallbackNoOp pins the TASK-3 blocker fix: the
// --job flat-scan fallback — a git repo with no local branches (a fresh git
// init before its first commit, or a detached HEAD), where the job's files
// live directly in the MAIN project root and ProjectRoot never left
// InvocationRoot — must never be swept. Sweeping there would run git add -A
// over the user's own uncommitted work — .env included (not covered by the
// mount-target exclusions) — and create the repo's first commit. The
// leftovers stay untracked and the repo keeps its unborn HEAD.
func TestSweepJobWorktreeFlatScanFallbackNoOp(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	// The user's own uncommitted work, .env included — the exact contents the
	// flat-scan fallback must never commit.
	writeFile(t, dir, "work.txt", "mine\n")
	writeFile(t, dir, ".env", "SECRET=token\n")

	var diag strings.Builder
	// Root.go's flat-scan branch never reassigns ProjectRoot away from the
	// InvocationRoot it started as (root.go:82-83, 106-119) — GitCommonDir
	// stays "" too, matching the real resolution.
	SweepJobWorktree(Root{Job: "ab12cd_x", ProjectRoot: dir, InvocationRoot: dir}, &diag)

	if hasCommits(t, dir) {
		t.Error("flat-scan fallback must not create the repo's first commit")
	}
	status := statusPorcelain(t, dir)
	if !strings.Contains(status, "work.txt") || !strings.Contains(status, ".env") {
		t.Errorf("leftovers must remain untracked after a flat-scan no-op, status:\n%s", status)
	}
	if diag.Len() != 0 {
		t.Errorf("diag = %q, want empty for a flat-scan fallback", diag.String())
	}
}

// TestSweepJobWorktreePreWorktreeJobNoOp pins the second TASK-3 blocker shape
// the flat-scan fix alone did not close: a pre-worktree job, whose branch is
// checked out in the main worktree itself (an explicitly supported
// transitional state — job.Discover keeps listing it, see
// internal/job/discover.go:76-82). git.WorktreeForBranch resolves such a
// branch to the main worktree, so root.go:137-151 sets ProjectRoot to the
// SAME path as InvocationRoot, with GitCommonDir non-empty (the main repo's
// own .git — a real, non-empty value, unlike the flat-scan fallback). The
// GitCommonDir-only gate this job's first fix used would have swept the main
// worktree here; the ProjectRoot == InvocationRoot gate must catch it too.
func TestSweepJobWorktreePreWorktreeJobNoOp(t *testing.T) {
	dir := initRepo(t)
	// The user's own uncommitted work, .env included.
	writeFile(t, dir, "work.txt", "mine\n")
	writeFile(t, dir, ".env", "SECRET=token\n")

	var diag strings.Builder
	SweepJobWorktree(Root{
		Job:            "ab12cd_x",
		ProjectRoot:    dir,
		InvocationRoot: dir,
		GitCommonDir:   filepath.Join(dir, ".git"), // non-empty, like the real main-worktree resolution
	}, &diag)

	if got := headSubject(t, dir); got != "init" {
		t.Errorf("pre-worktree job (main worktree) must not be swept (head = %q)", got)
	}
	status := statusPorcelain(t, dir)
	if !strings.Contains(status, "work.txt") || !strings.Contains(status, ".env") {
		t.Errorf("leftovers must remain untracked after a pre-worktree-job no-op, status:\n%s", status)
	}
	if diag.Len() != 0 {
		t.Errorf("diag = %q, want empty for a pre-worktree job", diag.String())
	}
}

// TestJobIDFromName verifies the id derivation: everything before the first
// underscore (slugs may contain underscores); a name without an underscore —
// or with an empty id part — falls back to the whole name.
func TestJobIDFromName(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"ab12cd_x", "ab12cd"},
		{"precisely_git-strictness", "precisely"},
		{"no-underscore", "no-underscore"},
		{"_leading-underscore", "_leading-underscore"}, // empty id part → whole name
	} {
		if got := jobIDFromName(tc.name); got != tc.want {
			t.Errorf("jobIDFromName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestSweepJobWorktreeFailureWarns verifies a real sweep failure (here: a
// stale index.lock that makes git add fail) surfaces as a warning on diag,
// never an abort — SweepJobWorktree has no error return.
func TestSweepJobWorktreeFailureWarns(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "tracked.txt", "v1\n")
	commitAll(t, dir, "setup")
	writeFile(t, dir, "tracked.txt", "v2\n")
	// A stale index.lock makes git add -A fail deterministically.
	if err := os.WriteFile(filepath.Join(dir, ".git", "index.lock"), []byte("lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	var diag strings.Builder
	SweepJobWorktree(Root{Job: "ab12cd_x", ProjectRoot: dir, InvocationRoot: t.TempDir(), GitCommonDir: filepath.Join(dir, ".git")}, &diag)
	if !strings.Contains(diag.String(), "mg: warning: could not commit leftover changes") {
		t.Errorf("diag = %q, want a 'could not commit leftover changes' warning", diag.String())
	}
}
