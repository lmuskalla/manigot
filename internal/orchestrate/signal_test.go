package orchestrate

import (
	"encoding/json"
	"strings"
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

// --- opencode `run --format json` JSONL shape (TASK-4) ----------------------

// opencodeJSONLLine builds one line of `opencode run --format json`'s
// output — a minimal but realistic event, matching what a live invocation
// actually emits (see docs/jobs/foycfl_jdi-for-opencode).
func opencodeJSONLLine(evType, partType, text string) string {
	if partType == "" {
		return `{"type":"` + evType + `","part":{}}`
	}
	return `{"type":"` + evType + `","part":{"type":"` + partType + `","text":"` + text + `"}}`
}

func TestDetectSignalOpenCodeJSONLPlainText(t *testing.T) {
	raw := []byte(strings.Join([]string{
		opencodeJSONLLine("step_start", "", ""),
		opencodeJSONLLine("text", "text", "Looked at tasks.md."),
		opencodeJSONLLine("step_finish", "", ""),
	}, "\n"))

	got := ResultText(raw)
	want := "Looked at tasks.md."
	if got != want {
		t.Errorf("ResultText = %q, want %q", got, want)
	}
	if _, ok := DetectSignal(raw); ok {
		t.Error("DetectSignal: expected no match, got one")
	}
}

func TestDetectSignalOpenCodeJSONLMatch(t *testing.T) {
	raw := []byte(strings.Join([]string{
		opencodeJSONLLine("step_start", "", ""),
		opencodeJSONLLine("tool_use", "", ""),
		opencodeJSONLLine("step_finish", "", ""),
		opencodeJSONLLine("step_start", "", ""),
		opencodeJSONLLine("text", "text", `Looked at tasks.md.\nNEEDS-HUMAN-INPUT: TASK-3 conflicts with TASK-5, need a human call.`),
		opencodeJSONLLine("step_finish", "", ""),
	}, "\n"))

	got, ok := DetectSignal(raw)
	if !ok {
		t.Fatal("DetectSignal: expected a match, got none")
	}
	want := "TASK-3 conflicts with TASK-5, need a human call."
	if got.Reason != want {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, want)
	}
}

// A non-"text" event's incidental data (e.g. a tool's own output) must not
// be scanned — only "text"-type events carry the agent's actual response,
// mirroring how the Claude-JSON path only scans the "result" field.
func TestDetectSignalOpenCodeJSONLIgnoresNonTextEvents(t *testing.T) {
	raw := []byte(strings.Join([]string{
		`{"type":"tool_use","part":{"type":"tool","output":"grep found: NEEDS-HUMAN-INPUT: this is inside a code comment"}}`,
		opencodeJSONLLine("text", "text", "All done, no issues."),
	}, "\n"))

	_, ok := DetectSignal(raw)
	if ok {
		t.Error("DetectSignal: matched inside a non-text event field, want no match")
	}
}

// Raw bytes that don't parse as JSONL at all (e.g. the plain-text --print
// fallback path) must fall through to the existing plain-text scan
// unaffected — this is the "no behavior change for the covered paths"
// guarantee TASK-4 requires.
func TestDetectSignalOpenCodeJSONLFallsBackOnMalformedLine(t *testing.T) {
	raw := []byte(opencodeJSONLLine("text", "text", "partial output") + "\nnot json at all\n")
	got := ResultText(raw)
	if got != string(raw) {
		t.Errorf("ResultText = %q, want raw returned unchanged (not valid JSONL)", got)
	}
}
