package git

import "strings"

// BranchTail returns the id_slug tail segment of a branch name (everything
// after the last "/", or the whole name when there is none). A job's branch
// embeds its id_slug as the tail segment (e.g. "feature/irw320_tui" →
// "irw320_tui"), and that id_slug is also the job's directory name — so
// branch-tail matching and job-name matching are the same concept.
func BranchTail(branch string) string {
	if i := strings.LastIndex(branch, "/"); i >= 0 {
		return branch[i+1:]
	}
	return branch
}

// ExactBranchMatch returns the branch whose tail segment (after the last "/")
// equals name, or "" when none does — the "exact match on the id_slug segment
// first" resolution the scripts used for --job, mg done, and mg delete.
func ExactBranchMatch(branches []string, name string) string {
	for _, b := range branches {
		if BranchTail(b) == name {
			return b
		}
	}
	return ""
}

// PrefixBranchMatches returns the branches whose tail segment starts with name
// — the prefix-match fallback (and the ambiguity error) for the same
// resolution paths.
func PrefixBranchMatches(branches []string, name string) []string {
	var matches []string
	for _, b := range branches {
		if strings.HasPrefix(BranchTail(b), name) {
			matches = append(matches, b)
		}
	}
	return matches
}
