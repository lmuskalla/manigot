package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
