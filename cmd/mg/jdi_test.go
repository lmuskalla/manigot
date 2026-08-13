package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestRunJDIJobShortFlagAccepted pins that `-j` is a recognized alias of
// `--job` in runJDI's own flag set: with no docs/ directory anywhere above
// the working dir, the run must get past flag parsing (an unknown flag would
// exit 2) and fail cleanly at project resolution instead.
func TestRunJDIJobShortFlagAccepted(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut strings.Builder
	if code := runJDI([]string{"-j", "somejob"}, &out, &errOut); code != 1 {
		t.Errorf("-j exit code = %d, want 1 (no docs/ dir); stderr:\n%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "could not find a docs/ directory") {
		t.Errorf("expected the no-docs error, got:\n%s", errOut.String())
	}
}

func TestRunJDIUnknownFlagRejected(t *testing.T) {
	var out, errOut strings.Builder
	if code := runJDI([]string{"-x", "somejob"}, &out, &errOut); code != 2 {
		t.Errorf("unknown flag exit code = %d, want 2", code)
	}
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
// attempt number logInvocation's own post-run header uses. The attempt is
// per agent: every agent's first call is attempt 1, so the happy path reads
// analyst 1 -> developer 1 -> reviewer 1, not a run-wide 1/2/3.
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
		"developer invoked (attempt 1)",
		"reviewer invoked (attempt 1)",
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

// TestRunOneBounceThenApproved covers the bounce path: a rejected review
// sends control back to the developer once, and a second approved review
// ends the run. The log headers assert the per-agent attempt semantics that
// motivated the brief — the developer's second call is that developer's
// attempt 2, NOT attempt 4 as a run-wide counter would produce, and the
// reviewer's first call is attempt 1 — with each invocation's invoked and
// finished headers sharing the same number.
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

	var log bytes.Buffer
	got := Run(root, j, r, &log, nil)
	if got.Kind != orchestrate.StopFinished {
		t.Fatalf("Run.Kind = %v, want StopFinished (reason: %s)", got.Kind, got.Reason)
	}
	wantCalls := []string{"developer", "reviewer", "developer", "reviewer"}
	if strings.Join(r.calls, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("calls = %v, want %v", r.calls, wantCalls)
	}

	// Per-agent attempts on the bounce path: each agent's first call is
	// attempt 1 and the bounced-back developer's second call is attempt 2 —
	// not attempt 4 as the old run-wide i+1 counter produced. The invoked and
	// finished headers of one invocation share the same number.
	assertLogOrder(t, log.String(), []string{
		"developer invoked (attempt 1)",
		"developer finished (attempt 1)",
		"reviewer invoked (attempt 1)",
		"reviewer finished (attempt 1)",
		"developer invoked (attempt 2)",
		"developer finished (attempt 2)",
		"reviewer invoked (attempt 2)",
		"reviewer finished (attempt 2)",
	})
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

// TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL proves the marker detection
// holds at the loop level against the real `opencode run --format json`
// JSONL shape — a stream of typed events whose response prose lives in a
// "text"-event's part.text (see orchestrate.realOpenCodeJSONL) — not just
// against the plain-text / Claude-JSON shapes the other marker tests use.
// It also pins the run.log contract for an opencode-driven run: the log must
// show the extracted prose (ResultText), never the raw JSONL blob.
func TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL(t *testing.T) {
	root, j := initTestRepo(t)

	// A minimal but real `opencode run --format json` output (the 3-line
	// step_start/text/step_finish shape captured live in the 63quv2
	// verification), with the marker inside the text event's part.text.
	const realShape = `{"type":"step_start","part":{"type":"step-start"}}
{"type":"text","part":{"type":"text","text":"Looked at tasks.md.\nNEEDS-HUMAN-INPUT: which auth provider should this use?"}}
{"type":"step_finish","part":{"type":"step-finish","reason":"stop"}}
`

	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		return []byte(realShape)
	}}

	var log bytes.Buffer
	got := Run(root, j, r, &log, nil)
	if got.Kind != orchestrate.StopNeedsHuman {
		t.Fatalf("Run.Kind = %v, want StopNeedsHuman", got.Kind)
	}
	if !strings.Contains(got.Reason, "which auth provider should this use?") {
		t.Errorf("Reason = %q, want it to contain the marker's reason text", got.Reason)
	}
	if len(r.calls) != 1 {
		t.Errorf("calls = %v, want exactly one invocation (stopped immediately)", r.calls)
	}
	out := log.String()
	if strings.Contains(out, `"type":"step_start"`) || strings.Contains(out, `{"type":"text"`) {
		t.Errorf("run.log contains raw JSONL, want the extracted prose (ResultText):\n%s", out)
	}
	if !strings.Contains(out, "which auth provider should this use?") {
		t.Errorf("run.log missing the marker reason prose, got:\n%s", out)
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

// TestRunStallProbeUsesFreshContext pins the stall backstop against a
// regression where the post-agent HEAD probe reused the pre-agent probe
// context: an agent invocation (runner.Run) takes minutes in production, so
// by the time the probe ran the 10s context was long expired and the probe
// returned "" — headAfter never equaled headBefore, noChange was always
// false, and a genuinely stuck agent ran to maxIterations instead of
// stopping after two no-op invocations. The test simulates that by lowering
// jdiProbeTimeout and making the fake runner's Run outlive it (but write
// nothing), then asserting the backstop still fires: it can only if the
// post-agent probe used a fresh context.
func TestRunStallProbeUsesFreshContext(t *testing.T) {
	root, j := initTestRepo(t)

	orig := jdiProbeTimeout
	jdiProbeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { jdiProbeTimeout = orig })

	// The fake agent "runs" longer than the probe timeout and writes
	// nothing at all — exactly the production shape of a stuck agent. 600ms
	// comfortably outlives the 200ms probe timeout, and 200ms leaves the
	// pre-agent probes (three fast, real git execs sharing the loop-top
	// context) plenty of room to complete within it even on a loaded CI
	// machine.
	r := &fakeRunner{t: t, root: root, fn: func(t *testing.T, root string, j job.Job, agent string, call int) []byte {
		time.Sleep(600 * time.Millisecond)
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
		t.Errorf("calls = %v, want exactly two invocations (stall detected on the second, not looped to maxIterations)", r.calls)
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
		"developer invoked (attempt 1)",
		"developer finished (attempt 1)",
		"Implemented TASK-1, all tests pass.",
		"reviewer invoked (attempt 1)",
		"reviewer finished (attempt 1)",
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
		"developer invoked (attempt 1)",
		"developer finished (attempt 1)",
		"reviewer invoked (attempt 1)",
		"reviewer finished (attempt 1)",
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

// notifyRequest is one HTTP request the fake ntfy server captured.
type notifyRequest struct {
	method  string
	path    string
	body    string
	headers http.Header
}

// newNotifyCapture starts an httptest server recording every request it
// receives (method, path, headers, body) and replying with status, so tests
// can assert on exactly what notifyStop/notifyCrashedRun sent.
func newNotifyCapture(t *testing.T, status int) (*httptest.Server, *[]notifyRequest) {
	t.Helper()
	var got []notifyRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, notifyRequest{method: r.Method, path: r.URL.Path, body: string(body), headers: r.Header.Clone()})
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// configureNtfy points the ntfy config (internal/notify's FromConfig, which
// reads config.EnvValue) at the test server by setting the process env the
// .env-less fake checkout falls back to.
func configureNtfy(t *testing.T, srvURL, topic, token string) {
	t.Helper()
	t.Setenv("NTFY_URL", srvURL)
	t.Setenv("NTFY_TOPIC", topic)
	t.Setenv("NTFY_TOKEN", token)
}

func TestNotifyStopFinished(t *testing.T) {
	profileCheckout(t, "")
	srv, got := newNotifyCapture(t, http.StatusOK)
	configureNtfy(t, srv.URL, "my-topic", "")

	var stderr bytes.Buffer
	notifyStop(&stderr, "aaaa01_test-job", LoopResult{Kind: orchestrate.StopFinished, Reason: "verdict.md's Overall verdict is APPROVED"})

	if len(*got) != 1 {
		t.Fatalf("received %d requests, want 1", len(*got))
	}
	req := (*got)[0]
	if req.headers.Get("Tags") != "white_check_mark" {
		t.Errorf("Tags header = %q, want white_check_mark", req.headers.Get("Tags"))
	}
	if req.headers.Get("Priority") != "" {
		t.Errorf("Priority header = %q, want unset (default priority) for a success", req.headers.Get("Priority"))
	}
	if !strings.Contains(req.headers.Get("Title"), "aaaa01_test-job") {
		t.Errorf("Title header = %q, want it to contain the job name", req.headers.Get("Title"))
	}
	if !strings.Contains(req.body, "APPROVED") {
		t.Errorf("body = %q, want it to contain the stop reason", req.body)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestNotifyStopNeedsHuman(t *testing.T) {
	profileCheckout(t, "")
	srv, got := newNotifyCapture(t, http.StatusOK)
	configureNtfy(t, srv.URL, "my-topic", "tok")

	var stderr bytes.Buffer
	notifyStop(&stderr, "aaaa01_test-job", LoopResult{Kind: orchestrate.StopNeedsHuman, Reason: "analyst made no progress on two consecutive runs"})

	if len(*got) != 1 {
		t.Fatalf("received %d requests, want 1", len(*got))
	}
	req := (*got)[0]
	if req.headers.Get("Tags") != "warning" {
		t.Errorf("Tags header = %q, want warning", req.headers.Get("Tags"))
	}
	if req.headers.Get("Priority") != "4" {
		t.Errorf("Priority header = %q, want 4 (high priority) for needs-human", req.headers.Get("Priority"))
	}
	if req.headers.Get("Authorization") != "Bearer tok" {
		t.Errorf("Authorization header = %q, want %q", req.headers.Get("Authorization"), "Bearer tok")
	}
	if !strings.Contains(req.headers.Get("Title"), "needs attention") {
		t.Errorf("Title header = %q, want it to say needs attention", req.headers.Get("Title"))
	}
	if !strings.Contains(req.body, "no progress") {
		t.Errorf("body = %q, want it to contain the stop reason", req.body)
	}
}

func TestNotifyStopUnconfiguredSendsNothing(t *testing.T) {
	profileCheckout(t, "")
	srv, got := newNotifyCapture(t, http.StatusOK)
	configureNtfy(t, srv.URL, "", "") // no NTFY_TOPIC → feature off

	var stderr bytes.Buffer
	notifyStop(&stderr, "aaaa01_test-job", LoopResult{Kind: orchestrate.StopNeedsHuman, Reason: "stalled"})

	if len(*got) != 0 {
		t.Errorf("received %d requests, want 0 (unconfigured must be a strict no-op)", len(*got))
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestNotifyStopSendFailureWarnsButDoesNotAbort(t *testing.T) {
	profileCheckout(t, "")
	srv, _ := newNotifyCapture(t, http.StatusInternalServerError)
	configureNtfy(t, srv.URL, "my-topic", "")

	var stderr bytes.Buffer
	notifyStop(&stderr, "aaaa01_test-job", LoopResult{Kind: orchestrate.StopFinished, Reason: "done"})

	out := stderr.String()
	if !strings.Contains(out, "mg jdi: warning: could not send ntfy notification") {
		t.Errorf("stderr = %q, want a 'could not send ntfy notification' warning", out)
	}
}

// writeStaleRunningSidecar plants an old JDIRunning sidecar — the on-disk
// signature of an mg-jdi run that was killed without ever reporting a stop
// (job.jdiRunningStaleAfter is 30 minutes, so 2 hours is safely stale).
func writeStaleRunningSidecar(t *testing.T, root, jobName string) {
	t.Helper()
	if err := os.MkdirAll(job.JDIStatusDir(root, jobName), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	body := `{"state":"running","agent":"developer","updated":"` + stale + `"}`
	if err := os.WriteFile(job.JDIStatusPath(root, jobName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyCrashedRunNotifiesAndDedups(t *testing.T) {
	profileCheckout(t, "")
	srv, got := newNotifyCapture(t, http.StatusOK)
	configureNtfy(t, srv.URL, "my-topic", "")
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	writeStaleRunningSidecar(t, root, jobName)
	j := job.Job{Name: jobName, Root: root}

	var stderr bytes.Buffer
	notifyCrashedRun(&stderr, root, j)

	if len(*got) != 1 {
		t.Fatalf("received %d requests, want 1 (the crash notification)", len(*got))
	}
	req := (*got)[0]
	if req.headers.Get("Tags") != "warning" {
		t.Errorf("Tags header = %q, want warning", req.headers.Get("Tags"))
	}
	if req.headers.Get("Priority") != "4" {
		t.Errorf("Priority header = %q, want 4 (high priority) for a crash", req.headers.Get("Priority"))
	}
	if !strings.Contains(req.headers.Get("Title"), jobName) {
		t.Errorf("Title header = %q, want it to contain the job name", req.headers.Get("Title"))
	}
	if !strings.Contains(req.body, "crashed or been killed") {
		t.Errorf("body = %q, want it to describe the crash", req.body)
	}
	if !strings.Contains(req.body, "Last agent: developer") {
		t.Errorf("body = %q, want it to name the last agent", req.body)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}

	// The stale sidecar is overwritten with a stopped state, so a second
	// start must not re-notify about the same already-known crash.
	notifyCrashedRun(&stderr, root, j)
	if len(*got) != 1 {
		t.Errorf("received %d requests after dedup, want still 1", len(*got))
	}
	if st, ok := job.ReadJDIStatus(root, jobName); !ok || st.State != job.JDIStoppedNeedsHuman {
		t.Errorf("sidecar after dedup = %+v (ok=%v), want stopped:needs-human", st, ok)
	}
}

func TestNotifyCrashedRunFreshRunningDoesNotNotify(t *testing.T) {
	profileCheckout(t, "")
	srv, got := newNotifyCapture(t, http.StatusOK)
	configureNtfy(t, srv.URL, "my-topic", "")
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	// A fresh running status means a live mg-jdi run — must not be reported
	// as a crash.
	if err := job.WriteJDIStatus(root, jobName, job.JDIRunning, "analyst"); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	notifyCrashedRun(&stderr, root, job.Job{Name: jobName, Root: root})

	if len(*got) != 0 {
		t.Errorf("received %d requests, want 0 (a live run is not a crash)", len(*got))
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestNotifyCrashedRunStoppedStatusDoesNotNotify(t *testing.T) {
	profileCheckout(t, "")
	srv, got := newNotifyCapture(t, http.StatusOK)
	configureNtfy(t, srv.URL, "my-topic", "")
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	if err := job.WriteJDIStatus(root, jobName, job.JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	notifyCrashedRun(&stderr, root, job.Job{Name: jobName, Root: root})

	if len(*got) != 0 {
		t.Errorf("received %d requests, want 0 (a stopped run is not a crash)", len(*got))
	}
}

func TestNotifyCrashedRunUnconfiguredLeavesSidecarUntouched(t *testing.T) {
	profileCheckout(t, "")
	srv, got := newNotifyCapture(t, http.StatusOK)
	configureNtfy(t, srv.URL, "", "") // no NTFY_TOPIC → feature off
	root := t.TempDir()
	const jobName = "aaaa01_test-job"
	writeStaleRunningSidecar(t, root, jobName)

	var stderr bytes.Buffer
	notifyCrashedRun(&stderr, root, job.Job{Name: jobName, Root: root})

	if len(*got) != 0 {
		t.Errorf("received %d requests, want 0 (unconfigured must be a strict no-op)", len(*got))
	}
	// Unconfigured behavior must be byte-for-byte identical: the stale sidecar
	// is left exactly as it was (ReadJDIStatus still degrades it away).
	if _, ok := job.StaleRunningJDI(root, jobName); !ok {
		t.Error("sidecar was modified despite ntfy being unconfigured — unconfigured behavior must be unchanged")
	}
}
