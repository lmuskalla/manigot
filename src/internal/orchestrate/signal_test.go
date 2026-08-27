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

// --- real `claude --print --output-format stream-json` output (captured live) --

// realClaudeStreamJSONL is verbatim stdout from a real
// `claude --print --verbose --output-format stream-json "say hi"`
// invocation — claude-code 2.1.247, the version the Dockerfile installs
// unpinned. The invocation failed auth (no credentials in the capture
// environment), but the event stream is emitted in full regardless, which
// pins the parser against the actual shape rather than an approximation:
// a system event, an assistant event whose message.content is an array of
// typed blocks (here one "text" block), and a final "result" event whose
// "result" field carries the final response text. The marker text
// "Not logged in · Please run /login" appears in both the assistant text
// block and the result field — exactly where a real response would carry it.
const realClaudeStreamJSONL = `{"type":"system","subtype":"init","cwd":"/workspace","session_id":"77e6849c-dd31-4c7c-8ae5-68e46a4036ca","tools":["Task","Bash","CronCreate","CronDelete","CronList","DesignSync","Edit","EnterWorktree","ExitWorktree","NotebookEdit","Read","ReportFindings","ScheduleWakeup","SendMessage","Skill","TaskOutput","TaskStop","ToolSearch","WebFetch","WebSearch","Workflow","Write"],"mcp_servers":[],"model":"claude-opus-5[1m]","permissionMode":"default","slash_commands":["deep-research","design-sync","dataviz","update-config","verify","debug","code-review","simplify","batch","fewer-permission-prompts","doctor","loop","claude-api","run","run-skill-generator","agents","auto-mode-setup","autocompact","clear","color","compact","config","context","effort","fast","heapdump","init","mcp","model","__remote-workflow","workflow-launch-exec","reload-skills","rename","security-review","usage","insights","recap","goal","design","design-consent","design-revoke","team-onboarding"],"terminal_slash_commands":["doctor","color"],"apiKeySource":"none","claude_code_version":"2.1.247","output_style":"default","agents":["claude","Explore","general-purpose","Plan","statusline-setup"],"skills":["deep-research","design-sync","dataviz","update-config","verify","debug","code-review","simplify","batch","fewer-permission-prompts","doctor","loop","claude-api","run","run-skill-generator"],"plugins":[],"capabilities":["interrupt_receipt_v1","interrupt_cancel_queued_v1","msg_lifecycle_v1"],"analytics_disabled":false,"product_feedback_disabled":false,"uuid":"f24986f2-d508-4c10-b028-968a18b04ccf","memory_paths":{"auto":"/home/claude/.claude/projects/-workspace/memory/"},"fast_mode_state":"off","fast_mode_disabled_reason":"sdk_opt_in_required"}
{"type":"assistant","message":{"diagnostics":null,"id":"4e00ecc7-57c0-48a8-8cbd-0ee551862004","container":null,"model":"<synthetic>","role":"assistant","stop_details":null,"stop_reason":"stop_sequence","stop_sequence":"","type":"message","usage":{"output_tokens_details":null,"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":null,"cache_creation":{"ephemeral_1h_input_tokens":0,"ephemeral_5m_input_tokens":0},"inference_geo":null,"iterations":null,"speed":null},"content":[{"type":"text","text":"Not logged in · Please run /login"}],"context_management":null},"parent_tool_use_id":null,"session_id":"77e6849c-dd31-4c7c-8ae5-68e46a4036ca","uuid":"79356088-8f50-4794-8170-15001867d8b5","timestamp":"2026-08-27T20:09:10.926Z","error":"authentication_failed","is_api_error_message":true}
{"is_error":true,"duration_api_ms":0,"num_turns":1,"stop_reason":"stop_sequence","session_id":"77e6849c-dd31-4c7c-8ae5-68e46a4036ca","total_cost_usd":0,"usage":{"output_tokens_details":{"thinking_tokens":0},"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard","cache_creation":{"ephemeral_1h_input_tokens":0,"ephemeral_5m_input_tokens":0},"inference_geo":"","iterations":[],"speed":"standard"},"modelUsage":{},"permission_denials":[],"terminal_reason":"api_error","fast_mode_state":"off","fast_mode_disabled_reason":"sdk_opt_in_required","subagent_stats":{"spawned":0,"requested":{"background":0,"foreground":0,"unset":0},"started_in_background":0,"max_depth":0,"spawned_by_subagents":0,"completed":0,"failed":0,"killed":{"parent":0,"user":0,"system":0},"refused":{"depth_limit":0,"concurrency_limit":0,"budget":0},"by_type":{}},"subtype":"success","api_error_status":null,"result":"Not logged in · Please run /login","type":"result","duration_ms":89,"uuid":"01fe2225-59d2-4f4d-8459-ffdb79106698","queued_turn_count":0}
`

// The real stream must extract to just the final result event's text — not
// the raw blob, and not an opencode-style extraction (a claude stream never
// satisfies opencodeResultText: its text lives in message.content, not
// part.text).
func TestResultTextRealClaudeStream(t *testing.T) {
	got := ResultText([]byte(realClaudeStreamJSONL))
	if got != "Not logged in · Please run /login" {
		t.Errorf("ResultText(real claude stream) = %q, want %q", got, "Not logged in · Please run /login")
	}
}

// The marker printed by the agent inside an assistant text event must be
// detected — exactly as it is in the Claude-JSON "result" field and the
// opencode "text" event. Both occurrences of the prose (assistant text block
// and result field) are replaced, matching how a real marker response would
// carry it.
func TestDetectSignalRealClaudeStreamTextEvent(t *testing.T) {
	raw := strings.Replace(realClaudeStreamJSONL, "Not logged in · Please run /login", "Done reviewing.\\nNEEDS-HUMAN-INPUT: TASK-3 contradicts TASK-5, need a call.", -1)
	got, ok := DetectSignal([]byte(raw))
	if !ok {
		t.Fatal("DetectSignal: expected a match on the marker in the real-shape assistant text event, got none")
	}
	want := "TASK-3 contradicts TASK-5, need a call."
	if got.Reason != want {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, want)
	}
}

// DetectSignal scans the assistant events' text content blocks for a claude
// stream — NOT the result event's "result" field. A marker that appears only
// in an assistant text event, with a clean result field, must still match;
// and ResultText must still return the result field's text for the same raw,
// pinning the two extractions apart.
func TestDetectSignalClaudeStreamScansAssistantTextNotResult(t *testing.T) {
	raw := []byte(`{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"NEEDS-HUMAN-INPUT: which auth provider should this use?"}]}}
{"type":"result","result":"Everything is fine."}`)

	if got := ResultText(raw); got != "Everything is fine." {
		t.Errorf("ResultText = %q, want the result event's text %q", got, "Everything is fine.")
	}
	got, ok := DetectSignal(raw)
	if !ok {
		t.Fatal("DetectSignal: expected a match on the marker in the assistant text event, got none")
	}
	want := "which auth provider should this use?"
	if got.Reason != want {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, want)
	}
}

// A tool_use content block's own payload and a user tool_result event's
// content must not be scanned — only assistant "text" blocks are the agent
// talking, mirroring how the opencode path only scans "text" events and the
// Claude-JSON path only scans the "result" field.
func TestDetectSignalClaudeStreamIgnoresToolOutput(t *testing.T) {
	raw := []byte(strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_01","name":"Grep","input":{"pattern":"NEEDS-HUMAN-INPUT:"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_01","content":"grep found: NEEDS-HUMAN-INPUT: this is inside a code comment, not the agent talking"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"All done, no issues."}]}}`,
		`{"type":"result","result":"All done, no issues."}`,
	}, "\n"))

	_, ok := DetectSignal(raw)
	if ok {
		t.Error("DetectSignal: matched inside a tool_use block or user tool_result, want no match")
	}
}

// A stream truncated to a single result event — or emitted with no newlines —
// must still parse through the stable single-result fallback (the first
// branch of ResultText), for both extraction and marker detection.
func TestResultTextSingleResultLineFallback(t *testing.T) {
	raw := []byte(`{"type":"result","result":"All done.","is_error":false,"duration_ms":88}`)
	if got := ResultText(raw); got != "All done." {
		t.Errorf("ResultText = %q, want %q (single-result fallback)", got, "All done.")
	}
	marker := []byte(`{"type":"result","result":"NEEDS-HUMAN-INPUT: which auth provider should this use?","is_error":false}`)
	got, ok := DetectSignal(marker)
	if !ok {
		t.Fatal("DetectSignal: expected a match via the single-result fallback, got none")
	}
	if got.Reason != "which auth provider should this use?" {
		t.Errorf("DetectSignal.Reason = %q, want %q", got.Reason, "which auth provider should this use?")
	}
}

// A malformed/partial stream — a line that doesn't parse as a JSON event —
// falls back to raw, exactly like the opencode JSONL path does: ResultText
// returns the raw bytes unchanged rather than guessing at a partial match.
func TestResultTextClaudeStreamMalformedFallsBackToRaw(t *testing.T) {
	raw := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"partial output"}]}}
not json at all
{"type":"result","result":"hi"}`)
	got := ResultText(raw)
	if got != string(raw) {
		t.Errorf("ResultText = %q, want raw returned unchanged (not a valid claude stream)", got)
	}
	if _, ok := DetectSignal(raw); ok {
		t.Error("DetectSignal: matched on a malformed stream, want no match")
	}
}

// The two JSONL shapes must never cross-detect: a claude stream has no
// part.text events (so opencodeResultText fails and the claude branch wins,
// keyed off a "result" event), and an opencode stream has no "result" event
// (so parseClaudeStream fails and the opencode branch wins).
func TestClaudeStreamAndOpenCodeJSONLNotCrossDetected(t *testing.T) {
	if got := ResultText([]byte(realClaudeStreamJSONL)); got != "Not logged in · Please run /login" {
		t.Errorf("ResultText(claude stream) = %q, want the result event's text (not mis-detected as opencode)", got)
	}
	if got := ResultText([]byte(realOpenCodeJSONL)); got != "DONE" {
		t.Errorf("ResultText(opencode stream) = %q, want %q (not mis-detected as claude)", got, "DONE")
	}
	if _, ok := DetectSignal([]byte(realClaudeStreamJSONL)); ok {
		t.Error("DetectSignal: matched on a clean real claude stream, want no match")
	}
}
