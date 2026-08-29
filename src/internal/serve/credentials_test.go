package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
)

// credentialKeys are the keys whose values must never appear in any response
// body — the daemon's credential surface: subscription tokens, API keys, the
// serve token, and the per-profile model keys. The test reads their values via
// the same config.EnvValue path the daemon itself reads (the .env file in the
// manigot checkout, then the process environment), so the test and the server
// can never drift on what counts as a credential.
var credentialKeys = []string{
	"CLAUDE_CODE_OAUTH_TOKEN",
	"CLAUDE_ACCOUNT_UUID",
	"CLAUDE_EMAIL",
	"CLAUDE_ORG_UUID",
	"ANTHROPIC_API_KEY",
	"OPENCODE_API_KEY",
	"ZHIPU_API_KEY",
	"MG_SERVE_TOKEN",
	"OPENCODE_ZAI_MODEL",
	"OPENCODE_GO_MODEL",
	"OPENCODE_ZEN_MODEL",
	"OPENCODE_ZEN_FREE_MODEL",
	"OPENCODE_MODEL",
	"OPENCODE_THEME",
}

// TestCredentialsNeverReturned is the brief's "no .env content, no keys, no
// tokens in any response, ever" guarantee enforced over the whole surface: a
// known credential value is planted in the daemon's real credential source
// (.env + process env), every endpoint is hit (with the correct bearer token),
// and every response body — including a 401 — is grepped for the values.
func TestCredentialsNeverReturned(t *testing.T) {
	// The credential source: a fake checkout's .env plus the process
	// environment, both read through config.EnvValue exactly as the daemon
	// reads them.
	envLines := map[string]string{}
	for _, key := range credentialKeys {
		envLines[key] = "fixture-" + key + "-VALUE"
		t.Setenv(key, envLines[key])
	}
	checkout := fakeCheckout(t, map[string]string{
		"analyst": "name: analyst\ndescription: Breaks requests into tasks.\n",
	})
	// The .env file itself — the daemon's preferred source. Two extra
	// non-credential lines ride along to pin the "no full line of .env
	// content" half of the guarantee.
	var envFile strings.Builder
	for _, key := range credentialKeys {
		envFile.WriteString(key + "=" + envLines[key] + "\n")
	}
	envFile.WriteString("# a comment line in the env\n")
	envFile.WriteString("UNRELATED_SETTING=some-value\n")
	if err := os.WriteFile(filepath.Join(checkout, ".env"), []byte(envFile.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// A non-git project with a job (exercises projects/jobs/files/jdi/agents),
	// and a git repo with a job branch (exercises diff).
	jobProj := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"),
		"tasks.md", "implementation.md", "verdict.md")
	if err := writeJDIStatusForTest(t, jobProj, "wood_oak"); err != nil {
		t.Fatal(err)
	}
	logDir := job.JDIStatusDir(jobProj, "wood_oak")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(job.JDIRunLogPath(jobProj, "wood_oak"), []byte("run log content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobProj, "docs", "jobs", "wood_oak", "session.log"), []byte("session log content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", pathWithRealGitOnly(t))
	gitProj := initGitRepo(t)
	runGitT(t, gitProj, "checkout", "-q", "-b", "feature/wood_test")
	if err := os.WriteFile(filepath.Join(gitProj, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitT(t, gitProj, "add", "feature.txt")
	runGitT(t, gitProj, "commit", "-q", "-m", "add the feature")
	runGitT(t, gitProj, "checkout", "-q", "main")

	reg := &Registry{entries: []Entry{entryFor(jobProj), entryFor(gitProj)}}
	token := envLines["MG_SERVE_TOKEN"]
	var audit strings.Builder
	srv := New(reg, "test-version", token, &audit)
	auth := "Bearer " + token

	// Every endpoint, plus a 401 (no token) and a 404 (unknown job) — the
	// error envelopes are response surfaces too.
	paths := []string{
		"/health",
		"/projects",
		"/projects/" + filepath.Base(jobProj) + "/jobs",
		"/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/files/brief.md",
		"/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/files/tasks.md",
		"/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/files/implementation.md",
		"/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/files/verdict.md",
		"/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/jdi",
		"/projects/" + filepath.Base(jobProj) + "/agents",
		"/projects/" + filepath.Base(gitProj) + "/jobs/wood_test/diff",
		"/projects/" + filepath.Base(gitProj) + "/jobs/wood_test/diff?full=1",
		"/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/jdi?bogus=1",
		"/projects/no-such-project/jobs",
		"/projects/" + filepath.Base(jobProj) + "/jobs/no-such-job/files/brief.md",
	}
	for _, p := range paths {
		rec := get(t, srv, p, auth)
		assertBodyHasNoCredentials(t, rec.Body.String(), envLines, envFile.String())
	}

	// Job two's mutating surface — every mutating endpoint's response or error
	// body (the 2xx success envelopes and the 4xx/5xx structured errors alike)
	// must never echo .env content. All of these run against the fake non-git
	// jobProj (gitProj has no docs/jobs, so its mutating routes resolve no
	// job), which exercises the resolution + error-envelope paths; the success
	// envelopes (create, edit-brief, orphans-list) carry only identifiers and
	// status text.
	mutating := []struct {
		method, path, body string
	}{
		// create: succeeds (non-git fallback) — the 201 envelope must not echo
		// credentials.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/jobs", `{"title":"Cred Check"}`},
		// create with a bad type: a 400 error envelope.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/jobs", `{"title":"X","type":"bogus"}`},
		// edit-brief: succeeds — the write + (non-git) warning envelope.
		{http.MethodPut, "/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/files/brief", "# Brief: Cred Check\n"},
		// launch-agent: 404 for an unknown agent (agentlist.Discover reads the
		// checkout's agents — see fakeCheckout above), never a credential echo.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/agents/no-such-agent", ""},
		// launch-jdi: 202 (the mg binary is stubbed), or a structured error.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/jdi", ""},
		// done: 409 verdict warning (the fake job has no verdict) — the
		// warning text must not echo credentials.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/done", ""},
		// push: non-git project → a structured 500 git error envelope.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/push", ""},
		// prune: docker absent → a structured 500 error envelope.
		{http.MethodPost, "/prune", ""},
		// orphans list: read-only, an empty (or populated) list envelope.
		{http.MethodGet, "/projects/" + filepath.Base(jobProj) + "/orphans", ""},
		// orphan delete: 404 for an unknown orphan — the error envelope.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/orphans/no-such-orphan/delete", ""},
		// session-log stream: a bad ?from is a 400 before any SSE body — the
		// error envelope (the stream itself is exercised in stream_test.go).
		{http.MethodGet, "/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/session-log/stream?from=-1", ""},
		// settings (job brother): the global profile GET/PUT and the project
		// settings GET/PUT. The PUTs write MANIGOT_PROFILE / manigot.json —
		// their envelopes carry only profile ids and ref names, and the 400s
		// (unknown profile, bad ref value) never echo .env content.
		{http.MethodGet, "/settings", ""},
		{http.MethodPut, "/settings", `{"profile":"zai"}`},
		{http.MethodPut, "/settings", `{"profile":"bogus-profile"}`},
		{http.MethodGet, "/projects/" + filepath.Base(jobProj) + "/settings", ""},
		{http.MethodPut, "/projects/" + filepath.Base(jobProj) + "/settings", `{"baseBranch":"develop","jobBranchPrefix":"jobs"}`},
		{http.MethodPut, "/projects/" + filepath.Base(jobProj) + "/settings", `{"baseBranch":"bad value"}`},
		// delete: resolves a real (non-git) job — the DeleteResult envelope.
		// Kept last: it removes wood_oak, so every earlier request still
		// resolves the job.
		{http.MethodPost, "/projects/" + filepath.Base(jobProj) + "/jobs/wood_oak/delete", ""},
	}
	stubJdiExe(t, "/bin/true")
	for _, m := range mutating {
		rec := request(t, srv, m.method, m.path, auth, m.body)
		assertBodyHasNoCredentials(t, rec.Body.String(), envLines, envFile.String())
	}

	// The 401 body (missing and wrong token) — an unauthenticated response
	// must not leak the token either.
	for _, badAuth := range []string{"", "Bearer wrong"} {
		rec := get(t, srv, "/projects", badAuth)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("bad auth %q: status = %d, want 401", badAuth, rec.Code)
		}
		assertBodyHasNoCredentials(t, rec.Body.String(), envLines, envFile.String())
	}

	// The audit log likewise never carries the Authorization header or the
	// token (the auth= field is the outcome classification only).
	if strings.Contains(audit.String(), token) {
		t.Errorf("audit log contains the token:\n%s", audit.String())
	}
	for _, line := range strings.Split(audit.String(), "\n") {
		if strings.Contains(line, "Authorization") || strings.Contains(line, "Bearer ") {
			t.Errorf("audit log echoes the Authorization header: %q", line)
		}
	}
}

// assertBodyHasNoCredentials fails the test if body contains any planted
// credential value or any full line of the .env file.
func assertBodyHasNoCredentials(t *testing.T, body string, envLines map[string]string, envFileContent string) {
	t.Helper()
	for _, key := range credentialKeys {
		if strings.Contains(body, envLines[key]) {
			t.Errorf("response leaks the value of %s:\n%s", key, body)
		}
	}
	for _, line := range strings.Split(envFileContent, "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(body, line) {
			t.Errorf("response contains a full .env line %q:\n%s", line, body)
		}
	}
}

// TestCredentialValuesDoNotAppearInErrorMessages: error envelopes carry only
// validated identifiers — never anything that could echo a credential.
func TestCredentialValuesDoNotAppearInErrorMessages(t *testing.T) {
	t.Setenv("MG_SERVE_TOKEN", "fixture-token-12345")
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "fixture-token-12345", nil)

	// The error envelope shape is exactly {"error": "..."} — nothing else.
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/nope/files/brief.md", "Bearer fixture-token-12345")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var envelope map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("error body not a JSON envelope: %v (%s)", err, rec.Body.String())
	}
	if len(envelope) != 1 || envelope["error"] == "" {
		t.Errorf("error envelope = %v, want exactly one error key", envelope)
	}
	if strings.Contains(rec.Body.String(), "fixture-token-12345") {
		t.Errorf("error body leaks the token: %s", rec.Body.String())
	}
}
