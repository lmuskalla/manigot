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

// TestAgentForKeyVNoLongerResolves confirms "v" (Developer's key in a
// previous revision) no longer resolves to any agent now that Developer is
// back on "d".
func TestAgentForKeyVNoLongerResolves(t *testing.T) {
	a := &App{}
	a.detail = newDetailView(mkStageJob(t, job.StageAnalyze), 80, 24)
	if got := a.agentForKey("v"); got != "" {
		t.Errorf(`agentForKey("v") = %q, want empty now that Developer uses "d"`, got)
	}
}

// TestRenderActionBarAlwaysShowsAllAgents confirms the action bar lists all
// five agent buttons regardless of the job's stage, and that the stage label
// is still shown as an informational hint. Uses a generous width so full
// labels are asserted here — the 80-column truncation behaviour has its own
// dedicated test (TestRenderActionBarTruncatesLabelsAt80ColsKeysStayIntact).
func TestRenderActionBarAlwaysShowsAllAgents(t *testing.T) {
	for _, stage := range []job.Stage{job.StageAnalyze, job.StageDevelop, job.StageReview} {
		j := mkStageJob(t, stage)
		d := newDetailView(j, 200, 24)
		bar := d.renderActionBar()

		if !strings.Contains(bar, "stage: "+string(stage)) {
			t.Errorf("stage=%s: action bar missing stage label:\n%s", stage, bar)
		}
		for _, want := range []string{"Product Owner", "Analyst", "Developer", "Reviewer", "Security", "Done"} {
			if !strings.Contains(bar, want) {
				t.Errorf("stage=%s: action bar missing %q:\n%s", stage, want, bar)
			}
		}
	}
}

// TestRenderActionBarUnifiedFormat confirms all six buttons — the five
// agents plus Done — share the same "[key] Label" structural format (TASK-8:
// no more "5 buttons + a separator + a 6th button in a different style").
func TestRenderActionBarUnifiedFormat(t *testing.T) {
	j := mkStageJob(t, job.StageAnalyze)
	d := newDetailView(j, 200, 24)
	bar := d.renderActionBar()

	if strings.Contains(bar, "│") {
		t.Errorf("action bar should no longer use a separator between the agent buttons and Done:\n%s", bar)
	}
	for _, want := range []string{"[p] Product Owner", "[a] Analyst", "[d] Developer", "[r] Reviewer", "[s] Security", "[D] Done"} {
		if !strings.Contains(bar, want) {
			t.Errorf("action bar missing button in the unified format %q:\n%s", want, bar)
		}
	}
}

// TestRenderActionBarTruncatesLabelsAt80ColsKeysStayIntact confirms the
// chosen narrow-width behaviour: at 80 columns, five full "[key] Label"
// agent buttons plus the "agents:" label don't fit on one line, so labels
// are truncated — but every bracketed key must still be present and
// untouched. The action bar itself is two lines (agents, then stage/done);
// only the agents line participates in truncation.
func TestRenderActionBarTruncatesLabelsAt80ColsKeysStayIntact(t *testing.T) {
	j := mkStageJob(t, job.StageAnalyze)
	d := newDetailView(j, 80, 24)
	bar := d.renderActionBar()

	lines := strings.Split(bar, "\n")
	if len(lines) != 2 {
		t.Fatalf("action bar should render on two lines (agents, then stage/done), got %d lines:\n%s", len(lines), bar)
	}
	agentsLine, stageLine := lines[0], lines[1]
	for _, key := range []string{"[p]", "[a]", "[d]", "[r]", "[s]"} {
		if !strings.Contains(agentsLine, key) {
			t.Errorf("agents line at 80 cols is missing key %q — keys must never be truncated:\n%s", key, agentsLine)
		}
	}
	if strings.Contains(agentsLine, "Product Owner") {
		t.Errorf("agents line at 80 cols should have truncated the longest label, not shown it in full:\n%s", agentsLine)
	}
	if !strings.Contains(stageLine, "[D]") {
		t.Errorf("stage line missing the Done button:\n%s", stageLine)
	}
}
