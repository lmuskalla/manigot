package session

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/config"
)

// convertAgents converts every agent markdown file in srcDir (a project's
// docs/agents/) into the target tool's agent format, writing the converted
// files into a fresh temp directory. It returns the temp dir path and true
// when srcDir contained at least one *.md file; ("", false, nil) when srcDir
// is missing or empty (nothing to convert); and ("", false, err) on any
// failure, after removing any temp dir it already created.
//
// For ToolClaudeCode no conversion is needed at all — the list-form
// frontmatter (name:/description:/tools: Read, Grep, ...) is Claude's native
// subagent schema — so it always returns ("", false, nil).
//
// For ToolOpenCode the conversion mirrors the Dockerfile's bake-time awk for
// the built-in agents (which strips the `name:`, `tools:` and `commit:`
// frontmatter keys from the OpenCode copies of agents/*.md): `name:` is
// dropped because OpenCode derives the agent name from the filename, and
// `tools:` is dropped because OpenCode requires tools as a map and would
// hard-error on the list form ("Expected object | undefined, got ...").
// `commit:` is dropped because it is manigot's own git-mount-mode marker (see
// agentgit.go), unknown to OpenCode's agent schema — left in, OpenCode
// forwards it as an extra top-level field into the chat completions request,
// which OpenCode Zen's strict validator rejects outright ("Extra inputs are
// not permitted, field: 'commit'"). A `permission:` block — the OpenCode
// schema the read-only agents (reviewer/security/analyst/owner) use to
// express their restriction — passes through untouched, since OpenCode
// itself recognizes that key. The strip also handles multi-line map-form
// `tools:` blocks, so a custom agent written as an object today converts
// cleanly instead of leaving orphaned indented keys.
//
// The caller shadows the docs mount's agents/ subpath with the returned temp
// dir for OpenCode sessions (see BuildDockerInvocation) and removes it via
// the invocation's Cleanup hook (see DockerInvocation.Run) — the host's
// docs/agents/ source tree is never modified.
func convertAgents(srcDir, tool string) (string, bool, error) {
	if tool != config.ToolOpenCode {
		return "", false, nil
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, e.Name()) // os.ReadDir sorts by name
	}
	if len(names) == 0 {
		return "", false, nil
	}

	tmp, err := os.MkdirTemp("", "manigot-agents-*")
	if err != nil {
		return "", false, err
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			os.RemoveAll(tmp)
			return "", false, err
		}
		if err := os.WriteFile(filepath.Join(tmp, name), convertAgentFile(data), 0o644); err != nil {
			os.RemoveAll(tmp)
			return "", false, err
		}
	}
	return tmp, true, nil
}

// convertAgentFile strips the frontmatter `name:`, `tools:` and `commit:`
// keys from an agent file, preserving the rest of the frontmatter and the
// body verbatim — the Go equivalent of the Dockerfile's bake-time awk:
//
//	awk 'BEGIN{fm=0} /^---$/{fm++; print; next} fm==1 && /^(name|tools|commit):/{next} {print}'
//
// One deliberate extension over the awk: a multi-line map-form `tools:` block
// (tools: followed by indented entries) is dropped whole, not just its header
// line, so object-form agent files convert to clean frontmatter instead of
// leaving orphaned indented keys behind.
func convertAgentFile(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	out := make([]string, 0, len(lines))
	inFrontmatter := false
	droppingToolsBlock := false
	for _, line := range lines {
		if line == "---" {
			inFrontmatter = !inFrontmatter
			droppingToolsBlock = false
			out = append(out, line)
			continue
		}
		if !inFrontmatter {
			out = append(out, line)
			continue
		}
		if droppingToolsBlock {
			// Indented continuation of a map-form tools: block — drop it
			// until the first non-indented line (a new key, or the closing
			// --- handled above).
			if line == "" || line[0] == ' ' || line[0] == '\t' {
				continue
			}
			droppingToolsBlock = false
		}
		if strings.HasPrefix(line, "name:") || strings.HasPrefix(line, "tools:") || strings.HasPrefix(line, "commit:") {
			if strings.HasPrefix(line, "tools:") {
				droppingToolsBlock = true
			}
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}
