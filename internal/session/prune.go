package session

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// PruneResult reports what a PruneOrphans run found and did.
type PruneResult struct {
	// Removed is how many EXITED manigot-* containers were removed.
	Removed int
	// Running is how many manigot-* containers are currently running —
	// reported, never touched.
	Running int
}

// dockerCommand is the seam PruneOrphans shells out through. A package-level
// var so tests can stub it without requiring docker (or a daemon) on the test
// machine — mirrors cmd/mg/diff.go's tigLookPath pattern.
var dockerCommand = func(args ...string) ([]byte, error) {
	return exec.Command("docker", args...).CombinedOutput()
}

// wrapDockerErr wraps a failed docker invocation, appending docker's own
// captured output (CombinedOutput) when there is any — a down daemon's
// "Cannot connect to the Docker daemon ..." diagnostic would otherwise be
// swallowed and the user would see only "exit status 1".
func wrapDockerErr(cmd string, err error, out []byte) error {
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return fmt.Errorf("docker %s: %w: %s", cmd, err, msg)
	}
	return fmt.Errorf("docker %s: %w", cmd, err)
}

// PruneOrphans removes every EXITED container whose name matches the manigot
// prefix ("manigot-...", as named by BuildDockerInvocation). Sessions normally
// run with --rm, so orphans are purely the residue of abnormal ends — a killed
// client, a killed pane/window, a host/daemon restart, a hung CLI — and
// cleanup therefore only ever touches exited manigot containers. Running
// manigot containers (a killed pane/window can leave an unattended agent that
// is still working) and every foreign container are never touched — the same
// semantics as `docker container prune`, scoped to manigot's own containers.
//
// The removed and running counts are both returned, so callers can report them
// (the explicit `mg prune` command) or stay quiet (the launch path). A
// pruning line is written to diag only when something was actually removed —
// the launch path stays silent on the common nothing-to-prune case.
//
// docker missing or the daemon down is an error; the caller decides how to
// surface it — the launch path warns on stderr and continues, `mg prune`
// exits 1.
func PruneOrphans(diag io.Writer) (PruneResult, error) {
	// The exited manigot containers — the only ones ever removed.
	out, err := dockerCommand("ps", "-aq", "--filter", "name=^/manigot-", "--filter", "status=exited")
	if err != nil {
		return PruneResult{}, wrapDockerErr("ps", err, out)
	}
	exited := strings.Fields(string(out))

	if len(exited) > 0 {
		rmOut, err := dockerCommand(append([]string{"rm"}, exited...)...)
		if err != nil {
			return PruneResult{}, wrapDockerErr("rm", err, rmOut)
		}
	}

	// The running manigot containers — reported, never touched. A plain
	// `docker ps -q` (no -a) lists only running containers.
	running, err := dockerCommand("ps", "-q", "--filter", "name=^/manigot-")
	if err != nil {
		return PruneResult{}, wrapDockerErr("ps", err, running)
	}

	if diag != nil && len(exited) > 0 {
		fmt.Fprintf(diag, "Pruned %d orphaned manigot container(s).\n", len(exited))
	}
	return PruneResult{Removed: len(exited), Running: len(strings.Fields(string(running)))}, nil
}
