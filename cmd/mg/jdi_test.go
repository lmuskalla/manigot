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

	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/orchestrate"
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
		Name:   jobName,
		Dir:    jobDir,
		Root:   root,
		ID:     "aaaa01",
		Branch: branch,
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

// TestRunLogsAgentInvoked confirms Run writes an "invoked" event to the log
// immediately before each agent invocation (TASK-2), reusing the same
// attempt number logInvocation's own post-run header uses.
func TestRunLogsAgentInvoked(t *testing.T) {
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

	var log bytes.Buffer
	got := Run(root, j, r, &log, nil)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}
	out := log.String()
	for _, want := range []string{
		"analyst invoked (attempt 1)",
		"developer invoked (attempt 2)",
		"reviewer invoked (attempt 3)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q, got:\n%s", want, out)
		}
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

// TestRunLogsJobFinishedOnNormalStop confirms Run writes a "job finished"
// event to the log at loop exit (TASK-4) once an agent has actually run —
// distinct from the immediate-stop case covered by
// TestRunLogsImmediateStopReason below, which must not repeat the same
// reason text twice in a row.
func TestRunLogsJobFinishedOnNormalStop(t *testing.T) {
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

	var log bytes.Buffer
	got := Run(root, j, r, &log, nil)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}
	out := log.String()
	if !strings.Contains(out, "mg jdi finished: stop-finished") {
		t.Errorf("log missing job-finished event, got:\n%s", out)
	}
	if !strings.Contains(out, got.Reason) {
		t.Errorf("log missing the finish reason %q, got:\n%s", got.Reason, out)
	}
}

// TestRunImmediateStopDoesNotDuplicateReason extends
// TestRunLogsImmediateStopReason: TASK-4's "job finished" event must still
// fire for the stop-before-any-agent-ran case, but must not print the same
// reason text logImmediateStop already printed a second time.
func TestRunImmediateStopDoesNotDuplicateReason(t *testing.T) {
	root, j := initTestRepo(t)
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
	out := log.String()
	if !strings.Contains(out, "mg jdi finished: stop-needs-human") {
		t.Errorf("log missing job-finished event, got:\n%s", out)
	}
	if strings.Count(out, got.Reason) != 1 {
		t.Errorf("reason %q appears %d times in log, want exactly 1 (not duplicated between logImmediateStop and logJobFinished):\n%s", got.Reason, strings.Count(out, got.Reason), out)
	}
}

// assertLogOrder fails the test unless every substring in wantInOrder
// appears in out, each strictly after the previous one — i.e. out reads as
// wantInOrder describes, start to finish.
func assertLogOrder(t *testing.T, out string, wantInOrder []string) {
	t.Helper()
	pos := 0
	for _, want := range wantInOrder {
		idx := strings.Index(out[pos:], want)
		if idx == -1 {
			t.Fatalf("log missing %q after position %d, want it in order %v; full log:\n%s", want, pos, wantInOrder, out)
		}
		pos += idx + len(want)
	}
}

// TestRunFullLogSequenceHappyPath is TASK-6's end-to-end coverage: a full
// mg-jdi run (mimicking main()'s own logStarted call ahead of Run) produces
// the complete started -> invoked -> finished -> ... -> job finished
// sequence the brief asked for, not just each event in isolation.
func TestRunFullLogSequenceHappyPath(t *testing.T) {
	root, j := initTestRepo(t)
	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		switch agent {
		case "analyst":
			writeJobFile(t, j, "tasks.md", "# Tasks\n\nTASK-1: do the thing\n")
			commit(t, root, "[aaaa01] tasks: add breakdown")
			return []byte("Wrote tasks.md with one task.")
		case "developer":
			writeJobFile(t, j, "implementation.md", "# Implementation\n\nTASK-1: did the thing\n")
			commit(t, root, "[aaaa01] TASK-1: do the thing")
			return []byte("Implemented TASK-1, all tests pass.")
		case "reviewer":
			writeJobFile(t, j, "verdict.md", "# Verdict\n\nTASK-1: PASS\n\n## Overall\n\nAPPROVED\n")
			commit(t, root, "[aaaa01] verdict: approved")
			return []byte("Reviewed and approved.")
		}
		return nil
	}}

	var log bytes.Buffer
	logStarted(&log, j.Name, "claude-pro")
	got := Run(root, j, r, &log, nil)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}

	assertLogOrder(t, log.String(), []string{
		"mg jdi started",
		"analyst invoked (attempt 1)",
		"analyst finished (attempt 1)",
		"Wrote tasks.md with one task.",
		"developer invoked (attempt 2)",
		"developer finished (attempt 2)",
		"Implemented TASK-1, all tests pass.",
		"reviewer invoked (attempt 3)",
		"reviewer finished (attempt 3)",
		"Reviewed and approved.",
		"mg jdi finished: stop-finished",
	})
}

// TestRunFullLogSequenceDedupsMatchingOutput is TASK-6's dedup-case
// end-to-end coverage: when an agent's response text is just an echo of the
// file it wrote (TASK-5), the full sequence still reads started -> invoked
// -> finished -> ... -> job finished, but the finished body is the short
// omission note instead of the file's full content repeated in the log.
func TestRunFullLogSequenceDedupsMatchingOutput(t *testing.T) {
	root, j := initTestRepo(t)
	const tasksContent = "# Tasks\n\nTASK-1: do the thing\n"
	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		switch agent {
		case "analyst":
			writeJobFile(t, j, "tasks.md", tasksContent)
			commit(t, root, "[aaaa01] tasks: add breakdown")
			return []byte(tasksContent)
		case "developer":
			writeJobFile(t, j, "implementation.md", "# Implementation\n\nTASK-1: did the thing\n")
			commit(t, root, "[aaaa01] TASK-1: do the thing")
			return []byte("done")
		case "reviewer":
			writeJobFile(t, j, "verdict.md", "# Verdict\n\nTASK-1: PASS\n\n## Overall\n\nAPPROVED\n")
			commit(t, root, "[aaaa01] verdict: approved")
			return []byte("done")
		}
		return nil
	}}

	var log bytes.Buffer
	logStarted(&log, j.Name, "claude-pro")
	got := Run(root, j, r, &log, nil)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}

	out := log.String()
	assertLogOrder(t, out, []string{
		"mg jdi started",
		"analyst invoked (attempt 1)",
		"analyst finished (attempt 1)",
		"(output matches tasks.md, omitted)",
		"developer invoked (attempt 2)",
		"developer finished (attempt 2)",
		"reviewer invoked (attempt 3)",
		"reviewer finished (attempt 3)",
		"mg jdi finished: stop-finished",
	})
	if strings.Contains(out, "TASK-1: do the thing") {
		t.Errorf("log = %q, want tasks.md's content not repeated verbatim (should be omitted)", out)
	}
}

type errRunner struct{}

func (errRunner) Run(agent string, j job.Job) ([]byte, error) {
	return nil, errors.New("boom")
}

// TestCommandAgentRunnerUsesGivenProfile confirms commandAgentRunner builds
// its --print session invocation with the profile it was constructed with
// (TASK-5), rather than the old hardcoded config.ProfileClaudePro pin —
// exercised for a non-default profile so a stale pin would be caught. Since
// the runner now calls the session package in-process (no run.sh exec), the
// pinned profile is asserted on the constructed options, and a full Run
// against the test repo must reach the docker launch (which fails with a
// docker-not-found error here) rather than a profile/auth/resolution error.
func TestCommandAgentRunnerUsesGivenProfile(t *testing.T) {
	root, j := initTestRepo(t)

	r := &commandAgentRunner{projectRoot: root, profile: "zai"}
	opts := r.agentInvocation("developer", j)
	if opts.Profile != "zai" {
		t.Errorf("invocation profile = %q, want zai", opts.Profile)
	}
	if opts.Agent != "developer" || opts.Job != j.Name || !opts.Print {
		t.Errorf("invocation = %+v", opts)
	}
}
