// Package main is the entry point for the manigot TUI.
//
// The manigot TUI is a host-side terminal interface for managing manigot
// jobs and launching agents without remembering command syntax. It runs on the
// user's machine (NOT inside the manigot container), reads a project's
// docs/jobs/ directories, and shells out to the host commands sc and
// mg-job. See docs/jobs/archive/irw320_tui/ for the full design.
//
// Run from the repository root:
//
//	go run ./tui
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/tui/internal/job"
	"github.com/lmuskalla/manigot/tui/internal/markdown"
	"github.com/lmuskalla/manigot/tui/internal/resolve"
	"github.com/lmuskalla/manigot/tui/internal/ui"
)

// version is the TUI version. Overridden at build time with:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "0.1.0-dev"

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mg-tui %s\n\n", version)
		fmt.Fprintf(os.Stderr, "Terminal UI for managing manigot jobs.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  mg-tui [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Run from anywhere inside a project that has a docs/ directory.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Work out which manigot checkout this binary belongs to and export it, so
	// the host commands can be found via $MANIGOT_HOME/scripts/*.sh even when
	// nothing is installed on $PATH and the wrapper script was bypassed.
	resolve.SeedHome()

	root, err := job.FindProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mg-tui: cannot read working directory: %v\n", err)
		os.Exit(1)
	}
	if root == "" {
		fmt.Fprintln(os.Stderr, "mg-tui: not inside a manigot project (no docs/ directory found in this or any parent).")
		fmt.Fprintln(os.Stderr, "Run from inside a project that has a docs/ directory.")
		os.Exit(1)
	}

	jobs, err := job.Discover(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mg-tui: cannot read jobs: %v\n", err)
		os.Exit(1)
	}

	// Resolve the markdown color style once, up front, and pin it for the rest
	// of the process — see markdown.DetectStyle's doc for exactly why this
	// must happen this way (in short: it reuses Bubble Tea's own cached
	// terminal probe instead of re-querying, which would stall for seconds).
	markdown.SetStyle(markdown.DetectStyle())

	p := tea.NewProgram(ui.NewApp(root, jobs), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mg-tui: %v\n", err)
		os.Exit(1)
	}
}
