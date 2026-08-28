package job

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/launch"
	"github.com/lmuskalla/manigot/internal/project"
)

// ErrCancelled is returned by FinishJob and DeleteJob when the user declines a
// confirmation prompt — the scripts' `exit 0` on a declined `[y/N]` answer.
// The CLI treats it as a clean exit with no further output.
var ErrCancelled = errors.New("cancelled")

// ErrJobNotFound is returned by FinishJob, DeleteJob and the orphan matchers
// when the given id resolves to no job at all. The CLI checks errors.Is on it
// to distinguish "there is no such job, try the orphaned-worktree path" from a
// real failure.
var ErrJobNotFound = errors.New("job not found")

// ErrGitSolverHandoff is returned by FinishJob when the user accepts the offer
// to hand a failed squash merge to @git-solver: the git-solver session was
// started (detached, via GitSolverLaunch) to resolve the conflict and finish
// the cleanup, so `mg done` is done for now — the CLI treats it as a clean
// exit, not an error.
var ErrGitSolverHandoff = errors.New("handed off to @git-solver")

// ErrSquashMergeConflict is returned by FinishJobWithOptions when
// FinishOptions.NoConflictRecovery is set and the squash merge conflicts: no
// automatic recovery is attempted (neither the @git-solver offer nor the
// fallback rollback — see FinishOptions's own doc for why), so the main
// worktree is left in its conflicted-merge state and the caller must resolve
// it — by hand, or by triggering @git-solver through a separate call —
// before the job can be finished. The job itself is already
// archived-and-committed in its own worktree by the time this can happen
// (that step runs before the merge attempt), which cannot be undone without a
// much larger restructuring than this — a caller must not assume "nothing
// happened" from this error alone.
var ErrSquashMergeConflict = errors.New("squash merge conflict — no automatic recovery attempted")

// FinishOptions carries FinishJob's optional behavior knobs, kept separate
// from its original positional parameters so every existing caller (the
// CLI's `mg done`, the TUI — both via FinishJob itself) keeps its current
// confirm-based behavior byte-for-byte unchanged; only a caller that opts in
// via FinishJobWithOptions sees the new behavior.
type FinishOptions struct {
	// NoConflictRecovery, when true, makes a squash-merge conflict return
	// ErrSquashMergeConflict immediately instead of taking FinishJob's
	// default interactive prompt-then-recover path (offer @git-solver, or
	// roll the main worktree back on decline). Intended for a caller with no
	// human able to answer that interactive prompt (e.g. the HTTP daemon) —
	// per the product decision recorded in this job's brief: report a
	// structured error and leave things as-is, requiring an explicit
	// follow-up decision through some other call, rather than silently
	// picking one of the two existing interactive behaviors.
	NoConflictRecovery bool
}

// GitSolverLaunch starts @git-solver on the host (`mg host`) in a new
// terminal/pane, pointed at the project root, with a prompt describing the
// interrupted `mg done` — the production wiring behind FinishJob's
// merge-failure handoff. A package-level variable so tests can stub it,
// mirroring launch.ExeOverride's seam pattern.
var GitSolverLaunch = launch.HostAgent

// jobNotFoundErr is the not-found error shape: its Error() text is exactly
// the wording finish-job.sh and delete-job.sh used (pinned by tests), while
// Unwrap keeps it distinguishable via errors.Is(err, ErrJobNotFound).
type jobNotFoundErr struct {
	msg string
}

func (e *jobNotFoundErr) Error() string { return e.msg }
func (e *jobNotFoundErr) Unwrap() error { return ErrJobNotFound }

// ConfirmFunc asks a y/N confirmation question (writing the prompt to the
// user's terminal and reading the answer) and reports whether the user
// answered yes. The CLI wires cli.Confirm; the TUI provides its own prompt or
// a pre-approved "already confirmed" path.
type ConfirmFunc func(prompt string) (bool, error)

// FinishResult is the outcome of a successful FinishJob.
type FinishResult struct {
	// JobName is the job's directory name (the id_slug).
	JobName string

	// Branch is the job branch that was merged and deleted.
	Branch string

	// BaseBranch is the branch the job was squash-merged into.
	BaseBranch string
}

// FinishJob ports scripts/finish-job.sh — the job-archiving lifecycle: resolve
// the job's branch + worktree, run the verdict and clean-tree checks (with the
// script's exact warnings and confirmations), move the job directory into
// docs/jobs/archive/ inside the job's own worktree, commit the archive move,
// squash-merge the branch onto the configured base branch in the main
// worktree, remove the job's worktree, delete the branch, and finally remove
// the job's mg-jdi status sidecar (best-effort — the archive itself has
// already succeeded by then).
//
// root is the project root, resolved by the caller (the CLI's docs-walk-up,
// or the TUI's a.root). confirm answers the script's `read -rp` prompts (a
// declined answer returns ErrCancelled, the script's `exit 0`); informational
// output goes to out.
func FinishJob(root, jobArg string, confirm ConfirmFunc, out io.Writer) (FinishResult, error) {
	return FinishJobWithOptions(root, jobArg, confirm, out, FinishOptions{})
}

// FinishJobWithOptions is FinishJob plus FinishOptions — see its own doc for
// the one behavior it can change (squash-merge-conflict recovery). Every
// other step is identical to FinishJob.
func FinishJobWithOptions(root, jobArg string, confirm ConfirmFunc, out io.Writer, opts FinishOptions) (FinishResult, error) {

	// ── Resolve the job's branch + worktree ────────────────────────────────
	branches, berr := git.LocalBranches(root)
	if berr != nil {
		return FinishResult{}, berr
	}
	branch := git.ExactBranchMatch(branches, jobArg)
	if branch == "" {
		prefixMatches := git.PrefixBranchMatches(branches, jobArg)
		switch len(prefixMatches) {
		case 0:
			return FinishResult{}, jobNotFoundError(jobArg, branches)
		case 1:
			branch = prefixMatches[0]
		default:
			return FinishResult{}, fmt.Errorf("job '%s' is ambiguous — matches branches: %s", jobArg, strings.Join(prefixMatches, " "))
		}
	}
	jobName := git.BranchTail(branch)

	wtPath, ok, werr := git.WorktreeForBranch(root, branch)
	if werr != nil {
		return FinishResult{}, werr
	}
	if !ok {
		return FinishResult{}, fmt.Errorf("branch '%s' has no git worktree — cannot finish job '%s'.\nA job's worktree is created by 'mg job' and should always exist for an open job; this is an inconsistent state.", branch, jobName)
	}

	jobDir := filepath.Join(wtPath, JobsRelDir, jobName)
	brief := filepath.Join(jobDir, "brief.md")
	verdict := filepath.Join(jobDir, "verdict.md")

	if _, err := os.Stat(brief); err != nil {
		return FinishResult{}, fmt.Errorf("brief.md not found in %s", jobDir)
	}

	// ── Verdict checks (same warnings + confirmations as the script) ──────
	if verdictData, err := os.ReadFile(verdict); err == nil {
		overall := verdictOverallMatch(verdictData)
		switch {
		case overall == "":
			fmt.Fprintln(out, "Warning: could not determine verdict status from verdict.md")
			if err := askConfirm(confirm, out, "  Continue anyway? [y/N] "); err != nil {
				return FinishResult{}, err
			}
		case verdictNotApprovedRe.MatchString(overall):
			fmt.Fprintf(out, "Warning: verdict is '%s' — job is not approved.\n", overall)
			if err := askConfirm(confirm, out, "  Continue anyway? [y/N] "); err != nil {
				return FinishResult{}, err
			}
		}
	} else if !os.IsNotExist(err) {
		return FinishResult{}, err
	} else {
		fmt.Fprintln(out, "Warning: no verdict.md found — job has not been reviewed.")
		if err := askConfirm(confirm, out, "  Continue anyway? [y/N] "); err != nil {
			return FinishResult{}, err
		}
	}

	// ── Git checks ─────────────────────────────────────────────────────────
	// The base branch the job merges into: the project's configured
	// baseBranch key wins when present; falling back to origin/HEAD's target,
	// then "main" — finish-job.sh's exact chain.
	settings, err := project.Load(root)
	if err != nil {
		return FinishResult{}, err
	}
	baseBranch := settings.BaseBranch
	if baseBranch == "" {
		baseBranch = git.SymbolicRefHead(root)
	}

	worktreeBranch, err := git.CurrentBranch(wtPath)
	if err != nil {
		return FinishResult{}, err
	}
	if worktreeBranch != branch {
		return FinishResult{}, fmt.Errorf("worktree at %s is on '%s', expected '%s'.\nSomeone may have checked out a different branch inside this job's worktree by hand — fix that before finishing.", wtPath, worktreeBranch, branch)
	}

	if dirty, err := git.WorkingTreeDirty(wtPath); err != nil {
		return FinishResult{}, err
	} else if dirty {
		return FinishResult{}, fmt.Errorf("uncommitted changes in the worktree for branch '%s'. Commit or stash before finishing.", branch)
	}

	// ── Info + confirmation ────────────────────────────────────────────────
	jobTitle := briefTitle(brief)
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Finishing job: %s\n", jobName)
	fmt.Fprintf(out, "  Worktree: %s\n", wtPath)
	fmt.Fprintf(out, "  Branch  : %s → %s\n", branch, baseBranch)
	fmt.Fprintf(out, "  Archive : %s/archive/%s\n", JobsRelDir, jobName)
	fmt.Fprintln(out, "")
	if err := askConfirm(confirm, out, "  Proceed? [y/N] "); err != nil {
		return FinishResult{}, err
	}

	// ── Archive inside the job's own worktree first ────────────────────────
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "→ Archiving job directory on %s...\n", branch)
	archiveDir := filepath.Join(wtPath, JobsRelDir, ArchiveDirName)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return FinishResult{}, err
	}
	if err := os.Rename(jobDir, filepath.Join(archiveDir, jobName)); err != nil {
		return FinishResult{}, err
	}

	// status: open → status: done in the archived brief.md
	archivedBrief := filepath.Join(archiveDir, jobName, "brief.md")
	if err := rewriteStatus(archivedBrief, "done"); err != nil {
		return FinishResult{}, err
	}

	if err := git.Stage(wtPath, filepath.Join(wtPath, JobsRelDir)); err != nil {
		return FinishResult{}, err
	}
	if err := git.CommitStaged(wtPath, "archive: "+jobName); err != nil {
		return FinishResult{}, err
	}

	// ── Squash-merge into the base branch (one commit for the whole job) ───
	fmt.Fprintf(out, "→ Switching to %s in the main worktree...\n", baseBranch)
	if err := git.Checkout(root, baseBranch); err != nil {
		return FinishResult{}, err
	}

	// Capture the pre-merge state so a failed squash merge can be rolled back
	// cleanly: a conflicted `git merge --squash` leaves the main worktree with
	// a half-staged, conflicted index and no MERGE_HEAD to abort from, and the
	// only clean undo is `git reset --hard` to the pre-merge HEAD — safe only
	// when the main worktree held no tracked local changes before the merge.
	preMergeHead, err := git.RevParse(root, "HEAD")
	if err != nil {
		return FinishResult{}, err
	}
	mainDirty, err := git.WorkingTreeDirty(root)
	if err != nil {
		return FinishResult{}, err
	}

	fmt.Fprintf(out, "→ Squash-merging %s...\n", branch)
	if err := git.SquashMerge(root, branch); err != nil {
		if opts.NoConflictRecovery {
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "✗ Squash merge failed:")
			fmt.Fprintln(out, "  "+err.Error())
			fmt.Fprintln(out, "  No automatic recovery was attempted (NoConflictRecovery): the main worktree is")
			fmt.Fprintf(out, "  left in its conflicted-merge state. The job directory is already archived on\n")
			fmt.Fprintf(out, "  %s, and the job's worktree and branch still exist — that step cannot be\n", branch)
			fmt.Fprintln(out, "  undone without a much larger restructuring. Resolve the conflict manually, or")
			fmt.Fprintln(out, "  trigger @git-solver through a separate call, then finish the job again.")
			return FinishResult{}, fmt.Errorf("%w: %v", ErrSquashMergeConflict, err)
		}
		return handleSquashMergeFailure(out, confirm, root, jobName, branch, baseBranch, preMergeHead, mainDirty, err)
	}
	subject := jobTitle
	if subject == "" {
		subject = jobName
	}
	if err := git.CommitStaged(root, subject+"\n\nJob: "+jobName); err != nil {
		return FinishResult{}, err
	}

	// ── Remove the job's worktree, then delete its branch ──────────────────
	mainWorktree, terr := git.RevParseToplevel(root)
	if terr != nil {
		mainWorktree = root
	}
	if filepath.Clean(wtPath) == filepath.Clean(mainWorktree) {
		fmt.Fprintln(out, "→ Worktree is the main worktree — skipping worktree removal.")
	} else {
		fmt.Fprintf(out, "→ Removing worktree %s...\n", wtPath)
		if err := git.WorktreeRemove(root, wtPath); err != nil {
			return FinishResult{}, err
		}
		_ = git.WorktreePrune(root)
	}

	fmt.Fprintf(out, "→ Deleting branch %s...\n", branch)
	if err := git.BranchDelete(root, branch); err != nil {
		return FinishResult{}, err
	}

	// Clean up after mg-jdi: the job is archived and its branch gone, so the
	// status/run.log sidecar is dead weight — mg-jdi never runs against an
	// archived job. Best-effort — the archive itself already succeeded, so a
	// failure here is a warning, not an abort.
	if removed, err := RemoveJDIStatus(root, jobName); err != nil {
		fmt.Fprintf(out, "  Warning: could not remove mg-jdi status for %s: %v\n", jobName, err)
	} else if removed {
		fmt.Fprintf(out, "→ Removing mg-jdi status for %s...\n", jobName)
	}

	// ── Done ────────────────────────────────────────────────────────────────
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "✓ Job finished: %s\n", jobName)
	fmt.Fprintf(out, "  Merged into : %s\n", baseBranch)
	fmt.Fprintf(out, "  Archived at : %s/archive/%s\n", JobsRelDir, jobName)
	return FinishResult{JobName: jobName, Branch: branch, BaseBranch: baseBranch}, nil
}

// askConfirm runs one confirmation; a declined answer becomes ErrCancelled.
func askConfirm(confirm ConfirmFunc, out io.Writer, prompt string) error {
	if confirm == nil {
		// No confirm function (tests / a non-interactive caller): treat a
		// prompt as declined — the scripts' default-no behavior.
		return ErrCancelled
	}
	ok, err := confirm(prompt)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCancelled
	}
	return nil
}

// handleSquashMergeFailure runs when the squash merge fails — usually a
// conflict. It offers to hand the broken state to @git-solver (mg host), which
// resolves the conflict and finishes the cleanup `mg done` would have done
// (worktree, branch, mg-jdi status); when the user declines, or the launch
// itself fails, it rolls the main worktree back to the pre-merge HEAD (via
// ResetHard — safe only when the main worktree was clean before the merge) so
// the repo is never left half-merged. Returns ErrGitSolverHandoff on a
// successful handoff; otherwise returns mergeErr for the caller to surface.
func handleSquashMergeFailure(out io.Writer, confirm ConfirmFunc, root, jobName, branch, baseBranch, preMergeHead string, mainDirty bool, mergeErr error) (FinishResult, error) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "✗ Squash merge failed:")
	fmt.Fprintln(out, "  "+mergeErr.Error())
	fmt.Fprintln(out, "  The main worktree may be left with a conflicted merge; the job directory is")
	fmt.Fprintf(out, "  already archived on %s, and the job's worktree and branch still exist.\n", branch)
	fmt.Fprintln(out, "  @git-solver can resolve the conflict and finish the cleanup (worktree, branch, mg-jdi status).")
	if err := askConfirm(confirm, out, "  Start @git-solver now (mg host)? [y/N] "); err == nil {
		desc, lerr := GitSolverLaunch("git-solver", gitSolverPrompt(jobName, branch, baseBranch), root, "", "")
		if lerr != nil {
			fmt.Fprintf(out, "  Warning: could not start @git-solver (%v) — rolling the merge back.\n", lerr)
		} else {
			fmt.Fprintf(out, "→ Started @git-solver in %s — it will resolve the conflict and finish the cleanup.\n", desc)
			fmt.Fprintln(out, "  Re-check the job with `mg jobs` when the session ends.")
			return FinishResult{}, ErrGitSolverHandoff
		}
	}
	// Declined (or the launch failed): roll the merge back so the repo is left
	// clean. Only safe when the main worktree held no tracked local changes
	// before the merge — reset --hard would destroy them.
	if !mainDirty {
		if rerr := git.ResetHard(root, preMergeHead); rerr != nil {
			fmt.Fprintf(out, "  Warning: could not roll the merge back: %v\n", rerr)
			fmt.Fprintln(out, "  Resolve the conflicted merge manually, or run `mg host -a git-solver` from the project root.")
		} else {
			fmt.Fprintln(out, "→ Rolled the failed merge back — the main worktree is clean again.")
			fmt.Fprintf(out, "  The job is archived on %s; its worktree and branch remain.\n", branch)
			fmt.Fprintln(out, "  To finish the job, run `mg host -a git-solver` from the project root (recommended),")
			fmt.Fprintf(out, "  or resolve the merge manually and remove the worktree/branch with `mg delete %s`.\n", jobName)
		}
	} else {
		fmt.Fprintln(out, "  The main worktree had uncommitted changes before the merge — could not auto-roll-back.")
		fmt.Fprintln(out, "  Resolve the conflicted merge manually, or run `mg host -a git-solver` from the project root.")
	}
	return FinishResult{}, mergeErr
}

// gitSolverPrompt builds the instruction handed to @git-solver when mg done's
// squash merge fails: the state of the interrupted finish (job archived on the
// branch, worktree + branch + mg-jdi sidecar remaining) and the cleanup
// expected of it.
func gitSolverPrompt(jobName, branch, baseBranch string) string {
	return fmt.Sprintf(
		"An interrupted `mg done` for job %s: the squash merge of branch %s into %s conflicted and was left in the main worktree (a conflicted `git merge --squash` in progress — there is no MERGE_HEAD). "+
			"The job directory was already archived (docs/jobs/archive/%s) on branch %s before the merge failed; the job's worktree and branch still exist. "+
			"Please: (1) inspect the actual state (git status, git log, git worktree list, conflict markers), (2) resolve the conflicts and commit the merge in the main worktree, "+
			"(3) remove the job's worktree and delete branch %s — the cleanup `mg done` would have done — (4) remove the job's mg-jdi status sidecar under .manigot/jdi-status/ if present, (5) report what you did. Do not push.",
		jobName, branch, baseBranch, jobName, branch, branch)
}

// jobNotFoundError builds the "job not found among local branches" error with
// the active-branch listing — the wording finish-job.sh and delete-job.sh
// shared (pinned by tests). Shared by FinishJob and DeleteJob.
func jobNotFoundError(jobArg string, branches []string) error {
	msg := fmt.Sprintf("job '%s' not found among local branches.\nActive job branches:", jobArg)
	for _, b := range branches {
		msg += "\n  " + b
	}
	return &jobNotFoundErr{msg: msg}
}

// briefTitle extracts the job title the way finish-job.sh did:
// `head -1 "$BRIEF" | sed 's/^# Brief: *//'`.
func briefTitle(briefPath string) string {
	f, err := os.Open(briefPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	line, _ := bufio.NewReader(f).ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	return regexp.MustCompile(`^# Brief: *`).ReplaceAllString(line, "")
}

// rewriteStatus replaces the brief's `status: <old>` line with `status: <new>`
// — the Go form of finish-job.sh's `sed -i "s/^status: .*/status: done/"`.
func rewriteStatus(briefPath, status string) error {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "status: ") {
			b.WriteString("status: " + status)
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return os.WriteFile(briefPath, []byte(b.String()), 0o644)
}
