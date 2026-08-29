// mg serve-projects — manage the projects registered for `mg serve` (the
// daemon's project registry, <manigot checkout>/config/serve.json) from the
// CLI: list them, add a project root, remove one by name. Nobody should have
// to remember the JSON shape by hand — this is `mg profiles` for serve.json.
//
// Writes go through internal/serve's AddRegistryEntry/RemoveRegistryEntry,
// which reuse LoadRegistry's full validation, so the CLI can never produce a
// registry the daemon would refuse to start on. The daemon reads the
// registry once at startup, so every mutating subcommand reminds that a
// running mg serve picks the change up on its next start.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lmuskalla/manigot/internal/serve"
)

const serveProjectsHelp = `mg serve-projects [list|add|rm] [--registry <path>]

Manages the projects registered for mg serve — the daemon's project registry
(<manigot checkout>/config/serve.json by default; override with --registry):

  mg serve-projects                     list the registered projects
  mg serve-projects add [path] [name]   register a project root — the path
                                         defaults to the current directory,
                                         the name to the path's base name
  mg serve-projects rm <name>           unregister the project named <name>

A name must be a single URL-safe path segment ([A-Za-z0-9._-]) and unique
across the registry; a path must be an existing directory. Registrations are
read once at daemon startup — restart a running mg serve to pick up changes.
`

// runServeProjects implements `mg serve-projects`.
func runServeProjects(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mg serve-projects", flag.ContinueOnError)
	fs.SetOutput(stderr)
	registry := fs.String("registry", "", "path to the project registry config (default <checkout>/config/serve.json)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "%s\nFlags:\n", serveProjectsHelp)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2 // the flag package already printed the error + usage
	}

	rest := fs.Args()
	if len(rest) > 0 && rest[0] == "help" {
		fmt.Fprint(stdout, serveProjectsHelp)
		return 0
	}

	// Same registry-path resolution as mg serve: --registry, else the
	// checkout's config/serve.json.
	registryPath := *registry
	if registryPath == "" {
		registryPath = serve.DefaultRegistryPath()
		if registryPath == "" {
			cliError(stderr, serve.ErrNoRegistryPath)
			return 1
		}
	}

	if len(rest) == 0 {
		return serveProjectsList(registryPath, stdout, stderr)
	}
	switch rest[0] {
	case "list":
		if len(rest) > 1 {
			fmt.Fprintln(stderr, "Error: mg serve-projects list takes no arguments.")
			return 1
		}
		return serveProjectsList(registryPath, stdout, stderr)
	case "add":
		return serveProjectsAdd(registryPath, rest[1:], stdout, stderr)
	case "rm", "remove":
		return serveProjectsRm(registryPath, rest[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown subcommand: %s\n", rest[0])
		fmt.Fprintln(stderr, "Usage: mg serve-projects [list|add|rm]")
		return 1
	}
}

// serveProjectsList implements `mg serve-projects` / `mg serve-projects list`
// — the registered projects in config-file order, plus the same missing-docs
// warnings the daemon prints at startup.
func serveProjectsList(registryPath string, stdout, stderr io.Writer) int {
	reg, err := serve.LoadRegistry(registryPath)
	if err != nil {
		cliError(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "Registry: %s\n\n", registryPath)
	entries := reg.Entries()
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "No projects registered.")
		fmt.Fprintln(stdout, "Add one with: mg serve-projects add [path] [name]")
		return 0
	}
	fmt.Fprintf(stdout, "  %-20s %s\n", "name", "path")
	fmt.Fprintf(stdout, "  %-20s %s\n", "----", "----")
	for _, e := range entries {
		fmt.Fprintf(stdout, "  %-20s %s\n", e.Name, e.Path)
	}
	reg.WarnMissingDocs(stderr)
	return 0
}

// serveProjectsAdd implements `mg serve-projects add [path] [name]` —
// registers a project root, defaulting the path to the current directory and
// the name to the (absolute) path's base name. The store applies the
// daemon's full validation; the warning mirrors the daemon's missing-docs
// startup warning for the entry just added.
func serveProjectsAdd(registryPath string, args []string, stdout, stderr io.Writer) int {
	if len(args) > 2 {
		fmt.Fprintln(stderr, "Error: mg serve-projects add takes at most a path and a name.")
		fmt.Fprintln(stderr, "Usage: mg serve-projects add [path] [name]")
		return 1
	}

	root := ""
	if len(args) >= 1 {
		root = args[0]
	}
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			cliError(stderr, fmt.Errorf("mg serve-projects add: determine the current directory: %w", err))
			return 1
		}
		root = wd
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		cliError(stderr, fmt.Errorf("mg serve-projects add: resolve %q: %w", root, err))
		return 1
	}
	abs = filepath.Clean(abs)

	name := ""
	if len(args) == 2 {
		name = args[1]
	}
	if name == "" {
		name = filepath.Base(abs)
	}

	entry, err := serve.AddRegistryEntry(registryPath, name, abs)
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Registered '%s' -> %s (registry: %s)\n", entry.Name, entry.Path, registryPath)
	if info, err := os.Stat(filepath.Join(entry.Path, "docs")); err != nil || !info.IsDir() {
		fmt.Fprintf(stdout, "Warning: %s has no docs/ directory — mg serve will list no jobs for it.\n", entry.Path)
	}
	fmt.Fprintln(stdout, "Restart a running mg serve to pick up registry changes.")
	return 0
}

// serveProjectsRm implements `mg serve-projects rm <name>` — unregisters the
// project registered under the given name.
func serveProjectsRm(registryPath string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: mg serve-projects rm <name>")
		return 1
	}
	entry, err := serve.RemoveRegistryEntry(registryPath, args[0])
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Unregistered '%s' (%s) from %s.\n", entry.Name, entry.Path, registryPath)
	fmt.Fprintln(stdout, "Restart a running mg serve to pick up registry changes.")
	return 0
}
