package main

import (
	"github.com/lmuskalla/manigot/internal/ui"
)

// pickerRunFunc runs an interactive single-select picker with the given title
// over the given pre-rendered rows and returns the chosen row's id. ok is
// false when the user dismissed the picker (esc/q) without choosing. It is
// the seam runJobs/runAgents use to run the picker — the same injection
// pattern runJobs already uses for the orphan-removal confirm func — so
// wiring tests can substitute a fake and never start a real Bubble Tea
// program.
type pickerRunFunc func(title string, rows []ui.PickerRow) (id string, ok bool, err error)

// ttyPicker is the real pickerRunFunc: it runs the picker on the terminal's
// alternate screen (ui.RunPicker) and reports the result. A cancelled picker
// reports ok=false so the caller can exit 0 quietly.
func ttyPicker(title string, rows []ui.PickerRow) (string, bool, error) {
	final, err := ui.RunPicker(ui.NewPicker(title, rows))
	if err != nil {
		return "", false, err
	}
	res := final.Result()
	if !res.Done || res.Cancelled {
		return "", false, nil
	}
	return res.ID, true, nil
}
