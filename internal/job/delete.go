package job

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/fs"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/project"
)

// DeleteResult is the outcome of a successful DeleteJob.
type DeleteResult struct {
	// JobName is the job's directory name (the id_slug).
	JobName string

	// Branch is the deleted job branch ("" for a non-git project, which has
	// no branch).
	Branch string

	// Dir is the removed job directory.
	Dir string
}

// DeleteJob ports scripts/delete-job.sh — the permanent job deletion lifecycle:
// resolve the job (branch + worktree for a git project, a plain docs/jobs
// directory otherwise), show the script's confirmation (with the dirty-worktree
// warning wording), then remove the worktree (--force) and force-delete the
// branch. A non-git project is a plain directory delete.
//
// root is the project root, resolved by the caller (the CLI's docs-walk-up,
// or the TUI's a.root). confirm answers the script's `read -rp` prompts (a
// declined answer returns ErrCancelled, the script's `exit 0`);
// informational output goes to out.
func DeleteJob(root, jobArg string, confirm ConfirmFunc, out io.Writer) (DeleteResult, error) {

	// ── Non-git project: plain directory delete, no git involved ──────────
	if _, err := git.LocalBranches(root); err != nil {
		if errors.Is(err, git.ErrNotARepo) {
			return deleteNonGit(root, jobArg, confirm, out)
		}
		return DeleteResult{}, err
	}

	// ── Resolve the job's branch + worktree ────────────────────────────────
	branches, berr := git.LocalBranches(root)
	if berr != nil {
		return DeleteResult{}, berr
	}
	branch := git.ExactBranchMatch(branches, jobArg)
	if branch == "" {
		prefixMatches := git.PrefixBranchMatches(branches, jobArg)
		switch len(prefixMatches) {
		case 0:
			return DeleteResult{}, jobNotFoundError(jobArg, branches)
		case 1:
			branch = prefixMatches[0]
		default:
			return DeleteResult{}, fmt.Errorf("job '%s' is ambiguous — matches branches: %s", jobArg, strings.Join(prefixMatches, " "))
		}
	}
	jobName := git.BranchTail(branch)

	wtPath, ok, werr := git.WorktreeForBranch(root, branch)
	if werr != nil {
		return DeleteResult{}, werr
	}
	if !ok {
		return DeleteResult{}, fmt.Errorf("branch '%s' has no git worktree — cannot delete job '%s'.\nA job's worktree is created by 'mg job' and should always exist for an open job; this is an inconsistent state.", branch, jobName)
	}

	jobDir := filepath.Join(wtPath, JobsRelDir, jobName)
	jobTitle := briefTitle(filepath.Join(jobDir, "brief.md"))

	// Is the resolved worktree the main worktree itself? Needed for the
	// confirmation wording and to decide whether worktree removal is even
	// possible (the main worktree cannot be removed).
	mainWorktree, terr := git.RevParseToplevel(root)
	if terr != nil {
		mainWorktree = root
	}
	mainWorktreeCase := filepath.Clean(wtPath) == filepath.Clean(mainWorktree)

	// A delete discards the worktree wholesale — surface a dirty tree in the
	// confirmation rather than erroring (unlike mg done, there is no "commit
	// first" requirement for a delete).
	dirty, derr := git.WorkingTreeDirty(wtPath)
	if derr != nil {
		return DeleteResult{}, derr
	}

	// ── Confirm ────────────────────────────────────────────────────────────
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "This will permanently delete job: %s\n", jobName)
	fmt.Fprintf(out, "  Title    : %s\n", titleOrName(jobTitle, jobName))
	fmt.Fprintf(out, "  Worktree : %s\n", wtPath)
	fmt.Fprintf(out, "  Branch   : %s (will be deleted, unmerged)\n", branch)
	if dirty {
		if mainWorktreeCase {
			fmt.Fprintln(out, "  Warning  : the main worktree has uncommitted changes — the switch off "+branch+" will carry")
			fmt.Fprintln(out, "             them onto the default branch if they don't conflict, or abort (deleting nothing).")
		} else {
			fmt.Fprintln(out, "  Warning  : this worktree has uncommitted changes — they will be discarded.")
		}
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "This cannot be undone.")
	if err := askConfirm(confirm, out, "  Proceed? [y/N] "); err != nil {
		return DeleteResult{}, err
	}

	// ── Resolve the base branch the main worktree is switched onto ─────────
	settings, err := project.Load(root)
	if err != nil {
		return DeleteResult{}, err
	}
	baseBranch := settings.BaseBranch
	if baseBranch == "" {
		baseBranch = git.SymbolicRefHead(root)
	}

	// The main worktree must not sit on the branch being deleted — switching
	// it off is what makes the branch deletable (and is exactly the
	// pre-worktree-job case, where there is no separate worktree to remove).
	currentMain, cerr := git.CurrentBranch(root)
	if cerr != nil {
		return DeleteResult{}, cerr
	}
	if currentMain == branch {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "→ Switching the main worktree off %s...\n", branch)
		if err := git.Checkout(root, baseBranch); err != nil {
			return DeleteResult{}, err
		}
	}

	// ── Remove the worktree, then delete the branch ────────────────────────
	if mainWorktreeCase {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "→ Job's worktree is the main worktree — skipping worktree removal.")
	} else {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "→ Removing worktree %s (and everything committed on %s)...\n", wtPath, branch)
		if err := git.WorktreeRemoveForce(root, wtPath); err != nil {
			return DeleteResult{}, err
		}
		_ = git.WorktreePrune(root)
	}

	fmt.Fprintf(out, "→ Deleting branch %s...\n", branch)
	if err := git.BranchDelete(root, branch); err != nil {
		return DeleteResult{}, err
	}

	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "✓ Job deleted: %s\n", jobName)
	fmt.Fprintf(out, "  Branch removed: %s\n", branch)
	return DeleteResult{JobName: jobName, Branch: branch, Dir: jobDir}, nil
}

// deleteNonGit handles the non-git project path: resolve the job directory
// (exact then prefix, excluding archive/), confirm, and remove it — a plain
// directory delete, no git involved.
func deleteNonGit(root, jobArg string, confirm ConfirmFunc, out io.Writer) (DeleteResult, error) {
	var jobDir, jobName string
	if fs.IsDir(filepath.Join(root, JobsRelDir, jobArg)) {
		jobDir = filepath.Join(root, JobsRelDir, jobArg)
		jobName = jobArg
	} else {
		match := PrefixJobDirName(root, jobArg)
		if match == "" {
			return DeleteResult{}, &jobNotFoundErr{msg: fmt.Sprintf("job '%s' not found under %s/", jobArg, JobsRelDir)}
		}
		jobDir = filepath.Join(root, JobsRelDir, match)
		jobName = match
	}

	jobTitle := briefTitle(filepath.Join(jobDir, "brief.md"))
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "This will permanently delete job: %s\n", jobName)
	fmt.Fprintf(out, "  Title  : %s\n", titleOrName(jobTitle, jobName))
	fmt.Fprintf(out, "  Dir    : %s/%s\n", JobsRelDir, jobName)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "This cannot be undone.")
	if err := askConfirm(confirm, out, "  Proceed? [y/N] "); err != nil {
		return DeleteResult{}, err
	}
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "→ Removing %s/%s...\n", JobsRelDir, jobName)
	if err := os.RemoveAll(jobDir); err != nil {
		return DeleteResult{}, err
	}
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "✓ Job deleted: %s\n", jobName)
	return DeleteResult{JobName: jobName, Dir: jobDir}, nil
}

// titleOrName returns title when non-empty, else name — the script's
// `${JOB_TITLE:-$JOB_NAME}` default.
func titleOrName(title, name string) string {
	if title == "" {
		return name
	}
	return title
}
