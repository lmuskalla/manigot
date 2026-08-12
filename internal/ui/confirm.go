package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/project"
)

// confirmAction is what a confirm view runs once the user confirms.
type confirmAction int

const (
	confirmDone confirmAction = iota
	confirmDelete
)

// confirmView is the TUI-side confirmation for the destructive done/delete
// actions — the in-process replacement for the subprocess prompts of
// finish-job.sh / delete-job.sh (which previously ran in the foreground via
// tea.ExecProcess because the scripts needed an interactive terminal). It
// shows the same summary lines the scripts showed before their
// `read -rp "  Proceed? [y/N] "` and answers y/n itself; the lifecycle then
// runs with every internal prompt pre-approved.
//
// Keys: y/enter confirms and runs the action in the background, n/esc/q
// cancels back to the detail view.
type confirmView struct {
	action confirmAction
	lines  []string
	width  int
	height int
}

// newConfirmView builds the confirm view for the given action on job j in the
// project at root.
func newConfirmView(action confirmAction, root string, j job.Job) *confirmView {
	v := &confirmView{action: action}
	if action == confirmDone {
		v.lines = doneConfirmLines(root, j)
	} else {
		v.lines = deleteConfirmLines(root, j)
	}
	return v
}

// resize updates the viewport on terminal resize.
func (v *confirmView) resize(width, height int) {
	v.width, v.height = width, height
}

// render draws the confirmation.
func (v *confirmView) render() string {
	title := "Mark job done"
	if v.action == confirmDelete {
		title = "Delete job"
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	for _, line := range v.lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("y/enter proceed · n/esc cancel"))
	return b.String()
}

// doneConfirmLines builds the summary lines finish-job.sh printed before its
// final "  Proceed? [y/N] " prompt.
func doneConfirmLines(root string, j job.Job) []string {
	var lines []string
	lines = append(lines, "  Finishing job: "+j.Name)

	wt := worktreePath(root, j)
	lines = append(lines, "  Worktree: "+wt)

	settings, _ := project.Load(root)
	base := settings.BaseBranch
	if base == "" {
		base = git.SymbolicRefHead(root)
	}
	lines = append(lines, fmt.Sprintf("  Branch  : %s → %s", j.Branch, base))
	lines = append(lines, "  Archive : "+job.JobsRelDir+"/archive/"+j.Name)

	if w := doneConfirmWarning(j); w != "" {
		lines = append(lines, w)
	}
	return lines
}

// doneConfirmWarning mirrors finish-job.sh's verdict warnings for the
// confirmation screen: no verdict, an undetermined verdict, or a REJECTED /
// NEEDS WORK verdict each get the script's exact warning line.
func doneConfirmWarning(j job.Job) string {
	if _, err := os.Stat(filepath.Join(j.Dir, "verdict.md")); err != nil {
		return "  Warning: no verdict.md found — job has not been reviewed."
	}
	overall := j.VerdictStatus()
	switch {
	case overall == "":
		return "  Warning: could not determine verdict status from verdict.md."
	case job.VerdictNotApproved(overall):
		return fmt.Sprintf("  Warning: verdict is '%s' — job is not approved.", overall)
	}
	return ""
}

// deleteConfirmLines builds the summary lines delete-job.sh printed before its
// "  Proceed? [y/N] " prompt, including the dirty-worktree warning wording.
func deleteConfirmLines(root string, j job.Job) []string {
	var lines []string
	title := j.Title
	if title == "" {
		title = j.Name
	}
	lines = append(lines, "  Title    : "+title)

	wt := worktreePath(root, j)
	lines = append(lines, "  Worktree : "+wt)
	lines = append(lines, fmt.Sprintf("  Branch   : %s (will be deleted, unmerged)", j.Branch))

	if dirty, err := git.WorkingTreeDirty(wt); err == nil && dirty {
		lines = append(lines, "  Warning  : this worktree has uncommitted changes — they will be discarded.")
	}

	lines = append(lines, "")
	lines = append(lines, "This cannot be undone.")
	return lines
}

// worktreePath resolves the job's worktree (falling back to the project root
// for the working-tree-only non-repo case).
func worktreePath(root string, j job.Job) string {
	if j.Branch != "" {
		if p, ok, _ := git.WorktreeForBranch(root, j.Branch); ok {
			return p
		}
	}
	return root
}
