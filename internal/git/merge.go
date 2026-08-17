package git

import (
	"context"
	"strings"
)

// MergeBranch merges branch (short name from LocalBranches) into the
// currently checked-out branch of the worktree at root, via
// `git merge --no-edit <branch>`. It is the host-side mutation behind the
// TUI detail view's "merge default branch" action: a way to bring a job's
// worktree up to speed with the project's base branch before starting work,
// without leaving the job branch.
//
// `--no-edit` accepts git's auto-generated merge-commit message, so a
// diverged merge that needs a merge commit never hangs on an editor prompt
// the TUI has no terminal to render into. GIT_TERMINAL_PROMPT=0 is set on
// the child process (same as Push) so a missing credential can't block on an
// interactive prompt either — that failure mode surfaces as a wrapped error.
// The merge targets the worktree's currently checked-out branch, so the
// caller must run it inside the job's own worktree (the TUI derives that
// root the same way commitAllCmd does).
//
// Outcomes: an up-to-date worktree is a successful no-op ("Already up to
// date."); a dirty worktree whose local changes would be overwritten is
// refused by git and surfaces as a wrapped error; a merge conflict leaves the
// tree in the conflicted state git put it in and also returns a wrapped
// error (the user resolves conflicts manually — MergeBranch never aborts or
// resolves). A non-repo / missing git binary returns ErrNotARepo; any other
// failure returns the wrapped error including git's stderr — and git's
// stdout too, when git reports the failure there.
func MergeBranch(root, branch string) error {
	return MergeBranchWithContext(context.Background(), root, branch)
}

// MergeBranchWithContext is MergeBranch with a caller-supplied context (see
// runCtx): used by the TUI's background merge cmd, so a stalled git can't
// hang the app's command channel.
func MergeBranchWithContext(ctx context.Context, root, branch string) error {
	out, stderr, err := runEnvCtx(ctx, root, []string{"GIT_TERMINAL_PROMPT=0"}, "merge", "--no-edit", branch)
	if err != nil {
		if notARepo(stderr, err) {
			return ErrNotARepo
		}
		// A failed merge reports its detail on stdout on some git versions
		// (e.g. the "CONFLICT (content): ..." lines of a conflicted merge,
		// with stderr left empty) and on stderr on others — include whichever
		// is non-empty (both, when both are), so the caller always sees why
		// the merge failed instead of a bare exit status.
		msg := strings.TrimSpace(stderr)
		if o := strings.TrimSpace(string(out)); o != "" {
			if msg != "" {
				msg += "\n" + o
			} else {
				msg = o
			}
		}
		return wrapErr("git merge "+branch, err, msg)
	}
	return nil
}
