package ui

import "time"

// activityFrames is the frame set of the activity indicator ("loading
// indicator for jdi" job): braille-dot spinners, the same family opencode
// shows in its bottom-left corner. The frames are deliberately kept in this
// one place so swapping the whole set (e.g. for the maximally-compatible
// ASCII "|/-\" fallback) is a single-line change — see the task's open
// questions.
var activityFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// activityInterval is how long each activity-indicator frame is shown — the
// cadence of the app's one timer-driven redraw (see App's spinnerTickMsg).
// ~100ms reads calm but clearly alive; anything below 60ms starts to look
// frantic.
const activityInterval = 100 * time.Millisecond

// activityFrame returns the activity-indicator frame for step, cycling
// safely through activityFrames for any step value: zero, negative (wraps
// backwards), or arbitrarily huge (modulo keeps it in range without
// overflow). Deterministic — the same step always yields the same frame, so
// callers can thread a step counter through renders and get a stable,
// animating glyph.
func activityFrame(step int) string {
	if len(activityFrames) == 0 {
		return ""
	}
	step %= len(activityFrames)
	if step < 0 {
		step += len(activityFrames)
	}
	return activityFrames[step]
}
