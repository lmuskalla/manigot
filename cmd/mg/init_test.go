package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initCheckout builds a fake manigot checkout with the project-template files
// init reads from, and points $MANIGOT_HOME at it.
func initCheckout(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tpl := filepath.Join(home, "project-template")
	for _, f := range []string{"docs/AGENTS.md", "docs/CLAUDE.md", ".manigot/manigot.json"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tpl, f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tpl, f), []byte("# "+f+" template\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The example job must NOT be copied by init.
	if err := os.MkdirAll(filepath.Join(tpl, "docs", "jobs", "abc123_example"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "docs", "jobs", "abc123_example", "brief.md"), []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", home)
	return home
}

func TestInitCreatesScaffold(t *testing.T) {
	initCheckout(t)
	proj := t.TempDir()
	t.Chdir(proj)
	var out strings.Builder
	code := runInit(nil, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	// The three scaffold pieces plus the empty jobs/ dir.
	for _, want := range []string{
		proj + "/docs/AGENTS.md",
		proj + "/docs/CLAUDE.md",
		proj + "/docs/jobs/ (empty)",
		proj + "/.manigot/manigot.json (baseBranch: main)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(proj, "docs", "jobs", "abc123_example")); err == nil {
		t.Error("the example job must not be copied")
	}
	if _, err := os.Stat(filepath.Join(proj, "docs", "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj, ".manigot", "manigot.json")); err != nil {
		t.Errorf("manigot.json not created: %v", err)
	}
	// The next-steps block.
	if !strings.Contains(out.String(), "✓ Project initialized for manigot.") {
		t.Errorf("missing next-steps:\n%s", out.String())
	}
}

func TestInitSkipsWhenAlreadyInitialized(t *testing.T) {
	initCheckout(t)
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "docs", "existing.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(proj)
	var out strings.Builder
	code := runInit(nil, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), proj+"/docs already exists — skipping template copy.") {
		t.Errorf("missing already-initialized line:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(proj, "docs", "AGENTS.md")); err == nil {
		t.Error("existing docs/ must not be overwritten")
	}
}

func TestInitNonTTYSkipsPrompter(t *testing.T) {
	initCheckout(t)
	proj := t.TempDir()
	t.Chdir(proj)
	var out strings.Builder
	code := runInit(nil, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "  Skipping @prompter offer (stdin is not a terminal).") {
		t.Errorf("missing skip note:\n%s", out.String())
	}
}

func TestInitUnknownArgument(t *testing.T) {
	initCheckout(t)
	var errOut strings.Builder
	code := runInit([]string{"--bogus"}, strings.NewReader(""), &strings.Builder{}, &errOut, false)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Unknown argument: --bogus") {
		t.Errorf("missing unknown-arg error:\n%s", errOut.String())
	}
}

func TestInitProfileValidation(t *testing.T) {
	initCheckout(t)
	for _, tc := range []struct {
		args    []string
		wantErr string
	}{
		{[]string{"--profile", "bogus"}, "Error: --profile must be 'claude-pro', 'zai' or 'opencode-go' (got 'bogus')."},
		{[]string{"--tool", "bogus"}, "Error: --tool must be 'claude-code' or 'opencode' (got 'bogus')."},
	} {
		var errOut strings.Builder
		code := runInit(tc.args, strings.NewReader(""), &strings.Builder{}, &errOut, false)
		if code != 1 {
			t.Errorf("%v: exit code = %d, want 1", tc.args, code)
		}
		if !strings.Contains(errOut.String(), tc.wantErr) {
			t.Errorf("%v: missing error %q:\n%s", tc.args, tc.wantErr, errOut.String())
		}
	}
}

func TestInitLegacyToolMapping(t *testing.T) {
	initCheckout(t)
	proj := t.TempDir()
	t.Chdir(proj)
	var out strings.Builder
	// --tool opencode maps to the zai profile; on a non-TTY the prompter is
	// skipped, so only the scaffold output is visible.
	code := runInit([]string{"--tool", "opencode"}, strings.NewReader(""), &out, &strings.Builder{}, false)
	if code != 0 {
		t.Fatalf("exit code = %d, output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "✓ Project initialized") {
		t.Errorf("missing success output:\n%s", out.String())
	}
}

func TestInitMissingTemplate(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_HOME", home)
	var errOut strings.Builder
	code := runInit(nil, strings.NewReader(""), &strings.Builder{}, &errOut, false)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: project template not found at") {
		t.Errorf("missing template error:\n%s", errOut.String())
	}
}
