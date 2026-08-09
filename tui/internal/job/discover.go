package job

import (
	"os"
	"path/filepath"
	"sort"
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
// found"). An error is only returned if the working directory itself cannot be
// determined.
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

// Discover enumerates jobs under <root>/docs/jobs, excluding the archive/
// directory. Jobs are sorted by date descending (newest first), with Name as a
// stable tiebreaker — the same "recent work first" ordering the README's job
// workflow implies.
//
// A missing docs/jobs directory (a fresh project with no jobs yet) is not
// an error: it returns an empty slice.
func Discover(root string) ([]Job, error) {
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
		jobs = append(jobs, j)
	}

	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].Date != jobs[j].Date {
			return jobs[i].Date > jobs[j].Date
		}
		return jobs[i].Name < jobs[j].Name
	})
	return jobs, nil
}
