// Package agents holds the canonical Go-side agent names — the single source
// of truth for every agent name the host-side Go references. The agents/*.md
// files are necessarily data-driven (loaded from disk by agentlist), but the
// Go-side lists — orchestrate.Sequence, the TUI's agentMeta/agentOrder,
// mg-jdi's AgentTargetFile — must agree by construction, not by convention: a
// renamed agent breaks the build everywhere it's referenced instead of
// silently breaking mg-jdi's target-file mapping or the TUI's key dispatch.
//
// The names match the @name in the agents/*.md files.
package agents

// The agent names, as launched with --agent <name>.
const (
	Analyst   = "analyst"
	Developer = "developer"
	Reviewer  = "reviewer"
	Owner     = "owner"
	Security  = "security"
)
