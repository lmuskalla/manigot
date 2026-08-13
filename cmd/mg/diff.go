package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/project"
)

// runDiff implements `mg diff` — the "see what has changed" view promised by
// docs/GIT_DIFF.md: resolve a job's branch and the project's base branch,
// then show the three-dot range <base>...<branch> (the one notation the doc
// says matters — only what the branch itself did, from the merge-base).
//
// Default output is the doc's "quick eyeball": `git log --oneline` followed
// by `git diff --stat`. `--name-only` swaps the stat for filenames only;
// `--full` prints the complete patch; `--tig` hands the range to the tig TUI
// on the host instead of printing. Job → branch resolution (exact match on
// the id_slug tail segment, then prefix) and the not-found / ambiguous error
// wording are exactly mg done / mg delete's.
func runDiff(args []string, stdout, stderr io.Writer) int {
	// Flags parse in any position relative to the job id (splitFlags keeps
	// the flag tokens and the positional remainder separate, since Go's flag
	// package stops at the first non-flag argument).
	flagArgs, rest := splitFlags(args, nil, map[string]bool{"--full": true, "--name-only": true, "--tig": true})
	fs := flag.NewFlagSet("mg diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	full := fs.Bool("full", false, "print the complete diff")
	nameOnly := fs.Bool("name-only", false, "print filenames only (plus the commit log)")
	tig := fs.Bool("tig", false, "browse the diff in tig (requires tig on the host)")
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// No help case, mirroring mg job: a bare --help is an unknown
			// argument.
			fmt.Fprintln(stderr, "Unknown argument: --help")
			return 1
		}
		fmt.Fprintln(stderr, flagParseError(err))
		return 1
	}
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "Usage: mg-diff <job-id-or-slug> [--full|--name-only|--tig]")
		return 1
	}
	if strings.HasPrefix(rest[0], "-") {
		fmt.Fprintf(stderr, "Unknown argument: %s\n", rest[0])
		return 1
	}
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "Unknown argument: %s\n", rest[1])
		return 1
	}
	jobArg := rest[0]

	root, err := job.FindProjectRoot()
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	if root == "" {
		fmt.Fprintln(stderr, "Error: could not find project root (no docs/ directory found).")
		return 1
	}

	// Resolve the job's branch, exact then prefix — same wording as done/delete.
	branches, err := git.LocalBranches(root)
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	branch, err := resolveJobBranch(branches, jobArg)
	if err != nil {
		cliError(stderr, err)
		return 1
	}

	// Resolve the base branch the same chain mg done/mg delete use: the
	// project's configured baseBranch, falling back to origin/HEAD → main.
	settings, err := project.Load(root)
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	base := settings.BaseBranch
	if base == "" {
		base = git.SymbolicRefHead(root)
	}

	if *tig {
		return runTig(stdout, stderr, root, jobArg, base+"..."+branch)
	}

	if *full {
		patch, err := git.Diff(root, base, branch)
		if err != nil {
			cliError(stderr, err)
			return 1
		}
		if patch == "" {
			fmt.Fprintf(stdout, "No changes on %s relative to %s.\n", branch, base)
			return 0
		}
		fmt.Fprintln(stdout, patch)
		return 0
	}

	// Quick eyeball: the branch's commits, then the files it changed.
	logs, err := git.LogOneline(root, base, branch)
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	var files string
	if *nameOnly {
		files, err = git.DiffNameOnly(root, base, branch)
	} else {
		files, err = git.DiffStat(root, base, branch)
	}
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	if logs == "" && files == "" {
		fmt.Fprintf(stdout, "No changes on %s relative to %s.\n", branch, base)
		return 0
	}
	fmt.Fprintln(stdout, logs)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, files)
	return 0
}

// resolveJobBranch resolves a job id/slug to its branch the way mg done and
// mg delete do: an exact match on the branch's id_slug tail segment first
// (e.g. "feature/sufficient_git-diff" → "sufficient_git-diff"), then a
// unique prefix match on that segment. Not-found and ambiguity use done/delete's
// exact error wording.
func resolveJobBranch(branches []string, jobArg string) (string, error) {
	branch := git.ExactBranchMatch(branches, jobArg)
	if branch != "" {
		return branch, nil
	}
	prefixMatches := git.PrefixBranchMatches(branches, jobArg)
	switch len(prefixMatches) {
	case 0:
		msg := fmt.Sprintf("job '%s' not found among local branches.\nActive job branches:", jobArg)
		for _, b := range branches {
			msg += "\n  " + b
		}
		return "", errors.New(msg)
	case 1:
		return prefixMatches[0], nil
	default:
		return "", fmt.Errorf("job '%s' is ambiguous — matches branches: %s", jobArg, strings.Join(prefixMatches, " "))
	}
}

// tigLookPath is exec.LookPath, split out so tests can stub it (or point PATH
// at a tig-less directory) without requiring tig on the test machine —
// mirrors internal/session's hostLookPath for the mg host CLI check.
var tigLookPath = exec.LookPath

// runTig spawns `tig <range>` interactively on the host, wiring stdin/stdout/
// stderr through so the ncurses UI renders in the caller's terminal and
// Ctrl+C / q reach it directly. tig is a host-side tool (the container image
// ships it, the host may not), so a missing binary is a clear error offering
// the plain-git alternatives — mirroring mg host's CLI-missing error.
func runTig(stdout, stderr io.Writer, root, jobArg, rangeStr string) int {
	tigPath, err := tigLookPath("tig")
	if err != nil {
		cliError(stderr, fmt.Errorf("tig is not installed on the host — 'mg diff --tig' browses the diff in the tig TUI, which runs on the host.\nInstall it (e.g. 'sudo apt install tig'), or use plain 'mg diff %s' / 'mg diff --full %s' for git output.", jobArg, jobArg))
		return 1
	}
	cmd := exec.Command(tigPath, rangeStr)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		cliError(stderr, err)
		return 1
	}
	return 0
}
