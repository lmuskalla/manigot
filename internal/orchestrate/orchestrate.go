// Package orchestrate decides which agent mg-jdi should run next for a job,
// or whether it should stop — the agent-sequence orchestration logic.
//
// Next is a pure function of state already visible on disk/in git: a job's
// job.Stage(), how many verdict commits exist on its branch
// (git.CountVerdictCommits), and whether the branch tip is itself one of
// those verdict commits (git.LatestCommitIsVerdict). No new state file is
// needed, and mg-jdi never has to trust in-memory state across loop
// iterations — every decision re-derives cleanly from what's already
// committed, so re-running mg-jdi against the same job after it was killed
// mid-loop is always safe.
package orchestrate

import (
	"github.com/lmuskalla/manigot/internal/agents"
	"github.com/lmuskalla/manigot/internal/job"
)

// Kind is what mg-jdi should do next.
type Kind int

const (
	// RunAgent means launch the agent named in Decision.Agent next.
	RunAgent Kind = iota
	// StopFinished means the job's verdict is APPROVED — mg-jdi's work is
	// done (it does not auto-merge; see the brief's completion criteria).
	StopFinished
	// StopNeedsHuman means mg-jdi cannot safely continue unattended and
	// must hand control back to a human.
	StopNeedsHuman
)

func (k Kind) String() string {
	switch k {
	case RunAgent:
		return "run-agent"
	case StopFinished:
		return "stop-finished"
	case StopNeedsHuman:
		return "stop-needs-human"
	default:
		return "unknown"
	}
}

// Decision is Next's result.
type Decision struct {
	// Kind is what to do.
	Kind Kind
	// Agent is set only when Kind == RunAgent: "analyst", "developer", or
	// "reviewer" — see Sequence.
	Agent string
	// Reason is a short, human-readable explanation, always set — surfaced
	// in mg-jdi's own log/status output.
	Reason string
}

// Sequence is the fixed, uniform mg-jdi agent order: the same for every job.Type — feature, fix, and chore all drive
// through analyst → developer → reviewer. Not branched on Job.Type anywhere
// in this package. Keyed off the agents package constants so a rename breaks
// the build.
var Sequence = []string{agents.Analyst, agents.Developer, agents.Reviewer}

// AgentTargetFile maps each driven agent (Sequence) to the job file it's
// expected to have written by the time its invocation returns — mg-jdi's
// output-dedup check reads this file fresh off disk (via j.Dir) to compare
// against the agent's own response text. Kept next to Sequence so the two
// can't drift.
var AgentTargetFile = map[string]string{
	agents.Analyst:   "tasks.md",
	agents.Developer: "implementation.md",
	agents.Reviewer:  "verdict.md",
}

// Next decides what mg-jdi should do next for a job.
//
// verdictRounds is git.CountVerdictCommits(root, branch, jobID) — how many
// "[ID] verdict: …" commits exist on the job's branch. latestCommitIsVerdict
// is git.LatestCommitIsVerdict(root, branch, jobID) — whether the branch tip
// is itself one of those commits, which is what lets StageImplement's two
// very different situations be told apart: a rejection @developer hasn't
// responded to yet (tip is the verdict commit) versus one it already has
// (something newer sits on top of it).
func Next(stage job.Stage, verdictRounds int, latestCommitIsVerdict bool) Decision {
	switch stage {
	case job.StageDefine:
		// brief.md itself isn't written — writing it is not any of the three
		// driven agents' job (a human fills in brief.md; @analyst starts
		// from it, per agents/analyst.md).
		return Decision{Kind: StopNeedsHuman, Reason: "brief.md is not written yet"}

	case job.StagePlan:
		return Decision{Kind: RunAgent, Agent: agents.Analyst, Reason: "brief.md written, tasks.md not written yet"}

	case job.StageReview:
		return Decision{Kind: RunAgent, Agent: agents.Reviewer, Reason: "implementation.md written, verdict.md not written yet"}

	case job.StageFinished:
		return Decision{Kind: StopFinished, Reason: "verdict.md's Overall verdict is APPROVED"}

	case job.StageImplement:
		switch {
		case verdictRounds == 0:
			// Either the very first pass (tasks.md written, implementation.md
			// not yet), or a job that started implementation but hasn't been
			// reviewed at all yet — either way, it's @developer's turn.
			return Decision{Kind: RunAgent, Agent: agents.Developer, Reason: "tasks.md written, no verdict yet"}
		case verdictRounds >= 2:
			// Two rejections already happened (a third could only exist if
			// job.Stage() were still StageImplement after a second rejection,
			// which is exactly this case) — the one-bounce retry budget is
			// exhausted regardless of what's on the tip.
			return Decision{Kind: StopNeedsHuman, Reason: "retry budget exhausted: 2 verdict commits already, still not approved"}
		case latestCommitIsVerdict:
			// Exactly 1 verdict commit, and it's still the tip: @reviewer
			// rejected once and @developer hasn't responded yet. This is the
			// one allowed bounce.
			return Decision{Kind: RunAgent, Agent: agents.Developer, Reason: "reviewer rejected once, bouncing back to developer (1 of 1 allowed retries)"}
		default:
			// Exactly 1 verdict commit, but something newer sits on top of
			// it: @developer already committed a fix since that verdict —
			// time to re-review, not another developer bounce.
			return Decision{Kind: RunAgent, Agent: agents.Reviewer, Reason: "developer has committed since the last verdict, re-review"}
		}

	default:
		return Decision{Kind: StopNeedsHuman, Reason: "unknown stage: " + string(stage)}
	}
}
