package brand

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLogoFoundFile verifies Logo returns the exact file content when
// assets/manigot.txt exists under the resolved home.
func TestLogoFoundFile(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := ".---.\n| # |\n'---'\n"
	if err := os.WriteFile(filepath.Join(home, "assets", "manigot.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", home)
	if got := Logo(); got != content {
		t.Errorf("Logo() = %q, want the exact file content %q", got, content)
	}
}

// TestLogoMissingFile verifies Logo returns "" when assets/manigot.txt does
// not exist — a missing logo is not an error (mirrors pickQuote).
func TestLogoMissingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MANIGOT_HOME", home)
	if got := Logo(); got != "" {
		t.Errorf("Logo() on a missing file = %q, want empty", got)
	}
}
