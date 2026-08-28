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
	"regexp"

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

// registryEntry is the on-disk JSON shape of one registry entry: an
// operator-chosen name (the URL segment the daemon serves it under) and its
// filesystem root. The flat-string form ("projects": ["/abs/root"]) is not
// supported — json.Unmarshal of a string into this struct fails naturally,
// which is exactly the "no backwards compatibility" behavior the brief calls
// for.
type registryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// registryFile is the on-disk JSON shape of the registry config file.
type registryFile struct {
	Projects []registryEntry `json:"projects"`
}

// Entry is one registered project: an operator-chosen name (the URL segment
// that resolves to it) and its absolute, cleaned filesystem root.
type Entry struct {
	Name string
	Path string
}

// projectNamePattern is the URL-safe charset a registry entry's name must
// match: ASCII letters, digits, dot, underscore, hyphen. A name is served
// verbatim in /projects and matched byte-for-byte against incoming URL
// segments (see api.go's resolveProject), so keeping the charset
// conservative avoids any ambiguity from encoding, case-folding, or reserved
// URL characters — a stricter requirement than validSegment's structural
// no-traversal/no-separator discipline, which this also reuses.
var projectNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// validProjectName reports whether name is an acceptable registry entry
// name: validSegment's structural discipline (non-empty, not "." or "..", no
// path separator, no NUL) plus the conservative URL-safe charset above.
func validProjectName(name string) bool {
	return validSegment(name) && projectNamePattern.MatchString(name)
}

// Registry is the daemon's explicit list of named project entries. It is
// read once at startup from a config file — no scanning, no auto-adopting
// directories the daemon finds. Changing the registered entries means
// editing the config file and restarting (v1). Every handler resolves ONLY
// against these entries via Entries()/Project() — the single choke point the
// zero-path-inputs enforcement (see api.go's resolution helpers) builds on.
type Registry struct {
	entries []Entry // name + absolute, cleaned path, in config-file order
}

// LoadRegistry reads the registry config file at path and validates every
// entry. A missing file is an empty registry, not an error (a fresh checkout
// simply has nothing registered yet). An unreadable-but-present file, an
// unparseable file, an invalid or duplicate name, a duplicate path, or an
// entry whose path is not an existing directory is an error — a broken
// registry must never silently serve a subset.
//
// Each entry's name must be non-empty, a single URL-safe path segment
// (validProjectName), and unique across the registry. Each entry's path must
// exist as a directory; a root without docs/ is accepted (it lists no jobs
// rather than failing startup) — the caller may warn about it, see
// (*Registry).WarnMissingDocs. Two entries registering the same path (under
// different names) are also refused: a path is intended to be reachable
// under exactly one operator-chosen identity, and rejecting keeps the
// "refuse to start on anything surprising" discipline the rest of the
// registry's validation follows, rather than silently picking one name over
// the other.
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
	seenNames := make(map[string]bool)
	seenPaths := make(map[string]string) // abs path -> the name that claimed it
	for _, e := range raw.Projects {
		if !validProjectName(e.Name) {
			return nil, fmt.Errorf("registry %s: entry name %q is invalid — it must be non-empty, a single URL-safe path segment (%s), not \".\" or \"..\"", path, e.Name, projectNamePattern.String())
		}
		if seenNames[e.Name] {
			return nil, fmt.Errorf("registry %s: duplicate entry name %q", path, e.Name)
		}

		abs, err := filepath.Abs(e.Path)
		if err != nil {
			return nil, fmt.Errorf("registry %s: entry %q: resolve %q: %w", path, e.Name, e.Path, err)
		}
		abs = filepath.Clean(abs)
		if other, ok := seenPaths[abs]; ok {
			return nil, fmt.Errorf("registry %s: entry %q and %q both register path %q", path, other, e.Name, abs)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("registry %s: entry %q: %q is not an existing directory: %w", path, e.Name, e.Path, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("registry %s: entry %q: %q is not a directory", path, e.Name, e.Path)
		}

		seenNames[e.Name] = true
		seenPaths[abs] = e.Name
		reg.entries = append(reg.entries, Entry{Name: e.Name, Path: abs})
	}
	return reg, nil
}

// Entries returns an ordered copy of the registered entries (name + path).
// The copy guarantees a caller cannot mutate the registry's internal slice.
func (r *Registry) Entries() []Entry {
	return append([]Entry(nil), r.entries...)
}

// Projects returns an ordered copy of the registered project roots' paths.
// The copy guarantees a caller cannot mutate the registry's internal slice.
func (r *Registry) Projects() []string {
	paths := make([]string, len(r.entries))
	for i, e := range r.entries {
		paths[i] = e.Path
	}
	return paths
}

// Project resolves a URL project segment against the registered entries by
// configured name ONLY — names are validated unique at load time, so there
// is no ambiguity to resolve. Nothing is ever joined into a filesystem path
// here — the returned root IS one of the registered paths, never a
// derivation from the input. ok is false when the segment matches no entry's
// name.
func (r *Registry) Project(segment string) (string, bool) {
	for _, e := range r.entries {
		if e.Name == segment {
			return e.Path, true
		}
	}
	return "", false
}

// WarnMissingDocs writes a warning line for every registered root without a
// docs/ directory to w — the "a registered project needs docs/ for jobs, but
// a root without docs/ lists no jobs rather than failing startup" behavior
// the brief calls for. w may be nil (no warnings).
func (r *Registry) WarnMissingDocs(w io.Writer) {
	for _, e := range r.entries {
		if !fs.IsDir(filepath.Join(e.Path, "docs")) {
			fmt.Fprintf(w, "Warning: registered project %s (%s) has no docs/ directory — it will list no jobs.\n", e.Name, e.Path)
		}
	}
}

// ErrNoRegistryPath is returned by mg serve when the registry path cannot be
// determined (no --registry flag and the manigot checkout cannot be located).
var ErrNoRegistryPath = errors.New("cannot determine the registry config location (set $MANIGOT_HOME or pass --registry)")
