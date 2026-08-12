package ui

import (
	"errors"
	"testing"
)

// TestCmdErrorTextPlainError verifies every error (they all come from the
// in-process Go lifecycle/session now, never from command resolution) renders
// as a one-liner.
func TestCmdErrorTextPlainError(t *testing.T) {
	got := cmdErrorText(errors.New("boom"))
	if got != "error: boom" {
		t.Errorf("cmdErrorText = %q, want %q", got, "error: boom")
	}
	if got := cmdErrorText(nil); got != "" {
		t.Errorf("cmdErrorText(nil) = %q, want empty", got)
	}
}
