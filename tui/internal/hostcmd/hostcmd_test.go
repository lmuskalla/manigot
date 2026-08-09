package hostcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate makes the test independent of what is installed on the machine: an
// empty $PATH and no resolve env vars, so no real job can be created and no
// existing install can influence the outcome.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("SAFECODE_JOB_BIN", "")
	t.Setenv("SAFECODE_HOME", "")
	return dir
}

func TestNewJobUnresolvable(t *testing.T) {
	isolate(t)

	_, err := NewJob("some title", "feature", t.TempDir())
	if err == nil {
		t.Fatal("expected an error when no job command can be resolved")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should explain the command is missing; got: %v", err)
	}
}

// The resolved command must be run by absolute path, in projectRoot, with $PWD
// matching — the script depends on all three.
func TestNewJobRunsResolvedCommand(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	out := filepath.Join(root, "args.txt")

	// A stub standing in for the real script: records how it was called.
	stub := filepath.Join(t.TempDir(), "stub.sh")
	script := "#!/bin/sh\n" +
		"{ echo \"argv0=$0\"; echo \"pwd=$PWD\"; echo \"cwd=$(pwd)\"; " +
		"for a in \"$@\"; do echo \"arg=$a\"; done; } > " + out + "\n" +
		"echo created\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SAFECODE_JOB_BIN", stub)

	got, err := NewJob("my title", "fix", root)
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	if !strings.Contains(got, "created") {
		t.Errorf("output not returned to the caller; got %q", got)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("stub did not run: %v", err)
	}
	recorded := string(raw)
	for _, want := range []string{
		"argv0=" + stub, // invoked by absolute path, not by bare name
		"pwd=" + root,   // $PWD explicitly set for find_project_root
		"cwd=" + root,   // and the real cwd agrees
		"arg=my title",
		"arg=--type",
		"arg=fix",
	} {
		if !strings.Contains(recorded, want) {
			t.Errorf("missing %q in stub record:\n%s", want, recorded)
		}
	}
}

// With no type given, no --type flag may be passed.
func TestNewJobOmitsEmptyType(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	out := filepath.Join(root, "args.txt")

	stub := filepath.Join(t.TempDir(), "stub.sh")
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"arg=$a\"; done > " + out + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SAFECODE_JOB_BIN", stub)

	if _, err := NewJob("t", "", root); err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	raw, _ := os.ReadFile(out)
	if strings.Contains(string(raw), "--type") {
		t.Errorf("--type passed for an empty job type: %s", raw)
	}
}
