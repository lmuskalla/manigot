package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/git"
	"github.com/lmuskalla/manigot/tui/internal/job"
	"github.com/lmuskalla/manigot/tui/internal/orchestrate"
)

// runGit runs git -C dir args, failing the test on any error — mirrors
// tui/internal/git's own test helper, duplicated here since it is
// unexported in that package.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// initTestRepo creates a fresh git repo at a temp dir with a job scaffold
// (brief.md only — tasks/implementation/verdict are written by the fake
// runner as the loop progresses) on its own job branch, and returns the
// job.Job ready to hand to Run.
func initTestRepo(t *testing.T) (root string, j job.Job) {
	t.Helper()
	root = t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "t@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")

	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README")
	runGit(t, root, "commit", "-q", "-m", "init")

	const jobName = "aaaa01_test-job"
	const branch = "feature/aaaa01_test-job"
	jobDir := filepath.Join(root, "docs", "jobs", jobName)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "# Brief: test job\n\nstatus: open\ntype: feature\nid: aaaa01\nbranch: " + branch + "\n\n" +
		"## What\n\nsomething substantial enough to count as written\nand a second line so isWritten's rule is satisfied\n"
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "checkout", "-q", "-b", branch)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "[aaaa01] scaffold job")

	j = job.Job{
		Name:            jobName,
		Dir:             jobDir,
		Root:            root,
		ID:              "aaaa01",
		Branch:          branch,
		OnCurrentBranch: true,
	}
	return root, j
}

// writeJobFile writes content to one of the job's four files.
func writeJobFile(t *testing.T, j job.Job, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(j.Dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, root, msg string) {
	t.Helper()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", msg)
}

// fakeRunner implements AgentRunner by calling fn, which simulates an agent
// invocation's effect on disk/git — the same effect a real one would have
// via its own file writes and commits — and returns canned output.
type fakeRunner struct {
	root  string
	calls []string
	fn    func(t *testing.T, root string, j job.Job, agent string, call int) []byte
	t     *testing.T
}

func (f *fakeRunner) Run(agent string, j job.Job) ([]byte, error) {
	f.calls = append(f.calls, agent)
	return f.fn(f.t, f.root, j, agent, len(f.calls)), nil
}

func TestRunHappyPath(t *testing.T) {
	root, j := initTestRepo(t)
	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		switch agent {
		case "analyst":
			writeJobFile(t, j, "tasks.md", "# Tasks\n\nTASK-1: do the thing\n")
			commit(t, root, "[aaaa01] tasks: add breakdown")
		case "developer":
			writeJobFile(t, j, "implementation.md", "# Implementation\n\nTASK-1: did the thing\n")
			commit(t, root, "[aaaa01] TASK-1: do the thing")
		case "reviewer":
			writeJobFile(t, j, "verdict.md", "# Verdict\n\nTASK-1: PASS\n\n## Overall\n\nAPPROVED\n")
			commit(t, root, "[aaaa01] verdict: approved")
		}
		return []byte("ok")
	}}

	got := Run(root, j, r, &bytes.Buffer{}, nil)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}
	wantCalls := []string{"analyst", "developer", "reviewer"}
	if strings.Join(r.calls, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("calls = %v, want %v", r.calls, wantCalls)
	}
}

// TestRunReportsStatus exercises Run's StatusFunc callback (TASK-8): a
// job.JDIRunning report before each invocation naming that invocation's
// agent, and a final job.JDIStoppedFinished report naming the last agent
// that ran.
func TestRunReportsStatus(t *testing.T) {
	root, j := initTestRepo(t)
	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		switch agent {
		case "analyst":
			writeJobFile(t, j, "tasks.md", "# Tasks\n\nTASK-1: do the thing\n")
			commit(t, root, "[aaaa01] tasks: add breakdown")
		case "developer":
			writeJobFile(t, j, "implementation.md", "# Implementation\n\nTASK-1: did the thing\n")
			commit(t, root, "[aaaa01] TASK-1: do the thing")
		case "reviewer":
			writeJobFile(t, j, "verdict.md", "# Verdict\n\nTASK-1: PASS\n\n## Overall\n\nAPPROVED\n")
			commit(t, root, "[aaaa01] verdict: approved")
		}
		return []byte("ok")
	}}

	type report struct {
		state job.JDIState
		agent string
	}
	var reports []report
	status := func(state job.JDIState, agent string) {
		reports = append(reports, report{state, agent})
	}

	got := Run(root, j, r, &bytes.Buffer{}, status)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}

	want := []report{
		{job.JDIRunning, "analyst"},
		{job.JDIRunning, "developer"},
		{job.JDIRunning, "reviewer"},
		{job.JDIStoppedFinished, "reviewer"},
	}
	if len(reports) != len(want) {
		t.Fatalf("reports = %+v, want %+v", reports, want)
	}
	for i, w := range want {
		if reports[i] != w {
			t.Errorf("reports[%d] = %+v, want %+v", i, reports[i], w)
		}
	}
}

func TestRunOneBounceThenApproved(t *testing.T) {
	root, j := initTestRepo(t)
	writeJobFile(t, j, "tasks.md", "# Tasks\n\nTASK-1: do the thing\n")
	commit(t, root, "[aaaa01] tasks: add breakdown")

	reviewCalls := 0
	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		switch agent {
		case "developer":
			writeJobFile(t, j, "implementation.md", fmt.Sprintf("# Implementation\n\nTASK-1: attempt %d\n", call))
			commit(t, root, fmt.Sprintf("[aaaa01] TASK-1: attempt %d", call))
		case "reviewer":
			reviewCalls++
			if reviewCalls == 1 {
				writeJobFile(t, j, "verdict.md", "# Verdict\n\nTASK-1: FAIL\n\n## Overall\n\nNEEDS WORK\n")
				commit(t, root, "[aaaa01] verdict: needs work")
			} else {
				writeJobFile(t, j, "verdict.md", "# Verdict\n\nTASK-1: PASS\n\n## Overall\n\nAPPROVED\n")
				commit(t, root, "[aaaa01] verdict: approved")
			}
		}
		return []byte("ok")
	}}

	got := Run(root, j, r, &bytes.Buffer{}, nil)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}
	wantCalls := []string{"developer", "reviewer", "developer", "reviewer"}
	if strings.Join(r.calls, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("calls = %v, want %v", r.calls, wantCalls)
	}
}

func TestRunStopsAfterOneBounceExhausted(t *testing.T) {
	root, j := initTestRepo(t)
	writeJobFile(t, j, "tasks.md", "# Tasks\n\nTASK-1: do the thing\n")
	commit(t, root, "[aaaa01] tasks: add breakdown")

	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		switch agent {
		case "developer":
			writeJobFile(t, j, "implementation.md", fmt.Sprintf("# Implementation\n\nTASK-1: attempt %d\n", call))
			commit(t, root, fmt.Sprintf("[aaaa01] TASK-1: attempt %d", call))
		case "reviewer":
			writeJobFile(t, j, "verdict.md", fmt.Sprintf("# Verdict\n\nTASK-1: FAIL (attempt %d)\n\n## Overall\n\nREJECTED\n", call))
			commit(t, root, fmt.Sprintf("[aaaa01] verdict: rejected (attempt %d)", call))
		}
		return []byte("ok")
	}}

	got := Run(root, j, r, &bytes.Buffer{}, nil)
	if got.Kind != orchestrate.StopNeedsHuman {
		t.Fatalf("Run.Kind = %v, want StopNeedsHuman (reason: %s)", got.Kind, got.Reason)
	}
	// developer, reviewer (round 1, rejected), developer (the one allowed
	// bounce), reviewer (round 2, rejected again) — then stop, no third
	// developer bounce.
	wantCalls := []string{"developer", "reviewer", "developer", "reviewer"}
	if strings.Join(r.calls, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("calls = %v, want %v", r.calls, wantCalls)
	}
	count, err := git.CountVerdictCommits(root, j.Branch, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("verdict commit count = %d, want 2", count)
	}
}

func TestRunStopsOnNeedsHumanMarker(t *testing.T) {
	root, j := initTestRepo(t)

	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		return []byte("I looked at the brief.\nNEEDS-HUMAN-INPUT: which auth provider should this use?\n")
	}}

	got := Run(root, j, r, &bytes.Buffer{}, nil)
	if got.Kind != orchestrate.StopNeedsHuman {
		t.Fatalf("Run.Kind = %v, want StopNeedsHuman", got.Kind)
	}
	if !strings.Contains(got.Reason, "which auth provider should this use?") {
		t.Errorf("Reason = %q, want it to contain the marker's reason text", got.Reason)
	}
	if len(r.calls) != 1 {
		t.Errorf("calls = %v, want exactly one invocation (stopped immediately)", r.calls)
	}
}

func TestRunStopsOnStall(t *testing.T) {
	root, j := initTestRepo(t)

	// analyst "runs" but writes/commits nothing at all — Stage() and HEAD
	// stay unchanged, so the second consecutive analyst invocation must
	// trip the stall backstop rather than loop forever.
	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		return []byte("thinking out loud, not sure what to do")
	}}

	got := Run(root, j, r, &bytes.Buffer{}, nil)
	if got.Kind != orchestrate.StopNeedsHuman {
		t.Fatalf("Run.Kind = %v, want StopNeedsHuman", got.Kind)
	}
	if !strings.Contains(got.Reason, "no progress") {
		t.Errorf("Reason = %q, want it to mention no progress", got.Reason)
	}
	if len(r.calls) != 2 {
		t.Errorf("calls = %v, want exactly two invocations (stall detected on the second)", r.calls)
	}
}

func TestRunStopsOnRunnerError(t *testing.T) {
	root, j := initTestRepo(t)
	writeJobFile(t, j, "tasks.md", "# Tasks\n\nTASK-1: do the thing\n")
	commit(t, root, "[aaaa01] tasks: add breakdown")

	r := &errRunner{}
	got := Run(root, j, r, &bytes.Buffer{}, nil)
	if got.Kind != orchestrate.StopNeedsHuman {
		t.Fatalf("Run.Kind = %v, want StopNeedsHuman", got.Kind)
	}
	if !strings.Contains(got.Reason, "boom") {
		t.Errorf("Reason = %q, want it to mention the runner error", got.Reason)
	}
}

// TestRunLogsImmediateStopReason confirms an immediate stop — Next()
// returning a Stop* decision before any agent has ever run, e.g. a
// genuinely-unwritten brief.md — writes its Reason to the log, not just to
// mg jdi's own return value. Before this fix, this case left the log at 0
// bytes, which read as "mg jdi didn't do anything at all" rather than "mg
// jdi correctly stopped immediately, here's why".
func TestRunLogsImmediateStopReason(t *testing.T) {
	root, j := initTestRepo(t)
	// Overwrite brief.md with the genuinely-unwritten scaffold shape (no
	// substantive content at all) so Stage() reports StageDefine and Next
	// stops before ever invoking an agent.
	writeJobFile(t, j, "brief.md", "# Brief: test job\n\nstatus: open\ntype: feature\nid: aaaa01\n\n"+
		"## What\n\n<!-- placeholder -->\n")
	commit(t, root, "[aaaa01] brief: reset to scaffold")

	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		t.Fatal("no agent should have been invoked for an immediate stop")
		return nil
	}}

	var log bytes.Buffer
	got := Run(root, j, r, &log, nil)
	if got.Kind != orchestrate.StopNeedsHuman {
		t.Fatalf("Run.Kind = %v, want StopNeedsHuman (reason: %s)", got.Kind, got.Reason)
	}
	if len(r.calls) != 0 {
		t.Fatalf("calls = %v, want no invocations (immediate stop)", r.calls)
	}
	if log.Len() == 0 {
		t.Fatal("run.log is empty, want it to contain the immediate-stop reason")
	}
	if !strings.Contains(log.String(), got.Reason) {
		t.Errorf("log = %q, want it to contain the stop reason %q", log.String(), got.Reason)
	}
}

type errRunner struct{}

func (errRunner) Run(agent string, j job.Job) ([]byte, error) {
	return nil, errors.New("boom")
}
