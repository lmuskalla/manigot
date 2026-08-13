package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/job"
)

// runDelete implements `mg delete` — the port of scripts/delete-job.sh, calling
// job.DeleteJob in-process with cli.Confirm for the script's confirmation.
//
// When the arg doesn't resolve to a job branch, it is tried against the
// project's orphaned worktrees (see job.MatchOrphan): an orphaned worktree has
// no branch and no registration, so `mg delete <name>` is the only way to
// reach it by name. The same confirmation discipline ("This cannot be
// undone.") applies.
func runDelete(args []string, r io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: mg-delete <job-id-or-slug>")
		return 1
	}

	root, err := job.FindProjectRoot()
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	if root == "" {
		fmt.Fprintln(stderr, "Error: could not find project root (no docs/ directory found).")
		return 1
	}

	br := bufio.NewReader(r)
	confirm := func(prompt string) (bool, error) {
		return cli.Confirm(prompt, br, stdout)
	}

	// A live job wins over an orphaned worktree of the same name: DeleteJob
	// resolves by branch first. Only when it reports not-found (ErrJobNotFound)
	// is the arg tried against the orphaned worktrees — an orphan has no
	// branch, so `mg delete <id>` reaching it by its id_slug is the tool's only
	// path to remove it. RemoveOrphans applies the same per-item confirmation.
	_, err = job.DeleteJob(root, args[0], confirm, stdout)
	if errors.Is(err, job.ErrCancelled) {
		// A declined confirmation is the script's `exit 0`, not an error.
		return 0
	}
	if errors.Is(err, job.ErrJobNotFound) {
		if o, ok := job.MatchOrphan(root, args[0]); ok {
			err = job.RemoveOrphans(root, []job.Orphan{o}, confirm, stdout)
			if errors.Is(err, job.ErrCancelled) {
				return 0
			}
			if err != nil {
				cliError(stderr, err)
				return 1
			}
			return 0
		}
	}
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	return 0
}
