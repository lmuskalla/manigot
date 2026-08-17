package ui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
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

// gitPanelOverlay renders the git panel as a popup modal floating over the
// still-visible detail view: the detail frame stays on screen underneath and
// the panel's styled box (gitPanelModalStyle) is drawn centered on top of it,
// so "g" reads as a popup rather than the full-frame swap it used to be — the
// surrounding detail content keeps giving context while the three actions sit
// in a box over it. The box has a solid background, so the cells it covers
// are replaced wholesale; everything else is left untouched. Purely
// compositional — the panel view itself, its keys, and the App's dispatch
// (updateGitPanel) are unchanged.
func (a *App) gitPanelOverlay() string {
	base := ""
	if a.detail != nil {
		base = a.detail.render()
	}
	if a.gitPanel == nil {
		return base
	}
	modal := gitPanelModalStyle.Render(a.gitPanel.render())
	return placeOverlay(base, modal, a.width, a.height)
}

// placeOverlay centers modal over base and merges the two cell by cell: the
// modal's solid box replaces the base cells it covers and the rest of the
// base stays visible, producing a floating popup over a live background (as
// opposed to a full-frame swap). Every base line is padded to the frame width
// first, so the box lands at the same centered column on every row regardless
// of how wide that row's content is, then spliced at the modal's column range
// with cut — ANSI-aware, so escape sequences and wide runes are never split
// mid-code — with the modal's own ANSI carried in wholesale. A base shorter
// than the frame is padded with blank rows so the box is never clipped at the
// bottom. The placement is clamped to the frame so an oversized modal hugs the
// top left instead of being pushed off-screen.
func placeOverlay(base, modal string, width, height int) string {
	modalLines := strings.Split(modal, "\n")
	modalW := 0
	for _, l := range modalLines {
		if w := ansi.StringWidth(l); w > modalW {
			modalW = w
		}
	}
	modalH := len(modalLines)
	x := clamp((width-modalW)/2, 0, width)
	y := clamp((height-modalH)/2, 0, height)
	pad := strings.Repeat(" ", width)
	baseLines := strings.Split(base, "\n")
	// A base shorter than the frame gets blank rows below it, so the frame is
	// always height rows tall and a box taller than the content still has
	// room (the blank area renders as empty terminal cells, like a short view
	// would on a real screen).
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	out := make([]string, 0, len(baseLines))
	for i, bl := range baseLines {
		if w := ansi.StringWidth(bl); w < width {
			bl += pad[:width-w]
		}
		if i >= y && i < y+modalH {
			ml := modalLines[i-y]
			prefix, suffix := cut(bl, x, x+ansi.StringWidth(ml))
			out = append(out, prefix+ml+suffix)
		} else {
			out = append(out, bl)
		}
	}
	return strings.Join(out, "\n")
}

// cut splits a rendered line s at display columns [lo, hi), returning
// everything before lo and everything from hi on with the middle dropped.
// ANSI escape sequences are zero-width and carried whole into whichever side
// they belong to — never split — and wide/combining runes are counted by
// display width, so a rune that straddles lo or hi is kept whole in the
// nearer side rather than torn apart.
func cut(s string, lo, hi int) (left, right string) {
	var lb, rb strings.Builder
	col := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			seq, n := scanAnsi(s[i:])
			// Sequences inside the dropped range are covered by the modal
			// box's own codes; those outside are carried through so the
			// surrounding base keeps its styling.
			if col < lo {
				lb.WriteString(seq)
			} else if col >= hi {
				rb.WriteString(seq)
			}
			i += n
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if col < lo {
			lb.WriteRune(r)
		} else if col >= hi {
			rb.WriteRune(r)
		}
		col += runewidth.RuneWidth(r)
		i += n
	}
	return lb.String(), rb.String()
}

// scanAnsi returns the escape sequence beginning at s (s[0] must be ESC) and
// its byte length. It handles the forms lipgloss/glamour emit — CSI (ESC [
// … final byte), OSC (ESC ] … BEL or ESC \), and the generic ESC + one-byte
// case — consuming an unrecognized prefix wholesale so no sequence can ever
// be split mid-code.
func scanAnsi(s string) (string, int) {
	if len(s) < 2 {
		return s, len(s)
	}
	switch s[1] {
	case '[': // CSI — params/intermediates (0x20–0x2F) then final (0x40–0x7E)
		for i := 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return s[:i+1], i + 1
			}
		}
		return s, len(s)
	case ']': // OSC — until BEL (0x07) or ST (ESC \)
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return s[:i+1], i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return s[:i+2], i + 2
			}
		}
		return s, len(s)
	default:
		return s[:2], 2
	}
}
