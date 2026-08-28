// Package serve is the manigot daemon — the "listener" control plane
// described in docs/web-interface.md and the listener brief: a long-running
// `mg serve` process exposing a read-only control API over the existing
// in-process machinery (internal/job, internal/git, the mg jdi state machine,
// the session launcher), so any surface — a web UI, a native GUI, a future
// CLI — can attach to it as a client, from localhost or from a VPS.
//
// One package owns the daemon: the project registry, the HTTP server and its
// handlers, auth, the audit log, and the per-project serialization skeleton —
// mirroring how internal/session owns the session launcher. The scope of the
// v1 API is deliberately read-only (see the brief's out-of-scope list);
// mutating endpoints are a later job and inherit the serialization pattern
// established here.
package serve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/fs"
)

// RegistryFileName is the name of the registry config file inside the
// manigot checkout's config/ directory.
const RegistryFileName = "serve.json"

// DefaultRegistryPath returns the default registry config location:
// <manigot checkout>/config/serve.json (config.Dir()). "" when the checkout
// cannot be located — the caller (mg serve) then reports a clear error.
func DefaultRegistryPath() string {
	dir := config.Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "serve.json")
}

// registryFile is the on-disk JSON shape of the registry config file.
type registryFile struct {
	Projects []string `json:"projects"`
}

// Registry is the daemon's explicit list of project roots. It is read once at
// startup from a config file — no scanning, no auto-adopting directories the
// daemon finds. Changing the registered roots means editing the config file
// and restarting (v1). Every handler resolves ONLY against these roots via
// Projects()/Project() — the single choke point the zero-path-inputs
// enforcement (see api.go's resolution helpers) builds on.
type Registry struct {
	projects []string // absolute, cleaned, in config-file order
}

// LoadRegistry reads the registry config file at path and validates every
// entry. A missing file is an empty registry, not an error (a fresh checkout
// simply has nothing registered yet). An unreadable-but-present file, an
// unparseable file, or an entry that is not an existing directory is an
// error — a broken registry must never silently serve a subset.
//
// Each registered root must exist as a directory. A root without docs/ is
// accepted (it lists no jobs rather than failing startup); the caller may
// warn about it — see (*Registry).WarnMissingDocs.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, fmt.Errorf("read registry %s: %w", path, err)
	}

	var raw registryFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse registry %s: %w", path, err)
	}

	reg := &Registry{}
	seen := make(map[string]bool)
	for _, p := range raw.Projects {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("registry %s: resolve %q: %w", path, p, err)
		}
		abs = filepath.Clean(abs)
		if seen[abs] {
			continue // duplicate registrations collapse to one
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("registry %s: %q is not an existing directory: %w", path, p, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("registry %s: %q is not a directory", path, p)
		}
		seen[abs] = true
		reg.projects = append(reg.projects, abs)
	}
	return reg, nil
}

// Projects returns an ordered copy of the registered project roots. The copy
// guarantees a caller cannot mutate the registry's internal slice.
func (r *Registry) Projects() []string {
	return append([]string(nil), r.projects...)
}

// Project resolves a URL project segment against the registered roots: an
// exact path match first, then a unique base-name match (e.g. "/projects/foo"
// resolves a root registered at /home/user/foo). Nothing is ever joined into
// a filesystem path here — the returned root IS one of the registered roots,
// never a derivation from the input. ok is false when the segment matches no
// root, or when several roots share the same base name (ambiguous — the
// caller returns 404 rather than guessing).
func (r *Registry) Project(segment string) (string, bool) {
	for _, root := range r.projects {
		if root == segment {
			return root, true
		}
	}
	var match string
	for _, root := range r.projects {
		if filepath.Base(root) == segment {
			if match != "" {
				return "", false // ambiguous base name
			}
			match = root
		}
	}
	if match != "" {
		return match, true
	}
	return "", false
}

// WarnMissingDocs writes a warning line for every registered root without a
// docs/ directory to w — the "a registered project needs docs/ for jobs, but
// a root without docs/ lists no jobs rather than failing startup" behavior
// the brief calls for. w may be nil (no warnings).
func (r *Registry) WarnMissingDocs(w io.Writer) {
	for _, root := range r.projects {
		if !fs.IsDir(filepath.Join(root, "docs")) {
			fmt.Fprintf(w, "Warning: registered project %s has no docs/ directory — it will list no jobs.\n", root)
		}
	}
}

// ErrNoRegistryPath is returned by mg serve when the registry path cannot be
// determined (no --registry flag and the manigot checkout cannot be located).
var ErrNoRegistryPath = errors.New("cannot determine the registry config location (set $MANIGOT_HOME or pass --registry)")
