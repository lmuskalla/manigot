// Package config persists local TUI preferences across sessions: which
// editor to open brief.md in, how many entries the dashboard's recent-activity
// strip may show, and which subscription profile (a bundle of agent CLI +
// credentials + model) launch.Agent starts a session under.
//
// The editor lives in config/tui-settings.json in the manigot checkout. The
// profile lives in manigot/.env as MANIGOT_PROFILE — the value bare `mg` runs
// and `mg profiles` write, so CLI and TUI share one profile default. The settings screen (see ui/settings.go)
// is the only writer; every other reader treats a missing or unreadable file
// as "use the defaults" rather than an error.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/home"
)

// ToolClaudeCode and ToolOpenCode are the legacy tool values once held by
// Settings.Tool (and still accepted by the session launcher's --tool alias).
// New code should use Profiles instead.
const (
	ToolClaudeCode = "claude-code"
	ToolOpenCode   = "opencode"
)

// ProfileClaudePro, ProfileZAI, ProfileOpenCodeGo, ProfileOpenCodeZen and
// ProfileOpenCodeZenFree are the five supported subscription profiles:
// Settings.Profile holds one of them, and the session launcher accepts them
// as --profile values.
const (
	ProfileClaudePro       = "claude-pro"
	ProfileZAI             = "zai"
	ProfileOpenCodeGo      = "opencode-go"
	ProfileOpenCodeZen     = "opencode-zen"
	ProfileOpenCodeZenFree = "opencode-zen-free"
)

// DefaultRecentActivityCount is the maximum number of entries the dashboard's
// recent-activity strip may show when Settings.RecentActivityCount is unset
// (zero) — the pre-existing fixed ceiling of 5.
const DefaultRecentActivityCount = 5

// Profile describes one of the subscription profiles manigot can run a session
// under. Each profile bundles the agent CLI to launch, the credentials it is
// billed against, and the model the CLI should default to — the whole point is
// that switching profile, not tool, is what changes which subscription is used.
type Profile struct {
	// ID is the value Settings.Profile (and the session launcher's
	// --profile flag) holds: ProfileClaudePro, ProfileZAI, ProfileOpenCodeGo,
	// ProfileOpenCodeZen or ProfileOpenCodeZenFree.
	ID string

	// Label is a short human name for the settings screen and help text.
	Label string

	// Tool is the agent CLI this profile launches: ToolClaudeCode or
	// ToolOpenCode.
	Tool string

	// Auth names the credential the profile is billed against, for display in
	// the settings screen and setup help.
	Auth string
}

// Profiles is the ordered list of subscription profiles, in the order the TUI
// settings screen cycles them. No model is listed here: the opencode profiles' model comes
// from the OPENCODE_ZAI_MODEL/OPENCODE_GO_MODEL/OPENCODE_ZEN_MODEL/
// OPENCODE_ZEN_FREE_MODEL values in manigot's .env, which the TUI does not
// read — showing a compiled-in default would mislead.
var profiles = []Profile{
	{ID: ProfileClaudePro, Label: "Claude Code · Claude Pro", Tool: ToolClaudeCode, Auth: "Claude Pro OAuth"},
	{ID: ProfileZAI, Label: "OpenCode · Z.AI Coding Plan", Tool: ToolOpenCode, Auth: "ZHIPU_API_KEY"},
	{ID: ProfileOpenCodeGo, Label: "OpenCode · Go", Tool: ToolOpenCode, Auth: "OPENCODE_API_KEY"},
	{ID: ProfileOpenCodeZen, Label: "OpenCode · Zen", Tool: ToolOpenCode, Auth: "OPENCODE_API_KEY"},
	{ID: ProfileOpenCodeZenFree, Label: "OpenCode · Zen Free", Tool: ToolOpenCode, Auth: "OPENCODE_API_KEY"},
}

// Profiles returns the supported subscription profiles. The slice is built
// fresh so a caller cannot mutate the shared table.
func Profiles() []Profile {
	return append([]Profile(nil), profiles...)
}

// ProfileByID returns the profile with the given ID.
func ProfileByID(id string) (Profile, bool) {
	for _, p := range profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// ProfileTool returns the agent CLI a profile runs, defaulting to
// ToolClaudeCode for an unrecognized ID.
func ProfileTool(id string) string {
	if p, ok := ProfileByID(id); ok {
		return p.Tool
	}
	return ToolClaudeCode
}

// Settings holds the TUI's persisted preferences.
type Settings struct {
	// Editor is the command run to open brief.md (the detail view's "e"
	// shortcut), e.g. "vim" or "nano". Empty means fall back to
	// $VISUAL/$EDITOR/nano/vi — see editor.Resolve.
	Editor string `json:"editor"`

	// Profile selects the subscription profile launch.Agent/Quick use:
	// ProfileClaudePro, ProfileZAI, ProfileOpenCodeGo, ProfileOpenCodeZen or
	// ProfileOpenCodeZenFree. It is the shared
	// default — loaded from MANIGOT_PROFILE in manigot/.env (with the legacy
	// profile/tool fields of tui-settings.json as migration fallbacks) and
	// persisted to .env by Save, never to tui-settings.json. Empty is treated
	// as ProfileClaudePro — see ProfileValue.
	Profile string `json:"profile,omitempty"`

	// RecentActivityCount is the maximum number of entries the dashboard's
	// recent-activity strip may show (git.RecentCommits' fetch count and the
	// clamp upper bound in recentActivityShown). 0 (unset — the JSON zero
	// value) means DefaultRecentActivityCount — see RecentActivityCountValue.
	// Valid user-set range is 1–100, enforced by the settings form.
	RecentActivityCount int `json:"recentActivityCount"`

	// Tool is the legacy pre-profile selector (ToolClaudeCode/ToolOpenCode),
	// kept only so old tui-settings.json files load. It is migrated into
	// Profile by Load and never written back.
	Tool string `json:"tool,omitempty"`

	// Terminal is the command used to spawn an agent session's terminal
	// (launch.Agent/Quick/AgentQuick), e.g. "kitty" or "alacritty -e". Empty
	// means fall back to auto-detect (macOS Terminal.app, then a Linux
	// emulator list). When set, it applies only when the TUI is not inside
	// tmux: inside tmux the launch is always a tmux split pane, regardless of
	// this setting — see internal/launch's buildCmd.
	Terminal string `json:"terminal,omitempty"`
}

// ProfileValue returns s.Profile, defaulting to ProfileClaudePro when unset.
func (s Settings) ProfileValue() string {
	if s.Profile == "" {
		return ProfileClaudePro
	}
	return s.Profile
}

// RecentActivityCountValue returns s.RecentActivityCount, defaulting to
// DefaultRecentActivityCount when unset (≤ 0, the JSON zero value).
func (s Settings) RecentActivityCountValue() int {
	if s.RecentActivityCount <= 0 {
		return DefaultRecentActivityCount
	}
	return s.RecentActivityCount
}

// EnvFile returns the manigot checkout's .env file path, or "" if Dir does.
// .env is the shared store for the default profile: MANIGOT_PROFILE there is
// read by bare `mg` runs, written by `mg profiles`, and read/written here so
// the TUI and CLI share one profile default.
func EnvFile() string {
	home := home.Root()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".env")
}

// GetEnv returns the value of key in the manigot checkout's .env ("" when
// absent or unreadable). Lines may carry an `export ` prefix or surrounding
// quotes — both are tolerated, mirroring the leniency of sourcing a shell
// env file. It is the generic form of the profile lookup that
// readEnvProfile wraps; the ported CLI commands (mg profiles, mg setup) use it
// to read any credential key.
func GetEnv(key string) string {
	v, _ := envValue(key)
	return v
}

// envValue returns the value of key in the manigot checkout's .env, reporting
// whether the key was present at all (an explicit "KEY=" counts as
// present-but-empty, which matters once the file is sourced: sourcing
// overwrites an inherited value with the empty one).
func envValue(key string) (string, bool) {
	data, err := os.ReadFile(EnvFile())
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		return unquoteEnvValue(strings.TrimSpace(value)), true
	}
	return "", false
}

// EnvValue returns the effective value of key the way the old scripts saw it:
// the .env value when the key is present there (sourcing exports it into the
// process environment, overwriting anything inherited), else the process
// environment. The ported CLI commands use it for readiness checks so an
// inherited export counts as configured, exactly like the scripts' `${!KEY:-}`
// reads did.
func EnvValue(key string) string {
	if v, ok := envValue(key); ok {
		return v
	}
	return os.Getenv(key)
}

// unquoteEnvValue strips one pair of surrounding double or single quotes from
// an .env value, mirroring how the shell would source it.
func unquoteEnvValue(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// UpsertEnv upserts key=value into the manigot checkout's .env, preserving
// every other line (credentials, comments, model overrides). The file is
// created with the standard header when it does not exist yet. value is
// written unquoted, matching the format config's own reader tolerates.
// It is the generic form of writeEnvProfile; the ported CLI commands use it to
// write any credential or model key.
func UpsertEnv(key, value string) error {
	path := EnvFile()
	if path == "" {
		return fmt.Errorf("cannot determine the manigot checkout to write %s into (set $%s)", key, home.EnvHome)
	}

	content, err := os.ReadFile(path)
	var lines []string
	switch {
	case err == nil:
		lines = strings.Split(string(content), "\n")
	case os.IsNotExist(err):
		// Seed a fresh file with the same header scripts/profiles.sh and
		// scripts/setup.sh write.
		lines = []string{"# manigot configuration — credentials and defaults (never commit this file)"}
	default:
		return err
	}

	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if k, _, ok := strings.Cut(strings.TrimPrefix(trimmed, "export "), "="); ok && strings.TrimSpace(k) == key {
			lines[i] = key + "=" + value
			replaced = true
			break
		}
	}
	if !replaced {
		if last := lines[len(lines)-1]; last == "" {
			// The file ended with a newline — reuse the trailing empty
			// element rather than leaving a blank line above the append.
			lines[len(lines)-1] = key + "=" + value
		} else {
			// No trailing newline — add one so the new line starts fresh.
			lines = append(lines, "", key+"="+value)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// readEnvProfile returns the MANIGOT_PROFILE value from manigot/.env ("" when
// absent or unreadable). It is a best-effort scan tolerant of shell-isms in
// the file: a missing .env is the normal pre-first
// save state and the caller falls back to the legacy tui-settings.json fields
// and finally to ProfileClaudePro. Lines may carry an `export ` prefix or
// surrounding quotes — both are tolerated.
func readEnvProfile() string {
	return GetEnv("MANIGOT_PROFILE")
}

// writeEnvProfile upserts MANIGOT_PROFILE=<profile> into manigot/.env,
// preserving every other line (credentials, comments, model overrides). The
// file is created with the standard header when it does not exist yet.
// profile must already be validated by the caller (see Save); it is written
// unquoted, matching the format config's own reader tolerates.
func writeEnvProfile(profile string) error {
	return UpsertEnv("MANIGOT_PROFILE", profile)
}

// Dir returns the directory the settings file lives in: config/ inside the
// manigot checkout the running binary belongs to (see home.Root). It is
// "" if that checkout could not be located.
func Dir() string {
	home := home.Root()
	if home == "" {
		return ""
	}
	return filepath.Join(home, "config")
}

// Path returns the settings file's path, or "" if Dir does.
func Path() string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "tui-settings.json")
}

// Load reads the settings. A file that does not exist yet (nothing has been
// saved, or the checkout could not be located) is not an error — it yields the
// zero-value Settings, which every caller treats as "defaults".
//
// Profile resolution order: MANIGOT_PROFILE in manigot/.env — the shared,
// central store, written by both `mg profiles` and the TUI's settings screen —
// wins when present and valid. Failing that, the legacy `profile` field of
// tui-settings.json (and the even older `tool` field, migrated:
// "claude-code" maps to ProfileClaudePro, "opencode" maps to ProfileZAI) is
// honored as a migration fallback so an existing user's choice survives the
// upgrade. Neither legacy field is written back — Save persists the profile to
// .env only.
func Load() (Settings, error) {
	path := Path()
	var s Settings
	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &s); err != nil {
				return Settings{}, fmt.Errorf("%s: %w", path, err)
			}
		case os.IsNotExist(err):
			// nothing saved yet — zero value, defaults apply
		default:
			return Settings{}, err
		}
	}

	if envProfile := readEnvProfile(); envProfile != "" {
		if _, ok := ProfileByID(envProfile); ok {
			s.Profile = envProfile
		}
	}
	if s.Profile == "" && s.Tool != "" {
		switch s.Tool {
		case ToolOpenCode:
			s.Profile = ProfileZAI
		default:
			s.Profile = ProfileClaudePro
		}
	}
	s.Tool = "" // legacy field — not written back
	return s, nil
}

// Save writes s to disk: the editor to config/tui-settings.json (creating
// config/ if needed) and the profile to manigot/.env as MANIGOT_PROFILE —
// the shared default bare `mg` and the TUI both use. The profile is no longer
// written to tui-settings.json: that field (like the older `tool` field) is
// legacy, read for migration by Load only.
func Save(s Settings) error {
	dir := Dir()
	if dir == "" {
		return fmt.Errorf("cannot determine the manigot checkout to save settings into (set $%s)", home.EnvHome)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	legacy := s
	legacy.Profile = "" // not persisted here anymore — `omitempty` drops it
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "tui-settings.json"), data, 0o644); err != nil {
		return err
	}

	profile := s.ProfileValue()
	if _, ok := ProfileByID(profile); !ok {
		return fmt.Errorf("cannot save: %q is not a valid profile id", profile)
	}
	return writeEnvProfile(profile)
}
