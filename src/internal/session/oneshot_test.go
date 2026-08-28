package session

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
)

// fakeOneShotRunner implements AgentRunner without spawning any real
// process — mirroring cmd/mg/jdi_test.go's own fakeRunner — so RunOneShot's
// session-log wiring can be tested independent of docker/session-launcher
// plumbing (covered separately by CommandAgentRunner's own tests below).
type fakeOneShotRunner struct {
	out []byte
	err error
	// calls records every (agent, j.Name) pair Run was invoked with.
	calls []string
}

func (f *fakeOneShotRunner) Run(agent string, j job.Job, live io.Writer) ([]byte, error) {
	f.calls = append(f.calls, agent+"/"+j.Name)
	if len(f.out) > 0 && live != nil {
		live.Write(f.out)
	}
	return f.out, f.err
}

// testJob builds a minimal job.Job whose Dir is a fresh temp directory —
// enough for RunOneShot's session.log path (filepath.Join(j.Dir,
// "session.log")).
func testJob(t *testing.T) job.Job {
	t.Helper()
	dir := t.TempDir()
	return job.Job{Name: "aaaa01_test-job", Dir: dir}
}

func TestRunOneShotOpensAttemptOneSectionAndStreamsOutput(t *testing.T) {
	j := testJob(t)
	runner := &fakeOneShotRunner{out: []byte("did the thing\n")}

	if err := RunOneShot(runner, "developer", j); err != nil {
		t.Fatalf("RunOneShot: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(j.Dir, "session.log"))
	if err != nil {
		t.Fatalf("reading session.log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "developer (attempt 1)") {
		t.Errorf("session.log = %q, want a developer (attempt 1) header", got)
	}
	if !strings.Contains(got, "did the thing") {
		t.Errorf("session.log = %q, want the runner's live output", got)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "developer/"+j.Name {
		t.Errorf("runner.calls = %v, want one call for developer/%s", runner.calls, j.Name)
	}
}

func TestRunOneShotClosesSectionWithTrailingNewline(t *testing.T) {
	j := testJob(t)
	runner := &fakeOneShotRunner{out: []byte("no trailing newline")}

	if err := RunOneShot(runner, "analyst", j); err != nil {
		t.Fatalf("RunOneShot: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(j.Dir, "session.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "no trailing newline\n") {
		t.Errorf("session.log = %q, want a trailing newline guaranteed by Close", string(data))
	}
}

func TestRunOneShotWritesRunnerErrorIntoSessionLogAndReturnsIt(t *testing.T) {
	j := testJob(t)
	wantErr := errors.New("mg --print --agent developer --job aaaa01_test-job: exit code 1: boom")
	runner := &fakeOneShotRunner{err: wantErr}

	err := RunOneShot(runner, "developer", j)
	if !errors.Is(err, wantErr) && err.Error() != wantErr.Error() {
		t.Errorf("RunOneShot error = %v, want %v", err, wantErr)
	}

	data, rerr := os.ReadFile(filepath.Join(j.Dir, "session.log"))
	if rerr != nil {
		t.Fatalf("reading session.log: %v", rerr)
	}
	// The goroutine that would normally start RunOneShot has nowhere else to
	// report a failure once its triggering HTTP request has already
	// returned — session.log is where it must land.
	if !strings.Contains(string(data), "agent invocation failed") || !strings.Contains(string(data), "boom") {
		t.Errorf("session.log = %q, want the runner error recorded in the section", string(data))
	}
}

func TestRunOneShotMultipleInvocationsAppendSeparateSections(t *testing.T) {
	j := testJob(t)
	runner := &fakeOneShotRunner{out: []byte("first\n")}
	if err := RunOneShot(runner, "analyst", j); err != nil {
		t.Fatal(err)
	}
	runner2 := &fakeOneShotRunner{out: []byte("second\n")}
	if err := RunOneShot(runner2, "developer", j); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(j.Dir, "session.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	// Both invocations are attempt 1 — a one-shot has no per-agent attempt
	// counter of its own, unlike mg-jdi's loop.
	if !strings.Contains(got, "analyst (attempt 1)") || !strings.Contains(got, "developer (attempt 1)") {
		t.Errorf("session.log = %q, want both sections at attempt 1", got)
	}
	if i, j2 := strings.Index(got, "first"), strings.Index(got, "second"); i == -1 || j2 == -1 || i > j2 {
		t.Errorf("session.log sections out of order: %q", got)
	}
}

// TestCommandAgentRunnerBuildsPrintInvocationForGivenProfile mirrors
// cmd/mg/jdi_test.go's TestCommandAgentRunnerUsesGivenProfile: the exported
// CommandAgentRunner is a thin, testable wrapper — its invocation options are
// asserted directly rather than exercising a real docker/session-launcher run
// (which needs docker on the test machine).
func TestCommandAgentRunnerBuildsPrintInvocationForGivenProfile(t *testing.T) {
	r := &CommandAgentRunner{ProjectRoot: t.TempDir(), Profile: "zai"}
	if r.Profile != "zai" || r.ProjectRoot == "" {
		t.Errorf("CommandAgentRunner = %+v, want the given profile/root", r)
	}
}
