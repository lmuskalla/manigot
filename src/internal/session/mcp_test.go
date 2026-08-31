package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeMCPServer writes a canonical MCP server definition file at
// <dir>/<name>.json.
func writeMCPServer(t *testing.T, dir, name string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMCPServersMissingDir(t *testing.T) {
	servers, err := loadMCPServers(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("loadMCPServers: %v", err)
	}
	if servers != nil {
		t.Errorf("loadMCPServers on a missing dir = %v, want nil", servers)
	}
}

func TestLoadMCPServersEmptyDir(t *testing.T) {
	dir := t.TempDir()
	servers, err := loadMCPServers(dir)
	if err != nil {
		t.Fatalf("loadMCPServers: %v", err)
	}
	if servers != nil {
		t.Errorf("loadMCPServers on an empty dir = %v, want nil", servers)
	}
}

func TestLoadMCPServersHTTP(t *testing.T) {
	dir := t.TempDir()
	writeMCPServer(t, dir, "context7", `{"type": "http", "url": "https://mcp.context7.com/mcp", "headers": {"CONTEXT7_API_KEY": "$CONTEXT7_API_KEY"}}`)
	servers, err := loadMCPServers(dir)
	if err != nil {
		t.Fatalf("loadMCPServers: %v", err)
	}
	want := MCPServers{
		"context7": {
			Type:    "http",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"CONTEXT7_API_KEY": "$CONTEXT7_API_KEY"},
		},
	}
	if !reflect.DeepEqual(servers, want) {
		t.Errorf("loadMCPServers = %+v, want %+v", servers, want)
	}
}

func TestLoadMCPServersStdio(t *testing.T) {
	dir := t.TempDir()
	writeMCPServer(t, dir, "local-thing", `{"type": "stdio", "command": "some-mcp", "args": ["--flag"], "env": {"TOKEN": "$SOME_TOKEN"}}`)
	servers, err := loadMCPServers(dir)
	if err != nil {
		t.Fatalf("loadMCPServers: %v", err)
	}
	want := MCPServers{
		"local-thing": {
			Type:    "stdio",
			Command: "some-mcp",
			Args:    []string{"--flag"},
			Env:     map[string]string{"TOKEN": "$SOME_TOKEN"},
		},
	}
	if !reflect.DeepEqual(servers, want) {
		t.Errorf("loadMCPServers = %+v, want %+v", servers, want)
	}
}

// TestLoadMCPServersIgnoresNonJSON: only *.json files at the top level of
// the dir are considered — a stray non-JSON file or a subdirectory is
// skipped, mirroring listSkills/convertAgents's own filtering.
func TestLoadMCPServersIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	writeMCPServer(t, dir, "context7", `{"type": "http", "url": "https://mcp.context7.com/mcp"}`)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	servers, err := loadMCPServers(dir)
	if err != nil {
		t.Fatalf("loadMCPServers: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("loadMCPServers = %+v, want exactly one entry (context7)", servers)
	}
}

func TestLoadMCPServersUnknownTypeErrors(t *testing.T) {
	dir := t.TempDir()
	writeMCPServer(t, dir, "bogus", `{"type": "websocket", "url": "wss://example.com"}`)
	if _, err := loadMCPServers(dir); err == nil {
		t.Error("loadMCPServers with an unknown type must error, got nil")
	}
}

func TestLoadMCPServersInvalidJSONErrors(t *testing.T) {
	dir := t.TempDir()
	writeMCPServer(t, dir, "bogus", `{not valid json`)
	if _, err := loadMCPServers(dir); err == nil {
		t.Error("loadMCPServers with invalid JSON must error, got nil")
	}
}

func TestMergeMCPServersProjectWinsByFilename(t *testing.T) {
	global := MCPServers{
		"context7":    {Type: "http", URL: "https://mcp.context7.com/mcp"},
		"only-global": {Type: "http", URL: "https://global.example.com/mcp"},
	}
	project := MCPServers{
		"context7":     {Type: "http", URL: "https://project-override.example.com/mcp"},
		"only-project": {Type: "http", URL: "https://project.example.com/mcp"},
	}
	merged := mergeMCPServers(global, project)
	want := MCPServers{
		"context7":     {Type: "http", URL: "https://project-override.example.com/mcp"},
		"only-global":  {Type: "http", URL: "https://global.example.com/mcp"},
		"only-project": {Type: "http", URL: "https://project.example.com/mcp"},
	}
	if !reflect.DeepEqual(merged, want) {
		t.Errorf("mergeMCPServers = %+v, want %+v", merged, want)
	}
}

func TestMergeMCPServersBothEmptyIsNil(t *testing.T) {
	if merged := mergeMCPServers(nil, nil); merged != nil {
		t.Errorf("mergeMCPServers(nil, nil) = %+v, want nil", merged)
	}
}

func TestDiscoverMCPServersGlobalAndProject(t *testing.T) {
	home := t.TempDir()
	writeMCPServer(t, filepath.Join(home, "mcp"), "context7", `{"type": "http", "url": "https://mcp.context7.com/mcp"}`)

	docs := t.TempDir()
	writeMCPServer(t, filepath.Join(docs, "mcp"), "project-only", `{"type": "http", "url": "https://project.example.com/mcp"}`)

	merged, err := discoverMCPServers(home, docs)
	if err != nil {
		t.Fatalf("discoverMCPServers: %v", err)
	}
	if len(merged) != 2 || merged["context7"].URL != "https://mcp.context7.com/mcp" || merged["project-only"].URL != "https://project.example.com/mcp" {
		t.Errorf("discoverMCPServers = %+v", merged)
	}
}

func TestDiscoverMCPServersNoneConfigured(t *testing.T) {
	merged, err := discoverMCPServers(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("discoverMCPServers: %v", err)
	}
	if merged != nil {
		t.Errorf("discoverMCPServers with nothing configured = %+v, want nil", merged)
	}
}

func TestDiscoverMCPServersEmptyHomeAndDocs(t *testing.T) {
	merged, err := discoverMCPServers("", "")
	if err != nil {
		t.Fatalf("discoverMCPServers: %v", err)
	}
	if merged != nil {
		t.Errorf("discoverMCPServers(\"\", \"\") = %+v, want nil", merged)
	}
}

// TestResolveMCPServersSubstitutesVar: a $VARNAME token in a header value is
// replaced with that key's value from manigot's own .env.
func TestResolveMCPServersSubstitutesVar(t *testing.T) {
	checkout(t, "CONTEXT7_API_KEY=ctx7-secret\n")
	servers := MCPServers{
		"context7": {
			Type:    "http",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"CONTEXT7_API_KEY": "$CONTEXT7_API_KEY"},
		},
	}
	resolved := resolveMCPServers(servers)
	want := MCPServers{
		"context7": {
			Type:    "http",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7-secret"},
		},
	}
	if !reflect.DeepEqual(resolved, want) {
		t.Errorf("resolveMCPServers = %+v, want %+v", resolved, want)
	}
}

// TestResolveMCPServersUnsetVarDropsHeader: an unset $VARNAME resolves to
// "", and the whole header/env entry is dropped rather than sent empty —
// Context7 works unauthenticated at a lower rate limit when
// CONTEXT7_API_KEY isn't configured.
func TestResolveMCPServersUnsetVarDropsHeader(t *testing.T) {
	checkout(t, "")
	servers := MCPServers{
		"context7": {
			Type:    "http",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"CONTEXT7_API_KEY": "$CONTEXT7_API_KEY"},
		},
	}
	resolved := resolveMCPServers(servers)
	want := MCPServers{
		"context7": {
			Type: "http",
			URL:  "https://mcp.context7.com/mcp",
		},
	}
	if !reflect.DeepEqual(resolved, want) {
		t.Errorf("resolveMCPServers = %+v, want %+v", resolved, want)
	}
}

// TestResolveMCPServersStdioEnvAndArgs: $VARNAME substitution also applies
// to a stdio server's command, args, and env values.
func TestResolveMCPServersStdioEnvAndArgs(t *testing.T) {
	checkout(t, "SOME_TOKEN=tok-123\n")
	servers := MCPServers{
		"local": {
			Type:    "stdio",
			Command: "some-mcp",
			Args:    []string{"--token=$SOME_TOKEN"},
			Env:     map[string]string{"TOKEN": "$SOME_TOKEN"},
		},
	}
	resolved := resolveMCPServers(servers)
	want := MCPServers{
		"local": {
			Type:    "stdio",
			Command: "some-mcp",
			Args:    []string{"--token=tok-123"},
			Env:     map[string]string{"TOKEN": "tok-123"},
		},
	}
	if !reflect.DeepEqual(resolved, want) {
		t.Errorf("resolveMCPServers = %+v, want %+v", resolved, want)
	}
}

func TestResolveMCPServersEmptyIsNil(t *testing.T) {
	if resolved := resolveMCPServers(nil); resolved != nil {
		t.Errorf("resolveMCPServers(nil) = %+v, want nil", resolved)
	}
}

// TestClaudeMCPConfigWrapsServers: the resolved server set serializes into
// Claude Code's native {"mcpServers": {...}} shape, field-for-field
// identical to the canonical schema.
func TestClaudeMCPConfigWrapsServers(t *testing.T) {
	servers := MCPServers{
		"context7": {
			Type:    "http",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7-secret"},
		},
	}
	data, ok, err := claudeMCPConfig(servers)
	if err != nil {
		t.Fatalf("claudeMCPConfig: %v", err)
	}
	if !ok {
		t.Fatal("claudeMCPConfig ok = false, want true")
	}
	var doc struct {
		MCPServers MCPServers `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal generated .mcp.json: %v", err)
	}
	if !reflect.DeepEqual(doc.MCPServers, servers) {
		t.Errorf("claudeMCPConfig round-trip = %+v, want %+v", doc.MCPServers, servers)
	}
}

func TestClaudeMCPConfigEmptyServers(t *testing.T) {
	data, ok, err := claudeMCPConfig(nil)
	if err != nil {
		t.Fatalf("claudeMCPConfig: %v", err)
	}
	if ok || data != nil {
		t.Errorf("claudeMCPConfig(nil) = (%q, %v), want (nil, false)", data, ok)
	}
}

// TestOpenCodeMCPBlockHTTPBecomesRemote: a canonical "http" server converts
// to OpenCode's "remote" type, url/headers carried through unchanged.
func TestOpenCodeMCPBlockHTTPBecomesRemote(t *testing.T) {
	servers := MCPServers{
		"context7": {
			Type:    "http",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7-secret"},
		},
	}
	block := openCodeMCPBlock(servers)
	want := map[string]openCodeMCPServer{
		"context7": {
			Type:    "remote",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7-secret"},
		},
	}
	if !reflect.DeepEqual(block, want) {
		t.Errorf("openCodeMCPBlock = %+v, want %+v", block, want)
	}
}

// TestOpenCodeMCPBlockStdioBecomesLocal: a canonical "stdio" server converts
// to OpenCode's "local" type, command+args merged into a single command
// array and env renamed to environment.
func TestOpenCodeMCPBlockStdioBecomesLocal(t *testing.T) {
	servers := MCPServers{
		"local-thing": {
			Type:    "stdio",
			Command: "some-mcp",
			Args:    []string{"--flag", "value"},
			Env:     map[string]string{"TOKEN": "tok-123"},
		},
	}
	block := openCodeMCPBlock(servers)
	want := map[string]openCodeMCPServer{
		"local-thing": {
			Type:        "local",
			Command:     []string{"some-mcp", "--flag", "value"},
			Environment: map[string]string{"TOKEN": "tok-123"},
		},
	}
	if !reflect.DeepEqual(block, want) {
		t.Errorf("openCodeMCPBlock = %+v, want %+v", block, want)
	}
}

func TestOpenCodeMCPBlockEmptyIsNil(t *testing.T) {
	if block := openCodeMCPBlock(nil); block != nil {
		t.Errorf("openCodeMCPBlock(nil) = %+v, want nil", block)
	}
}

// TestBuildOpenCodeConfigModelOnly pins TASK-4's model migration: with no
// MCP servers configured, the generated opencode.json still carries the
// resolved model — the logic scripts/entrypoint.sh used to write via
// {env:OPENCODE_MODEL} substitution, now resolved and written directly.
func TestBuildOpenCodeConfigModelOnly(t *testing.T) {
	data, ok, err := buildOpenCodeConfig("zai-coding-plan/glm-5.2", nil)
	if err != nil {
		t.Fatalf("buildOpenCodeConfig: %v", err)
	}
	if !ok {
		t.Fatal("buildOpenCodeConfig ok = false, want true")
	}
	var cfg openCodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal generated opencode.json: %v", err)
	}
	if cfg.Schema != openCodeConfigSchema || cfg.Model != "zai-coding-plan/glm-5.2" || len(cfg.MCP) != 0 {
		t.Errorf("buildOpenCodeConfig = %+v", cfg)
	}
}

// TestBuildOpenCodeConfigModelAndMCP: model and mcp compose into one
// generated file.
func TestBuildOpenCodeConfigModelAndMCP(t *testing.T) {
	servers := MCPServers{"context7": {Type: "http", URL: "https://mcp.context7.com/mcp"}}
	data, ok, err := buildOpenCodeConfig("zai-coding-plan/glm-5.2", servers)
	if err != nil {
		t.Fatalf("buildOpenCodeConfig: %v", err)
	}
	if !ok {
		t.Fatal("buildOpenCodeConfig ok = false, want true")
	}
	var cfg openCodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal generated opencode.json: %v", err)
	}
	if cfg.Model != "zai-coding-plan/glm-5.2" {
		t.Errorf("buildOpenCodeConfig Model = %q", cfg.Model)
	}
	if got := cfg.MCP["context7"]; got.Type != "remote" || got.URL != "https://mcp.context7.com/mcp" {
		t.Errorf("buildOpenCodeConfig MCP[context7] = %+v", got)
	}
}

// TestBuildOpenCodeConfigNothingConfigured: no model, no servers — nothing
// is generated at all, matching the pre-this-job behavior of
// entrypoint.sh's guarded write.
func TestBuildOpenCodeConfigNothingConfigured(t *testing.T) {
	data, ok, err := buildOpenCodeConfig("", nil)
	if err != nil {
		t.Fatalf("buildOpenCodeConfig: %v", err)
	}
	if ok || data != nil {
		t.Errorf("buildOpenCodeConfig(\"\", nil) = (%q, %v), want (nil, false)", data, ok)
	}
}

// TestBuildOpenCodeConfigMCPOnlyNoModel: MCP servers configured but no
// model resolved — the file is still generated (mcp alone is enough).
func TestBuildOpenCodeConfigMCPOnlyNoModel(t *testing.T) {
	servers := MCPServers{"context7": {Type: "http", URL: "https://mcp.context7.com/mcp"}}
	data, ok, err := buildOpenCodeConfig("", servers)
	if err != nil {
		t.Fatalf("buildOpenCodeConfig: %v", err)
	}
	if !ok || data == nil {
		t.Fatal("buildOpenCodeConfig with mcp servers but no model must still generate a file")
	}
	var cfg openCodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal generated opencode.json: %v", err)
	}
	if cfg.Model != "" {
		t.Errorf("buildOpenCodeConfig Model = %q, want empty", cfg.Model)
	}
}
