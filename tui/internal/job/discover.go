package job

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/lmuskalla/manigot/tui/internal/git"
)

// JobsRelDir is where live jobs live, relative to the project root.
// Kept in sync with scripts/new-job.sh:10 and scripts/finish-job.sh:9.
const JobsRelDir = "docs/jobs"

// ArchiveDirName is the subdirectory under JobsRelDir that holds finished
// jobs. finish-job.sh moves done jobs here and filters it out with
// `-v '/archive'`; discovery does the same.
const ArchiveDirName = "archive"

// FindProjectRoot walks up from the current working directory until it finds a
// directory containing a docs/ subdirectory, mirroring the find_project_root
// helper in scripts/run.sh:46, scripts/new-job.sh:37 and scripts/finish-job.sh:21.
//
// It returns ("", nil) when no such directory exists before the filesystem
// root — the same convention the bash scripts use (empty string means "not
// found"). An error is only returned if the working directory itself cannot
// be determined.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fi, statErr := os.Stat(filepath.Join(dir, "docs")); statErr == nil && fi.IsDir() {
			return filepath.Clean(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding docs/.
			return "", nil
		}
		dir = parent
	}
}

// Discover enumerates every open job (207bfu_git-worktrees, Decision 5): each
// job lives in its own git worktree (created by scripts/new-job.sh), so for
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
// the file new-job.sh's scaffold always writes. This is what keeps the main
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
