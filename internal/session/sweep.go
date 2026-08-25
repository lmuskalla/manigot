package session

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lmuskalla/manigot/internal/git"
)

// SweepJobWorktree commits whatever an agent session left uncommitted in a
// job worktree, after the container run has returned. It is the host-side
// guarantee behind "every agent always commits": agents don't reliably follow
// commit instructions, and the read-only agents — the analyst above all,
// whose tasks.md this commits — physically cannot commit at all (`commit:
// false` → read-only git mount), so the host sweeps the worktree after every
// job-worktree session and mg done's clean-tree check stops being tripped by
// agent leftovers.
//
// It is a no-op for a plain (non-job) session — root.Job == "" — where the
// user's own uncommitted work lives, and for any --job resolution that never
// left the main worktree — root.ProjectRoot == root.InvocationRoot — which
// covers two distinct shapes: the flat-scan fallback (a git repo with no
// local branches, or a non-git project: the job's files live directly in the
// main project root) and a pre-worktree job (the job's branch is checked out
// in the main worktree itself, an explicitly supported transitional state —
// see job.Discover / FinishJob's main-worktree handling —
// where git.WorktreeForBranch resolves to the main worktree, not a linked
// one). Both would otherwise commit the user's own uncommitted work —
// including an unexcluded .env — onto the job branch; a linked job worktree's
// ProjectRoot always differs from InvocationRoot, so the comparison is a
// precise worktree gate in every resolution shape.
// A clean worktree is not an error either (ErrNothingToCommit is swallowed),
// and a broken-worktree CommitAll failure of the ErrNotARepo kind is
// swallowed too. Any other failure warns on diag — stderr — and never aborts
// the caller.
//
// The sweep commit reuses the TUI "c" key's message convention
// ([<id>] chore: commit all), which deliberately does not match the
// verdict-commit pattern ([<id>] verdict: ...) that mg-jdi's retry budget and
// re-review decisions count (git.CountVerdictCommits /
// git.LatestCommitIsVerdict), so the state machine is unaffected. Because the
// sweep runs inside the session path — before mg-jdi's post-run stall probe
// reads HEAD — the sweep commit counts as agent progress there.
func SweepJobWorktree(root Root, diag io.Writer) {
	// Job-worktree sessions only. root.Job alone is not enough: the --job
	// flat-scan fallback and a pre-worktree job (branch checked out in the
	// main worktree itself) both set Job while ProjectRoot never left the
	// MAIN project root, where the user's own uncommitted work lives;
	// sweeping there would commit it — .env included — as a commit on the job
	// branch (or the repo's first commit, in the flat-scan case). A linked
	// job worktree's ProjectRoot always differs from InvocationRoot, so that
	// comparison is the worktree gate in every resolution shape.
	if root.Job == "" || root.ProjectRoot == root.InvocationRoot {
		return
	}
	id := jobIDFromName(root.Job)
	msg := fmt.Sprintf("[%s] chore: commit all", id)
	if err := git.CommitAll(root.ProjectRoot, msg); err != nil {
		switch {
		case errors.Is(err, git.ErrNothingToCommit), errors.Is(err, git.ErrNotARepo):
			// Clean worktree or a non-git job — nothing to sweep.
		default:
			fmt.Fprintf(diag, "mg: warning: could not commit leftover changes in %s: %v\n", root.ProjectRoot, err)
		}
		return
	}
	fmt.Fprintf(diag, "mg: committed leftover changes in %s (%s).\n", root.ProjectRoot, msg)
}

// jobIDFromName derives the job id from a job name (<id>_<slug>): everything
// before the first underscore — the slug part may itself contain underscores,
// so the split is on the first one only. Falls back to the whole name when
// there is no underscore, or when the id part is empty.
func jobIDFromName(name string) string {
	id, _, _ := strings.Cut(name, "_")
	if id == "" {
		return name
	}
	return id
}
