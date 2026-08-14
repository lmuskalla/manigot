// Package main is the single manigot host-side entry point: one `mg` binary
// implements every command (session, profiles, setup, agents, job, jobs, done,
// delete, init, tui, jdi, host) in-process. The only bash left in the project
// is scripts/entrypoint.sh, which runs inside the container image.
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
		os.Exit(runAgents(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin), ttyPicker))
	case "job":
		os.Exit(runJob(args[1:], os.Stdout, os.Stderr))
	case "jobs":
		os.Exit(runJobs(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin), ttyPicker))
	case "tui":
		os.Exit(runTUI(args[1:], os.Stdout, os.Stderr))
	case "jdi", "made-man":
		os.Exit(runJDI(args[1:], os.Stdout, os.Stderr))
	case "done":
		os.Exit(runDone(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "delete":
		os.Exit(runDelete(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "diff":
		os.Exit(runDiff(args[1:], os.Stdout, os.Stderr))
	case "init":
		os.Exit(runInit(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	case "host", "wild":
		os.Exit(runHost(args[1:], os.Stdin, os.Stdout, os.Stderr))
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
  mg --agent/-a <name>            Start straight in an agent (e.g. analyst)
  mg --job/-j <id>                Start with a job's brief.md as the prompt
  mg --agent <name> --job <id>    Combine the two

Commands:
  mg profiles [name]              List the profiles (and which is the default),
                                   set the default used by bare `
		mid = `, or pick
                                   one interactively (no name, on a TTY)
  mg setup [name] [--check]       Configure credentials for your subscriptions,
                                   interactively, or report status with --check
  mg agents                       List available agents and pick one to start,
                                   via an interactive picker on a TTY
                                   (type to filter, enter to choose;
                                   thematic alias: mg crew)
  mg init [--profile <name>]      Bootstrap this project for the job workflow
  mg job "<title>" [--type feature|fix|chore]
                                   Create a job directory + branch + worktree
                                   (off main)
  mg jobs                         List jobs and pick one to start a session
                                   in, via an interactive picker on a TTY
                                   (type to filter, enter to choose); the
                                   session launches in the agent appropriate
                                   to the job's stage (analyst/developer/
                                   reviewer)
  mg done <id>                    Archive a finished job (merge + remove worktree)
  mg delete <id>                  Permanently delete a job (worktree + branch, no merge)
  mg diff <id> [--full|--name-only|--tig]
                                   Show what a job's branch changed, three-dot
                                   against the base branch (log + diff --stat
                                   by default; --full for the patch, --name-only
                                   for filenames, --tig to browse in tig)
  mg tui                          Terminal UI for browsing jobs and firing agents
  mg jdi --job/-j <id> [--profile <name>]
                                   Drive a job's analyst -> developer ->
                                   reviewer sequence unattended
                                   (thematic alias: mg made-man)
  mg host                         Run a session directly on the host — no
                                   docker container; the CLI runs as-is, so
                                   the agent can touch the host itself
                                   (thematic alias: mg wild)

  mg -h, --help, help             Show this help

A bare `
		tail = ` works in any project — no setup required. Project context and the
job workflow (mg job/tui/jdi) additionally need a docs/ directory; see
'Per-project setup' in the manigot README to add one.
`
	)
	fmt.Print(head + "`mg`" + mid + "`mg`" + tail)
}
