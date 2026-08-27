package git

import "strings"

// Three-dot diff + log helpers behind `mg diff` (see docs/GIT_DIFF.md): the
// "see what has changed" view for a job branch, diffed against the project's
// base branch from the merge-base, so only what the branch itself did shows.
//
// Every helper takes the base and branch as separate short names and builds
// the three-dot range `<base>...<branch>` itself — the one notation
// GIT_DIFF.md's "The one thing that matters" insists on — and shares the
// package's degrade rules: a non-repo / missing git binary returns
// ErrNotARepo, a real git failure (e.g. a missing branch) returns the wrapped
// error including git's stderr. A range with nothing to show (an undiverged
// branch, or no commits yet) is an empty string with a nil error, not a
// failure — git exits 0 with no output for an empty three-dot range.
//
// The raw git stdout is returned with trailing newlines trimmed, so callers
// can embed it in their own output without double-blank-line artifacts; the
// leading-space column alignment of `git diff --stat` is preserved as git
// emits it.

// diffOut runs `git -C root <args>` and normalizes the result for the
// helpers below: the trimmed stdout on success, ErrNotARepo for the
// not-a-repo / git-missing case, and the wrapped error (with git's stderr)
// for any real git failure.
func diffOut(root string, args ...string) (string, error) {
	out, stderr, err := run(root, args...)
	if err != nil {
		if notARepo(stderr, err) {
			return "", ErrNotARepo
		}
		return "", wrapErr("git "+strings.Join(args, " "), err, stderr)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Diff returns the complete three-dot diff of everything branch changed
// relative to its merge-base with base — `git diff <base>...<branch>`, the
// full patch behind `mg diff --full`.
func Diff(root, base, branch string) (string, error) {
	return diffOut(root, "diff", base+"..."+branch)
}

// DiffStat returns the `git diff --stat <base>...<branch>` summary: files
// changed plus line counts, the "quick eyeball" files view of GIT_DIFF.md.
func DiffStat(root, base, branch string) (string, error) {
	return diffOut(root, "diff", "--stat", base+"..."+branch)
}

// DiffNameOnly returns `git diff --name-only <base>...<branch>`: just the
// filenames the branch changed, one per line.
func DiffNameOnly(root, base, branch string) (string, error) {
	return diffOut(root, "diff", "--name-only", base+"..."+branch)
}

// LogOneline returns `git log --oneline <base>...<branch>`: the branch's own
// commits, one short hash + subject per line — the "the commits themselves"
// half of GIT_DIFF.md's quick eyeball.
func LogOneline(root, base, branch string) (string, error) {
	return diffOut(root, "log", "--oneline", base+"..."+branch)
}
