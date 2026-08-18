package job

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lmuskalla/manigot/internal/git"
)

// JobsRelDir is where live jobs live, relative to the project root.
const JobsRelDir = "docs/jobs"

// ArchiveDirName is the subdirectory under JobsRelDir that holds finished
// jobs. mg done moves done jobs here, and discovery excludes it.
const ArchiveDirName = "archive"

// FindProjectRoot walks up from the current working directory until it finds a
// directory containing a docs/ subdirectory — the shared project-root walk-up
// the session launcher and the CLI commands all use.
//
// It returns ("", nil) when no such directory exists before the filesystem
// root (or before the invoking directory's own git repo boundary, when it's
// inside one — see FindProjectRootFrom) — the same convention the bash
// scripts use (empty string means "not found"). An error is only returned if
// the working directory itself cannot be determined.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindProjectRootFrom(dir)
}

// FindProjectRootFrom is FindProjectRoot's parameterized form: it walks up
// from startDir instead of the process's working directory. The session
// launcher uses it to resolve a project from an explicit root (mg-jdi runs
// its --print invocations against the project it was started in, not the
// process cwd).
//
// The walk-up never crosses the invoking directory's own git repo boundary:
// a project's path must always be respected as-is, so a repo nested inside
// another repo's working copy (each with its own .git — e.g. a sub-project
// checked out inside a monorepo) must never have its root resolved to the
// outer repo just because the outer one happens to have a docs/ dir and the
// inner one doesn't yet. When startDir is inside a git repo, reaching that
// repo's toplevel without finding docs/ stops the walk right there.
func FindProjectRootFrom(startDir string) (string, error) {
	toplevel, gitErr := git.RevParseToplevel(startDir)
	dir := startDir
	for {
		if fi, statErr := os.Stat(filepath.Join(dir, "docs")); statErr == nil && fi.IsDir() {
			return filepath.Clean(dir), nil
		}
		if gitErr == nil && filepath.Clean(dir) == filepath.Clean(toplevel) {
			// Reached this repo's boundary without finding docs/ — never
			// cross into a parent/unrelated git repo beyond it.
			return "", nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding docs/.
			return "", nil
		}
		dir = parent
	}
}

// Discover enumerates every open job: each
// job lives in its own git worktree (created by mg job), so for
// every local branch that has a registered worktree (via
// git.WorktreeForBranch), Discover reads whatever job directories exist under
// that worktree's own docs/jobs/ straight off disk.
//
// The main worktree (root itself) is scanned like any other worktree, not
// skipped. In steady state it sits on the base branch and its docs/jobs/
// holds nothing but archive/ (excluded below), so it contributes nothing; but
// a pre-worktree job whose branch is still checked out in the main worktree —
// the transitional case this change itself is in — IS listed, so the TUI and
// mg-jdi keep working on it. Its Branch is the main worktree's checked-out
// branch, which is authoritative for it.
//
// Every job's brief.md is read from its own worktree's working tree (no
// branch check, no `git show`) — a job's own worktree is unconditionally the
// live, correct checkout for it.
//
// A directory under docs/jobs/ only counts as a job if it has a brief.md —
// the file mg job's scaffold always writes. This is what keeps the main
// worktree's non-job content (a stray empty directory, or anything else
// without a brief.md) from being mislisted as a job — mg-jdi's status/run.log
// sidecar lives under .manigot/jdi-status/, outside docs/jobs/ entirely —
// matching the pre-worktree enumeration, which only ever saw tracked job
// directories via `git ls-tree`.
//
// Graceful fallback: a non-repo (git unavailable) or a repo with no branches
// yet (an unborn HEAD, before worktrees are even possible) falls back to the
// old working-tree-only enumeration (discoverWorkingTree), so a project that
// isn't under git — or a fresh repo before its first commit — keeps behaving
// exactly as before.
func Discover(root string) ([]Job, error) {
	branches, err := git.LocalBranches(root)
	if err != nil || len(branches) == 0 {
		return discoverWorkingTree(root)
	}

	var jobs []Job
	for _, branch := range branches {
		wtPath, ok, werr := git.WorktreeForBranch(root, branch)
		if werr != nil || !ok {
			continue
		}

		jobsDir := filepath.Join(wtPath, JobsRelDir)
		entries, derr := os.ReadDir(jobsDir)
		if derr != nil {
			// No docs/jobs under this worktree at all — not a job worktree
			// (or the read simply failed); either way, nothing to list here.
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == ArchiveDirName {
				continue
			}
			jobDir := filepath.Join(jobsDir, e.Name())
			if _, serr := os.Stat(filepath.Join(jobDir, "brief.md")); serr != nil {
				// No brief.md — not a job (a stray empty directory, or
				// anything else without a brief.md). ReadJob's
				// half-formed-job tolerance still applies once a job dir
				// is identified.
				continue
			}
			j, _ := ReadJob(jobDir) // ReadJob never hard-fails; see its doc.
			j.Root = root
			j.Branch = branch // the worktree's own branch is authoritative.
			jobs = append(jobs, j)
		}
	}

	sortJobs(jobs)
	return jobs, nil
}

// discoverWorkingTree is the pre-worktree enumeration: list docs/jobs/* in
// the working tree only, excluding archive/. Used as the not-a-repo /
// no-branches fallback, so projects that aren't under git (or a fresh repo
// before its first commit) keep working exactly as before.
func discoverWorkingTree(root string) ([]Job, error) {
	jobsDir := filepath.Join(root, JobsRelDir)
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	jobs := make([]Job, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == ArchiveDirName {
			continue
		}
		dir := filepath.Join(jobsDir, e.Name())
		j, _ := ReadJob(dir) // ReadJob never hard-fails; see its docs.
		j.Root = root
		jobs = append(jobs, j)
	}

	sortJobs(jobs)
	return jobs, nil
}

// PrefixJobDirName returns the first directory name (not the full path)
// under root's docs/jobs/ whose name starts with prefix, excluding archive/ —
// the deterministic stand-in for the scripts' `find docs/jobs -maxdepth 1
// -type d -name '<prefix>*' -not -name archive | head -1`. Returns "" when
// there is none; callers join the root themselves. The single docs/jobs scan
// shared by the --job no-branches fallback (session) and the non-git delete
// path (job).
func PrefixJobDirName(root, prefix string) string {
	jobsDir := filepath.Join(root, JobsRelDir)
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		return ""
	}
	// os.ReadDir already sorts by name.
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ArchiveDirName {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			return e.Name()
		}
	}
	return ""
}

// existingJobIDs returns the set of every job id already in use in the
// project rooted at root — open jobs (from Discover, which covers the
// worktree-per-branch layout and falls back to the working tree for non-git
// or branchless repos) plus archived jobs in docs/jobs/archive/. CreateJob
// uses it to guarantee a word id is never re-used, including against jobs
// that were archived long ago (the confirmed never-reuse policy). Old random
// ids (e.g. "irw320") are ordinary set members; mixed old/new formats are
// handled uniformly.
func existingJobIDs(root string) (map[string]bool, error) {
	ids := make(map[string]bool)
	jobs, err := Discover(root)
	if err != nil {
		return nil, err
	}
	for _, j := range jobs {
		ids[j.ID] = true
	}

	archiveDir := filepath.Join(root, JobsRelDir, ArchiveDirName)
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return ids, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		j, _ := ReadJob(filepath.Join(archiveDir, e.Name()))
		ids[j.ID] = true
	}
	return ids, nil
}

// sortJobs orders jobs by date descending (newest first) with Name as a stable
// tiebreaker — the "recent work first" ordering the README's job workflow
// implies. Shared by both the worktree-backed and working-tree paths.
func sortJobs(jobs []Job) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].Date != jobs[j].Date {
			return jobs[i].Date > jobs[j].Date
		}
		return jobs[i].Name < jobs[j].Name
	})
}
