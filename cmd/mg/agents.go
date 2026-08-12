package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lmuskalla/manigot/internal/agentlist"
	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/home"
	"github.com/lmuskalla/manigot/internal/job"
)

// runAgents implements `mg agents` (thematic alias `mg crew`) — the port of
// scripts/agents.sh with identical output wording. passthrough holds the args
// after the subcommand (e.g. --profile zai), which the session launch re-execs
// the binary with, mirroring the script's `exec run.sh --agent "$CHOSEN" "$@"`.
func runAgents(passthrough []string, r io.Reader, stdout, stderr io.Writer, tty bool) int {
	home := home.Root()
	if home == "" {
		fmt.Fprintln(stderr, "Error: cannot locate the manigot checkout.")
		return 1
	}
	globalDir := filepath.Join(home, "agents")
	if info, err := os.Stat(globalDir); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "Error: no agents/ directory found at %s\n", globalDir)
		return 1
	}

	projectRoot, err := job.FindProjectRoot()
	if err != nil {
		fmt.Fprintf(stderr, "mg agents: %v\n", err)
		return 1
	}
	var projectAgentsDir string
	if projectRoot != "" {
		projectAgentsDir = filepath.Join(projectRoot, "docs", "agents")
	}

	agents, err := agentlist.Discover(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "Available agents:")
	fmt.Fprintln(stdout, "")
	for i, a := range agents {
		fmt.Fprintf(stdout, "  %2d) %-14s %s%s\n", i+1, a.Name, a.Description, agentSource(home, projectAgentsDir, a.Name))
	}
	fmt.Fprintln(stdout, "")

	if !tty {
		fmt.Fprintln(stderr, "Error: mg agents needs an interactive terminal to select an agent.")
		return 1
	}

	choice, err := cli.Select(fmt.Sprintf("Select an agent [1-%d]: ", len(agents)), len(agents), false, r, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "mg agents: %v\n", err)
		return 1
	}
	chosen := agents[choice-1].Name
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "→ Starting a session in @%s...\n", chosen)
	fmt.Fprintln(stdout, "")

	// Re-exec the same binary's session path (strangler stage 0: that still
	// lands in run.sh; once Phase 3 ports the session in-process, this keeps
	// working unchanged).
	launchArgs := append([]string{"--agent", chosen}, passthrough...)
	return reexec(launchArgs, stderr)
}

// agentSource returns the display tag for an agent whose file comes from the
// project: " (project override)" when a global agent of the same name exists,
// " (project)" for a project-only addition, else "" (global).
func agentSource(home, projectAgentsDir, name string) string {
	if projectAgentsDir == "" {
		return ""
	}
	projectFile := filepath.Join(projectAgentsDir, name+".md")
	if !isRegularFile(projectFile) {
		return ""
	}
	if isRegularFile(filepath.Join(home, "agents", name+".md")) {
		return " (project override)"
	}
	return " (project)"
}

// isRegularFile reports whether path exists and is a regular file.
func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
