package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// testPickerRows builds a deterministic row set for picker tests: three rows
// whose display labels carry the full "Alpha Job" text but whose search keys
// use deliberately non-overlapping letters, so type-to-filter tests can pick
// a row with a single character.
func testPickerRows() []PickerRow {
	return []PickerRow{
		{ID: "aaa01", SearchKey: "aaa01 x", Label: "aaa01     feature 2026-01-01 Alpha Job"},
		{ID: "def02", SearchKey: "def02 blue", Label: "def02     fix      2026-02-02 Beta Job"},
		{ID: "ghj03", SearchKey: "ghj03 x", Label: "ghj03     feature 2026-03-03 Gamma Job"},
	}
}

// TestPickerNavigation covers the cursor keys: up/down/k/j move and clamp at
// the ends, home/end jump — the same surface agentspicker_test.go pins for
// the TUI's agent picker.
func TestPickerNavigation(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	if p.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", p.cursor)
	}
	p.Update(keyMsg("up")) // already at top — must not go negative
	if p.cursor != 0 {
		t.Errorf("up at top moved cursor to %d, want 0", p.cursor)
	}
	p.Update(keyMsg("down"))
	if p.cursor != 1 {
		t.Errorf("down = %d, want 1", p.cursor)
	}
	p.Update(keyMsg("j"))
	if p.cursor != 2 {
		t.Errorf("j = %d, want 2", p.cursor)
	}
	p.Update(keyMsg("down")) // already at bottom — must not overrun
	if p.cursor != 2 {
		t.Errorf("down at bottom moved cursor to %d, want 2", p.cursor)
	}
	p.Update(keyMsg("k"))
	if p.cursor != 1 {
		t.Errorf("k = %d, want 1", p.cursor)
	}
	p.Update(keyMsg("home"))
	if p.cursor != 0 {
		t.Errorf("home = %d, want 0", p.cursor)
	}
	p.Update(keyMsg("end"))
	if p.cursor != 2 {
		t.Errorf("end = %d, want 2", p.cursor)
	}
	p.Update(keyMsg("g"))
	if p.cursor != 0 {
		t.Errorf("g = %d, want 0", p.cursor)
	}
	p.Update(keyMsg("G"))
	if p.cursor != 2 {
		t.Errorf("G = %d, want 2", p.cursor)
	}
}

// TestPickerSubmitAndCancel covers the two ways the picker finishes: enter
// submits the row under the cursor (recording its ID), esc/q/ctrl+c cancel.
func TestPickerSubmitAndCancel(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	p.cursor = 1
	m, cmd := p.Update(keyMsg("enter"))
	got := m.(*Picker)
	if cmd == nil {
		t.Error("enter should quit the program")
	}
	if res := got.Result(); !res.Done || res.Cancelled || res.ID != "def02" {
		t.Errorf("Result() = %+v, want Done submit of def02", res)
	}

	for _, key := range []string{"esc", "q", "ctrl+c"} {
		p := NewPicker("Select a job", testPickerRows())
		m, cmd := p.Update(keyMsg(key))
		got := m.(*Picker)
		if cmd == nil {
			t.Errorf("%s should quit the program", key)
		}
		if res := got.Result(); !res.Done || !res.Cancelled {
			t.Errorf("%s Result() = %+v, want Done cancel", key, res)
		}
	}
}

// TestPickerEnterWithNoRows guards the empty-list edge: there is nothing to
// submit, so enter must neither crash nor record a result.
func TestPickerEnterWithNoRows(t *testing.T) {
	p := NewPicker("Select a job", nil)
	m, cmd := p.Update(keyMsg("enter"))
	if cmd != nil {
		t.Error("enter with no rows should not quit")
	}
	if res := m.(*Picker).Result(); res.Done {
		t.Errorf("Result() = %+v, want not done", res)
	}
}

// TestPickerStartAt pins the initial-cursor setter: StartAt moves the cursor
// to the requested row, clamps at both ends, and is a no-op on an empty row
// list — so a caller can open on a chosen row (the active default) without
// risking an out-of-range cursor.
func TestPickerStartAt(t *testing.T) {
	// A middle row lands exactly.
	p := NewPicker("t", testPickerRows())
	p.StartAt(1)
	if p.cursor != 1 {
		t.Errorf("StartAt(1) cursor = %d, want 1", p.cursor)
	}

	// Below zero clamps to the first row.
	p.StartAt(-5)
	if p.cursor != 0 {
		t.Errorf("StartAt(-5) cursor = %d, want 0", p.cursor)
	}

	// Past the end clamps to the last row.
	p.StartAt(99)
	if p.cursor != 2 {
		t.Errorf("StartAt(99) cursor = %d, want 2", p.cursor)
	}

	// An empty row list is a no-op, not a crash.
	p = NewPicker("t", nil)
	p.StartAt(3)
	if p.cursor != 0 {
		t.Errorf("StartAt on empty rows cursor = %d, want 0", p.cursor)
	}
}

// TestPickerScrolls exercises the windowing: with fewer visible rows than
// rows, the offset follows the cursor so the highlighted row stays on
// screen, and scrolls back when the cursor returns up.
func TestPickerScrolls(t *testing.T) {
	rows := make([]PickerRow, 10)
	for i := range rows {
		rows[i] = PickerRow{ID: string(rune('a' + i)), SearchKey: "row", Label: "row" + string(rune('0'+i))}
	}
	p := NewPicker("t", rows)
	p.height = 6 // rows area = 6 - 4 = 2
	for i := 0; i < 9; i++ {
		p.Update(keyMsg("down"))
	}
	if p.cursor != 9 {
		t.Errorf("cursor = %d, want 9", p.cursor)
	}
	if p.offset != 8 {
		t.Errorf("offset = %d, want 8 (window must follow the cursor down)", p.offset)
	}
	p.Update(keyMsg("up")) // 9 → 8: still visible, window holds
	if p.offset != 8 {
		t.Errorf("offset = %d, want 8 (window holds while cursor visible)", p.offset)
	}
	p.Update(keyMsg("up")) // 8 → 7: window must scroll back up
	if p.offset != 7 {
		t.Errorf("offset = %d, want 7 (window must follow the cursor up)", p.offset)
	}
}

// TestPickerScrollRenderShowsWindow pins the rendered side of scrolling: only
// the rows inside the window appear, the scrolled-off ones do not.
func TestPickerScrollRenderShowsWindow(t *testing.T) {
	rows := make([]PickerRow, 10)
	for i := range rows {
		rows[i] = PickerRow{ID: string(rune('a' + i)), SearchKey: "row", Label: "row" + string(rune('0'+i))}
	}
	p := NewPicker("t", rows)
	p.height = 6
	for i := 0; i < 9; i++ {
		p.Update(keyMsg("down"))
	}
	out := p.View()
	if !strings.Contains(out, "row9") || !strings.Contains(out, "row8") {
		t.Errorf("window should show the bottom rows:\n%s", out)
	}
	if strings.Contains(out, "row0") {
		t.Errorf("scrolled-off rows must not render:\n%s", out)
	}
}

// TestPickerResize covers tea.WindowSizeMsg handling.
func TestPickerResize(t *testing.T) {
	p := NewPicker("t", testPickerRows())
	m, _ := p.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := m.(*Picker)
	if got.width != 100 || got.height != 40 {
		t.Errorf("resize = %dx%d, want 100x40", got.width, got.height)
	}
}

// TestPickerRender pins the rendered surface: title, every row, and the
// footer key hint.
func TestPickerRender(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	out := p.View()
	for _, want := range []string{"Select a job", "aaa01", "Alpha Job", "def02", "Beta Job", "ghj03", "Gamma Job", "enter select", "esc/q cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
}

// --- type-to-filter ---------------------------------------------------------

// TestPickerFilterNarrows covers the core filtering behaviour: typing against
// a row's search key narrows the list, and the cursor is clamped into the
// filtered list.
func TestPickerFilterNarrows(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	p.cursor = 2 // start at the bottom so clamping is observable
	p.Update(keyMsg("b"))
	if p.filter != "b" {
		t.Fatalf("filter = %q, want b", p.filter)
	}
	if got := p.filtered(); len(got) != 1 || got[0].ID != "def02" {
		t.Errorf("filtered = %+v, want just def02", got)
	}
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped into the filtered list)", p.cursor)
	}
	// The filter is case-insensitive.
	p.Update(keyMsg("A")) // "bA" — no match
	if got := p.filtered(); len(got) != 0 {
		t.Errorf("filter bA = %+v, want no rows", got)
	}
}

// TestPickerFilterEscClearsBeforeCancel pins the two-stage esc: a first esc
// clears the filter (restoring the full list) and only a second one cancels.
func TestPickerFilterEscClearsBeforeCancel(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	p.Update(keyMsg("b"))
	p.cursor = 0
	m, cmd := p.Update(keyMsg("esc"))
	if cmd != nil {
		t.Error("esc with an active filter should not quit")
	}
	if got := m.(*Picker); got.filter != "" || len(got.rows) != 3 || len(got.filtered()) != 3 {
		t.Errorf("after esc: filter = %q, rows = %d, want cleared full list", got.filter, len(got.filtered()))
	}
	if res := m.(*Picker).Result(); res.Done {
		t.Errorf("Result() = %+v, want not done (filter was cleared, not cancelled)", res)
	}

	m, cmd = p.Update(keyMsg("esc"))
	if cmd == nil {
		t.Error("second esc (no filter) should quit")
	}
	if res := m.(*Picker).Result(); !res.Done || !res.Cancelled {
		t.Errorf("second esc Result() = %+v, want Done cancel", res)
	}
}

// TestPickerFilterBackspace covers editing the filter: backspace deletes the
// last rune, widening the match set again.
func TestPickerFilterBackspace(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	p.Update(keyMsg("b"))
	p.Update(keyMsg("l")) // "bl" — matches blue/def02
	if got := p.filtered(); len(got) != 1 || got[0].ID != "def02" {
		t.Fatalf("filter bl = %+v, want just def02", got)
	}
	p.Update(keyMsg("backspace")) // "b" again
	if p.filter != "b" {
		t.Errorf("filter = %q, want b", p.filter)
	}
	if got := p.filtered(); len(got) != 1 || got[0].ID != "def02" {
		t.Errorf("filter b = %+v, want just def02", got)
	}
	p.Update(keyMsg("backspace")) // "" — full list back
	if p.filter != "" {
		t.Errorf("filter = %q, want empty", p.filter)
	}
	if len(p.filtered()) != 3 {
		t.Errorf("filtered = %d rows, want 3 (full list)", len(p.filtered()))
	}
	p.Update(keyMsg("backspace")) // empty filter — backspace must be a no-op
	if p.filter != "" {
		t.Errorf("filter = %q after backspace on empty filter, want still empty", p.filter)
	}
}

// TestPickerFilterNavInterplay pins the navigation-vs-input resolution:
// with no filter, j/k navigate and q cancels; once a filter is active the
// same keys extend it, and navigation moves to the arrows. Clearing the
// filter (esc) flips the keys back.
func TestPickerFilterNavInterplay(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	p.Update(keyMsg("j")) // no filter yet — navigates
	if p.cursor != 1 {
		t.Errorf("j with no filter should navigate, cursor = %d, want 1", p.cursor)
	}
	p.Update(keyMsg("q")) // no filter yet — cancels
	if res := p.Result(); !res.Done || !res.Cancelled {
		t.Errorf("q with no filter Result() = %+v, want Done cancel", res)
	}

	// With a filter active the same keys type.
	p = NewPicker("Select a job", testPickerRows())
	p.Update(keyMsg("a")) // start the filter ("aaa01 x" — one match)
	p.Update(keyMsg("j")) // extends the filter, does not navigate
	if p.filter != "aj" {
		t.Errorf("filter = %q, want aj (j types while filtering)", p.filter)
	}
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (j must not navigate while filtering)", p.cursor)
	}
	p.Update(keyMsg("q")) // q types while filtering, does not cancel
	if p.filter != "ajq" {
		t.Errorf("filter = %q, want ajq (q types while filtering)", p.filter)
	}
	if res := p.Result(); res.Done {
		t.Errorf("Result() = %+v, want not done (q typed, not cancelled)", res)
	}
	// "ajq" matches nothing, so arrows have no row to move to.
	p.Update(keyMsg("down"))
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (no matches to navigate)", p.cursor)
	}

	// esc flips the keys back to navigation; a fresh filter matching two
	// rows ("x" — aaa01 and ghj03) lets the arrows move within it.
	p.Update(keyMsg("esc"))
	if p.filter != "" {
		t.Fatalf("filter = %q, want empty after esc", p.filter)
	}
	p.Update(keyMsg("down"))
	if p.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (j-style keys back after clearing)", p.cursor)
	}
	p.Update(keyMsg("x"))
	if len(p.filtered()) != 2 {
		t.Fatalf("filter x = %+v, want two rows", p.filtered())
	}
	p.Update(keyMsg("down"))
	if p.cursor != 1 {
		t.Errorf("down while filtering = %d, want 1 (arrows navigate the filtered list)", p.cursor)
	}
}

// TestPickerFilterSubmit verifies enter reports the filtered row's ID, and
// that a filter matching nothing cannot be submitted.
func TestPickerFilterSubmit(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	p.Update(keyMsg("h")) // matches ghj03 / Gamma Job
	if got := p.filtered(); len(got) != 1 || got[0].ID != "ghj03" {
		t.Fatalf("filter h = %+v, want just ghj03", got)
	}
	m, cmd := p.Update(keyMsg("enter"))
	if cmd == nil {
		t.Error("enter should quit")
	}
	if res := m.(*Picker).Result(); !res.Done || res.ID != "ghj03" {
		t.Errorf("Result() = %+v, want Done submit of ghj03", res)
	}

	p = NewPicker("Select a job", testPickerRows())
	p.Update(keyMsg("z")) // no row contains z
	if len(p.filtered()) != 0 {
		t.Fatalf("filter z = %+v, want no rows", p.filtered())
	}
	m, cmd = p.Update(keyMsg("enter"))
	if cmd != nil {
		t.Error("enter with an empty filtered list should not quit")
	}
	if res := m.(*Picker).Result(); res.Done {
		t.Errorf("Result() = %+v, want not done", res)
	}
}

// TestPickerFilterRender pins the filtered render: the filter line and the
// changed footer appear, and a matching-nothing filter renders "no matches".
func TestPickerFilterRender(t *testing.T) {
	p := NewPicker("Select a job", testPickerRows())
	p.Update(keyMsg("b"))
	out := p.View()
	for _, want := range []string{"filter: b", "def02", "Beta Job", "esc clear filter"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "aaa01") {
		t.Errorf("filtered-out row must not render:\n%s", out)
	}

	p.Update(keyMsg("z")) // "bz" — no matches
	out = p.View()
	if !strings.Contains(out, "no matches") {
		t.Errorf("missing no-matches hint:\n%s", out)
	}
}
