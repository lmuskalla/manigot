package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/ui"
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
	code := runProfiles([]string{"--help"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
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
	code := runProfiles([]string{"zai"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
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
	code := runProfiles([]string{"opencode-go"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
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
	code := runProfiles([]string{"opencode-zen"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
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
	code := runProfiles([]string{"bogus"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
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
	code := runProfiles([]string{"zai", "extra"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
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
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
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
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "✗ missing CLAUDE_CODE_OAUTH_TOKEN") {
		t.Errorf("claude-pro should report the first missing key:\n%s", out.String())
	}
}

// TestProfilesListInteractiveSelect covers the TTY submit path with an
// injected picker (the seam — no real Bubble Tea program): choosing a profile
// other than the active default writes MANIGOT_PROFILE and prints the set
// confirmation.
func TestProfilesListInteractiveSelect(t *testing.T) {
	dir := profileCheckout(t, "# header\n")
	var out strings.Builder
	// Empty .env means the active default is claude-pro; choose zai.
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("zai", true))
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

// TestProfilesListInteractiveEnterKeeps covers the "enter keeps the current
// default" affordance: choosing the already-active profile prints "Keeping X."
// instead of re-writing .env.
func TestProfilesListInteractiveEnterKeeps(t *testing.T) {
	profileCheckout(t, "MANIGOT_PROFILE=zai\n")
	var out strings.Builder
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("zai", true))
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "Keeping zai.") {
		t.Errorf("missing keeping message:\n%s", out.String())
	}
}

// TestProfilesListInteractiveCancel covers the cancelled-picker path: esc/q
// exits 0 quietly, exactly like the old q-quits path.
func TestProfilesListInteractiveCancel(t *testing.T) {
	var out strings.Builder
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("", false))
	if code != 0 {
		t.Errorf("cancel exit code = %d, want 0", code)
	}
}

// TestProfilesPickerGetsProfileRows pins the picker wiring: on a TTY the
// picker is fed the title plus one pre-rendered row per profile (ID,
// padded-column label with the * active mark, search key id+label+tool+model
// +creds), and the cursor start index is the active default's position — so a
// bare enter keeps it.
func TestProfilesPickerGetsProfileRows(t *testing.T) {
	// opencode-go is the active default — the third of the four profiles, so
	// the picker must open on start index 2.
	profileCheckout(t, "MANIGOT_PROFILE=opencode-go\n")

	var gotTitle string
	var gotRows []ui.PickerRow
	var gotStart int
	pick := func(title string, rows []ui.PickerRow, start int) (string, bool, error) {
		gotTitle, gotRows, gotStart = title, rows, start
		return "", false, nil // cancelled
	}
	var out strings.Builder
	code := runProfiles([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pick)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (cancel exits quietly)", code)
	}
	if gotTitle != "Select the default profile" {
		t.Errorf("picker title = %q, want %q", gotTitle, "Select the default profile")
	}
	if len(gotRows) != 4 {
		t.Fatalf("picker rows = %d, want 4", len(gotRows))
	}
	wantIDs := []string{"claude-pro", "zai", "opencode-go", "opencode-zen"}
	for i, want := range wantIDs {
		if gotRows[i].ID != want {
			t.Errorf("row %d ID = %q, want %q", i, gotRows[i].ID, want)
		}
	}
	// The search key carries id + label + tool + model + creds so
	// type-to-filter can match on any of them.
	wantSearchParts := [][]string{
		{"claude-pro", "Claude Code", "claude-code", "(Claude Code default)", "✗ missing CLAUDE_CODE_OAUTH_TOKEN"},
		{"zai", "Z.AI Coding Plan", "opencode", "zai-coding-plan/glm-5.2", "✗ missing ZHIPU_API_KEY"},
		{"opencode-go", "OpenCode · Go", "opencode", "opencode-go/glm-5.2", "✗ missing OPENCODE_API_KEY"},
		{"opencode-zen", "OpenCode · Zen", "opencode", "opencode/deepseek-v4-flash-free", "✗ missing OPENCODE_API_KEY"},
	}
	for i, parts := range wantSearchParts {
		key := gotRows[i].SearchKey
		for _, want := range parts {
			if !strings.Contains(key, want) {
				t.Errorf("row %d search key missing %q: %q", i, want, key)
			}
		}
	}
	// The label keeps the padded columns (tool/model/creds) and the * active
	// mark on the active default's row.
	if !strings.Contains(gotRows[0].Label, "claude-code") || !strings.Contains(gotRows[0].Label, "✗ missing CLAUDE_CODE_OAUTH_TOKEN") {
		t.Errorf("row 0 label missing tool/model/creds columns: %q", gotRows[0].Label)
	}
	if !strings.HasPrefix(gotRows[2].Label, "*opencode-go") {
		t.Errorf("row 2 (active default) label missing the * mark: %q", gotRows[2].Label)
	}
	if gotStart != 2 {
		t.Errorf("start index = %d, want 2 (opencode-go is the active default)", gotStart)
	}
}
