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
