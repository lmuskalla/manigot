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
	"github.com/lmuskalla/manigot/internal/project"
)

// ErrCancelled is returned by FinishJob and DeleteJob when the user declines a
// confirmation prompt — the scripts' `exit 0` on a declined `[y/N]` answer.
// The CLI treats it as a clean exit with no further output.
var ErrCancelled = errors.New("cancelled")

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
// worktree, remove the job's worktree, and delete the branch.
//
// root is the project root, resolved by the caller (the CLI's docs-walk-up,
// or the TUI's a.root). confirm answers the script's `read -rp` prompts (a
// declined answer returns ErrCancelled, the script's `exit 0`); informational
// output goes to out.
func FinishJob(root, jobArg string, confirm ConfirmFunc, out io.Writer) (FinishResult, error) {

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

	fmt.Fprintf(out, "→ Squash-merging %s...\n", branch)
	if err := git.SquashMerge(root, branch); err != nil {
		return FinishResult{}, err
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

// jobNotFoundError builds the "job not found among local branches" error with
// the active-branch listing — the wording finish-job.sh and delete-job.sh
// shared (pinned by tests). Shared by FinishJob and DeleteJob.
func jobNotFoundError(jobArg string, branches []string) error {
	msg := fmt.Sprintf("job '%s' not found among local branches.\nActive job branches:", jobArg)
	for _, b := range branches {
		msg += "\n  " + b
	}
	return errors.New(msg)
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
