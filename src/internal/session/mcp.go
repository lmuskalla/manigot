// mcp.go implements manigot's MCP (Model Context Protocol) server delivery
// mechanism: a global+project directory pair (<home>/mcp/, docs/mcp/)
// mirroring how agents/ + docs/agents/ and skills/ + docs/skills/ are
// discovered and merged (see agentconv.go/skillconv.go), a canonical
// CLI-agnostic per-server schema, $VARNAME secret resolution against
// manigot's own .env, and conversion into each CLI's native config shape
// (Claude Code's .mcp.json, OpenCode's opencode.json `mcp` block).
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lmuskalla/manigot/internal/config"
)

// MCPServer is one canonical MCP server definition — manigot's own
// CLI-agnostic shape, not either CLI's native one. A hosted HTTP server sets
// Type "http" plus URL and optional Headers; a locally-spawned server sets
// Type "stdio" plus Command, optional Args and optional Env. Any string
// value may contain "$VARNAME" tokens, resolved against manigot's own .env
// by resolveMCPServers (see below) — servers never carry a literal secret in
// a committed file.
type MCPServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPServers is a name → server map, keyed by the defining file's basename
// without its .json extension — the identifier both CLIs address the server
// by once converted.
type MCPServers map[string]MCPServer

// loadMCPServers enumerates every *.json file directly under dir, parsing
// each into an MCPServer keyed by its filename (without the .json
// extension), mirroring listSkills/convertAgents's discovery pattern. A
// missing dir returns (nil, nil) — MCP servers are optional, every caller
// skips cleanly when there is nothing to deliver. A file whose Type is
// neither "http" nor "stdio" is a hard parse error: a malformed server
// definition must fail loudly rather than silently vanish from every
// session.
func loadMCPServers(dir string) (MCPServers, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var servers MCPServers
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var s MCPServer
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if s.Type != "http" && s.Type != "stdio" {
			return nil, fmt.Errorf("parse %s: unknown type %q (want \"http\" or \"stdio\")", path, s.Type)
		}
		if servers == nil {
			servers = MCPServers{}
		}
		servers[strings.TrimSuffix(e.Name(), ".json")] = s
	}
	return servers, nil
}

// mergeMCPServers merges the global set with the project set by name — the
// project entry wins on a filename collision (replacing the global server
// entirely), and a project-only filename adds a new server — exactly the
// agents/skills global+project precedence. Either input may be nil; the
// result is nil only when both are empty.
func mergeMCPServers(global, project MCPServers) MCPServers {
	if len(global) == 0 && len(project) == 0 {
		return nil
	}
	merged := make(MCPServers, len(global)+len(project))
	for name, s := range global {
		merged[name] = s
	}
	for name, s := range project {
		merged[name] = s
	}
	return merged
}

// discoverMCPServers loads and merges the global (<homeDir>/mcp) and project
// (<docsDir>/mcp) server sets — the entry point BuildDockerInvocation calls
// once per session. Either homeDir or docsDir may be "" (no manigot checkout
// resolved / no docs dir), in which case that side contributes nothing.
func discoverMCPServers(homeDir, docsDir string) (MCPServers, error) {
	var global, project MCPServers
	var err error
	if homeDir != "" {
		if global, err = loadMCPServers(filepath.Join(homeDir, "mcp")); err != nil {
			return nil, fmt.Errorf("load global mcp servers: %w", err)
		}
	}
	if docsDir != "" {
		if project, err = loadMCPServers(filepath.Join(docsDir, "mcp")); err != nil {
			return nil, fmt.Errorf("load project mcp servers: %w", err)
		}
	}
	return mergeMCPServers(global, project), nil
}

// mcpVarPattern matches a "$VARNAME" token: a dollar sign followed by a
// shell-identifier-shaped name (letters, digits, underscore; not starting
// with a digit).
var mcpVarPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)

// resolveEnvRefs substitutes every "$VARNAME" token in s with the value of
// that key in manigot's own .env (config.EnvValue — the same resolution
// CheckAuth already uses for credential keys elsewhere in this package). An
// unset key resolves to "" rather than an error; see resolveEnvMap for how
// an empty result is handled per field. Deliberately does not rely on either
// CLI's own config-file env expansion (OpenCode's {env:VAR}, an unconfirmed
// Claude Code equivalent) — manigot resolves the token itself, host-side,
// uniformly for both CLIs (see the brief's "Out of scope" note).
func resolveEnvRefs(s string) string {
	if s == "" {
		return s
	}
	return mcpVarPattern.ReplaceAllStringFunc(s, func(m string) string {
		return config.EnvValue(m[1:])
	})
}

// resolveEnvMap resolves every value in m via resolveEnvRefs, dropping any
// entry whose resolved value comes out empty (an unset .env key) — e.g.
// Context7's optional CONTEXT7_API_KEY header is omitted entirely instead of
// being sent as an empty string, so the server falls back to its
// unauthenticated rate limit rather than seeing an empty credential. Returns
// nil when the result would be empty.
func resolveEnvMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if rv := resolveEnvRefs(v); rv != "" {
			out[k] = rv
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// resolveEnvSlice resolves every element of args via resolveEnvRefs. Returns
// nil when args is empty.
func resolveEnvSlice(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = resolveEnvRefs(a)
	}
	return out
}

// resolveMCPServers resolves every "$VARNAME" token in every string value of
// every server in servers (see resolveEnvRefs/resolveEnvMap/resolveEnvSlice).
// Returns nil when servers is empty.
func resolveMCPServers(servers MCPServers) MCPServers {
	if len(servers) == 0 {
		return nil
	}
	resolved := make(MCPServers, len(servers))
	for name, s := range servers {
		resolved[name] = MCPServer{
			Type:    s.Type,
			URL:     resolveEnvRefs(s.URL),
			Headers: resolveEnvMap(s.Headers),
			Command: resolveEnvRefs(s.Command),
			Args:    resolveEnvSlice(s.Args),
			Env:     resolveEnvMap(s.Env),
		}
	}
	return resolved
}

// openCodeMCPServer is one entry of OpenCode's native mcp config block
// (Config.mcp in the opencode-ai package's own config schema — confirmed
// against the installed opencode-ai 1.18.25 binary, not documented anywhere
// in this codebase before this job, since neither the schema nor any usage
// of it existed here). A hosted server (canonical Type "http") becomes
// OpenCode's "remote" type, carrying url/headers; a locally-spawned server
// (canonical Type "stdio") becomes "local", carrying command (the
// executable and its args merged into a single array — OpenCode's schema
// has no separate args field) and environment (OpenCode's name for env
// vars, not "env"). Field names deliberately differ from the canonical
// schema; this struct is the remap TASK-4 exists to do.
type openCodeMCPServer struct {
	Type        string            `json:"type"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// openCodeMCPBlock converts the resolved canonical server set into
// OpenCode's native mcp config block (Config.mcp: a name → server map).
// Returns nil for an empty input. loadMCPServers already rejects any Type
// other than "http"/"stdio" at parse time, so every entry here converts.
func openCodeMCPBlock(servers MCPServers) map[string]openCodeMCPServer {
	if len(servers) == 0 {
		return nil
	}
	block := make(map[string]openCodeMCPServer, len(servers))
	for name, s := range servers {
		switch s.Type {
		case "http":
			block[name] = openCodeMCPServer{Type: "remote", URL: s.URL, Headers: s.Headers}
		case "stdio":
			cmd := append([]string{s.Command}, s.Args...)
			block[name] = openCodeMCPServer{Type: "local", Command: cmd, Environment: s.Env}
		}
	}
	return block
}

// openCodeConfigSchema is the "$schema" value scripts/entrypoint.sh's
// removed model-only write used — kept here so the generated file still
// self-describes for editors/tooling that resolve it.
const openCodeConfigSchema = "https://opencode.ai/config.json"

// openCodeConfig is the generated opencode.json's shape: the resolved model
// (formerly written by scripts/entrypoint.sh via a "{env:OPENCODE_MODEL}"
// config-file substitution; now resolved host-side and written directly —
// the model name is not a secret, so no indirection is needed) plus the
// resolved mcp block (TASK-4). tui.json (the theme) stays a separate file,
// written by entrypoint.sh untouched — nothing to merge there.
type openCodeConfig struct {
	Schema string                       `json:"$schema"`
	Model  string                       `json:"model,omitempty"`
	MCP    map[string]openCodeMCPServer `json:"mcp,omitempty"`
}

// buildOpenCodeConfig assembles the complete opencode.json content: model
// (possibly "") plus the resolved mcp server set (possibly empty). Returns
// (nil, false, nil) when there is nothing to write at all — model=="" and no
// servers — matching the pre-this-job behavior of entrypoint.sh's guarded
// write (no model, no file, OpenCode falls back to its own built-in
// default).
func buildOpenCodeConfig(model string, servers MCPServers) ([]byte, bool, error) {
	mcpBlock := openCodeMCPBlock(servers)
	if model == "" && len(mcpBlock) == 0 {
		return nil, false, nil
	}
	cfg := openCodeConfig{Schema: openCodeConfigSchema, Model: model, MCP: mcpBlock}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// claudeMCPConfig serializes the resolved server set into Claude Code's
// native .mcp.json shape: {"mcpServers": {"<name>": {...}}}. The canonical
// schema (type/url/headers or type/command/args/env) already matches
// Claude's own per-server shape field-for-field, so this is a wholesale
// wrap, not a field remap. Returns (nil, false, nil) when servers is empty —
// nothing to generate.
func claudeMCPConfig(servers MCPServers) ([]byte, bool, error) {
	if len(servers) == 0 {
		return nil, false, nil
	}
	doc := struct {
		MCPServers MCPServers `json:"mcpServers"`
	}{MCPServers: servers}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// stageMCPFile writes data into filename inside a fresh temp directory,
// mirroring convertAgents/stageGlobalSkills's single-generated-file staging
// pattern: BuildDockerInvocation mounts the returned dir's file read-only
// and removes the whole dir via the invocation's Cleanup hook, so the host's
// own tree is never touched by a generated config.
func stageMCPFile(filename string, data []byte) (string, error) {
	tmp, err := os.MkdirTemp("", "manigot-mcp-*")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, filename), data, 0o644); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return tmp, nil
}
