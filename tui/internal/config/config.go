// Package config persists local TUI preferences across sessions: which
// editor to open brief.md in, and which agent tool (Claude Code or OpenCode)
// launch.Agent starts. The settings screen (see ui/settings.go) is the only
// writer; every other reader treats a missing or unreadable file as "use the
// defaults" rather than an error.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lmuskalla/manigot/tui/internal/resolve"
)

// ToolClaudeCode and ToolOpenCode are the values Settings.Tool holds and the
// values accepted by scripts/run.sh's --tool flag.
const (
	ToolClaudeCode = "claude-code"
	ToolOpenCode   = "opencode"
)

// Settings holds the TUI's persisted preferences.
type Settings struct {
	// Editor is the command run to open brief.md (the detail view's "e"
	// shortcut), e.g. "vim" or "nano". Empty means fall back to
	// $VISUAL/$EDITOR/nano/vi — see editor.Resolve.
	Editor string `json:"editor"`

	// Tool selects the agent CLI launch.Agent starts: ToolClaudeCode or
	// ToolOpenCode. Empty is treated as ToolClaudeCode, matching
	// scripts/run.sh's own default — see ToolValue.
	Tool string `json:"tool"`
}

// ToolValue returns s.Tool, defaulting to ToolClaudeCode when unset.
func (s Settings) ToolValue() string {
	if s.Tool == "" {
		return ToolClaudeCode
	}
	return s.Tool
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
