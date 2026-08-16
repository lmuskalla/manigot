package ui

import "github.com/lmuskalla/manigot/internal/agents"

// agentMeta describes one agent's action-bar entry: the single-key trigger and
// the human label shown on the button.
//
// The keys are chosen so they never collide with the detail view's other
// bindings (tab file nav, 1-6 file/log/diff select, e edit, D mark done, j run
// mg-jdi, x/del remove job, t tig, b switch branch, P push to origin, esc/q,
// ctrl+r). Developer uses "d" (case matters: distinct from the Shift'd "D"
// mark-done binding). This table is the single source of truth for both the
// rendering and the key handling. Agent names are the agents package
// constants, so a rename breaks the build here too.
var agentMeta = map[string]struct {
	key     string
	display string
}{
	agents.Analyst:   {key: "a", display: "Analyst"},
	agents.Owner:     {key: "o", display: "Owner"},
	agents.Developer: {key: "d", display: "Developer"},
	agents.Reviewer:  {key: "r", display: "Reviewer"},
	agents.Security:  {key: "s", display: "Security"},
}

// agentOrder is the fixed display order for the action bar's five agent
// buttons. All five are always shown, regardless of the job's current stage
// (see app.go's agentForKey) — this list, not job.Stage().Agents(), is now
// the single source of truth for the order they render in.
var agentOrder = []string{agents.Owner, agents.Analyst, agents.Developer, agents.Reviewer, agents.Security}

// agentKeyFor reports the trigger key bound to an agent ("" if unknown).
func agentKeyFor(agent string) string {
	if m, ok := agentMeta[agent]; ok {
		return m.key
	}
	return ""
}
