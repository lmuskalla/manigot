package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/safecode/tui/internal/job"
)

// mkStageJob writes just enough files under a fresh job dir to land the job
// in the given stage (see job.Stage's precedence), then returns the
// discovered job.
func mkStageJob(t *testing.T, stage job.Stage) job.Job {
	t.Helper()
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "zz0001_x")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: X\n\nstatus: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	switch stage {
	case job.StageDevelop, job.StageReview:
		if err := os.WriteFile(filepath.Join(jobDir, "tasks.md"),
			[]byte("# Tasks: X\n\n## Task breakdown\n\nTASK-1: real work here\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if stage == job.StageReview {
		if err := os.WriteFile(filepath.Join(jobDir, "implementation.md"),
			[]byte("# Implementation: X\n\n## Summary\n\nReal prose line one here.\nMore real prose line two.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}
	if got := jobs[0].Stage(); got != stage {
		t.Fatalf("mkStageJob: built stage %s, want %s", got, stage)
	}
	return jobs[0]
}

// TestAgentForKeyIgnoresStage is a regression test for the "launch agents
// without workflow" brief: every one of the five agent keys must resolve to
// its agent no matter which stage the open job is currently in — launching an
// agent is no longer gated by job.Stage().
func TestAgentForKeyIgnoresStage(t *testing.T) {
	wantByKey := map[string]string{
		"p": "product-owner",
		"a": "analyst",
		"d": "developer",
		"r": "reviewer",
		"s": "security",
	}

	for _, stage := range []job.Stage{job.StageAnalyze, job.StageDevelop, job.StageReview} {
		j := mkStageJob(t, stage)
		a := &App{root: t.TempDir()}
		a.detail = newDetailView(j, 80, 24)

		for key, want := range wantByKey {
			if got := a.agentForKey(key); got != want {
				t.Errorf("stage=%s key=%q: agentForKey = %q, want %q", stage, key, got, want)
			}
		}
	}
}

// TestAgentForKeyUnknownKey confirms a key with no agent binding still
// returns "".
func TestAgentForKeyUnknownKey(t *testing.T) {
	a := &App{}
	a.detail = newDetailView(mkStageJob(t, job.StageAnalyze), 80, 24)
	if got := a.agentForKey("z"); got != "" {
		t.Errorf("agentForKey(unbound key) = %q, want empty", got)
	}
}

// TestAgentForKeyNoDetail confirms agentForKey is safe to call with no open
// detail view.
func TestAgentForKeyNoDetail(t *testing.T) {
	a := &App{}
	if got := a.agentForKey("d"); got != "" {
		t.Errorf("agentForKey with no detail = %q, want empty", got)
	}
}

// TestRenderActionBarAlwaysShowsAllAgents confirms the action bar lists all
// five agent buttons regardless of the job's stage, and that the stage label
// is still shown as an informational hint.
func TestRenderActionBarAlwaysShowsAllAgents(t *testing.T) {
	for _, stage := range []job.Stage{job.StageAnalyze, job.StageDevelop, job.StageReview} {
		j := mkStageJob(t, stage)
		d := newDetailView(j, 80, 24)
		bar := d.renderActionBar()

		if !strings.Contains(bar, "stage: "+string(stage)) {
			t.Errorf("stage=%s: action bar missing stage label:\n%s", stage, bar)
		}
		for _, want := range []string{"Product Owner", "Analyst", "Developer", "Reviewer", "Security"} {
			if !strings.Contains(bar, want) {
				t.Errorf("stage=%s: action bar missing %q:\n%s", stage, want, bar)
			}
		}
	}
}
