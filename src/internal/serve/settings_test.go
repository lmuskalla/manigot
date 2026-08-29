package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/project"
)

// --- TASK-1: GET/PUT /settings ------------------------------------------------

// TestGetSettingsDefaultsToClaudePro: with no MANIGOT_PROFILE configured,
// GET /settings reports the claude-pro fallback — the same "Active default"
// chain `mg profiles` displays.
func TestGetSettingsDefaultsToClaudePro(t *testing.T) {
	fakeCheckout(t, nil) // a checkout with no .env
	srv := New(&Registry{}, "test-version", "", nil)

	rec := get(t, srv, "/settings", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Profile != config.ProfileClaudePro {
		t.Errorf("profile = %q, want %q", body.Profile, config.ProfileClaudePro)
	}
}

// TestGetSettingsReadsConfiguredDefault: a MANIGOT_PROFILE in the checkout's
// .env is reported as the effective default.
func TestGetSettingsReadsConfiguredDefault(t *testing.T) {
	checkout := fakeCheckout(t, nil)
	if err := os.WriteFile(filepath.Join(checkout, ".env"), []byte("MANIGOT_PROFILE=zai\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(&Registry{}, "test-version", "", nil)

	rec := get(t, srv, "/settings", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Profile != config.ProfileZAI {
		t.Errorf("profile = %q, want zai", body.Profile)
	}
}

// TestPutSettingsProfileWritesSharedDefault: PUT /settings persists the
// default profile exactly the way `mg profiles <name>` does — MANIGOT_PROFILE
// in manigot's .env, preserving every other line already in the file.
func TestPutSettingsProfileWritesSharedDefault(t *testing.T) {
	checkout := fakeCheckout(t, nil)
	if err := os.WriteFile(filepath.Join(checkout, ".env"), []byte(
		"# manigot configuration — credentials and defaults (never commit this file)\nZHIPU_API_KEY=keep-me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(&Registry{}, "test-version", "", nil)

	rec := put(t, srv, "/settings", "", `{"profile":"opencode-zen"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Profile != "opencode-zen" {
		t.Errorf("response profile = %q, want opencode-zen", body.Profile)
	}

	// The write is the shared one: config.EnvValue (the reader every launch
	// path uses) sees it, and the pre-existing credential line survived.
	if got := config.EnvValue("MANIGOT_PROFILE"); got != "opencode-zen" {
		t.Errorf("MANIGOT_PROFILE = %q, want opencode-zen", got)
	}
	data, err := os.ReadFile(filepath.Join(checkout, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ZHIPU_API_KEY=keep-me") {
		t.Errorf(".env lost its other lines:\n%s", string(data))
	}

	// And GET now reports the new default.
	rec = get(t, srv, "/settings", "")
	var after settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Profile != "opencode-zen" {
		t.Errorf("GET after PUT = %q, want opencode-zen", after.Profile)
	}
}

// TestPutSettingsValidation: an empty body/field is a 400 (never a silent
// reset), and an unknown profile id is a 400 — the same wording the
// launch endpoints' resolveProfile uses.
func TestPutSettingsValidation(t *testing.T) {
	fakeCheckout(t, nil)
	srv := New(&Registry{}, "test-version", "", nil)

	if rec := put(t, srv, "/settings", "", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("empty body: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if rec := put(t, srv, "/settings", "", `{"profile":"  "}`); rec.Code != http.StatusBadRequest {
		t.Errorf("blank profile: status = %d, want 400", rec.Code)
	}
	rec := put(t, srv, "/settings", "", `{"profile":"bogus-profile"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown profile: status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unknown profile") {
		t.Errorf("400 body missing the unknown-profile wording: %s", rec.Body.String())
	}
}

// --- TASK-2: GET/PUT /projects/{project}/settings ------------------------------

// TestGetProjectSettingsDefaults: a project without .manigot/manigot.json
// reports the effective defaults — baseBranch "main", no prefix.
func TestGetProjectSettingsDefaults(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)

	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/settings", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body projectSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.BaseBranch != "main" || body.JobBranchPrefix != "" {
		t.Errorf("defaults = %+v, want baseBranch=main prefix empty", body)
	}

	if rec := get(t, srv, "/projects/no-such-project/settings", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown project: status = %d, want 404", rec.Code)
	}
}

// TestPutProjectSettingsRoundtrip: PUT persists via project.Save and both the
// API's GET and project.Load (the reader mg job/done use) see the new values.
func TestPutProjectSettingsRoundtrip(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)

	rec := put(t, srv, "/projects/"+filepath.Base(root)+"/settings", "",
		`{"baseBranch":"develop","jobBranchPrefix":"jobs"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body projectSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.BaseBranch != "develop" || body.JobBranchPrefix != "jobs" {
		t.Errorf("response = %+v, want develop/jobs", body)
	}

	stored, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BaseBranch != "develop" || stored.JobBranchPrefix != "jobs" {
		t.Errorf("project.Load = %+v, want develop/jobs — the write must be the TUI's own store", stored)
	}

	// A wholesale PUT with an absent field clears that setting (the PUT
	// files/brief precedent: replace, not patch) — baseBranch falls back to
	// the effective "main", the prefix to none.
	rec = put(t, srv, "/projects/"+filepath.Base(root)+"/settings", "", `{"baseBranch":"main"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear prefix: status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	stored, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BaseBranch != "main" || stored.JobBranchPrefix != "" {
		t.Errorf("after clear = %+v, want main/empty", stored)
	}
}

// TestPutProjectSettingsDrivesJobCreation: the settings this endpoint writes
// are the ones the lifecycle reads — after setting a jobBranchPrefix, a job
// created through the API branches under the prefix's namespace.
func TestPutProjectSettingsDrivesJobCreation(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root := initGitRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	if rec := put(t, srv, "/projects/"+base+"/settings", "", `{"jobBranchPrefix":"mg"}`); rec.Code != http.StatusOK {
		t.Fatalf("settings PUT: status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	rec := post(t, srv, "/projects/"+base+"/jobs", "", `{"title":"Prefixed Job"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var created createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Branch, "mg/") {
		t.Errorf("created branch = %q, want the mg/ prefix to apply", created.Branch)
	}
}

// TestPutProjectSettingsValidation: garbage ref values are 400s at write time,
// never stored values that later surface as opaque git failures.
func TestPutProjectSettingsValidation(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	bad := []string{
		"fea ture",             // space
		"release/1.2/../evil",  // '..'
		".hidden",              // leading dot
		"trailing.",            // trailing dot
		"/leading",             // leading slash
		"trailing/",            // trailing slash
		"branch.lock",          // .lock suffix
		"-injected-flag",       // would parse as a git option
		"care~t",               // ~
		"caret^head",           // ^
		"colon:ref",            // :
		"star*",                // *
		"brack[et",             // [
		"back\\slash",          // backslash (also a Windows separator)
		"at@{brace",            // @{
		"@",                    // lone @
		"question?mark",        // ?
		"nul\x00byte",          // control characters
		strings.Repeat("x", refComponentMaxLen+1), // over the length cap
	}
	for _, value := range bad {
		rec := put(t, srv, "/projects/"+base+"/settings", "",
			`{"baseBranch":`+jsonString(value)+`}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("baseBranch %q: status = %d, want 400 (body %s)", value, rec.Code, rec.Body.String())
		}
		rec = put(t, srv, "/projects/"+base+"/settings", "",
			`{"jobBranchPrefix":`+jsonString(value)+`}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("jobBranchPrefix %q: status = %d, want 400 (body %s)", value, rec.Code, rec.Body.String())
		}
	}

	// Nothing was stored by the rejected requests.
	if _, err := os.Stat(filepath.Join(root, ".manigot", "manigot.json")); !os.IsNotExist(err) {
		t.Errorf("a rejected PUT still wrote the settings file")
	}

	// The legitimate namespaced shapes pass: release branches, prefix chains.
	for _, value := range []string{"main", "develop", "release/1.2", "origin/main", "jobs", "team/feature"} {
		if reason, ok := refComponentProblem(value); !ok {
			t.Errorf("refComponentProblem(%q) rejected: %s", value, reason)
		}
	}
}

// jsonString marshals s as a JSON string literal — the bad-value table above
// contains characters that must reach the decoder escaped, so hand-building
// the body with fmt would be wrong.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TestRefComponentProblemEmptyIsValid: empty is a valid shape (it means
// "clear the setting"); whether empty is allowed is the caller's decision.
func TestRefComponentProblemEmptyIsValid(t *testing.T) {
	if reason, ok := refComponentProblem(""); !ok {
		t.Errorf("refComponentProblem(\"\") rejected: %s", reason)
	}
}
