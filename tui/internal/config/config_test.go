package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkout builds a minimal fake manigot checkout (just the one file
// resolve.looksLikeCheckout keys off) and points $MANIGOT_HOME at it, so
// Dir/Load/Save are exercised without depending on where `go test` happens to
// run from.
func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", dir)
	return dir
}

func TestLoadMissingFileReturnsZeroValue(t *testing.T) {
	checkout(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s != (Settings{}) {
		t.Errorf("Load on missing file = %+v, want zero value", s)
	}
}

func TestLoadUnresolvableHomeReturnsZeroValue(t *testing.T) {
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("PATH", t.TempDir()) // hide any real manigot checkout on $PATH
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s != (Settings{}) {
		t.Errorf("Load with no resolvable home = %+v, want zero value", s)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := checkout(t)
	want := Settings{Editor: "vim", Profile: ProfileZAI}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(dir, "config", "tui-settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file not written at %s: %v", path, err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("Load after Save = %+v, want %+v", got, want)
	}
}

func TestSaveUnresolvableHomeErrors(t *testing.T) {
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("PATH", t.TempDir())
	if err := Save(Settings{Editor: "vim"}); err == nil {
		t.Fatal("expected an error when no checkout can be located")
	}
}

func TestProfileValueDefaultsToClaudePro(t *testing.T) {
	if got := (Settings{}).ProfileValue(); got != ProfileClaudePro {
		t.Errorf("zero-value ProfileValue = %q, want %q", got, ProfileClaudePro)
	}
	if got := (Settings{Profile: ProfileOpenCodeGo}).ProfileValue(); got != ProfileOpenCodeGo {
		t.Errorf("ProfileValue = %q, want %q", got, ProfileOpenCodeGo)
	}
}

// writeSettings writes a tui-settings.json into the fake checkout.
func writeSettings(t *testing.T, data string) {
	t.Helper()
	dir := checkout(t)
	path := filepath.Join(dir, "config", "tui-settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeEnv writes a .env into the fake checkout.
func writeEnv(t *testing.T, content string) {
	t.Helper()
	dir := checkout(t)
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// contains reports whether haystack contains needle.
func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func TestLoadPrefersEnvProfileOverLegacySettings(t *testing.T) {
	// The shared store (MANIGOT_PROFILE in .env) wins over a profile the old
	// version saved into tui-settings.json.
	dir := checkout(t)
	settingsPath := filepath.Join(dir, "config", "tui-settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"editor":"vim","profile":"zai"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MANIGOT_PROFILE=opencode-go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileOpenCodeGo {
		t.Errorf("profile = %q, want %q (env wins over legacy tui-settings profile)", s.Profile, ProfileOpenCodeGo)
	}
	if s.Editor != "vim" {
		t.Errorf("editor = %q, want vim (still read from tui-settings.json)", s.Editor)
	}
}

func TestLoadEnvProfileWinsOverToolMigration(t *testing.T) {
	dir := checkout(t)
	settingsPath := filepath.Join(dir, "config", "tui-settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"tool":"opencode"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MANIGOT_PROFILE=claude-pro\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileClaudePro {
		t.Errorf("profile = %q, want %q", s.Profile, ProfileClaudePro)
	}
}

func TestLoadIgnoresInvalidEnvProfile(t *testing.T) {
	// A typo'd MANIGOT_PROFILE must not leak into the TUI — fall through to
	// the legacy tui-settings profile, then to the default.
	dir := checkout(t)
	settingsPath := filepath.Join(dir, "config", "tui-settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"profile":"zai"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MANIGOT_PROFILE=bogus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileZAI {
		t.Errorf("profile = %q, want %q (invalid env value ignored, legacy honored)", s.Profile, ProfileZAI)
	}

	// And with no legacy value at all, an invalid env value yields the default.
	writeEnv(t, "MANIGOT_PROFILE=bogus\n")
	s, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s.ProfileValue(); got != ProfileClaudePro {
		t.Errorf("ProfileValue = %q, want %q", got, ProfileClaudePro)
	}
}

func TestLoadHonorsEnvWhenSettingsFileMissing(t *testing.T) {
	writeEnv(t, "MANIGOT_PROFILE=zai\n")
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileZAI {
		t.Errorf("profile = %q, want %q", s.Profile, ProfileZAI)
	}
}

func TestLoadMigratesLegacyProfileFieldWithoutEnv(t *testing.T) {
	// No .env yet (fresh upgrade): the legacy profile field in
	// tui-settings.json is honored so the user's choice survives.
	writeSettings(t, `{"editor":"vim","profile":"opencode-go"}`)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileOpenCodeGo {
		t.Errorf("profile = %q, want %q", s.Profile, ProfileOpenCodeGo)
	}
}

func TestSaveWritesProfileToEnvNotSettings(t *testing.T) {
	dir := checkout(t)
	if err := Save(Settings{Editor: "vim", Profile: ProfileOpenCodeGo}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	settingsData, err := os.ReadFile(filepath.Join(dir, "config", "tui-settings.json"))
	if err != nil {
		t.Fatalf("read tui-settings.json: %v", err)
	}
	if got := string(settingsData); contains(got, `"profile"`) {
		t.Errorf("tui-settings.json still contains a profile field:\n%s", got)
	}

	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	env := string(envData)
	if !contains(env, "MANIGOT_PROFILE=opencode-go") {
		t.Errorf(".env missing MANIGOT_PROFILE=opencode-go:\n%s", env)
	}
}

func TestSaveCreatesEnvWithHeader(t *testing.T) {
	dir := checkout(t)
	if err := Save(Settings{Editor: "vim", Profile: ProfileZAI}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		t.Errorf("first line of fresh .env should be the header comment, got %q", lines[0])
	}
	if lines[len(lines)-1] != "MANIGOT_PROFILE=zai" {
		t.Errorf("last line = %q, want MANIGOT_PROFILE=zai", lines[len(lines)-1])
	}
}

func TestSavePreservesOtherEnvLines(t *testing.T) {
	dir := checkout(t)
	original := "# manigot configuration — credentials and defaults (never commit this file)\n" +
		"CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret\n" +
		"ZHIPU_API_KEY=zhipu-secret\n" +
		"OPENCODE_ZAI_MODEL=zai-coding-plan/glm-5.2\n" +
		"# a comment the user wrote\n" +
		"OPENCODE_GO_MODEL=opencode-go/glm-5.2"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(Settings{Editor: "vim", Profile: ProfileZAI}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	env := string(got)
	for _, want := range []string{
		"CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret",
		"ZHIPU_API_KEY=zhipu-secret",
		"OPENCODE_ZAI_MODEL=zai-coding-plan/glm-5.2",
		"# a comment the user wrote",
		"OPENCODE_GO_MODEL=opencode-go/glm-5.2",
		"MANIGOT_PROFILE=zai",
	} {
		if !contains(env, want) {
			t.Errorf(".env missing %q:\n%s", want, env)
		}
	}
	if strings.Count(env, "MANIGOT_PROFILE=") != 1 {
		t.Errorf("MANIGOT_PROFILE should appear exactly once:\n%s", env)
	}
}

func TestSaveReplacesExistingEnvProfile(t *testing.T) {
	dir := checkout(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("# header\nMANIGOT_PROFILE=zai\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(Settings{Editor: "vim", Profile: ProfileClaudePro}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if got := string(env); !contains(got, "MANIGOT_PROFILE=claude-pro") || contains(got, "MANIGOT_PROFILE=zai") {
		t.Errorf("existing MANIGOT_PROFILE not replaced in place:\n%s", got)
	}
}

func TestReadEnvProfileToleratesExportAndQuotes(t *testing.T) {
	dir := checkout(t)
	cases := []struct{ line, want string }{
		{"MANIGOT_PROFILE=zai", "zai"},
		{"export MANIGOT_PROFILE=opencode-go", "opencode-go"},
		{`MANIGOT_PROFILE="claude-pro"`, "claude-pro"},
		{"  MANIGOT_PROFILE=zai  ", "zai"},
	}
	for _, tc := range cases {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(tc.line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readEnvProfile(); got != tc.want {
			t.Errorf("readEnvProfile(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestSaveRejectsInvalidProfile(t *testing.T) {
	checkout(t)
	if err := Save(Settings{Editor: "vim", Profile: "bogus"}); err == nil {
		t.Fatal("expected an error saving an unknown profile id")
	}
}

func TestLoadMigratesLegacyToolField(t *testing.T) {
	// claude-code maps to claude-pro.
	writeSettings(t, `{"editor":"vim","tool":"claude-code"}`)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileClaudePro {
		t.Errorf("legacy tool claude-code -> profile %q, want %q", s.Profile, ProfileClaudePro)
	}

	// opencode maps to zai (the opencode subscription manigot configured first).
	writeSettings(t, `{"tool":"opencode"}`)
	s, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileZAI {
		t.Errorf("legacy tool opencode -> profile %q, want %q", s.Profile, ProfileZAI)
	}
}

func TestLoadKeepsExplicitProfile(t *testing.T) {
	// A settings file that already has a profile wins over any legacy tool
	// field, and the legacy field is dropped.
	writeSettings(t, `{"profile":"opencode-go","tool":"opencode"}`)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != ProfileOpenCodeGo {
		t.Errorf("profile = %q, want %q", s.Profile, ProfileOpenCodeGo)
	}
	if s.Tool != "" {
		t.Errorf("legacy tool field not cleared, got %q", s.Tool)
	}
}
