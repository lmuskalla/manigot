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

// filledBrief / filledTasks / filledImplementation are past-scaffold, real
// content for each file — reused across the stage-progression tests below so
// each only has to override the file that actually changes.
const filledBrief = "# Brief: X\n\nstatus: open\nid: abc123\ndate: 2026-08-08\n\n" +
	"## What\n\nAdd a widget so users can schedule recurring exports.\n\n" +
	"## Why\n\nSeveral customers asked for this instead of the manual workaround.\n"

const filledTasks = "# Tasks: X\n\nid: abc123\n\n## Task breakdown\n\nTASK-1: real work here\n"

const filledImplementation = "# Implementation: X\n\nid: abc123\n\n## Summary\n\n" +
	"Real prose line one here.\nMore real prose line two.\n"

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
	path := writeTempFile(t, "tasks.md", filledTasks)
	if !FileIsWritten(path) {
		t.Error("filled tasks.md classified as unwritten")
	}
}

func TestFilledImplementationByProseIsWritten(t *testing.T) {
	// No TASK- markers (implementation.md is prose, not a task list), but real
	// substantive lines under Summary → must count as written.
	path := writeTempFile(t, "implementation.md", filledImplementation)
	if !FileIsWritten(path) {
		t.Error("prose-filled implementation.md classified as unwritten")
	}
}

func TestMissingFileIsNotWritten(t *testing.T) {
	if FileIsWritten(filepath.Join(t.TempDir(), "nope.md")) {
		t.Error("missing file classified as written")
	}
}

// TestJobStageProgression walks a job through every stage in order, writing
// one more file at each step — the same progression a job goes through in
// real use (brief → tasks → implementation → verdict).
func TestJobStageProgression(t *testing.T) {
	dir := t.TempDir()

	// The files written below live in the working tree, so this job is a
	// current-branch job for read-strategy purposes: Stage() must read them
	// from disk rather than via git show.
	mkJob := func() Job { j, _ := ReadJob(dir); j.OnCurrentBranch = true; return j }

	// No brief at all (ReadJob defaults status=open, no file) → define.
	if got := mkJob().Stage(); got != StageDefine {
		t.Errorf("no brief.md stage = %s, want define", got)
	}

	// Scaffold brief.md (unwritten) → still define.
	os.WriteFile(filepath.Join(dir, "brief.md"), []byte(tmplBrief), 0o644)
	if got := mkJob().Stage(); got != StageDefine {
		t.Errorf("scaffold brief.md stage = %s, want define", got)
	}

	// Real brief content, no tasks.md yet → plan.
	os.WriteFile(filepath.Join(dir, "brief.md"), []byte(filledBrief), 0o644)
	if got := mkJob().Stage(); got != StagePlan {
		t.Errorf("written brief.md stage = %s, want plan", got)
	}

	// Scaffold tasks.md (unwritten) → still plan.
	os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(tmplTasks), 0o644)
	if got := mkJob().Stage(); got != StagePlan {
		t.Errorf("scaffold tasks.md stage = %s, want plan", got)
	}

	// Written tasks.md → implement.
	os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(filledTasks), 0o644)
	if got := mkJob().Stage(); got != StageImplement {
		t.Errorf("written tasks.md stage = %s, want implement", got)
	}

	// Written implementation.md, no verdict yet → review.
	os.WriteFile(filepath.Join(dir, "implementation.md"), []byte(filledImplementation), 0o644)
	if got := mkJob().Stage(); got != StageReview {
		t.Errorf("written implementation.md stage = %s, want review", got)
	}

	// Verdict written but REJECTED → bounces back to implement, not review.
	rejected := "# Verdict: X\n\nid: abc123\n\n## Review\n\nTASK-1: FAIL — off-by-one in the export loop.\n\n" +
		"## Overall\n\nREJECTED — TASK-1 has a bug.\n"
	os.WriteFile(filepath.Join(dir, "verdict.md"), []byte(rejected), 0o644)
	if got := mkJob().Stage(); got != StageImplement {
		t.Errorf("rejected verdict.md stage = %s, want implement (bounced back)", got)
	}

	// Verdict written and APPROVED → finished.
	approved := "# Verdict: X\n\nid: abc123\n\n## Review\n\nTASK-1: PASS — matches the brief.\n\n" +
		"## Overall\n\nAPPROVED — nice work.\n"
	os.WriteFile(filepath.Join(dir, "verdict.md"), []byte(approved), 0o644)
	if got := mkJob().Stage(); got != StageFinished {
		t.Errorf("approved verdict.md stage = %s, want finished", got)
	}
}

// TestJobStageVerdictAmbiguousBouncesToImplement covers a verdict.md that has
// real content (so fileWritten("verdict.md") is true) but whose "## Overall"
// section says neither APPROVED nor REJECTED/NEEDS WORK — the safe default is
// "not approved", same as finish-job.sh's own behaviour when it can't
// determine the verdict status.
func TestJobStageVerdictAmbiguousBouncesToImplement(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "brief.md"), []byte(filledBrief), 0o644)
	os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(filledTasks), 0o644)
	os.WriteFile(filepath.Join(dir, "implementation.md"), []byte(filledImplementation), 0o644)
	ambiguous := "# Verdict: X\n\nid: abc123\n\n## Review\n\nTASK-1: looks fine, still checking edge cases.\nTASK-2: pending.\n"
	os.WriteFile(filepath.Join(dir, "verdict.md"), []byte(ambiguous), 0o644)

	j, _ := ReadJob(dir)
	j.OnCurrentBranch = true
	if got := j.Stage(); got != StageImplement {
		t.Errorf("ambiguous verdict.md stage = %s, want implement", got)
	}
}

// TestStageOffBranchImplementReadsViaGit confirms Stage() reads a job's files
// from its branch via git (not the working tree) when OnCurrentBranch is
// false. Without this, every cross-branch job would falsely report stage
// define because its files aren't checked out into the working tree at all.
func TestStageOffBranchImplementReadsViaGit(t *testing.T) {
	dir, def := gitInitRepo(t)

	// Build the job on a feature branch: a written brief + tasks.md → stage
	// implement. The brief's id matches the "ddd01_d" dir name (filledBrief's
	// hardcoded "abc123" would otherwise override the ID this test filters
	// on).
	gitRun(t, dir, "checkout", "-q", "-b", "feature/ddd01_d")
	brief := "# Brief: Ddd\n\nstatus: open\nid: ddd01\nbranch: feature/ddd01_d\ndate: 2026-01-01\n\n" +
		"## What\n\nAdd a widget so users can schedule recurring exports.\n\n" +
		"## Why\n\nSeveral customers asked for this instead of the manual workaround.\n"
	gitCommitJob(t, dir, "ddd01_d", brief)
	if err := os.WriteFile(filepath.Join(dir, "docs", "jobs", "ddd01_d", "tasks.md"), []byte(filledTasks), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "tasks")
	gitRun(t, dir, "checkout", "-q", def)

	// The job dir is NOT in the default branch's working tree.
	if _, err := os.Stat(filepath.Join(dir, "docs", "jobs", "ddd01_d")); !os.IsNotExist(err) {
		t.Fatalf("ddd01_d unexpectedly checked out on default branch: %v", err)
	}

	// Discover from the default branch and find the off-branch job.
	jobs, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	var j Job
	for _, cand := range jobs {
		if cand.ID == "ddd01" {
			j = cand
		}
	}
	if j.ID == "" {
		t.Fatalf("ddd01 not discovered: %+v", jobs)
	}
	if j.OnCurrentBranch {
		t.Fatal("ddd01 should be an off-branch job (OnCurrentBranch=false) for this test")
	}
	if got := j.Stage(); got != StageImplement {
		t.Errorf("off-branch job Stage = %s, want implement (tasks.md read via git show)", got)
	}
}

// TestStageOffBranchDefineWhenBriefUnwritten confirms an off-branch job whose
// brief is just the bare frontmatter (no real body) reports define via the
// git-read path.
func TestStageOffBranchDefineWhenBriefUnwritten(t *testing.T) {
	dir, def := gitInitRepo(t)
	gitRun(t, dir, "checkout", "-q", "-b", "feature/eee01_e")
	gitCommitJob(t, dir, "eee01_e", "# Brief: Eee\n\nstatus: open\nid: eee01\nbranch: feature/eee01_e\ndate: 2026-01-01\n")
	gitRun(t, dir, "checkout", "-q", def)

	jobs, _ := Discover(dir)
	var j Job
	for _, cand := range jobs {
		if cand.ID == "eee01" {
			j = cand
		}
	}
	if j.ID == "" {
		t.Fatalf("eee01 not discovered: %+v", jobs)
	}
	if got := j.Stage(); got != StageDefine {
		t.Errorf("off-branch brief-only Stage = %s, want define", got)
	}
}
