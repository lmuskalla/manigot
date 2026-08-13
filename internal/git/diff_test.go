package git

import (
	"errors"
	"strings"
	"testing"
)

// Tests for the three-dot diff + log helpers (TASK-1 of the mg diff job).
// Every test builds a throwaway repo via the package's existing helpers and
// exercises real git, same as the rest of this package — the whole point of
// these helpers is to shell out to git, so stubbing it would test nothing.

// TestDiffHelpers checks the full three-dot outputs against a branch that
// diverged from the base branch with two commits.
func TestDiffHelpers(t *testing.T) {
	dir, def := initRepo(t)
	runGit(t, dir, "checkout", "-q", "-b", "feature/abc123_x")
	writeFile(t, dir, "docs/jobs/abc123_x/brief.md", "brief\n")
	commitAll(t, dir, "[abc123] TASK-1: do a thing")
	writeFile(t, dir, "docs/jobs/abc123_x/tasks.md", "tasks\n")
	commitAll(t, dir, "[abc123] TASK-2: do another thing")
	runGit(t, dir, "checkout", "-q", def)

	// Diff: the full patch carries the diff headers and both files.
	patch, err := Diff(dir, def, "feature/abc123_x")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(patch, "diff --git") {
		t.Errorf("Diff missing patch header:\n%s", patch)
	}
	if !strings.Contains(patch, "docs/jobs/abc123_x/brief.md") {
		t.Errorf("Diff missing brief.md file header:\n%s", patch)
	}
	if !strings.Contains(patch, "docs/jobs/abc123_x/tasks.md") {
		t.Errorf("Diff missing tasks.md file header:\n%s", patch)
	}

	// DiffStat: both files listed with line counts.
	stat, err := DiffStat(dir, def, "feature/abc123_x")
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}
	if !strings.Contains(stat, "docs/jobs/abc123_x/brief.md") {
		t.Errorf("DiffStat missing brief.md:\n%s", stat)
	}
	if !strings.Contains(stat, "docs/jobs/abc123_x/tasks.md") {
		t.Errorf("DiffStat missing tasks.md:\n%s", stat)
	}

	// DiffNameOnly: one filename per line, nothing else.
	names, err := DiffNameOnly(dir, def, "feature/abc123_x")
	if err != nil {
		t.Fatalf("DiffNameOnly: %v", err)
	}
	lines := strings.Split(names, "\n")
	if len(lines) != 2 {
		t.Errorf("DiffNameOnly = %d lines, want 2:\n%s", len(lines), names)
	}
	if !strings.Contains(names, "docs/jobs/abc123_x/brief.md") {
		t.Errorf("DiffNameOnly missing brief.md:\n%s", names)
	}
	if !strings.Contains(names, "docs/jobs/abc123_x/tasks.md") {
		t.Errorf("DiffNameOnly missing tasks.md:\n%s", names)
	}

	// LogOneline: the branch's own two commits, most recent first.
	logs, err := LogOneline(dir, def, "feature/abc123_x")
	if err != nil {
		t.Fatalf("LogOneline: %v", err)
	}
	if !strings.Contains(logs, "[abc123] TASK-2: do another thing") {
		t.Errorf("LogOneline missing TASK-2 commit:\n%s", logs)
	}
	if !strings.Contains(logs, "[abc123] TASK-1: do a thing") {
		t.Errorf("LogOneline missing TASK-1 commit:\n%s", logs)
	}
	// The base branch's own commit is not part of the range.
	if strings.Contains(logs, "init") {
		t.Errorf("LogOneline included the base branch's commit:\n%s", logs)
	}
}

// TestDiffHelpersUndiverged pins the "nothing to show" degrade: a range where
// branch and base point at the same commit is an empty result, not an error.
func TestDiffHelpersUndiverged(t *testing.T) {
	dir, def := initRepo(t)

	for name, fn := range map[string]func() (string, error){
		"Diff":         func() (string, error) { return Diff(dir, def, def) },
		"DiffStat":     func() (string, error) { return DiffStat(dir, def, def) },
		"DiffNameOnly": func() (string, error) { return DiffNameOnly(dir, def, def) },
		"LogOneline":   func() (string, error) { return LogOneline(dir, def, def) },
	} {
		t.Run(name, func(t *testing.T) {
			out, err := fn()
			if err != nil {
				t.Fatalf("%s on an undiverged range: %v", name, err)
			}
			if out != "" {
				t.Errorf("%s on an undiverged range = %q, want empty", name, out)
			}
		})
	}
}

// TestDiffHelpersMissingBranch pins the real-git-failure wrap: a range naming
// a branch that doesn't exist is a wrapped git error (with git's stderr), not
// a silent empty result and not ErrNotARepo.
func TestDiffHelpersMissingBranch(t *testing.T) {
	dir, def := initRepo(t)

	for name, fn := range map[string]func() (string, error){
		"Diff":         func() (string, error) { return Diff(dir, def, "no-such-branch") },
		"DiffStat":     func() (string, error) { return DiffStat(dir, def, "no-such-branch") },
		"DiffNameOnly": func() (string, error) { return DiffNameOnly(dir, def, "no-such-branch") },
		"LogOneline":   func() (string, error) { return LogOneline(dir, def, "no-such-branch") },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := fn()
			if err == nil {
				t.Fatalf("%s on a missing branch: expected an error, got nil", name)
			}
			if errors.Is(err, ErrNotARepo) {
				t.Errorf("%s on a missing branch misclassified as ErrNotARepo: %v", name, err)
			}
		})
	}
}

// TestDiffHelpersNotARepo pins the package's not-a-repo degrade for every
// helper.
func TestDiffHelpersNotARepo(t *testing.T) {
	for name, fn := range map[string]func() (string, error){
		"Diff":         func() (string, error) { return Diff(t.TempDir(), "main", "feature/x") },
		"DiffStat":     func() (string, error) { return DiffStat(t.TempDir(), "main", "feature/x") },
		"DiffNameOnly": func() (string, error) { return DiffNameOnly(t.TempDir(), "main", "feature/x") },
		"LogOneline":   func() (string, error) { return LogOneline(t.TempDir(), "main", "feature/x") },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := fn()
			if !errors.Is(err, ErrNotARepo) {
				t.Errorf("%s on non-repo: err = %v, want ErrNotARepo", name, err)
			}
		})
	}
}
