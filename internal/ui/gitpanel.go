package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// gpAction is what gitPanelView.update returns for the App to act on.
type gpAction int

const (
	gpNone   gpAction = iota
	gpCancel          // esc / q — discard and return to the detail view
	gpSubmit          // enter — run the highlighted git action
)

// gitAction is one selectable git command in the panel.
type gitAction int

const (
	gitActionCommitAll gitAction = iota
	gitActionPush
	gitActionMerge
)

// gitPanelActions lists the panel's actions in display order, with the label
// shown for each row. This is the single source of truth for both the
// rendering and the App's dispatch (updateGitPanel's switch on gitAction).
// The three actions mirror the detail view's git surface: the "c" commit-all
// and "P" push-to-origin accelerators plus the "merge default branch" action
// that brings a job's worktree up to speed with the project's base branch.
var gitPanelActions = []struct {
	action gitAction
	label  string
}{
	{gitActionCommitAll, "Commit all"},
	{gitActionPush, "Push to origin"},
	{gitActionMerge, "Merge default branch"},
}

// gitPanelView is the small "git" modal opened by "g" from the detail view: a
// non-destructive picker of the git commands the detail view offers, one per
// row, moved through with ↑/↓/k/j and run with enter. It deliberately has no
// type-to-filter (three fixed rows need none, unlike the agents picker) and
// no filtering bookkeeping at all — the App acts on gpSubmit via the
// selected() action, exactly like agentsPickerView hands its apSubmit to
// launch.AgentQuick. Like every other overlay view it renders nothing by
// itself at the App level; the App's View() switches on stateGitPanel.
type gitPanelView struct {
	cursor int
	width  int
	height int
}

// newGitPanelView builds the git panel for the given viewport size.
func newGitPanelView(width, height int) *gitPanelView {
	return &gitPanelView{width: width, height: height}
}

// resize updates the viewport on terminal resize.
func (v *gitPanelView) resize(width, height int) {
	v.width, v.height = width, height
}

// update processes a key for the panel and reports the resulting action:
// ↑/↓/k/j move the cursor (clamped to the three rows), enter submits, esc/q
// cancel. Every other key is a no-op.
func (v *gitPanelView) update(msg tea.KeyMsg) gpAction {
	switch msg.String() {
	case "enter":
		return gpSubmit
	case "esc", "q":
		return gpCancel
	case "up", "k":
		if v.cursor > 0 {
			v.cursor--
		}
	case "down", "j":
		if v.cursor < len(gitPanelActions)-1 {
			v.cursor++
		}
	}
	return gpNone
}

// selected returns the action under the cursor, or false if the cursor is out
// of range (defensive — the cursor is always clamped by update).
func (v *gitPanelView) selected() (gitAction, bool) {
	if v.cursor < 0 || v.cursor >= len(gitPanelActions) {
		return gitActionCommitAll, false
	}
	return gitPanelActions[v.cursor].action, true
}

// render draws the panel: a title, one row per action (the cursor row
// highlighted the same way the agents picker highlights its selected row),
// and a footer key hint.
func (v *gitPanelView) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Git"))
	b.WriteString("\n\n")
	for i, a := range gitPanelActions {
		if i == v.cursor {
			b.WriteString(selectedStyle.Render("▶ " + a.label))
		} else {
			b.WriteString(dimStyle.Render("  " + a.label))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓/k/j navigate · enter run · esc/q cancel"))
	return b.String()
}
