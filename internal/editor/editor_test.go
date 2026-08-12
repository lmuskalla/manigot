package editor

import (
	"errors"
	"os/exec"
	"testing"
)

// stubLookPath replaces LookPath for the duration of a test and returns a
// func to restore it, so tests never depend on the host's actually
// installed editors.
func stubLookPath(t *testing.T, found map[string]bool) {
	t.Helper()
	orig := LookPath
	LookPath = func(name string) (string, error) {
		if found[name] {
			return "/usr/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { LookPath = orig })
}

func TestResolveVisualTakesPriority(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")
	t.Setenv("EDITOR", "nano")
	stubLookPath(t, map[string]bool{"nano": true, "vi": true})

	got, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "code --wait" {
		t.Errorf("got %q, want %q", got, "code --wait")
	}
}

func TestResolveOverrideTakesPriorityOverVisual(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")
	t.Setenv("EDITOR", "nano")
	stubLookPath(t, map[string]bool{"nano": true, "vi": true})

	got, err := Resolve("vim")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "vim" {
		t.Errorf("got %q, want %q", got, "vim")
	}
}

func TestResolveEditorWhenNoVisual(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "vim")
	stubLookPath(t, map[string]bool{"nano": true, "vi": true})

	got, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "vim" {
		t.Errorf("got %q, want %q", got, "vim")
	}
}

func TestResolveFallsBackToNano(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	stubLookPath(t, map[string]bool{"nano": true, "vi": true})

	got, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "nano" {
		t.Errorf("got %q, want %q", got, "nano")
	}
}

func TestResolveFallsBackToViWhenNanoMissing(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	stubLookPath(t, map[string]bool{"vi": true})

	got, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "vi" {
		t.Errorf("got %q, want %q", got, "vi")
	}
}

func TestResolveErrorsWhenNothingFound(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	stubLookPath(t, map[string]bool{})

	_, err := Resolve("")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got err=%v, want ErrNotFound", err)
	}
}

func TestCommandBuildsShellWrapper(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "myeditor --flag")

	cmd, err := Command("/tmp/some file.md", "")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	want := []string{"sh", "-c", `myeditor --flag "$1"`, "--", "/tmp/some file.md"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], want[i])
		}
	}
}

func TestCommandUsesOverrideEvenWithVisualSet(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")

	cmd, err := Command("/tmp/x.md", "vim")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	want := []string{"sh", "-c", `vim "$1"`, "--", "/tmp/x.md"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], want[i])
		}
	}
}

func TestCommandPropagatesResolveError(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	stubLookPath(t, map[string]bool{})

	_, err := Command("/tmp/x.md", "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got err=%v, want ErrNotFound", err)
	}
}
