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

// Discover enumerates every in-flight job across all local branches, not just
// the working tree of the currently-checked-out branch. For each local branch
// it lists that branch's job directories (excluding archive/), reads each
// job's brief.md, and builds a Job with Branch set to the branch it was found
// on and OnCurrentBranch flagging the one branch matching the working tree.
//
// Read strategy: a job found on the currently-checked-out branch has its brief
// read from the working tree (os.ReadFile) so uncommitted edits still show;
// every other job's brief is read via `git show <branch>:<path>` (the
// "keep-track-of-jobs" brief's last edge case).
//
// Duplicate copies of the same job ID (a job appearing on more than one
// branch — e.g. a stale branch after a merge) are collapsed by dedupByID,
// preferring the copy whose discovery branch equals the brief's own `branch:`
// frontmatter field. The date-desc / name-tiebreak sort still applies.
//
// Graceful fallback: a non-repo (or git unavailable) and a repo with no
// branches yet both fall back to the old working-tree-only enumeration, so a
// project that isn't under git, or a fresh repo before its first commit, keeps
// behaving exactly as before. A missing docs/jobs is not an error: it returns
// an empty slice.
//
// Discover keeps its pre-feature signature, so the call sites in tui/main.go
// and tui/internal/ui/app.go (refresh, new-job) are unchanged.
func Discover(root string) ([]Job, error) {
	branches, err := git.LocalBranches(root)
	if err != nil || len(branches) == 0 {
		// Not a repo / git missing / no commits yet → fall back to the
		// working tree, exactly the pre-feature behaviour.
		return discoverWorkingTree(root)
	}

	current, _ := git.CurrentBranch(root) // "" on detached HEAD → nothing is "current"

	var jobs []Job
	for _, branch := range branches {
		dirs, lerr := git.ListJobDirs(root, branch)
		if lerr != nil {
			continue // a branch git can't list is simply skipped.
		}
		onCurrent := branch == current
		for _, name := range dirs {
			jobs = append(jobs, readJobOnBranch(root, branch, name, onCurrent))
		}
	}

	jobs = dedupByID(jobs)
	sortJobs(jobs)
	return jobs, nil
}

// readJobOnBranch builds one Job for the job directory name living on branch,
// reading its brief from the working tree when onCurrent is true (so
// uncommitted edits show) and from git otherwise. Dir is always set to the
// path the job would have in the working tree, so the detail view / Stage can
// read the four files via whichever strategy OnCurrentBranch dictates.
//
// The brief's own `branch:` frontmatter value is preserved in briefBranch
// (parseBrief sets it on Branch first); Branch is then overwritten with the
// discovery branch, since Branch is what the UI displays and checks out.
func readJobOnBranch(root, branch, name string, onCurrent bool) Job {
	dir := filepath.Join(root, JobsRelDir, name)
	var j Job
	if onCurrent {
		j, _ = ReadJob(dir)
	} else {
		data, err := git.ShowFile(root, branch, filepath.ToSlash(filepath.Join(JobsRelDir, name, "brief.md")))
		if err != nil {
			// brief.md missing on this branch: best-effort defaults, same as
			// ReadJob's missing-file behaviour. Branch is still recorded.
			j = ReadJobFromBytes(name, dir, nil)
		} else {
			j = ReadJobFromBytes(name, dir, data)
		}
	}
	j.briefBranch = j.Branch // parsed from frontmatter by ReadJob/ReadJobFromBytes
	j.Branch = branch        // discovery branch: what the UI shows and checks out
	j.Root = root
	j.OnCurrentBranch = onCurrent
	return j
}

// dedupByID collapses jobs sharing an ID to a single entry, keeping the copy
// whose Branch (discovery branch) equals the brief's own `branch:` frontmatter
// field (briefBranch). This handles the brief's "a job dir appearing on
// multiple branches (stale branch after a merge)" edge case: the
// frontmatter-named branch is authoritative, so a stale copy on some other
// branch is dropped and the job is listed once.
//
// Because every copy of the same job shares one brief file, briefBranch is
// identical across copies — so dedup's tie-break is "prefer the copy found on
// the brief's own branch". When no copy matches (e.g. the brief is missing or
// has no `branch:` field) the first-seen copy wins; discovery order is git's
// refname order, which is deterministic.
//
// With no duplicates it is a no-op.
func dedupByID(jobs []Job) []Job {
	if len(jobs) <= 1 {
		return jobs
	}
	firstIdx := make(map[string]int) // id → index in result
	result := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		idx, seen := firstIdx[j.ID]
		if !seen {
			firstIdx[j.ID] = len(result)
			result = append(result, j)
			continue
		}
		// Both copies share the same briefBranch (same brief file); prefer
		// the candidate only if it is on the brief's branch and the existing
		// one isn't. A missing briefBranch (no `branch:` field) means "no
		// authoritative branch" → keep the first-seen copy.
		fb := j.briefBranch
		candidateOnHome := fb != "" && j.Branch == fb
		existingOnHome := fb != "" && result[idx].Branch == fb
		if candidateOnHome && !existingOnHome {
			result[idx] = j
		}
	}
	return result
}

// discoverWorkingTree is the pre-feature enumeration: list docs/jobs/* in the
// working tree only, excluding archive/. Used as the not-a-repo / no-branches
// fallback, so projects that aren't under git (or a fresh repo before its first
// commit) keep working exactly as before.
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
		// In the working-tree-only path there is no branch information
		// available (we may not even be in a repo), so Branch is left empty.
		// But the job's files ARE in the working tree (it's the only source),
		// so OnCurrentBranch is true: read paths must use Dir, not git show.
		j.Root = root
		j.OnCurrentBranch = true
		jobs = append(jobs, j)
	}

	sortJobs(jobs)
	return jobs, nil
}

// sortJobs orders jobs by date descending (newest first) with Name as a stable
// tiebreaker — the "recent work first" ordering the README's job workflow
// implies. Shared by both the cross-branch and working-tree paths.
func sortJobs(jobs []Job) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].Date != jobs[j].Date {
			return jobs[i].Date > jobs[j].Date
		}
		return jobs[i].Name < jobs[j].Name
	})
}
