package orchestrate

import (
	"encoding/json"
	"testing"
)

func TestDetectSignalPlainText(t *testing.T) {
	raw := []byte("I read the brief and started on TASK-1.\nNEEDS-HUMAN-INPUT: the brief doesn't say which auth provider to use.\n")
	got, ok := DetectSignal(raw)
	if !ok {
		t.Fatal("DetectSignal: expected a match, got none")
	}
	want := "the brief doesn't say which auth provider to use."
	if got.Reason != want {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, want)
	}
}

func TestDetectSignalJSONResult(t *testing.T) {
	payload := resultPayload{
		Type:   "result",
		Result: "Looked at tasks.md.\nNEEDS-HUMAN-INPUT: TASK-3 conflicts with TASK-5, need a human call.\n",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := DetectSignal(raw)
	if !ok {
		t.Fatal("DetectSignal: expected a match, got none")
	}
	want := "TASK-3 conflicts with TASK-5, need a human call."
	if got.Reason != want {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, want)
	}
}

func TestDetectSignalNoMatch(t *testing.T) {
	raw := []byte("Everything went fine, committed TASK-1.\n")
	_, ok := DetectSignal(raw)
	if ok {
		t.Error("DetectSignal: expected no match, got one")
	}
}

func TestDetectSignalEmpty(t *testing.T) {
	_, ok := DetectSignal(nil)
	if ok {
		t.Error("DetectSignal(nil): expected no match, got one")
	}
	_, ok = DetectSignal([]byte(""))
	if ok {
		t.Error(`DetectSignal(""): expected no match, got one`)
	}
}

// A tool call's own incidental output (e.g. a grep match) sitting outside
// the JSON payload's "result" field must not false-positive — only the
// agent's actual final response (the "result" field) is scanned when the
// output is valid --output-format json.
func TestDetectSignalJSONIgnoresNonResultFields(t *testing.T) {
	raw := []byte(`{"type":"result","result":"All done, no issues.","some_tool_output":"grep found: NEEDS-HUMAN-INPUT: this is inside a code comment, not the agent talking"}`)
	_, ok := DetectSignal(raw)
	if ok {
		t.Error("DetectSignal: matched inside a non-result JSON field, want no match")
	}
}

// Not anchored to the very start of the string — a line further down still
// matches, since the marker is defined as "a line starting with exactly
// NEEDS-HUMAN-INPUT:", not "the first line of the output".
func TestDetectSignalMatchesMidOutput(t *testing.T) {
	raw := []byte("line one\nline two\nNEEDS-HUMAN-INPUT: blocked on a decision\nline four\n")
	got, ok := DetectSignal(raw)
	if !ok {
		t.Fatal("DetectSignal: expected a match")
	}
	if got.Reason != "blocked on a decision" {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, "blocked on a decision")
	}
}

// Case-sensitive and anchored to the start of the line — neither a
// lowercase variant nor the marker appearing mid-line (not at line start)
// should match.
func TestDetectSignalCaseSensitiveAndLineAnchored(t *testing.T) {
	cases := []string{
		"needs-human-input: lowercase should not match\n",
		"some text NEEDS-HUMAN-INPUT: not at the start of the line\n",
	}
	for _, raw := range cases {
		if _, ok := DetectSignal([]byte(raw)); ok {
			t.Errorf("DetectSignal(%q): expected no match", raw)
		}
	}
}

func TestDetectSignalReasonMissing(t *testing.T) {
	raw := []byte("NEEDS-HUMAN-INPUT:\n")
	got, ok := DetectSignal(raw)
	if !ok {
		t.Fatal("DetectSignal: expected a match even with an empty reason")
	}
	if got.Reason != "" {
		t.Errorf("DetectSignal.Reason = %q, want empty", got.Reason)
	}
}
