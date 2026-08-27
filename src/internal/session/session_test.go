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
		"OPENCODE_ZAI_MODEL", "OPENCODE_GO_MODEL", "OPENCODE_ZEN_MODEL", "OPENCODE_ZEN_FREE_MODEL", "OPENCODE_MODEL", "OPENCODE_THEME", "MANIGOT_PROFILE",
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

func TestParseArgsShortFlags(t *testing.T) {
	o := ParseArgs([]string{"-a", "analyst", "-j", "abc123_x", "--prompt", "hello", "extra"})
	if o.Agent != "analyst" || o.Job != "abc123_x" || o.Prompt != "hello" {
		t.Errorf("ParseArgs = %+v", o)
	}
	if len(o.Pass) != 1 || o.Pass[0] != "extra" {
		t.Errorf("passthrough = %v, want [extra]", o.Pass)
	}
}

func TestParseArgsShortAndLongLastWins(t *testing.T) {
	o := ParseArgs([]string{"--job", "long_id", "-j", "short_id"})
	if o.Job != "short_id" {
		t.Errorf("--job then -j: Job = %q, want short_id (last wins)", o.Job)
	}
	o = ParseArgs([]string{"-a", "first", "--agent", "second"})
	if o.Agent != "second" {
		t.Errorf("-a then --agent: Agent = %q, want second (last wins)", o.Agent)
	}
}

func TestParseArgsShortFlagWithoutValue(t *testing.T) {
	// A known flag left without its value at the end leaves the field unset —
	// the same silent-ignore behavior as a bare "--agent".
	o := ParseArgs([]string{"-a"})
	if o.Agent != "" {
		t.Errorf("-a without a value: Agent = %q, want empty", o.Agent)
	}
	o = ParseArgs([]string{"-j"})
	if o.Job != "" {
		t.Errorf("-j without a value: Job = %q, want empty", o.Job)
	}
}

func TestParseArgsShortFlagsDoNotSwallowPassthrough(t *testing.T) {
	// Unknown flags and bare words still go through verbatim — the -a/-j
	// aliases must not change the passthrough rule.
	o := ParseArgs([]string{"-x", "foo", "-a", "analyst"})
	if o.Agent != "analyst" {
		t.Errorf("Agent = %q, want analyst", o.Agent)
	}
	if len(o.Pass) != 2 || o.Pass[0] != "-x" || o.Pass[1] != "foo" {
		t.Errorf("passthrough = %v, want [-x foo]", o.Pass)
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
	if err == nil || !strings.Contains(err.Error(), "--profile must be one of: claude-pro|zai|opencode-go|opencode-zen|opencode-zen-free (got 'bogus').") {
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
	if err == nil || !strings.Contains(err.Error(), "--tool must be 'claude-code' or 'opencode' (got 'bogus').") {
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
	if err == nil || !strings.Contains(err.Error(), "MANIGOT_PROFILE in") || !strings.Contains(err.Error(), "is not a valid profile (got 'bogus').") {
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
	// Only the profile's own keys are forwarded — the CLAUDE_* subscription
	// keys belong to claude-pro and must not leak into an opencode run's env.
	for _, k := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"} {
		if contains(info.KeyEnv, "-e", k+"=") {
			t.Errorf("zai KeyEnv forwards %s, want only the profile's own keys: %v", k, info.KeyEnv)
		}
	}
}

func TestCheckAuthClaudeProForwardsSubscriptionKeys(t *testing.T) {
	checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	for _, want := range []string{"-e", "CLAUDE_CODE_OAUTH_TOKEN=t", "-e", "CLAUDE_ACCOUNT_UUID=u", "-e", "CLAUDE_EMAIL=e", "-e", "CLAUDE_ORG_UUID=o"} {
		if !contains(info.KeyEnv, want) {
			t.Errorf("claude-pro KeyEnv missing %q: %v", want, info.KeyEnv)
		}
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

func TestResolveProfileOpenCodeZenForwarding(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=opencode-zen\nOPENCODE_API_KEY=zen-secret\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	// Shares the OpenCode key with opencode-go by design; the model defaults
	// to the free Zen model and is overridable via OPENCODE_ZEN_MODEL.
	if len(info.OpenCodeKeys) != 1 || info.OpenCodeKeys[0] != "OPENCODE_API_KEY" {
		t.Errorf("zen OpenCodeKeys = %v, want [OPENCODE_API_KEY]", info.OpenCodeKeys)
	}
	if info.OpenCodeModel != "opencode/deepseek-v4-flash" {
		t.Errorf("zen default model = %q", info.OpenCodeModel)
	}
	if !contains(info.KeyEnv, "-e", "OPENCODE_API_KEY=zen-secret") || !contains(info.KeyEnv, "-e", "OPENCODE_MODEL=opencode/deepseek-v4-flash") {
		t.Errorf("zen KeyEnv = %v", info.KeyEnv)
	}
	// Only the profile's own key is forwarded — no CLAUDE_* subscription keys.
	for _, k := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"} {
		if contains(info.KeyEnv, "-e", k+"=") {
			t.Errorf("zen KeyEnv forwards %s, want only the profile's own keys: %v", k, info.KeyEnv)
		}
	}
}

func TestResolveProfileOpenCodeZenModelOverride(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=opencode-zen\nOPENCODE_API_KEY=k\nOPENCODE_ZEN_MODEL=opencode-zen/custom\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.OpenCodeModel != "opencode-zen/custom" {
		t.Errorf("opencode-zen model override = %q", info.OpenCodeModel)
	}
}

func TestResolveProfileOpenCodeZenMissingKey(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=opencode-zen\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	err = info.CheckAuth()
	if err == nil || !strings.Contains(err.Error(), "profile 'opencode-zen' is missing its API key.") {
		t.Errorf("missing opencode-zen key error = %v", err)
	}
	if !strings.Contains(err.Error(), "  OPENCODE_API_KEY") {
		t.Errorf("missing key listing: %v", err)
	}
}

func TestResolveProfileOpenCodeZenFreeForwarding(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=opencode-zen-free\nOPENCODE_API_KEY=zen-secret\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	// Shares the OpenCode key with opencode-go/opencode-zen by design; the
	// model defaults to the free Zen model and is overridable via
	// OPENCODE_ZEN_FREE_MODEL.
	if len(info.OpenCodeKeys) != 1 || info.OpenCodeKeys[0] != "OPENCODE_API_KEY" {
		t.Errorf("zen-free OpenCodeKeys = %v, want [OPENCODE_API_KEY]", info.OpenCodeKeys)
	}
	if info.OpenCodeModel != "opencode/deepseek-v4-flash-free" {
		t.Errorf("zen-free default model = %q", info.OpenCodeModel)
	}
	if !contains(info.KeyEnv, "-e", "OPENCODE_API_KEY=zen-secret") || !contains(info.KeyEnv, "-e", "OPENCODE_MODEL=opencode/deepseek-v4-flash-free") {
		t.Errorf("zen-free KeyEnv = %v", info.KeyEnv)
	}
	// Only the profile's own key is forwarded — no CLAUDE_* subscription keys.
	for _, k := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"} {
		if contains(info.KeyEnv, "-e", k+"=") {
			t.Errorf("zen-free KeyEnv forwards %s, want only the profile's own keys: %v", k, info.KeyEnv)
		}
	}
}

func TestResolveProfileOpenCodeZenFreeModelOverride(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=opencode-zen-free\nOPENCODE_API_KEY=k\nOPENCODE_ZEN_FREE_MODEL=opencode/custom-free\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.OpenCodeModel != "opencode/custom-free" {
		t.Errorf("opencode-zen-free model override = %q", info.OpenCodeModel)
	}
}

func TestResolveProfileOpenCodeZenFreeMissingKey(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=opencode-zen-free\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	err = info.CheckAuth()
	if err == nil || !strings.Contains(err.Error(), "profile 'opencode-zen-free' is missing its API key.") {
		t.Errorf("missing opencode-zen-free key error = %v", err)
	}
	if !strings.Contains(err.Error(), "  OPENCODE_API_KEY") {
		t.Errorf("missing key listing: %v", err)
	}
}

// TestResolveProfileThemeForwardedIndependentOfProfile — the global theme
// setting (OPENCODE_THEME) is forwarded regardless of which opencode profile
// (or which of its API keys) is in use, unlike the per-profile model.
func TestResolveProfileThemeForwardedIndependentOfProfile(t *testing.T) {
	for _, profile := range []string{"zai", "opencode-go", "opencode-zen", "opencode-zen-free"} {
		t.Run(profile, func(t *testing.T) {
			checkout(t, "MANIGOT_PROFILE="+profile+"\nZHIPU_API_KEY=z\nOPENCODE_API_KEY=k\nOPENCODE_THEME=nord\n")
			info, err := ResolveProfile(Options{})
			if err != nil {
				t.Fatalf("ResolveProfile: %v", err)
			}
			if info.OpenCodeTheme != "nord" {
				t.Errorf("OpenCodeTheme = %q, want nord", info.OpenCodeTheme)
			}
			if err := info.CheckAuth(); err != nil {
				t.Fatalf("CheckAuth: %v", err)
			}
			if !contains(info.KeyEnv, "-e", "OPENCODE_THEME=nord") {
				t.Errorf("%s KeyEnv missing OPENCODE_THEME=nord: %v", profile, info.KeyEnv)
			}
		})
	}
}

// TestResolveProfileThemeUnsetOmitsKeyEnv — no OPENCODE_THEME in .env means
// no -e OPENCODE_THEME= at all, letting OpenCode fall back to its own
// default/config.
func TestResolveProfileThemeUnsetOmitsKeyEnv(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=z\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.OpenCodeTheme != "" {
		t.Errorf("OpenCodeTheme = %q, want empty", info.OpenCodeTheme)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	for _, arg := range info.KeyEnv {
		if strings.HasPrefix(arg, "OPENCODE_THEME=") {
			t.Errorf("KeyEnv should not contain OPENCODE_THEME when unset: %v", info.KeyEnv)
		}
	}
}

// TestResolveProfileThemeNotForwardedForClaudePro — claude-pro is not an
// opencode run, so OpenCodeTheme stays empty and no theme env var is
// forwarded, even if OPENCODE_THEME happens to be set.
func TestResolveProfileThemeNotForwardedForClaudePro(t *testing.T) {
	checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nOPENCODE_THEME=nord\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.OpenCodeTheme != "" {
		t.Errorf("claude-pro OpenCodeTheme = %q, want empty", info.OpenCodeTheme)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	for _, arg := range info.KeyEnv {
		if strings.HasPrefix(arg, "OPENCODE_THEME=") {
			t.Errorf("claude-pro KeyEnv should never contain OPENCODE_THEME: %v", info.KeyEnv)
		}
	}
}

// TestResolveProfileThemeForwardedForLegacyOpenCode — the legacy,
// profile-less --tool opencode path also gets the global theme, since it's
// independent of profile/API key.
func TestResolveProfileThemeForwardedForLegacyOpenCode(t *testing.T) {
	checkout(t, "OPENAI_API_KEY=sk-oa\nOPENCODE_THEME=gruvbox\n")
	info, err := ResolveProfile(Options{Tool: "opencode"})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	if !contains(info.KeyEnv, "-e", "OPENCODE_THEME=gruvbox") {
		t.Errorf("legacy KeyEnv missing OPENCODE_THEME=gruvbox: %v", info.KeyEnv)
	}
}

func TestResolveProfileClaudeRequiresToken(t *testing.T) {
	checkout(t, "")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	err = info.CheckAuth()
	if err == nil || !strings.Contains(err.Error(), "CLAUDE_CODE_OAUTH_TOKEN is not set.") {
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
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY is set — this overrides your subscription and bills per token.") {
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
	if err == nil || !strings.Contains(err.Error(), "profile 'zai' is missing its API key.") {
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
	if err == nil || !strings.Contains(err.Error(), "--tool opencode needs at least one provider API key.") {
		t.Errorf("legacy no-keys error = %v", err)
	}
}

func TestResolveProfilePrintRejectedForLegacy(t *testing.T) {
	checkout(t, "OPENAI_API_KEY=sk-oa\n")
	_, err := ResolveProfile(Options{Tool: "opencode", Print: true})
	if err == nil || !strings.Contains(err.Error(), "--print is not supported with the legacy --tool opencode (no --profile).") {
		t.Errorf("legacy --print rejection = %v", err)
	}
}

func TestResolveProfilePrintAllowedForProfiles(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=k\n")
	if _, err := ResolveProfile(Options{Print: true}); err != nil {
		t.Errorf("--print with zai profile should be allowed: %v", err)
	}
}

// addUserProfile adds a user-defined opencode profile to the checkout's store.
func addUserProfile(t *testing.T) {
	t.Helper()
	if err := config.AddProfile(config.Profile{
		ID: "custom", Label: "Custom", Tool: config.ToolOpenCode,
		AuthKeys: []string{"OPENCODE_API_KEY"},
		ModelEnv: "OPENCODE_CUSTOM_MODEL", ModelDefault: "opencode/default-model",
	}); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
}

// TestResolveUserDefinedProfileEndToEnd pins TASK-3/TASK-9: a user-defined
// opencode profile resolves through ResolveProfile → CheckAuth exactly like a
// built-in — tool, auth keys, and model env/default all from the store.
func TestResolveUserDefinedProfileEndToEnd(t *testing.T) {
	checkout(t, "OPENCODE_API_KEY=user-secret\nOPENCODE_CUSTOM_MODEL=opencode/custom\n")
	addUserProfile(t)
	info, err := ResolveProfile(Options{Profile: "custom"})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.Tool != config.ToolOpenCode {
		t.Errorf("tool = %q, want opencode", info.Tool)
	}
	if len(info.OpenCodeKeys) != 1 || info.OpenCodeKeys[0] != "OPENCODE_API_KEY" {
		t.Errorf("OpenCodeKeys = %v, want [OPENCODE_API_KEY]", info.OpenCodeKeys)
	}
	if info.OpenCodeModel != "opencode/custom" {
		t.Errorf("OpenCodeModel = %q, want opencode/custom (from OPENCODE_CUSTOM_MODEL)", info.OpenCodeModel)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	if !contains(info.KeyEnv, "-e", "OPENCODE_API_KEY=user-secret") || !contains(info.KeyEnv, "-e", "OPENCODE_MODEL=opencode/custom") {
		t.Errorf("KeyEnv = %v", info.KeyEnv)
	}
	for _, k := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"} {
		if contains(info.KeyEnv, "-e", k+"=") {
			t.Errorf("user opencode profile forwards %s: %v", k, info.KeyEnv)
		}
	}
}

// TestResolveUserDefinedProfileModelFallback: with no model-env override set,
// a user-defined opencode profile falls back to its stored ModelDefault.
func TestResolveUserDefinedProfileModelFallback(t *testing.T) {
	checkout(t, "OPENCODE_API_KEY=k\n")
	addUserProfile(t)
	info, err := ResolveProfile(Options{Profile: "custom"})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.OpenCodeModel != "opencode/default-model" {
		t.Errorf("OpenCodeModel = %q, want the profile's ModelDefault", info.OpenCodeModel)
	}
}

// TestResolveUserDefinedProfileAsDefault: a user-defined profile set as the
// MANIGOT_PROFILE default resolves like any other default.
func TestResolveUserDefinedProfileAsDefault(t *testing.T) {
	checkout(t, "MANIGOT_PROFILE=custom\nOPENCODE_API_KEY=k\n")
	addUserProfile(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.Profile != "custom" || info.Tool != config.ToolOpenCode {
		t.Errorf("user profile via MANIGOT_PROFILE = %+v", info)
	}
}

// TestProfileValidListIncludesUserProfile: the "--profile must be one of"
// error message grows to include user-defined profile ids.
func TestProfileValidListIncludesUserProfile(t *testing.T) {
	checkout(t, "")
	addUserProfile(t)
	_, err := ResolveProfile(Options{Profile: "bogus"})
	if err == nil || !strings.Contains(err.Error(), "claude-pro|zai|opencode-go|opencode-zen|opencode-zen-free|custom") {
		t.Errorf("valid list should include the user profile: %v", err)
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
