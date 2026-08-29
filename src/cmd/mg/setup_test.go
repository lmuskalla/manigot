package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/config"
)

func TestSetupCheckAll(t *testing.T) {
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\nOPENCODE_API_KEY=k\n")
	var out strings.Builder
	code := runSetup([]string{"--check"}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "✓ claude-pro   ready") {
		t.Errorf("claude-pro not ready:\n%s", got)
	}
	if !strings.Contains(got, "✗ zai          missing: ZHIPU_API_KEY   (fix with: mg setup zai)") {
		t.Errorf("zai not missing:\n%s", got)
	}
	if !strings.Contains(got, "✓ opencode-go  ready") {
		t.Errorf("opencode-go not ready:\n%s", got)
	}
	if !strings.Contains(got, "✓ opencode-zen ready") {
		t.Errorf("opencode-zen not ready:\n%s", got)
	}
	if !strings.Contains(got, "✓ opencode-zen-free ready") {
		t.Errorf("opencode-zen-free not ready:\n%s", got)
	}
}

func TestSetupCheckSingleProfile(t *testing.T) {
	profileCheckout(t, "ZHIPU_API_KEY=k\n")
	var out strings.Builder
	code := runSetup([]string{"zai", "--check"}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "✓ zai          ready") {
		t.Errorf("zai not ready:\n%s", got)
	}
	if strings.Contains(got, "claude-pro") {
		t.Errorf("single-profile check leaked other profiles:\n%s", got)
	}
}

func TestSetupCheckClaudeProMissingAll(t *testing.T) {
	profileCheckout(t, "")
	var out strings.Builder
	code := runSetup([]string{"claude-pro", "--check"}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	want := "✗ claude-pro   missing: CLAUDE_CODE_OAUTH_TOKEN CLAUDE_ACCOUNT_UUID CLAUDE_EMAIL CLAUDE_ORG_UUID   (fix with: mg setup claude-pro)"
	if !strings.Contains(out.String(), want) {
		t.Errorf("missing-keys line wrong:\n%s\nwant contains:\n%s", out.String(), want)
	}
}

func TestSetupHelp(t *testing.T) {
	var out strings.Builder
	code := runSetup([]string{"--help"}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "mg setup [profile] [--check]") {
		t.Errorf("help missing title:\n%s", out.String())
	}
}

func TestSetupUnknownArgument(t *testing.T) {
	var out, errOut strings.Builder
	code := runSetup([]string{"bogus"}, strings.NewReader(""), &out, &errOut, false)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: unknown argument 'bogus'.") {
		t.Errorf("missing unknown-arg error:\n%s", errOut.String())
	}
}

func TestSetupTwoProfilesRejected(t *testing.T) {
	var out, errOut strings.Builder
	code := runSetup([]string{"zai", "opencode-go"}, strings.NewReader(""), &out, &errOut, false)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: give a single profile, not several.") {
		t.Errorf("missing several-profiles error:\n%s", errOut.String())
	}
}

func TestSetupRefusesNonTTY(t *testing.T) {
	var out, errOut strings.Builder
	code := runSetup([]string{}, strings.NewReader(""), &out, &errOut, false)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "mg setup: interactive setup needs a terminal.") {
		t.Errorf("missing non-TTY refusal:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Use 'mg setup --check' for a non-interactive status report.") {
		t.Errorf("missing --check hint:\n%s", errOut.String())
	}
}

func TestSetupWizardWritesEnv(t *testing.T) {
	dir := profileCheckout(t, "")
	var out strings.Builder
	// zai wizard: skip the key (bare prompt, empty answer → skipped note),
	// then accept the default model.
	in := "\n\n"
	code := runSetup([]string{"zai"}, strings.NewReader(in), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(env)
	if !strings.Contains(got, "OPENCODE_ZAI_MODEL=zai-coding-plan/glm-5.2") {
		t.Errorf(".env missing default model:\n%s", got)
	}
	if strings.Contains(got, "ZHIPU_API_KEY=") {
		t.Errorf(".env should not contain an empty ZHIPU_API_KEY:\n%s", got)
	}
	if !strings.Contains(out.String(), "(skipped — ZHIPU_API_KEY not set)") {
		t.Errorf("missing skipped note:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Done. Switch the default with: mg profiles <name>") {
		t.Errorf("missing done message:\n%s", out.String())
	}
}

func TestSetupWizardTypedValuesWriteEnv(t *testing.T) {
	dir := profileCheckout(t, "OPENCODE_ZAI_MODEL=old-model\n")
	var out strings.Builder
	// zai wizard with the key unset: prompt 1 is the bare key prompt, prompt 2
	// the model prompt — typing both writes both (overwriting the model).
	code := runSetup([]string{"zai"}, strings.NewReader("new-key\nnew-model\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(env)
	if !strings.Contains(got, "ZHIPU_API_KEY=new-key") {
		t.Errorf(".env missing typed key:\n%s", got)
	}
	if !strings.Contains(got, "OPENCODE_ZAI_MODEL=new-model") {
		t.Errorf(".env missing typed model:\n%s", got)
	}
	if strings.Contains(got, "old-model") {
		t.Errorf(".env kept the old model:\n%s", got)
	}
}

func TestSetupWizardOpenCodeZenWritesEnv(t *testing.T) {
	dir := profileCheckout(t, "")
	var out strings.Builder
	// opencode-zen wizard: type the shared key, then accept the default model.
	in := "zen-key\n\n"
	code := runSetup([]string{"opencode-zen"}, strings.NewReader(in), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(env)
	if !strings.Contains(got, "OPENCODE_API_KEY=zen-key") {
		t.Errorf(".env missing typed key:\n%s", got)
	}
	if !strings.Contains(got, "OPENCODE_ZEN_MODEL=opencode/deepseek-v4-flash") {
		t.Errorf(".env missing default model:\n%s", got)
	}
	if !strings.Contains(out.String(), "Done. Switch the default with: mg profiles <name>") {
		t.Errorf("missing done message:\n%s", out.String())
	}
}

func TestSetupWizardOpenCodeZenFreeWritesEnv(t *testing.T) {
	dir := profileCheckout(t, "")
	var out strings.Builder
	// opencode-zen-free wizard: type the shared key, then accept the default model.
	in := "zen-key\n\n"
	code := runSetup([]string{"opencode-zen-free"}, strings.NewReader(in), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(env)
	if !strings.Contains(got, "OPENCODE_API_KEY=zen-key") {
		t.Errorf(".env missing typed key:\n%s", got)
	}
	if !strings.Contains(got, "OPENCODE_ZEN_FREE_MODEL=opencode/deepseek-v4-flash-free") {
		t.Errorf(".env missing default model:\n%s", got)
	}
	if !strings.Contains(out.String(), "Done. Switch the default with: mg profiles <name>") {
		t.Errorf("missing done message:\n%s", out.String())
	}
}

func TestSetupWizardAlreadyConfiguredSkipsPrompts(t *testing.T) {
	dir := profileCheckout(t, "ZHIPU_API_KEY=existing-key\n")
	var out strings.Builder
	// Key already set: the wizard reports it and only prompts for the model.
	// A single empty answer accepts the default model.
	code := runSetup([]string{"zai"}, strings.NewReader("\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "✓ Already configured (ZHIPU_API_KEY exis…-key).") {
		t.Errorf("missing already-configured line:\n%s", out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "ZHIPU_API_KEY=existing-key") {
		t.Errorf(".env was modified:\n%s", env)
	}
}

func TestSetupWizardAlreadyConfiguredClaudePro(t *testing.T) {
	dir := profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-longsecret\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=me@x.io\nCLAUDE_ORG_UUID=o\n")
	var out strings.Builder
	code := runSetup([]string{"claude-pro"}, strings.NewReader(""), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "✓ Already configured (token sk-a…cret, me@x.io).") {
		t.Errorf("missing already-configured line:\n%s", out.String())
	}
	// Nothing should have been rewritten.
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-longsecret") {
		t.Errorf(".env was modified:\n%s", env)
	}
}

func TestClaudeAccountFromJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, _, _, ok := claudeAccountFromJSON(); ok {
		t.Fatal("missing ~/.claude.json should report not-ok")
	}
	cfg := `{"oauthAccount": {"accountUuid": "uuid-1", "emailAddress": "me@x.io", "organizationUuid": "org-1"}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	uuid, email, org, ok := claudeAccountFromJSON()
	if !ok {
		t.Fatal("complete oauthAccount should report ok")
	}
	if uuid != "uuid-1" || email != "me@x.io" || org != "org-1" {
		t.Errorf("got (%q, %q, %q), want (uuid-1, me@x.io, org-1)", uuid, email, org)
	}
}

func TestClaudeAccountFromJSONIncomplete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Empty oauthAccount or missing fields must report not-ok.
	for _, cfg := range []string{`{}`, `{"oauthAccount": {}}`, `{"oauthAccount": {"accountUuid": "u"}}`} {
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, _, ok := claudeAccountFromJSON(); ok {
			t.Errorf("config %s should report not-ok", cfg)
		}
	}
}

func TestSetupNtfyWritesEnv(t *testing.T) {
	dir := profileCheckout(t, "")
	var out strings.Builder
	// Type all three values: URL, topic, token. One shared bufio.Reader, per
	// the wizard's own contract (a fresh wrap per prompt would lose whatever
	// the previous one buffered past its newline).
	in := bufio.NewReader(strings.NewReader("https://ntfy.example.com\nmy-secret-topic\ns3cret\n"))
	setupNtfy(in, &out)

	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(env)
	if !strings.Contains(got, "NTFY_URL=https://ntfy.example.com") {
		t.Errorf(".env missing typed NTFY_URL:\n%s", got)
	}
	if !strings.Contains(got, "NTFY_TOPIC=my-secret-topic") {
		t.Errorf(".env missing typed NTFY_TOPIC:\n%s", got)
	}
	if !strings.Contains(got, "NTFY_TOKEN=s3cret") {
		t.Errorf(".env missing typed NTFY_TOKEN:\n%s", got)
	}
}

func TestSetupNtfySkipsWhenTopicEmpty(t *testing.T) {
	dir := profileCheckout(t, "")
	var out strings.Builder
	// Accept the default URL, leave topic and token empty (bare prompts).
	setupNtfy(bufio.NewReader(strings.NewReader("\n\n\n")), &out)

	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(env)
	if !strings.Contains(got, "NTFY_URL=https://ntfy.sh") {
		t.Errorf(".env missing the default NTFY_URL:\n%s", got)
	}
	if strings.Contains(got, "NTFY_TOPIC=") {
		t.Errorf(".env should not contain an empty NTFY_TOPIC (opt-in, off by default):\n%s", got)
	}
	if strings.Contains(got, "NTFY_TOKEN=") {
		t.Errorf(".env should not contain an empty NTFY_TOKEN:\n%s", got)
	}
	if !strings.Contains(out.String(), "(skipped — NTFY_TOKEN not set)") {
		t.Errorf("missing skipped note:\n%s", out.String())
	}
}

func TestSetupNtfyAlreadyConfigured(t *testing.T) {
	dir := profileCheckout(t, "NTFY_TOPIC=existing-topic\n")
	var out strings.Builder
	setupNtfy(bufio.NewReader(strings.NewReader("")), &out)
	if !strings.Contains(out.String(), "✓ Already configured (NTFY_TOPIC exis…opic).") {
		t.Errorf("missing already-configured line:\n%s", out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "NTFY_TOPIC=existing-topic") {
		t.Errorf(".env was modified:\n%s", env)
	}
}

func TestSetupWizardUserDefinedProfile(t *testing.T) {
	dir := profileCheckout(t, "")
	if err := config.AddProfile(config.Profile{
		ID: "zen-alt", Label: "Zen Alt", Tool: config.ToolOpenCode,
		AuthKeys: []string{"OPENCODE_API_KEY"}, ModelEnv: "OPENCODE_ZEN_MODEL", ModelDefault: "opencode/custom-model",
	}); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	var out strings.Builder
	// Type the key, accept the default model.
	code := runSetup([]string{"zen-alt"}, strings.NewReader("alt-key\n\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(env)
	if !strings.Contains(got, "OPENCODE_API_KEY=alt-key") {
		t.Errorf(".env missing typed key:\n%s", got)
	}
	if !strings.Contains(got, "OPENCODE_ZEN_MODEL=opencode/custom-model") {
		t.Errorf(".env missing the user profile's default model:\n%s", got)
	}
	if !strings.Contains(out.String(), "zen-alt") {
		t.Errorf("wizard output missing the user profile id:\n%s", out.String())
	}
}

// TestSetupWizardUserDefinedClaudeProfileModel pins TASK-6: a user-defined
// claude-code profile with a ModelEnv/ModelDefault gets an optional model
// prompt, analogous to setupOpenCodeProfile's model block.
func TestSetupWizardUserDefinedClaudeProfileModel(t *testing.T) {
	dir := profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	if err := config.AddProfile(config.Profile{
		ID: "claude-opus", Label: "Claude Opus", Tool: config.ToolClaudeCode,
		AuthKeys: []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ModelEnv: "CLAUDE_OPUS_MODEL", ModelDefault: "opus",
	}); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	var out strings.Builder
	// Credentials are already configured (the fixed CLAUDE_* keys), so the
	// only prompt is the model one — accept the default.
	code := runSetup([]string{"claude-opus"}, strings.NewReader("\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "CLAUDE_OPUS_MODEL=opus") {
		t.Errorf(".env missing the user claude profile's default model:\n%s", env)
	}
	if !strings.Contains(out.String(), "Optional — the model this profile defaults to") {
		t.Errorf("wizard output missing the model prompt:\n%s", out.String())
	}
}

// TestSetupWizardClaudeProNoModelPrompt pins the no-model case: claude-pro
// defines neither ModelEnv nor ModelDefault, so its wizard output is
// unchanged — no model prompt block at all.
func TestSetupWizardClaudeProNoModelPrompt(t *testing.T) {
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	var out strings.Builder
	code := runSetup([]string{"claude-pro"}, strings.NewReader(""), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	if strings.Contains(out.String(), "Optional — the model this profile defaults to") {
		t.Errorf("claude-pro must not show a model prompt:\n%s", out.String())
	}
}

func TestSetupCheckIncludesUserProfile(t *testing.T) {
	profileCheckout(t, "")
	if err := config.AddProfile(config.Profile{ID: "zen-alt", Tool: config.ToolOpenCode, AuthKeys: []string{"OPENCODE_API_KEY"}}); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	var out strings.Builder
	code := runSetup([]string{"--check"}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "✗ zen-alt") || !strings.Contains(got, "missing: OPENCODE_API_KEY") {
		t.Errorf("--check missing the user profile row:\n%s", got)
	}
}

func TestSetupWizardShowsNtfyBlock(t *testing.T) {
	// Pre-configure every profile key so the wizard early-returns for the
	// profiles (model prompts excepted) and only the ntfy block needs input.
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\nZHIPU_API_KEY=k\nOPENCODE_API_KEY=k\n")
	var out strings.Builder
	// zai model, opencode-go model, opencode-zen model, opencode-zen-free
	// model, then ntfy URL/topic/token — seven prompts.
	code := runSetup([]string{}, strings.NewReader("\n\n\n\n\n\n\n"), &out, &strings.Builder{}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "ntfy — push notifications for mg jdi (optional)") {
		t.Errorf("full wizard missing the ntfy block:\n%s", out.String())
	}
}
