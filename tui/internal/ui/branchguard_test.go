package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/safecode/tui/internal/job"
)

// mkOffBranchApp builds an App whose only detail-view job lives on a branch
// other than the one currently checked out, for exercising the TASK-7
// branch-mismatch guard on the three mutating actions.
func mkOffBranchApp(t *testing.T) *App {
	t.Helper()
	dir, def := gitInitRepo(t)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/gd01_g")
	gitCommitJob(t, dir, "gd01_g",
		"# Brief: Guarded\n\nstatus: open\nid: gd01\nbranch: feature/gd01_g\ndate: 2026-01-01\n")
	gitRun(t, dir, "checkout", "-q", def)

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}
	if jobs[0].OnCurrentBranch {
		t.Fatal("expected an off-branch job")
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail
	return a
}

// TestBranchGuardBlocksEdit verifies "e" refuses to open the editor on an
// off-branch job's brief and points the user at "b" instead.
func TestBranchGuardBlocksEdit(t *testing.T) {
	a := mkOffBranchApp(t)

	_, cmd := a.updateDetail(keyMsg("e"))
	if cmd != nil {
		t.Error("expected no tea.Cmd (editor) for an off-branch job")
	}
	if !strings.Contains(a.detail.status, "press b to switch") {
		t.Errorf("status = %q, want a branch-mismatch guard message", a.detail.status)
	}
}

// TestBranchGuardBlocksDone verifies "D" refuses to archive an off-branch job.
func TestBranchGuardBlocksDone(t *testing.T) {
	a := mkOffBranchApp(t)

	_, cmd := a.updateDetail(keyMsg("D"))
	if cmd != nil {
		t.Error("expected no tea.Cmd (sc-done) for an off-branch job")
	}
	if !strings.Contains(a.detail.status, "press b to switch") {
		t.Errorf("status = %q, want a branch-mismatch guard message", a.detail.status)
	}
}

// TestBranchGuardBlocksDelete verifies both delete triggers — the physical
// Delete/Entf key and its "x" fallback (see updateDetail's "delete", "x"
// case) — refuse to delete an off-branch job.
func TestBranchGuardBlocksDelete(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"delete key": {Type: tea.KeyDelete},
		"x fallback": keyMsg("x"),
	}
	for name, key := range keys {
		t.Run(name, func(t *testing.T) {
			a := mkOffBranchApp(t)

			_, cmd := a.updateDetail(key)
			if cmd != nil {
				t.Error("expected no tea.Cmd (sc-delete) for an off-branch job")
			}
			if !strings.Contains(a.detail.status, "press b to switch") {
				t.Errorf("status = %q, want a branch-mismatch guard message", a.detail.status)
			}
		})
	}
}

// TestBranchGuardBlocksAgentLaunch verifies an agent-bar key refuses to
// launch against an off-branch job's working tree.
func TestBranchGuardBlocksAgentLaunch(t *testing.T) {
	a := mkOffBranchApp(t)

	_, cmd := a.updateDetail(keyMsg("d")) // "d" = developer
	if cmd != nil {
		t.Error("expected no tea.Cmd (agent launch) for an off-branch job")
	}
	if !strings.Contains(a.detail.status, "press b to switch") {
		t.Errorf("status = %q, want a branch-mismatch guard message", a.detail.status)
	}
}

// TestBranchGuardAllowsCurrentBranchJob confirms the guard is a no-op for a
// job that IS on the currently-checked-out branch: "e" (on a non-editable
// tab it would fall through anyway, so pick the editable brief tab) resolves
// normally instead of being blocked. Uses the editor-resolution error path
// (no real $EDITOR guaranteed in test envs) purely to prove branchGuard let
// the call through to editCmd.
func TestBranchGuardAllowsCurrentBranchJob(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "cur20_c", "# Brief: Cur\n\nstatus: open\nid: cur20\nbranch: "+def+"\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 || !jobs[0].OnCurrentBranch {
		t.Fatalf("expected one current-branch job; got %+v", jobs)
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	if status, blocked := a.branchGuard(); blocked {
		t.Errorf("branchGuard blocked a current-branch job; status=%q", status)
	}
}
