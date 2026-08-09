package resolve

import (
	"path/filepath"
	"testing"
)

// The settled naming decision, spelled out so a rename cannot happen by
// accident: sc/sc-job/sc-done are each the sole candidate name — no short
// alias, no legacy fallback.
func TestCommandSpecs(t *testing.T) {
	cases := []struct {
		spec   Spec
		label  string
		env    string
		names  []string
		script string
	}{
		{Safecode(), "sc", "SAFECODE_BIN", []string{"sc"}, "scripts/run.sh"},
		{Job(), "sc-job", "SAFECODE_JOB_BIN", []string{"sc-job"}, "scripts/new-job.sh"},
		{Done(), "sc-done", "SAFECODE_DONE_BIN", []string{"sc-done"}, "scripts/finish-job.sh"},
	}
	for _, c := range cases {
		if c.spec.Label != c.label {
			t.Errorf("Label = %q, want %q", c.spec.Label, c.label)
		}
		if c.spec.EnvVar != c.env {
			t.Errorf("%s: EnvVar = %q, want %q", c.label, c.spec.EnvVar, c.env)
		}
		if c.spec.Script != c.script {
			t.Errorf("%s: Script = %q, want %q", c.label, c.spec.Script, c.script)
		}
		if len(c.spec.Names) != len(c.names) {
			t.Fatalf("%s: Names = %v, want %v", c.label, c.spec.Names, c.names)
		}
		for i := range c.names {
			if c.spec.Names[i] != c.names[i] {
				t.Errorf("%s: Names[%d] = %q, want %q", c.label, i, c.spec.Names[i], c.names[i])
			}
		}
	}
}

// Specs must be independent copies — a caller appending to Names must not
// corrupt the next caller's lookup order.
func TestCommandSpecsAreCopies(t *testing.T) {
	a := Job()
	a.Names = append(a.Names[:1:1], "mutated")
	if got := Job().Names; len(got) != 1 || got[0] != "sc-job" {
		t.Errorf("Job() candidate list was mutated: Names = %v", got)
	}
}

// Every spec must be resolvable from a checkout alone, which is also a check
// that its Script path matches a real script in this repository.
func TestSpecScriptsResolveFromCheckout(t *testing.T) {
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if !looksLikeCheckout(repo) {
		t.Skipf("%s is not a safecode checkout", repo)
	}
	for _, spec := range []Spec{Safecode(), Job(), Done()} {
		isolate(t)
		t.Setenv("SAFECODE_HOME", repo)

		got, err := Resolve(spec)
		if err != nil {
			t.Errorf("%s: %v", spec.Label, err)
			continue
		}
		if want := filepath.Join(repo, spec.Script); got.Path != want {
			t.Errorf("%s: Path = %q, want %q", spec.Label, got.Path, want)
		}
	}
}
