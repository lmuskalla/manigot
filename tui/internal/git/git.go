// Package git is a thin exec-backed wrapper over the git commands the safecode
// TUI needs for cross-branch, git-backed job discovery (see the
// "keep-track-of-jobs" brief). It is the only place in the TUI that shells out
// to git, so the job/launch/ui packages ask about branches and per-branch file
// contents through it rather than each shelling out ad-hoc.
//
// Every function degrades gracefully: a missing git binary, a directory that
// isn't a repository, or a missing branch/path returns a classified error
// (ErrNotARepo / os.ErrNotExist) or an empty result rather than crashing —
// mirroring how job.Discover already tolerates a missing docs/jobs.
//
// root is always passed to git via `git -C <root>`, so callers may hand over an
// absolute project root without worrying about the process's own cwd.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// JobsRelDir is where live jobs live, relative to the project root. Kept in
// sync with job.JobsRelDir (and the bash scripts); duplicated here so this
// package has no import cycle on job.
const JobsRelDir = "docs/jobs"

// ArchiveDirName is the subdirectory under JobsRelDir that holds finished
// jobs; ListJobDirs excludes it, mirroring job.Discover and finish-job.sh's
// `-v '/archive'` filter.
const ArchiveDirName = "archive"

// ErrNotARepo is returned when root is not inside a git repository or the git
// binary itself cannot be found. Callers that want to degrade gracefully
// (job.Discover's not-a-repo fallback to the working tree) test for it via
// errors.Is.
var ErrNotARepo = errors.New("not a git repository (or git not installed)")

// run executes `git -C root <args>` and returns raw stdout, raw stderr, and the
// exec error (nil on success). Callers interpret the result: the package-level
// helpers below normalize the not-a-repo / missing-path cases.
func run(root string, args ...string) ([]byte, string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return out, stderr.String(), err
}

// notARepo reports whether a git failure is the not-a-repo or git-missing case
// (as opposed to a real, recoverable error like a missing path). The git binary
// being absent surfaces as exec.ErrNotFound; a real directory that isn't a
// repository surfaces via git's own "not a git repository" stderr.
func notARepo(stderr string, err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	return strings.Contains(stderr, "not a git repository")
}

// wrapErr builds an informative error from a non-not-a-repo git failure,
// appending the trimmed stderr so callers can see git's own explanation.
func wrapErr(prefix string, err error, stderr string) error {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%s: %w: %s", prefix, err, msg)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// LocalBranches returns the short names of every local branch (refs/heads/*)
// in the repository at root, in git's refname order. A repository with no
// commits yet (an unborn HEAD, no refs under refs/heads/) returns an empty
// slice and a nil error. A non-repo or a missing git binary returns ErrNotARepo.
func LocalBranches(root string) ([]string, error) {
	out, stderr, err := run(root, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		if notARepo(stderr, err) {
			return nil, ErrNotARepo
		}
		return nil, wrapErr("git for-each-ref", err, stderr)
	}
	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// CurrentBranch returns the short name of the branch checked out in root, or
// ("", nil) for a detached HEAD. A non-repo / missing git binary returns
// ("", ErrNotARepo).
//
// `git symbolic-ref --quiet --short HEAD` exits 1 with no output on a detached
// HEAD (the --quiet flag suppresses its stderr message); that is the exact
// "no branch" signal we want, distinct from a real failure.
func CurrentBranch(root string) (string, error) {
	out, stderr, err := run(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		if notARepo(stderr, err) {
			return "", ErrNotARepo
		}
		// An empty stderr with a non-zero exit is the detached-HEAD case:
		// --quiet suppressed the message, so treat it as "no branch".
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.TrimSpace(stderr) == "" {
			return "", nil
		}
		return "", wrapErr("git symbolic-ref", err, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}

// ListJobDirs returns the top-level job directory names living under
// <root>/docs/jobs on the given branch, excluding the archive/ subdirectory and
// any non-directory entries (e.g. a stray docs/jobs/README.md). The branch is
// the short name from LocalBranches.
//
// It uses the `<branch>:docs/jobs` tree-ish form so the listed names are plain
// basenames (aaaa01_a, bbbb02_b) rather than repo-root-relative paths. A branch
// that has no docs/jobs (or where the path simply doesn't exist on that branch)
// returns an empty slice and a nil error — a branch with no jobs is normal,
// not a failure. An entirely missing git binary / non-repo returns ErrNotARepo.
func ListJobDirs(root, branch string) ([]string, error) {
	out, stderr, err := run(root, "ls-tree", branch+":"+JobsRelDir)
	if err != nil {
		if notARepo(stderr, err) {
			return nil, ErrNotARepo
		}
		// Any other ls-tree failure (docs/jobs absent on the branch, the
		// branch itself being absent — both surface as "Not a valid object
		// name") means "no jobs here" from the caller's perspective, so it
		// degrades to an empty result rather than an error.
		return nil, nil
	}
	var dirs []string
	for _, line := range strings.Split(string(out), "\n") {
		// Format per entry: "<mode> <type> <object>\t<name>".
		if line = strings.TrimRight(line, "\r"); line == "" {
			continue
		}
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		meta, name := line[:tab], line[tab+1:]
		fields := strings.Fields(meta)
		// fields[1] is the object type ("tree" for a directory, "blob" for a file).
		if len(fields) < 2 || fields[1] != "tree" {
			continue
		}
		if name == ArchiveDirName {
			continue
		}
		dirs = append(dirs, name)
	}
	return dirs, nil
}

// ShowFile returns the bytes of path as it exists on branch (short name from
// LocalBranches), via `git show <branch>:<path>`. path is relative to the
// repository root (e.g. "docs/jobs/abc123_x/brief.md").
//
// A path or branch that does not exist returns an error wrapping os.ErrNotExist
// so callers can test with errors.Is(err, os.ErrNotExist). A non-repo / missing
// git binary returns ErrNotARepo.
func ShowFile(root, branch, path string) ([]byte, error) {
	out, stderr, err := run(root, "show", branch+":"+path)
	if err != nil {
		if notARepo(stderr, err) {
			return nil, ErrNotARepo
		}
		// git's "does not exist in '<branch>'" covers a missing path; an
		// unknown branch surfaces as "invalid object name". Both mean "this
		// file isn't available on this branch", which the detail view and
		// Stage() treat as not-written — so they map to os.ErrNotExist.
		msg := strings.TrimSpace(stderr)
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "invalid object name") || strings.Contains(msg, "Not a valid object name") {
			return nil, fmt.Errorf("git show %s:%s: %w", branch, path, os.ErrNotExist)
		}
		return nil, wrapErr(fmt.Sprintf("git show %s:%s", branch, path), err, stderr)
	}
	return out, nil
}

// Checkout switches the working tree at root to branch (short name), via
// `git checkout <branch>`. It is the host-side mutation behind the TUI's
// "switch to this job's branch" action (detail-view key "c"). A checkout that
// git refuses (e.g. uncommitted changes that would be overwritten) returns the
// wrapped error including git's stderr so the caller can surface the reason in
// the status line.
func Checkout(root, branch string) error {
	_, stderr, err := run(root, "checkout", branch)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git checkout "+branch, err, stderr)
	}
	return nil
}
