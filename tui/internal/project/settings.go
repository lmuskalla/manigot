// Package project persists project-scoped manigot settings — conventions that
// belong to the project itself (not to a single user's TUI) and are meant to be
// committed and shared across a team. It is the project-level counterpart to
// the personal/global config package (editor, subscription profile), which
// stays gitignored and per-user.
//
// The first such project convention is the base branch: the ref new job
// branches are cut from and the TUI's "m" quick-checkout lands on. It lives
// here rather than in config because base branch is a shared project
// convention, not a personal preference.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings holds the project-scoped manigot conventions, read from / written
// to docs/manigot.json in the target project's root. Only fields with a
// project-wide (committable, shareable) lifecycle belong here; anything
// personal (editor, subscription profile) stays in the global config package.
type Settings struct {
	// BaseBranch is the ref new job branches are cut from and the TUI's
	// list-view "m" quick-checkout lands on. Empty is treated as "main" —
	// see BaseBranchValue.
	BaseBranch string `json:"baseBranch,omitempty"`
}

// BaseBranchValue returns s.BaseBranch, defaulting to "main" when unset so
// every reader (the TUI's "m" checkout, scripts/new-job.sh) gets a usable ref
// even when docs/manigot.json is absent or doesn't set the key.
func (s Settings) BaseBranchValue() string {
	if s.BaseBranch == "" {
		return "main"
	}
	return s.BaseBranch
}

// Path returns the project settings file's path under the given project root:
// <root>/docs/manigot.json. The root is the target project root the TUI was
// launched in (App.root), not the manigot checkout — these settings travel
// with the project being worked on, not with the tool.
func Path(root string) string {
	return filepath.Join(root, "docs", "manigot.json")
}

// Load reads the project settings file under root. A file that does not exist
// yet (the project hasn't been initialized, or no project settings have ever
// been saved) is not an error — it yields the zero-value Settings, which every
// caller treats as "defaults" via BaseBranchValue. This mirrors config.Load's
// shape exactly so the two packages degrade the same way.
func Load(root string) (Settings, error) {
	data, err := os.ReadFile(Path(root))
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("%s: %w", Path(root), err)
	}
	return s, nil
}

// Save writes s to the project settings file under root. The docs/ directory
// is created if it doesn't already exist, but in any initialized project
// (which is the TUI's baseline — jobs live in docs/jobs/) it always does.
// The file is committable: it holds only a public ref name, no secrets.
func Save(root string, s Settings) error {
	dir := filepath.Join(root, "docs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(root), data, 0o644)
}
