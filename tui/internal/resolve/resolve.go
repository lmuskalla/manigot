// Package resolve locates the manigot host commands that the TUI shells out
// to (sc, mg-job, mg-done, mg-delete).
//
// The TUI must not assume a particular installation method. Users may symlink
// the launchers into /usr/local/bin under their canonical names, or not
// install them on $PATH at all and run everything out of a checkout. Shell
// aliases (alias mg-job=…) are deliberately *not* a supported discovery
// mechanism: aliases only exist inside interactive shells, so neither
// exec.Command nor `bash -lc` can expand them. The env var overrides below are
// the escape hatch for those users.
//
// Resolution tries, in order:
//
//  1. the command's explicit env var override (an absolute path, or a command
//     name to look up on $PATH),
//  2. the canonical command name on $PATH,
//  3. the implementing script inside a manigot checkout.
//
// The first hit wins, and Resolve reports both the absolute path it found and a
// short description of how it got there, so the UI can explain itself.
//
// # Environment variable contract
//
// These five names are a public, user-facing contract. Renaming one is a
// breaking change for anyone who has put it in their shell profile.
//
//	MANIGOT_BIN         path to (or name of) the manigot launcher
//	                     — scripts/run.sh
//	MANIGOT_JOB_BIN     path to (or name of) the job-creation command
//	                     — scripts/new-job.sh
//	MANIGOT_DONE_BIN    path to (or name of) the job-completion command
//	                     — scripts/finish-job.sh
//	MANIGOT_DELETE_BIN  path to (or name of) the job-deletion command
//	                     — scripts/delete-job.sh
//	MANIGOT_HOME        path to a manigot checkout, used for the script
//	                     fallback ($MANIGOT_HOME/scripts/<name>.sh)
//
// The four *_BIN variables take either an absolute or relative path (anything
// containing a path separator) or a bare command name that is looked up on
// $PATH. A value that cannot be resolved is an error: the resolver does not
// silently ignore a misspelled override.
//
// MANIGOT_HOME is exported by scripts/tui.sh, so the wrapper-script
// install needs no configuration at all; setting it by hand is only necessary
// when running the binary directly from an unusual location.
package resolve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The environment variables that override resolution. See the package doc for
// the full contract; these constants are the single source of truth for the
// names, so callers and messages never spell them out by hand.
const (
	// EnvManigot overrides the manigot launcher (scripts/run.sh).
	EnvManigot = "MANIGOT_BIN"

	// EnvJob overrides the job-creation command (scripts/new-job.sh).
	EnvJob = "MANIGOT_JOB_BIN"

	// EnvDone overrides the job-completion command (scripts/finish-job.sh).
	EnvDone = "MANIGOT_DONE_BIN"

	// EnvDelete overrides the job-deletion command (scripts/delete-job.sh).
	EnvDelete = "MANIGOT_DELETE_BIN"

	// EnvHome points at a manigot checkout, enabling the script fallback.
	EnvHome = "MANIGOT_HOME"
)

// Spec describes one host command and every way it may be found.
type Spec struct {
	// Label is the canonical, user-facing name of the command. Used in
	// messages only.
	Label string

	// EnvVar names the environment variable that overrides resolution
	// entirely. Its value may be a path to an executable or a bare command
	// name to look up on $PATH.
	EnvVar string

	// Names are the command names looked up on $PATH, in priority order.
	// Every current Spec has exactly one, but Resolve supports several so a
	// future rename can add a transitional alias without changing callers.
	Names []string

	// Script is the path of the implementing script relative to a manigot
	// checkout (e.g. "scripts/new-job.sh"). Empty disables the fallback.
	Script string
}

// Result describes a successful resolution.
type Result struct {
	// Path is an absolute path to an executable.
	Path string

	// How is a short human description of which strategy matched, e.g.
	// "$MANIGOT_JOB_BIN" or "mg-job on $PATH".
	How string
}

// NotFoundError reports that none of the strategies located the command. It
// carries the full list of what was tried, and a hint at how to fix it, so the
// UI can show a diagnosis rather than just "command not found".
type NotFoundError struct {
	Spec  Spec
	Tried []string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found — tried: %s. %s",
		e.Spec.Label, e.TriedList(), e.Hint())
}

// TriedList renders the attempted strategies as one readable line, in the order
// they were tried.
func (e *NotFoundError) TriedList() string {
	if len(e.Tried) == 0 {
		return "nothing"
	}
	return strings.Join(e.Tried, " · ")
}

// Hint is a one-line suggestion for fixing the failure: install the launchers,
// or point the relevant env var at the executable.
func (e *NotFoundError) Hint() string {
	install := "install the manigot launchers with `make install` in your checkout"
	if e.Spec.EnvVar == "" {
		return install
	}
	example := e.Spec.Script
	if example == "" {
		example = e.Spec.Label
	}
	return fmt.Sprintf("%s, or set $%s=/path/to/manigot/%s (or $%s=/path/to/manigot)",
		install, e.Spec.EnvVar, example, EnvHome)
}

// Resolve finds the command described by s. See the package doc for the order.
func Resolve(s Spec) (Result, error) {
	var tried []string

	// 1. Explicit override. If it is set but unusable that is a configuration
	// error, so fail loudly rather than silently falling through to $PATH.
	if s.EnvVar != "" {
		if raw := strings.TrimSpace(os.Getenv(s.EnvVar)); raw != "" {
			path, err := resolveOverride(raw)
			if err != nil {
				return Result{}, fmt.Errorf("$%s is set to %q but %w", s.EnvVar, raw, err)
			}
			return Result{Path: path, How: "$" + s.EnvVar}, nil
		}
		tried = append(tried, "$"+s.EnvVar+" (unset)")
	}

	// 2. Candidate command names on $PATH, in priority order.
	for _, name := range s.Names {
		if path, err := exec.LookPath(name); err == nil {
			return Result{Path: absOrSame(path), How: name + " on $PATH"}, nil
		}
		tried = append(tried, name+" on $PATH")
	}

	// 3. The script inside a manigot checkout.
	if s.Script != "" {
		roots := repoRoots()
		for _, root := range roots {
			candidate := filepath.Join(root, s.Script)
			if isExecutableFile(candidate) {
				return Result{Path: candidate, How: candidate}, nil
			}
			tried = append(tried, candidate)
		}
		if len(roots) == 0 {
			tried = append(tried, "$"+EnvHome+"/"+s.Script+" ($"+EnvHome+" unset)")
		}
	}

	return Result{}, &NotFoundError{Spec: s, Tried: tried}
}

// resolveOverride interprets an env var value: anything containing a path
// separator is treated as a path, everything else as a command name on $PATH.
func resolveOverride(raw string) (string, error) {
	if strings.ContainsRune(raw, filepath.Separator) {
		path := absOrSame(raw)
		if !isExecutableFile(path) {
			return "", fmt.Errorf("that is not an executable file")
		}
		return path, nil
	}
	path, err := exec.LookPath(raw)
	if err != nil {
		return "", fmt.Errorf("that name is not on $PATH: %w", err)
	}
	return absOrSame(path), nil
}

// repoRoots lists the manigot checkouts to look for scripts in, most
// trustworthy first: what the user (or the wrapper script) declared via
// $MANIGOT_HOME, then whatever the running binary's own location implies.
func repoRoots() []string {
	var roots []string
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		roots = append(roots, dir)
	}

	if home := strings.TrimSpace(os.Getenv(EnvHome)); home != "" {
		add(absOrSame(home))
	}
	for _, dir := range executableRoots() {
		add(dir)
	}
	return roots
}

// executableRoots derives candidate manigot checkouts from the location of the
// running binary, so a directly-invoked bin/manigot-tui still finds scripts/
// without the wrapper script's $MANIGOT_HOME.
//
// Two shapes are considered, for both the literal path and the symlink-resolved
// one (an install is typically a symlink from /usr/local/bin into a checkout):
// the binary sitting in the checkout root, and one directory below it (bin/).
//
// Under `go run` the binary lives in a temporary build directory, which is not
// a checkout — looksLikeCheckout rejects it, so that case falls through to
// $MANIGOT_HOME or $PATH rather than producing a bogus root.
func executableRoots() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}

	paths := []string{exe}
	// EvalSymlinks fails on a broken link; ignore it and keep the literal path.
	if real, err := filepath.EvalSymlinks(exe); err == nil && real != exe {
		paths = append(paths, real)
	}

	var roots []string
	for _, p := range paths {
		dir := filepath.Dir(p)
		for _, candidate := range []string{dir, filepath.Dir(dir)} {
			if looksLikeCheckout(candidate) {
				roots = append(roots, candidate)
			}
		}
	}
	return roots
}

// looksLikeCheckout reports whether dir is plausibly a manigot checkout. It
// keys off scripts/run.sh, the one file that has always been there, so that a
// coincidental directory layout (or a `go run` temp dir) is not mistaken for
// one.
func looksLikeCheckout(dir string) bool {
	if dir == "" || dir == "." || dir == string(filepath.Separator) {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "scripts", "run.sh"))
	return err == nil && !info.IsDir()
}

// Home returns the manigot checkout the TUI considers itself to belong to, or
// "" if it cannot tell. Callers use it to seed $MANIGOT_HOME for child
// processes; see SeedHome.
func Home() string {
	roots := repoRoots()
	if len(roots) == 0 {
		return ""
	}
	return roots[0]
}

// SeedHome sets $MANIGOT_HOME for this process (and therefore for every child
// it spawns) when it is not already set and a checkout could be derived from
// the binary's own location. Call it once at startup: it means a directly
// invoked bin/manigot-tui passes the same context to the scripts it runs as
// the wrapper script would. It returns the value in effect, or "".
func SeedHome() string {
	if home := strings.TrimSpace(os.Getenv(EnvHome)); home != "" {
		return home
	}
	for _, dir := range executableRoots() {
		if err := os.Setenv(EnvHome, dir); err == nil {
			return dir
		}
	}
	return ""
}

// isExecutableFile reports whether path is a regular file with an execute bit
// set. Symlinks are followed, since that is how the launchers are installed.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// absOrSame makes path absolute, falling back to the input if the working
// directory cannot be determined.
func absOrSame(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
