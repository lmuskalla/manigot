package git

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

// commitAllAt stages every change under dir and commits it with an explicit
// author/committer timestamp, `secs` seconds after a fixed epoch. git log's
// default ordering is reverse-chronological by commit time; commits made in
// the same real-world second (as tests otherwise do) can tie, so the
// ordering test below needs commits with deterministic, distinct timestamps
// rather than relying on wall-clock spacing.
func commitAllAt(t *testing.T, dir, msg string, secs int) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	date := fmt.Sprintf("2026-01-01T00:00:%02dZ", secs)
	cmd := exec.Command("git", "-C", dir, "commit", "-q", "-m", msg)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit -m %q in %s: %v\n%s", msg, dir, err, out)
	}
}

// TestRecentCommitsDedupsSharedTip is the brief's explicit open question: a
// branch that hasn't diverged from another shares its tip commit, and that
// commit must appear exactly once in the result, not once per branch that
// can reach it.
func TestRecentCommitsDedupsSharedTip(t *testing.T) {
	dir, def := initRepo(t)
	// "other" branches off at the exact same commit as def and never diverges.
	runGit(t, dir, "branch", "other")

	got, err := RecentCommits(dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("RecentCommits with two branches sharing one commit = %d entries, want 1: %+v", len(got), got)
	}

	seen := map[string]bool{}
	for _, c := range got {
		if seen[c.Hash] {
			t.Errorf("commit %s appeared more than once: %+v", c.Hash, got)
		}
		seen[c.Hash] = true
	}
	_ = def
}

// TestRecentCommitsOrderingMostRecentFirst verifies commits come back newest
// first, across branches (not grouped/sorted per-branch). All three commits
// get explicit, strictly increasing timestamps via commitAllAt: commits made
// within the same real-world second (as plain commitAll would do here) can
// tie under git log's default reverse-chronological ordering, which would
// make this test flaky rather than actually asserting order.
func TestRecentCommitsOrderingMostRecentFirst(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	writeFile(t, dir, "a.txt", "0\n")
	commitAllAt(t, dir, "init", 0)
	def, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("read default branch: %v", err)
	}

	writeFile(t, dir, "a.txt", "1\n")
	commitAllAt(t, dir, "second", 1)

	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "a.txt", "2\n")
	commitAllAt(t, dir, "third on feature", 2)
	runGit(t, dir, "checkout", "-q", def)

	got, err := RecentCommits(dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("RecentCommits = %d entries, want 3: %+v", len(got), got)
	}
	if got[0].Subject != "third on feature" {
		t.Errorf("most recent commit = %q, want %q", got[0].Subject, "third on feature")
	}
	if got[0].Branch != "feature" {
		t.Errorf("most recent commit branch = %q, want %q", got[0].Branch, "feature")
	}
	if got[1].Subject != "second" {
		t.Errorf("second-most-recent commit = %q, want %q", got[1].Subject, "second")
	}
	if got[2].Subject != "init" {
		t.Errorf("oldest returned commit = %q, want %q", got[2].Subject, "init")
	}
}

// TestRecentCommitsTruncatesToN verifies a history longer than n is capped.
func TestRecentCommitsTruncatesToN(t *testing.T) {
	dir, _ := initRepo(t)
	for i := 0; i < 4; i++ {
		writeFile(t, dir, "a.txt", string(rune('a'+i)))
		commitAll(t, dir, "commit "+string(rune('a'+i)))
	}
	// 5 commits total (init + 4), asking for 3 should return exactly 3.
	got, err := RecentCommits(dir, 3)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("RecentCommits(dir, 3) = %d entries, want 3: %+v", len(got), got)
	}
}

// TestRecentCommitsFewerThanN verifies a history shorter than n simply
// returns everything there is, rather than padding or erroring.
func TestRecentCommitsFewerThanN(t *testing.T) {
	dir, _ := initRepo(t)
	got, err := RecentCommits(dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("RecentCommits on a 1-commit repo asking for 5 = %d entries, want 1: %+v", len(got), got)
	}
}

// TestRecentCommitsEmptyRepo verifies a repo with no commits yet (unborn
// HEAD, no local branches) degrades to an empty slice and a nil error,
// mirroring LocalBranches' own degrade for the same case.
func TestRecentCommitsEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")

	got, err := RecentCommits(dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits on an empty repo: unexpected error %v", err)
	}
	if len(got) != 0 {
		t.Errorf("RecentCommits on an empty repo = %v, want empty", got)
	}
}

// TestRecentCommitsNotARepo verifies a non-repo directory returns
// ErrNotARepo rather than a generic error.
func TestRecentCommitsNotARepo(t *testing.T) {
	_, err := RecentCommits(t.TempDir(), 5)
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("RecentCommits on non-repo: err = %v, want ErrNotARepo", err)
	}
}
