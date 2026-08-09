package ui

// agentMeta describes one agent's action-bar entry: the single-key trigger and
// the human label shown on the button.
//
// The keys are chosen so they never collide with the detail view's other
// bindings (tab/h/l file nav, j/k scroll, 1-4 file select, e edit, D mark
// done, c switch branch, esc/q, ctrl+r). Developer uses "v" rather than the
// more obvious "d" — "d" collides with "D" mark-done (the two differ only by
// Shift on adjacent buttons, a real mis-press risk); "v" was unbound at
// detail-view scope when this was chosen. This table is the single source of
// truth for both the rendering and the key handling.
var agentMeta = map[string]struct {
	key     string
	display string
}{
	"analyst":       {key: "a", display: "Analyst"},
	"product-owner": {key: "p", display: "Product Owner"},
	"developer":     {key: "v", display: "Developer"},
	"reviewer":      {key: "r", display: "Reviewer"},
	"security":      {key: "s", display: "Security"},
}

// agentOrder is the fixed display order for the action bar's five agent
// buttons. All five are always shown, regardless of the job's current stage
// (see app.go's agentForKey) — this list, not job.Stage().Agents(), is now
// the single source of truth for the order they render in.
var agentOrder = []string{"product-owner", "analyst", "developer", "reviewer", "security"}

// agentKeyFor reports the trigger key bound to an agent ("" if unknown).
func agentKeyFor(agent string) string {
	if m, ok := agentMeta[agent]; ok {
		return m.key
	}
	return ""
}
