package session

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestPruneOrphansRemovesExitedOnly pins the core contract: exited manigot-*
// containers are removed, running manigot-* containers and foreign containers
// are never touched, and the returned counts reflect both groups. dockerCommand
// is stubbed (the tigLookPath seam pattern), so no docker is needed on the
// test machine.
func TestPruneOrphansRemovesExitedOnly(t *testing.T) {
	var calls [][]string
	old := dockerCommand
	dockerCommand = func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		joined := strings.Join(args, " ")
		switch {
		case args[0] == "ps" && strings.Contains(joined, "status=exited"):
			// The exited listing: two manigot containers, one foreign.
			return []byte("abc123\ndef456\n"), nil
		case args[0] == "ps":
			// The running listing: one manigot container still up.
			return []byte("ghi789\n"), nil
		case args[0] == "rm":
			return []byte(""), nil
		}
		return nil, fmt.Errorf("unexpected docker call: %v", args)
	}
	t.Cleanup(func() { dockerCommand = old })

	var diag strings.Builder
	res, err := PruneOrphans(&diag)
	if err != nil {
		t.Fatalf("PruneOrphans: %v", err)
	}
	if res.Removed != 2 {
		t.Errorf("Removed = %d, want 2", res.Removed)
	}
	if res.Running != 1 {
		t.Errorf("Running = %d, want 1", res.Running)
	}

	// The exited listing used the manigot-prefix + exited filters.
	if len(calls) != 3 {
		t.Fatalf("docker calls = %d, want 3 (ps exited, ps running, rm):\n%v", len(calls), calls)
	}
	psExited := strings.Join(calls[0], " ")
	for _, want := range []string{"ps", "-aq", "--filter", "name=^/manigot-", "--filter", "status=exited"} {
		if !strings.Contains(psExited, want) {
			t.Errorf("exited listing %q missing %q", psExited, want)
		}
	}

	// rm got exactly the exited ids — the running id is never passed. The
	// call order is ps (exited) → rm → ps (running).
	rm := calls[1]
	if rm[0] != "rm" {
		t.Errorf("second call = %v, want docker rm", rm)
	}
	if got := strings.Join(rm[1:], " "); got != "abc123 def456" {
		t.Errorf("rm args = %q, want exactly the exited ids", got)
	}
	if strings.Contains(strings.Join(rm, " "), "ghi789") {
		t.Errorf("rm touched the running container: %v", rm)
	}

	if !strings.Contains(diag.String(), "Pruned 2 orphaned manigot container(s).") {
		t.Errorf("diag = %q, want the pruned line", diag.String())
	}
}

// TestPruneOrphansNothingToPrune: no exited containers means no rm call at
// all, the removed count is 0, and the running count is still reported.
func TestPruneOrphansNothingToPrune(t *testing.T) {
	var calls [][]string
	old := dockerCommand
	dockerCommand = func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		if args[0] == "ps" && strings.Contains(strings.Join(args, " "), "status=exited") {
			return []byte(""), nil
		}
		return []byte(""), nil
	}
	t.Cleanup(func() { dockerCommand = old })

	var diag strings.Builder
	res, err := PruneOrphans(&diag)
	if err != nil {
		t.Fatalf("PruneOrphans: %v", err)
	}
	if res.Removed != 0 || res.Running != 0 {
		t.Errorf("result = %+v, want zero counts", res)
	}
	for _, c := range calls {
		if c[0] == "rm" {
			t.Errorf("rm called with nothing to prune: %v", calls)
		}
	}
	if diag.Len() != 0 {
		t.Errorf("diag = %q, want empty when nothing was pruned", diag.String())
	}
}

// TestPruneOrphansDockerMissing: a failing docker invocation (missing binary
// or daemon down) is an error the caller decides how to surface — never a
// panic, never a partial removal.
func TestPruneOrphansDockerMissing(t *testing.T) {
	old := dockerCommand
	dockerCommand = func(args ...string) ([]byte, error) {
		return nil, errors.New(`exec: "docker": executable file not found in $PATH`)
	}
	t.Cleanup(func() { dockerCommand = old })

	_, err := PruneOrphans(&strings.Builder{})
	if err == nil {
		t.Fatal("PruneOrphans with docker missing = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "docker ps") {
		t.Errorf("error = %v, want it wrapped as a docker ps failure", err)
	}
}

// TestPruneOrphansDaemonDownIncludesOutput: when docker is installed but the
// daemon is down, docker's own diagnostic (captured by CombinedOutput) must
// be included in the wrapped error — the user sees "Cannot connect to the
// Docker daemon ..." instead of only "exit status 1".
func TestPruneOrphansDaemonDownIncludesOutput(t *testing.T) {
	old := dockerCommand
	dockerCommand = func(args ...string) ([]byte, error) {
		return []byte("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?\n"), errors.New("exit status 1")
	}
	t.Cleanup(func() { dockerCommand = old })

	_, err := PruneOrphans(&strings.Builder{})
	if err == nil {
		t.Fatal("PruneOrphans with daemon down = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
		t.Errorf("error = %v, want docker's own diagnostic included", err)
	}
	if !strings.Contains(err.Error(), "docker ps") {
		t.Errorf("error = %v, want it wrapped as a docker ps failure", err)
	}
}

// TestPruneOrphansRmFailure: a failing docker rm surfaces as an error and
// reports nothing removed.
func TestPruneOrphansRmFailure(t *testing.T) {
	old := dockerCommand
	dockerCommand = func(args ...string) ([]byte, error) {
		if args[0] == "rm" {
			return nil, errors.New("Error response from daemon: boom")
		}
		if strings.Contains(strings.Join(args, " "), "status=exited") {
			return []byte("abc123\n"), nil
		}
		return []byte(""), nil
	}
	t.Cleanup(func() { dockerCommand = old })

	_, err := PruneOrphans(&strings.Builder{})
	if err == nil {
		t.Fatal("PruneOrphans with failing rm = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "docker rm") {
		t.Errorf("error = %v, want it wrapped as a docker rm failure", err)
	}
}
