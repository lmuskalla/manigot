package main

import (
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
	var jobType, baseBranchOverride string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--type":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "Unknown argument: %s\n", args[i])
				return 1
			}
			jobType = args[i+1]
			i++
		case "--base-branch":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "Unknown argument: %s\n", args[i])
				return 1
			}
			baseBranchOverride = args[i+1]
			i++
		default:
			fmt.Fprintf(stderr, "Unknown argument: %s\n", args[i])
			return 1
		}
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

	if _, err := job.CreateJob(root, title, job.CreateOptions{Type: jobType, BaseBranchOverride: baseBranchOverride}, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
