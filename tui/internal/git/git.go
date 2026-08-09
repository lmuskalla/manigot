// Package git is a thin exec-backed wrapper over the git commands the manigot
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
	"regexp"
	"strconv"
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

// Commit is one entry in the recent-activity log built by RecentCommits.
type Commit struct {
	Hash      string
	ShortHash string
	Subject   string
	RelTime   string
	Branch    string
}

// recentCommitsFieldSep delimits the --format fields RecentCommits parses
// below. It's the ASCII Unit Separator, which — unlike a comma, space or pipe
// — never legitimately appears in a commit subject.
const recentCommitsFieldSep = "\x1f"

// RecentCommits returns the last n commits reachable from any local branch in
// root, deduped by commit hash and ordered most-recent first.
//
// It runs a single `git log --source -n <n> <every local branch>` rather than
// querying each branch tip separately and merging results by hand: passing
// every branch as a positional ref makes a single `git log` union-traverse
// their combined history and visit each commit exactly once, so a commit
// reachable from several branches (e.g. a job branch that hasn't diverged
// from main yet) still only appears once, and the overall most-recent-first
// order falls out of git's own traversal instead of needing a manual
// hash-set merge. `--source` (surfaced per-line via `%S`) reports which of
// the given refs a commit was reached through, which doubles as the "which
// branch is this commit on" label — including for shared/undiverged commits.
//
// Branches are passed to git log in a deterministic order — the current
// branch first, then the rest in LocalBranches' order — so which branch gets
// credited (via %S) for a commit reachable from more than one of them is
// deterministic: the current branch wins ties.
//
// A repository with no commits yet or no local branches (an unborn HEAD)
// returns an empty slice and a nil error, matching LocalBranches' own
// degrade. A non-repo or missing git binary returns ErrNotARepo.
func RecentCommits(root string, n int) ([]Commit, error) {
	if n <= 0 {
		return nil, nil
	}
	branches, err := LocalBranches(root)
	if err != nil {
		return nil, err
	}
	if len(branches) == 0 {
		return nil, nil
	}

	cur, _ := CurrentBranch(root) // "" (detached HEAD / error) leaves ordering as-is
	ordered := make([]string, 0, len(branches))
	if cur != "" {
		ordered = append(ordered, cur)
	}
	for _, b := range branches {
		if b == cur {
			continue
		}
		ordered = append(ordered, b)
	}

	format := strings.Join([]string{"%H", "%h", "%s", "%cr", "%S"}, recentCommitsFieldSep)
	args := append([]string{"log", "-n", strconv.Itoa(n), "--source", "--format=" + format}, ordered...)

	out, stderr, err := run(root, args...)
	if err != nil {
		if notARepo(stderr, err) {
			return nil, ErrNotARepo
		}
		return nil, wrapErr("git log", err, stderr)
	}

	var commits []Commit
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, recentCommitsFieldSep)
		if len(fields) != 5 {
			continue
		}
		commits = append(commits, Commit{
			Hash:      fields[0],
			ShortHash: fields[1],
			Subject:   fields[2],
			RelTime:   fields[3],
			Branch:    fields[4],
		})
	}
	return commits, nil
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

// verdictCommitPattern matches a commit subject following @reviewer's own
// convention (agents/reviewer.md): "[<jobID>] verdict: <summary>". Anchored
// at the start of the subject and exact on jobID (via regexp.QuoteMeta), so
// it never matches a different job's verdict commit or a message that merely
// mentions "verdict" elsewhere.
func verdictCommitPattern(jobID string) *regexp.Regexp {
	return regexp.MustCompile(`^\[` + regexp.QuoteMeta(jobID) + `\] verdict:`)
}

// CountVerdictCommits returns how many commits reachable from branch (short
// name from LocalBranches) have a subject matching verdictCommitPattern for
// jobID. Used by tui/internal/orchestrate (see the "fully autonomous mode"
// brief, Decision 2) to track the one-bounce retry budget: 0 or 1 such
// commits means a REJECTED/NEEDS WORK verdict may still bounce back to
// @developer once; 2 or more means that budget is exhausted.
//
// A commit message a human hand-edited to not follow the exact convention is
// simply not counted rather than causing a crash or a false positive — this
// function only ever errors on a real git failure, never on an unparseable
// message. A branch with no matching commits, or a branch/jobID that doesn't
// exist at all, returns (0, nil): mirrors ListJobDirs' "nothing here" degrade
// for an absent ref rather than surfacing it as an error, since a job branch
// that doesn't exist yet simply has no verdict commits from the caller's
// perspective. A non-repo / missing git binary returns (0, ErrNotARepo).
func CountVerdictCommits(root, branch, jobID string) (int, error) {
	out, stderr, err := run(root, "log", branch, "--format=%s")
	if err != nil {
		if notARepo(stderr, err) {
			return 0, ErrNotARepo
		}
		return 0, nil
	}
	pattern := verdictCommitPattern(jobID)
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if pattern.MatchString(line) {
			count++
		}
	}
	return count, nil
}

// LatestCommitIsVerdict reports whether the most recent commit on branch
// (the tip, per `git log -1`) is itself a verdict commit for jobID (see
// verdictCommitPattern / CountVerdictCommits).
//
// This resolves an ambiguity job.Stage() cannot: once a REJECTED/NEEDS WORK
// verdict.md exists, Stage() stays StageImplement until verdict.md's own
// content changes again — it does not distinguish "@reviewer just rejected,
// @developer hasn't responded yet" (the verdict commit is still the tip) from
// "@developer has already committed a fix since that verdict" (something
// newer sits on top of it). tui/internal/orchestrate uses this alongside
// CountVerdictCommits to tell the two apart without any new state file.
//
// A branch with no commits at all, or a branch/jobID that doesn't exist,
// returns (false, nil) — mirrors CountVerdictCommits' "nothing here" degrade.
// A non-repo / missing git binary returns (false, ErrNotARepo).
func LatestCommitIsVerdict(root, branch, jobID string) (bool, error) {
	out, stderr, err := run(root, "log", "-1", branch, "--format=%s")
	if err != nil {
		if notARepo(stderr, err) {
			return false, ErrNotARepo
		}
		return false, nil
	}
	subject := strings.TrimRight(string(out), "\n")
	if subject == "" {
		// An unborn/empty branch (should not normally happen for a real job
		// branch, but guard it the same as the other "nothing here" cases).
		return false, nil
	}
	return verdictCommitPattern(jobID).MatchString(subject), nil
}

// HeadCommit returns the full commit hash branch (short name from
// LocalBranches) currently points at, via `git rev-parse <branch>`.
//
// Used by mg-jdi's stall backstop (Decision 1a in the "fully autonomous
// mode" brief's tasks.md): if the same agent is invoked twice in a row for
// the same job with neither job.Stage() nor the branch HEAD having moved,
// the previous invocation made no persisted progress at all, so mg-jdi stops
// rather than looping indefinitely.
//
// A branch that doesn't exist, or an unborn repo with no commits yet,
// returns ("", nil) — mirrors the package's other "nothing here" degrades. A
// non-repo / missing git binary returns ("", ErrNotARepo).
func HeadCommit(root, branch string) (string, error) {
	out, stderr, err := run(root, "rev-parse", branch)
	if err != nil {
		if notARepo(stderr, err) {
			return "", ErrNotARepo
		}
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}
