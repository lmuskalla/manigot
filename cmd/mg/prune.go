package main

import (
	"fmt"
	"io"
)

// runPrune implements `mg prune` — the explicit, on-demand counterpart of the
// automatic launch-path cleanup: remove every EXITED manigot-* container
// (sessions run with --rm, so these are the residue of abnormal ends), report
// how many were removed, and report — without removing — how many manigot-*
// containers are still running (an unattended agent may still be working).
// Foreign containers are never touched.
//
// Takes no arguments; docker missing or the daemon down is a clear error and
// exit 1 — unlike the launch path, where a prune failure only warns.
func runPrune(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		fmt.Fprintf(stderr, "Unknown argument: %s\n", a)
		return 1
	}

	res, err := pruneOrphans(stderr)
	if err != nil {
		cliError(stderr, fmt.Errorf("mg prune: %v", err))
		return 1
	}

	if res.Removed > 0 {
		fmt.Fprintf(stdout, "Removed %d orphaned manigot container(s).\n", res.Removed)
	} else {
		fmt.Fprintln(stdout, "Nothing to prune.")
	}
	fmt.Fprintf(stdout, "%d running manigot container(s) left untouched.\n", res.Running)
	return 0
}
