package markdown

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/glamour"
)

func TestRenderProducesNonEmpty(t *testing.T) {
	src := "# Title\n\nSome **bold** text and a paragraph.\n"
	out := Render(src, 60)
	if strings.TrimSpace(out) == "" {
		t.Fatal("Render returned empty output")
	}
	// Glamour renders the heading; the word "Title" must survive.
	if !strings.Contains(out, "Title") {
		t.Errorf("rendered output missing heading text; got:\n%s", out)
	}
}

func TestRenderFallsBackOnTinyWidth(t *testing.T) {
	// Should not panic or return empty for a tiny width.
	out := Render("# H\n\nbody\n", 4)
	if strings.TrimSpace(out) == "" {
		t.Fatal("Render returned empty for tiny width")
	}
}

func TestViewerScroll(t *testing.T) {
	// Build content that renders to many lines.
	src := "# Heading\n\n" + strings.Repeat("line of content\n", 40)
	v := NewViewer(40, 5)
	v.SetContent(src)

	if v.LineCount() <= 5 {
		t.Fatalf("expected more than 5 rendered lines, got %d", v.LineCount())
	}
	startOffset := 0
	if v.offset != startOffset {
		t.Errorf("initial offset = %d, want 0", v.offset)
	}

	before := v.offset
	v.ScrollDown(3)
	if v.offset != before+3 {
		t.Errorf("after ScrollDown(3) offset = %d, want %d", v.offset, before+3)
	}

	// PageDown should move by height-1 = 4.
	before = v.offset
	v.PageDown()
	if v.offset != before+4 {
		t.Errorf("after PageDown offset = %d, want %d", v.offset, before+4)
	}

	// ScrollUp at top clamps to 0.
	v.Top()
	if v.offset != 0 {
		t.Errorf("after Top offset = %d, want 0", v.offset)
	}
	v.ScrollUp(10)
	if v.offset != 0 {
		t.Errorf("ScrollUp at top should clamp to 0, got %d", v.offset)
	}
}

func TestViewerBottomClamps(t *testing.T) {
	src := strings.Repeat("line\n", 40)
	v := NewViewer(40, 5)
	v.SetContent(src)
	v.Bottom()
	max := v.LineCount() - 5
	if v.offset != max {
		t.Errorf("after Bottom offset = %d, want %d (lineCount=%d)", v.offset, max, v.LineCount())
	}
	// Scrolling past the end clamps.
	v.ScrollDown(100)
	if v.offset != max {
		t.Errorf("ScrollDown past end should clamp to %d, got %d", max, v.offset)
	}
}

func TestViewerPosition(t *testing.T) {
	src := strings.Repeat("line\n", 40)
	v := NewViewer(40, 5)
	v.SetContent(src)
	got := v.Position()
	want := "1/" + strconv.Itoa(v.LineCount())
	if got != want {
		t.Errorf("Position = %q, want %q", got, want)
	}
}

// TestRendererReusedPerWidth is a regression test for TASK-2: Render must not
// build a brand-new glamour.TermRenderer (and re-trigger its WithAutoStyle
// terminal probe) on every call. Two calls at the same width should reuse the
// exact same *glamour.TermRenderer instance from rendererCache.
func TestRendererReusedPerWidth(t *testing.T) {
	rendererMu.Lock()
	rendererCache = map[int]*glamour.TermRenderer{}
	rendererMu.Unlock()

	Render("# a\n", 42)
	Render("# b\n", 42)

	rendererMu.Lock()
	defer rendererMu.Unlock()
	if len(rendererCache) != 1 {
		t.Fatalf("expected exactly one cached renderer for width 42, got %d entries", len(rendererCache))
	}
	if _, ok := rendererCache[42]; !ok {
		t.Fatalf("expected a cached renderer keyed by width 42, got keys %v", keysOf(rendererCache))
	}
}

// TestRendererCacheKeyedByWidth ensures a different wrap width gets its own
// renderer instead of reusing one built for a different width (WithWordWrap
// is width-dependent, so sharing across widths would produce stale wrapping).
func TestRendererCacheKeyedByWidth(t *testing.T) {
	rendererMu.Lock()
	rendererCache = map[int]*glamour.TermRenderer{}
	rendererMu.Unlock()

	Render("# a\n", 30)
	Render("# a\n", 60)

	rendererMu.Lock()
	defer rendererMu.Unlock()
	if len(rendererCache) != 2 {
		t.Fatalf("expected two cached renderers (one per width), got %d entries: %v", len(rendererCache), keysOf(rendererCache))
	}
}

func keysOf(m map[int]*glamour.TermRenderer) []int {
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestViewerResizeReRenders(t *testing.T) {
	src := strings.Repeat("word ", 200)
	v := NewViewer(80, 10)
	v.SetContent(src)
	wide := v.LineCount()
	v.Resize(30, 10)
	narrow := v.LineCount()
	if narrow <= wide {
		t.Errorf("expected narrower width to produce more lines (wide=%d narrow=%d)", wide, narrow)
	}
}

// TestSetSizeDoesNotRerender verifies SetSize only updates the viewport
// dimensions, leaving the previously rendered content (and its wrap width)
// untouched — callers that need a re-render at the new size are expected to
// follow up with SetContent (see detailView.ensureCurrentSized), and calling
// SetSize alone must not pay for a render.
func TestSetSizeDoesNotRerender(t *testing.T) {
	src := strings.Repeat("word ", 200)
	v := NewViewer(80, 10)
	v.SetContent(src)
	wide := v.LineCount()

	v.SetSize(20, 10)
	if got := v.LineCount(); got != wide {
		t.Errorf("SetSize re-rendered content: LineCount = %d, want unchanged %d", got, wide)
	}

	// A subsequent SetContent picks up the new width.
	v.SetContent(src)
	if got := v.LineCount(); got == wide {
		t.Errorf("SetContent after SetSize did not re-wrap at the new width: LineCount still %d", got)
	}
}

func TestViewerViewHonoursHeight(t *testing.T) {
	src := strings.Repeat("line\n", 40)
	v := NewViewer(80, 5)
	v.SetContent(src)
	view := v.View()
	lines := strings.Count(view, "\n") + 1
	if lines > 5 {
		t.Errorf("View returned %d lines, want at most 5", lines)
	}
}

// --- plain-text (verbatim) rendering (TASK-2 of the "diff on new lines"
// job) ----------------------------------------------------------------

// TestRenderPlainOneChangePerLine is the core verbatim guarantee: consecutive
// non-blank lines must NOT be merged into one paragraph (what glamour does —
// the reported bug). Each source line stays its own rendered line.
func TestRenderPlainOneChangePerLine(t *testing.T) {
	src := "abc1234 Add gallery\ndef5678 Fix bug\n\n docs/jobs/x/brief.md | 2 ++\n docs/jobs/x/impl.md | 1 +"
	// Wide width: narrow widths would wrap the merged paragraph apart and
	// mask the bug.
	out := RenderPlain(src, 160)
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("RenderPlain produced %d lines, want 5 (one per source line):\n%q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "abc1234 Add gallery") || strings.Contains(lines[0], "def5678") {
		t.Errorf("line 0 should hold only the first commit:\n%q", lines[0])
	}
	if !strings.Contains(lines[1], "def5678 Fix bug") {
		t.Errorf("line 1 should hold only the second commit:\n%q", lines[1])
	}
	if !strings.Contains(lines[3], "docs/jobs/x/brief.md | 2 ++") || strings.Contains(lines[3], "impl.md") {
		t.Errorf("line 3 should hold only the first stat entry:\n%q", lines[3])
	}
	if !strings.Contains(lines[4], "docs/jobs/x/impl.md | 1 +") {
		t.Errorf("line 4 should hold only the second stat entry:\n%q", lines[4])
	}
}

// TestRenderPlainPreservesLeadingSpaces pins the other half of what makes
// verbatim rendering readable: `git diff --stat`'s column alignment (leading
// spaces) must survive, unlike glamour's paragraph rendering which strips
// leading whitespace.
func TestRenderPlainPreservesLeadingSpaces(t *testing.T) {
	out := RenderPlain(" docs/jobs/x/brief.md | 2 ++\n docs/jobs/x/impl.md | 1 +", 60)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("RenderPlain produced %d lines, want 2:\n%q", len(lines), lines)
	}
	for i, l := range lines {
		if !strings.HasPrefix(l, " ") {
			t.Errorf("line %d lost its leading space: %q", i, l)
		}
	}
}

// TestRenderPlainWrapsLongLines verifies the "wraps only lines exceeding the
// width" half: a line longer than the width is wrapped at word boundaries
// (with the wrap column aligned), while short lines stay untouched.
func TestRenderPlainWrapsLongLines(t *testing.T) {
	long := strings.Repeat("word ", 40) // 200 chars, wraps at width 30
	out := RenderPlain(long, 30)
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Errorf("RenderPlain should wrap a 200-char line at width 30 into multiple lines, got %d:\n%q", len(lines), lines)
	}
	for i, l := range lines {
		if l == "" {
			t.Errorf("unexpected empty wrapped line at index %d", i)
		}
		if len([]rune(l)) > 30 {
			t.Errorf("wrapped line %d exceeds the width: %d > 30", i, len([]rune(l)))
		}
	}
	// The words themselves must all survive, in order.
	if !strings.HasPrefix(strings.Join(lines, "\n"), long[:len("word")]) {
		t.Errorf("wrapped output lost the original text start:\n%q", lines)
	}
	if !strings.Contains(strings.Join(lines, " "), "word") {
		t.Errorf("wrapped output lost words:\n%q", lines)
	}
}

// TestRenderPlainEmptyAndWhitespace pins the degenerate cases: empty source
// renders to nothing (no padded blank line), and trailing newlines don't
// produce phantom empty lines.
func TestRenderPlainEmptyAndWhitespace(t *testing.T) {
	if got := RenderPlain("", 40); got != "" {
		t.Errorf("RenderPlain(\"\") = %q, want empty", got)
	}
	out := RenderPlain("abc\ndef\n", 40)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 || lines[1] == "" {
		t.Errorf("trailing newline produced a phantom line: %q", lines)
	}
}

// TestViewerRawModeOneChangePerLine exercises the Viewer's raw mode end to
// end: the same content that glamour merges onto one line stays one line per
// entry once SetRaw(true) is applied, and the same viewer toggled back to
// markdown shows the merged behavior — proving the mode is a per-viewer
// switch, not a global.
func TestViewerRawModeOneChangePerLine(t *testing.T) {
	src := "abc1234 Add gallery\ndef5678 Fix bug\n\n docs/jobs/x/brief.md | 2 ++\n docs/jobs/x/impl.md | 1 +"

	raw := NewViewer(160, 40)
	raw.SetRaw(true)
	raw.SetContent(src)
	rawLines := strings.Split(raw.View(), "\n")
	if len(rawLines) != 5 {
		t.Fatalf("raw viewer rendered %d lines, want 5 (one per source line):\n%q", len(rawLines), rawLines)
	}

	md := NewViewer(160, 40)
	md.SetContent(src)
	mdLines := strings.Split(md.View(), "\n")
	merged := false
	for _, l := range mdLines {
		if strings.Contains(l, "Add gallery") && strings.Contains(l, "Fix bug") {
			merged = true
		}
	}
	if !merged {
		t.Errorf("markdown viewer should still merge the paragraph at a wide width (proving raw is opt-in):\n%q", mdLines)
	}
}

// TestViewerSetRawBeforeContent pins the construction-order case: setting raw
// mode on a viewer that has no content yet must not fabricate a rendered
// line — the later SetContent renders at the new mode. (A rebuild on empty
// src produced a phantom line that made the diff tab look pre-rendered.)
func TestViewerSetRawBeforeContent(t *testing.T) {
	v := NewViewer(160, 40)
	v.SetRaw(true)
	if v.LineCount() != 0 {
		t.Errorf("SetRaw(true) on a content-less viewer fabricated %d lines, want 0", v.LineCount())
	}

	v.SetContent("abc1234 Add gallery\ndef5678 Fix bug")
	if len(v.lines) != 2 {
		t.Errorf("SetContent after SetRaw rendered %d lines, want 2 (raw mode):\n%q", len(v.lines), v.lines)
	}
}

// TestViewerSetRawReRenders pins that SetRaw re-renders the current content
// immediately (a no-op when the mode is unchanged), and that the markdown
// mode keeps glamour's merged-paragraph behavior for the same content.
func TestViewerSetRawReRenders(t *testing.T) {
	src := "abc1234 Add gallery\ndef5678 Fix bug"

	merged := func(lines []string) bool {
		for _, l := range lines {
			if strings.Contains(l, "Add gallery") && strings.Contains(l, "Fix bug") {
				return true
			}
		}
		return false
	}

	v := NewViewer(160, 40)
	v.SetContent(src)
	if !merged(v.lines) {
		t.Fatalf("markdown-mode viewer at a wide width should merge the paragraph, got:\n%q", v.lines)
	}

	v.SetRaw(true)
	if merged(v.lines) {
		t.Errorf("SetRaw(true) still shows the merged line:\n%q", v.lines)
	}
	if len(v.lines) != 2 {
		t.Errorf("SetRaw(true) rendered %d lines, want 2 (one per commit):\n%q", len(v.lines), v.lines)
	}

	// Toggling back restores the markdown rendering.
	v.SetRaw(false)
	if !merged(v.lines) {
		t.Errorf("SetRaw(false) did not restore the merged markdown rendering:\n%q", v.lines)
	}

	// A no-op SetRaw must not disturb anything.
	before := append([]string(nil), v.lines...)
	v.SetRaw(false)
	if len(v.lines) != len(before) || v.lines[0] != before[0] {
		t.Errorf("no-op SetRaw changed the rendered lines")
	}
}
