package orchestrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lmuskalla/manigot/tui/internal/job"
)

func TestNext(t *testing.T) {
	tests := []struct {
		name                  string
		stage                 job.Stage
		verdictRounds         int
		latestCommitIsVerdict bool
		wantKind              Kind
		wantAgent             string
	}{
		{
			name:     "define: brief.md not written",
			stage:    job.StageDefine,
			wantKind: StopNeedsHuman,
		},
		{
			name:      "plan: tasks.md not written",
			stage:     job.StagePlan,
			wantKind:  RunAgent,
			wantAgent: "analyst",
		},
		{
			name:      "review: verdict.md not written",
			stage:     job.StageReview,
			wantKind:  RunAgent,
			wantAgent: "reviewer",
		},
		{
			name:     "finished: verdict approved",
			stage:    job.StageFinished,
			wantKind: StopFinished,
		},
		{
			name:          "implement, 0 verdicts: first developer pass",
			stage:         job.StageImplement,
			verdictRounds: 0,
			wantKind:      RunAgent,
			wantAgent:     "developer",
		},
		{
			name:                  "implement, 0 verdicts, tip flag irrelevant",
			stage:                 job.StageImplement,
			verdictRounds:         0,
			latestCommitIsVerdict: true, // should never happen with 0 rounds, but must not change the outcome
			wantKind:              RunAgent,
			wantAgent:             "developer",
		},
		{
			name:                  "implement, 1 verdict, tip is the verdict: bounce to developer",
			stage:                 job.StageImplement,
			verdictRounds:         1,
			latestCommitIsVerdict: true,
			wantKind:              RunAgent,
			wantAgent:             "developer",
		},
		{
			name:                  "implement, 1 verdict, developer already responded: re-review",
			stage:                 job.StageImplement,
			verdictRounds:         1,
			latestCommitIsVerdict: false,
			wantKind:              RunAgent,
			wantAgent:             "reviewer",
		},
		{
			name:                  "implement, 2 verdicts, tip is the verdict: budget exhausted",
			stage:                 job.StageImplement,
			verdictRounds:         2,
			latestCommitIsVerdict: true,
			wantKind:              StopNeedsHuman,
		},
		{
			name:                  "implement, 2 verdicts, developer responded again: still budget exhausted",
			stage:                 job.StageImplement,
			verdictRounds:         2,
			latestCommitIsVerdict: false,
			wantKind:              StopNeedsHuman,
		},
		{
			name:          "implement, 3+ verdicts: still budget exhausted",
			stage:         job.StageImplement,
			verdictRounds: 5,
			wantKind:      StopNeedsHuman,
		},
		{
			name:     "unknown stage",
			stage:    job.Stage("bogus"),
			wantKind: StopNeedsHuman,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Next(tc.stage, tc.verdictRounds, tc.latestCommitIsVerdict)
			if got.Kind != tc.wantKind {
				t.Errorf("Next(%v, %d, %v).Kind = %v, want %v", tc.stage, tc.verdictRounds, tc.latestCommitIsVerdict, got.Kind, tc.wantKind)
			}
			if got.Kind == RunAgent && got.Agent != tc.wantAgent {
				t.Errorf("Next(%v, %d, %v).Agent = %q, want %q", tc.stage, tc.verdictRounds, tc.latestCommitIsVerdict, got.Agent, tc.wantAgent)
			}
			if got.Kind != RunAgent && got.Agent != "" {
				t.Errorf("Next(%v, %d, %v).Agent = %q, want empty for Kind=%v", tc.stage, tc.verdictRounds, tc.latestCommitIsVerdict, got.Agent, got.Kind)
			}
			if got.Reason == "" {
				t.Errorf("Next(%v, %d, %v).Reason is empty, want a non-empty explanation", tc.stage, tc.verdictRounds, tc.latestCommitIsVerdict)
			}
		})
	}
}

// TestNextStagesCoverEveryDefinedStage guards against a future job.Stage
// addition silently falling into Next's "unknown stage" default without a
// deliberate decision — every entry in job.Stages must appear as its own
// subtest above.
func TestNextStagesCoverEveryDefinedStage(t *testing.T) {
	covered := map[job.Stage]bool{
		job.StageDefine:    true,
		job.StagePlan:      true,
		job.StageImplement: true,
		job.StageReview:    true,
		job.StageFinished:  true,
	}
	for _, s := range job.Stages {
		if !covered[s] {
			t.Errorf("job.Stages contains %q, which orchestrate_test.go's coverage map doesn't know about — add a case", s)
		}
	}
}

// TestNextRealButTerseBriefRunsAnalyst confirms the fix for the "jdi does
// not work" job holds at the orchestration layer, not just in
// job.FileIsWritten directly: a job whose brief.md has one filled section
// (mirroring real usage — and specifically this job's own brief.md, see
// job.TestTerseRealBriefIsWritten) and no tasks.md yet must make Next
// return RunAgent("analyst"), not StopNeedsHuman. Before the fix, mg jdi
// stopped immediately on a job exactly like this, before ever invoking an
// agent.
func TestNextRealButTerseBriefRunsAnalyst(t *testing.T) {
	terseRealBrief := "# Brief: jdi does not work\n\n" +
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

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(terseRealBrief), 0o644); err != nil {
		t.Fatal(err)
	}
	// No tasks.md yet.

	j, err := job.ReadJob(dir)
	if err != nil {
		t.Fatal(err)
	}
	j.OnCurrentBranch = true

	stage := j.Stage()
	if stage != job.StagePlan {
		t.Fatalf("terse-but-real brief.md job stage = %s, want plan", stage)
	}

	got := Next(stage, 0, false)
	if got.Kind != RunAgent || got.Agent != "analyst" {
		t.Errorf("Next(%v, 0, false) = {Kind: %v, Agent: %q}, want {Kind: RunAgent, Agent: \"analyst\"}", stage, got.Kind, got.Agent)
	}
}

func TestKindString(t *testing.T) {
	tests := map[Kind]string{
		RunAgent:       "run-agent",
		StopFinished:   "stop-finished",
		StopNeedsHuman: "stop-needs-human",
		Kind(99):       "unknown",
	}
	for k, want := range tests {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}
