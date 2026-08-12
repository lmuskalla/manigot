// Package agentlist discovers every agent available to the current project,
// mirroring scripts/agents.sh's own listing so the TUI's agent picker
// (TASK-3) shows exactly the same set `mg agents` would.
package agentlist

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lmuskalla/manigot/internal/home"
)

// Agent describes one entry in the picker: its name (the `@name` an agent
// session is launched with) and its one-line description, taken from the
// agent file's `description:` frontmatter key.
type Agent struct {
	Name        string
	Description string
}

// Discover returns every agent available to the project at projectRoot, in
// the same order scripts/agents.sh presents them: the global agents/*.md
// shipped in the manigot checkout (sorted by name), each swapped for its
// docs/agents/<name>.md override in projectRoot when one exists, followed by
// any project-only docs/agents/ additions that don't shadow a global name
// (also sorted by name).
//
// The manigot checkout root is resolved via home.Root() — the same
// $MANIGOT_HOME-then-executable-location logic every other host-command
// resolution in this TUI already uses — rather than a second lookup
// strategy. A checkout that can't be found (e.g. a binary copied somewhere
// home.Root() can't place) is reported as an error so the caller can
// degrade to a status line instead of showing an empty or broken picker.
func Discover(projectRoot string) ([]Agent, error) {
	home := home.Root()
	if home == "" {
		return nil, errors.New("cannot find the manigot checkout (no $MANIGOT_HOME and the running binary's location doesn't look like one) — agents/*.md cannot be listed")
	}
	globalDir := filepath.Join(home, "agents")
	globalFiles, err := globAgentFiles(globalDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", globalDir, err)
	}

	var projectDir string
	if projectRoot != "" {
		projectDir = filepath.Join(projectRoot, "docs", "agents")
	}

	globalNames := map[string]bool{}
	var agents []Agent
	for _, f := range globalFiles {
		name := agentName(f)
		globalNames[name] = true
		file := f
		if projectDir != "" {
			override := filepath.Join(projectDir, name+".md")
			if isFile(override) {
				file = override
			}
		}
		desc, err := readDescription(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		agents = append(agents, Agent{Name: name, Description: desc})
	}

	if projectDir != "" {
		projectFiles, err := globAgentFiles(projectDir)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", projectDir, err)
		}
		for _, f := range projectFiles {
			name := agentName(f)
			if globalNames[name] {
				continue // already listed above, using the override
			}
			desc, err := readDescription(f)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", f, err)
			}
			agents = append(agents, Agent{Name: name, Description: desc})
		}
	}

	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents found under %s", globalDir)
	}
	return agents, nil
}

// globAgentFiles lists the *.md files directly inside dir, sorted by name. A
// missing directory is not an error — it returns an empty slice, matching
// scripts/agents.sh's own "docs/agents/ is optional" handling for the
// project-agents case (the global agents dir is expected to always exist;
// its absence is caught separately by the caller).
func globAgentFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// agentName derives an agent's name from its file path: the base name
// without the .md extension.
func agentName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// readDescription extracts the first `description:` frontmatter line's value
// from an agent file, matching scripts/agents.sh's `describe()` (a `sed -n
// '/^description:/s/^description: *//p' | head -1`). Returns "(no
// description)" when the file has none, same as the shell script's `${desc:-
// (no description)}` fallback.
func readDescription(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "description:") {
			desc := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			if desc == "" {
				break
			}
			return desc, nil
		}
	}
	return "(no description)", nil
}

// isFile reports whether path exists and is a regular file (not a
// directory).
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
