package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hostCheckout builds a minimal manigot checkout with the given .env content
// and points $MANIGOT_HOME at it, so runHost's session flow (ResolveProfile /
// CheckAuth) resolves credentials hermetically. Every credential key is
// cleared from the process env first — config.EnvValue falls back to the
// process environment when the .env doesn't define a key, and the test
// machine's own env may carry real credentials.
func hostCheckout(t *testing.T, env string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{
		"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID",
		"ANTHROPIC_API_KEY", "ZHIPU_API_KEY", "OPENCODE_API_KEY", "OPENAI_API_KEY",
		"OPENCODE_ZAI_MODEL", "OPENCODE_GO_MODEL", "OPENCODE_MODEL", "MANIGOT_PROFILE",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("MANIGOT_HOME", dir)
	return dir
}

func TestRunHostPrintRejected(t *testing.T) {
	hostCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	t.Chdir(t.TempDir())
	var out, errOut strings.Builder
	if code := runHost([]string{"--print"}, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("--print exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "--print is not supported with mg host") {
		t.Errorf("missing print rejection:\n%s", errOut.String())
	}
}

// TestRunHostMissingBinary exercises the full host flow (profile → root →
// auth → build) ending in the CLI-presence check: with PATH stripped, the
// host CLI cannot be found and mg host must fail with the clear
// "not installed on the host" error rather than a confusing exec failure.
func TestRunHostMissingBinary(t *testing.T) {
	hostCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	t.Chdir(t.TempDir())
	t.Setenv("PATH", "/nonexistent")
	var out, errOut strings.Builder
	if code := runHost(nil, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("missing-binary exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "not installed on the host") {
		t.Errorf("missing missing-binary error:\n%s", errOut.String())
	}
}

func TestRunHostInvalidProfile(t *testing.T) {
	var out, errOut strings.Builder
	if code := runHost([]string{"--profile", "bogus"}, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("invalid-profile exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "--profile must be one of: claude-pro|zai|opencode-go") {
		t.Errorf("missing profile error:\n%s", errOut.String())
	}
}

func TestRunHostAuthRequired(t *testing.T) {
	hostCheckout(t, "") // no credentials in the checkout's .env
	t.Chdir(t.TempDir())
	var out, errOut strings.Builder
	if code := runHost(nil, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("no-auth exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "CLAUDE_CODE_OAUTH_TOKEN is not set") {
		t.Errorf("missing auth error:\n%s", errOut.String())
	}
}

func TestPrintHelpListsHost(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printHelp()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	help := string(out)
	for _, want := range []string{"mg host", "mg wild", "mg --agent/-a <name>", "mg --job/-j <id>"} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing %q", want)
		}
	}
}
