package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/config"
)

// checkout builds a minimal fake manigot checkout and points $MANIGOT_HOME at
// it so config.EnvValue resolves there. env is the .env content ("" = none).
func checkout(t *testing.T, env string) string {
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
	// Hermetic: clear every credential key so inherited host env can't leak in.
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

func TestParseArgs(t *testing.T) {
	o := ParseArgs([]string{"--agent", "analyst", "--job", "abc123_x", "--prompt", "hello", "--tool", "claude-code", "--profile", "zai", "--print", "extra"})
	if o.Agent != "analyst" || o.Job != "abc123_x" || o.Prompt != "hello" || o.Tool != "claude-code" || o.Profile != "zai" || !o.Print {
		t.Errorf("ParseArgs = %+v", o)
	}
	if len(o.Pass) != 1 || o.Pass[0] != "extra" {
		t.Errorf("passthrough = %v, want [extra]", o.Pass)
	}
}

func TestParseArgsBarePrint(t *testing.T) {
	o := ParseArgs([]string{"--print"})
	if !o.Print {
		t.Error("--print not parsed")
	}
}

func TestResolveProfileExplicitWins(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=k\nCLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	info, err := ResolveProfile(Options{Profile: "claude-pro"})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.Profile != "claude-pro" || info.Tool != config.ToolClaudeCode {
		t.Errorf("explicit --profile claude-pro = %+v", info)
	}
}

func TestResolveProfileInvalidExplicit(t *testing.T) {
	checkout(t, "")
	_, err := ResolveProfile(Options{Profile: "bogus"})
	if err == nil || !strings.Contains(err.Error(), "Error: --profile must be one of: claude-pro|zai|opencode-go (got 'bogus').") {
		t.Errorf("invalid --profile error = %v", err)
	}
}

func TestResolveProfileLegacyToolClaudeCode(t *testing.T) {
	checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	info, err := ResolveProfile(Options{Tool: "claude-code"})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.Profile != "claude-pro" || info.Tool != config.ToolClaudeCode {
		t.Errorf("--tool claude-code = %+v", info)
	}
}

func TestResolveProfileInvalidLegacyTool(t *testing.T) {
	checkout(t, "")
	_, err := ResolveProfile(Options{Tool: "bogus"})
	if err == nil || !strings.Contains(err.Error(), "Error: --tool must be 'claude-code' or 'opencode' (got 'bogus').") {
		t.Errorf("invalid --tool error = %v", err)
	}
}

func TestResolveProfileDefaultsToManigotProfile(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=k\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.Profile != "zai" || info.Tool != config.ToolOpenCode {
		t.Errorf("default via MANIGOT_PROFILE = %+v", info)
	}
}

func TestResolveProfileInvalidManigotProfile(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=bogus\n")
	_, err := ResolveProfile(Options{})
	if err == nil || !strings.Contains(err.Error(), "Error: MANIGOT_PROFILE in") || !strings.Contains(err.Error(), "is not a valid profile (got 'bogus').") {
		t.Errorf("invalid MANIGOT_PROFILE error = %v", err)
	}
}

func TestResolveProfileDefaultsToClaudePro(t *testing.T) {
	checkout(t, "")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	// Resolution picks claude-pro; the auth check then fails on the missing
	// token (a separate step, matching run.sh's ordering).
	if info.Profile != "claude-pro" || info.Tool != config.ToolClaudeCode {
		t.Errorf("default = %+v", info)
	}
	if err := info.CheckAuth(); err == nil || !strings.Contains(err.Error(), "CLAUDE_CODE_OAUTH_TOKEN is not set") {
		t.Errorf("unexpected auth error: %v", err)
	}
}

func TestResolveProfileZAIForwarding(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=z-secret\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	if info.OpenCodeModel != "zai-coding-plan/glm-5.2" {
		t.Errorf("zai default model = %q", info.OpenCodeModel)
	}
	if !contains(info.KeyEnv, "-e", "ZHIPU_API_KEY=z-secret") || !contains(info.KeyEnv, "-e", "OPENCODE_MODEL=zai-coding-plan/glm-5.2") {
		t.Errorf("zai KeyEnv = %v", info.KeyEnv)
	}
}

func TestResolveProfileOpenCodeGoModelOverride(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=opencode-go\nOPENCODE_API_KEY=k\nOPENCODE_GO_MODEL=opencode-go/custom\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.OpenCodeModel != "opencode-go/custom" {
		t.Errorf("opencode-go model override = %q", info.OpenCodeModel)
	}
}

func TestResolveProfileClaudeRequiresToken(t *testing.T) {
	checkout(t, "")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	err = info.CheckAuth()
	if err == nil || !strings.Contains(err.Error(), "Error: CLAUDE_CODE_OAUTH_TOKEN is not set.") {
		t.Errorf("missing token error = %v", err)
	}
	if !strings.Contains(err.Error(), "Add it to ") || !strings.Contains(err.Error(), "or run 'mg setup claude-pro' for help:") {
		t.Errorf("token error missing guidance: %v", err)
	}
}

func TestResolveProfileClaudeRefusesAnthropicKey(t *testing.T) {
	checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-api03-xxx")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	err = info.CheckAuth()
	if err == nil || !strings.Contains(err.Error(), "Error: ANTHROPIC_API_KEY is set — this overrides your subscription and bills per token.") {
		t.Errorf("ANTHROPIC_API_KEY error = %v", err)
	}
}

func TestResolveProfileOpenCodeMissingKey(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	err = info.CheckAuth()
	if err == nil || !strings.Contains(err.Error(), "Error: profile 'zai' is missing its API key.") {
		t.Errorf("missing opencode key error = %v", err)
	}
	if !strings.Contains(err.Error(), "  ZHIPU_API_KEY") {
		t.Errorf("missing key listing: %v", err)
	}
}

func TestResolveProfileLegacyToolOpenCode(t *testing.T) {
	checkout(t, "OPENAI_API_KEY=sk-oa\nOPENCODE_MODEL=my-model\n")
	info, err := ResolveProfile(Options{Tool: "opencode"})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	if info.Profile != "" || info.Tool != config.ToolOpenCode {
		t.Errorf("legacy --tool opencode = %+v", info)
	}
	if len(info.OpenCodeKeys) != 9 {
		t.Errorf("legacy keys = %v", info.OpenCodeKeys)
	}
	if !contains(info.KeyEnv, "-e", "OPENAI_API_KEY=sk-oa") || !contains(info.KeyEnv, "-e", "OPENCODE_MODEL=my-model") {
		t.Errorf("legacy KeyEnv = %v", info.KeyEnv)
	}
}

func TestResolveProfileLegacyToolOpenCodeNoKeys(t *testing.T) {
	checkout(t, "")
	info, err := ResolveProfile(Options{Tool: "opencode"})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	err = info.CheckAuth()
	if err == nil || !strings.Contains(err.Error(), "Error: --tool opencode needs at least one provider API key.") {
		t.Errorf("legacy no-keys error = %v", err)
	}
}

func TestResolveProfilePrintRejectedForLegacy(t *testing.T) {
	checkout(t, "OPENAI_API_KEY=sk-oa\n")
	_, err := ResolveProfile(Options{Tool: "opencode", Print: true})
	if err == nil || !strings.Contains(err.Error(), "Error: --print is not supported with the legacy --tool opencode (no --profile).") {
		t.Errorf("legacy --print rejection = %v", err)
	}
}

func TestResolveProfilePrintAllowedForProfiles(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=k\n")
	if _, err := ResolveProfile(Options{Print: true}); err != nil {
		t.Errorf("--print with zai profile should be allowed: %v", err)
	}
}

func TestResolveProfileEnvFileBeatsProcessEnv(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=from-env-file\n")
	t.Setenv("ZHIPU_API_KEY", "from-process")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	if !contains(info.KeyEnv, "-e", "ZHIPU_API_KEY=from-env-file") {
		t.Errorf(".env value should win over the process env: %v", info.KeyEnv)
	}
}

// contains reports whether want appears as an adjacent pair in args.
func contains(args []string, want ...string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		match := true
		for j := range want {
			if args[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
