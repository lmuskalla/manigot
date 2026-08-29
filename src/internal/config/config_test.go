package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkout builds a minimal fake manigot checkout (just the one file
// home.looksLikeCheckout keys off) and points $MANIGOT_HOME at it, so
// Dir/Load/Save are exercised without depending on where `go test` happens to
// run from.
func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
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

// TestBuiltInProfilesCarryAuthAndModelMetadata pins TASK-1's data model: the
// built-in table is the single source of each profile's auth keys and model
// env/default (moved here from cmd/mg/profiles.go and internal/session).
// claude-pro's ModelDefault is empty — it defines no model override, and the
// "(Claude Code default)" sentinel now lives only in cmd/mg/profiles.go's
// display-only profileModel(), never in this data field (see TASK-1/TASK-5).
func TestBuiltInProfilesCarryAuthAndModelMetadata(t *testing.T) {
	if got := len(Profiles()); got != 5 {
		t.Fatalf("built-in profile count = %d, want 5", got)
	}
	for _, tc := range []struct {
		id           string
		wantAuthKeys []string
		wantModelEnv string
		wantModelDef string
	}{
		{ProfileClaudePro, []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"}, "", ""},
		{ProfileZAI, []string{"ZHIPU_API_KEY"}, "OPENCODE_ZAI_MODEL", "zai-coding-plan/glm-5.2"},
		{ProfileOpenCodeGo, []string{"OPENCODE_API_KEY"}, "OPENCODE_GO_MODEL", "opencode-go/glm-5.2"},
		{ProfileOpenCodeZen, []string{"OPENCODE_API_KEY"}, "OPENCODE_ZEN_MODEL", "opencode/deepseek-v4-flash"},
		{ProfileOpenCodeZenFree, []string{"OPENCODE_API_KEY"}, "OPENCODE_ZEN_FREE_MODEL", "opencode/deepseek-v4-flash-free"},
	} {
		p, ok := ProfileByID(tc.id)
		if !ok {
			t.Fatalf("ProfileByID(%q) not found", tc.id)
		}
		if len(p.AuthKeys) != len(tc.wantAuthKeys) {
			t.Errorf("%s AuthKeys = %v, want %v", tc.id, p.AuthKeys, tc.wantAuthKeys)
		}
		for i, k := range tc.wantAuthKeys {
			if p.AuthKeys[i] != k {
				t.Errorf("%s AuthKeys[%d] = %q, want %q", tc.id, i, p.AuthKeys[i], k)
			}
		}
		if p.ModelEnv != tc.wantModelEnv {
			t.Errorf("%s ModelEnv = %q, want %q", tc.id, p.ModelEnv, tc.wantModelEnv)
		}
		if p.ModelDefault != tc.wantModelDef {
			t.Errorf("%s ModelDefault = %q, want %q", tc.id, p.ModelDefault, tc.wantModelDef)
		}
	}
}

// userProfile is a representative user-defined opencode profile used by the
// store tests.
func userProfile() Profile {
	return Profile{
		ID:           "zen-alt",
		Label:        "OpenCode · Zen (alternate)",
		Tool:         ToolOpenCode,
		Auth:         "OPENCODE_API_KEY",
		AuthKeys:     []string{"OPENCODE_API_KEY"},
		ModelEnv:     "OPENCODE_ZEN_MODEL",
		ModelDefault: "opencode/deepseek-v4-flash-free",
	}
}

func TestProfilesMergesBuiltInsThenUserProfiles(t *testing.T) {
	checkout(t)
	if err := AddProfile(userProfile()); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	second := userProfile()
	second.ID = "go-alt"
	if err := AddProfile(second); err != nil {
		t.Fatalf("AddProfile second: %v", err)
	}

	ps := Profiles()
	if len(ps) != 7 {
		t.Fatalf("Profiles() = %d profiles, want 7 (5 built-in + 2 user)", len(ps))
	}
	// Built-ins keep their canonical order first.
	wantIDs := []string{ProfileClaudePro, ProfileZAI, ProfileOpenCodeGo, ProfileOpenCodeZen, ProfileOpenCodeZenFree, "zen-alt", "go-alt"}
	for i, want := range wantIDs {
		if ps[i].ID != want {
			t.Errorf("Profiles()[%d].ID = %q, want %q", i, ps[i].ID, want)
		}
	}
}

func TestProfileByIDFindsUserProfile(t *testing.T) {
	checkout(t)
	if err := AddProfile(userProfile()); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	p, ok := ProfileByID("zen-alt")
	if !ok {
		t.Fatal("ProfileByID(zen-alt) not found after AddProfile")
	}
	if p.ModelDefault != "opencode/deepseek-v4-flash-free" || p.Tool != ToolOpenCode {
		t.Errorf("ProfileByID(zen-alt) = %+v", p)
	}
}

func TestAddProfileRejectsBuiltInID(t *testing.T) {
	checkout(t)
	if err := AddProfile(Profile{ID: ProfileClaudePro, Tool: ToolOpenCode}); err == nil {
		t.Fatal("expected an error adding a profile whose id collides with a built-in")
	}
}

func TestAddProfileRejectsDuplicateUserID(t *testing.T) {
	checkout(t)
	if err := AddProfile(userProfile()); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := AddProfile(userProfile()); err == nil {
		t.Fatal("expected an error adding a duplicate user profile id")
	}
}

func TestAddProfileRejectsEmptyID(t *testing.T) {
	checkout(t)
	if err := AddProfile(Profile{}); err == nil {
		t.Fatal("expected an error adding a profile with no id")
	}
}

func TestRemoveProfileRemovesUserProfile(t *testing.T) {
	checkout(t)
	if err := AddProfile(userProfile()); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := RemoveProfile("zen-alt"); err != nil {
		t.Fatalf("RemoveProfile: %v", err)
	}
	if _, ok := ProfileByID("zen-alt"); ok {
		t.Error("ProfileByID(zen-alt) still found after RemoveProfile")
	}
	// The built-ins are untouched.
	if len(Profiles()) != 5 {
		t.Errorf("after removing the only user profile, Profiles() = %d, want 5", len(Profiles()))
	}
}

func TestRemoveProfileUnknownIsError(t *testing.T) {
	checkout(t)
	if err := RemoveProfile("nope"); err == nil {
		t.Fatal("expected an error removing a non-existent user profile")
	}
}

func TestRemoveProfileDoesNotTouchBuiltIns(t *testing.T) {
	checkout(t)
	// Built-ins are not in the user store — removing a built-in id must fail
	// (built-ins are not deletable), and the profile must remain.
	if err := RemoveProfile(ProfileZAI); err == nil {
		t.Fatal("expected an error removing a built-in profile id")
	}
	if _, ok := ProfileByID(ProfileZAI); !ok {
		t.Error("built-in zai disappeared after a failed RemoveProfile")
	}
}

func TestAddProfilePersistsAcrossCalls(t *testing.T) {
	checkout(t)
	if err := AddProfile(userProfile()); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	// A fresh read of Profiles/ProfileByID must see the persisted store (no
	// in-memory caching that a later call could miss).
	ps := Profiles()
	found := false
	for _, p := range ps {
		if p.ID == "zen-alt" {
			found = true
		}
	}
	if !found {
		t.Error("user profile not present after re-reading Profiles()")
	}
}

func TestCorruptUserStoreDegradesToBuiltInsOnly(t *testing.T) {
	dir := checkout(t)
	path := filepath.Join(dir, "config", "profiles.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ps := Profiles(); len(ps) != 5 {
		t.Errorf("Profiles() with a corrupt store = %d, want 5 (built-ins only)", len(ps))
	}
	if _, ok := ProfileByID("anything"); ok {
		t.Error("ProfileByID should not find anything in a corrupt store")
	}
	// The write paths must surface the corrupt store rather than clobber it.
	if err := AddProfile(userProfile()); err == nil {
		t.Error("AddProfile should error on a corrupt store, not silently overwrite it")
	}
}

func TestProfilesReturnsFreshSlice(t *testing.T) {
	checkout(t)
	ps := Profiles()
	ps[0].Label = "mutated"
	again := Profiles()
	if again[0].Label == "mutated" {
		t.Error("mutating the returned slice leaked into the shared table")
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

func TestRecentActivityCountValueDefaultsToFive(t *testing.T) {
	if got := (Settings{}).RecentActivityCountValue(); got != DefaultRecentActivityCount {
		t.Errorf("zero-value RecentActivityCountValue = %d, want %d", got, DefaultRecentActivityCount)
	}
	if got := (Settings{RecentActivityCount: 10}).RecentActivityCountValue(); got != 10 {
		t.Errorf("RecentActivityCountValue = %d, want 10", got)
	}
	// A negative value (never produced by the form, but not worth trusting
	// from a hand-edited file) is treated as unset too.
	if got := (Settings{RecentActivityCount: -3}).RecentActivityCountValue(); got != DefaultRecentActivityCount {
		t.Errorf("negative RecentActivityCountValue = %d, want %d", got, DefaultRecentActivityCount)
	}
}

func TestSaveThenLoadRoundTripsRecentActivityCount(t *testing.T) {
	dir := checkout(t)
	want := Settings{Editor: "vim", Profile: ProfileZAI, RecentActivityCount: 12}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	settingsData, err := os.ReadFile(filepath.Join(dir, "config", "tui-settings.json"))
	if err != nil {
		t.Fatalf("read tui-settings.json: %v", err)
	}
	if got := string(settingsData); !contains(got, `"recentActivityCount": 12`) {
		t.Errorf("tui-settings.json missing recentActivityCount: %s", got)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RecentActivityCount != want.RecentActivityCount {
		t.Errorf("RecentActivityCount after round-trip = %d, want %d", got.RecentActivityCount, want.RecentActivityCount)
	}
}

func TestSaveThenLoadRoundTripsTerminal(t *testing.T) {
	dir := checkout(t)
	want := Settings{Editor: "vim", Profile: ProfileZAI, Terminal: "kitty"}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	settingsData, err := os.ReadFile(filepath.Join(dir, "config", "tui-settings.json"))
	if err != nil {
		t.Fatalf("read tui-settings.json: %v", err)
	}
	if got := string(settingsData); !contains(got, `"terminal": "kitty"`) {
		t.Errorf("tui-settings.json missing terminal: %s", got)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Terminal != want.Terminal {
		t.Errorf("Terminal after round-trip = %q, want %q", got.Terminal, want.Terminal)
	}
}

func TestThemeValueDefaultsToEmpty(t *testing.T) {
	if got := (Settings{}).ThemeValue(); got != "" {
		t.Errorf("zero-value ThemeValue = %q, want empty", got)
	}
	if got := (Settings{Theme: "nord"}).ThemeValue(); got != "nord" {
		t.Errorf("ThemeValue = %q, want %q", got, "nord")
	}
}

func TestSaveThenLoadRoundTripsTheme(t *testing.T) {
	dir := checkout(t)
	want := Settings{Editor: "vim", Profile: ProfileZAI, Theme: "nord"}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if got := string(envData); !contains(got, "OPENCODE_THEME=nord") {
		t.Errorf(".env missing OPENCODE_THEME=nord:\n%s", got)
	}
	// Not written to tui-settings.json — .env is the single shared store.
	settingsData, err := os.ReadFile(filepath.Join(dir, "config", "tui-settings.json"))
	if err != nil {
		t.Fatalf("read tui-settings.json: %v", err)
	}
	if contains(string(settingsData), "theme") {
		t.Errorf("tui-settings.json should not contain a theme field:\n%s", settingsData)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != want.Theme {
		t.Errorf("Theme after round-trip = %q, want %q", got.Theme, want.Theme)
	}
}

func TestThemeDefaultsToEmptyAutoDetect(t *testing.T) {
	checkout(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Theme != "" {
		t.Errorf("zero-value Theme = %q, want empty (let OpenCode use its own default)", s.Theme)
	}
}

func TestTerminalDefaultsToEmptyAutoDetect(t *testing.T) {
	checkout(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Terminal != "" {
		t.Errorf("zero-value Terminal = %q, want empty (auto-detect)", s.Terminal)
	}
}

func TestLoadUnsetRecentActivityCountDefaults(t *testing.T) {
	// A settings file without the field (an older version's file) loads as 0
	// → the default, never an error.
	writeSettings(t, `{"editor":"vim"}`)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s.RecentActivityCountValue(); got != DefaultRecentActivityCount {
		t.Errorf("RecentActivityCountValue = %d, want default %d", got, DefaultRecentActivityCount)
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
	// MANIGOT_PROFILE and OPENCODE_THEME (empty — never set by this test) are
	// both written by Save; order between them isn't guaranteed contract, so
	// check presence rather than an exact last line.
	if !contains(string(data), "MANIGOT_PROFILE=zai") {
		t.Errorf(".env missing MANIGOT_PROFILE=zai:\n%s", data)
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

func TestGetEnvReadsAnyKey(t *testing.T) {
	dir := checkout(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("# header\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret\nexport ZHIPU_API_KEY=zhipu-secret\nOPENCODE_GO_MODEL=\"opencode-go/glm-5.2\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ key, want string }{
		{"CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-secret"},
		{"ZHIPU_API_KEY", "zhipu-secret"},
		{"OPENCODE_GO_MODEL", "opencode-go/glm-5.2"},
		{"MANIGOT_PROFILE", ""}, // absent
		{"NOPE", ""},
	}
	for _, tc := range cases {
		if got := GetEnv(tc.key); got != tc.want {
			t.Errorf("GetEnv(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestGetEnvMissingFileReturnsEmpty(t *testing.T) {
	checkout(t)
	if got := GetEnv("ZHIPU_API_KEY"); got != "" {
		t.Errorf("GetEnv on missing .env = %q, want empty", got)
	}
}

func TestEnvValueFallsBackToProcessEnv(t *testing.T) {
	checkout(t)
	t.Setenv("ZHIPU_API_KEY", "inherited-secret")
	if got := EnvValue("ZHIPU_API_KEY"); got != "inherited-secret" {
		t.Errorf("EnvValue without .env = %q, want the inherited value", got)
	}
}

func TestEnvValueEnvFileWinsOverProcessEnv(t *testing.T) {
	// Once .env sets the key it wins (sourcing overwrites), and an explicit
	// empty value still counts as set (overwriting the inherited one).
	t.Setenv("ZHIPU_API_KEY", "inherited-secret")

	dir := checkout(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("ZHIPU_API_KEY=from-env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := EnvValue("ZHIPU_API_KEY"); got != "from-env" {
		t.Errorf("EnvValue with .env value = %q, want %q", got, "from-env")
	}

	dir = checkout(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("ZHIPU_API_KEY=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := EnvValue("ZHIPU_API_KEY"); got != "" {
		t.Errorf("EnvValue with explicit empty .env value = %q, want empty (overwrites inherited)", got)
	}
}

func TestUpsertEnvCreatesFile(t *testing.T) {
	dir := checkout(t)
	if err := UpsertEnv("ZHIPU_API_KEY", "zhipu-secret"); err != nil {
		t.Fatalf("UpsertEnv: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	got := string(env)
	if !strings.Contains(got, "ZHIPU_API_KEY=zhipu-secret") {
		t.Errorf(".env missing ZHIPU_API_KEY=zhipu-secret:\n%s", got)
	}
	if !strings.HasPrefix(got, "# manigot configuration") {
		t.Errorf(".env missing the standard header:\n%s", got)
	}
}

func TestUpsertEnvPreservesOtherLinesAndReplaces(t *testing.T) {
	dir := checkout(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("# header\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret\nOPENCODE_ZAI_MODEL=zai-coding-plan/glm-5.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpsertEnv("CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-new"); err != nil {
		t.Fatalf("UpsertEnv: %v", err)
	}
	if err := UpsertEnv("ZHIPU_API_KEY", "zhipu-secret"); err != nil {
		t.Fatalf("UpsertEnv append: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	got := string(env)
	for _, want := range []string{
		"# header",
		"CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-new",
		"OPENCODE_ZAI_MODEL=zai-coding-plan/glm-5.2",
		"ZHIPU_API_KEY=zhipu-secret",
	} {
		if !strings.Contains(got, want) {
			t.Errorf(".env missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "CLAUDE_CODE_OAUTH_TOKEN=") != 1 {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN should appear exactly once:\n%s", got)
	}
}

func TestUpsertEnvUnresolvableHomeErrors(t *testing.T) {
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("PATH", t.TempDir())
	if err := UpsertEnv("ZHIPU_API_KEY", "x"); err == nil {
		t.Fatal("expected an error when no checkout can be located")
	}
}
