package resolve

import (
	"path/filepath"
	"testing"
)

// The settled naming decision, spelled out so a rename cannot happen by
// accident: mg-job/mg-done/mg-delete are each the sole candidate Names entry
// — no short alias, no legacy fallback. Label is the human-facing "mg
// <subcommand>" phrasing (scripts/mg.sh dispatches these via `make install`'s
// single `mg` symlink); Names/Script/EnvVar still target the old standalone
// script names, which remain valid, untried-by-default $PATH candidates.
func TestCommandSpecs(t *testing.T) {
	cases := []struct {
		spec   Spec
		label  string
		env    string
		names  []string
		script string
	}{
		{Manigot(), "mg", "MANIGOT_BIN", []string{"mg"}, "scripts/run.sh"},
		{Job(), "mg job", "MANIGOT_JOB_BIN", []string{"mg-job"}, "scripts/new-job.sh"},
		{Done(), "mg done", "MANIGOT_DONE_BIN", []string{"mg-done"}, "scripts/finish-job.sh"},
		{Delete(), "mg delete", "MANIGOT_DELETE_BIN", []string{"mg-delete"}, "scripts/delete-job.sh"},
		{Jdi(), "mg jdi", "MANIGOT_JDI_BIN", []string{"mg-jdi"}, "scripts/jdi.sh"},
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
	if got := Job().Names; len(got) != 1 || got[0] != "mg-job" {
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
		t.Skipf("%s is not a manigot checkout", repo)
	}
	for _, spec := range []Spec{Manigot(), Job(), Done(), Delete(), Jdi()} {
		isolate(t)
		t.Setenv("MANIGOT_HOME", repo)

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
