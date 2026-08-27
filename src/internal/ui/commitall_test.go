package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
)

// TestCommitAllKeyCommitsDirtyWorktree exercises the full "c" flow: pressing
// it on a dirty worktree fires commitAllCmd, running that tea.Cmd stages and
// commits everything for real in the job's own worktree (git add -A + git
// commit), and feeding the resulting commitAllMsg back through Update sets
// the detail view's status.
func TestCommitAllKeyCommitsDirtyWorktree(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/cmt01_c", "cmt01_c", "# Brief: CommitAll\n\nstatus: open\nid: cmt01\nbranch: feature/cmt01_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}

	// Dirty the job worktree: a modified tracked file and a new untracked
	// one — the leftovers agents sometimes leave behind.
	if err := os.WriteFile(filepath.Join(wtPath, "README"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "leftover.txt"), []byte("agent leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	_, cmd := a.updateDetail(keyMsg("c"))
	if cmd == nil {
		t.Fatal("expected \"c\" to return a tea.Cmd, got nil")
	}

	msg := cmd()
	commitAllM, ok := msg.(commitAllMsg)
	if !ok {
		t.Fatalf("expected commitAllMsg, got %T", msg)
	}
	if commitAllM.err != nil {
		t.Fatalf("commit all failed: %v", commitAllM.err)
	}

	// The commit really landed on the job branch, in the job's own worktree.
	out, err := exec.Command("git", "-C", wtPath, "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if subject := strings.TrimSpace(string(out)); subject != "[cmt01] chore: commit all" {
		t.Errorf("commit subject = %q, want %q", subject, "[cmt01] chore: commit all")
	}

	// And the worktree is clean afterwards.
	statusOut, err := exec.Command("git", "-C", wtPath, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if len(statusOut) != 0 {
		t.Errorf("expected a clean job worktree after commit all, got:\n%s", statusOut)
	}

	// The main worktree must be untouched — the commit went to the job
	// worktree (git -C <worktree>), never a.root.
	mainOut, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatalf("git log (main): %v", err)
	}
	if strings.Contains(string(mainOut), "[cmt01] chore: commit all") {
		t.Errorf("commit-all leaked into the main worktree's history:\n%s", mainOut)
	}

	// Setting a non-empty status arms the blink-then-expire timer (see
	// App.setStatus/statusExpireMsg), so a follow-up cmd is now expected —
	// unlike before that mechanism existed.
	model, followUp := a.Update(commitAllM)
	if followUp == nil {
		t.Errorf("expected a follow-up cmd arming the status-expiry timer, got nil")
	}
	got := model.(*App)
	if got.detail == nil {
		t.Fatal("detail view should stay open after a successful commit all")
	}
	if got.detail.status != "→ committed all changes" {
		t.Errorf("status = %q, want %q", got.detail.status, "→ committed all changes")
	}
}

// TestCommitAllMsgNothingToCommitStatus verifies the distinct non-failure
// outcome: an already-clean worktree surfaces as a "nothing to commit" status,
// not an error.
func TestCommitAllMsgNothingToCommitStatus(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cmt02_c", "cmt02_c", "# Brief: C\n\nstatus: open\nid: cmt02\nbranch: feature/cmt02_c\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// Setting a non-empty status arms the blink-then-expire timer (see
	// App.setStatus/statusExpireMsg), so a follow-up cmd is now expected —
	// unlike before that mechanism existed.
	model, cmd := a.Update(commitAllMsg{err: git.ErrNothingToCommit})
	if cmd == nil {
		t.Errorf("expected a follow-up cmd arming the status-expiry timer, got nil")
	}
	got := model.(*App)
	if got.detail == nil {
		t.Fatal("detail view should stay open on a nothing-to-commit outcome")
	}
	if got.detail.status != "nothing to commit" {
		t.Errorf("status = %q, want %q", got.detail.status, "nothing to commit")
	}
}

// TestCommitAllMsgErrorSurfacesInDetailStatus verifies a real commit-all
// failure (e.g. git refusing the commit) is reported in the detail view's
// status line via the usual cmdErrorText formatting.
func TestCommitAllMsgErrorSurfacesInDetailStatus(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cmt03_c", "cmt03_c", "# Brief: C\n\nstatus: open\nid: cmt03\nbranch: feature/cmt03_c\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// Setting a non-empty status arms the blink-then-expire timer (see
	// App.setStatus/statusExpireMsg), so a follow-up cmd is now expected —
	// unlike before that mechanism existed.
	wantErr := errors.New("git commit: exit status 128: some failure")
	model, cmd := a.Update(commitAllMsg{err: wantErr})
	if cmd == nil {
		t.Errorf("expected a follow-up cmd arming the status-expiry timer, got nil")
	}
	got := model.(*App)
	if got.detail == nil {
		t.Fatal("detail view should stay open after a failed commit all")
	}
	if want := cmdErrorText(wantErr); got.detail.status != want {
		t.Errorf("status = %q, want %q", got.detail.status, want)
	}
}

// TestCommitAllKeyWithoutBranchIsNoop mirrors TestPushKeyWithoutBranchIsNoop:
// a job discovered outside a git repository (Discover's working-tree-only
// fallback, where Branch is left empty) reports a status instead of
// dispatching a commit with no branch.
func TestCommitAllKeyWithoutBranchIsNoop(t *testing.T) {
	dir := t.TempDir() // not a git repo: job.Discover falls back to discoverWorkingTree
	jobDir := filepath.Join(dir, "docs", "jobs", "nob01_n")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	brief := "# Brief: NoBranch\n\nstatus: open\nid: nob01\ndate: 2026-01-01\n"
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatalf("write brief.md: %v", err)
	}

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 || jobs[0].Branch != "" {
		t.Fatalf("expected one job with no branch info; got %+v", jobs)
	}
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// Setting the "no branch known" status arms the blink-then-expire timer
	// (see App.setStatus/statusExpireMsg), so a follow-up cmd is expected —
	// not the commit itself, just the status-expiry arming.
	_, cmd := a.updateDetail(keyMsg("c"))
	if cmd == nil {
		t.Error("expected a follow-up cmd arming the status-expiry timer, got nil")
	}
	if !strings.Contains(a.detail.status, "no branch known for this job") {
		t.Errorf("status = %q, want it to explain the missing branch", a.detail.status)
	}
}
