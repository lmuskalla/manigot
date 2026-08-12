package main

import (
	"fmt"
	"io"
	"os"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/session"
)

// runSession implements the bare-`mg` session path: parse the run.sh flags,
// resolve the profile and project root, validate credentials, build the docker
// invocation, and run it with stdio wired through. The step order matches
// run.sh's (profile → root/--job → auth → build), so error precedence is
// identical. It is the in-process replacement for exec'ing scripts/run.sh
// (which stays on disk only until Phase 5 removes it).
func runSession(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	opts := session.ParseArgs(args)

	info, err := session.ResolveProfile(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	root, err := session.ResolveRoot(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if err := info.CheckAuth(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	inv, err := session.BuildDockerInvocation(opts, info, root, cli.IsTerminal(stdin), stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return inv.Run(stdin, stdout, stderr)
}
