package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
)

// gitAddBareOrigin creates a bare repo elsewhere and wires it up as dir's
// origin remote — a real remote for pushCmd's `git push` to exercise, same
// spirit as the git package's own bareRepo test helper.
func gitAddBareOrigin(t *testing.T, dir string) string {
	t.Helper()
	origin := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "--bare", origin)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare %s: %v\n%s", origin, err, out)
	}
	gitRun(t, dir, "remote", "add", "origin", origin)
	return origin
}

// TestPushKeyPushesBranchToOrigin exercises the full "P" flow: pressing it
// fires pushCmd, running that tea.Cmd pushes the job's branch to origin for
// real, and feeding the resulting pushMsg back through Update sets the
// detail view's status.
func TestPushKeyPushesBranchToOrigin(t *testing.T) {
	dir, _ := gitInitRepo(t)
	origin := gitAddBareOrigin(t, dir)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/psh10_p", "psh10_p",
		"# Brief: Push\n\nstatus: open\nid: psh10\nbranch: feature/psh10_p\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	_, cmd := a.updateDetail(keyMsg("P"))
	if cmd == nil {
		t.Fatal("expected \"P\" to return a tea.Cmd, got nil")
	}

	msg := cmd()
	pushM, ok := msg.(pushMsg)
	if !ok {
		t.Fatalf("expected pushMsg, got %T", msg)
	}
	if pushM.err != nil {
		t.Fatalf("push failed: %v", pushM.err)
	}
	if pushM.branch != "feature/psh10_p" {
		t.Errorf("pushMsg.branch = %q, want feature/psh10_p", pushM.branch)
	}

	// The branch ref really landed on the bare remote.
	localHash, execErr := exec.Command("git", "-C", dir, "rev-parse", "feature/psh10_p").Output()
	if execErr != nil {
		t.Fatal(execErr)
	}
	remoteHash, execErr := exec.Command("git", "-C", origin, "rev-parse", "feature/psh10_p").Output()
	if execErr != nil {
		t.Fatalf("rev-parse on remote after push: %v", execErr)
	}
	if strings.TrimSpace(string(localHash)) != strings.TrimSpace(string(remoteHash)) {
		t.Errorf("remote feature/psh10_p = %q, want %q (local)", remoteHash, localHash)
	}

	model, followUp := a.Update(pushM)
	if followUp != nil {
		t.Errorf("expected no follow-up cmd from pushMsg handling, got one")
	}
	got := model.(*App)
	if got.detail == nil {
		t.Fatal("detail view should stay open after a successful push")
	}
	if !strings.Contains(got.detail.status, "pushed feature/psh10_p to origin") {
		t.Errorf("status = %q, want it to mention the pushed branch", got.detail.status)
	}
}

// TestPushMsgErrorSurfacesInDetailStatus verifies a push git refuses (e.g.
// no origin remote configured) is reported in the detail view's status line.
func TestPushMsgErrorSurfacesInDetailStatus(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur10_c", "cur10_c", "# Brief: Cur\n\nstatus: open\nid: cur10\nbranch: feature/cur10_c\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	wantErr := errors.New("git push origin feature/cur10_c: exit status 128: fatal: 'origin' does not appear to be a git repository")
	model, cmd := a.Update(pushMsg{branch: "feature/cur10_c", err: wantErr})
	if cmd != nil {
		t.Errorf("expected no follow-up cmd, got one")
	}
	got := model.(*App)

	if got.detail == nil {
		t.Fatal("detail view should stay open after a failed push")
	}
	want := cmdErrorText(wantErr)
	if got.detail.status != want {
		t.Errorf("status = %q, want %q", got.detail.status, want)
	}
}

// TestPushKeyWithoutBranchIsNoop mirrors the deleted TestCheckoutKeyWithoutBranchIsNoop:
// a job discovered outside a git repository (Discover's working-tree-only
// fallback, where Branch is left empty — see discoverWorkingTree) reports a
// status instead of dispatching a push with an empty branch name.
func TestPushKeyWithoutBranchIsNoop(t *testing.T) {
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

	_, cmd := a.updateDetail(keyMsg("P"))
	if cmd != nil {
		t.Error("expected no tea.Cmd when the job has no known branch")
	}
	if a.detail.status == "" {
		t.Error("expected a status message explaining why \"P\" did nothing")
	}
}
