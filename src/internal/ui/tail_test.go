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

// stubJqLookPath points launch.JqLookPath at fn and restores it after the
// test — the internal/ui equivalent of the seam internal/launch's own tests
// use (mirroring stubTigLookPath in tig_test.go), so jq availability can be
// controlled without depending on whether the test machine has jq.
func stubJqLookPath(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	old := launch.JqLookPath
	launch.JqLookPath = fn
	t.Cleanup(func() { launch.JqLookPath = old })
}

// jqResolves always resolves jq (the availability gate's happy path).
func jqResolves(name string) (string, error) { return "/usr/bin/jq", nil }

// jqMissing never resolves jq (the availability gate's unhappy path).
func jqMissing(name string) (string, error) {
	return "", errors.New("exec: \"jq\": executable file not found in $PATH")
}

// --- footer hint (TASK-4) ----------------------------------------------------

// TestDetailFooterTailHintShownWhenSessionLogExists: the "· l tail" hint
// renders only when a session.log exists for this job — the file-exists gate
// from the analysis, mirroring the conditional "e edit" and "t tig" hints.
func TestDetailFooterTailHintShownWhenSessionLogExists(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa08_t")

	// No session.log yet: no hint.
	d := newDetailView(j, 80, 24)
	if strings.Contains(d.renderFooter(), "l tail") {
		t.Errorf("footer shows the tail hint with no session.log:\n%s", d.renderFooter())
	}

	// A session.log appears (mg jdi drove the job): the hint shows.
	if err := os.WriteFile(filepath.Join(j.Dir, "session.log"), []byte("=== mg jdi started ===\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d2 := newDetailView(j, 80, 24)
	if !strings.Contains(d2.renderFooter(), "l tail") {
		t.Errorf("footer missing the tail hint when a session.log exists:\n%s", d2.renderFooter())
	}
}

// --- "l" key gate ------------------------------------------------------------

// TestTailKeyReportsNoSessionLogAndLaunchesNothing: with no session.log, "l"
// must report the gate and never reach the launch path at all — the tmux stub
// call log stays completely empty (not even list-panes, which the launch
// path's replace policy runs first), mirroring
// TestTigKeyReportsNotInstalledAndLaunchesNothing's assertion that a
// gated-off launch never spawns anything.
func TestTailKeyReportsNoSessionLogAndLaunchesNothing(t *testing.T) {
	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa09_g")

	logPath := writeTmuxStub(t)

	a := NewApp(root, []job.Job{j})
	a.width, a.height = 80, 24
	a.detail = newDetailView(j, 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("l"))
	if !strings.Contains(a.detail.status, "no mg jdi run has happened for this job yet") {
		t.Errorf("status = %q, want it to explain no session.log exists yet", a.detail.status)
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

// --- "l" success path (tails session.log in a tmux pane) ---------------------

// TestTailKeyLaunchesTailInTmuxPane exercises the full "l" flow with a real
// session.log in the job's dir, jq stubbed as available, and a stubbed tmux
// on PATH + $TMUX: the split-window invocation must carry the jq-piped
// `tail -f '<path>' | jq -R -r 'fromjson? // .'` inner command (routed
// through launch.Tail → launchDetached's tmux path, exactly like tig/agent
// launches), the pane must be tagged, and the status reports where it
// opened. jq is stubbed rather than left to the test machine's actual PATH
// so the exact command asserted here doesn't depend on whether jq happens to
// be installed (see TestTailKeyFallsBackToPlainTailWhenJqMissing for the
// other branch).
func TestTailKeyLaunchesTailInTmuxPane(t *testing.T) {
	stubJqLookPath(t, jqResolves)

	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa10_t")

	sessionLog := filepath.Join(j.Dir, "session.log")
	if err := os.WriteFile(sessionLog, []byte("=== mg jdi started ===\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logPath := writeTmuxStub(t)

	a := NewApp(root, []job.Job{j})
	a.width, a.height = 80, 24
	a.detail = newDetailView(j, 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("l"))
	if !strings.Contains(a.detail.status, "→ tailing session.log in tmux pane") {
		t.Errorf("status = %q, want it to report the tail launch in a tmux pane", a.detail.status)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("tmux stub call log not written: %v", err)
	}
	wantInner := "tail -f '" + sessionLog + `' | jq -R -r 'fromjson? // .'`
	if !strings.Contains(string(calls), wantInner) {
		t.Errorf("tmux split-window not invoked with the jq-piped tail inner command:\n%s", calls)
	}
	if !strings.Contains(string(calls), "select-pane -t %100 -T manigot") {
		t.Errorf("select-pane tagging of the new pane missing:\n%s", calls)
	}
}

// TestTailKeyFallsBackToPlainTailWhenJqMissing mirrors
// TestTailKeyLaunchesTailInTmuxPane with jq stubbed as missing: the
// split-window invocation must carry the plain `tail -f '<path>'` inner
// command, silently falling back instead of erroring.
func TestTailKeyFallsBackToPlainTailWhenJqMissing(t *testing.T) {
	stubJqLookPath(t, jqMissing)

	root := t.TempDir()
	j := discoverOneJob(t, root, "aaaa11_p")

	sessionLog := filepath.Join(j.Dir, "session.log")
	if err := os.WriteFile(sessionLog, []byte("=== mg jdi started ===\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logPath := writeTmuxStub(t)

	a := NewApp(root, []job.Job{j})
	a.width, a.height = 80, 24
	a.detail = newDetailView(j, 80, 24)
	a.state = stateDetail

	a.updateDetail(keyMsg("l"))
	if !strings.Contains(a.detail.status, "→ tailing session.log in tmux pane") {
		t.Errorf("status = %q, want it to report the tail launch in a tmux pane", a.detail.status)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("tmux stub call log not written: %v", err)
	}
	if !strings.Contains(string(calls), "tail -f '"+sessionLog+"'") {
		t.Errorf("tmux split-window not invoked with the tail inner command:\n%s", calls)
	}
	if strings.Contains(string(calls), "jq") {
		t.Errorf("tmux split-window unexpectedly invoked with a jq pipe:\n%s", calls)
	}
}