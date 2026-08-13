package session

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/fs"
	"github.com/lmuskalla/manigot/internal/home"
)

// agentCommits reports whether the resolved agent may make git commits — the
// `commit:` frontmatter marker in agents/*.md (commit: true for developer,
// reviewer, and quality, who commit their work; commit: false for the
// read-only agents).
// The session launcher uses it to decide the git-common-dir mount mode: rw
// for committing agents, ro (+ GIT_OPTIONAL_LOCKS=0) for non-committing ones.
//
// Resolution mirrors agentlist.Discover's global-vs-project override logic: a
// docs/agents/<name>.md override in the project replaces the global
// agents/<name>.md wholesale, so the override's own marker decides. Every
// unknown case — no --agent, no agent file found (global or project), or a
// marker that is absent/unparseable — defaults to true (rw), so a committing
// agent is never broken by a missing or stale marker.
func agentCommits(opts Options, root Root) bool {
	name := opts.Agent
	if name == "" {
		return true
	}
	// Project override first (docs/agents/<name>.md), then the global file.
	path := filepath.Join(root.DocsDir, "agents", name+".md")
	if !fs.IsFile(path) {
		homeDir := home.Root()
		if homeDir == "" {
			return true
		}
		path = filepath.Join(homeDir, "agents", name+".md")
	}
	if !fs.IsFile(path) {
		return true
	}
	return commitMarker(path)
}

// commitMarker reads the `commit:` frontmatter key from an agent file: true
// for "true"/"yes"/"1", false for "false"/"no"/"0", and the default (true)
// when the key is absent or unparseable. Only the frontmatter block (the
// region between the leading --- markers) is scanned, so a "commit:" mention
// in the body can't leak in.
func commitMarker(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	inFrontmatter := false
	for _, line := range strings.Split(string(data), "\n") {
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			// Closing delimiter: the frontmatter is over.
			break
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(line, "commit:") {
			switch strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "commit:"))) {
			case "true", "yes", "1":
				return true
			case "false", "no", "0":
				return false
			default:
				return true
			}
		}
	}
	return true
}
