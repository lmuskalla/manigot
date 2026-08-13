// Package git is a thin exec-backed wrapper over the git commands the manigot
// TUI needs for worktree-backed job discovery (see the "git worktrees" brief).
// It is the only place in the TUI that shells out to git, so the
// job/launch/ui packages ask about branches and worktrees through it rather
// than each shelling out ad-hoc.
//
// Every function degrades gracefully: a missing git binary, a directory that
// isn't a repository, or a missing branch returns a classified error
// (ErrNotARepo) or an empty result rather than crashing — mirroring how
// job.Discover already tolerates a missing docs/jobs.
//
// root is always passed to git via `git -C <root>`, so callers may hand over an
// absolute project root without worrying about the process's own cwd.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// JobsRelDir is where live jobs live, relative to the project root. Kept in
// sync with job.JobsRelDir (and the bash scripts); duplicated here so this
// package has no import cycle on job.
const JobsRelDir = "docs/jobs"

// ArchiveDirName is the subdirectory under JobsRelDir that holds finished
// jobs.
const ArchiveDirName = "archive"

// ErrNotARepo is returned when root is not inside a git repository or the git
// binary itself cannot be found. Callers that want to degrade gracefully
// (job.Discover's not-a-repo fallback to the working tree) test for it via
// errors.Is.
var ErrNotARepo = errors.New("not a git repository (or git not installed)")

// run executes `git -C root <args>` and returns raw stdout, raw stderr, and the
// exec error (nil on success). Callers interpret the result: the package-level
// helpers below normalize the not-a-repo / missing-path cases.
//
// run carries no context and thus no timeout — the interactive session and the
// user-driven mg done/mg delete paths wait on git as long as git needs. The
// non-interactive callers (the TUI's background push/commit cmds, mg-jdi's
// per-iteration probes) use the WithContext variants below instead, which
// bound the call via runCtx.
func run(root string, args ...string) ([]byte, string, error) {
	return runCtx(context.Background(), root, args...)
}

// runCtx is run's context-aware counterpart: `git -C root <args>` with ctx
// applied to the child process (exec.CommandContext), so a caller can bound a
// git call with a timeout. When the context fires, CommandContext kills the
// child but its exec error is a bare "signal: killed" — this returns the
// context's own error (context.DeadlineExceeded / context.Canceled) instead,
// so callers can errors.Is against it, and the timeout surfaces as an
// ordinary wrapped error (via wrapErr) — never a panic.
func runCtx(ctx context.Context, root string, args ...string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && ctx.Err() != nil {
		return out, stderr.String(), ctx.Err()
	}
	return out, stderr.String(), err
}

// runEnv is run's env-aware counterpart: it executes `git -C root <args>` with
// extraEnv appended to the child process's inherited environment. Kept
// separate from run (rather than adding a variadic env parameter there) so
// every existing call site — none of which need custom env — is untouched.
func runEnv(root string, extraEnv []string, args ...string) ([]byte, string, error) {
	return runEnvCtx(context.Background(), root, extraEnv, args...)
}

// runEnvCtx is runEnv's context-aware counterpart, mirroring runCtx's
// timeout semantics (the context's own error is returned when it fires).
func runEnvCtx(ctx context.Context, root string, extraEnv []string, args ...string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && ctx.Err() != nil {
		return out, stderr.String(), ctx.Err()
	}
	return out, stderr.String(), err
}

// notARepo reports whether a git failure is the not-a-repo or git-missing case
// (as opposed to a real, recoverable error like a missing path). The git binary
// being absent surfaces as exec.ErrNotFound; a real directory that isn't a
// repository surfaces via git's own "not a git repository" stderr. The match is
// case-insensitive: `git diff` reports "warning: Not a git repository" (capital
// N, exit 129) while the plumbing commands report "fatal: not a git repository"
// (lowercase, exit 128) — both are the same signal.
func notARepo(stderr string, err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(stderr), "not a git repository")
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

	args := append([]string{"log", "-n", strconv.Itoa(n), "--source", "--format=" + commitLogFormat()}, ordered...)
	return logCommits(root, args...)
}

// commitLogFormat is the --format argument RecentCommits and BranchCommits
// both pass to git log, so the two can never drift.
func commitLogFormat() string {
	return strings.Join([]string{"%H", "%h", "%s", "%cr", "%S"}, recentCommitsFieldSep)
}

// logCommits runs `git log` with the given args (everything after the leading
// "log" subcommand) and parses its output into Commit entries using the
// shared recentCommitsFieldSep format. It is the single parsing path behind
// both RecentCommits and BranchCommits, applying the package's degrade rules:
// a non-repo / missing git binary returns ErrNotARepo, any other git failure
// returns the wrapped error including git's stderr.
func logCommits(root string, args ...string) ([]Commit, error) {
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

// BranchCommits returns the last n commits reachable from a single local
// branch (short name from LocalBranches), ordered most-recent first — the
// per-branch counterpart of RecentCommits, used by the detail view's bottom
// git-log strip to show just one job's branch.
//
// It runs `git log -n <n> --source --format=<same format> <branch>` and
// parses through the same shared logCommits helper, so the line shape (and
// therefore the UI rendering) cannot drift from RecentCommits'.
//
// A branch that doesn't exist — or an unborn repo with no commits yet, where
// no ref under refs/heads/ can exist — returns an empty slice and a nil
// error, mirroring the package's other "nothing here" degrades; a caller
// rendering a strip treats that as "no log to show". A non-repo / missing git
// binary returns ErrNotARepo.
func BranchCommits(root, branch string, n int) ([]Commit, error) {
	if n <= 0 {
		return nil, nil
	}
	exists, err := RefExists(root, branch)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return logCommits(root, "log", "-n", strconv.Itoa(n), "--source", "--format="+commitLogFormat(), branch)
}

// Checkout switches the working tree at root to branch (short name), via
// `git checkout <branch>`. A checkout that git refuses (e.g. uncommitted
// changes that would be overwritten) returns the wrapped error including git's
// stderr so the caller can surface the reason in the status line.
//
// Currently unused: the worktree model removed its only
// two callers (the TUI's "switch to this job's branch" action and mg-jdi's
// ensureOnBranch), since a job's own worktree is always already on the job
// branch. Kept deliberately (with its tests) in case a branch-switch action is
// needed again.
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

// CommitFile stages path (relative to root) and commits it with message, via
// `git add` followed by `git commit -- path`. Used by the TUI's "e" edit
// action to auto-commit brief.md right after a successful edit, so a job's
// brief never lingers as an uncommitted change the way finish-job.sh's
// clean-tree check would otherwise reject.
//
// A save that left the file unchanged (nothing staged) is not an error —
// `git commit` on a clean pathspec exits non-zero with "nothing to commit" on
// stdout, which this treats as a successful no-op rather than surfacing an
// error to the user for simply opening and closing the editor. Any other
// commit failure returns the wrapped error including git's stderr.
func CommitFile(root, path, message string) error {
	return CommitFileWithContext(context.Background(), root, path, message)
}

// CommitFileWithContext is CommitFile with a caller-supplied context (see
// runCtx): used by the TUI's background auto-commit of brief.md after an
// edit, so a stalled git can't hang the app's command channel.
func CommitFileWithContext(ctx context.Context, root, path, message string) error {
	if _, stderr, err := runCtx(ctx, root, "add", "--", path); err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git add "+path, err, stderr)
	}
	out, stderr, err := runCtx(ctx, root, "commit", "-m", message, "--", path)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		if strings.Contains(string(out), "nothing to commit") {
			return nil
		}
		return wrapErr("git commit "+path, err, stderr)
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
// jobID. Used by internal/orchestrate to track the one-bounce retry
// budget: 0 or 1 such
// commits means a REJECTED/NEEDS WORK verdict may still bounce back to
// @developer once; 2 or more means that budget is exhausted.
//
// A commit message a human hand-edited to not follow the exact convention is
// simply not counted rather than causing a crash or a false positive — this
// function only ever errors on a real git failure, never on an unparseable
// message. A branch with no matching commits, or a branch/jobID that doesn't
// exist at all, returns (0, nil): mirrors the other "nothing here" degrades
// for an absent ref rather than surfacing it as an error, since a job branch
// that doesn't exist yet simply has no verdict commits from the caller's
// perspective. A non-repo / missing git binary returns (0, ErrNotARepo).
func CountVerdictCommits(root, branch, jobID string) (int, error) {
	return CountVerdictCommitsWithContext(context.Background(), root, branch, jobID)
}

// CountVerdictCommitsWithContext is CountVerdictCommits with a caller-supplied
// context (see runCtx): used by mg-jdi's per-iteration probe, which must not
// stall the whole run on a hung git.
func CountVerdictCommitsWithContext(ctx context.Context, root, branch, jobID string) (int, error) {
	out, stderr, err := runCtx(ctx, root, "log", branch, "--format=%s")
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
// newer sits on top of it). internal/orchestrate uses this alongside
// CountVerdictCommits to tell the two apart without any new state file.
//
// A branch with no commits at all, or a branch/jobID that doesn't exist,
// returns (false, nil) — mirrors CountVerdictCommits' "nothing here" degrade.
// A non-repo / missing git binary returns (false, ErrNotARepo).
func LatestCommitIsVerdict(root, branch, jobID string) (bool, error) {
	return LatestCommitIsVerdictWithContext(context.Background(), root, branch, jobID)
}

// LatestCommitIsVerdictWithContext is LatestCommitIsVerdict with a
// caller-supplied context (see runCtx): used by mg-jdi's per-iteration probe.
func LatestCommitIsVerdictWithContext(ctx context.Context, root, branch, jobID string) (bool, error) {
	out, stderr, err := runCtx(ctx, root, "log", "-1", branch, "--format=%s")
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
// Used by mg-jdi's stall backstop: if the same agent is invoked twice in a
// row for
// the same job with neither job.Stage() nor the branch HEAD having moved,
// the previous invocation made no persisted progress at all, so mg-jdi stops
// rather than looping indefinitely.
//
// A branch that doesn't exist, or an unborn repo with no commits yet,
// returns ("", nil) — mirrors the package's other "nothing here" degrades. A
// non-repo / missing git binary returns ("", ErrNotARepo).
func HeadCommit(root, branch string) (string, error) {
	return HeadCommitWithContext(context.Background(), root, branch)
}

// HeadCommitWithContext is HeadCommit with a caller-supplied context (see
// runCtx): used by mg-jdi's per-iteration stall-probe.
func HeadCommitWithContext(ctx context.Context, root, branch string) (string, error) {
	out, stderr, err := runCtx(ctx, root, "rev-parse", branch)
	if err != nil {
		if notARepo(stderr, err) {
			return "", ErrNotARepo
		}
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// WorktreeForBranch returns the absolute path of the worktree currently
// checked out to branch (short name from LocalBranches) in the repository at
// root, via `git worktree list --porcelain` — the Go-side equivalent of the
// old worktree.sh helper. ok is false when no worktree has that
// branch checked out (including a bare or detached-HEAD entry, which has no
// "branch" line at all) — not itself an error, since "this branch has no
// worktree yet" is a normal, expected answer (e.g. a branch mg job hasn't
// been used to create one for). A non-repo /
// missing git binary returns ("", false, ErrNotARepo).
//
// Matched against the full ref (refs/heads/<branch>), not a bare-name
// substring, so a branch name that is a prefix of another (e.g. "feature/x"
// vs. "feature/x-y") can never cross-match — mirroring the bash helper's own
// exact-ref comparison.
func WorktreeForBranch(root, branch string) (path string, ok bool, err error) {
	out, stderr, err := run(root, "worktree", "list", "--porcelain")
	if err != nil {
		if notARepo(stderr, err) {
			return "", false, ErrNotARepo
		}
		return "", false, wrapErr("git worktree list", err, stderr)
	}

	matchRef := "refs/heads/" + branch
	var current string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			if strings.TrimPrefix(line, "branch ") == matchRef {
				return current, true, nil
			}
		case line == "":
			// Blank line: end of this worktree's block.
			current = ""
		}
	}
	return "", false, nil
}

// WorktreeGitDirs returns the absolute gitdir paths of every linked worktree
// in the repository at root, excluding the worktree at currentPath (the
// caller's own worktree — its gitdir must stay writable for commits) and the
// main worktree (whose gitdir is the common dir itself, already mounted by
// the session launcher). Derived from `git worktree list --porcelain` — the
// same listing WorktreeForBranch parses — plus a per-worktree
// `git rev-parse --git-dir` to resolve each linked worktree's gitdir (the
// porcelain format carries the working-tree path, not the gitdir).
//
// A worktree whose gitdir cannot be resolved (e.g. a prunable entry whose
// working tree was deleted, or a missing path) is skipped, not an error —
// the session launcher must skip missing sources anyway (docker would
// otherwise create an empty, root-owned directory at the mount target). A
// non-repo / missing git binary returns ErrNotARepo.
func WorktreeGitDirs(root, currentPath string) ([]string, error) {
	out, stderr, err := run(root, "worktree", "list", "--porcelain")
	if err != nil {
		if notARepo(stderr, err) {
			return nil, ErrNotARepo
		}
		return nil, wrapErr("git worktree list", err, stderr)
	}

	currentPath = filepath.Clean(currentPath)
	commonDir := GitCommonDir(root)

	var dirs []string
	var current string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			// A new block: resolve the previous worktree's gitdir first.
			if current != "" {
				if dir := linkedWorktreeGitDir(current, currentPath, commonDir); dir != "" {
					dirs = append(dirs, dir)
				}
			}
			current = strings.TrimPrefix(line, "worktree ")
		}
	}
	// The porcelain output's last block may not end with a blank line.
	if current != "" {
		if dir := linkedWorktreeGitDir(current, currentPath, commonDir); dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

// linkedWorktreeGitDir resolves the gitdir for one worktree path from a
// porcelain listing: "" when it is the caller's own worktree, the main
// worktree (its gitdir is the common dir itself), or a worktree whose gitdir
// cannot be resolved (missing/prunable — a source that must be skipped).
func linkedWorktreeGitDir(path, currentPath, commonDir string) string {
	if filepath.Clean(path) == currentPath {
		return ""
	}
	dir := gitDir(path)
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	if commonDir != "" && dir == filepath.Clean(commonDir) {
		return ""
	}
	return dir
}

// gitDir resolves the repository's git dir for the repo at root — the
// worktree-specific gitdir for a linked worktree, the common git dir for the
// main one — via `git rev-parse --path-format=absolute --git-dir`, with the
// pre-2.31 fallback (a relative path, joined against root). "" when it cannot
// be determined.
func gitDir(root string) string {
	out, _, err := run(root, "rev-parse", "--path-format=absolute", "--git-dir")
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	out, _, err = run(root, "rev-parse", "--git-dir")
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir)
}

// Push pushes branch (short name from LocalBranches) to origin, via
// `git push -u origin <branch>`. It is the host-side mutation behind the
// TUI detail view's "P" push-to-origin action: a quick way to make a job's
// branch visible on another host via a plain git push/pull, without needing
// that branch checked out in the working tree first (unlike Checkout,
// CommitFile and the other mutating actions, Push operates on the named
// branch ref directly).
//
// The -u sets upstream tracking on first push; a later push against an
// already-tracking branch is then a plain fast-forward push. This never
// force-pushes, so a diverged remote (someone else pushed to the same branch
// in the meantime) surfaces as a normal rejected-push error rather than
// silently overwriting it.
//
// GIT_TERMINAL_PROMPT=0 is set on the child process so a missing/invalid
// credential never blocks on an interactive prompt the TUI has no terminal
// to render into — that failure mode instead surfaces as a wrapped error,
// same as a missing origin remote or a rejected push.
func Push(root, branch string) error {
	return PushWithContext(context.Background(), root, branch)
}

// PushWithContext is Push with a caller-supplied context (see runCtx): used
// by the TUI's background push cmd, which must not hang the app's command
// channel on a stalled network forever.
func PushWithContext(ctx context.Context, root, branch string) error {
	_, stderr, err := runEnvCtx(ctx, root, []string{"GIT_TERMINAL_PROMPT=0"}, "push", "-u", "origin", branch)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git push origin "+branch, err, stderr)
	}
	return nil
}

// --- Job-lifecycle operations (ported from scripts/new-job.sh,
// scripts/finish-job.sh and scripts/delete-job.sh) ---------------------------
//
// Every function below is exec-backed like the helpers above and shares their
// degrade rules: a non-repo / missing git binary returns ErrNotARepo, a real
// git failure returns the wrapped error including git's stderr.

// WorktreeAdd creates a new worktree at path (absolute) with a new branch
// branch (short name) branched off base (short name), via
// `git worktree add <path> -b <branch> <base>` — the exact invocation
// scripts/new-job.sh used to create a job's own worktree.
func WorktreeAdd(root, path, branch, base string) error {
	_, stderr, err := run(root, "worktree", "add", path, "-b", branch, base)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git worktree add "+branch, err, stderr)
	}
	return nil
}

// WorktreeRemove removes the worktree at path, via `git worktree remove <path>`.
// The worktree must be clean (no uncommitted changes); use WorktreeRemoveForce
// when a dirty worktree is an acceptable discard.
func WorktreeRemove(root, path string) error {
	_, stderr, err := run(root, "worktree", "remove", path)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git worktree remove "+path, err, stderr)
	}
	return nil
}

// WorktreeRemoveForce removes the worktree at path, discarding any uncommitted
// changes, via `git worktree remove --force <path>`.
func WorktreeRemoveForce(root, path string) error {
	_, stderr, err := run(root, "worktree", "remove", "--force", path)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git worktree remove --force "+path, err, stderr)
	}
	return nil
}

// WorktreePrune prunes stale worktree metadata via `git worktree prune`. It is
// a best-effort cleanup (the scripts ran it with `|| true`) and always returns
// nil, mirroring that intent.
func WorktreePrune(root string) error {
	_, _, _ = run(root, "worktree", "prune")
	return nil
}

// BranchDelete force-deletes branch (short name) via `git branch -D <branch>`
// — no merge, matching the destructive semantics of finish-job.sh's final step
// and delete-job.sh's branch delete. The branch must not be checked out in any
// worktree (git refuses); callers remove the worktree first.
func BranchDelete(root, branch string) error {
	_, stderr, err := run(root, "branch", "-D", branch)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git branch -D "+branch, err, stderr)
	}
	return nil
}

// SquashMerge stages the squashed changes of branch (short name) into root's
// index via `git merge --squash <branch>`, without committing. The caller
// commits the result with CommitStaged — together the two replace
// finish-job.sh's `git merge --squash` + `git commit -m` pair, which folds the
// whole job branch into a single commit on the base branch.
func SquashMerge(root, branch string) error {
	_, stderr, err := run(root, "merge", "--squash", branch)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git merge --squash "+branch, err, stderr)
	}
	return nil
}

// CommitStaged creates a commit with the staged changes and message in root,
// via `git commit -m <message>`. A multi-line message (subject\n\nbody, the
// finish-job.sh "<title>\n\nJob: <name>" shape) is passed as one argument and
// git splits subject/body itself, exactly as the shell did.
func CommitStaged(root, message string) error {
	_, stderr, err := run(root, "commit", "-m", message)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git commit", err, stderr)
	}
	return nil
}

// Stage stages the pathspec (relative to root, or absolute) into the index of
// the repository at root via `git add -- <pathspec>` — the Go form of
// new-job.sh's `git -C <job-dir> add .` and finish-job.sh's
// `git -C <worktree> add <worktree>/docs/jobs` (both plain `git add`, no -A:
// a fresh job directory has nothing to delete).
func Stage(root, pathspec string) error {
	_, stderr, err := run(root, "add", "--", pathspec)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		return wrapErr("git add "+pathspec, err, stderr)
	}
	return nil
}

// WorkingTreeDirty reports whether root has uncommitted changes — unstaged
// tracked modifications or staged changes — via `git diff --quiet` and
// `git diff --cached --quiet`, the exact pair finish-job.sh and delete-job.sh
// used for their clean-tree checks. Untracked files are not reported (git diff
// ignores them), matching the scripts. A non-repo returns ErrNotARepo.
func WorkingTreeDirty(root string) (bool, error) {
	dirty := false
	for _, args := range [][]string{{"diff", "--quiet"}, {"diff", "--cached", "--quiet"}} {
		_, stderr, err := run(root, args...)
		if err != nil {
			if notARepo(stderr, err) {
				return false, ErrNotARepo
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				// Exit 1 from `git diff --quiet` is the "differences exist"
				// signal, not a failure.
				dirty = true
				continue
			}
			return false, wrapErr("git "+strings.Join(args, " "), err, stderr)
		}
	}
	return dirty, nil
}

// SymbolicRefHead resolves origin/HEAD's target branch short name via
// `git symbolic-ref refs/remotes/origin/HEAD`, falling back to "main" when
// there is no origin/HEAD or it cannot be read — finish-job.sh's
// `git symbolic-ref ... | sed 's@^refs/remotes/origin/@@' || echo "main"`
// resolution of the default branch a project integrates on.
func SymbolicRefHead(root string) string {
	out, _, err := run(root, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "main"
	}
	ref := strings.TrimSpace(string(out))
	ref = strings.TrimPrefix(ref, "refs/remotes/origin/")
	if ref == "" {
		return "main"
	}
	return ref
}

// RefExists reports whether ref (short name, e.g. "feature/x") exists as a
// local branch in the repository at root, via
// `git rev-parse --verify --quiet refs/heads/<ref>` — the namespace-collision
// pre-check of new-job.sh (a plain branch "feature" blocks the whole
// "feature/..." namespace in git, so job creation checks every ancestor path
// segment before creating the branch). A ref that doesn't exist is not an
// error: it returns (false, nil).
func RefExists(root, ref string) (bool, error) {
	_, stderr, err := run(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+ref)
	if err == nil {
		return true, nil
	}
	if notARepo(stderr, err) {
		return false, ErrNotARepo
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// --quiet suppresses output; a non-zero exit (typically 1) means the
		// ref does not exist.
		return false, nil
	}
	return false, wrapErr("git rev-parse --verify "+ref, err, stderr)
}

// GitCommonDir returns the repository's common git dir for the worktree at
// root, via `git rev-parse --path-format=absolute --git-common-dir` with the
// pre-2.31 fallback (a relative path, joined against root). For a job
// worktree this is the main repo's .git directory — the path the session
// launcher mounts into the container so a worktree's `.git` pointer file
// resolves there. "" when it cannot be determined.
func GitCommonDir(root string) string {
	out, _, err := run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	// Pre-2.31 git: --git-common-dir without --path-format returns a path
	// relative to the caller's cwd.
	out, _, err = run(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir)
}

// GitPath resolves a git-internal path (e.g. "info/exclude") for the
// repository at root via `git rev-parse --path-format=absolute --git-path
// <path>`, with the pre-2.31 fallback that returns a path relative to root.
// "" when it cannot be determined. Used to locate the main worktree's
// .git/info/exclude when nested job worktrees must be excluded from its status.
func GitPath(root, path string) string {
	out, _, err := run(root, "rev-parse", "--path-format=absolute", "--git-path", path)
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	out, _, err = run(root, "rev-parse", "--git-path", path)
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir)
}

// ExcludePath appends pattern to the repository's .git/info/exclude file
// (creating the file if missing), idempotently — an already-present line is
// left alone. This is the Go form of new-job.sh's exclude-file block, which
// keeps nested job worktrees out of the main worktree's `git add -A`.
// An absent or unreadable file is created, not an error.
func ExcludePath(root, pattern string) error {
	path := GitPath(root, "info/exclude")
	if path == "" {
		// Git couldn't resolve the file at all — ErrNotARepo covers the
		// not-a-repo case; anything else is opaque.
		_, _, err := run(root, "rev-parse", "--git-dir")
		if err != nil {
			return ErrNotARepo
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(pattern + "\n")
	return err
}

// mountTargetExcludePatterns are the repo-relative paths manigot's container
// docs mounts collide with: /workspace/.opencode for OpenCode profiles and
// /workspace/.claude for Claude Code (see internal/session/docker.go's
// docsMountTarget). Inside the container those mount targets make the job
// reachable at a second path that looks like a different copy of docs/, and
// git — which has no notion of mount points — happily tracks whatever an
// agent stages through them, leaving a stale duplicate of the job in the
// repository that later trips FinishJob's clean-tree check (see
// docs/BUG_report-mg-done-dirty-worktree-stale-job-copy.md). The paths must
// therefore never be tracked.
var mountTargetExcludePatterns = []string{".opencode/", ".claude/"}

// ExcludeMountTargets appends every container docs-mount target path
// (.opencode/, .claude/) to the repository's .git/info/exclude via
// ExcludePath, so git never tracks the mounted docs under the colliding
// repo-relative path: `git add .` skips it, and an explicit
// `git add .opencode/...` fails loudly ("paths are ignored") instead of
// silently creating the duplicate. info/exclude lives in the repository's
// common git dir and is shared by every worktree, so one call protects the
// main worktree and all job worktrees alike; ExcludePath's idempotency keeps
// repeated calls (job creation plus every session launch) from duplicating
// lines. A non-repo root is not an error — there is no git tracking to
// protect (ErrNotARepo is absorbed, matching the best-effort nature of the
// call sites).
func ExcludeMountTargets(root string) error {
	for _, pattern := range mountTargetExcludePatterns {
		if err := ExcludePath(root, pattern); err != nil && !errors.Is(err, ErrNotARepo) {
			return err
		}
	}
	return nil
}

// RevParseToplevel returns the absolute top-level working-tree path of the
// repository at root, via `git rev-parse --show-toplevel` — the
// MAIN_WORKTREE computation of finish-job.sh and delete-job.sh, used to detect
// the pre-worktree case where a job's branch is checked out in the main
// worktree itself (which cannot be removed).
func RevParseToplevel(root string) (string, error) {
	out, stderr, err := run(root, "rev-parse", "--show-toplevel")
	if err != nil {
		if notARepo(stderr, err) {
			return "", ErrNotARepo
		}
		return "", wrapErr("git rev-parse --show-toplevel", err, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}

// ConfigUserName returns the repository's configured user.name ("" when unset
// or not a repo) — the AUTHOR source of new-job.sh (`git config user.name`
// with its `|| echo "unknown"` fallback applied by the caller).
func ConfigUserName(root string) string {
	return configValue(root, "user.name")
}

// ConfigEmail returns the repository's configured user.email ("" when unset or
// not a repo) — the project-side git identity the session launcher forwards
// into the container when no GIT_AUTHOR_EMAIL env var is set.
func ConfigEmail(root string) string {
	return configValue(root, "user.email")
}

// configValue returns `git -C root config <key>` ("" when unset or not a
// repo).
func configValue(root, key string) string {
	out, _, err := run(root, "config", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
