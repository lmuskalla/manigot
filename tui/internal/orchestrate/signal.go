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
// response (see scripts/entrypoint.sh): a single JSON object whose "result"
// field carries the agent's final response text.
type resultPayload struct {
	Type   string `json:"type"`
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

// DetectSignal scans an agent invocation's captured output (TASK-2's
// `--print` output) for a line matching `^NEEDS-HUMAN-INPUT:` and returns the
// reason text after it. ok is false when no such line is present.
//
// JSON is tried first: when raw parses as a `--output-format json` payload
// (a top-level object with `"type":"result"`), only its "result" field is
// scanned — this is what keeps a marker match from false-positiving on
// incidental tool-call output (e.g. a grep result containing the literal
// string "NEEDS-HUMAN-INPUT:" while the agent reads the codebase), since
// --output-format json separates the agent's actual final response from
// everything else. If raw isn't valid JSON in that shape, DetectSignal falls
// back to scanning the raw bytes directly — the plain `--print` fallback
// path scripts/entrypoint.sh uses if a future claude version ever drops
// --output-format json support.
func DetectSignal(raw []byte) (Signal, bool) {
	text := ResultText(raw)
	m := needsHumanRe.FindStringSubmatch(text)
	if m == nil {
		return Signal{}, false
	}
	return Signal{Reason: strings.TrimSpace(m[1])}, true
}

// ResultText extracts an agent invocation's final-response text from raw:
// the "result" field of a `--output-format json` payload if raw parses as
// one, the concatenated text of every "text" event if raw parses as an
// `opencode run --format json` JSONL stream (see opencodeResultText),
// otherwise raw itself (interpreted as plain text). Exported for
// tui/cmd/jdi's own logging (TASK-7): a human reading mg-jdi's run.log
// should see the agent's prose, not a raw JSON blob.
func ResultText(raw []byte) string {
	var payload resultPayload
	if err := json.Unmarshal(raw, &payload); err == nil && payload.Type == "result" {
		return payload.Result
	}
	if text, ok := opencodeResultText(raw); ok {
		return text
	}
	return string(raw)
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
