package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/lmuskalla/manigot/internal/job"
)

// runJob implements `mg job` — the port of scripts/new-job.sh, calling
// job.CreateJob in-process with the script's argument parsing and error
// wording.
func runJob(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: mg-job \"title of job\" [--type feature|fix|chore] [--base-branch <name>]")
		return 1
	}
	title := args[0]

	fs := flag.NewFlagSet("mg job", flag.ContinueOnError)
	// Discard the flag package's own diagnostics: the script's loop printed
	// exactly one error line, and the branches below print that mapped line.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	jobType := fs.String("type", "", "job type: feature, fix, or chore")
	baseBranchOverride := fs.String("base-branch", "", "branch to cut the job branch from")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// The script's own loop had no help case — a bare --help was
			// just an unknown argument.
			fmt.Fprintln(stderr, "Unknown argument: --help")
			return 1
		}
		// flagParseError reproduces the script's "Unknown argument: <flag>"
		// wording (pinned by tests).
		fmt.Fprintln(stderr, flagParseError(err))
		return 1
	}
	// flag.FlagSet stops at the first non-flag argument and leaves the
	// remainder in fs.Args(); the script's hand-rolled loop rejected any such
	// positional as an unknown argument, so restore that here.
	if rest := fs.Args(); len(rest) > 0 {
		fmt.Fprintf(stderr, "Unknown argument: %s\n", rest[0])
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

	if _, err := job.CreateJob(root, title, job.CreateOptions{Type: *jobType, BaseBranchOverride: *baseBranchOverride}, stdout); err != nil {
		cliError(stderr, err)
		return 1
	}
	return 0
}
