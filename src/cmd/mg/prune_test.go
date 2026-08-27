package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/session"
)

// TestRunPruneUnknownArg pins the no-arguments contract: any argument —
// including --help, mirroring mg job/mg diff — is an unknown argument.
func TestRunPruneUnknownArg(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--help"}, "Unknown argument: --help\n"},
		{[]string{"--bogus"}, "Unknown argument: --bogus\n"},
		{[]string{"stray"}, "Unknown argument: stray\n"},
	} {
		t.Run(tc.args[0], func(t *testing.T) {
			var out, errOut strings.Builder
			if code := runPrune(tc.args, &out, &errOut); code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if errOut.String() != tc.want {
				t.Errorf("stderr = %q, want exactly %q", errOut.String(), tc.want)
			}
		})
	}
}

// TestRunPruneRemovesAndReports: the removed count and the untouched running
// count are both printed, exit 0.
func TestRunPruneRemovesAndReports(t *testing.T) {
	old := pruneOrphans
	pruneOrphans = func(diag io.Writer) (session.PruneResult, error) {
		return session.PruneResult{Removed: 3, Running: 1}, nil
	}
	t.Cleanup(func() { pruneOrphans = old })

	var out, errOut strings.Builder
	if code := runPrune(nil, &out, &errOut); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	want := "Removed 3 orphaned manigot container(s).\n1 running manigot container(s) left untouched.\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

// TestRunPruneNothingToPrune: the "nothing to prune" line replaces the removed
// count when there is nothing to remove; the running count is still reported.
func TestRunPruneNothingToPrune(t *testing.T) {
	old := pruneOrphans
	pruneOrphans = func(diag io.Writer) (session.PruneResult, error) {
		return session.PruneResult{Removed: 0, Running: 0}, nil
	}
	t.Cleanup(func() { pruneOrphans = old })

	var out, errOut strings.Builder
	if code := runPrune(nil, &out, &errOut); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	want := "Nothing to prune.\n0 running manigot container(s) left untouched.\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
}

// TestRunPruneDockerUnavailableExitsOne: docker missing or the daemon down is
// a clear error and exit 1 — unlike the launch path, where the same failure
// only warns.
func TestRunPruneDockerUnavailableExitsOne(t *testing.T) {
	old := pruneOrphans
	pruneOrphans = func(diag io.Writer) (session.PruneResult, error) {
		return session.PruneResult{}, errors.New("docker ps: exec: \"docker\": executable file not found in $PATH")
	}
	t.Cleanup(func() { pruneOrphans = old })

	var out, errOut strings.Builder
	if code := runPrune(nil, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: mg prune: docker ps:") {
		t.Errorf("missing the clear docker-unavailable error:\n%s", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty on error", out.String())
	}
}
