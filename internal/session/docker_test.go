package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docProject builds a docs-initialized scratch project (with an AGENTS.md) and
// a session checkout with full claude-pro credentials, then chdirs into the
// project so ResolveRoot resolves it.
func docProject(t *testing.T) (root, home string) {
	t.Helper()
	root = projectCheckout(t, t.TempDir(), true)
	if err := os.WriteFile(filepath.Join(root, "docs", "AGENTS.md"), []byte("# ctx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home = checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-token\nCLAUDE_ACCOUNT_UUID=uuid-1\nCLAUDE_EMAIL=me@x.io\nCLAUDE_ORG_UUID=org-1\n")
	t.Chdir(root)
	return root, home
}

func containsAll(t *testing.T, argv []string, wants ...string) {
	t.Helper()
	joined := strings.Join(argv, "\n")
	for _, w := range wants {
		if !strings.Contains(joined, w) {
			t.Errorf("argv missing %q\nargv:\n%s", w, joined)
		}
	}
}

func TestBuildPlainClaudeSession(t *testing.T) {
	root, _ := docProject(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	var diag strings.Builder
	inv, err := BuildDockerInvocation(Options{}, info, r, false, &diag)
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv,
		"docker", "run", "--rm",
		"--name", "manigot-"+filepath.Base(root)+"-",
		"--user",
		"-v", root+":/workspace:z",
		"-v", filepath.Join(root, "docs")+":/workspace/.claude:z",
		"-v", filepath.Join(root, "docs", "AGENTS.md")+":/workspace/.claude/CLAUDE.md:ro",
		"-e", "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-token",
		"-e", "CLAUDE_ACCOUNT_UUID=uuid-1",
		"-e", "CLAUDE_EMAIL=me@x.io",
		"-e", "CLAUDE_ORG_UUID=org-1",
		"-e", "GIT_AUTHOR_NAME_CFG=Test",
		"-e", "GIT_AUTHOR_EMAIL_CFG=test@x.io",
		"-e", "MANIGOT_TOOL=claude-code",
		"-e", "MANIGOT_PRINT=false",
		"-e", "MANIGOT_QUOTE=",
		"--network=bridge", "--memory=2g", "--security-opt=no-new-privileges",
		"manigot",
	)
	// Non-interactive stdin → no -it.
	if strings.Contains(strings.Join(inv.Argv, "\n"), "-it") {
		t.Errorf("non-interactive build must not pass -it:\n%s", strings.Join(inv.Argv, "\n"))
	}
	// Banner diagnostics went to the diag writer, not stdout.
	if !strings.Contains(diag.String(), "║           manigot") {
		t.Errorf("missing banner in diag:\n%s", diag.String())
	}
	if strings.Contains(diag.String(), "Shadowed : none") == false {
		t.Errorf("missing shadowed-none line:\n%s", diag.String())
	}
}

func TestBuildInteractiveTTY(t *testing.T) {
	_, _ = docProject(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildDockerInvocation(Options{}, info, r, true, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv, "-it")
}

func TestBuildPrintMode(t *testing.T) {
	_, _ = docProject(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildDockerInvocation(Options{Print: true}, info, r, true, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	// --print drops -it even on a TTY, and sets MANIGOT_PRINT=true.
	if strings.Contains(strings.Join(inv.Argv, "\n"), "-it") {
		t.Errorf("print build must not pass -it")
	}
	containsAll(t, inv.Argv, "-e", "MANIGOT_PRINT=true")
}

func TestBuildOpenCodeProfile(t *testing.T) {
	root, _ := docProject(t)
	home := checkout(t, "MANIGOT_PROFILE=zai\nZHIPU_API_KEY=z-secret\n")
	_ = home
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
	inv, err := BuildDockerInvocation(Options{}, info, r, false, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv,
		"-e", "ZHIPU_API_KEY=z-secret",
		"-e", "OPENCODE_MODEL=zai-coding-plan/glm-5.2",
		"-e", "MANIGOT_TOOL=opencode",
		"-v", filepath.Join(root, "docs")+":/workspace/.opencode:z",
		"-v", filepath.Join(root, "docs", "AGENTS.md")+":/workspace/AGENTS.md:ro",
	)
}

func TestBuildJobWorktreeGitCommonDirMount(t *testing.T) {
	_, home := docProject(t)
	_ = home
	// Rebuild the checkout env for this test (docProject already did, but the
	// worktree project needs a fresh cwd context).
	root, jobName, wtPath := worktreeProject(t)
	t.Chdir(root)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{Job: jobName})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	// The worktree's gitdir lives in the main repo's .git/worktrees/ — the
	// common dir is that .git path.
	inv, err := BuildDockerInvocation(Options{Job: jobName}, info, r, false, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv,
		"-v", filepath.Clean(wtPath)+":/workspace:z",
		"-v", r.GitCommonDir+":"+r.GitCommonDir+":z",
	)
}

func TestBuildJobPromptAndPrintMarker(t *testing.T) {
	root, jobName, _ := worktreeProject(t)
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

	// Interactive: the job prompt without the marker.
	inv, err := BuildDockerInvocation(Options{Job: jobName}, info, r, false, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv, "Please work on the job at /workspace/docs/jobs/"+jobName+" — start by reading brief.md")
	if strings.Contains(strings.Join(inv.Argv, "\n"), "NEEDS-HUMAN-INPUT") {
		t.Errorf("interactive session must not carry the NEEDS-HUMAN-INPUT marker")
	}

	// --print: the marker sentence is appended.
	inv, err = BuildDockerInvocation(Options{Job: jobName, Print: true}, info, r, false, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	if !strings.Contains(strings.Join(inv.Argv, "\n"), "NEEDS-HUMAN-INPUT:") {
		t.Errorf("print session must carry the NEEDS-HUMAN-INPUT marker")
	}
}

func TestBuildPromptWinsOverJobAndAgentFlag(t *testing.T) {
	_, _ = docProject(t)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	inv, err := BuildDockerInvocation(Options{Agent: "analyst", Prompt: "hello there"}, info, r, false, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv, "--agent", "analyst", "hello there")
}

func TestEnvShadowMounts(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".env", ".env.local", ".env.example", ".env.sample", "other.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mounts := findEnvFiles(root)
	if len(mounts) != 2 {
		t.Fatalf("findEnvFiles = %v, want only .env and .env.local", mounts)
	}
	// Also verify the shadow arg shape via a full build.
	home := checkout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")
	_ = home
	t.Chdir(root)
	info, err := ResolveProfile(Options{})
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	r, err := ResolveRoot(Options{})
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	var diag strings.Builder
	inv, err := BuildDockerInvocation(Options{}, info, r, false, &diag)
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv,
		"--mount", "type=bind,source=/dev/null,target=/workspace/.env,readonly",
		"--mount", "type=bind,source=/dev/null,target=/workspace/.env.local,readonly",
	)
	if strings.Contains(strings.Join(inv.Argv, "\n"), ".env.example") {
		t.Errorf("*.example must not be shadowed")
	}
	if !strings.Contains(diag.String(), "  Shadowing: ") {
		t.Errorf("missing shadowing diagnostics:\n%s", diag.String())
	}
}

func TestPickQuoteFromJSON(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "assets", "quotes.json"), []byte("[\"first\", \"second\", \"third\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", home)
	q := pickQuote()
	if q != "first" && q != "second" && q != "third" {
		t.Errorf("pickQuote = %q, want one of the file's quotes", q)
	}
}

func TestPickQuoteMissingFile(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", home)
	if q := pickQuote(); q != "" {
		t.Errorf("pickQuote on missing file = %q, want empty", q)
	}
}

func TestBuildOpenCodeModelForwardedAsIsLegacy(t *testing.T) {
	root := projectCheckout(t, t.TempDir(), true)
	t.Chdir(root)
	checkout(t, "OPENAI_API_KEY=sk-oa\nOPENCODE_MODEL=custom/model\n")
	info, err := ResolveProfile(Options{Tool: "opencode"})
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
	inv, err := BuildDockerInvocation(Options{}, info, r, false, &strings.Builder{})
	if err != nil {
		t.Fatalf("BuildDockerInvocation: %v", err)
	}
	containsAll(t, inv.Argv, "-e", "OPENCODE_MODEL=custom/model", "-e", "MANIGOT_TOOL=opencode")
}
