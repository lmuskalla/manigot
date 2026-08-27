// Package brand holds the manigot ASCII logo — the single canonical source
// of the logo string shared by every render site: the host-side session
// banner (internal/session) and the TUI job-list header (internal/ui). Both
// call brand.Logo(), so the two surfaces can never drift.
package brand

import (
	"os"
	"path/filepath"

	"github.com/lmuskalla/manigot/internal/home"
)

// Logo returns the ASCII logo from assets/manigot.txt, or "" when the file
// is missing or unreadable — the same no-error convention pickQuote applies
// to assets/quotes.json (internal/session/docker.go): a missing logo just
// means no logo this session, never an error.
func Logo() string {
	root := home.Root()
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "assets", "manigot.txt"))
	if err != nil {
		return ""
	}
	return string(data)
}
