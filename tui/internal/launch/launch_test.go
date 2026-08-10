package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShellCommandFormat(t *testing.T) {
	got := shellCommand("/usr/local/bin/manigot", "developer", "irw320", "/home/me/proj", "claude-pro")
	wantInner := "cd '/home/me/proj' && '/usr/local/bin/manigot' --profile 'claude-pro' --agent 'developer' --job 'irw320'"
	if !strings.HasPrefix(got, wantInner) {
		t.Errorf("shellCommand =\n %q\nwant prefix\n %q", got, wantInner)
	}
	// The inner command must be wrapped so a failure holds the window open
	// (TASK-6) rather than the terminal closing immediately.
	if got != holdOnFailure(wantInner) {
		t.Errorf("shellCommand does not wrap its inner command with holdOnFailure:\n%q", got)
	}
}

// --- quick (bare-session) launcher ------------------------------------------

func TestQuickShellCommandFormat(t *testing.T) {
	got := quickShellCommand("/usr/local/bin/manigot", "/home/me/proj", "claude-pro")
	wantInner := "cd '/home/me/proj' && '/usr/local/bin/manigot' --profile 'claude-pro'"
	if !strings.HasPrefix(got, wantInner) {
		t.Errorf("quickShellCommand =\n %q\nwant prefix\n %q", got, wantInner)
	}
	// Must be wrapped by holdOnFailure, exactly like the agent path.
	if got != holdOnFailure(wantInner) {
		t.Errorf("quickShellCommand does not wrap its inner command with holdOnFailure:\n%q", got)
	}
}

// A bare session must never carry --agent or --job — that's the whole point of
// the quick launch path.
func TestQuickShellCommandOmitsAgentAndJob(t *testing.T) {
	got := quickShellCommand("/usr/local/bin/manigot", "/home/me/proj", "claude-pro")
	if strings.Contains(got, "--agent") {
		t.Errorf("quickShellCommand unexpectedly contains --agent: %q", got)
	}
	if strings.Contains(got, "--job") {
		t.Errorf("quickShellCommand unexpectedly contains --job: %q", got)
	}
}

// An empty profile must default to claude-pro, mirroring the agent path and
// scripts/run.sh's own default.
func TestQuickShellCommandDefaultsEmptyProfile(t *testing.T) {
	got := quickShellCommand("/bin/manigot", "/home/me/proj", "")
	if !strings.Contains(got, "--profile 'claude-pro'") {
		t.Errorf("quickShellCommand with empty profile = %q, want it to default to claude-pro", got)
	}
}

func TestQuickShellCommandPassesZAIProfile(t *testing.T) {
	got := quickShellCommand("/bin/manigot", "/home/me/proj", "zai")
	if !strings.Contains(got, "--profile 'zai'") {
		t.Errorf("quickShellCommand with zai profile = %q, want --profile 'zai'", got)
	}
}

// A checkout in a directory with spaces must still produce a single word for
// the manigot path, since the string is re-parsed by osascript / bash -lc.
func TestQuickShellCommandQuotesPathWithSpaces(t *testing.T) {
	got := quickShellCommand("/Users/me/My Projects/manigot/scripts/run.sh", "/tmp/p", "claude-pro")
	if !strings.Contains(got, `'/Users/me/My Projects/manigot/scripts/run.sh'`) {
		t.Errorf("manigot path not quoted as one word in %q", got)
	}
}

// An empty profile must default to claude-pro, matching scripts/run.sh's own
// default, rather than passing an empty --profile value through.
func TestShellCommandDefaultsEmptyProfile(t *testing.T) {
	got := shellCommand("/bin/manigot", "developer", "irw320", "/home/me/proj", "")
	if !strings.Contains(got, "--profile 'claude-pro'") {
		t.Errorf("shellCommand with empty profile = %q, want it to default to claude-pro", got)
	}
}

func TestShellCommandPassesZAIProfile(t *testing.T) {
	got := shellCommand("/bin/manigot", "developer", "irw320", "/home/me/proj", "zai")
	if !strings.Contains(got, "--profile 'zai'") {
		t.Errorf("shellCommand with zai profile = %q, want --profile 'zai'", got)
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
// the manigot path, since the string is re-parsed by osascript / bash -lc.
func TestShellCommandQuotesPathWithSpaces(t *testing.T) {
	got := shellCommand("/Users/me/My Projects/manigot/scripts/run.sh", "reviewer", "abc123", "/tmp/p", "claude-code")
	if !strings.Contains(got, `'/Users/me/My Projects/manigot/scripts/run.sh'`) {
		t.Errorf("manigot path not quoted as one word in %q", got)
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
	got := shellCommand("/bin/manigot", "a", "b", "/path/with'quote", "claude-pro")
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

// --- Jdi (detached, no terminal window — Decision 7a) -----------------------

func TestJdiUnresolvable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("MANIGOT_JDI_BIN", "")
	t.Setenv("MANIGOT_HOME", "")

	err := Jdi("ab0001_x", t.TempDir())
	if err == nil {
		t.Fatal("expected an error when mg-jdi cannot be resolved")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should explain the command is missing; got: %v", err)
	}
}

// The resolved command must be started detached (no terminal emulator
// involved at all, unlike Agent/Quick), by absolute path, in projectRoot,
// with $PWD matching — the same invocation contract hostcmd's NewJob uses.
func TestJdiStartsResolvedCommandDetached(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("MANIGOT_HOME", "")

	root := t.TempDir()
	out := filepath.Join(root, "args.txt")

	stub := filepath.Join(t.TempDir(), "stub.sh")
	script := "#!/bin/sh\n" +
		"{ echo \"argv0=$0\"; echo \"pwd=$PWD\"; echo \"cwd=$(pwd)\"; " +
		"for a in \"$@\"; do echo \"arg=$a\"; done; } > " + out + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_JDI_BIN", stub)

	if err := Jdi("ab0001_x", root); err != nil {
		t.Fatalf("Jdi: %v", err)
	}

	// Jdi starts the process detached and reaps it asynchronously — Start()
	// only guarantees the process began, not that it has finished writing
	// its marker file yet, so poll briefly rather than reading immediately.
	var raw []byte
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(out); err == nil {
			raw = data
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if raw == nil {
		t.Fatal("stub did not run within the timeout")
	}

	recorded := string(raw)
	for _, want := range []string{
		"argv0=" + stub, // invoked by absolute path, not by bare name
		"pwd=" + root,   // $PWD explicitly set for job.FindProjectRoot
		"cwd=" + root,   // and the real cwd agrees
		"arg=--job",
		"arg=ab0001_x",
	} {
		if !strings.Contains(recorded, want) {
			t.Errorf("missing %q in stub record:\n%s", want, recorded)
		}
	}
}
