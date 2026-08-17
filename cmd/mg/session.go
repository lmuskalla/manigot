package main

import (
	"fmt"
	"io"
	"os"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/session"
)

// pruneOrphans is session.PruneOrphans, split out so tests can stub it
// without requiring docker on the test machine — mirrors tigLookPath's
// exec.LookPath split.
var pruneOrphans = session.PruneOrphans

// runSession implements the bare-`mg` session path: parse the session flags
// (session.ParseArgs), resolve the profile and project root, validate
// credentials, build the docker invocation, and run it with stdio wired
// through. The step order (profile → root/--job → auth → build) matches the
// old run.sh, so error precedence is unchanged.
func runSession(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	opts := session.ParseArgs(args)

	info, err := session.ResolveProfile(opts)
	if err != nil {
		cliError(stderr, err)
		return 1
	}

	root, err := session.ResolveRoot(opts)
	if err != nil {
		cliError(stderr, err)
		return 1
	}

	if err := info.CheckAuth(); err != nil {
		cliError(stderr, err)
		return 1
	}

	inv, err := session.BuildDockerInvocation(opts, info, root, cli.IsTerminal(stdin), stderr)
	if err != nil {
		cliError(stderr, err)
		return 1
	}

	// Prune orphaned manigot containers (the residue of abnormal ends — every
	// session runs with --rm, so the only containers left are crash leftovers)
	// before launching, self-healing any residue on the next invocation. The
	// prune is fail-soft: a failure only warns on stderr and never aborts the
	// launch — the session itself is the point, cleanup is best-effort.
	if _, err := pruneOrphans(stderr); err != nil {
		fmt.Fprintf(stderr, "mg: warning: could not prune orphaned containers: %v\n", err)
	}

	return inv.Run(stdin, stdout, stderr)
}
