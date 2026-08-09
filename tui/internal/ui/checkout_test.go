package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/safecode/tui/internal/job"
)

// keyMsg builds a single-rune key press, e.g. keyMsg("c"), for driving
// updateDetail directly in tests below.
func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestCheckoutKeySwitchesBranchAndRebuildsDetail exercises the full "c" flow:
// pressing it in the detail view of an off-branch job fires checkoutCmd,
// running that tea.Cmd checks out the job's branch for real, and feeding the
// resulting checkoutMsg back through Update rebuilds the detail view so the
// job now reads from the working tree (OnCurrentBranch flips to true and the
// tabs show the branch's real content instead of the git-show snapshot).
func TestCheckoutKeySwitchesBranchAndRebuildsDetail(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/off10_o")
	gitCommitJob(t, dir, "off10_o",
		"# Brief: Off\n\nstatus: open\nid: off10\nbranch: feature/off10_o\ndate: 2026-01-01\n\n## What\n\nZZOFFBODY\n")
	gitRun(t, dir, "checkout", "-q", def)

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}
	if jobs[0].OnCurrentBranch {
		t.Fatal("expected an off-branch job to start")
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// Press "c" — must return a tea.Cmd rather than mutate git inline.
	_, cmd := a.updateDetail(keyMsg("c"))
	if cmd == nil {
		t.Fatal("expected \"c\" to return a tea.Cmd, got nil")
	}

	msg := cmd()
	checkoutM, ok := msg.(checkoutMsg)
	if !ok {
		t.Fatalf("expected checkoutMsg, got %T", msg)
	}
	if checkoutM.err != nil {
		t.Fatalf("checkout failed: %v", checkoutM.err)
	}
	if checkoutM.branch != "feature/off10_o" {
		t.Errorf("checkoutMsg.branch = %q, want feature/off10_o", checkoutM.branch)
	}

	// The working tree really did switch.
	out, execErr := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if execErr != nil || strings.TrimSpace(string(out)) != "feature/off10_o" {
		t.Fatalf("git HEAD after checkout = %q (err=%v), want feature/off10_o", out, execErr)
	}

	model, followUp := a.Update(checkoutM)
	if followUp != nil {
		t.Errorf("expected no follow-up cmd from checkoutMsg handling, got one")
	}
	got := model.(*App)

	if got.detail == nil {
		t.Fatal("detail view should stay open after a successful checkout")
	}
	if !got.detail.job.OnCurrentBranch {
		t.Error("job should be OnCurrentBranch after switching to its own branch")
	}
	if !strings.Contains(got.detail.tabs[0].viewer.View(), "ZZOFFBODY") {
		t.Errorf("brief tab not reloaded from the working tree after checkout; view:\n%s", got.detail.tabs[0].viewer.View())
	}
	if !strings.Contains(got.detail.status, "switched to feature/off10_o") {
		t.Errorf("status = %q, want it to mention the switched branch", got.detail.status)
	}
}

// TestCheckoutMsgErrorSurfacesInDetailStatus verifies a checkout git refuses
// (e.g. uncommitted changes it would clobber) is reported in the detail
// view's status line, and does not touch the job list or the open job.
func TestCheckoutMsgErrorSurfacesInDetailStatus(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "cur10_c", "# Brief: Cur\n\nstatus: open\nid: cur10\nbranch: "+def+"\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	wantErr := errors.New("git checkout other: exit status 1: error: your local changes would be overwritten")
	model, cmd := a.Update(checkoutMsg{branch: "other", err: wantErr})
	if cmd != nil {
		t.Errorf("expected no follow-up cmd, got one")
	}
	got := model.(*App)

	if got.detail == nil {
		t.Fatal("detail view should stay open after a failed checkout")
	}
	want := cmdErrorText(wantErr)
	if got.detail.status != want {
		t.Errorf("status = %q, want %q", got.detail.status, want)
	}
	if got.detail.job.ID != "cur10" {
		t.Errorf("open job changed after a failed checkout: %q", got.detail.job.ID)
	}
	if len(got.jobs) != 1 {
		t.Errorf("job list should be untouched by a failed checkout; got %d job(s)", len(got.jobs))
	}
}

// TestCheckoutKeyWithoutBranchIsNoop verifies a job discovered outside a git
// repository (Discover's working-tree-only fallback, where Branch is left
// empty — see discoverWorkingTree) reports a status instead of dispatching a
// checkout with an empty branch name.
func TestCheckoutKeyWithoutBranchIsNoop(t *testing.T) {
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

	_, cmd := a.updateDetail(keyMsg("c"))
	if cmd != nil {
		t.Error("expected no tea.Cmd when the job has no known branch")
	}
	if a.detail.status == "" {
		t.Error("expected a status message explaining why \"c\" did nothing")
	}
}

// --- list-view "m" back-to-main quick checkout (TASK-4) --------------------

// gitEnsureMain renames def to a branch literally named "main" if it isn't
// already, so the "m" key's hardcoded "main" target is exercised regardless
// of this git installation's init.defaultBranch config (which may be
// "master" or anything else).
func gitEnsureMain(t *testing.T, dir, def string) {
	t.Helper()
	if def == "main" {
		return
	}
	gitRun(t, dir, "branch", "-m", def, "main")
}

// TestMainKeySwitchesToMainFromList exercises the full "m" flow from the list
// view (no detail open): pressing it fires checkoutCmd("main"), running that
// tea.Cmd checks out main for real, and feeding the resulting checkoutMsg
// back through Update updates a.status and a.currentBranch/a.jobs — the
// a.detail == nil branch of the checkoutMsg handler that
// TestCheckoutKeySwitchesBranchAndRebuildsDetail above doesn't exercise.
func TestMainKeySwitchesToMainFromList(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitEnsureMain(t, dir, def)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/off10_o")
	gitCommitJob(t, dir, "off10_o",
		"# Brief: Off\n\nstatus: open\nid: off10\nbranch: feature/off10_o\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.state = stateList
	if a.currentBranch != "feature/off10_o" {
		t.Fatalf("currentBranch = %q, want feature/off10_o", a.currentBranch)
	}

	model, cmd := a.updateList(keyMsg("m"))
	got := model.(*App)
	if cmd == nil {
		t.Fatal("expected \"m\" to return a tea.Cmd, got nil")
	}

	msg := cmd()
	checkoutM, ok := msg.(checkoutMsg)
	if !ok {
		t.Fatalf("expected checkoutMsg, got %T", msg)
	}
	if checkoutM.err != nil {
		t.Fatalf("checkout failed: %v", checkoutM.err)
	}
	if checkoutM.branch != "main" {
		t.Errorf("checkoutMsg.branch = %q, want main", checkoutM.branch)
	}

	// The working tree really did switch.
	out, execErr := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if execErr != nil || strings.TrimSpace(string(out)) != "main" {
		t.Fatalf("git HEAD after checkout = %q (err=%v), want main", out, execErr)
	}

	model2, followUp := got.Update(checkoutM)
	if followUp != nil {
		t.Errorf("expected no follow-up cmd from checkoutMsg handling, got one")
	}
	final := model2.(*App)

	if final.detail != nil {
		t.Error("checking out main from the list must not open a detail view")
	}
	if final.currentBranch != "main" {
		t.Errorf("currentBranch after checkout = %q, want main", final.currentBranch)
	}
	if !strings.Contains(final.status, "switched to main") {
		t.Errorf("status = %q, want it to mention switching to main", final.status)
	}
	// The off-branch job should now show as off-branch relative to main.
	if len(final.jobs) != 1 || final.jobs[0].OnCurrentBranch {
		t.Errorf("jobs after checkout = %+v, want the job to no longer be OnCurrentBranch", final.jobs)
	}
}

// TestMainKeyAlreadyOnMainStillReportsStatus verifies checking out main while
// already on it — a no-op as far as git is concerned (`git checkout` on the
// already-current branch still exits 0) — still surfaces a clear status
// rather than silently doing nothing.
func TestMainKeyAlreadyOnMainStillReportsStatus(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitEnsureMain(t, dir, def)
	gitCommitJob(t, dir, "cur10_c", "# Brief: Cur\n\nstatus: open\nid: cur10\nbranch: main\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.state = stateList

	model, cmd := a.updateList(keyMsg("m"))
	got := model.(*App)
	if cmd == nil {
		t.Fatal("expected \"m\" to return a tea.Cmd even when already on main")
	}
	msg := cmd()
	checkoutM := msg.(checkoutMsg)
	if checkoutM.err != nil {
		t.Fatalf("checkout of the already-current branch failed: %v", checkoutM.err)
	}

	model2, _ := got.Update(checkoutM)
	final := model2.(*App)
	if !strings.Contains(final.status, "switched to main") {
		t.Errorf("status = %q, want it to still mention main on a no-op checkout", final.status)
	}
}

// TestMainKeyRefusedCheckoutSurfacesStatusInList verifies a checkout git
// refuses (e.g. uncommitted changes it would clobber) is reported in
// a.status when there is no open detail view, and leaves the job list alone.
func TestMainKeyRefusedCheckoutSurfacesStatusInList(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitCommitJob(t, dir, "cur10_c", "# Brief: Cur\n\nstatus: open\nid: cur10\nbranch: "+def+"\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.state = stateList

	wantErr := errors.New("git checkout main: exit status 1: error: your local changes would be overwritten")
	model, cmd := a.Update(checkoutMsg{branch: "main", err: wantErr})
	if cmd != nil {
		t.Errorf("expected no follow-up cmd, got one")
	}
	got := model.(*App)

	if got.detail != nil {
		t.Error("no detail view should be open after a list-view checkout failure")
	}
	want := cmdErrorText(wantErr)
	if got.status != want {
		t.Errorf("status = %q, want %q", got.status, want)
	}
	if len(got.jobs) != 1 {
		t.Errorf("job list should be untouched by a failed checkout; got %d job(s)", len(got.jobs))
	}
}
