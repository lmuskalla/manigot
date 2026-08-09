// Package main is the entry point for the safecode TUI.
//
// The safecode TUI is a host-side terminal interface for managing safecode
// jobs and launching agents without remembering command syntax. It runs on the
// user's machine (NOT inside the safecode container), reads a project's
// docs/processes/ directories, and shells out to the host commands sc and
// sc-job. See docs/processes/irw320_tui/ for the full design.
//
// Run from the repository root:
//
//	go run ./tui
package main

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/lmuskalla/safecode/tui/internal/job"
	"codeberg.org/lmuskalla/safecode/tui/internal/resolve"
	"codeberg.org/lmuskalla/safecode/tui/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// version is the TUI version. Overridden at build time with:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "0.1.0-dev"

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "sc-tui %s\n\n", version)
		fmt.Fprintf(os.Stderr, "Terminal UI for managing safecode jobs.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  sc-tui [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Run from anywhere inside a project that has a docs/ directory.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Work out which safecode checkout this binary belongs to and export it, so
	// the host commands can be found via $SAFECODE_HOME/scripts/*.sh even when
	// nothing is installed on $PATH and the wrapper script was bypassed.
	resolve.SeedHome()

	root, err := job.FindProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sc-tui: cannot read working directory: %v\n", err)
		os.Exit(1)
	}
	if root == "" {
		fmt.Fprintln(os.Stderr, "sc-tui: not inside a safecode project (no docs/ directory found in this or any parent).")
		fmt.Fprintln(os.Stderr, "Run from inside a project that has a docs/ directory.")
		os.Exit(1)
	}

	jobs, err := job.Discover(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sc-tui: cannot read jobs: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.NewApp(root, jobs), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sc-tui: %v\n", err)
		os.Exit(1)
	}
}
