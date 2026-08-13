package job

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestIDAvailable(t *testing.T) {
	existing := map[string]bool{"flower": true, "flowerbed": true, "cat": true}
	cases := []struct {
		name      string
		candidate string
		want      bool
	}{
		{"exact collision", "flower", false},
		{"candidate is prefix of an existing id", "flowerb", false},
		{"existing id is prefix of candidate", "caterpillar", false},
		{"free word", "garden", true},
		{"unrelated word", "sunset", true},
	}
	for _, c := range cases {
		if got := idAvailable(c.candidate, existing); got != c.want {
			t.Errorf("idAvailable(%q) = %v, want %v", c.candidate, got, c.want)
		}
	}
}

// seqPick returns a pick func that hands out the given words in order.
func seqPick(words ...string) func() (string, error) {
	i := 0
	return func() (string, error) {
		w := words[i]
		i++
		return w, nil
	}
}

func TestUniqueJobIDRetriesOnExactCollision(t *testing.T) {
	got, err := uniqueJobID(seqPick("flower", "flower", "garden"), func() (map[string]bool, error) {
		return map[string]bool{"flower": true}, nil
	})
	if err != nil {
		t.Fatalf("uniqueJobID: %v", err)
	}
	if got != "garden" {
		t.Errorf("uniqueJobID = %q, want garden (retried past the taken word)", got)
	}
}

func TestUniqueJobIDRejectsPrefixCollisions(t *testing.T) {
	// "flowerbed" has the existing "flower" as a prefix; "flo" is a prefix of
	// the existing "flower". Both must be rejected before "garden" wins.
	got, err := uniqueJobID(seqPick("flowerbed", "flo", "garden"), func() (map[string]bool, error) {
		return map[string]bool{"flower": true}, nil
	})
	if err != nil {
		t.Fatalf("uniqueJobID: %v", err)
	}
	if got != "garden" {
		t.Errorf("uniqueJobID = %q, want garden (prefix-colliding words rejected)", got)
	}
}

func TestUniqueJobIDExhaustion(t *testing.T) {
	// Every draw collides with the taken word, so all maxIDAttempts are
	// consumed and the documented exhaustion error is returned.
	_, err := uniqueJobID(func() (string, error) { return "flower", nil }, func() (map[string]bool, error) {
		return map[string]bool{"flower": true}, nil
	})
	if err == nil {
		t.Fatal("uniqueJobID: expected an exhaustion error")
	}
	if !strings.Contains(err.Error(), "word list is exhausted") ||
		!strings.Contains(err.Error(), "internal/job/words.go") {
		t.Errorf("exhaustion error does not point at the word list: %v", err)
	}
}

func TestUniqueJobIDScanErrorFailsCreate(t *testing.T) {
	_, err := uniqueJobID(seqPick("garden"), func() (map[string]bool, error) {
		return nil, errors.New("scan boom")
	})
	if err == nil || !strings.Contains(err.Error(), "scanning existing job ids") {
		t.Errorf("scan error not wrapped: %v", err)
	}
}

// TestCreateJobDefaultPathAvoidsTakenWord exercises the wiring: CreateJob
// without an injected RandomID runs the uniqueness retry loop against the
// real scan, so the resulting id must not equal, be a prefix of, or be
// prefixed by the taken word — whatever the random draw produces.
func TestCreateJobDefaultPathAvoidsTakenWord(t *testing.T) {
	dir := createCheckout(t, t.TempDir())
	if _, err := CreateJob(dir, "Taken Word", CreateOptions{RandomID: fixedID("flower")}, io.Discard); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	res, err := CreateJob(dir, "Second Job", CreateOptions{}, io.Discard)
	if err != nil {
		t.Fatalf("CreateJob (default path): %v", err)
	}
	id := res.Job.ID
	if !jobWordRe.MatchString(id) {
		t.Errorf("default path produced non-word id %q", id)
	}
	if id == "flower" || strings.HasPrefix(id, "flower") || strings.HasPrefix("flower", id) {
		t.Errorf("default path reused or prefix-collided with the taken word: id = %q", id)
	}
}
