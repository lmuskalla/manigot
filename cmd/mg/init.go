package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/home"
)

// prompterInstruction is the exact instruction text init.sh handed to the
// @prompter agent.
const prompterInstruction = "Read this project at /workspace — its directory structure, README, config files, package manifests, build/test commands, and key source files — then rewrite /workspace/docs/AGENTS.md as a clear, specific project context. Keep the existing section structure (the title, the brief description, and the ## Stack, ## Architecture, ## Commands, ## Hard rules headings) and fill each one in with concrete, accurate details about THIS project, replacing the placeholder text. Write it tool-neutral (for \"the agent\", not one vendor) and keep it concise. Do not invent things you cannot verify from the project; where the project doesn't dictate a value, leave a short explicit placeholder rather than guessing."

// runInit implements `mg init` — the port of scripts/init.sh with identical
// output wording. r is the interactive input (used only when tty).
func runInit(args []string, r io.Reader, stdout, stderr io.Writer, tty bool) int {
	home := home.Root()
	if home == "" {
		fmt.Fprintln(stderr, "Error: cannot locate the manigot checkout.")
		return 1
	}
	templateDir := filepath.Join(home, "project-template", "docs")
	settingsTemplate := filepath.Join(home, "project-template", ".manigot", "manigot.json")
	if info, err := os.Stat(templateDir); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "Error: project template not found at %s.\n", templateDir)
		return 1
	}
	for _, f := range []string{"AGENTS.md", "CLAUDE.md"} {
		if info, err := os.Stat(filepath.Join(templateDir, f)); err != nil || info.IsDir() {
			fmt.Fprintf(stderr, "Error: template file %s is missing.\n", filepath.Join(templateDir, f))
			return 1
		}
	}
	if info, err := os.Stat(settingsTemplate); err != nil || info.IsDir() {
		fmt.Fprintf(stderr, "Error: settings template %s is missing.\n", settingsTemplate)
		return 1
	}

	// --profile is canonical; --tool is a legacy alias mapped to the matching
	// profile (claude-code → claude-pro, opencode → zai).
	fs := flag.NewFlagSet("mg init", flag.ContinueOnError)
	// Discard the flag package's own diagnostics: the script's loop printed
	// exactly one error line, and the branches below print that mapped line.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	toolArg := fs.String("tool", "", "")
	profileArg := fs.String("profile", "", "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// The script's own loop had no help case — a bare --help was
			// just an unknown argument.
			fmt.Fprintln(stderr, "Unknown argument: --help")
			return 1
		}
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

	profile := ""
	switch {
	case *profileArg != "":
		switch *profileArg {
		case config.ProfileClaudePro, config.ProfileZAI, config.ProfileOpenCodeGo, config.ProfileOpenCodeZen:
			profile = *profileArg
		default:
			fmt.Fprintf(stderr, "Error: --profile must be 'claude-pro', 'zai', 'opencode-go' or 'opencode-zen' (got '%s').\n", *profileArg)
			return 1
		}
	case *toolArg != "":
		switch *toolArg {
		case "claude-code":
			profile = config.ProfileClaudePro
		case "opencode":
			profile = config.ProfileZAI
		default:
			fmt.Fprintf(stderr, "Error: --tool must be 'claude-code' or 'opencode' (got '%s').\n", *toolArg)
			return 1
		}
	}

	// Target dir: git top-level when in a repo, else $PWD — NOT the docs
	// walk-up, since init is what creates docs/.
	target, terr := git.RevParseToplevel(".")
	if terr != nil {
		// Not in a repo (or git missing) — fall back to $PWD, the same
		// "empty means not a repo" degrade init.sh's gitToplevel had.
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		target = cwd
	}
	target = filepath.Clean(target)
	docsDir := filepath.Join(target, "docs")

	if info, err := os.Stat(docsDir); err == nil && info.IsDir() {
		fmt.Fprintf(stdout, "  Docs     : %s already exists — skipping template copy.\n", docsDir)
	} else {
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		if err := copyFile(filepath.Join(templateDir, "AGENTS.md"), filepath.Join(docsDir, "AGENTS.md")); err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		if err := copyFile(filepath.Join(templateDir, "CLAUDE.md"), filepath.Join(docsDir, "CLAUDE.md")); err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		if err := os.MkdirAll(filepath.Join(docsDir, "jobs"), 0o755); err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		if err := os.MkdirAll(filepath.Join(target, ".manigot"), 0o755); err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		if err := copyFile(settingsTemplate, filepath.Join(target, ".manigot", "manigot.json")); err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "  Created  : %s/AGENTS.md\n", docsDir)
		fmt.Fprintf(stdout, "            %s/CLAUDE.md\n", docsDir)
		fmt.Fprintf(stdout, "            %s/jobs/ (empty)\n", docsDir)
		fmt.Fprintf(stdout, "            %s/.manigot/manigot.json (baseBranch: main)\n", target)
	}

	// @prompter hand-off — default no; non-TTY stdin skips with a note.
	if !tty {
		fmt.Fprintln(stdout, "  Skipping @prompter offer (stdin is not a terminal).")
	} else {
		yes, err := cli.Confirm("Generate a project prompt with @prompter? [y/N] ", r, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "mg init: %v\n", err)
			return 1
		}
		if yes {
			fmt.Fprintln(stdout, "  Launching @prompter to write your project context…")
			launchArgs := []string{"--agent", "prompter", "--prompt", prompterInstruction}
			if profile != "" {
				launchArgs = append(launchArgs, "--profile", profile)
			}
			return reexec(launchArgs, stderr)
		}
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "✓ Project initialized for manigot.")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  Next steps:")
	fmt.Fprintln(stdout, "  1. Edit your project context:")
	fmt.Fprintf(stdout, "     %s/AGENTS.md\n", docsDir)
	fmt.Fprintln(stdout, "     (or re-run 'mg init' and answer 'y' to have @prompter draft it)")
	fmt.Fprintln(stdout, "  2. Create your first job:")
	fmt.Fprintln(stdout, `     mg job "title of your first job"`)
	return 0
}

// copyFile copies a file's contents and mode to dst (creating parents assumed
// to exist).
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// reexec runs the current binary with the given args — the launch path for
// the @prompter hand-off — and exits with its code.
func reexec(args []string, stderr io.Writer) int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "mg: %v\n", err)
		return 1
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "mg: %v\n", err)
		return 1
	}
	return 0
}
