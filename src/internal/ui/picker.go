package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PickerRow is one selectable entry in a Picker: a pre-rendered display
// label plus the identity and search key the picker needs. The caller builds
// Label so rows carry the same column formatting as the plain listing the
// picker replaces; the picker only truncates it to the terminal width. ID is
// what enter reports back through Result, SearchKey is what type-to-filter
// matches against.
type PickerRow struct {
	ID        string
	SearchKey string
	Label     string
}

// PickerResult reports how a Picker finished. Done is false until the user
// submits (enter) or dismisses (esc/q, ctrl+c) the picker.
type PickerResult struct {
	// ID is the chosen row's ID on submit; empty on cancel.
	ID string
	// Cancelled is true when the user dismissed the picker without choosing.
	Cancelled bool
	// Done is true once the picker finished (submit or cancel).
	Done bool
}

// pickerChrome is the number of render rows outside the row list — title
// line, blank spacer, blank line before the footer, footer — that the row
// area must leave room for. Type-to-filter adds one more when a filter is
// active (see rowsAreaHeight).
const pickerChrome = 4

// Picker is a reusable single-select list picker: a title plus a scrollable,
// cursor-highlighted list of pre-rendered rows, narrowed by typing. It is a
// self-contained tea.Model — run it with RunPicker (or any tea.NewProgram)
// and read Result() off the finished model; drive it in tests directly
// through Update/View.
//
// Keys: ↑/↓/k/j move, home/end (and g/G) jump, enter submits, esc/q (and
// ctrl+c) cancel. Typing filters the list against each row's SearchKey
// (case-insensitive substring); backspace edits the filter and esc clears it
// — only falling through to cancel once the filter is already empty. While a
// filter is active every printable key extends it (so j/k/q keep their
// type-to-filter role) and navigation moves to the arrow/home/end keys, so
// the two never fight. The list scrolls when the rows do not fit the
// terminal height. The window size is picked up from tea.WindowSizeMsg;
// before the first one arrives the picker renders at a comfortable 80×24
// default.
type Picker struct {
	title  string
	rows   []PickerRow
	cursor int
	offset int    // first visible row index into the filtered rows
	filter string // type-to-filter query; empty = full list
	width  int
	height int

	result PickerResult
}

// NewPicker builds a picker with the given title over the given rows.
func NewPicker(title string, rows []PickerRow) *Picker {
	return &Picker{title: title, rows: rows, width: 80, height: 24}
}

// StartAt moves the picker's cursor to the given row index before it first
// renders, clamping i into [0, len(rows)-1] and scrolling the window so the
// chosen row is visible. It is a no-op on an empty row list. NewPicker starts
// at row 0; callers that want the cursor on a specific row — e.g. a settings
// list that opens on the active default — call StartAt before running.
func (p *Picker) StartAt(i int) {
	if len(p.rows) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(p.rows) {
		i = len(p.rows) - 1
	}
	p.cursor = i
	p.clampView()
}

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		p.clampView()
		return p, nil
	case tea.KeyMsg:
		return p, p.handleKey(msg)
	}
	return p, nil
}

// handleKey processes a key press, mutating the cursor/filter/offset and
// returning the program command: nil for an ordinary key, tea.Quit once the
// picker is done (submit or cancel).
func (p *Picker) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Keys that mean the same thing whether or not a filter is active.
	switch msg.String() {
	case "enter":
		if rows := p.filtered(); len(rows) > 0 {
			p.result = PickerResult{ID: rows[p.cursor].ID, Done: true}
			return tea.Quit
		}
		return nil
	case "esc":
		// First esc clears a filter; only once the filter is already empty
		// does it fall through to cancel.
		if p.filter != "" {
			p.filter = ""
			break
		}
		p.result = PickerResult{Cancelled: true, Done: true}
		return tea.Quit
	case "ctrl+c":
		p.result = PickerResult{Cancelled: true, Done: true}
		return tea.Quit
	case "up":
		if p.cursor > 0 {
			p.cursor--
		}
		break
	case "down":
		if p.cursor < len(p.filtered())-1 {
			p.cursor++
		}
		break
	case "home":
		p.cursor = 0
		break
	case "end":
		if n := len(p.filtered()); n > 0 {
			p.cursor = n - 1
		}
		break
	case "backspace":
		if p.filter != "" {
			r := []rune(p.filter)
			p.filter = string(r[:len(r)-1])
		}
		break
	default:
		// Remaining keys: printable runes (control keys, which real
		// terminals send with a non-KeyRunes type, are ignored).
		if msg.Type != tea.KeyRunes {
			return nil
		}
		if p.filter != "" {
			// While a filter is active every printable key — including
			// j/k/q/g — extends it; navigation stays on the arrow and
			// home/end keys so the two never fight.
			p.filter += string(msg.Runes)
			break
		}
		switch msg.String() {
		case "q":
			p.result = PickerResult{Cancelled: true, Done: true}
			return tea.Quit
		case "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "j":
			if p.cursor < len(p.filtered())-1 {
				p.cursor++
			}
		case "g":
			p.cursor = 0
		case "G":
			if n := len(p.filtered()); n > 0 {
				p.cursor = n - 1
			}
		default:
			p.filter += string(msg.Runes)
		}
	}
	p.clampView()
	return nil
}

// filtered returns the rows matching the active filter — the full list when
// the filter is empty. Matching is a case-insensitive substring test against
// each row's SearchKey.
func (p *Picker) filtered() []PickerRow {
	if p.filter == "" {
		return p.rows
	}
	q := strings.ToLower(p.filter)
	out := make([]PickerRow, 0, len(p.rows))
	for _, r := range p.rows {
		if strings.Contains(strings.ToLower(r.SearchKey), q) {
			out = append(out, r)
		}
	}
	return out
}

// clampView keeps the cursor inside the filtered row list and the offset
// such that the cursor row is visible, scrolling the window when it is not.
func (p *Picker) clampView() {
	n := len(p.filtered())
	if n == 0 {
		p.cursor, p.offset = 0, 0
		return
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= n {
		p.cursor = n - 1
	}
	area := p.rowsAreaHeight()
	if p.offset < 0 {
		p.offset = 0
	}
	if p.offset > p.cursor {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+area {
		p.offset = p.cursor - area + 1
	}
}

// rowsAreaHeight returns how many rows fit the viewport below the chrome
// (title, spacers, footer) and the filter line when one is shown, at least
// one.
func (p *Picker) rowsAreaHeight() int {
	h := p.height - pickerChrome
	if p.filter != "" {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

// View implements tea.Model.
func (p *Picker) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(p.title))
	b.WriteString("\n\n")
	if p.filter != "" {
		b.WriteString(accentStyle.Render("filter: " + p.filter))
		b.WriteString("\n")
	}

	rows := p.filtered()
	area := p.rowsAreaHeight()
	if len(rows) == 0 {
		// "no matches" only makes sense for a filter that matched nothing; a
		// picker with no rows at all renders an empty list instead.
		if p.filter != "" {
			b.WriteString(dimStyle.Render("  no matches"))
			b.WriteString("\n")
		}
	} else {
		for i := p.offset; i < len(rows) && i < p.offset+area; i++ {
			label := truncate(rows[i].Label, p.width-2)
			if i == p.cursor {
				b.WriteString(selectedStyle.Render("▶ " + label))
			} else {
				b.WriteString(dimStyle.Render("  " + label))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(p.footer()))
	return b.String()
}

// footer renders the key hint, which changes while a filter is active: every
// printable key extends it and esc's first press clears it rather than
// cancelling.
func (p *Picker) footer() string {
	if p.filter != "" {
		return "↑/↓ navigate · enter select · esc clear filter"
	}
	return "↑/↓/k/j navigate · enter select · type to filter · esc/q cancel"
}

// Result reports how the picker finished: the chosen row's ID on submit, or
// Cancelled for esc/q/ctrl+c. Done is false until either happens.
func (p *Picker) Result() PickerResult {
	return p.result
}

// RunPicker runs the picker on the terminal's alternate screen and returns
// the finished model, whose Result() the caller reads. The alt screen gives
// the picker the same full-terminal presence as the TUI, and is restored —
// along with whatever the caller printed before it — when the program ends.
func RunPicker(p *Picker) (*Picker, error) {
	prog := tea.NewProgram(p, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return nil, err
	}
	return final.(*Picker), nil
}
