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
// cadence of the app's activity-indicator timer redraw (see App's
// spinnerTickMsg). ~100ms reads calm but clearly alive; anything below 60ms
// starts to look frantic.
const activityInterval = 100 * time.Millisecond

// statusLifetime is how long a transient status message (the list footer's
// status or the detail footer's status) stays before it blinks then clears —
// see App's statusExpireMsg. Set at arm time (App.setStatus /
// detailView.setStatus) as `statusNow() + statusLifetime`.
const statusLifetime = 3 * time.Second

// statusBlinkInterval is the cadence of the status-expiry timer: the interval
// between statusExpireMsg ticks while a status is set, and how long each
// blink toggle stays on/off.
const statusBlinkInterval = 200 * time.Millisecond

// statusBlinkWindow is the duration immediately before statusLifetime expiry
// during which the status blinks (toggles visible/hidden) before being
// cleared entirely. At the statusBlinkInterval cadence this ~600ms window
// gives roughly three on/off toggles before the status disappears.
const statusBlinkWindow = 600 * time.Millisecond

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
