package job

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/git"
)

// gitWorktreeForBranch reports whether branch has a registered worktree in
// repoDir, via the git package itself.
func gitWorktreeForBranch(t *testing.T, repoDir, branch string) (string, bool, error) {
	t.Helper()
	return git.WorktreeForBranch(repoDir, branch)
}

// orphanWorktree creates a git repo at repoDir, registers a worktree at wtPath
// on branch (like git worktree add), then deletes the worktree's gitdir
// metadata and its branch behind git's back — leaving the working directory
// with a .git file that points at a gitdir no longer existing. That is exactly
// the orphan shape the five dead dirs in .manigot-worktrees/ have (a dir whose
// .git file points at a vanished gitdir, with no branch and no registration —
// "a job scaffolded and then abandoned"), and the fixture `mg jobs` / `mg
// delete` must clean up.
func orphanWorktree(t *testing.T, repoDir, wtPath, branch string) {
	t.Helper()
	gitCmd(t, repoDir, "worktree", "add", wtPath, "-b", branch)
	// The worktree's .git file names a gitdir under the main repo's
	// .git/worktrees/<name>; delete that metadata dir, leaving wtPath behind.
	gitdir := readGitdir(filepath.Join(wtPath, ".git"))
	if err := os.RemoveAll(gitdir); err != nil {
		t.Fatal(err)
	}
	// A scaffolded-and-abandoned job leaves no branch either.
	gitCmd(t, repoDir, "branch", "-D", branch)
	// Nothing should be registered for this worktree anymore.
	if _, ok, err := gitWorktreeForBranch(t, repoDir, branch); err != nil || ok {
		t.Fatalf("worktree still registered for %s: ok=%v err=%v", branch, ok, err)
	}
}

func TestDiscoverOrphansDetectsDeadDir(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	// Sibling layout: <dirname(root)>/.manigot-worktrees/<basename(root)>/<name>.
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	wtPath := filepath.Join(wtParent, "o3kk3n_jdi-is-broken")
	orphanWorktree(t, root, wtPath, "feature/o3kk3n_jdi-is-broken")

	// A live worktree next to it must NOT be reported.
	liveWT := filepath.Join(wtParent, "ab12cd_live")
	gitCmd(t, root, "worktree", "add", liveWT, "-b", "feature/ab12cd_live")

	orphans, err := DiscoverOrphans(root)
	if err != nil {
		t.Fatalf("DiscoverOrphans: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("len(orphans) = %d, want 1 (the dead dir only): %+v", len(orphans), orphans)
	}
	if orphans[0].Name != "o3kk3n_jdi-is-broken" {
		t.Errorf("orphan Name = %q, want o3kk3n_jdi-is-broken", orphans[0].Name)
	}
	if orphans[0].Dir != wtPath {
		t.Errorf("orphan Dir = %q, want %q", orphans[0].Dir, wtPath)
	}
	if !strings.Contains(orphans[0].GitDir, ".git/worktrees/") {
		t.Errorf("orphan GitDir = %q, want a .git/worktrees path", orphans[0].GitDir)
	}
}

func TestDiscoverOrphansNestedLayout(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	// Force the nested (mount-point) layout: <root>/.manigot-worktrees/<name>.
	res, err := CreateJob(root, "Nested Orphan", CreateOptions{
		RandomID:    fixedID("ab12cd"),
		DeviceCheck: func(parent, root string) bool { return true },
	}, bytes.NewBuffer(nil))
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	// Kill the gitdir metadata behind git's back, leaving the dir behind.
	gitdir := strings.TrimPrefix(readGitdir(filepath.Join(res.WorktreePath, ".git")), "file://")
	gitCmd(t, root, "worktree", "remove", "--force", res.WorktreePath)
	if err := os.MkdirAll(res.WorktreePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res.WorktreePath, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orphans, err := DiscoverOrphans(root)
	if err != nil {
		t.Fatalf("DiscoverOrphans: %v", err)
	}
	if len(orphans) != 1 || orphans[0].Name != "ab12cd_nested-orphan" {
		t.Fatalf("orphans = %+v, want the nested-layout dead dir ab12cd_nested-orphan", orphans)
	}
	if !strings.Contains(orphans[0].Dir, filepath.Join(root, ".manigot-worktrees")) {
		t.Errorf("orphan Dir = %q, want the nested layout", orphans[0].Dir)
	}
}

func TestDiscoverOrphansEmpty(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	orphans, err := DiscoverOrphans(root)
	if err != nil {
		t.Fatalf("DiscoverOrphans: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("orphans = %+v, want none on a clean repo", orphans)
	}
}

func TestDiscoverOrphansNonGit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	orphans, err := DiscoverOrphans(root)
	if err != nil {
		t.Fatalf("DiscoverOrphans on non-git: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("orphans = %+v, want none on a non-git project", orphans)
	}
}

func TestDiscoverOrphansIgnoresStandaloneRepo(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	// A .git *directory* under .manigot-worktrees is a standalone repo, not a
	// linked worktree — it must never be reported as an orphan.
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	standalone := filepath.Join(wtParent, "real-repo")
	if err := os.MkdirAll(filepath.Join(standalone, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	orphans, err := DiscoverOrphans(root)
	if err != nil {
		t.Fatalf("DiscoverOrphans: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("orphans = %+v, want none (standalone .git dir is not a worktree)", orphans)
	}
}

func TestMatchOrphan(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	orphanWorktree(t, root, filepath.Join(wtParent, "o3kk3n_jdi-is-broken"), "feature/o3kk3n_jdi-is-broken")

	// Exact.
	o, ok := MatchOrphan(root, "o3kk3n_jdi-is-broken")
	if !ok || o.Name != "o3kk3n_jdi-is-broken" {
		t.Errorf("exact match = %+v ok=%v", o, ok)
	}
	// Prefix.
	o, ok = MatchOrphan(root, "o3kk3n")
	if !ok || o.Name != "o3kk3n_jdi-is-broken" {
		t.Errorf("prefix match = %+v ok=%v", o, ok)
	}
	// No match.
	if _, ok := MatchOrphan(root, "zzzz99"); ok {
		t.Error("MatchOrphan(zzzz99) = ok, want false")
	}
}

func TestRemoveOrphans(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	orphanWorktree(t, root, filepath.Join(wtParent, "o3kk3n_jdi-is-broken"), "feature/o3kk3n_jdi-is-broken")
	// An abandoned job's mg-jdi sidecar (same name) is stale too.
	if err := WriteJDIStatus(root, "o3kk3n_jdi-is-broken", JDIStoppedNeedsHuman, "analyst"); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := RemoveOrphans(root, []Orphan{{Name: "o3kk3n_jdi-is-broken", Dir: filepath.Join(wtParent, "o3kk3n_jdi-is-broken")}}, yesConfirm, &out)
	if err != nil {
		t.Fatalf("RemoveOrphans: %v\n%s", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(wtParent, "o3kk3n_jdi-is-broken")); !os.IsNotExist(err) {
		t.Errorf("orphan dir still exists after removal: %v", err)
	}
	if _, err := os.Stat(JDIStatusDir(root, "o3kk3n_jdi-is-broken")); !os.IsNotExist(err) {
		t.Errorf("mg-jdi sidecar still exists after orphan removal: %v", err)
	}
	if !strings.Contains(out.String(), "✓ Orphan removed: o3kk3n_jdi-is-broken") {
		t.Errorf("missing removal line:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "This cannot be undone.") {
		t.Errorf("missing 'This cannot be undone.':\n%s", out.String())
	}
}

func TestRemoveOrphansDeclined(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	orphanPath := filepath.Join(wtParent, "o3kk3n_jdi-is-broken")
	orphanWorktree(t, root, orphanPath, "feature/o3kk3n_jdi-is-broken")

	var out bytes.Buffer
	err := RemoveOrphans(root, []Orphan{{Name: "o3kk3n_jdi-is-broken", Dir: orphanPath}}, noConfirm, &out)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("declined removal: err = %v, want ErrCancelled", err)
	}
	if _, err := os.Stat(orphanPath); err != nil {
		t.Errorf("orphan dir removed despite a declined confirmation: %v", err)
	}
}

func TestRemoveOrphansStopsOnDecline(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	first := filepath.Join(wtParent, "aaa01_first")
	second := filepath.Join(wtParent, "bbb02_second")
	orphanWorktree(t, root, first, "feature/aaa01_first")
	orphanWorktree(t, root, second, "feature/bbb02_second")

	var out bytes.Buffer
	err := RemoveOrphans(root, []Orphan{
		{Name: "aaa01_first", Dir: first},
		{Name: "bbb02_second", Dir: second},
	}, noConfirm, &out)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("declined removal: err = %v, want ErrCancelled", err)
	}
	if _, err := os.Stat(first); err != nil {
		t.Errorf("first orphan removed: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Errorf("second orphan removed: %v", err)
	}
}
