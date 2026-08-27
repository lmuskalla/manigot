package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/config"
)

// fakeHostBinary makes hostLookPath succeed without requiring claude/opencode
// on the test machine's PATH.
func fakeHostBinary(t *testing.T) {
	t.Helper()
	old := hostLookPath
	hostLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { hostLookPath = old })
}

// envMap parses an env slice (KEY=VALUE lines) into a map for value-precise
// assertions. Needed because the checkout helper clears the credential keys to
// empty in the process env, so presence checks against the joined env would
// false-positive on the cleared keys.
func envMap(t *testing.T, env []string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, line := range env {
		if k, v, ok := strings.Cut(line, "="); ok {
			m[k] = v
		}
	}
	return m
}

func TestBuildPlainClaudeHostSession(t *testing.T) {
	root, _ := docProject(t)
	fakeHostBinary(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	var diag strings.Builder
	inv, err := BuildHostInvocation(Options{}, info, r, &diag)
	if err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	// A direct claude invocation from the resolved root — not a docker run.
	if inv.Argv[0] != "claude" {
		t.Errorf("argv[0] = %q, want claude", inv.Argv[0])
	}
	if inv.Dir != root {
		t.Errorf("Dir = %q, want %q", inv.Dir, root)
	}
	// No docker machinery: no docker/run, mounts, -e env args, limits or image.
	joined := strings.Join(inv.Argv, "\n")
	for _, forbidden := range []string{"docker", "run", "--rm", "-v ", "-e", "--network=bridge", "--memory=", "--security-opt=", "manigot"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("host argv must not carry %q:\n%s", forbidden, joined)
		}
	}
	// No auto-approval flags — host sessions stay supervised.
	for _, forbidden := range []string{"--dangerously-skip-permissions", "--auto"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("host argv must not carry the auto-approval flag %q:\n%s", forbidden, joined)
		}
	}
	// Credentials live in the child env (hostEnv), not docker -e args.
	env := strings.Join(inv.Env, "\n")
	for _, want := range []string{
		"CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-token",
		"CLAUDE_ACCOUNT_UUID=uuid-1",
		"CLAUDE_EMAIL=me@x.io",
		"CLAUDE_ORG_UUID=org-1",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("host env missing %q", want)
		}
	}
	if !strings.Contains(diag.String(), "Entering host mode") {
		t.Errorf("missing host-mode banner:\n%s", diag.String())
	}
}

func TestBuildHostOpenCodeProfile(t *testing.T) {
	_, _ = docProject(t)
	fakeHostBinary(t)
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=z-secret\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildHostInvocation(Options{}, info, r, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	if inv.Argv[0] != "opencode" {
		t.Errorf("argv[0] = %q, want opencode", inv.Argv[0])
	}
	// The profile's plan model is forwarded via opencode's own --model flag
	// (never by writing the user's host opencode config, and never via
	// OPENCODE_MODEL env, which opencode does not read).
	containsAll(t, inv.Argv, "--model", "zai-coding-plan/glm-5.2")
	m := envMap(t, inv.Env)
	if m["ZHIPU_API_KEY"] != "z-secret" {
		t.Errorf("host env ZHIPU_API_KEY = %q, want z-secret", m["ZHIPU_API_KEY"])
	}
	if m["OPENCODE_MODEL"] != "" {
		t.Errorf("host env must not forward OPENCODE_MODEL — the model goes via --model (got %q)", m["OPENCODE_MODEL"])
	}
	joined := strings.Join(inv.Argv, "\n")
	for _, forbidden := range []string{"--auto", "-e", "docker"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("opencode host argv must not carry %q:\n%s", forbidden, joined)
		}
	}
	// An opencode profile must not forward the claude-pro subscription keys.
	for _, k := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"} {
		if m[k] != "" {
			t.Errorf("opencode host env forwards %s=%q, want only the profile's own keys", k, m[k])
		}
	}
}

// TestBuildHostOpenCodeStripsTmux — the mg host OpenCode path shares the
// docker path's root cause: opencode runs with the full host env (TMUX set),
// so it would emit tmux's DCS-passthrough OSC 52 form, which default tmux
// config (allow-passthrough off) discards. hostEnv must filter TMUX/TMUX_PANE
// out of the OpenCode child env (Claude Code keeps the full host env).
func TestBuildHostOpenCodeStripsTmux(t *testing.T) {
	_, _ = docProject(t)
	fakeHostBinary(t)
	checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=z-secret\n")
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	t.Setenv("TMUX_PANE", "%5")
	t.Setenv("TERM", "xterm-256color")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if info.Tool != config.ToolOpenCode {
		t.Fatalf("zai profile tool = %q, want %q", info.Tool, config.ToolOpenCode)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildHostInvocation(Options{}, info, r, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	m := envMap(t, inv.Env)
	if m["TMUX"] != "" {
		t.Errorf("opencode host env must not carry TMUX (got %q)", m["TMUX"])
	}
	if m["TMUX_PANE"] != "" {
		t.Errorf("opencode host env must not carry TMUX_PANE (got %q)", m["TMUX_PANE"])
	}
	// The rest of the host env passes through untouched.
	if m["TERM"] != "xterm-256color" {
		t.Errorf("host env TERM = %q, want xterm-256color", m["TERM"])
	}
	if m["ZHIPU_API_KEY"] != "z-secret" {
		t.Errorf("host env ZHIPU_API_KEY = %q, want z-secret", m["ZHIPU_API_KEY"])
	}
}

// TestBuildHostClaudeKeepsTmux — the Claude Code host env is byte-identical
// to the host environment: TMUX/TMUX_PANE stay (its copy path is mostly
// native mouse selection and must not be regressed).
func TestBuildHostClaudeKeepsTmux(t *testing.T) {
	_, _ = docProject(t)
	fakeHostBinary(t)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	t.Setenv("TMUX_PANE", "%5")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildHostInvocation(Options{}, info, r, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	m := envMap(t, inv.Env)
	if m["TMUX"] != "/tmp/tmux-1000/default,1234,0" {
		t.Errorf("claude host env must keep TMUX (got %q)", m["TMUX"])
	}
	if m["TMUX_PANE"] != "%5" {
		t.Errorf("claude host env must keep TMUX_PANE (got %q)", m["TMUX_PANE"])
	}
}

func TestBuildHostOpenCodeZenProfile(t *testing.T) {
	_, _ = docProject(t)
	fakeHostBinary(t)
	checkout(t, "MANIGOT_PROFILE=opencode-zen\nOPENCODE_API_KEY=zen-secret\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildHostInvocation(Options{}, info, r, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	if inv.Argv[0] != "opencode" {
		t.Errorf("argv[0] = %q, want opencode", inv.Argv[0])
	}
	// The Zen profile's plan model is forwarded via opencode's own --model flag.
	containsAll(t, inv.Argv, "--model", "opencode/deepseek-v4-flash-free")
	m := envMap(t, inv.Env)
	if m["OPENCODE_API_KEY"] != "zen-secret" {
		t.Errorf("host env OPENCODE_API_KEY = %q, want zen-secret", m["OPENCODE_API_KEY"])
	}
	if m["OPENCODE_MODEL"] != "" {
		t.Errorf("host env must not forward OPENCODE_MODEL — the model goes via --model (got %q)", m["OPENCODE_MODEL"])
	}
	joined := strings.Join(inv.Argv, "\n")
	for _, forbidden := range []string{"--auto", "-e", "docker"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("opencode host argv must not carry %q:\n%s", forbidden, joined)
		}
	}
	// An opencode profile must not forward the claude-pro subscription keys.
	for _, k := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"} {
		if m[k] != "" {
			t.Errorf("opencode host env forwards %s=%q, want only the profile's own keys", k, m[k])
		}
	}
}

func TestBuildHostPrintRejected(t *testing.T) {
	_, _ = docProject(t)
	fakeHostBinary(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	_, err = BuildHostInvocation(Options{Print: true}, info, r, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "--print is not supported with mg host") {
		t.Errorf("print rejection = %v", err)
	}
}

func TestBuildHostMissingBinary(t *testing.T) {
	_, _ = docProject(t)
	old := hostLookPath
	hostLookPath = func(name string) (string, error) { return "", errors.New("exec: not found") }
	t.Cleanup(func() { hostLookPath = old })
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	_, err = BuildHostInvocation(Options{}, info, r, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "not installed on the host") {
		t.Errorf("missing-binary error = %v", err)
	}
}

func TestBuildHostJobPromptHostPath(t *testing.T) {
	root, jobName, wtPath := worktreeProject(t)
	fakeHostBinary(t)
	t.Chdir(root)
	checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{Job: jobName})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildHostInvocation(Options{Job: jobName}, info, r, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	// The CLI runs from the job's worktree, and the prompt names the job's
	// host path (not the container's /workspace path).
	if inv.Dir != filepath.Clean(wtPath) {
		t.Errorf("Dir = %q, want the job worktree %q", inv.Dir, wtPath)
	}
	containsAll(t, inv.Argv, "Please work on the job at "+filepath.Join(filepath.Clean(wtPath), "docs", "jobs", jobName)+" — start by reading brief.md")
	if strings.Contains(strings.Join(inv.Argv, "\n"), "/workspace/") {
		t.Errorf("host prompt must not use the container's /workspace path:\n%s", strings.Join(inv.Argv, "\n"))
	}
}

func TestHostInvocationRun(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "fake-claude")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho \"cwd=$PWD\"\necho \"args=$*\"\necho \"token=$CLAUDE_CODE_OAUTH_TOKEN\"\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	inv := HostInvocation{
		Argv: []string{stub, "hello world"},
		Dir:  dir,
		Env:  append(os.Environ(), "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-token"),
	}
	var out strings.Builder
	code := inv.Run(os.Stdin, &out, &strings.Builder{})
	if code != 3 {
		t.Errorf("Run exit code = %d, want 3 (the stub's own exit code)", code)
	}
	if !strings.Contains(out.String(), "cwd="+dir) {
		t.Errorf("Run did not run in Dir %q:\n%s", dir, out.String())
	}
	if !strings.Contains(out.String(), "args=hello world") {
		t.Errorf("Run did not pass the argv through:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "token=sk-ant-oat01-token") {
		t.Errorf("Run did not apply Env:\n%s", out.String())
	}
}

// TestInstallHostGlobalAgents — mg host runs the CLI directly with no
// mounts, so the manigot checkout's global agents are surfaced by symlinking
// them into the host CLI's global agents dir, without clobbering an existing
// host agent of the same name.
func TestInstallHostGlobalAgents(t *testing.T) {
	home := checkout(t, "")
	writeAgent(t, home, "analyst.md", "---\nname: analyst\ndescription: Analyst.\n---\n\nBody.\n")
	writeAgent(t, home, "reviewer.md", "---\nname: reviewer\ndescription: Reviewer.\n---\n\nBody.\n")

	// Point $HOME at a temp dir so symlinks land there, not the real home.
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	n, err := installHostGlobalAgents(config.ToolClaudeCode)
	if err != nil {
		t.Fatalf("installHostGlobalAgents: %v", err)
	}
	if n != 2 {
		t.Errorf("installed = %d, want 2", n)
	}

	// Claude links into ~/.claude/agents/.
	target := filepath.Join(hostHome, ".claude", "agents")
	for _, name := range []string{"analyst.md", "reviewer.md"} {
		link := filepath.Join(target, name)
		dest, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("expected symlink %s: %v", link, err)
		}
		if dest != filepath.Join(home, "agents", name) {
			t.Errorf("symlink %s -> %q, want %q", link, dest, filepath.Join(home, "agents", name))
		}
	}
}

// TestInstallHostGlobalAgentsOpenCodeTarget — OpenCode cannot load the raw
// list-form agents (it hard-errors on the tools: key), so the host target
// gets a CONVERTED COPY (name:/tools: stripped, permission: passed through)
// written into ~/.config/opencode/agents/ — not a symlink to the raw file —
// and an existing host agent is never clobbered.
func TestInstallHostGlobalAgentsOpenCodeTarget(t *testing.T) {
	home := checkout(t, "")
	writeAgent(t, home, "analyst.md", "---\nname: analyst\ndescription: Analyst.\ntools: Read, Grep, Glob\n---\n\nBody.\n")
	writeAgent(t, home, "reviewer.md", "---\nname: reviewer\ndescription: Reviewer.\ntools: Read, Grep, Glob\npermission: bash\n---\n\nBody.\n")

	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	// A user's own host agent of the same name — must survive untouched.
	existing := filepath.Join(hostHome, ".config", "opencode", "agents", "analyst.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	userContent := "---\ndescription: user's own.\n---\n"
	if err := os.WriteFile(existing, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := installHostGlobalAgents(config.ToolOpenCode)
	if err != nil {
		t.Fatalf("installHostGlobalAgents: %v", err)
	}
	// Only reviewer.md is installed — analyst.md exists already and is preserved.
	if n != 1 {
		t.Errorf("installed = %d, want 1 (analyst already exists)", n)
	}
	if data, err := os.ReadFile(existing); err != nil || string(data) != userContent {
		t.Errorf("existing host agent was modified: %q, %v", string(data), err)
	}
	// The installed reviewer.md is a REGULAR FILE (not a symlink) containing
	// the CONVERTED content: name:/tools: stripped, permission: passed through.
	installed := filepath.Join(hostHome, ".config", "opencode", "agents", "reviewer.md")
	fi, err := os.Lstat(installed)
	if err != nil {
		t.Fatalf("reviewer.md was not installed: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("opencode target must be a regular converted file, not a symlink to the raw agent")
	}
	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed reviewer.md: %v", err)
	}
	converted := string(data)
	if strings.Contains(converted, "name:") || strings.Contains(converted, "tools:") {
		t.Errorf("installed opencode agent still carries the raw list-form keys:\n%s", converted)
	}
	if !strings.Contains(converted, "permission: bash") {
		t.Errorf("installed opencode agent lost the permission: block:\n%s", converted)
	}
	if !strings.Contains(converted, "Body.") {
		t.Errorf("installed opencode agent lost the body:\n%s", converted)
	}
}

// TestInstallHostGlobalAgentsOpenCodeReplacesStaleRawSymlink — a symlink the
// pre-fix installer placed (pointing at the checkout's agents/<name>) would
// still hard-error OpenCode; the installer must replace its own stale raw
// link with a converted copy, while leaving a foreign symlink alone.
func TestInstallHostGlobalAgentsOpenCodeReplacesStaleRawSymlink(t *testing.T) {
	home := checkout(t, "")
	writeAgent(t, home, "reviewer.md", "---\nname: reviewer\ndescription: Reviewer.\ntools: Read, Grep, Glob\npermission: bash\n---\n\nBody.\n")
	writeAgent(t, home, "security.md", "---\nname: security\ndescription: Security.\ntools: Read\n---\n\nBody.\n")

	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	targetDir := filepath.Join(hostHome, ".config", "opencode", "agents")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// The pre-fix state: a raw symlink pointing at the checkout's agents dir.
	stale := filepath.Join(targetDir, "reviewer.md")
	if err := os.Symlink(filepath.Join(home, "agents", "reviewer.md"), stale); err != nil {
		t.Fatal(err)
	}
	// A foreign symlink (the user's own link elsewhere) — must survive.
	foreign := filepath.Join(targetDir, "security.md")
	if err := os.Symlink("/some/other/place/security.md", foreign); err != nil {
		t.Fatal(err)
	}

	n, err := installHostGlobalAgents(config.ToolOpenCode)
	if err != nil {
		t.Fatalf("installHostGlobalAgents: %v", err)
	}
	// Only the stale raw link is replaced; the foreign symlink is skipped.
	if n != 1 {
		t.Errorf("installed = %d, want 1 (only the stale raw link replaced)", n)
	}
	if dest, err := os.Readlink(foreign); err != nil || dest != "/some/other/place/security.md" {
		t.Errorf("foreign symlink was touched: %q, %v", dest, err)
	}
	// The stale symlink is gone; a regular converted file sits in its place.
	fi, err := os.Lstat(stale)
	if err != nil {
		t.Fatalf("target missing after replace: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("stale raw symlink was not replaced with a regular converted file")
	}
	data, err := os.ReadFile(stale)
	if err != nil {
		t.Fatalf("read replaced target: %v", err)
	}
	converted := string(data)
	if strings.Contains(converted, "tools:") || strings.Contains(converted, "name:") {
		t.Errorf("replaced target still carries the raw list-form keys:\n%s", converted)
	}
	if !strings.Contains(converted, "permission: bash") {
		t.Errorf("replaced target lost the permission: block:\n%s", converted)
	}
}

// TestInstallHostGlobalAgentsNoAgents — with no agents/ dir in the checkout,
// nothing is installed and no host config dir is created (no side effects on
// the user's home).
func TestInstallHostGlobalAgentsNoAgents(t *testing.T) {
	checkout(t, "")
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	if n, err := installHostGlobalAgents(config.ToolClaudeCode); err != nil || n != 0 {
		t.Errorf("no-agents install = (%d, %v), want (0, nil)", n, err)
	}
	if _, err := os.Stat(filepath.Join(hostHome, ".claude", "agents")); !os.IsNotExist(err) {
		t.Errorf("no-agents install created a host config dir: %v", err)
	}
}

// TestBuildHostLinksGlobalAgents — BuildHostInvocation surfaces the checkout's
// global agents into the host CLI's config dir and reports it on diag.
func TestBuildHostLinksGlobalAgents(t *testing.T) {
	_, _ = docProject(t)
	// Re-point the home at a checkout that has agents, and $HOME at a temp dir
	// so the symlinks land somewhere isolated.
	home := checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-token\nCLAUDE_ACCOUNT_UUID=uuid-1\nCLAUDE_EMAIL=me@x.io\nCLAUDE_ORG_UUID=org-1\n")
	writeAgent(t, home, "analyst.md", "---\nname: analyst\ndescription: Analyst.\n---\n\nBody.\n")
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	fakeHostBinary(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	var diag strings.Builder
	if _, err := BuildHostInvocation(Options{}, info, r, &diag); err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	if _, err := os.Readlink(filepath.Join(hostHome, ".claude", "agents", "analyst.md")); err != nil {
		t.Errorf("host invocation did not link the global agent: %v", err)
	}
	if !strings.Contains(diag.String(), "Installed : 1 global agent(s)") {
		t.Errorf("missing Installed diag line:\n%s", diag.String())
	}
}

// TestBuildHostOpenCodeWritesConvertedGlobalAgents — under an opencode
// profile, BuildHostInvocation delivers CONVERTED copies (not raw symlinks)
// into ~/.config/opencode/agents/, so the host CLI can actually load the
// global agents.
func TestBuildHostOpenCodeWritesConvertedGlobalAgents(t *testing.T) {
	_, _ = docProject(t)
	home := checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=z-secret\n")
	writeAgent(t, home, "reviewer.md", "---\nname: reviewer\ndescription: Reviewer.\ntools: Read, Grep, Glob\npermission: bash\n---\n\nBody.\n")
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	fakeHostBinary(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if err := info.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	var diag strings.Builder
	if _, err := BuildHostInvocation(Options{}, info, r, &diag); err != nil {
		t.Fatalf("BuildHostInvocation: %v", err)
	}
	installed := filepath.Join(hostHome, ".config", "opencode", "agents", "reviewer.md")
	fi, err := os.Lstat(installed)
	if err != nil {
		t.Fatalf("host invocation did not install the converted agent: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("opencode host target must be a regular converted file, not a raw symlink")
	}
	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed agent: %v", err)
	}
	converted := string(data)
	if strings.Contains(converted, "tools:") || strings.Contains(converted, "name:") {
		t.Errorf("installed opencode agent carries the raw list-form keys:\n%s", converted)
	}
	if !strings.Contains(converted, "permission: bash") {
		t.Errorf("installed opencode agent lost the permission: block:\n%s", converted)
	}
	if !strings.Contains(diag.String(), "Installed : 1 global agent(s)") {
		t.Errorf("missing Installed diag line:\n%s", diag.String())
	}
}
