// mg tui — the host-side terminal interface for managing manigot jobs and
// launching agents without remembering command syntax. Runs on the user's
// machine (NOT inside the manigot container), reads a project's docs/jobs/
// directories, and launches sessions in-process. This is the fold of the
// former cmd/tui main into the single mg binary (runTUI).
//
// See docs/jobs/archive/irw320_tui/ for the full design.
package main

import (
	"flag"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lmuskalla/manigot/internal/home"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/markdown"
	"github.com/lmuskalla/manigot/internal/ui"
)

// tuiVersion is the TUI version. Overridden at build time with:
//
//	go build -ldflags "-X main.tuiVersion=1.2.3"
var tuiVersion = "0.1.0-dev"

// runTUI implements `mg tui` — the Bubble Tea program, run in-process. It
// returns the process exit code.
func runTUI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mg tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "mg tui %s\n\n", tuiVersion)
		fmt.Fprintf(stderr, "Terminal UI for managing manigot jobs.\n\n")
		fmt.Fprintf(stderr, "Usage:\n")
		fmt.Fprintf(stderr, "  mg tui [flags]\n\n")
		fmt.Fprintf(stderr, "Run from anywhere inside a project that has a docs/ directory.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2 // the flag package already printed the error + usage
	}

	// Work out which manigot checkout this binary belongs to and export it,
	// so config/agentlist locate the checkout's data files even when nothing
	// is installed on $PATH and no wrapper script was involved.
	home.Seed()

	root, err := job.FindProjectRoot()
	if err != nil {
		fmt.Fprintf(stderr, "mg tui: cannot read working directory: %v\n", err)
		return 1
	}
	if root == "" {
		fmt.Fprintln(stderr, "mg tui: not inside a manigot project (no docs/ directory found in this or any parent).")
		fmt.Fprintln(stderr, "Run from inside a project that has a docs/ directory.")
		return 1
	}

	jobs, err := job.Discover(root)
	if err != nil {
		fmt.Fprintf(stderr, "mg tui: cannot read jobs: %v\n", err)
		return 1
	}

	// Resolve the markdown color style once, up front, and pin it for the rest
	// of the process — see markdown.DetectStyle's doc for exactly why this
	// must happen this way (in short: it reuses Bubble Tea's own cached
	// terminal probe instead of re-querying, which would stall for seconds).
	markdown.SetStyle(markdown.DetectStyle())

	p := tea.NewProgram(ui.NewApp(root, jobs), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(stderr, "mg tui: %v\n", err)
		return 1
	}
	return 0
}
