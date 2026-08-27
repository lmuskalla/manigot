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

// A permission: block (the OpenCode read-only restriction — see the
// read-only agents' files) must survive the strip untouched: name:/tools:/
// commit: are dropped, permission: and its indented rules are preserved
// verbatim, so the OpenCode copy enforces the same read-only restriction
// under OpenCode's own schema that tools: expresses under Claude Code. This
// includes the deny rules (git worktree*, git branch -D*, ...); commit: is
// stripped alongside name:/tools: — see TestConvertAgentFileStripsCommitMarker.
func TestConvertAgentFilePreservesPermissionBlock(t *testing.T) {
	src := `---
name: reviewer
description: Reviews changes against the original task requirements.
tools: Read, Write, Grep, Glob, Bash
commit: true
permission:
  edit:
    "*": deny
    "docs/jobs/**/verdict.md": allow
  bash:
    "*": deny
    "git add *": allow
    "git worktree*": deny
    "git branch -D*": deny
  task: deny
  webfetch: deny
  websearch: deny
  question: deny
---

You are read-only.
`
	want := `---
description: Reviews changes against the original task requirements.
permission:
  edit:
    "*": deny
    "docs/jobs/**/verdict.md": allow
  bash:
    "*": deny
    "git add *": allow
    "git worktree*": deny
    "git branch -D*": deny
  task: deny
  webfetch: deny
  websearch: deny
  question: deny
---

You are read-only.
`
	if got := string(convertAgentFile([]byte(src))); got != want {
		t.Errorf("convertAgentFile dropped or mangled the permission block:\n got: %q\nwant: %q", got, want)
	}
}

// A permission: block that directly follows a multi-line map-form tools:
// block must also survive: the drop-block state machine (droppingToolsBlock)
// has to end at the permission: line — its first non-indented line — rather
// than eating the whole permission block as a tools continuation.
func TestConvertAgentFilePermissionAfterMapFormTools(t *testing.T) {
	src := `---
name: custom
description: A custom read-only agent.
tools:
  read: true
permission:
  edit: deny
  bash: deny
---

Body.
`
	want := `---
description: A custom read-only agent.
permission:
  edit: deny
  bash: deny
---

Body.
`
	if got := string(convertAgentFile([]byte(src))); got != want {
		t.Errorf("convertAgentFile mangled permission: following a map-form tools: block:\n got: %q\nwant: %q", got, want)
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

// The commit: frontmatter marker (which agents commit — see agentgit.go) is
// manigot's own construct, meaningless to OpenCode's agent schema. It must be
// stripped like name:/tools: — left in, OpenCode forwards it as an extra
// top-level field into the chat completions request, which OpenCode Zen's
// strict validator rejects ("Extra inputs are not permitted, field:
// 'commit'").
func TestConvertAgentFileStripsCommitMarker(t *testing.T) {
	src := `---
name: developer
description: Implements tasks.
tools: Read, Write, Edit, Bash, Grep, Glob
commit: true
---

Body.
`
	want := `---
description: Implements tasks.
---

Body.
`
	if got := string(convertAgentFile([]byte(src))); got != want {
		t.Errorf("convertAgentFile did not strip the commit: marker:\n got: %q\nwant: %q", got, want)
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
