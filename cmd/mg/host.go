package main

import (
	"io"
	"os"

	"github.com/lmuskalla/manigot/internal/session"
)

// runHost implements the `mg host` session path (thematic alias `mg wild`):
// the same flow as runSession — parse the session flags, resolve the profile
// and project root, validate credentials — but instead of building a docker
// invocation, it builds a direct invocation of the profile's agent CLI running
// on the host itself (session.BuildHostInvocation). Host mode is for work that
// must happen on the host, outside the container; the CLI runs as installed on
// the host, supervised (no auto-approval flags), with the profile's
// credentials in its environment.
func runHost(args []string, stdin *os.File, stdout, stderr io.Writer) int {
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

	inv, err := session.BuildHostInvocation(opts, info, root, stderr)
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	return inv.Run(stdin, stdout, stderr)
}
