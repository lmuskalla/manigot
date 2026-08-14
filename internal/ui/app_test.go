package ui

import "testing"

// TestTruncate pins the exported shared truncation helper every agents-list
// surface uses (see AgentDescriptionWidth): at-or-under-cap strings pass
// through whole, over-cap strings are cut to n-1 characters plus an ellipsis,
// and n <= 0 leaves the string untouched.
func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"short", 10, "short"},            // at or under the cap — unchanged
		{"short", 5, "short"},             // exactly the cap — unchanged
		{"hello world", 10, "hello wor…"}, // one over — n-1 chars + ellipsis
		{"hello world", 5, "hell…"},       // well over — n-1 chars + ellipsis
		{"", 5, ""},                       // empty — unchanged
		{"hello", 0, "hello"},             // n <= 0 — unchanged
		{"hello", -3, "hello"},            // negative — unchanged
	}
	for _, c := range cases {
		if got := Truncate(c.s, c.n); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

// TestTruncateWrapperMatchesExported pins the unexported truncate as a thin
// wrapper over Truncate, so existing callers keep identical behavior.
func TestTruncateWrapperMatchesExported(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 5, 60, 200} {
		if got := truncate("hello world", n); got != Truncate("hello world", n) {
			t.Errorf("truncate(%q, %d) = %q, Truncate = %q — wrapper must match", "hello world", n, got, Truncate("hello world", n))
		}
	}
}
