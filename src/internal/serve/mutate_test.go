package serve

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/launch"
)

// fixedID forces CreateJob to use the given id — the deterministic-id
// counterpart of internal/job's own test helper, needed here so endpoint tests
// know the exact branch/job name a created job will carry.
func fixedID(id string) func() (string, error) {
	return func() (string, error) { return id, nil }
}

// yesConfirm answers every prompt with yes — the same pre-approved confirm the
// HTTP delete endpoint relies on.
func yesConfirm(prompt string) (bool, error) { return true, nil }

// gitJobProject creates a real git project (initGitRepo: main + initial
// commit) with a docs/jobs dir, then creates a job with the fixed id "ab12cd"
// via job.CreateJob (branch feature/ab12cd_roundtrip-job, real worktree).
// Requires real git on PATH — the caller must t.Setenv("PATH",
// pathWithRealGitOnly(t)) first. Returns the project root and the create
// result.
func gitJobProject(t *testing.T) (root string, res job.CreateResult) {
	t.Helper()
	root = initGitRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	res, err := job.CreateJob(root, "Roundtrip Job", job.CreateOptions{RandomID: fixedID("ab12cd")}, &out)
	if err != nil {
		t.Fatalf("CreateJob: %v\n%s", err, out.String())
	}
	return root, res
}

// --- TASK-3: POST /projects/{project}/jobs -----------------------------------

// TestHandleCreateJobGitProject: a real git project gets a real branch +
// worktree, reported in the response.
func TestHandleCreateJobGitProject(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root := initGitRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs", "", `{"title":"Add Gallery Block"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var body createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Job.ID == "" || body.Job.Name == "" {
		t.Errorf("job row missing id/name: %+v", body.Job)
	}
	if body.Job.Title != "Add Gallery Block" || body.Job.Type != "feature" || body.Job.Status != "open" {
		t.Errorf("job row = %+v, want title/type/status", body.Job)
	}
	if body.Branch == "" || !strings.HasPrefix(body.Branch, "feature/") {
		t.Errorf("branch = %q, want feature/...", body.Branch)
	}
	if body.WorktreePath == "" {
		t.Errorf("worktreePath empty for a git project")
	} else if _, err := os.Stat(body.WorktreePath); err != nil {
		t.Errorf("worktreePath %q does not exist: %v", body.WorktreePath, err)
	}

	// The created job is discoverable through the API.
	rec = get(t, srv, "/projects/"+base+"/jobs", "")
	var list struct {
		Jobs []jobRow `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list not JSON: %v", err)
	}
	found := false
	for _, j := range list.Jobs {
		if j.Name == body.Job.Name {
			found = true
		}
	}
	if !found {
		t.Errorf("created job %q not listed after creation:\n%s", body.Job.Name, rec.Body.String())
	}

	// Clean up the created worktree + branch.
	if _, err := job.DeleteJob(root, body.Job.Name, yesConfirm, io.Discard); err != nil {
		t.Errorf("cleanup DeleteJob: %v", err)
	}
}

// TestHandleCreateJobNonGitProject: a non-git project takes the plain-directory
// fallback — branch "(no git)", no worktree path.
func TestHandleCreateJobNonGitProject(t *testing.T) {
	root := fakeJobProject(t, "existing_job", minimalBrief("Existing", "ex", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs", "", `{"title":"Plain Job"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var body createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Branch != "(no git)" {
		t.Errorf("branch = %q, want (no git)", body.Branch)
	}
	if body.WorktreePath != "" {
		t.Errorf("worktreePath = %q, want empty for a non-git project", body.WorktreePath)
	}
	// The job directory exists under docs/jobs.
	if _, err := os.Stat(filepath.Join(root, "docs", "jobs", body.Job.Name)); err != nil {
		t.Errorf("created job dir missing: %v", err)
	}
}

// TestHandleCreateJobValidation: bad type and empty title are clean 400s —
// not CreateJob's own "Invalid type" text surfacing as a 500.
func TestHandleCreateJobValidation(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	if rec := post(t, srv, "/projects/"+base+"/jobs", "", `{"title":"X","type":"bogus"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad type: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if rec := post(t, srv, "/projects/"+base+"/jobs", "", `{"type":"feature"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("missing title: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if rec := post(t, srv, "/projects/no-such-project/jobs", "", `{"title":"X"}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown project: status = %d, want 404", rec.Code)
	}
}

// --- TASK-4: PUT /projects/{project}/jobs/{job}/files/brief ------------------

// TestHandleEditBriefReplacesAndCommits: a git-project job's brief.md is
// replaced AND committed in the job's own worktree (the sweep commit), so the
// edit never sits as an uncommitted change.
func TestHandleEditBriefReplacesAndCommits(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root, res := gitJobProject(t)
	defer os.RemoveAll(res.WorktreePath)
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	newBrief := "# Brief: Edited\n\nstatus: open\ntype: feature\nid: ab12cd\n\n## What\n\nChanged.\n"
	rec := put(t, srv, "/projects/"+base+"/jobs/ab12cd/files/brief", "", newBrief)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body editBriefResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Status != "ok" {
		t.Errorf("status field = %q, want ok", body.Status)
	}

	// The file on disk (in the job's worktree) was replaced.
	got, err := os.ReadFile(filepath.Join(res.Job.Dir, "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != newBrief {
		t.Errorf("brief.md = %q, want the PUT body", string(got))
	}

	// The edit was committed in the job's worktree — the sweep commit.
	last := gitCmd(t, res.WorktreePath, "log", "-1", "--format=%s")
	if !strings.Contains(last, "[ab12cd] chore: commit all") {
		t.Errorf("worktree HEAD = %q, want the [ab12cd] chore: commit all sweep commit", last)
	}
}

// TestHandleEditBriefNonGitProject: on a non-git project the write still
// succeeds; only the commit cannot happen (no worktree to resolve) — reported
// as a warning, not a failure.
func TestHandleEditBriefNonGitProject(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	newBrief := "# Brief: Changed\n\nstatus: open\n"
	rec := put(t, srv, "/projects/"+base+"/jobs/wood_oak/files/brief", "", newBrief)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "jobs", "wood_oak", "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != newBrief {
		t.Errorf("brief.md = %q, want the PUT body", string(got))
	}
}

// TestHandleEditBriefValidation: an empty body and an unknown job are clean
// 400/404s.
func TestHandleEditBriefValidation(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	if rec := put(t, srv, "/projects/"+base+"/jobs/wood_oak/files/brief", "", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("empty body: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if rec := put(t, srv, "/projects/"+base+"/jobs/no-such-job/files/brief", "", "# Brief: X\n"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown job: status = %d, want 404", rec.Code)
	}
}

// --- TASK-5: POST /projects/{project}/jobs/{job}/agents/{agent} --------------

// TestHandleLaunchAgentDetached: a known agent returns 202 immediately (the
// run happens in a background goroutine) and writes a session.log section for
// the job — the caller's only window onto the detached run.
func TestHandleLaunchAgentDetached(t *testing.T) {
	fakeCheckout(t, map[string]string{
		"analyst":  "name: analyst\ndescription: Breaks requests into tasks.\n",
		"reviewer": "name: reviewer\ndescription: Reviews changes.\n",
	})
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/wood_oak/agents/analyst", "", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %s)", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["status"] != "started" || body["agent"] != "analyst" || body["job"] != "wood_oak" {
		t.Errorf("body = %v, want status=started agent=analyst job=wood_oak", body)
	}

	// The detached run opens a session.log section in the job dir (the
	// goroutine writes its section header immediately, before the invocation
	// itself runs — which will fail fast in this test env without docker).
	logPath := filepath.Join(root, "docs", "jobs", "wood_oak", "session.log")
	waitFor(t, func() bool {
		data, err := os.ReadFile(logPath)
		return err == nil && strings.Contains(string(data), "=== ")
	}, "session.log section header")
}

// TestHandleLaunchAgentUnknownAndBadProfile: an unknown agent is a fast 404
// (validated against agentlist.Discover before any container spin-up); a bad
// profile is a 400.
func TestHandleLaunchAgentUnknownAndBadProfile(t *testing.T) {
	fakeCheckout(t, map[string]string{
		"analyst": "name: analyst\ndescription: Breaks requests into tasks.\n",
	})
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	if rec := post(t, srv, "/projects/"+base+"/jobs/wood_oak/agents/no-such-agent", "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown agent: status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	if rec := post(t, srv, "/projects/"+base+"/jobs/wood_oak/agents/analyst", "", `{"profile":"bogus-profile"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad profile: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
}

// --- TASK-6: POST /projects/{project}/jobs/{job}/jdi -------------------------

// stubJdiExe points the launch package's executable lookup at a harmless
// command for the duration of a test — launch.Jdi spawns whatever JdiExe
// returns with the mg-jdi argv, and the default (os.Executable()) would be the
// test binary itself, recursively re-running the whole suite. Mirrors
// internal/launch's own stubJdiExe.
func stubJdiExe(t *testing.T, stub string) {
	t.Helper()
	old := launch.JdiExe
	launch.JdiExe = func() (string, error) { return stub, nil }
	t.Cleanup(func() { launch.JdiExe = old })
}

// TestHandleLaunchJDIDetached: launching mg jdi detached returns 202; an
// already-running mg-jdi (per the status sidecar) is a 409.
func TestHandleLaunchJDIDetached(t *testing.T) {
	stubJdiExe(t, "/bin/true") // never spawn the real mg (or the test binary)
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/wood_oak/jdi", "", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %s)", rec.Code, rec.Body.String())
	}

	// Now mark the job as already running → the next launch is a 409.
	if err := job.WriteJDIStatus(root, "wood_oak", job.JDIRunning, "reviewer"); err != nil {
		t.Fatal(err)
	}
	rec = post(t, srv, "/projects/"+base+"/jobs/wood_oak/jdi", "", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("already-running launch: status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
}

// --- TASK-7: POST /projects/{project}/jobs/{job}/done ------------------------

// TestHandleDoneJobVerdictWarningRequiresForce: a job without an approved
// verdict is refused with a 409 echoing the CLI's warning text unless the
// caller passes {"force": true}.
func TestHandleDoneJobVerdictWarningRequiresForce(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root, res := gitJobProject(t)
	defer os.RemoveAll(res.WorktreePath)
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/ab12cd/done", "", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Warning:") {
		t.Errorf("409 body missing the verdict warning: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "force") {
		t.Errorf("409 body missing the force hint: %s", rec.Body.String())
	}
}

// TestHandleDoneJobForceFinishes: with {"force": true} the done endpoint runs
// FinishJob to completion — archive, squash merge, branch delete, worktree
// removal.
func TestHandleDoneJobForceFinishes(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root, res := gitJobProject(t)
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/ab12cd/done", "", `{"force":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body doneResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.JobName != "ab12cd_roundtrip-job" || body.Branch != "feature/ab12cd_roundtrip-job" || body.BaseBranch != "main" {
		t.Errorf("doneResponse = %+v", body)
	}
	// Branch deleted, worktree removed, main back on main.
	if ok, _ := gitExists(root, "feature/ab12cd_roundtrip-job"); ok {
		t.Error("job branch still exists after done")
	}
	if _, err := os.Stat(res.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("job worktree still exists after done")
	}
	if cur := gitCmd(t, root, "rev-parse", "--abbrev-ref", "HEAD"); cur != "main" {
		t.Errorf("main worktree on %q, want main", cur)
	}
}

// TestHandleDoneJobConflictIsStructured409: a squash-merge conflict under the
// done endpoint returns a structured 409 (ErrSquashMergeConflict) — no
// automatic rollback, no git-solver handoff — leaving the main worktree in its
// conflicted state for an explicit follow-up.
func TestHandleDoneJobConflictIsStructured409(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root, res := gitJobProject(t)
	defer os.RemoveAll(res.WorktreePath)
	// Both sides modify docs/jobs/.gitkeep differently → the squash merge
	// conflicts.
	writeTestFile(t, filepath.Join(root, "docs", "jobs", ".gitkeep"), "main side\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "main side change")
	writeTestFile(t, filepath.Join(res.WorktreePath, "docs", "jobs", ".gitkeep"), "job side\n")
	gitCmd(t, res.WorktreePath, "add", "-A")
	gitCmd(t, res.WorktreePath, "commit", "-q", "-m", "job side change")

	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)
	rec := post(t, srv, "/projects/"+base+"/jobs/ab12cd/done", "", `{"force":true}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "squash merge conflict") {
		t.Errorf("409 body missing the conflict marker: %s", rec.Body.String())
	}
	// The 409 must state explicitly that the job is already archived-and-
	// committed in its own worktree (the task's requirement that the response
	// not imply "nothing happened") — a naive retry of done would otherwise
	// fail confusingly with "brief.md not found" once the job dir is archived.
	if !strings.Contains(rec.Body.String(), "already archived") {
		t.Errorf("409 body must state the job is already archived: %s", rec.Body.String())
	}
	// The conflict is left in place (no automatic rollback).
	if u := gitCmd(t, root, "ls-files", "-u"); u == "" {
		t.Errorf("no unmerged entries left in place — the done endpoint must not auto-rollback")
	}
}

// --- TASK-8: POST /projects/{project}/jobs/{job}/delete ----------------------

// TestHandleDeleteJobGitProject: deleting a git-project job removes its
// worktree and branch, reporting the DeleteResult.
func TestHandleDeleteJobGitProject(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root, res := gitJobProject(t)
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/ab12cd/delete", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body deleteJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.JobName != "ab12cd_roundtrip-job" || body.Branch != "feature/ab12cd_roundtrip-job" {
		t.Errorf("deleteJobResponse = %+v", body)
	}
	if _, err := os.Stat(res.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree still exists after delete")
	}
	if ok, _ := gitExists(root, "feature/ab12cd_roundtrip-job"); ok {
		t.Error("branch still exists after delete")
	}
}

// TestHandleDeleteJobNonGitProject: deleting a non-git job is a plain
// directory delete.
func TestHandleDeleteJobNonGitProject(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/wood_oak/delete", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body deleteJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.JobName != "wood_oak" || body.Dir == "" {
		t.Errorf("deleteJobResponse = %+v", body)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "jobs", "wood_oak")); !os.IsNotExist(err) {
		t.Errorf("job dir still exists after delete")
	}
}

// TestHandleDeleteJobUnknownJob: unknown job → 404.
func TestHandleDeleteJobUnknownJob(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	if rec := post(t, srv, "/projects/"+base+"/jobs/no-such-job/delete", "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- TASK-9: POST /projects/{project}/jobs/{job}/push ------------------------

// TestHandlePushJobNoRemote: pushing with no origin remote is a structured
// error carrying git's own message — never silently swallowed, never a hang.
func TestHandlePushJobNoRemote(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root, res := gitJobProject(t)
	defer os.RemoveAll(res.WorktreePath)
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/ab12cd/push", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "origin") {
		t.Errorf("500 body missing git's origin error: %s", rec.Body.String())
	}
}

// TestHandlePushJobWithRemote: with a real (bare) origin remote configured,
// the push succeeds.
func TestHandlePushJobWithRemote(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	root, res := gitJobProject(t)
	defer os.RemoveAll(res.WorktreePath)

	// A bare remote the push can reach.
	bare := t.TempDir()
	runGitT(t, bare, "init", "-q", "--bare")
	runGitT(t, root, "remote", "add", "origin", bare)

	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := post(t, srv, "/projects/"+base+"/jobs/ab12cd/push", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["status"] != "pushed" {
		t.Errorf("body = %v, want status=pushed", body)
	}
	// The branch exists on the remote.
	branches := gitCmd(t, bare, "branch", "--list")
	if !strings.Contains(branches, "feature/ab12cd_roundtrip-job") {
		t.Errorf("remote branches = %q, want the pushed job branch", branches)
	}
}

// --- TASK-10: POST /prune ----------------------------------------------------

// TestHandlePruneReportsCounts: /prune mirrors mg prune's reporting. Docker is
// absent in this environment, so the endpoint surfaces the docker error as a
// structured 500 — the meaningful contract here is that the response is a
// JSON envelope, never a silent no-op.
func TestHandlePrune(t *testing.T) {
	srv := New(&Registry{}, "test-version", "", nil)
	rec := post(t, srv, "/prune", "", "")
	// Docker is not available in the test env → 500 with the docker error.
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (docker unavailable) (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "docker") {
		t.Errorf("500 body missing the docker error: %s", rec.Body.String())
	}
}

// --- TASK-11: /projects/{project}/orphans ------------------------------------

// fakeOrphan creates a leftover worktree dir (a .git file pointing at a
// nonexistent gitdir) under the project's nested .manigot-worktrees parent —
// the shape DiscoverOrphans reports as an orphan.
func fakeOrphan(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, ".manigot-worktrees", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A .git pointer naming a gitdir that no longer exists.
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /nonexistent/"+name+"-gitdir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestHandleOrphansListAndDelete: GET lists the project's orphaned worktrees;
// POST /orphans/{name}/delete removes one named orphan.
func TestHandleOrphansListAndDelete(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	fakeOrphan(t, root, "abandoned_1")
	fakeOrphan(t, root, "abandoned_2")
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := get(t, srv, "/projects/"+base+"/orphans", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET orphans: status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var list orphansResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list not JSON: %v", err)
	}
	if len(list.Orphans) != 2 {
		t.Fatalf("orphans = %d, want 2:\n%s", len(list.Orphans), rec.Body.String())
	}
	if list.Orphans[0].Name != "abandoned_1" || list.Orphans[1].Name != "abandoned_2" {
		t.Errorf("orphan rows = %+v, want sorted by name", list.Orphans)
	}

	// Delete one named orphan.
	rec = post(t, srv, "/projects/"+base+"/orphans/abandoned_1/delete", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete orphan: status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".manigot-worktrees", "abandoned_1")); !os.IsNotExist(err) {
		t.Errorf("orphan dir still exists after delete")
	}
	// The other orphan is untouched.
	if _, err := os.Stat(filepath.Join(root, ".manigot-worktrees", "abandoned_2")); err != nil {
		t.Errorf("unrelated orphan removed: %v", err)
	}

	// Unknown orphan → 404.
	rec = post(t, srv, "/projects/"+base+"/orphans/no-such-orphan/delete", "", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown orphan: status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
}

// writeTestFile writes a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitCmd runs git -C dir args, failing the test on error, returning trimmed
// stdout.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGitOut(t, dir, args...)
}

// runGitOut runs git -C dir args, failing the test on error, returning trimmed
// stdout.
func runGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitExists reports whether the branch exists in the repo at root.
func gitExists(root, branch string) (bool, error) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).CombinedOutput()
	if err != nil {
		return false, nil // ref does not exist
	}
	_ = out
	return true, nil
}

// waitFor polls cond until it returns true or the deadline passes.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}