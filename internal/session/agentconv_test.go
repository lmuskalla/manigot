package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lmuskalla/manigot/internal/config"
)

func TestConvertAgentFileStripsListFormFrontmatter(t *testing.T) {
	// The built-in style: name: and tools: dropped, the rest of the
	// frontmatter and the body preserved verbatim — the Go equivalent of the
	// Dockerfile's bake-time awk for the OpenCode copies.
	src := `---
name: analyst
description: Breaks a brief into atomic tasks.
tools: Read, Grep, Glob, Write, Edit
---

You are a senior software architect.
`
	want := `---
description: Breaks a brief into atomic tasks.
---

You are a senior software architect.
`
	if got := string(convertAgentFile([]byte(src))); got != want {
		t.Errorf("convertAgentFile:\n got: %q\nwant: %q", got, want)
	}
}

func TestConvertAgentFileStripsMapFormToolsBlock(t *testing.T) {
	// A custom agent written as an object today (multi-line map-form tools:)
	// must convert cleanly: the whole indented block is dropped, not just the
	// tools: header line, so no orphaned keys survive.
	src := `---
name: custom
description: A custom agent.
tools:
  read: true
  write: true
mode: subagent
---

Body.
`
	want := `---
description: A custom agent.
mode: subagent
---

Body.
`
	if got := string(convertAgentFile([]byte(src))); got != want {
		t.Errorf("convertAgentFile:\n got: %q\nwant: %q", got, want)
	}
}

func TestConvertAgentFileNoFrontmatterPassthrough(t *testing.T) {
	src := "Just a body, no frontmatter.\n"
	if got := string(convertAgentFile([]byte(src))); got != src {
		t.Errorf("convertAgentFile changed a frontmatter-less file:\n got: %q\nwant: %q", got, src)
	}
}

func TestConvertAgentFileBodyToolsLineUntouched(t *testing.T) {
	// Only the frontmatter is converted — a "tools:" mention in the body
	// (outside the frontmatter block) must survive.
	src := `---
name: a
description: d
tools: Read
---

The tools: key in the body is prose.
`
	want := `---
description: d
---

The tools: key in the body is prose.
`
	if got := string(convertAgentFile([]byte(src))); got != want {
		t.Errorf("convertAgentFile touched the body:\n got: %q\nwant: %q", got, want)
	}
}

func TestConvertAgentsOpenCode(t *testing.T) {
	dir := t.TempDir()
	agents := filepath.Join(dir, "docs", "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(agents, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("custom.md", "---\nname: custom\ndescription: Custom agent.\ntools: Read, Grep\n---\n\nBody.\n")
	write("notes.txt", "not an agent") // non-.md ignored

	tmp, ok, err := convertAgents(agents, config.ToolOpenCode)
	if err != nil {
		t.Fatalf("convertAgents: %v", err)
	}
	if !ok {
		t.Fatal("convertAgents reported no agents for a non-empty docs/agents/")
	}
	defer os.RemoveAll(tmp)

	data, err := os.ReadFile(filepath.Join(tmp, "custom.md"))
	if err != nil {
		t.Fatalf("converted custom.md missing: %v", err)
	}
	want := "---\ndescription: Custom agent.\n---\n\nBody.\n"
	if got := string(data); got != want {
		t.Errorf("converted content:\n got: %q\nwant: %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(tmp, "notes.txt")); !os.IsNotExist(err) {
		t.Errorf("non-.md file was copied into the converted dir: %v", err)
	}
}

func TestConvertAgentsNoOpCases(t *testing.T) {
	// Claude Code needs no conversion — its list form is the native schema.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "agents", "a.md"), []byte("---\ntools: Read\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if tmp, ok, err := convertAgents(filepath.Join(dir, "docs", "agents"), config.ToolClaudeCode); err != nil || ok || tmp != "" {
		t.Errorf("claude tool: got (%q, %v, %v), want (\"\", false, nil)", tmp, ok, err)
	}

	// Missing docs/agents/ is not an error.
	if tmp, ok, err := convertAgents(filepath.Join(dir, "docs", "nope"), config.ToolOpenCode); err != nil || ok || tmp != "" {
		t.Errorf("missing dir: got (%q, %v, %v), want (\"\", false, nil)", tmp, ok, err)
	}

	// Empty docs/agents/ is not an error.
	if err := os.MkdirAll(filepath.Join(dir, "docs", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if tmp, ok, err := convertAgents(filepath.Join(dir, "docs", "empty"), config.ToolOpenCode); err != nil || ok || tmp != "" {
		t.Errorf("empty dir: got (%q, %v, %v), want (\"\", false, nil)", tmp, ok, err)
	}
}
