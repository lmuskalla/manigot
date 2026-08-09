package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/tui/internal/job"
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

// assertBlocked asserts the shared reaction to a branchGuard block: no
// footer status text (the reaction lives entirely in the off-branch hint up
// in the title+meta line, not a second message down at the footer — see
// detailView.blockedByBranch), branchFlash set so that hint blinks, and a
// non-nil cmd — now always the branchFlashDoneMsg timer from
// blockedByBranchCmd, never the actual editor/mg-done/mg-delete/agent-launch
// action, since the blocked branch returns before ever calling those.
func assertBlocked(t *testing.T, a *App, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Error("expected the branchFlashDoneMsg timer cmd even when blocked")
	}
	if a.detail.status != "" {
		t.Errorf("status = %q, want empty — the reaction is the flashing hint up top, not a footer message", a.detail.status)
	}
	if !a.detail.branchFlash {
		t.Error("a branch-mismatch block should set branchFlash so the title+meta hint blinks")
	}
}

// TestBranchGuardBlocksEdit verifies "e" refuses to open the editor on an
// off-branch job's brief.
func TestBranchGuardBlocksEdit(t *testing.T) {
	a := mkOffBranchApp(t)
	_, cmd := a.updateDetail(keyMsg("e"))
	assertBlocked(t, a, cmd)
}

// TestBranchGuardBlocksDone verifies "D" refuses to archive an off-branch job.
func TestBranchGuardBlocksDone(t *testing.T) {
	a := mkOffBranchApp(t)
	_, cmd := a.updateDetail(keyMsg("D"))
	assertBlocked(t, a, cmd)
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
			assertBlocked(t, a, cmd)
		})
	}
}

// TestBranchGuardBlocksAgentLaunch verifies an agent-bar key refuses to
// launch against an off-branch job's working tree.
func TestBranchGuardBlocksAgentLaunch(t *testing.T) {
	a := mkOffBranchApp(t)
	_, cmd := a.updateDetail(keyMsg("d")) // "d" = developer
	assertBlocked(t, a, cmd)
}

// TestBranchGuardBlocksJdi verifies "J" refuses to launch mg-jdi against an
// off-branch job's working tree, the same as the five single-agent keys.
func TestBranchGuardBlocksJdi(t *testing.T) {
	a := mkOffBranchApp(t)
	_, cmd := a.updateDetail(keyMsg("J"))
	assertBlocked(t, a, cmd)
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

// TestBranchFlashClearsOnMatchingGen simulates blockedByBranchCmd's timer
// firing (without actually waiting branchFlashDuration): a branchFlashDoneMsg
// carrying the generation the block just set clears branchFlash.
func TestBranchFlashClearsOnMatchingGen(t *testing.T) {
	a := mkOffBranchApp(t)
	a.updateDetail(keyMsg("e"))
	if !a.detail.branchFlash {
		t.Fatal("setup: expected branchFlash set after a blocked action")
	}
	gen := a.detail.branchFlashGen

	model, _ := a.Update(branchFlashDoneMsg{gen: gen})
	got := model.(*App)
	if got.detail.branchFlash {
		t.Error("a branchFlashDoneMsg with the current generation should clear branchFlash")
	}
}

// TestBranchFlashStaleGenIgnored guards the race blockedByBranchCmd's gen
// exists for: a delayed timer from an earlier blocked attempt must not clear
// a flash a later attempt (re)triggered in the meantime.
func TestBranchFlashStaleGenIgnored(t *testing.T) {
	a := mkOffBranchApp(t)
	a.updateDetail(keyMsg("e")) // first blocked attempt
	staleGen := a.detail.branchFlashGen
	a.updateDetail(keyMsg("D")) // second blocked attempt — bumps the generation again

	model, _ := a.Update(branchFlashDoneMsg{gen: staleGen})
	got := model.(*App)
	if !got.detail.branchFlash {
		t.Error("a stale generation's timer must not clear a flash a later attempt re-triggered")
	}
}

// TestBranchFlashDoneMsgNoDetail confirms a branchFlashDoneMsg arriving after
// the user has already left the detail view (e.g. pressed esc before the
// timer fired) is a safe no-op rather than a nil-pointer panic.
func TestBranchFlashDoneMsgNoDetail(t *testing.T) {
	a := &App{}
	model, cmd := a.Update(branchFlashDoneMsg{gen: 1})
	if cmd != nil {
		t.Error("expected no follow-up cmd")
	}
	if model.(*App).detail != nil {
		t.Error("detail should still be nil")
	}
}
