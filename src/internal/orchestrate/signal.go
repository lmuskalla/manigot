package orchestrate

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Signal is a detected NEEDS-HUMAN-INPUT marker (see docs/AGENTS.md) found
// in an agent invocation's captured output.
type Signal struct {
	// Reason is the one-sentence explanation following the marker, with
	// surrounding whitespace trimmed. May be empty if the agent omitted it
	// despite emitting the marker line itself.
	Reason string
}

// needsHumanRe matches a line starting with exactly "NEEDS-HUMAN-INPUT:"
// (anchored, case-sensitive — see docs/AGENTS.md), capturing whatever
// follows it on the same line as the reason.
var needsHumanRe = regexp.MustCompile(`(?m)^NEEDS-HUMAN-INPUT:[ \t]*(.*)$`)

// resultPayload is the shape of a `claude --print --output-format json`
// response (the stable single-result form — kept as the defensive fallback
// now that scripts/entrypoint.sh's claude --print branch uses stream-json,
// which has been more version-volatile): a single JSON object whose "result"
// field carries the agent's final response text.
type resultPayload struct {
	Type   string `json:"type"`
	Result string `json:"result"`
}

// claudeStreamEvent is one line of `claude --print --output-format
// stream-json` output (see scripts/entrypoint.sh): a stream of typed events
// — system, assistant, user, result — one JSON object per line (JSONL).
// Assistant events carry the agent's message.content blocks (type "text" or
// "tool_use"); user events carry tool_result blocks; the final event is type
// "result" with a "result" field carrying the final response text. Only the
// fields parseClaudeStream needs are declared; every other field round-trips
// as a zero value.
type claudeStreamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	Result string `json:"result"`
}

// opencodeEvent is one line of `opencode run --format json`'s output (see
// scripts/entrypoint.sh's opencode --print branch): a stream of typed
// events (step_start, tool_use, text, step_finish, ...), one JSON object per
// line (JSONL) rather than Claude's single top-level object. Only "text"
// events carry response prose — the fields below are the ones
// opencodeResultText needs; every other event type round-trips as a zero
// Part, which is fine since only Type=="text" is inspected.
type opencodeEvent struct {
	Type string `json:"type"`
	Part struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"part"`
}

// DetectSignal scans an agent invocation's captured output (the `--print`
// output) for a line matching `^NEEDS-HUMAN-INPUT:` and returns the
// reason text after it. ok is false when no such line is present.
//
// JSON is tried first: when raw parses as a `--output-format json` payload
// (a top-level object with `"type":"result"`), only its "result" field is
// scanned — this is what keeps a marker match from false-positiving on
// incidental tool-call output (e.g. a grep result containing the literal
// string "NEEDS-HUMAN-INPUT:" while the agent reads the codebase), since
// --output-format json separates the agent's actual final response from
// everything else. When raw parses as Claude's --output-format stream-json
// shape, only assistant events' text content blocks are scanned — never tool
// output (a "tool_use" content block's or a user "tool_result" event's
// payload could contain the marker string incidentally). If raw isn't valid
// JSON in either shape, DetectSignal falls back to scanning the raw bytes
// directly — the plain `--print` fallback path scripts/entrypoint.sh uses if
// a future claude version ever drops --output-format stream-json support.
func DetectSignal(raw []byte) (Signal, bool) {
	text := ResultText(raw)
	// The stream-json shape overrides ResultText's result-field extraction:
	// the marker lives in what the assistant actually said, and only
	// assistant text content blocks are the agent talking. A degenerate
	// stream with no assistant text at all (a lone result event — the old
	// single-result shape) keeps ResultText's result-field scan instead.
	if stream, ok := parseClaudeStream(raw); ok && len(stream.assistant) > 0 {
		text = strings.Join(stream.assistant, "\n")
	}
	m := needsHumanRe.FindStringSubmatch(text)
	if m == nil {
		return Signal{}, false
	}
	return Signal{Reason: strings.TrimSpace(m[1])}, true
}

// ResultText extracts an agent invocation's final-response text from raw:
// the "result" field of a `--output-format json` payload if raw parses as
// one, the concatenated text of every "text" event if raw parses as an
// `opencode run --format json` JSONL stream (see opencodeResultText), the
// final "result" event's text if raw parses as Claude's --output-format
// stream-json shape (see parseClaudeStream), otherwise raw itself
// (interpreted as plain text). Exported for mg-jdi's
// own logging: a human reading the run.log should see the agent's prose,
// not a raw JSON blob.
func ResultText(raw []byte) string {
	var payload resultPayload
	if err := json.Unmarshal(raw, &payload); err == nil && payload.Type == "result" {
		return payload.Result
	}
	if text, ok := opencodeResultText(raw); ok {
		return text
	}
	if stream, ok := parseClaudeStream(raw); ok {
		if stream.result != "" {
			return stream.result
		}
		return strings.Join(stream.assistant, "\n")
	}
	return string(raw)
}

// claudeStream is the parsed shape of a stream-json capture: the final
// "result" event's "result" text, and the text content blocks of every
// assistant event, in order.
type claudeStream struct {
	result    string
	assistant []string
}

// parseClaudeStream parses raw as `claude --print --output-format
// stream-json` output: one JSON object per non-blank line (JSONL), as
// opposed to Claude's single top-level object (tried first, in ResultText)
// and opencode's part-carrying JSONL (tried before this). ok is false unless
// every non-blank line parses cleanly as a JSON object with a non-empty
// "type" field and at least one type "result" event was found — the key-off
// that keeps a claude stream from being mis-detected as opencode JSONL
// (which never has a "result" event) and vice versa (opencode events carry
// part.text, never message.content, so a claude stream never satisfies
// opencodeResultText). Tool output is never collected: only assistant
// "text" content blocks are, never "tool_use" blocks or user "tool_result"
// events.
func parseClaudeStream(raw []byte) (claudeStream, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return claudeStream{}, false
	}
	var stream claudeStream
	sawResult := false
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev claudeStreamEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.Type == "" {
			return claudeStream{}, false
		}
		switch ev.Type {
		case "assistant":
			for _, block := range ev.Message.Content {
				if block.Type == "text" {
					stream.assistant = append(stream.assistant, block.Text)
				}
			}
		case "result":
			sawResult = true
			stream.result = ev.Result
		}
	}
	if !sawResult {
		return claudeStream{}, false
	}
	return stream, true
}

// opencodeResultText concatenates the text of every "text" event in raw,
// interpreted as `opencode run --format json` output: one JSON object per
// non-blank line (JSONL), as opposed to Claude's single top-level object
// (tried first, above). ok is false unless every non-blank line parses
// cleanly as a JSON object with a non-empty "type" field and at least one
// "text" event was found — a single line that doesn't fit that shape (e.g.
// plain prose, or the `--print` plain-text fallback path) means raw isn't
// this format at all, so ResultText falls back to scanning it directly
// instead of guessing at a partial match.
func opencodeResultText(raw []byte) (string, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "", false
	}
	lines := strings.Split(trimmed, "\n")
	var texts []string
	sawText := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev opencodeEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.Type == "" {
			return "", false
		}
		if ev.Type == "text" && ev.Part.Type == "text" {
			texts = append(texts, ev.Part.Text)
			sawText = true
		}
	}
	if !sawText {
		return "", false
	}
	return strings.Join(texts, "\n"), true
}
