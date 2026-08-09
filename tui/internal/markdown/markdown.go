// Package markdown renders safecode job markdown files to terminal-styled text
// and provides a scrollable Viewer over the rendered output.
//
// It wraps Charm's Glamour (the same family as the Bubble Tea stack chosen in
// TASK-1) for the markdown→ANSI conversion, and adds a small scroll buffer so
// the detail view (TASK-6) can page through a file that is taller than the
// terminal.
package markdown

import (
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
)

// minWidth is the smallest wrap width we'll pass to Glamour. Below this, table
// columns and code blocks render badly.
const minWidth = 20

// rendererCache holds one glamour.TermRenderer per wrap width (WithWordWrap is
// the only width-dependent option we pass), so repeated calls to Render at the
// same width reuse it instead of constructing a new one.
//
// This matters beyond the obvious construction cost: glamour.WithAutoStyle()
// probes the terminal's background color via termenv.HasDarkBackground(),
// which writes an OSC query to the tty and then reads raw bytes directly off
// stdin until it sees the matching response (termenv_unix.go's
// readNextResponse). That read races with — and can consume/discard — Bubble
// Tea's own raw-mode stdin reader, which is what made keystrokes get lost or
// delayed (TASK-1). Building the renderer once per width means that probe
// only ever runs once per width instead of on nearly every keypress.
var (
	rendererMu    sync.Mutex
	rendererCache = map[int]*glamour.TermRenderer{}
)

func rendererFor(width int) (*glamour.TermRenderer, error) {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	if r, ok := rendererCache[width]; ok {
		return r, nil
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(), // adapt to the user's terminal theme
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}
	rendererCache[width] = r
	return r, nil
}

// Render converts markdown source to ANSI-styled, word-wrapped text. It always
// returns a string and never fails hard: on any renderer error it returns the
// original source so a file is always readable.
func Render(src string, width int) string {
	if width < minWidth {
		width = minWidth
	}
	r, err := rendererFor(width)
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return strings.TrimRight(out, "\n")
}

// Viewer is a scrollable buffer over one rendered markdown file. It has no
// dependency on Bubble Tea so it is straightforward to unit-test; the detail
// view drives it with key events and calls View() each frame.
type Viewer struct {
	src    string // raw source, kept so Resize can re-wrap
	width  int
	height int
	lines  []string // rendered, line-split
	offset int      // index of the top visible line
}

// NewViewer returns a viewer with the given viewport size and no content.
func NewViewer(width, height int) *Viewer {
	return &Viewer{width: width, height: height}
}

// SetContent renders src at the current width and stores the result.
func (v *Viewer) SetContent(src string) {
	v.src = src
	v.rebuild()
}

// Resize re-renders at the new dimensions (a width change means re-wrapping).
func (v *Viewer) Resize(width, height int) {
	if width == v.width && height == v.height {
		return
	}
	v.width, v.height = width, height
	v.rebuild()
}

func (v *Viewer) rebuild() {
	v.lines = strings.Split(Render(v.src, v.width), "\n")
	v.clamp()
}

// LineCount returns the number of rendered lines.
func (v *Viewer) LineCount() int { return len(v.lines) }

// View returns the visible slice of rendered lines, joined with newlines.
func (v *Viewer) View() string {
	if len(v.lines) == 0 || v.height <= 0 {
		return ""
	}
	end := v.offset + v.height
	if end > len(v.lines) {
		end = len(v.lines)
	}
	return strings.Join(v.lines[v.offset:end], "\n")
}

// Position reports scroll progress as "top/total" for a status line.
func (v *Viewer) Position() string {
	total := len(v.lines)
	top := v.offset + 1
	if total == 0 {
		return "0/0"
	}
	return strconv.Itoa(top) + "/" + strconv.Itoa(total)
}

// clamp keeps the offset within the scrollable range.
func (v *Viewer) clamp() {
	if v.height <= 0 {
		v.offset = 0
		return
	}
	max := len(v.lines) - v.height
	if max < 0 {
		max = 0
	}
	switch {
	case v.offset > max:
		v.offset = max
	case v.offset < 0:
		v.offset = 0
	}
}

// ScrollDown moves the viewport down by n lines (positive n).
func (v *Viewer) ScrollDown(n int) { v.offset += n; v.clamp() }

// ScrollUp moves the viewport up by n lines.
func (v *Viewer) ScrollUp(n int) { v.offset -= n; v.clamp() }

// PageDown scrolls by one viewport height (minus one line of overlap).
func (v *Viewer) PageDown() { v.ScrollDown(max(1, v.height-1)) }

// PageUp scrolls up by one viewport height.
func (v *Viewer) PageUp() { v.ScrollUp(max(1, v.height-1)) }

// Top jumps to the first line.
func (v *Viewer) Top() { v.offset = 0; v.clamp() }

// Bottom jumps to the last page.
func (v *Viewer) Bottom() { v.offset = max(0, len(v.lines)-1); v.clamp() }
