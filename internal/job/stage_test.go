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

// terseRealBrief mirrors real-world usage: exactly one section ("## What")
// filled in with genuine prose, while the rest of new-job.sh's scaffold
// sections ("## Why", "## Out of scope", "## Notes") are left untouched as
// their HTML-comment placeholders — shaped after this job's own brief.md
// (docs/jobs/4i5tcx_jdi-does-not-work/brief.md). A brief like this is
// "written" by any human standard, but only produces a single substantive
// line, which is what TASK-1 is regression-testing against isWritten's old
// "≥2 substantive lines" rule.
const terseRealBrief = "# Brief: jdi does not work\n\n" +
	"status: open\n" +
	"type: feature\n" +
	"id: 4i5tcx\n" +
	"branch: feature/4i5tcx_jdi-does-not-work\n" +
	"date: 2026-08-10\n" +
	"author: Leander Muskalla\n\n" +
	"## What\n\n" +
	"jdi gets immediately stuck. I've tried it for an easy job, but it immediately " +
	"said \"needs human\" after a few seconds. run.log is empty and status just says " +
	"\"stopped:needs-human\". I'm pretty sure the LLM wasn't even fast enough to come " +
	"to that conclusion in the time it already stopped. So something is amiss on our side.\n\n" +
	"## Why\n\n" +
	"<!-- Why does this need to exist? What problem does it solve for the user? -->\n\n" +
	"## Out of scope\n\n" +
	"<!-- What are we explicitly NOT doing in this job? -->\n\n" +
	"## Notes\n\n" +
	"<!-- Anything the analyst or developer should know before starting. -->\n"

// TestTerseRealBriefIsWritten reproduces the "jdi does not work" bug: a real
// brief with only one filled section must be classified as written, and a
// job built from it must report StagePlan (ready for @analyst), not
// StageDefine. This fails against the pre-fix "≥2 substantive lines" rule in
// isWritten, confirming the diagnosis before TASK-2's fix lands.
func TestTerseRealBriefIsWritten(t *testing.T) {
	path := writeTempFile(t, "brief.md", terseRealBrief)
	if !FileIsWritten(path) {
		t.Error("terse-but-real brief.md classified as unwritten")
	}
}

func TestTerseRealBriefJobStageIsPlan(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(terseRealBrief), 0o644); err != nil {
		t.Fatal(err)
	}
	j, err := ReadJob(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := j.Stage(); got != StagePlan {
		t.Errorf("terse-but-real brief.md job stage = %s, want plan", got)
	}
}

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

	// Stage() always reads straight from j.Dir (see the package doc).
	mkJob := func() Job { j, _ := ReadJob(dir); return j }

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
	if got := j.Stage(); got != StageImplement {
		t.Errorf("ambiguous verdict.md stage = %s, want implement", got)
	}
}

// TestStageOfDiscoveredWorktreeJob confirms Stage() works correctly for a
// job as actually returned by Discover — reading straight from its own
// worktree, no branch check involved (207bfu_git-worktrees).
func TestStageOfDiscoveredWorktreeJob(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	brief := "# Brief: Ddd\n\nstatus: open\nid: ddd01\nbranch: feature/ddd01_d\ndate: 2026-01-01\n\n" +
		"## What\n\nAdd a widget so users can schedule recurring exports.\n\n" +
		"## Why\n\nSeveral customers asked for this instead of the manual workaround.\n"
	wtPath := addJobWorktree(t, dir, wts, "feature/ddd01_d", "ddd01_d", brief)

	if err := os.WriteFile(filepath.Join(wtPath, "docs", "jobs", "ddd01_d", "tasks.md"), []byte(filledTasks), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "-A")
	gitRun(t, wtPath, "commit", "-q", "-m", "tasks")

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
	if got := j.Stage(); got != StageImplement {
		t.Errorf("worktree job Stage = %s, want implement", got)
	}
}

// TestVerdictOverallMatch pins the single verdict-section extraction
// (verdictOverallSection / verdictOverallMatch, unified in the "improve code
// quality" job's TASK-4). The whole-section behavior wins over finish-job.sh's
// grep -A5 window: a status line anywhere under the "## Overall" heading is
// recognised — the "status beyond line 5" case below documents exactly that
// decision — and the scaffold's HTML-comment guidance lines are not verdict
// content.
func TestVerdictOverallMatch(t *testing.T) {
	cases := []struct {
		name string
		md   string
		want string
	}{
		{"approved", "# Verdict\n\n## Overall\n\nAPPROVED\n", "APPROVED"},
		{"needs work", "# Verdict\n\n## Overall\n\nNEEDS WORK — fix TASK-1\n", "NEEDS WORK — fix TASK-1"},
		{"rejected in first lines", "# Verdict\n\n## Overall\n\nThe reviewer says REJECTED.\nmore\nmore\nmore\nmore\n", "The reviewer says REJECTED."},
		{"no heading", "# Verdict\n\nJust text.\n", ""},
		// The decided corner: the section (not grep's -A5 window) wins, so a
		// genuine status more than five lines below the heading IS recognised.
		{"status beyond line 5 (section wins)", "# Verdict\n\n## Overall\n\nok\nok\nok\nok\nok\nok\nAPPROVED\n", "APPROVED"},
		// Scaffold guidance comments are stripped, not read as verdicts.
		{"scaffold comment is not a status", "# Verdict\n\n## Overall\n\n<!-- APPROVED / REJECTED / NEEDS WORK -->\n", ""},
		{"status line plus comment", "# Verdict\n\n## Overall\n\nREJECTED — fix TASK-1\n\n<!-- Summary -->\n", "REJECTED — fix TASK-1"},
		{"status beyond next heading", "# Verdict\n\n## Overall\n\nAPPROVED\n\n## Security\n\nnone\n", "APPROVED"},
	}
	for _, c := range cases {
		if got := verdictOverallMatch([]byte(c.md)); got != c.want {
			t.Errorf("%s: verdictOverallMatch = %q, want %q", c.name, got, c.want)
		}
	}
}
