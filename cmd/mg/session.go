package main

import (
	"io"
	"os"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/session"
)

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
	return inv.Run(stdin, stdout, stderr)
}
