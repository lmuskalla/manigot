package job

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lmuskalla/manigot/internal/git"
)

// Orphan is a leftover worktree directory under the project's
// .manigot-worktrees/ whose git registration is gone: the directory still
// exists, but the gitdir its .git file points at no longer does. git worktree
// prune cannot reach these — it only prunes stale *metadata* for worktrees
// whose working directory vanished, and deliberately leaves working
// directories behind — so a job scaffolded and then abandoned leaves a dead
// directory that no tool path removes until now.
type Orphan struct {
	// Name is the base directory name (the id_slug), e.g.
	// "o3kk3n_jdi-is-broken" — the name mg delete resolves by.
	Name string

	// Dir is the absolute path of the leftover directory.
	Dir string

	// GitDir is the gitdir path the .git file recorded, now gone. Always
	// non-empty for a reported orphan — detection requires a .git pointer file
	// naming a vanished gitdir.
	GitDir string
}

// worktreeParents returns the two candidate parent directories under which a
// project's job worktrees live: the sibling layout
// (<dirname(root)>/.manigot-worktrees/<basename(root)>) and the nested
// (mount-point) layout (<root>/.manigot-worktrees). Both are scanned because
// the layout is decided per-creation, and either (or, after a migration, both)
// can hold leftover worktrees.
func worktreeParents(root string) []string {
	return []string{
		filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root)),
		filepath.Join(root, ".manigot-worktrees"),
	}
}

// DiscoverOrphans scans the project's .manigot-worktrees parents (sibling and
// nested layouts) and returns every leftover worktree directory whose .git
// file points to a gitdir that no longer exists.
//
// A directory only counts as an orphan when its .git file names a gitdir that
// is gone. A live worktree's .git file names an existing gitdir, so it is
// skipped; a directory with no .git file, or with a .git *directory* (a
// standalone repository), is also skipped — the tool removes worktrees, not
// arbitrary junk. No git invocation is involved: the scan is pure filesystem,
// so it degrades gracefully on a non-repo or a repo with no .manigot-worktrees
// parent at all (an empty result, not an error).
func DiscoverOrphans(root string) ([]Orphan, error) {
	var orphans []Orphan
	seen := map[string]bool{}

	for _, parent := range worktreeParents(root) {
		entries, err := os.ReadDir(parent)
		if err != nil {
			// No .manigot-worktrees here — not an error.
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(parent, e.Name())
			if seen[dir] {
				continue
			}
			gitdir := readGitdir(filepath.Join(dir, ".git"))
			if gitdir == "" {
				// No .git pointer file at all — a stray non-worktree
				// directory, or a .git *directory* (a standalone repository).
				// Neither is a linked worktree; never report or touch it.
				continue
			}
			// A .git file naming a gitdir that still exists → a live worktree
			// (registered or not), not an orphan.
			if _, err := os.Stat(gitdir); err == nil {
				continue
			}
			seen[dir] = true
			orphans = append(orphans, Orphan{Name: e.Name(), Dir: dir, GitDir: gitdir})
		}
	}

	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Name < orphans[j].Name })
	return orphans, nil
}

// readGitdir parses a worktree's .git pointer file ("gitdir: <path>", the
// format git worktree add writes) and returns the gitdir path it names, "" for
// a file that is not such a pointer (including a .git *directory*, which has a
// different shape and is never a worktree pointer).
func readGitdir(dotgit string) string {
	fi, err := os.Stat(dotgit)
	if err != nil || fi.IsDir() {
		return ""
	}
	data, err := os.ReadFile(dotgit)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir:") {
		return ""
	}
	path := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(dotgit), path)
	}
	return filepath.Clean(path)
}

// MatchOrphan resolves jobArg against the project's orphaned worktrees, exact
// then prefix on the orphan's base name — the same resolution DeleteJob
// applies to branches, so `mg delete o3kk3n` reaches the orphan dir
// "o3kk3n_jdi-is-broken". ok is false when nothing (or more than one prefix
// candidate) matches.
func MatchOrphan(root, jobArg string) (o Orphan, ok bool) {
	orphans, err := DiscoverOrphans(root)
	if err != nil {
		return Orphan{}, false
	}
	for _, orph := range orphans {
		if orph.Name == jobArg {
			return orph, true
		}
	}
	var matches []Orphan
	for _, orph := range orphans {
		if strings.HasPrefix(orph.Name, jobArg) {
			matches = append(matches, orph)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return Orphan{}, false
}

// RemoveOrphans permanently deletes the given orphaned worktrees, each with
// the same confirmation discipline as DeleteJob: an explicit listing of what
// will be removed, "This cannot be undone.", and a Proceed? prompt. A declined
// confirmation returns ErrCancelled (the scripts' `exit 0`) and stops — no
// further orphan is touched. After removal it prunes stale worktree metadata
// (mirroring `git worktree prune`), so the metadata side of the hole is closed
// too.
func RemoveOrphans(root string, orphans []Orphan, confirm ConfirmFunc, out io.Writer) error {
	for _, o := range orphans {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "This will permanently delete orphaned worktree: %s\n", o.Name)
		fmt.Fprintf(out, "  Dir     : %s\n", o.Dir)
		if o.GitDir != "" {
			fmt.Fprintf(out, "  Gitdir  : %s (no longer exists)\n", o.GitDir)
		}
		fmt.Fprintln(out, "  Note    : no branch or worktree registration — a leftover from an abandoned job")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "This cannot be undone.")
		if err := askConfirm(confirm, out, "  Proceed? [y/N] "); err != nil {
			return err
		}
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "→ Removing orphaned worktree %s...\n", o.Dir)
		if err := os.RemoveAll(o.Dir); err != nil {
			return err
		}
		// An abandoned job may also have left its mg-jdi sidecar behind (the
		// same name as the orphan). Best-effort — the orphan itself is gone.
		if removed, err := RemoveJDIStatus(root, o.Name); err != nil {
			fmt.Fprintf(out, "  Warning: could not remove mg-jdi status for %s: %v\n", o.Name, err)
		} else if removed {
			fmt.Fprintf(out, "→ Removing mg-jdi status for %s...\n", o.Name)
		}
		fmt.Fprintf(out, "✓ Orphan removed: %s\n", o.Name)
	}
	_ = git.WorktreePrune(root)
	return nil
}

// RemoveOrphansConfirmed removes every orphan without further confirmation —
// the caller (mg jobs' batch removal offer) has already confirmed the whole
// set. Each removal prints its progress to out, and stale worktree metadata is
// pruned at the end, mirroring `git worktree prune`.
func RemoveOrphansConfirmed(root string, orphans []Orphan, out io.Writer) error {
	for _, o := range orphans {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "→ Removing orphaned worktree %s...\n", o.Dir)
		if err := os.RemoveAll(o.Dir); err != nil {
			return err
		}
		// Same best-effort sidecar cleanup as RemoveOrphans above.
		if removed, err := RemoveJDIStatus(root, o.Name); err != nil {
			fmt.Fprintf(out, "  Warning: could not remove mg-jdi status for %s: %v\n", o.Name, err)
		} else if removed {
			fmt.Fprintf(out, "→ Removing mg-jdi status for %s...\n", o.Name)
		}
		fmt.Fprintf(out, "✓ Orphan removed: %s\n", o.Name)
	}
	_ = git.WorktreePrune(root)
	return nil
}
