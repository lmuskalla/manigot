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

// --- real `opencode run --format json` output (captured live, 63quv2) ------

// realOpenCodeJSONL is verbatim stdout from a real `opencode run "List the
// files in the current directory with the bash ls command..." --format json`
// invocation — opencode-ai 1.18.16, the version the Dockerfile installs
// unpinned. It pins the parser against the actual event shape, not an
// approximation: step_start events carry part.type "step-start" (no text),
// tool_use events carry part.type "tool" with the tool's own output inside
// part.state.output rather than part.text, step_finish events carry
// part.type "step-finish" with tokens/cost, and the agent's response prose
// appears only in a "text"-typed event's part.text (here "DONE").
const realOpenCodeJSONL = `{"type":"step_start","timestamp":1786618464127,"sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","part":{"id":"prt_ffac2477c0014ppVfdepkHU94l","messageID":"msg_ffac240b4001K6GGOCvL5ukQzJ","sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","snapshot":"e5125c9478968a4a4526711ea47879c76b5c0a08","type":"step-start"}}
{"type":"tool_use","timestamp":1786618465184,"sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","part":{"type":"tool","tool":"bash","callID":"call_00_ET_TZxYVSgG65xbQXy2xoET3952","state":{"status":"completed","input":{"command":"ls"},"output":"AGENTS.md\nCONSOLIDATION-BRIEF.md\nDockerfile\nMakefile\nREADME.md\nagents\nassets\ncmd\ndocs\ngo.mod\ngo.sum\ninternal\nproject-template\nscripts\n","metadata":{"output":"AGENTS.md\nCONSOLIDATION-BRIEF.md\nDockerfile\nMakefile\nREADME.md\nagents\nassets\ncmd\ndocs\ngo.mod\ngo.sum\ninternal\nproject-template\nscripts\n","exit":0,"truncated":false},"title":"ls","time":{"start":1786618465099,"end":1786618465176}},"id":"prt_ffac24ad60013uXEiPO8j0pfhM","sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","messageID":"msg_ffac240b4001K6GGOCvL5ukQzJ"}}
{"type":"step_finish","timestamp":1786618465205,"sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","part":{"id":"prt_ffac24baf001mY1uWfvIJGFLWQ","reason":"tool-calls","snapshot":"e5125c9478968a4a4526711ea47879c76b5c0a08","messageID":"msg_ffac240b4001K6GGOCvL5ukQzJ","sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","type":"step-finish","tokens":{"total":13325,"input":98,"output":43,"reasoning":0,"cache":{"write":0,"read":13184}},"cost":0.0000313376}}
{"type":"step_start","timestamp":1786618466385,"sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","part":{"id":"prt_ffac2504e001Wm9GQ3LLN4MgnM","messageID":"msg_ffac24bc600122PO0yr6sPGZVh","sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","snapshot":"e5125c9478968a4a4526711ea47879c76b5c0a08","type":"step-start"}}
{"type":"text","timestamp":1786618467248,"sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","part":{"id":"prt_ffac25351001mv18y7YW5IPaOC","messageID":"msg_ffac24bc600122PO0yr6sPGZVh","sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","type":"text","text":"DONE","time":{"start":1786618467153,"end":1786618467240}}}
{"type":"step_finish","timestamp":1786618467269,"sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","part":{"id":"prt_ffac253c1001fEYHCFYfTxbx2z","reason":"stop","snapshot":"e5125c9478968a4a4526711ea47879c76b5c0a08","messageID":"msg_ffac24bc600122PO0yr6sPGZVh","sessionID":"ses_0053dc0a6ffeLfOVWup0QCNjl2","type":"step-finish","tokens":{"total":13384,"input":69,"output":3,"reasoning":0,"cache":{"write":0,"read":13312}},"cost":0.0000238868}}
`

// The real output shape must extract to just the agent's response prose and
// must not false-positive on the tool_use event (whose bash output carries
// plenty of non-text payload, e.g. the `ls` listing).
func TestResultTextRealOpenCodeOutput(t *testing.T) {
	got := ResultText([]byte(realOpenCodeJSONL))
	if got != "DONE" {
		t.Errorf("ResultText(real opencode output) = %q, want %q", got, "DONE")
	}
	if _, ok := DetectSignal([]byte(realOpenCodeJSONL)); ok {
		t.Error("DetectSignal: expected no match on a clean real opencode run, got one")
	}
}

// The marker printed by the agent inside the final "text" event (the only
// place its response prose lives) must be detected, exactly as it is in the
// Claude-JSON "result" field.
func TestDetectSignalRealOpenCodeTextEvent(t *testing.T) {
	raw := strings.Replace(realOpenCodeJSONL, `"text":"DONE"`, `"text":"Done reviewing.\nNEEDS-HUMAN-INPUT: TASK-3 contradicts TASK-5, need a call."`, 1)
	got, ok := DetectSignal([]byte(raw))
	if !ok {
		t.Fatal("DetectSignal: expected a match on the marker in the real-shape text event, got none")
	}
	want := "TASK-3 contradicts TASK-5, need a call."
	if got.Reason != want {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, want)
	}
}

// A tool_use event's own output (part.state.output — the bash tool's stdout)
// must not be scanned for the marker, even when it contains the literal
// string — only "text" events carry the agent talking, mirroring the
// Claude-JSON path's "result"-field-only scan.
func TestDetectSignalRealOpenCodeToolOutputIgnored(t *testing.T) {
	raw := strings.Replace(realOpenCodeJSONL, `"command":"ls"`, `"command":"grep -r NEEDS-HUMAN-INPUT: ."`, 1)
	raw = strings.Replace(raw, `"output":"AGENTS.md`, `"output":"NEEDS-HUMAN-INPUT: this is inside a grep result, not the agent talking\nAGENTS.md`, 1)
	if _, ok := DetectSignal([]byte(raw)); ok {
		t.Error("DetectSignal: matched inside a tool_use event's state.output, want no match")
	}
}
