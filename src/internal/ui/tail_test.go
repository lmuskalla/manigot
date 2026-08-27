package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
)

// --- footer hint (TASK-4) ----------------------------------------------------

// TestDetailFooterTailHintShownWhenRunLogExists: the "· l tail" hint renders
// only when a run.log exists for this job — the file-exists gate from the
// analysis, mirroring the conditional "e edit" and "t tig" hints.
func TestDetailFooterTailHintShownWhenRunLogExists(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa08_t")

	// No run.log yet: no hint.
	d := newDetailView(j, 80, 24)
	if strings.Contains(d.renderFooter(), "l tail") {
		t.Errorf("footer shows the tail hint with no run.log:\n%s", d.renderFooter())
	}

	// A run.log appears (mg jdi drove the job): the hint shows.
	if err := os.MkdirAll(job.JDIStatusDir(root, j.Name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(job.JDIRunLogPath(root, j.Name), []byte("=== mg jdi started ===\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d2 := newDetailView(j, 80, 24)
	if !strings.Contains(d2.renderFooter(), "l tail") {
		t.Errorf("footer missing the tail hint when a run.log exists:\n%s", d2.renderFooter())
	}
}

// --- "l" key gate ------------------------------------------------------------

// TestTailKeyReportsNoRunLogAndLaunchesNothing: with no run.log, "l" must
// report the gate and never reach the launch path at all — the tmux stub call
// log stays completely empty (not even list-panes, which the launch path's
// replace policy runs first), mirroring TestTigKeyReportsNotInstalledAndLaunchesNothing's
// assertion that a gated-off launch never spawns anything.
func TestTailKeyReportsNoRunLogAndLaunchesNothing(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa09_g")

	logPath := writeTmuxStub(t)

	a := NewApp(root, []job.Job{j})
	a.width, a.height = 80, 24
	a.detail = newDetailView(j, 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("l"))
	if !strings.Contains(a.detail.status, "no mg jdi run has happened for this job yet") {
		t.Errorf("status = %q, want it to explain no run.log exists yet", a.detail.status)
	}
	// The stub's call log only exists once the stub has been invoked at all —
	// a never-reached launch path leaves no file behind, which is exactly the
	// empty log this assertion means.
	calls, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("tmux stub call log not readable: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("launch path reached despite the gate — tmux stub was invoked:\n%s", calls)
	}
}

// --- "l" success path (tails run.log in a tmux pane) -------------------------

// TestTailKeyLaunchesTailInTmuxPane exercises the full "l" flow with a real
// run.log sidecar and a stubbed tmux on PATH + $TMUX: the split-window
// invocation must carry the `tail -f '<path>'` inner command (routed through
// launch.Tail → launchDetached's tmux path, exactly like tig/agent launches),
// the pane must be tagged, and the status reports where it opened.
func TestTailKeyLaunchesTailInTmuxPane(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa10_t")

	if err := os.MkdirAll(job.JDIStatusDir(root, j.Name), 0o755); err != nil {
		t.Fatal(err)
	}
	runLog := job.JDIRunLogPath(root, j.Name)
	if err := os.WriteFile(runLog, []byte("=== mg jdi started ===\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logPath := writeTmuxStub(t)

	a := NewApp(root, []job.Job{j})
	a.width, a.height = 80, 24
	a.detail = newDetailView(j, 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("l"))
	if !strings.Contains(a.detail.status, "→ tailing run.log in tmux pane") {
		t.Errorf("status = %q, want it to report the tail launch in a tmux pane", a.detail.status)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("tmux stub call log not written: %v", err)
	}
	if !strings.Contains(string(calls), "tail -f '"+runLog+"'") {
		t.Errorf("tmux split-window not invoked with the tail inner command:\n%s", calls)
	}
	if !strings.Contains(string(calls), "select-pane -t %100 -T manigot") {
		t.Errorf("select-pane tagging of the new pane missing:\n%s", calls)
	}
}