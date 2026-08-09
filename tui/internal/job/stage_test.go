package job

import (
	"os"
	"path/filepath"
	"testing"
)

// The exact scaffold templates new-job.sh writes, verbatim. FileIsWritten must
// classify all of these as NOT written.

const tmplBrief = "# Brief: X\n\n" +
	"status: open\n" +
	"type: feature\n" +
	"id: abc123\n" +
	"branch: feature/abc123_x\n" +
	"date: 2026-08-08\n" +
	"author: Test\n\n" +
	"## What\n\n" +
	"<!-- What needs to be done? Be specific. -->\n\n" +
	"## Why\n\n" +
	"<!-- Why does this need to exist? -->\n"

const tmplTasks = "# Tasks: X\n\n" +
	"id: abc123\n" +
	"status: open\n" +
	"analyst:\n" +
	"date:\n\n" +
	"<!-- Produced by @analyst from brief.md. -->\n\n" +
	"## Task breakdown\n\n" +
	"<!-- TASK-1: description\n" +
	"     files: list of files likely affected\n" +
	"     depends: none\n" +
	"     risk: low / medium / high — reason\n\n" +
	"TASK-2: ...\n" +
	"-->\n"

const tmplImplementation = "# Implementation: X\n\n" +
	"id: abc123\n" +
	"status: open\n" +
	"developer:\n" +
	"date:\n\n" +
	"<!-- Produced by @developer after implementation. -->\n\n" +
	"## Summary\n\n" +
	"<!-- What was implemented, task by task. -->\n\n" +
	"## Changes\n\n" +
	"<!-- List of files changed. -->\n\n" +
	"## Known issues / follow-ups\n\n" +
	"<!-- Anything out of scope but worth tracking. -->\n"

const tmplVerdict = "# Verdict: X\n\n" +
	"id: abc123\n" +
	"status: open\n" +
	"reviewer:\n" +
	"date:\n\n" +
	"<!-- Produced by @reviewer and/or @security. -->\n\n" +
	"## Review\n\n" +
	"<!-- TASK-1: PASS / FAIL -->\n\n" +
	"## Security\n\n" +
	"<!-- findings or 'none' -->\n\n" +
	"## Overall\n\n" +
	"<!-- APPROVED / REJECTED -->\n"

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name)
}

func TestScaffoldTemplatesAreNotWritten(t *testing.T) {
	for name, content := range map[string]string{
		"brief.md":          tmplBrief,
		"tasks.md":          tmplTasks,
		"implementation.md": tmplImplementation,
		"verdict.md":        tmplVerdict,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeTempFile(t, name, content)
			if FileIsWritten(path) {
				t.Errorf("scaffold %s classified as written (should be unwritten)", name)
			}
		})
	}
}

func TestFilledTasksIsWritten(t *testing.T) {
	filled := "# Tasks: X\n\nid: abc123\nstatus: open\nanalyst: a@b\ndate: 2026-08-08\n\n" +
		"<!-- Produced by @analyst. -->\n\n" +
		"## Task breakdown\n\n" +
		"TASK-1: Do the first thing.\n" +
		"     files: a.go\n" +
		"     depends: none\n" +
		"     risk: low\n\n" +
		"TASK-2: Do the second thing.\n"
	path := writeTempFile(t, "tasks.md", filled)
	if !FileIsWritten(path) {
		t.Error("filled tasks.md classified as unwritten")
	}
}

func TestFilledImplementationByProseIsWritten(t *testing.T) {
	// No TASK- markers (implementation.md is prose, not a task list), but real
	// substantive lines under Summary → must count as written.
	filled := "# Implementation: X\n\nid: abc123\nstatus: open\ndeveloper: d@b\ndate: 2026-08-08\n\n" +
		"## Summary\n\n" +
		"Added the gallery block component and its tests.\n" +
		"It renders images from the media collection.\n\n" +
		"## Changes\n\n" +
		"src/blocks/gallery.tsx: new component.\n"
	path := writeTempFile(t, "implementation.md", filled)
	if !FileIsWritten(path) {
		t.Error("prose-filled implementation.md classified as unwritten")
	}
}

func TestMissingFileIsNotWritten(t *testing.T) {
	if FileIsWritten(filepath.Join(t.TempDir(), "nope.md")) {
		t.Error("missing file classified as written")
	}
}

func TestJobStage(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "brief.md"), []byte(tmplBrief), 0o644)

	mkJob := func() Job { j, _ := ReadJob(dir); return j }

	// Only brief → analyze.
	if got := mkJob().Stage(); got != StageAnalyze {
		t.Errorf("brief-only stage = %s, want analyze", got)
	}

	// tasks written → develop.
	os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(tmplTasks), 0o644)
	if got := mkJob().Stage(); got != StageAnalyze {
		t.Errorf("scaffold tasks.md should still be analyze; got %s", got)
	}
	os.WriteFile(filepath.Join(dir, "tasks.md"),
		[]byte("# Tasks: X\n\nid: abc123\n\n## Task breakdown\n\nTASK-1: real work here\n"), 0o644)
	if got := mkJob().Stage(); got != StageDevelop {
		t.Errorf("written tasks.md stage = %s, want develop", got)
	}

	// implementation written → review (takes precedence over tasks).
	os.WriteFile(filepath.Join(dir, "implementation.md"),
		[]byte("# Implementation: X\n\nid: abc123\n\n## Summary\n\nReal prose line one here.\nMore real prose line two.\n"), 0o644)
	if got := mkJob().Stage(); got != StageReview {
		t.Errorf("written implementation.md stage = %s, want review", got)
	}
}
