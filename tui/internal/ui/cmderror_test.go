package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/resolve"
)

func TestCmdErrorTextResolutionFailure(t *testing.T) {
	err := &resolve.NotFoundError{
		Spec:  resolve.Job(),
		Tried: []string{"$MANIGOT_JOB_BIN (unset)", "mg-job on $PATH"},
	}
	got := cmdErrorText(err)

	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("want a 3-line diagnosis, got %d line(s):\n%s", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "error: ") {
		t.Errorf("first line should start with 'error: '; got %q", lines[0])
	}
	// The strategies tried must be listed, in order.
	if !strings.Contains(lines[1], "$MANIGOT_JOB_BIN (unset)") ||
		!strings.Contains(lines[1], "mg-job on $PATH") {
		t.Errorf("tried list incomplete: %q", lines[1])
	}
	// And the fix line must name the env var to set.
	if !strings.Contains(lines[2], resolve.EnvJob) {
		t.Errorf("fix line should name $%s; got %q", resolve.EnvJob, lines[2])
	}
}

func TestCmdErrorTextPlainError(t *testing.T) {
	got := cmdErrorText(errors.New("boom"))
	if got != "error: boom" {
		t.Errorf("cmdErrorText = %q, want %q", got, "error: boom")
	}
	if got := cmdErrorText(nil); got != "" {
		t.Errorf("cmdErrorText(nil) = %q, want empty", got)
	}
}
