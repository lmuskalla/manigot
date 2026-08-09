package launch

import (
	"strings"
	"testing"
)

func TestShellCommandFormat(t *testing.T) {
	got := shellCommand("/usr/local/bin/safecode", "developer", "irw320", "/home/me/proj", "claude-code")
	wantInner := "cd '/home/me/proj' && '/usr/local/bin/safecode' --tool 'claude-code' --agent 'developer' --job 'irw320'"
	if !strings.HasPrefix(got, wantInner) {
		t.Errorf("shellCommand =\n %q\nwant prefix\n %q", got, wantInner)
	}
	// The inner command must be wrapped so a failure holds the window open
	// (TASK-6) rather than the terminal closing immediately.
	if got != holdOnFailure(wantInner) {
		t.Errorf("shellCommand does not wrap its inner command with holdOnFailure:\n%q", got)
	}
}

// An empty tool must default to claude-code, matching scripts/run.sh's own
// default, rather than passing an empty --tool value through.
func TestShellCommandDefaultsEmptyTool(t *testing.T) {
	got := shellCommand("/bin/safecode", "developer", "irw320", "/home/me/proj", "")
	if !strings.Contains(got, "--tool 'claude-code'") {
		t.Errorf("shellCommand with empty tool = %q, want it to default to claude-code", got)
	}
}

func TestShellCommandPassesOpencodeTool(t *testing.T) {
	got := shellCommand("/bin/safecode", "developer", "irw320", "/home/me/proj", "opencode")
	if !strings.Contains(got, "--tool 'opencode'") {
		t.Errorf("shellCommand with opencode tool = %q, want --tool 'opencode'", got)
	}
}

// TestHoldOnFailureExitsCleanlyOnSuccess documents the intended shape of the
// wrapper: it must not alter the wrapped command itself, only append
// exit-code-conditional hold logic after it.
func TestHoldOnFailureExitsCleanlyOnSuccess(t *testing.T) {
	got := holdOnFailure("true")
	if !strings.HasPrefix(got, "true;") {
		t.Errorf("holdOnFailure should prefix the wrapped command unchanged, got %q", got)
	}
	if !strings.Contains(got, `ec=$?`) {
		t.Errorf("holdOnFailure should capture the exit code, got %q", got)
	}
	if !strings.Contains(got, `if [ "$ec" -ne 0 ]`) {
		t.Errorf("holdOnFailure should only hold on a non-zero exit, got %q", got)
	}
	if !strings.Contains(got, `exit "$ec"`) {
		t.Errorf("holdOnFailure should preserve the original exit code, got %q", got)
	}
}

// A checkout in a directory with spaces must still produce a single word for
// the safecode path, since the string is re-parsed by osascript / bash -lc.
func TestShellCommandQuotesPathWithSpaces(t *testing.T) {
	got := shellCommand("/Users/me/My Projects/safecode/scripts/run.sh", "reviewer", "abc123", "/tmp/p", "claude-code")
	if !strings.Contains(got, `'/Users/me/My Projects/safecode/scripts/run.sh'`) {
		t.Errorf("safecode path not quoted as one word in %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"":           "''",
		"plain":      "'plain'",
		"with space": "'with space'",
		"a'b":        "'a'\\''b'", // embedded quote escaped via '\''
		`back\slash`: `'back\slash'`,
	}
	for in, want := range cases {
		got := shellQuote(in)
		if got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestShellQuoteNeutralizesInjection ensures a value engineered to break out of
// its quotes cannot inject shell metacharacters: after quoting, the dangerous
// substring must be contained inside the escaped sequence, not terminate the
// outer single-quoted word.
func TestShellQuoteNeutralizesInjection(t *testing.T) {
	mal := "'; rm -rf /; '"
	q := shellQuote(mal)
	// The result must start and end with a single quote and contain no raw
	// unescaped "; rm -rf" outside of quotes — i.e. the only way "rm" appears
	// is inside the escaped inner segment.
	if !strings.HasPrefix(q, "'") || !strings.HasSuffix(q, "'") {
		t.Fatalf("quoted value not single-quote wrapped: %q", q)
	}
	// Re-splitting on the escape idiom, no segment should be a bare shell
	// command. Concretely: there must be no occurrence of "; '" (close+open
	// without the escape) which would indicate breakout.
	if strings.Contains(q, "; '") && !strings.Contains(q, `'\''`) {
		t.Errorf("quoting allowed breakout in %q", q)
	}
	// And the canonical escaped form must be present.
	if !strings.Contains(q, `'\''`) {
		t.Errorf("expected the \\'' escape idiom in %q", q)
	}
}

func TestShellCommandQuoteEscape(t *testing.T) {
	// A projectRoot with a quote still ends up as one safe argument.
	got := shellCommand("/bin/safecode", "a", "b", "/path/with'quote", "claude-code")
	// The root must appear escaped, not as a raw close-quote.
	if !strings.Contains(got, `'/path/with'\''quote'`) {
		t.Errorf("root quote not escaped in %q", got)
	}
}

func TestBuildCmdSmoke(t *testing.T) {
	// buildCmd must not panic and must return either a runnable command or a
	// descriptive error, depending on what is installed in this environment.
	cmd, desc, err := buildCmd("echo hi")
	if err != nil {
		if !strings.Contains(err.Error(), "terminal launcher") {
			t.Errorf("unexpected error: %v", err)
		}
		// desc should be empty on error.
		if desc != "" {
			t.Errorf("on error, desc = %q, want empty", desc)
		}
		return
	}
	if cmd == nil {
		t.Fatal("buildCmd returned nil cmd and nil error")
	}
	if cmd.Path == "" {
		t.Error("returned cmd has empty Path")
	}
	if desc == "" {
		t.Error("returned cmd has empty description")
	}
}
