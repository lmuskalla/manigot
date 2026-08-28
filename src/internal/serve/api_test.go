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

// fakeJobProject creates a project root at a temp dir with a job dir
// docs/jobs/<name>/ carrying a brief.md with the given content, plus the
// named extra files (each written with a one-line body). Returns the root.
func fakeJobProject(t *testing.T, name, brief string, extraFiles ...string) string {
	t.Helper()
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", name)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range extraFiles {
		if err := os.WriteFile(filepath.Join(jobDir, f), []byte("content of "+f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// minimalBrief returns a brief.md body with the given frontmatter fields.
func minimalBrief(title, id, status, typ, date string) string {
	return "# Brief: " + title + "\n\n" +
		"status: " + status + "\n" +
		"type: " + typ + "\n" +
		"id: " + id + "\n" +
		"date: " + date + "\n" +
		"author: Test\n\n## What\n\nA test job.\n"
}

// TestHandleProjectsListsRegisteredRoots: /projects returns the registry data
// — each root's absolute path and base name — and nothing else.
func TestHandleProjectsListsRegisteredRoots(t *testing.T) {
	root := t.TempDir()
	reg := &Registry{projects: []string{filepath.Clean(root)}}
	srv := New(reg, "test-version", "", nil)

	rec := get(t, srv, "/projects", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Projects []projectRow `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if len(body.Projects) != 1 {
		t.Fatalf("projects = %v, want 1", body.Projects)
	}
	if body.Projects[0].Path != filepath.Clean(root) || body.Projects[0].Name != filepath.Base(root) {
		t.Errorf("project row = %+v, want path=%s name=%s", body.Projects[0], filepath.Clean(root), filepath.Base(root))
	}
}

// TestHandleProjectsEmptyRegistry: an empty registry lists no projects — a
// valid, normal response.
func TestHandleProjectsEmptyRegistry(t *testing.T) {
	srv := New(&Registry{}, "test-version", "", nil)
	rec := get(t, srv, "/projects", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"projects":[]`) {
		t.Errorf("body = %s, want an empty projects list", rec.Body.String())
	}
}

// TestHandleProjectJobsInfoDesign: one row per job in Discover's sort order
// (date descending), carrying the TUI's info-design fields: id / status /
// stage / type / date / title.
func TestHandleProjectJobsInfoDesign(t *testing.T) {
	root := fakeJobProject(t, "older_job", minimalBrief("Older", "old", "open", "feature", "2026-08-01"), "tasks.md")
	_ = fakeJobProjectInto(t, root, "newer_job", minimalBrief("Newer", "new", "open", "fix", "2026-08-10"), "tasks.md", "implementation.md")

	reg := &Registry{projects: []string{filepath.Clean(root)}}
	srv := New(reg, "test-version", "", nil)

	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Jobs []jobRow `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if len(body.Jobs) != 2 {
		t.Fatalf("jobs = %d, want 2:\n%s", len(body.Jobs), rec.Body.String())
	}

	// Newest first: "newer_job" (2026-08-10) before "older_job" (2026-08-01).
	first := body.Jobs[0]
	if first.Name != "newer_job" {
		t.Errorf("first job = %q, want newer_job (date-sorted)", first.Name)
	}
	if first.ID != "new" || first.Status != "open" || first.Type != "fix" || first.Date != "2026-08-10" || first.Title != "Newer" {
		t.Errorf("newer_job row = %+v, want the brief's frontmatter fields", first)
	}
	// newer_job has implementation.md but no verdict → stage review.
	if first.Stage != "review" {
		t.Errorf("newer_job stage = %q, want review", first.Stage)
	}
	// older_job has tasks.md but no implementation.md → stage implement.
	second := body.Jobs[1]
	if second.Name != "older_job" || second.Stage != "implement" {
		t.Errorf("older_job row = %+v, want name=older_job stage=implement", second)
	}
}

// fakeJobProjectInto adds another job dir into an existing project root —
// the second half of a two-job project (fakeJobProject creates its own root).
func fakeJobProjectInto(t *testing.T, root, name, brief string, extraFiles ...string) string {
	t.Helper()
	jobDir := filepath.Join(root, "docs", "jobs", name)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range extraFiles {
		if err := os.WriteFile(filepath.Join(jobDir, f), []byte("content of "+f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestHandleProjectJobsJDIState: a job with a mg-jdi status sidecar carries
// its state/agent/updated in the row; one without carries no jdi field.
func TestHandleProjectJobsJDIState(t *testing.T) {
	root := fakeJobProject(t, "driven_job", minimalBrief("Driven", "drv", "open", "feature", "2026-08-01"))

	// Write a jdi status sidecar for the job (job.WriteJDIStatus).
	if err := writeJDIStatusForTest(t, root, "driven_job"); err != nil {
		t.Fatal(err)
	}

	reg := &Registry{projects: []string{filepath.Clean(root)}}
	srv := New(reg, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Jobs []jobRow `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if len(body.Jobs) != 1 || body.Jobs[0].JDI == nil {
		t.Fatalf("jdi state missing: %s", rec.Body.String())
	}
	if body.Jobs[0].JDI.State != "stopped:finished" || body.Jobs[0].JDI.Agent != "reviewer" {
		t.Errorf("jdi row = %+v, want the written sidecar state", body.Jobs[0].JDI)
	}
	if body.Jobs[0].JDI.Updated == "" {
		t.Errorf("jdi row updated = empty, want RFC3339")
	}
}

// TestHandleProjectJobsNoJDIState: a job mg-jdi never drove has no jdi field
// in its row.
func TestHandleProjectJobsNoJDIState(t *testing.T) {
	root := fakeJobProject(t, "plain_job", minimalBrief("Plain", "pln", "open", "feature", "2026-08-01"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs", "")
	var body struct {
		Jobs []jobRow `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if len(body.Jobs) != 1 || body.Jobs[0].JDI != nil {
		t.Errorf("job row = %+v, want nil jdi for an undriven job", body.Jobs[0])
	}
}

// TestHandleJobFileServesRawMarkdown: a whitelisted file comes back as raw
// markdown text with the markdown content type.
func TestHandleJobFileServesRawMarkdown(t *testing.T) {
	root := fakeJobProject(t, "some_job", minimalBrief("Some", "som", "open", "feature", "2026-08-01"), "tasks.md")
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)

	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/some_job/files/brief.md", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Errorf("Content-Type = %q, want text/markdown", ct)
	}
	if !strings.Contains(rec.Body.String(), "# Brief: Some") {
		t.Errorf("body = %q, want the raw brief content", rec.Body.String())
	}
}

// TestHandleJobFileMissingIs404: a whitelisted file that has not been written
// yet (e.g. verdict.md before review) is a normal 404, not an error.
func TestHandleJobFileMissingIs404(t *testing.T) {
	root := fakeJobProject(t, "some_job", minimalBrief("Some", "som", "open", "feature", "2026-08-01"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)
	if rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/some_job/files/verdict.md", ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing verdict.md: status = %d, want 404", rec.Code)
	}
}

// TestHandleJobFileNonWhitelistedIs404: the file whitelist (brief|tasks|
// implementation|verdict) is airtight — a session.log, an encoded traversal,
// or any other file under the job dir is never served. (Raw `..` segments in
// the URL path never even reach the handler — ServeMux's own path
// sanitization redirects them — so the encoded forms are the ones that must
// be rejected here.)
func TestHandleJobFileNonWhitelistedIs404(t *testing.T) {
	root := fakeJobProject(t, "some_job", minimalBrief("Some", "som", "open", "feature", "2026-08-01"), "session.log")
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)

	for _, file := range []string{"session.log", "run.log", "brief", "README.md", "..%2fbrief.md", "..%2f..%2fetc%2fpasswd", "%2e%2e%2fbrief.md", "brief.md%00"} {
		rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/some_job/files/"+file, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("file %q: status = %d, want 404", file, rec.Code)
		}
	}

	// Raw `..` in the path is redirected by ServeMux's sanitization — still
	// never served, never read.
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/some_job/files/../brief.md", "")
	if rec.Code < 300 || rec.Code >= 400 {
		t.Errorf("raw .. file segment: status = %d, want a 3xx sanitization redirect", rec.Code)
	}
}

// TestHandleProjectJobsUnknownProjectIs404: an unregistered project segment
// (and an invalid one) is a 404 — nothing is ever resolved outside the
// registry. Encoded traversal reaches the handler and is rejected there; raw
// `..` is redirected by ServeMux's own sanitization first.
func TestHandleProjectJobsUnknownProjectIs404(t *testing.T) {
	root := fakeJobProject(t, "some_job", minimalBrief("Some", "som", "open", "feature", "2026-08-01"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)

	for _, seg := range []string{"no-such-project", "%2e%2e", "%2e", "%2fetc", "a%2fb", "a%5cb", "a%00b", "..%2f..%2fetc%2fpasswd"} {
		rec := get(t, srv, "/projects/"+seg+"/jobs", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("project segment %q: status = %d, want 404", seg, rec.Code)
		}
	}

	// Raw `..` and `.` segments are cleaned to a redirect by ServeMux before
	// routing — still never resolved against anything.
	for _, seg := range []string{"..", "."} {
		rec := get(t, srv, "/projects/"+seg+"/jobs", "")
		if rec.Code < 300 || rec.Code >= 400 {
			t.Errorf("raw project segment %q: status = %d, want a 3xx sanitization redirect", seg, rec.Code)
		}
	}
}

// TestHandleJobFileUnknownJobIs404: an unknown job segment (no ID, name or
// unique prefix match) is a 404.
func TestHandleJobFileUnknownJobIs404(t *testing.T) {
	root := fakeJobProject(t, "some_job", minimalBrief("Some", "som", "open", "feature", "2026-08-01"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/no-such-job/files/brief.md", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestHandleJobResolvesByIDNameAndPrefix: the job segment resolves by ID,
// then by name, then by unique name prefix.
func TestHandleJobResolvesByIDNameAndPrefix(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	_ = fakeJobProjectInto(t, root, "iron_steel", minimalBrief("Steel", "irn", "open", "fix", "2026-08-02"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)

	base := "/projects/" + filepath.Base(root) + "/jobs/"
	for _, seg := range []string{"wod", "wood_oak", "wood", "iron", "irn"} {
		rec := get(t, srv, base+seg+"/files/brief.md", "")
		if rec.Code != http.StatusOK {
			t.Errorf("job segment %q: status = %d, want 200 (body %s)", seg, rec.Code, rec.Body.String())
		}
	}
}

// TestHandleJobAmbiguousPrefixIs409: a job segment matching several job names
// by prefix is a 409 with the CLI's ambiguity wording.
func TestHandleJobAmbiguousPrefixIs409(t *testing.T) {
	root := fakeJobProject(t, "wood_a", minimalBrief("A", "aaa", "open", "feature", "2026-08-01"))
	_ = fakeJobProjectInto(t, root, "wood_b", minimalBrief("B", "bbb", "open", "feature", "2026-08-02"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)

	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/wood/files/brief.md", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ambiguous") {
		t.Errorf("body = %s, want the ambiguity wording", rec.Body.String())
	}
}

// TestServeShutdownSeam pins the httptest-able serve loop: Serve on a bound
// listener answers requests until Shutdown drains and returns.
func TestServeShutdownSeam(t *testing.T) {
	srv := New(&Registry{}, "test-version", "", nil)
	ln, err := netListen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()

	// The server answers on the bound listener.
	base := "http://" + ln.Addr().String()
	resp, err := httpGet(base + "/projects")
	if err != nil {
		t.Fatalf("GET before shutdown: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Shutdown drains and the serve loop returns http.ErrServerClosed.
	if err := srv.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := <-done; err != nil && err != http.ErrServerClosed {
		t.Errorf("Serve returned %v, want ErrServerClosed", err)
	}
}

// --- TASK-4: jdi / diff / agents endpoints ----------------------------------

// TestHandleJobJDIStatusAndLogs: the jdi endpoint serves the mg-jdi status
// sidecar, the run.log tail, and the job's session.log tail.
func TestHandleJobJDIStatusAndLogs(t *testing.T) {
	root := fakeJobProject(t, "driven_job", minimalBrief("Driven", "drv", "open", "feature", "2026-08-01"))
	if err := writeJDIStatusForTest(t, root, "driven_job"); err != nil {
		t.Fatal(err)
	}
	// run.log under .manigot/jdi-status/<name>/.
	logDir := job.JDIStatusDir(root, "driven_job")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(job.JDIRunLogPath(root, "driven_job"), []byte("run log line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// session.log in the job's own dir.
	if err := os.WriteFile(filepath.Join(root, "docs", "jobs", "driven_job", "session.log"), []byte("session log line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/driven_job/jdi", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body jobJDIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Status == nil || body.Status.State != "stopped:finished" || body.Status.Agent != "reviewer" {
		t.Errorf("status = %+v, want the written sidecar", body.Status)
	}
	if body.RunLog == nil || !strings.Contains(*body.RunLog, "run log line") {
		t.Errorf("runLog = %v, want the run.log tail", body.RunLog)
	}
	if body.SessionLog == nil || !strings.Contains(*body.SessionLog, "session log line") {
		t.Errorf("sessionLog = %v, want the session.log tail", body.SessionLog)
	}
}

// TestHandleJobJDIUndrivenJobIsNulls: a job mg-jdi never drove returns nulls
// (no status, no run.log, no session.log) — a normal "no run yet", not an
// error.
func TestHandleJobJDIUndrivenJobIsNulls(t *testing.T) {
	root := fakeJobProject(t, "plain_job", minimalBrief("Plain", "pln", "open", "feature", "2026-08-01"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/plain_job/jdi", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body jobJDIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Status != nil || body.RunLog != nil || body.SessionLog != nil {
		t.Errorf("undriven job = %+v, want all-null fields", body)
	}
}

// TestHandleJobJDIEmptyLogsPresent: an existing-but-empty run.log is "there is
// a run happening, nothing logged yet" — an empty string, not null.
func TestHandleJobJDIEmptyLogsPresent(t *testing.T) {
	root := fakeJobProject(t, "driven_job", minimalBrief("Driven", "drv", "open", "feature", "2026-08-01"))
	if err := writeJDIStatusForTest(t, root, "driven_job"); err != nil {
		t.Fatal(err)
	}
	logDir := job.JDIStatusDir(root, "driven_job")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(job.JDIRunLogPath(root, "driven_job"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/driven_job/jdi", "")
	var body jobJDIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.RunLog == nil || *body.RunLog != "" {
		t.Errorf("runLog = %v, want an empty present string", body.RunLog)
	}
}

// TestHandleJobJDIAllMissingStatusNoJDI — the jdi endpoint's job resolution is
// the same choke point: an unknown job is a 404.
func TestHandleJobJDIUnknownJobIs404(t *testing.T) {
	root := fakeJobProject(t, "plain_job", minimalBrief("Plain", "pln", "open", "feature", "2026-08-01"))
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/nope/jdi", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestHandleJobDiffQuickEyeball: the diff endpoint returns the quick eyeball
// (log + stat) for the resolved job branch against the base branch — the same
// two calls mg diff makes. Needs a real git repo (PATH restricted to real
// git, since the session shim refuses git init).
func TestHandleJobDiffQuickEyeball(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := initGitRepo(t)
	runGitT(t, dir, "checkout", "-q", "-b", "feature/wood_test")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitT(t, dir, "add", "feature.txt")
	runGitT(t, dir, "commit", "-q", "-m", "add the feature")
	runGitT(t, dir, "checkout", "-q", "main")

	srv := New(&Registry{projects: []string{filepath.Clean(dir)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(dir)+"/jobs/wood_test/diff", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body jobDiffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Log == nil || !strings.Contains(*body.Log, "add the feature") {
		t.Errorf("log = %v, want the branch's commit", body.Log)
	}
	if body.Stat == nil || !strings.Contains(*body.Stat, "feature.txt") {
		t.Errorf("stat = %v, want the changed file", body.Stat)
	}
	if body.Patch != nil {
		t.Errorf("patch = %v, want nil for the quick eyeball", body.Patch)
	}
}

// TestHandleJobDiffFullPatch: ?full=1 returns the complete patch.
func TestHandleJobDiffFullPatch(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := initGitRepo(t)
	runGitT(t, dir, "checkout", "-q", "-b", "feature/wood_test")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitT(t, dir, "add", "feature.txt")
	runGitT(t, dir, "commit", "-q", "-m", "add the feature")
	runGitT(t, dir, "checkout", "-q", "main")

	srv := New(&Registry{projects: []string{filepath.Clean(dir)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(dir)+"/jobs/wood_test/diff?full=1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body jobDiffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Patch == nil || !strings.Contains(*body.Patch, "+feature") {
		t.Errorf("patch = %v, want the full patch", body.Patch)
	}
	if body.Log != nil || body.Stat != nil {
		t.Errorf("log/stat = %v/%v, want nil for ?full=1", body.Log, body.Stat)
	}
}

// TestHandleJobDiffResolvesBaseBranchFromConfig: the base branch comes from
// the project's configured baseBranch (project.Load), exactly as mg diff
// resolves it — proven by a repo where main and the configured base differ,
// so the wrong base would show an extra commit.
func TestHandleJobDiffResolvesBaseBranchFromConfig(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := initGitRepo(t)
	// trunk = main + one extra commit; baseBranch is configured to trunk.
	runGitT(t, dir, "checkout", "-q", "-b", "trunk")
	if err := os.WriteFile(filepath.Join(dir, "trunk.txt"), []byte("trunk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitT(t, dir, "add", "trunk.txt")
	runGitT(t, dir, "commit", "-q", "-m", "trunk work")
	// The job branch is cut from trunk and adds one commit of its own.
	runGitT(t, dir, "checkout", "-q", "-b", "feature/wood_test")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitT(t, dir, "add", "feature.txt")
	runGitT(t, dir, "commit", "-q", "-m", "job work")
	runGitT(t, dir, "checkout", "-q", "main")
	// Configure baseBranch=trunk.
	if err := os.MkdirAll(filepath.Join(dir, ".manigot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".manigot", "manigot.json"), []byte(`{"baseBranch": "trunk"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(&Registry{projects: []string{filepath.Clean(dir)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(dir)+"/jobs/wood_test/diff", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body jobDiffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	// Only the job's own commit — had the base fallen back to main, the
	// trunk commit would appear too.
	if body.Log == nil || strings.Contains(*body.Log, "trunk work") || !strings.Contains(*body.Log, "job work") {
		t.Errorf("log = %v, want only the job's commit against base trunk", body.Log)
	}
}

// TestHandleJobDiffUnknownJobIs404: an unresolvable job segment is a 404 with
// the CLI's not-found wording.
func TestHandleJobDiffUnknownJobIs404(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := initGitRepo(t)
	srv := New(&Registry{projects: []string{filepath.Clean(dir)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(dir)+"/jobs/nope/diff", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not found among local branches") {
		t.Errorf("body = %s, want the CLI's not-found wording", rec.Body.String())
	}
}

// TestHandleJobDiffAmbiguousIs409: a job segment matching several branches by
// prefix is a 409 with the CLI's ambiguity wording.
func TestHandleJobDiffAmbiguousIs409(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := initGitRepo(t)
	runGitT(t, dir, "checkout", "-q", "-b", "feature/wood_a")
	runGitT(t, dir, "checkout", "-q", "main")
	runGitT(t, dir, "checkout", "-q", "-b", "feature/wood_b")
	runGitT(t, dir, "checkout", "-q", "main")

	srv := New(&Registry{projects: []string{filepath.Clean(dir)}}, "test-version", "", nil)
	rec := get(t, srv, "/projects/"+filepath.Base(dir)+"/jobs/wood/diff", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ambiguous") {
		t.Errorf("body = %s, want the CLI's ambiguity wording", rec.Body.String())
	}
}

// TestHandleProjectAgentsNameAndDescription: the agents endpoint lists name +
// description only, from the same agentlist.Discover the mg agents picker
// uses.
func TestHandleProjectAgentsNameAndDescription(t *testing.T) {
	fakeCheckout(t, map[string]string{
		"analyst":  "name: analyst\ndescription: Breaks requests into tasks.\n",
		"reviewer": "name: reviewer\ndescription: Reviews changes.\n",
	})
	root := t.TempDir() // no docs/ needed — agents are global
	srv := New(&Registry{projects: []string{filepath.Clean(root)}}, "test-version", "", nil)

	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/agents", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body agentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if len(body.Agents) != 2 {
		t.Fatalf("agents = %d, want 2:\n%s", len(body.Agents), rec.Body.String())
	}
	// Sorted by name: analyst first.
	if body.Agents[0].Name != "analyst" || !strings.Contains(body.Agents[0].Description, "tasks") {
		t.Errorf("agent[0] = %+v, want analyst with its description", body.Agents[0])
	}
	if body.Agents[1].Name != "reviewer" {
		t.Errorf("agent[1] = %+v, want reviewer", body.Agents[1])
	}
	// No full agent-file contents leak into the response.
	if strings.Contains(rec.Body.String(), "name: analyst") {
		t.Errorf("response leaks the raw agent file: %s", rec.Body.String())
	}
}

// --- TASK-5: /health ---------------------------------------------------------

// TestHandleHealthReportsVersionImageAndProfiles: /health reports the version,
// docker image presence, and per-profile readiness — identifiers and booleans
// only, never credential values. The docker presence and readiness booleans
// depend on the machine the test runs on (session.ImagePresent degrades to
// false when docker is missing; profile readiness is an env read), so the
// shape — every profile present, every field present — is what is pinned.
func TestHandleHealthReportsVersionImageAndProfiles(t *testing.T) {
	srv := New(&Registry{}, "1.2.3-test", "", nil)
	rec := get(t, srv, "/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Version != "1.2.3-test" {
		t.Errorf("version = %q, want the version passed into New", body.Version)
	}
	// imagePresent must be a real boolean (present in the JSON), whatever its
	// value on this machine.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["imagePresent"]; !ok {
		t.Errorf("imagePresent missing from /health: %s", rec.Body.String())
	}

	if len(body.Profiles) != 5 {
		t.Fatalf("profiles = %d, want 5 (claude-pro, zai, opencode-go, opencode-zen, opencode-zen-free):\n%s",
			len(body.Profiles), rec.Body.String())
	}
	seen := map[string]bool{}
	for _, p := range body.Profiles {
		if p.ID == "" {
			t.Errorf("profile row with empty id: %+v", p)
		}
		seen[p.ID] = true
	}
	for _, id := range []string{"claude-pro", "zai", "opencode-go", "opencode-zen", "opencode-zen-free"} {
		if !seen[id] {
			t.Errorf("profile %q missing from /health", id)
		}
	}
}

// TestHandleHealthRequiresAuth: /health is inside the same auth boundary as
// every other endpoint — with a token configured, it is a 401 without one.
func TestHandleHealthRequiresAuth(t *testing.T) {
	srv := New(&Registry{}, "1.2.3-test", "sekrit-token", nil)
	if rec := get(t, srv, "/health", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", rec.Code)
	}
	if rec := get(t, srv, "/health", "Bearer sekrit-token"); rec.Code != http.StatusOK {
		t.Errorf("with token: status = %d, want 200", rec.Code)
	}
}
