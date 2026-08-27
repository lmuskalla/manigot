package job

import (
	"regexp"
	"testing"
)

// jobWordRe is the charset/length contract for every entry in jobWords: a
// word must be lowercase a-z only, at most 12 characters — the size that fits
// the TUI's widened job-id column and stays typable.
var jobWordRe = regexp.MustCompile(`^[a-z]{1,12}$`)

func TestJobWordsInvariants(t *testing.T) {
	if len(jobWords) < 1000 {
		t.Errorf("jobWords has %d entries, want at least 1000 (headroom for the never-reuse policy)", len(jobWords))
	}
	seen := make(map[string]bool, len(jobWords))
	for _, w := range jobWords {
		if !jobWordRe.MatchString(w) {
			t.Errorf("jobWords entry %q violates the charset/length contract (want %s)", w, jobWordRe)
		}
		if seen[w] {
			t.Errorf("jobWords contains duplicate %q", w)
		}
		seen[w] = true
	}
}
