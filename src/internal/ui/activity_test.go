package ui

import "testing"

// TestActivityFrameCycles verifies the frame set actually animates (frame 0
// and frame 1 differ) and that activityFrame wraps at the frame count, so a
// monotonically increasing step counter loops through the set instead of
// falling off the end.
func TestActivityFrameCycles(t *testing.T) {
	if activityFrame(0) == activityFrame(1) {
		t.Fatalf("frame 0 (%q) equals frame 1 (%q) — the spinner would not animate", activityFrame(0), activityFrame(1))
	}
	if got := activityFrame(len(activityFrames)); got != activityFrame(0) {
		t.Errorf("activityFrame(len(frames)) = %q, want frame 0 (%q): the cycle must wrap at the frame count", got, activityFrame(0))
	}
	if got := activityFrame(len(activityFrames) + 1); got != activityFrame(1) {
		t.Errorf("activityFrame(len(frames)+1) = %q, want frame 1 (%q)", got, activityFrame(1))
	}
}

// TestActivityFrameDeterministic verifies activityFrame is a pure function of
// the step: steps a full cycle apart yield the same frame, so callers can
// thread a step counter through renders and get a stable, animating glyph.
func TestActivityFrameDeterministic(t *testing.T) {
	for i := 0; i < len(activityFrames); i++ {
		if got := activityFrame(i); got != activityFrame(i+len(activityFrames)) {
			t.Errorf("activityFrame(%d) = %q, want the same frame as step %d (%q)", i, got, i+len(activityFrames), activityFrame(i+len(activityFrames)))
		}
	}
}

// TestActivityFrameHandlesAnyStep verifies the "cycles safely for any step"
// contract: zero, negative (wraps backwards), and arbitrarily huge steps all
// map to a valid frame without panicking or returning empty.
func TestActivityFrameHandlesAnyStep(t *testing.T) {
	for _, step := range []int{0, 1, -1, -len(activityFrames), -len(activityFrames) - 1, len(activityFrames), 1_000_000, 1 << 40} {
		if got := activityFrame(step); got == "" {
			t.Errorf("activityFrame(%d) returned an empty frame, want a real one", step)
		}
	}
	// Negative steps wrap backwards: -1 is the last frame, -len is frame 0.
	if got := activityFrame(-1); got != activityFrame(len(activityFrames)-1) {
		t.Errorf("activityFrame(-1) = %q, want the last frame %q", got, activityFrame(len(activityFrames)-1))
	}
	if got := activityFrame(-len(activityFrames)); got != activityFrame(0) {
		t.Errorf("activityFrame(-len(frames)) = %q, want frame 0 %q", got, activityFrame(0))
	}
}
