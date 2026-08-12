package git

import (
	"reflect"
	"testing"
)

func TestBranchTail(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"feature/abc123_hello", "abc123_hello"},
		{"feature/irw320_tui", "irw320_tui"},
		{"fix/abc123_x", "abc123_x"},
		{"abc123_hello", "abc123_hello"},                // no "/" — the whole name
		{"", ""},                                        // empty input
		{"feature/", ""},                                // trailing slash
		{"prefix/feature/abc123_hello", "abc123_hello"}, // nested prefix
	}
	for _, tt := range tests {
		if got := BranchTail(tt.branch); got != tt.want {
			t.Errorf("BranchTail(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestExactBranchMatch(t *testing.T) {
	branches := []string{"main", "feature/abc123_hello", "fix/abc123_x", "feature/zzz999_nope"}

	tests := []struct {
		name string
		want string
	}{
		{"abc123_hello", "feature/abc123_hello"},
		{"abc123_x", "fix/abc123_x"},
		{"zzz999_nope", "feature/zzz999_nope"},
		{"main", "main"},             // a bare branch is its own tail
		{"missing", ""},              // no match at all
		{"abc123", ""},               // prefix is not an exact match
		{"feature/abc123_hello", ""}, // full branch name is not a tail match
	}
	for _, tt := range tests {
		if got := ExactBranchMatch(branches, tt.name); got != tt.want {
			t.Errorf("ExactBranchMatch(%v, %q) = %q, want %q", branches, tt.name, got, tt.want)
		}
	}
}

func TestPrefixBranchMatches(t *testing.T) {
	branches := []string{"main", "feature/abc123_hello", "fix/abc123_x", "feature/abc124_other", "feature/zzz999_nope"}

	tests := []struct {
		name string
		want []string
	}{
		{"abc12", []string{"feature/abc123_hello", "fix/abc123_x", "feature/abc124_other"}},
		{"abc123", []string{"feature/abc123_hello", "fix/abc123_x"}},
		{"zzz", []string{"feature/zzz999_nope"}},
		{"main", []string{"main"}}, // a bare branch matches against itself
		{"missing", nil},           // no match at all
	}
	for _, tt := range tests {
		if got := PrefixBranchMatches(branches, tt.name); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("PrefixBranchMatches(%v, %q) = %v, want %v", branches, tt.name, got, tt.want)
		}
	}
}
