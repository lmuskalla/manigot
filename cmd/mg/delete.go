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
func runDelete(args []string, r io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: mg-delete <job-id-or-slug>")
		return 1
	}

	root, err := job.FindProjectRoot()
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	_, err = job.DeleteJob(root, args[0], confirm, stdout)
	if errors.Is(err, job.ErrCancelled) {
		// A declined confirmation is the script's `exit 0`, not an error.
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
