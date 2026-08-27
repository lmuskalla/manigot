// Package home locates the manigot checkout the running binary belongs to —
// the source of .env, config/tui-settings.json, agents/, skills/, assets/,
// prompts/ (the system-wide meta prompt) and the project template. It
// replaced the resolve package's checkout derivation: once the single `mg`
// binary became the whole host-side tool there were no host scripts left to
// resolve, but the checkout that provides the binary's data files still needs
// locating — for an installed symlink (whose real target lives in a checkout),
// a checkout's own bin/mg, and `go run` from the checkout root.
package home

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// EnvHome is the environment variable that overrides the checkout location
// ($MANIGOT_HOME). It is a public, user-facing contract — renaming it breaks
// anyone who has put it in their shell profile.
const EnvHome = "MANIGOT_HOME"

// Root returns the manigot checkout root, or "" when it cannot be located.
// Resolution order:
//
//  1. $MANIGOT_HOME (an explicit, user-set override);
//  2. the running binary's own location — bin/mg inside the checkout, or a
//     symlinked install resolved back into it (the binary one directory
//     below the checkout root, and the root itself);
//  3. the working directory (covers `cd src && go run ./cmd/mg` from the
//     checkout root, where the binary lives in a temp build dir that looks
//     like nothing).
func Root() string {
	if home := strings.TrimSpace(os.Getenv(EnvHome)); home != "" {
		return absOrSame(home)
	}
	for _, dir := range executableRoots() {
		return dir
	}
	if cwd, err := os.Getwd(); err == nil {
		if looksLikeCheckout(cwd) {
			return cwd
		}
	}
	return ""
}

// Seed sets $MANIGOT_HOME for this process (and therefore for every child it
// spawns) when it is not already set and a checkout could be derived from the
// binary's own location. Call it once at startup — it means a directly
// invoked bin/mg carries its checkout context the same way a wrapper script
// would have. It returns the value in effect, or "".
func Seed() string {
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

// executableRoots derives candidate manigot checkouts from the location of the
// running binary. Two shapes are considered, for both the literal path and the
// symlink-resolved one (an install is typically a symlink from /usr/local/bin
// into a checkout): the binary sitting in the checkout root, and one directory
// below it (bin/). Under `go run` the binary lives in a temporary build
// directory, which is not a checkout — looksLikeCheckout rejects it, so that
// case falls through to the working-directory fallback.
func executableRoots() []string {
	executableRootsOnce.Do(func() {
		executableRootsCache = computeExecutableRoots()
	})
	return executableRootsCache
}

// executableRootsOnce / executableRootsCache memoize computeExecutableRoots:
// os.Executable and filepath.EvalSymlinks are process-constant and not cheap,
// and Root() sits on config's env-read hot path — deriving them once per
// process is the win. The MANIGOT_HOME env check in Root() stays uncached: it
// is cheap and must be read fresh (Seed sets it at startup, and tests set it
// per-test).
var (
	executableRootsOnce  sync.Once
	executableRootsCache []string
)

// computeExecutableRoots is executableRoots' uncached body.
func computeExecutableRoots() []string {
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
	seen := map[string]bool{}
	for _, p := range paths {
		dir := filepath.Dir(p)
		for _, candidate := range []string{dir, filepath.Dir(dir)} {
			if candidate == "" || seen[candidate] || !looksLikeCheckout(candidate) {
				continue
			}
			seen[candidate] = true
			roots = append(roots, candidate)
		}
	}
	return roots
}

// looksLikeCheckout reports whether dir is plausibly a manigot checkout. It
// keys off scripts/entrypoint.sh — the one script that survives the
// consolidation (it runs inside the container image, not the host) — so that
// neither a coincidental directory layout nor a `go run` temp dir is mistaken
// for one.
func looksLikeCheckout(dir string) bool {
	if dir == "" || dir == "." || dir == string(filepath.Separator) {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "scripts", "entrypoint.sh"))
	return err == nil && !info.IsDir()
}

// absOrSame makes path absolute, falling back to the input if the working
// directory cannot be determined.
func absOrSame(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
