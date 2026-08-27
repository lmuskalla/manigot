package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/launch"
)

// stubTigLookPath points launch.TigLookPath at fn and restores it after the
// test — the internal/ui equivalent of the seam internal/launch's own tests
// use, so tig availability can be controlled without a real tig on PATH.
func stubTigLookPath(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	old := launch.TigLookPath
	launch.TigLookPath = fn
	t.Cleanup(func() { launch.TigLookPath = old })
}

// tigAvailable always resolves tig (the availability gate's happy path).
func tigResolves(name string) (string, error) { return "/usr/bin/tig", nil }

// tigMissing never resolves tig (the availability gate's unhappy path).
func tigMissing(name string) (string, error) { return "", errors.New("exec: \"tig\": executable file not found in $PATH") }

// recordExeOverride points launch.ExeOverride at a stub that records whether
// it was consulted and fails if it is — pressing "t" when the launch must
// not happen (tig unavailable, or a branch-less job) must never reach the
// launch path's binary resolution. Mirrors jdilaunch_test's marker pattern:
// the assertion is on the seam being reached at all, not on its return value.
func recordExeOverride(t *testing.T) *bool {
	t.Helper()
	called := false
	old := launch.ExeOverride
	launch.ExeOverride = func() (string, error) { called = true; return "", errors.New("must not be consulted") }
	t.Cleanup(func() { launch.ExeOverride = old })
	return &called
}

// failExeOverride points launch.ExeOverride at a resolution failure and
// restores it after the test.
func failExeOverride(t *testing.T) {
	t.Helper()
	old := launch.ExeOverride
	launch.ExeOverride = func() (string, error) { return "", errors.New("mg: not found") }
	t.Cleanup(func() { launch.ExeOverride = old })
}

// --- footer hint (TASK-4) ----------------------------------------------------

// TestDetailFooterTigHintWhenAvailableAndBranchSet: the "· t tig" hint renders
// only when tig resolves on the host AND the job has a branch — the "if
// available" gate from the brief, mirroring the conditional "e edit" hint.
func TestDetailFooterTigHintWhenAvailableAndBranchSet(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur40_c", "cur40_c", "# Brief: Cur\n\nstatus: open\nid: cur40\nbranch: feature/cur40_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	stubTigLookPath(t, tigResolves)
	d := newDetailView(jobs[0], 80, 24)
	if !strings.Contains(d.renderFooter(), "t tig") {
		t.Errorf("footer missing the tig hint when tig is available and the job has a branch:\n%s", d.renderFooter())
	}
}

// TestDetailFooterTigHintOmittedWhenTigUnavailable: with tig not installed on
// the host, the hint must not render even for a job with a branch.
func TestDetailFooterTigHintOmittedWhenTigUnavailable(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur41_c", "cur41_c", "# Brief: Cur\n\nstatus: open\nid: cur41\nbranch: feature/cur41_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	stubTigLookPath(t, tigMissing)
	d := newDetailView(jobs[0], 80, 24)
	if strings.Contains(d.renderFooter(), "t tig") {
		t.Errorf("footer shows the tig hint when tig is not installed:\n%s", d.renderFooter())
	}
}

// TestDetailFooterTigHintOmittedForBranchlessJob: a job with no branch (the
// non-repo / working-tree fallback) has nothing to browse in tig, so the hint
// must not render even when tig is available.
func TestDetailFooterTigHintOmittedForBranchlessJob(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0004_w")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: W\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}
	if jobs[0].Branch != "" {
		t.Fatalf("test job unexpectedly has a branch: %q", jobs[0].Branch)
	}

	stubTigLookPath(t, tigResolves)
	d := newDetailView(jobs[0], 80, 24)
	if strings.Contains(d.renderFooter(), "t tig") {
		t.Errorf("footer shows the tig hint for a branch-less job:\n%s", d.renderFooter())
	}
}

// --- "t" key (TASK-5) --------------------------------------------------------

// TestTigKeyReportsNotInstalledAndLaunchesNothing: with tig unavailable, "t"
// must report the not-installed status and never reach the launch path at all
// (ExeOverride must not be consulted).
func TestTigKeyReportsNotInstalledAndLaunchesNothing(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur42_c", "cur42_c", "# Brief: Cur\n\nstatus: open\nid: cur42\nbranch: feature/cur42_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	stubTigLookPath(t, tigMissing)
	called := recordExeOverride(t)

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("t"))
	if !strings.Contains(a.detail.status, "tig is not installed") {
		t.Errorf("status = %q, want it to explain tig is not installed", a.detail.status)
	}
	if *called {
		t.Error("ExeOverride was consulted although the launch should have been gated off")
	}
}

// TestTigKeyOnBranchlessJobReportsNoBranch: a job with no branch (the
// non-repo / working-tree fallback) gets the same "no branch known" guard the
// "P" push key uses, and nothing is launched.
func TestTigKeyOnBranchlessJobReportsNoBranch(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0005_v")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: V\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}
	if jobs[0].Branch != "" {
		t.Fatalf("test job unexpectedly has a branch: %q", jobs[0].Branch)
	}

	stubTigLookPath(t, tigResolves)
	called := recordExeOverride(t)

	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("t"))
	if !strings.Contains(a.detail.status, "no branch known for this job") {
		t.Errorf("status = %q, want the no-branch guard", a.detail.status)
	}
	if *called {
		t.Error("ExeOverride was consulted although a branch-less job has nothing to launch")
	}
}

// TestTigKeySurfacesExeResolutionFailure: with tig available and a branch set
// but the mg binary unresolvable, "t" must surface the launch error — proving
// the key routes to launch.Tig (rather than being gated off somewhere).
func TestTigKeySurfacesExeResolutionFailure(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur43_c", "cur43_c", "# Brief: Cur\n\nstatus: open\nid: cur43\nbranch: feature/cur43_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	stubTigLookPath(t, tigResolves)
	failExeOverride(t)

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("t"))
	if !strings.Contains(a.detail.status, "not found") {
		t.Errorf("status = %q, want it to explain the mg binary could not be resolved", a.detail.status)
	}
}

// --- "t" success path (launches tig in a tmux pane) --------------------------

// writeTmuxStub installs a minimal fake `tmux` on PATH with $TMUX set — a
// self-contained copy of the internal/launch package's own stub, per this
// codebase's convention of duplicated test helpers (see detail_test.go's
// gitRun/gitInitRepo/addJobWorktree copies). It records every invocation to
// the returned log file and answers like a minimal tmux: split-window prints
// an incrementing pane id, list-panes prints nothing, kill-pane/select-pane
// are no-ops.
func writeTmuxStub(t *testing.T) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	bin := filepath.Join(dir, "tmux")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> '" + logPath + "'\n" +
		"case \"$1\" in\n" +
		"  list-panes) ;;\n" +
		"  split-window) n=100; [ -f '" + dir + "/counter' ] && read -r n < '" + dir + "/counter'; echo \"%$n\"; echo $((n+1)) > '" + dir + "/counter' ;;\n" +
		"  kill-pane) ;;\n" +
		"  select-pane) ;;\n" +
		"esac\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	return logPath
}

// TestTigKeyLaunchesTigInTmuxPane exercises the full "t" flow on a job in its
// own worktree with a stubbed tmux on PATH + $TMUX: the split-window
// invocation must carry the `diff <name> --tig` inner command (the tig launch
// routed through launch.Tig → launchDetached's tmux path, exactly like agent
// launches), the pane must be tagged, and the status reports where it opened.
func TestTigKeyLaunchesTigInTmuxPane(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/cur44_c", "cur44_c", "# Brief: Cur\n\nstatus: open\nid: cur44\nbranch: feature/cur44_c\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%+v", err, jobs)
	}

	stubTigLookPath(t, tigResolves)
	logPath := writeTmuxStub(t)

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("t"))
	if !strings.Contains(a.detail.status, "→ tig in tmux pane") {
		t.Errorf("status = %q, want it to report the tig launch in a tmux pane", a.detail.status)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("tmux stub call log not written: %v", err)
	}
	if !strings.Contains(string(calls), "diff 'cur44_c' --tig") {
		t.Errorf("tmux split-window not invoked with the tig inner command:\n%s", calls)
	}
	if !strings.Contains(string(calls), "select-pane -t %100 -T manigot") {
		t.Errorf("select-pane tagging of the new pane missing:\n%s", calls)
	}
}
