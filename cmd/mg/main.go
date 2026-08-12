// Package main is the single manigot host-side entry point: one `mg` binary
// implements every command (session, profiles, setup, agents, job, done,
// delete, init, tui, jdi) in-process. The only bash left in the project is
// scripts/entrypoint.sh, which runs inside the container image.
package main

import (
	"fmt"
	"os"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/home"
)

// version is the mg version. Overridden at build time with:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "0.1.0-dev"

func main() {
	// Seed $MANIGOT_HOME so the config/agentlist/init data files resolve to
	// this checkout even when the binary is invoked directly from bin/mg (no
	// wrapper script involved).
	home.Seed()

	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runSession(args, os.Stdin, os.Stdout, os.Stderr))
	}

	switch args[0] {
	case "-h", "--help", "help":
		printHelp()
		os.Exit(0)
	case "profiles":
		os.Exit(runProfiles(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	case "setup":
		os.Exit(runSetup(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	case "agents", "crew":
		os.Exit(runAgents(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	case "job":
		os.Exit(runJob(args[1:], os.Stdout, os.Stderr))
	case "tui":
		os.Exit(runTUI(args[1:], os.Stdout, os.Stderr))
	case "jdi", "made-man":
		os.Exit(runJDI(args[1:], os.Stdout, os.Stderr))
	case "done":
		os.Exit(runDone(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "delete":
		os.Exit(runDelete(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "init":
		os.Exit(runInit(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	default:
		// Bare `mg`, or the session flags (--agent/--job/--tool/--profile/
		// --print with passthrough) — the in-process session launcher.
		os.Exit(runSession(args, os.Stdin, os.Stdout, os.Stderr))
	}
}

func printHelp() {
	// Built with concatenation because the text itself contains backticks
	// (`` bare `mg` ``), which cannot appear inside a raw string literal.
	const (
		head = `mg — isolated agent environment per project (Claude Code or OpenCode, sandboxed in Docker)

Profiles bundle an agent CLI with one of your subscriptions:
  claude-pro    Claude Code, billed to your Claude Pro/Max subscription
  zai           OpenCode, billed to your Z.AI Coding Plan
  opencode-go   OpenCode, billed to the OpenCode Go subscription

Usage:
  mg                              Start a session in the current project
  mg --profile <name>             ...with the given profile (claude-pro|zai|opencode-go)
  mg --agent <name>               Start straight in an agent (e.g. analyst)
  mg --job <id>                   Start with a job's brief.md as the prompt
  mg --agent <name> --job <id>    Combine the two

Commands:
  mg profiles [name]              List the profiles (and which is the default),
                                   set the default used by bare `
		mid = `, or pick
                                   one interactively (no name, on a TTY)
  mg setup [name] [--check]       Configure credentials for your subscriptions,
                                   interactively, or report status with --check
  mg agents                       List available agents and pick one to start
                                   (thematic alias: mg crew)
  mg init [--profile <name>]      Bootstrap this project for the job workflow
  mg job "<title>" [--type feature|fix|chore]
                                   Create a job directory + branch + worktree
                                   (off main)
  mg done <id>                    Archive a finished job (merge + remove worktree)
  mg delete <id>                  Permanently delete a job (worktree + branch, no merge)
  mg tui                          Terminal UI for browsing jobs and firing agents
  mg jdi --job <id> [--profile <name>]
                                   Drive a job's analyst -> developer ->
                                   reviewer sequence unattended
                                   (thematic alias: mg made-man)

  mg -h, --help, help             Show this help

A bare `
		tail = ` works in any project — no setup required. Project context and the
job workflow (mg job/tui/jdi) additionally need a docs/ directory; see
'Per-project setup' in the manigot README to add one.
`
	)
	fmt.Print(head + "`mg`" + mid + "`mg`" + tail)
}
