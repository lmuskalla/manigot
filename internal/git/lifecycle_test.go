package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the job-lifecycle operations ported from scripts/new-job.sh,
// scripts/finish-job.sh and scripts/delete-job.sh (TASK-15 of the mg
// consolidation job). Every test builds a throwaway repo via the package's
// existing helpers and exercises real git, same as the rest of this package.

func TestWorktreeAdd(t *testing.T) {
	dir, def := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "job-x")

	if err := WorktreeAdd(dir, wtPath, "feature/abc123_x", def); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	// The new branch exists and points at the base branch's commit.
	if ok, err := RefExists(dir, "feature/abc123_x"); err != nil || !ok {
		t.Fatalf("branch after WorktreeAdd: ok=%v err=%v", ok, err)
	}
	// The worktree has the branch checked out and resolves back to it.
	got, ok, err := WorktreeForBranch(dir, "feature/abc123_x")
	if err != nil || !ok {
		t.Fatalf("WorktreeForBranch after WorktreeAdd: path=%q ok=%v err=%v", got, ok, err)
	}
	gotPath, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	wantPath, err := filepath.EvalSymlinks(wtPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != wantPath {
		t.Errorf("worktree path = %q, want %q", gotPath, wantPath)
	}
}

func TestWorktreeAddNotARepo(t *testing.T) {
	err := WorktreeAdd(t.TempDir(), t.TempDir(), "feature/x", "main")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("WorktreeAdd on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestWorktreeRemove(t *testing.T) {
	dir, def := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "job-x")
	runGit(t, dir, "worktree", "add", wtPath, "-b", "feature/abc123_x", def)

	if err := WorktreeRemove(dir, wtPath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after remove: %v", err)
	}
	if _, ok, err := WorktreeForBranch(dir, "feature/abc123_x"); err != nil || ok {
		t.Errorf("worktree still registered after remove: ok=%v err=%v", ok, err)
	}
}

func TestWorktreeRemoveForce(t *testing.T) {
	dir, def := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "job-x")
	runGit(t, dir, "worktree", "add", wtPath, "-b", "feature/abc123_x", def)

	// Dirty the worktree — a plain remove would refuse.
	writeFile(t, wtPath, "dirty.txt", "uncommitted\n")
	if err := WorktreeRemove(dir, wtPath); err == nil {
		t.Fatal("WorktreeRemove on a dirty worktree: expected an error, got nil")
	}
	if err := WorktreeRemoveForce(dir, wtPath); err != nil {
		t.Fatalf("WorktreeRemoveForce on a dirty worktree: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after force remove: %v", err)
	}
}

func TestWorktreePrune(t *testing.T) {
	dir, def := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "job-x")
	runGit(t, dir, "worktree", "add", wtPath, "-b", "feature/abc123_x", def)
	// Remove the worktree directory behind git's back, then prune — a
	// best-effort call that must not error either way.
	os.RemoveAll(wtPath)
	if err := WorktreePrune(dir); err != nil {
		t.Fatalf("WorktreePrune: unexpected error %v", err)
	}
}

func TestBranchDelete(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/abc123_x")

	if err := BranchDelete(dir, "feature/abc123_x"); err != nil {
		t.Fatalf("BranchDelete: %v", err)
	}
	if ok, _ := RefExists(dir, "feature/abc123_x"); ok {
		t.Error("branch still exists after BranchDelete")
	}
	// The base branch survives.
	if ok, _ := RefExists(dir, def); !ok {
		t.Errorf("base branch %q was deleted", def)
	}
}

func TestSquashMergeAndCommitStaged(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "checkout", "-q", "-b", "feature/abc123_x")
	writeFile(t, dir, "docs/jobs/abc123_x/brief.md", "brief\n")
	writeFile(t, dir, "docs/jobs/abc123_x/tasks.md", "tasks\n")
	commitAll(t, dir, "Scaffold job abc123_x")
	writeFile(t, dir, "docs/jobs/abc123_x/implementation.md", "impl\n")
	commitAll(t, dir, "[abc123] implementation: add summary")

	runGit(t, dir, "checkout", "-q", def)
	if err := SquashMerge(dir, "feature/abc123_x"); err != nil {
		t.Fatalf("SquashMerge: %v", err)
	}
	if err := CommitStaged(dir, "Add gallery\n\nJob: abc123_x"); err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}

	// The job's files are now on the base branch as a single commit.
	if _, err := os.Stat(filepath.Join(dir, "docs/jobs/abc123_x/implementation.md")); err != nil {
		t.Errorf("job file missing on base branch after squash merge: %v", err)
	}
	out, err := execOutput(dir, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Add gallery") {
		t.Errorf("squash commit subject = %q, want %q prefix", out, "Add gallery")
	}
}

func TestWorkingTreeDirty(t *testing.T) {
	dir, _ := initRepo(t)

	// Clean.
	dirty, err := WorkingTreeDirty(dir)
	if err != nil {
		t.Fatalf("WorkingTreeDirty on clean tree: %v", err)
	}
	if dirty {
		t.Error("WorkingTreeDirty on a clean tree = true, want false")
	}

	// Unstaged modification.
	writeFile(t, dir, "README", "modified\n")
	dirty, err = WorkingTreeDirty(dir)
	if err != nil {
		t.Fatalf("WorkingTreeDirty with unstaged change: %v", err)
	}
	if !dirty {
		t.Error("WorkingTreeDirty with an unstaged change = false, want true")
	}

	// Staged change only.
	runGit(t, dir, "add", "README")
	dirty, err = WorkingTreeDirty(dir)
	if err != nil {
		t.Fatalf("WorkingTreeDirty with staged change: %v", err)
	}
	if !dirty {
		t.Error("WorkingTreeDirty with a staged change = false, want true")
	}

	// Untracked files are not reported (git diff ignores them), matching the
	// scripts' clean-tree check.
	writeFile(t, dir, "untracked.txt", "new\n")
	runGit(t, dir, "reset", "-q", "HEAD", "README")
	writeFile(t, dir, "README", "init\n")
	dirty, err = WorkingTreeDirty(dir)
	if err != nil {
		t.Fatalf("WorkingTreeDirty with only untracked file: %v", err)
	}
	if dirty {
		t.Error("WorkingTreeDirty with only an untracked file = true, want false")
	}
}

func TestWorkingTreeDirtyNotARepo(t *testing.T) {
	_, err := WorkingTreeDirty(t.TempDir())
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("WorkingTreeDirty on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestSymbolicRefHead(t *testing.T) {
	dir, _ := initRepo(t)

	// No origin/HEAD → the "main" fallback.
	if got := SymbolicRefHead(dir); got != "main" {
		t.Errorf("SymbolicRefHead without origin/HEAD = %q, want main", got)
	}

	// With origin/HEAD pointing at refs/remotes/origin/development.
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/development")
	if got := SymbolicRefHead(dir); got != "development" {
		t.Errorf("SymbolicRefHead = %q, want development", got)
	}
}

func TestRefExists(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "branch", "feature/abc123_x")

	ok, err := RefExists(dir, "feature/abc123_x")
	if err != nil || !ok {
		t.Fatalf("RefExists on existing branch: ok=%v err=%v", ok, err)
	}
	ok, err = RefExists(dir, def)
	if err != nil || !ok {
		t.Fatalf("RefExists on base branch: ok=%v err=%v", ok, err)
	}
	ok, err = RefExists(dir, "no-such-branch")
	if err != nil {
		t.Fatalf("RefExists on missing branch: unexpected error %v", err)
	}
	if ok {
		t.Error("RefExists on a missing branch = true, want false")
	}
}

func TestRefExistsNotARepo(t *testing.T) {
	_, err := RefExists(t.TempDir(), "main")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("RefExists on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestGitCommonDir(t *testing.T) {
	dir, def := initRepo(t)
	mainGitDir, err := execOutput(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		t.Fatal(err)
	}
	if got := GitCommonDir(dir); got != mainGitDir {
		t.Errorf("GitCommonDir(main) = %q, want %q", got, mainGitDir)
	}

	// A worktree's common dir is the main repo's .git, not the worktree's own.
	wtPath := filepath.Join(t.TempDir(), "job-x")
	runGit(t, dir, "worktree", "add", wtPath, "-b", "feature/abc123_x", def)
	if got := GitCommonDir(wtPath); got != mainGitDir {
		t.Errorf("GitCommonDir(worktree) = %q, want main git dir %q", got, mainGitDir)
	}
}

func TestGitCommonDirNotARepo(t *testing.T) {
	if got := GitCommonDir(t.TempDir()); got != "" {
		t.Errorf("GitCommonDir on non-repo = %q, want empty", got)
	}
}

func TestExcludePath(t *testing.T) {
	dir, _ := initRepo(t)
	exclude := filepath.Join(dir, ".git", "info", "exclude")

	if err := ExcludePath(dir, ".manigot-worktrees/"); err != nil {
		t.Fatalf("ExcludePath: %v", err)
	}
	data, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}
	if !strings.Contains(string(data), ".manigot-worktrees/") {
		t.Errorf("exclude file does not contain the pattern: %q", data)
	}

	// Idempotent: a second call must not duplicate the line.
	if err := ExcludePath(dir, ".manigot-worktrees/"); err != nil {
		t.Fatalf("ExcludePath (second): %v", err)
	}
	data2, _ := os.ReadFile(exclude)
	if strings.Count(string(data2), ".manigot-worktrees/") != 1 {
		t.Errorf("exclude pattern duplicated: %q", data2)
	}
}

func TestExcludePathNotARepo(t *testing.T) {
	err := ExcludePath(t.TempDir(), ".manigot-worktrees/")
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("ExcludePath on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestExcludeMountTargets(t *testing.T) {
	dir, _ := initRepo(t)
	exclude := filepath.Join(dir, ".git", "info", "exclude")

	if err := ExcludeMountTargets(dir); err != nil {
		t.Fatalf("ExcludeMountTargets: %v", err)
	}
	data, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}
	for _, pattern := range mountTargetExcludePatterns {
		if !strings.Contains(string(data), pattern) {
			t.Errorf("exclude file missing %q: %q", pattern, data)
		}
	}

	// Idempotent: a second call must not duplicate any line.
	if err := ExcludeMountTargets(dir); err != nil {
		t.Fatalf("ExcludeMountTargets (second): %v", err)
	}
	data2, _ := os.ReadFile(exclude)
	for _, pattern := range mountTargetExcludePatterns {
		if strings.Count(string(data2), pattern) != 1 {
			t.Errorf("exclude pattern %q duplicated: %q", pattern, data2)
		}
	}
}

func TestExcludeMountTargetsNotARepo(t *testing.T) {
	// A non-repo is not an error — there is no git tracking to protect.
	if err := ExcludeMountTargets(t.TempDir()); err != nil {
		t.Errorf("ExcludeMountTargets on non-repo: err = %v, want nil", err)
	}
}

func TestRevParseToplevel(t *testing.T) {
	dir, _ := initRepo(t)
	got, err := RevParseToplevel(dir)
	if err != nil {
		t.Fatalf("RevParseToplevel: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(dir) {
		t.Errorf("RevParseToplevel = %q, want %q", got, dir)
	}
}

func TestRevParseToplevelNotARepo(t *testing.T) {
	_, err := RevParseToplevel(t.TempDir())
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("RevParseToplevel on non-repo: err = %v, want ErrNotARepo", err)
	}
}

func TestConfigUserNameEmail(t *testing.T) {
	// Isolate from the host's global git config so only the repo's own
	// local config is visible to `git config` reads.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	dir, _ := initRepo(t)
	if got := ConfigUserName(dir); got != "Test" {
		t.Errorf("ConfigUserName = %q, want Test", got)
	}
	if got := ConfigEmail(dir); got != "t@example.com" {
		t.Errorf("ConfigEmail = %q, want t@example.com", got)
	}

	// Unset key → "".
	runGit(t, dir, "config", "--unset", "user.email")
	if got := ConfigEmail(dir); got != "" {
		t.Errorf("ConfigEmail after unset = %q, want empty", got)
	}
}

func TestStage(t *testing.T) {
	dir, _ := initRepo(t)
	writeFile(t, dir, "docs/jobs/abc123_x/brief.md", "brief\n")
	writeFile(t, dir, "docs/jobs/abc123_x/tasks.md", "tasks\n")

	if err := Stage(dir, "."); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	// The staged files are committed by the follow-up CommitStaged.
	if err := CommitStaged(dir, "Scaffold job abc123_x"); err != nil {
		t.Fatalf("CommitStaged after Stage: %v", err)
	}
	out, err := execOutput(dir, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if out != "Scaffold job abc123_x" {
		t.Errorf("commit subject = %q, want %q", out, "Scaffold job abc123_x")
	}
}

// execOutput runs `git -C dir args` and returns trimmed stdout.
func execOutput(dir string, args ...string) (string, error) {
	out, _, err := run(dir, args...)
	return strings.TrimSpace(string(out)), err
}
