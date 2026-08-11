// Package config persists local TUI preferences across sessions: which
// editor to open brief.md in, and which subscription profile (a bundle of
// agent CLI + credentials + model) launch.Agent starts a session under. The
// settings screen (see ui/settings.go) is the only writer; every other reader
// treats a missing or unreadable file as "use the defaults" rather than an
// error.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lmuskalla/manigot/tui/internal/resolve"
)

// ToolClaudeCode and ToolOpenCode are the legacy tool values once held by
// Settings.Tool (and still accepted by scripts/run.sh's --tool flag). They are
// kept for migration and for the run.sh contract; new code should use
// Profiles instead.
const (
	ToolClaudeCode = "claude-code"
	ToolOpenCode   = "opencode"
)

// ProfileClaudePro, ProfileZAI and ProfileOpenCodeGo are the three supported
// subscription profiles: Settings.Profile holds one of them, and scripts/run.sh
// accepts them as --profile values. Keep the IDs in sync with the profile
// table in scripts/run.sh.
const (
	ProfileClaudePro  = "claude-pro"
	ProfileZAI        = "zai"
	ProfileOpenCodeGo = "opencode-go"
)

// Profile describes one of the subscription profiles manigot can run a session
// under. Each profile bundles the agent CLI to launch, the credentials it is
// billed against, and the model the CLI should default to — the whole point is
// that switching profile, not tool, is what changes which subscription is used.
type Profile struct {
	// ID is the value Settings.Profile and scripts/run.sh's --profile flag
	// hold: ProfileClaudePro, ProfileZAI or ProfileOpenCodeGo.
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
// settings screen cycles them. Keep the IDs in sync with the profile table in
// scripts/run.sh. No model is listed here: the opencode profiles' model comes
// from the OPENCODE_ZAI_MODEL/OPENCODE_GO_MODEL values in manigot's .env,
// which the TUI does not read — showing a compiled-in default would mislead.
var profiles = []Profile{
	{ID: ProfileClaudePro, Label: "Claude Code · Claude Pro", Tool: ToolClaudeCode, Auth: "Claude Pro OAuth"},
	{ID: ProfileZAI, Label: "OpenCode · Z.AI Coding Plan", Tool: ToolOpenCode, Auth: "ZHIPU_API_KEY"},
	{ID: ProfileOpenCodeGo, Label: "OpenCode · Go", Tool: ToolOpenCode, Auth: "OPENCODE_API_KEY"},
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
	// ProfileClaudePro, ProfileZAI or ProfileOpenCodeGo. Empty is treated as
	// ProfileClaudePro — see ProfileValue.
	Profile string `json:"profile,omitempty"`

	// Tool is the legacy pre-profile selector (ToolClaudeCode/ToolOpenCode),
	// kept only so old tui-settings.json files load. It is migrated into
	// Profile by Load and never written back.
	Tool string `json:"tool,omitempty"`
}

// ProfileValue returns s.Profile, defaulting to ProfileClaudePro when unset.
func (s Settings) ProfileValue() string {
	if s.Profile == "" {
		return ProfileClaudePro
	}
	return s.Profile
}

// Dir returns the directory the settings file lives in: config/ inside the
// manigot checkout the running binary belongs to (see resolve.Home). It is
// "" if that checkout could not be located.
func Dir() string {
	home := resolve.Home()
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

// Load reads the settings file. A file that does not exist yet (nothing has
// been saved, or the checkout could not be located) is not an error — it
// yields the zero-value Settings, which every caller treats as "defaults".
// A file written by an older manigot that only stored the legacy Tool field
// is migrated: "claude-code" maps to ProfileClaudePro, "opencode" maps to
// ProfileZAI (the opencode subscription manigot historically configured).
func Load() (Settings, error) {
	path := Path()
	if path == "" {
		return Settings{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("%s: %w", path, err)
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

// Save writes s to the settings file, creating config/ if it does not exist
// yet.
func Save(s Settings) error {
	dir := Dir()
	if dir == "" {
		return fmt.Errorf("cannot determine the manigot checkout to save settings into (set $%s)", resolve.EnvHome)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tui-settings.json"), data, 0o644)
}
