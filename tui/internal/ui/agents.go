package ui

// agentMeta describes one agent's action-bar entry: the single-key trigger and
// the human label shown on the button.
//
// The keys are chosen so they never collide with the detail view's other
// bindings (tab/h/l file nav, j/k scroll, 1-4 file select, esc/q). TASK-8 binds
// these keys to the terminal launcher; this table is the single source of truth
// for both the rendering and the key handling.
var agentMeta = map[string]struct {
	key     string
	display string
}{
	"analyst":       {key: "a", display: "Analyst"},
	"product-owner": {key: "p", display: "Product Owner"},
	"developer":     {key: "d", display: "Developer"},
	"reviewer":      {key: "r", display: "Reviewer"},
	"security":      {key: "s", display: "Security"},
}

// agentKeyFor reports the trigger key bound to an agent ("" if unknown).
func agentKeyFor(agent string) string {
	if m, ok := agentMeta[agent]; ok {
		return m.key
	}
	return ""
}
