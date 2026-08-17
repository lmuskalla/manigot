package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// profileCheckout builds a minimal fake manigot checkout (the file
// home.looksLikeCheckout keys off) with an optional .env, and points
// $MANIGOT_HOME at it so config.GetEnv/UpsertEnv resolve there.
func profileCheckout(t *testing.T, env string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if env != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// EnvValue falls back to the process environment; clear every credential
	// key so the tests are hermetic regardless of the host's env.
	for _, k := range []string{
		"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID",
		"ZHIPU_API_KEY", "OPENCODE_API_KEY", "OPENCODE_ZAI_MODEL", "OPENCODE_GO_MODEL", "OPENCODE_ZEN_MODEL",
		"MANIGOT_PROFILE",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("MANIGOT_HOME", dir)
	return dir
}

func TestProfilesHelp(t *testing.T) {
	var out, errOut strings.Builder
	code := runProfiles([]string{"--help"}, strings.NewReader(""), &out, &errOut, false)
	if code != 0 {
		t.Errorf("help exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "mg profiles [name]") {
		t.Errorf("help output missing title:\n%s", out.String())
	}
}

func TestProfilesSetValid(t *testing.T) {
	dir := profileCheckout(t, "# header\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret\n")
	var out, errOut strings.Builder
	code := runProfiles([]string{"zai"}, strings.NewReader(""), &out, &errOut, false)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, errOut.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "MANIGOT_PROFILE=zai") {
		t.Errorf(".env missing MANIGOT_PROFILE=zai:\n%s", env)
	}
	// The other line is preserved.
	if !strings.Contains(string(env), "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret") {
		t.Errorf(".env lost an existing credential:\n%s", env)
	}
	if !strings.Contains(out.String(), "Default profile set to zai") {
		t.Errorf("missing set confirmation:\n%s", out.String())
	}
}

func TestProfilesSetWarnsOnMissingCreds(t *testing.T) {
	profileCheckout(t, "")
	var out, errOut strings.Builder
	code := runProfiles([]string{"opencode-go"}, strings.NewReader(""), &out, &errOut, false)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Warning: OPENCODE_API_KEY is not set") {
		t.Errorf("missing missing-creds warning:\n%s", out.String())
	}
}

func TestProfilesSetOpenCodeZen(t *testing.T) {
	dir := profileCheckout(t, "OPENCODE_API_KEY=zen-key\n")
	var out, errOut strings.Builder
	code := runProfiles([]string{"opencode-zen"}, strings.NewReader(""), &out, &errOut, false)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, errOut.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "MANIGOT_PROFILE=opencode-zen") {
		t.Errorf(".env missing MANIGOT_PROFILE=opencode-zen:\n%s", env)
	}
	if !strings.Contains(out.String(), "Default profile set to opencode-zen") {
		t.Errorf("missing set confirmation:\n%s", out.String())
	}
	// The shared OPENCODE_API_KEY is present — no missing-creds warning.
	if strings.Contains(out.String(), "Warning:") {
		t.Errorf("unexpected warning with the key set:\n%s", out.String())
	}
}

func TestProfilesSetUnknownProfile(t *testing.T) {
	profileCheckout(t, "")
	var out, errOut strings.Builder
	code := runProfiles([]string{"bogus"}, strings.NewReader(""), &out, &errOut, false)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: unknown profile 'bogus'.") {
		t.Errorf("missing unknown-profile error:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Valid profiles: claude-pro zai opencode-go opencode-zen") {
		t.Errorf("missing valid-profiles hint:\n%s", errOut.String())
	}
}

func TestProfilesTooManyArgs(t *testing.T) {
	var out, errOut strings.Builder
	code := runProfiles([]string{"zai", "extra"}, strings.NewReader(""), &out, &errOut, false)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: too many arguments.") {
		t.Errorf("missing too-many-arguments error:\n%s", errOut.String())
	}
}

func TestProfilesListNonTTY(t *testing.T) {
	profileCheckout(t, "MANIGOT_PROFILE=opencode-go\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret\nCLAUDE_ACCOUNT_UUID=uuid\nCLAUDE_EMAIL=e@x.io\nCLAUDE_ORG_UUID=org\nOPENCODE_API_KEY=ok-key\n")
	var out strings.Builder
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	for _, want := range []string{
		"Active default: opencode-go",
		"*opencode-go",
		"✓ ready",
		"✗ missing ZHIPU_API_KEY",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("listing missing %q:\n%s", want, got)
		}
	}
}

func TestProfilesListShowsMissingClaudeKeys(t *testing.T) {
	profileCheckout(t, "MANIGOT_PROFILE=claude-pro\n")
	var out strings.Builder
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "✗ missing CLAUDE_CODE_OAUTH_TOKEN") {
		t.Errorf("claude-pro should report the first missing key:\n%s", out.String())
	}
}

func TestProfilesListInteractiveSelect(t *testing.T) {
	dir := profileCheckout(t, "# header\n")
	var out strings.Builder
	// Empty .env means the active default is claude-pro; select 2 (zai).
	code := runProfiles([]string{}, strings.NewReader("2\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "MANIGOT_PROFILE=zai") {
		t.Errorf(".env missing MANIGOT_PROFILE=zai:\n%s", env)
	}
	if !strings.Contains(out.String(), "Default profile set to zai") {
		t.Errorf("missing set confirmation:\n%s", out.String())
	}
}

func TestProfilesListInteractiveEnterKeeps(t *testing.T) {
	profileCheckout(t, "MANIGOT_PROFILE=zai\n")
	var out strings.Builder
	code := runProfiles([]string{}, strings.NewReader("\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "Keeping zai.") {
		t.Errorf("missing keeping message:\n%s", out.String())
	}
}

func TestProfilesListInteractiveQuit(t *testing.T) {
	var out strings.Builder
	code := runProfiles([]string{}, strings.NewReader("q\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Errorf("quit exit code = %d, want 0", code)
	}
}
