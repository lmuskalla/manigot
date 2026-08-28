package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmuskalla/manigot/internal/agentlist"
	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/launch"
	"github.com/lmuskalla/manigot/internal/session"
)

// maxJSONBodyBytes bounds every JSON request body this API decodes — a sane
// cap well above any realistic request (a title, a profile name, a force
// flag) while still refusing an unbounded upload.
const maxJSONBodyBytes = 1 << 20 // 1 MiB

// approvedConfirm is the pre-approved job.ConfirmFunc every mutating endpoint
// that wraps a confirm-gated job.* function passes: the HTTP call itself is
// the confirmation, mirroring internal/ui/app.go's own yesConfirm — the same
// precedent the TUI already established for its confirm-then-call pattern.
// Any warning this API wants to gate behind an explicit decision (see
// handleDoneJob's {"force":true} requirement) is intercepted before the
// wrapped function's own confirm prompts are ever reached.
func approvedConfirm(prompt string) (bool, error) { return true, nil }

// decodeJSONBody decodes r's body into v, tolerating a fully empty body (a
// zero-value v — every mutating endpoint's request body is either optional
// or has every field itself optional). A non-empty-but-invalid body is a 400.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes))
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true // an empty body decodes as EOF — same as no body
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// resolveProfile validates an optional profile string (from a request body),
// defaulting to config.ProfileClaudePro like every other launch path (mg
// jdi's own --profile flag, the session launcher). An unrecognized non-empty
// value is reported to the caller (ok=false; the handler must return
// immediately) rather than silently falling through to a launch failure deep
// inside a container spin-up.
func resolveProfile(w http.ResponseWriter, raw string) (profile string, ok bool) {
	profile = strings.TrimSpace(raw)
	if profile == "" {
		return config.ProfileClaudePro, true
	}
	if _, known := config.ProfileByID(profile); !known {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown profile %q", profile))
		return "", false
	}
	return profile, true
}

// --- POST /projects/{project}/jobs (TASK-3) ----------------------------------

// createJobRequest is the create-job endpoint's request body. Type defaults
// to "feature" when empty; BaseBranch optionally overrides the project's
// configured base branch for this one job.
type createJobRequest struct {
	Title      string `json:"title"`
	Type       string `json:"type"`
	BaseBranch string `json:"baseBranch"`
}

// createJobResponse is the create-job endpoint's response: the created job's
// row (the same shape GET .../jobs returns) plus the branch/worktree path
// from job.CreateResult.
type createJobResponse struct {
	Job          jobRow `json:"job"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktreePath,omitempty"`
}

// validJobTypes mirrors job.CreateJob's own accepted set (internal/job/
// create.go) — checked here so a bad value is a clean 400 instead of
// CreateJob's own "Invalid type '...' ..." error text surfacing as a 500.
var validJobTypes = map[string]bool{"": true, "feature": true, "fix": true, "chore": true}

// jobRowFromJob builds a jobRow — the exact shape handleProjectJobs returns —
// from an already-resolved job.Job, so a mutating endpoint's response carries
// the identical fields a subsequent GET .../jobs listing would show for it.
func jobRowFromJob(j job.Job) jobRow {
	return jobRow{
		ID:     j.ID,
		Name:   j.Name,
		Status: j.Status,
		Stage:  string(j.Stage()),
		Type:   j.Type,
		Date:   j.Date,
		Title:  j.Title,
		Branch: j.Branch,
	}
}

// handleCreateJob creates a new job (job.CreateJob) under the project lock —
// git-mutating: it creates a branch + worktree, one of the operations the
// brief's Notes names explicitly.
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}

	var req createJobRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if !validJobTypes[req.Type] {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid type %q — use feature, fix, or chore", req.Type))
		return
	}

	s.locks.Lock(root)
	defer s.locks.Unlock(root)

	res, err := job.CreateJob(root, req.Title, job.CreateOptions{Type: req.Type, BaseBranchOverride: req.BaseBranch}, io.Discard)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createJobResponse{
		Job:          jobRowFromJob(res.Job),
		Branch:       res.Branch,
		WorktreePath: res.WorktreePath,
	})
}

// --- PUT /projects/{project}/jobs/{job}/files/brief (TASK-4) -----------------

// maxBriefBytes bounds the edit-brief endpoint's request body — a sane cap
// well above any realistic brief.md while still refusing an unbounded upload.
const maxBriefBytes = 1 << 20 // 1 MiB

// editBriefResponse is the edit-brief endpoint's response. Warning is set
// only when the post-write commit sweep could not be attempted or failed —
// the write itself already succeeded either way, so this is a diagnostic, not
// a failure of the request.
type editBriefResponse struct {
	Status  string `json:"status"`
	Warning string `json:"warning,omitempty"`
}

// handleEditBrief replaces a job's brief.md wholesale with the raw request
// body — the one job file this API makes writable (tasks/implementation/
// verdict stay read-only, written only by their respective agents: no
// $EDITOR here, this is an HTTP body write) — then commits the change in the
// job's own worktree via session.SweepJobWorktree (the same "commit
// everything uncommitted" primitive every session-ending sweep already
// uses), so the edit never sits as an uncommitted change waiting for
// something else to sweep it.
//
// Takes the project lock: the commit itself touches git, even though the
// brief's Notes paragraph only names create/done/delete/push explicitly in
// its git-mutating set — a real race against a concurrent done/delete on the
// same worktree otherwise (this task's own reasoned addition, not something
// the brief pins either way).
func (s *Server) handleEditBrief(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBriefBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body")
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "request body must not be empty")
		return
	}
	if len(body) > maxBriefBytes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("request body exceeds the %d byte limit", maxBriefBytes))
		return
	}

	s.locks.Lock(root)
	defer s.locks.Unlock(root)

	if err := os.WriteFile(filepath.Join(j.Dir, "brief.md"), body, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "could not write brief.md")
		return
	}

	resp := editBriefResponse{Status: "ok"}
	var diag bytes.Buffer
	if wt, wtOK, werr := git.WorktreeForBranch(root, j.Branch); werr != nil {
		resp.Warning = "brief.md written, but could not resolve the job's worktree to commit it: " + werr.Error()
	} else if wtOK {
		session.SweepJobWorktree(session.Root{ProjectRoot: wt, InvocationRoot: root, Job: j.Name}, &diag)
		if diag.Len() > 0 {
			resp.Warning = strings.TrimSpace(diag.String())
		}
	} else {
		// A branch with no registered worktree is an inconsistent state — the
		// edit is written but cannot be committed, so say so rather than
		// claiming a clean ok.
		resp.Warning = fmt.Sprintf("brief.md written, but branch '%s' has no registered worktree to commit it — the edit may be lost on a later checkout", j.Branch)
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- POST /projects/{project}/jobs/{job}/agents/{agent} (TASK-5) ------------

// handleLaunchAgent launches one agent, one-shot, via internal/session's
// RunOneShot/CommandAgentRunner (TASK-2) — never an attached/interactive
// session, per the brief's explicit out-of-scope note. The agent name is
// validated against agentlist.Discover(root) first, for a fast 404 on an
// unknown agent rather than letting an invalid agent fail deep inside a
// container spin-up. Does NOT take s.locks: launching an agent doesn't touch
// git state and shouldn't block behind an unrelated done/delete (the brief's
// own Notes).
//
// Responds 202 Accepted immediately — the run happens in a background
// goroutine; the caller watches it via the session-log SSE stream (TASK-12)
// or by polling GET .../jdi's sessionLog tail.
func (s *Server) handleLaunchAgent(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}

	agent := r.PathValue("agent")
	if !validSegment(agent) {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	agents, err := agentlist.Discover(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list agents")
		return
	}
	known := false
	for _, a := range agents {
		if a.Name == agent {
			known = true
			break
		}
	}
	if !known {
		writeError(w, http.StatusNotFound, fmt.Sprintf("agent '%s' not found", agent))
		return
	}

	var req struct {
		Profile string `json:"profile"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	profile, ok := resolveProfile(w, req.Profile)
	if !ok {
		return
	}

	runner := &session.CommandAgentRunner{ProjectRoot: root, Profile: profile}
	go session.RunOneShot(runner, agent, j)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "agent": agent, "job": j.Name})
}

// --- POST /projects/{project}/jobs/{job}/jdi (TASK-6) -----------------------

// handleLaunchJDI launches `mg jdi` detached for a job (launch.Jdi — already
// exactly the shape this needs: a detached subprocess, no attached terminal,
// stdio discarded, reaped asynchronously). Before launching it checks
// job.ReadJDIStatus for an already-running state and rejects with 409 if
// found — a best-effort guard (not airtight: two launches issued
// back-to-back before the first process writes its own status sidecar could
// still both proceed) against two concurrent mg-jdi loops racing on the same
// job's git branch. No s.locks — same reasoning as handleLaunchAgent.
func (s *Server) handleLaunchJDI(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}

	if st, running := job.ReadJDIStatus(root, j.Name); running && st.State == job.JDIRunning {
		writeError(w, http.StatusConflict, fmt.Sprintf("mg jdi is already running for job '%s'", j.Name))
		return
	}

	var req struct {
		Profile string `json:"profile"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	profile, ok := resolveProfile(w, req.Profile)
	if !ok {
		return
	}

	if err := launch.Jdi(j.Name, root, profile); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "job": j.Name})
}

// --- POST /projects/{project}/jobs/{job}/done (TASK-7) ----------------------

// doneRequest is the done endpoint's request body: force, when true,
// proceeds past the "verdict not approved" / "no verdict.md" warnings the CLI
// would otherwise interactively confirm past — there is no human physically
// reading a terminal warning over this API, so the default is to refuse
// (409) rather than silently auto-approve a risky merge the way the TUI's
// yesConfirm does today.
type doneRequest struct {
	Force bool `json:"force"`
}

// doneResponse is the done endpoint's response: the finished job's name, the
// branch that was merged and deleted, and the base branch it was merged into.
type doneResponse struct {
	JobName    string `json:"jobName"`
	Branch     string `json:"branch"`
	BaseBranch string `json:"baseBranch"`
}

// doneVerdictWarning reproduces FinishJob's own pre-merge verdict check
// (internal/job/finish.go) without duplicating its regex classification: it
// reads the same job.Job.VerdictStatus/job.VerdictNotApproved exported
// helpers FinishJob itself is built on, so the two can never drift. hasWarning
// is true — with the CLI's own warning text — for exactly the three cases
// FinishJob would otherwise interactively confirm past: an undetermined
// status, a REJECTED/NEEDS WORK status, or a missing verdict.md.
func doneVerdictWarning(j job.Job) (warning string, hasWarning bool) {
	verdictPath := filepath.Join(j.Dir, "verdict.md")
	switch _, err := os.Stat(verdictPath); {
	case err == nil:
		status := j.VerdictStatus()
		switch {
		case status == "":
			return "Warning: could not determine verdict status from verdict.md", true
		case job.VerdictNotApproved(status):
			return fmt.Sprintf("Warning: verdict is '%s' — job is not approved.", status), true
		default:
			return "", false
		}
	case os.IsNotExist(err):
		return "Warning: no verdict.md found — job has not been reviewed.", true
	default:
		return "", false // unreadable for some other reason — let FinishJob itself surface it
	}
}

// handleDoneJob archives a finished job (job.FinishJobWithOptions) under the
// project lock (git-mutating, named explicitly in the brief's Notes).
//
// A squash-merge conflict is reported as a structured 409 error
// (job.ErrSquashMergeConflict, via FinishOptions.NoConflictRecovery) and the
// job is left exactly where FinishJob left it — no automatic rollback, no
// automatic @git-solver handoff — per the brief's own explicit pinning that
// this must not silently fall back to either of FinishJob's two interactive
// behaviors; resolving it is an explicit follow-up decision through some
// other call (e.g. launching @git-solver via the agent-launch endpoint, or
// `mg host` from a terminal).
//
// Separately (not pinned by the brief at all): the earlier "verdict not
// approved" / "no verdict.md" warnings require {"force": true} to proceed
// past — a 409 echoing the CLI's own warning text when force is absent —
// rather than silently auto-approving a risky merge the way the TUI's
// yesConfirm does today.
func (s *Server) handleDoneJob(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}

	var req doneRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if warning, has := doneVerdictWarning(j); has && !req.Force {
		writeError(w, http.StatusConflict, warning+" Pass {\"force\": true} to proceed anyway.")
		return
	}

	s.locks.Lock(root)
	defer s.locks.Unlock(root)

	var out bytes.Buffer
	res, err := job.FinishJobWithOptions(root, j.Name, approvedConfirm, &out, job.FinishOptions{NoConflictRecovery: true})
	if err != nil {
		if errors.Is(err, job.ErrSquashMergeConflict) {
			// The 409 must say explicitly that the job is already
			// archived-and-committed in its own worktree — FinishJobWithOptions
			// writes that explanation to out before returning the error (the
			// archive step runs before the merge attempt and cannot be undone).
			// A client reading only err.Error() would assume "nothing happened"
			// and retry done, which then fails confusingly with "brief.md not
			// found" because the job dir was renamed into docs/jobs/archive/.
			msg := err.Error()
			if s := strings.TrimSpace(out.String()); s != "" {
				msg += "\n\n" + s
			}
			writeError(w, http.StatusConflict, msg)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doneResponse{JobName: res.JobName, Branch: res.Branch, BaseBranch: res.BaseBranch})
}

// --- POST /projects/{project}/jobs/{job}/delete (TASK-8) --------------------

// deleteJobResponse is the delete endpoint's response — job.DeleteResult's
// fields.
type deleteJobResponse struct {
	JobName string `json:"jobName"`
	Branch  string `json:"branch,omitempty"`
	Dir     string `json:"dir"`
}

// handleDeleteJob permanently deletes a job (job.DeleteJob) under the project
// lock (git-mutating, named explicitly in the brief's Notes). The HTTP call
// itself is the confirmation (approvedConfirm) — no merge step, so unlike
// done there is no conflict-handling ambiguity here.
func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}

	s.locks.Lock(root)
	defer s.locks.Unlock(root)

	var out bytes.Buffer
	res, err := job.DeleteJob(root, j.Name, approvedConfirm, &out)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deleteJobResponse{JobName: res.JobName, Branch: res.Branch, Dir: res.Dir})
}

// --- POST /projects/{project}/jobs/{job}/push (TASK-9) ----------------------

// pushTimeout bounds the push endpoint's git call, mirroring the TUI git
// panel's own hostGitTimeout (src/internal/ui/app.go): long enough for a
// real push, short enough that a stalled/unreachable remote can't hang the
// request forever.
const pushTimeout = 30 * time.Second

// handlePushJob pushes a job's branch to origin (git.PushWithContext — `git
// push -u origin <branch>`, mirroring the TUI git panel's "Push to origin"
// action) under the project lock (git-mutating, named explicitly in the
// brief's Notes). A push failure (auth, network, non-fast-forward) surfaces
// as a structured error with git's own message, never silently swallowed.
func (s *Server) handlePushJob(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}

	s.locks.Lock(root)
	defer s.locks.Unlock(root)

	ctx, cancel := context.WithTimeout(r.Context(), pushTimeout)
	defer cancel()
	if err := git.PushWithContext(ctx, root, j.Branch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "pushed", "branch": j.Branch})
}

// --- POST /prune (TASK-10) ---------------------------------------------------

// pruneResponse mirrors mg prune's own reporting (session.PruneResult).
type pruneResponse struct {
	Removed int `json:"removed"`
	Running int `json:"running"`
}

// handlePrune wraps session.PruneOrphans — the same function cmd/mg/prune.go's
// runPrune calls. Deliberately top-level (not nested under /projects/{project})
// rather than project-scoped: orphaned manigot-* containers are not
// partitioned by project (docker has no concept of "which registered project"
// a leftover container belongs to). No s.locks: no git or project-scoped
// state at all.
func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	res, err := session.PruneOrphans(io.Discard)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pruneResponse{Removed: res.Removed, Running: res.Running})
}

// --- /projects/{project}/orphans (TASK-11) ----------------------------------

// orphanRow is one orphaned worktree in the response — job.Orphan's fields.
type orphanRow struct {
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	GitDir string `json:"gitDir"`
}

// orphansResponse is the list-orphans endpoint's response shape.
type orphansResponse struct {
	Orphans []orphanRow `json:"orphans"`
}

// handleProjectOrphans lists a project's orphaned worktrees (job.
// DiscoverOrphans) — read-only, no lock.
func (s *Server) handleProjectOrphans(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	orphans, err := job.DiscoverOrphans(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list orphaned worktrees")
		return
	}
	rows := make([]orphanRow, 0, len(orphans))
	for _, o := range orphans {
		rows = append(rows, orphanRow{Name: o.Name, Dir: o.Dir, GitDir: o.GitDir})
	}
	writeJSON(w, http.StatusOK, orphansResponse{Orphans: rows})
}

// handleDeleteOrphan resolves one named orphan (job.MatchOrphan, exact then
// prefix on the orphan's base name) and removes it (job.
// RemoveOrphansConfirmed — the HTTP call itself is the confirmation, no
// further prompt) under the project lock: RemoveOrphansConfirmed calls
// git.WorktreePrune, which touches git worktree metadata and can race with a
// concurrent create/done/delete on the same project — this task's own
// reasoned addition, since the brief's Notes paragraph does not name orphan
// cleanup in its git-mutating set at all.
//
// One named resource per call, consistent with every other mutating endpoint
// in this API, rather than a single "delete everything" call that risks
// removing something the caller didn't expect to see gone.
func (s *Server) handleDeleteOrphan(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	name := r.PathValue("name")
	if !validSegment(name) {
		writeError(w, http.StatusNotFound, "orphan not found")
		return
	}
	o, found := job.MatchOrphan(root, name)
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("orphan '%s' not found among the project's orphaned worktrees", name))
		return
	}

	s.locks.Lock(root)
	defer s.locks.Unlock(root)

	var out bytes.Buffer
	if err := job.RemoveOrphansConfirmed(root, []job.Orphan{o}, &out); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orphanRow{Name: o.Name, Dir: o.Dir, GitDir: o.GitDir})
}
