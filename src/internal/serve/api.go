package serve

import (
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
	"github.com/lmuskalla/manigot/internal/project"
	"github.com/lmuskalla/manigot/internal/session"
)

// --- JSON helpers ------------------------------------------------------------

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// apiError is the JSON error envelope every non-2xx response carries.
type apiError struct {
	Error string `json:"error"`
}

// writeError writes a JSON error envelope with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

// --- Resolution helpers — the zero-path-inputs choke point -------------------
//
// Every handler resolves its URL segments ONLY through these helpers. A
// segment is validated as a plain identifier (validSegment), then matched —
// never joined into a filesystem path. The registered roots (TASK-1), the
// discovered jobs, and the file whitelist are the only sources of paths the
// handlers ever open.

// validSegment reports whether a URL segment is a plain, trustworthy
// identifier: non-empty, not "." or ".." (traversal), containing no path
// separator (forward or backslash — the backslash is a Windows separator and
// must never reach a filesystem path), and no NUL byte. Percent-encoded and
// double-encoded variants all decode to one of these forms before matching,
// so rejecting the decoded form rejects them all.
func validSegment(seg string) bool {
	if seg == "" || seg == "." || seg == ".." {
		return false
	}
	if strings.ContainsAny(seg, "/\\") {
		return false
	}
	return !strings.ContainsRune(seg, 0)
}

// resolveProject maps a URL project segment to a registered root. The segment
// is validated, then matched against the registry by exact path, then unique
// base name (Registry.Project) — it is never joined into a filesystem path.
// An invalid segment or no match is a 404; the handler returns false and must
// return immediately.
func (s *Server) resolveProject(w http.ResponseWriter, segment string) (string, bool) {
	if !validSegment(segment) {
		writeError(w, http.StatusNotFound, "project not found")
		return "", false
	}
	root, ok := s.reg.Project(segment)
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return "", false
	}
	return root, true
}

// resolveJob maps a URL job segment to a discovered job under root. The
// segment is validated, then matched against the project's discovered jobs by
// ID, then by id_slug name (the job directory name — the same identity the
// CLI's branch-tail matching uses), then by unique name prefix. It is never
// treated as a path. Not-found is a 404; an ambiguous prefix is a 409; the
// handler returns false and must return immediately.
func (s *Server) resolveJob(w http.ResponseWriter, root, segment string) (job.Job, bool) {
	if !validSegment(segment) {
		writeError(w, http.StatusNotFound, "job not found")
		return job.Job{}, false
	}
	jobs, err := job.Discover(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read jobs")
		return job.Job{}, false
	}

	// Exact by ID, then by name.
	for _, j := range jobs {
		if j.ID == segment || j.Name == segment {
			return j, true
		}
	}

	// Unique prefix on the name.
	var matches []job.Job
	for _, j := range jobs {
		if strings.HasPrefix(j.Name, segment) {
			matches = append(matches, j)
		}
	}
	switch len(matches) {
	case 0:
		writeError(w, http.StatusNotFound, fmt.Sprintf("job '%s' not found among the project's jobs", segment))
		return job.Job{}, false
	case 1:
		return matches[0], true
	default:
		names := make([]string, 0, len(matches))
		for _, j := range matches {
			names = append(names, j.Name)
		}
		writeError(w, http.StatusConflict, fmt.Sprintf("job '%s' is ambiguous — matches jobs: %s", segment, strings.Join(names, " ")))
		return job.Job{}, false
	}
}

// jobFileNames is the whitelist of job files the API serves: a job's four
// markdown files, read as raw markdown text. Nothing else under the job dir
// is ever served.
var jobFileNames = map[string]bool{
	"brief.md":          true,
	"tasks.md":          true,
	"implementation.md": true,
	"verdict.md":        true,
}

// --- /health -----------------------------------------------------------------

// healthProfileRow is one profile's credential readiness in the /health
// response: ready = the profile's ResolveProfile + CheckAuth succeed (the
// same check the session launcher runs). A missing key is "not ready", never
// an error; the credential values themselves are never reported.
type healthProfileRow struct {
	ID    string `json:"id"`
	Ready bool   `json:"ready"`
}

// healthResponse is the /health response shape: the mg version, whether the
// docker image is present (session.ImagePresent), and per-profile readiness.
type healthResponse struct {
	Version      string             `json:"version"`
	ImagePresent bool               `json:"imagePresent"`
	Profiles     []healthProfileRow `json:"profiles"`
}

// profileReady reduces a profile's credential check to a boolean.
func profileReady(id string) bool {
	info, err := session.ResolveProfile(session.Options{Profile: id})
	if err != nil {
		return false
	}
	return info.CheckAuth() == nil
}

// handleHealth serves the daemon's health/status: version, docker image
// presence, and profile readiness — pure booleans and identifiers, never
// credential material.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Version:      s.version,
		ImagePresent: session.ImagePresent(),
	}
	for _, p := range config.Profiles() {
		resp.Profiles = append(resp.Profiles, healthProfileRow{ID: p.ID, Ready: profileReady(p.ID)})
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- /projects ---------------------------------------------------------------

// projectRow is one entry in the /projects response: a registered root and
// its base name (the URL segment that resolves to it).
type projectRow struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// projectsResponse is the /projects response shape.
type projectsResponse struct {
	Projects []projectRow `json:"projects"`
}

// handleProjects lists the registered project roots — pure registry data, the
// daemon never scans for projects it does not know.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	roots := s.reg.Projects()
	rows := make([]projectRow, 0, len(roots))
	for _, root := range roots {
		rows = append(rows, projectRow{Path: root, Name: filepath.Base(root)})
	}
	writeJSON(w, http.StatusOK, projectsResponse{Projects: rows})
}

// --- /projects/{project}/jobs ------------------------------------------------

// jdiStatusRow is the mg-jdi activity state for a job: state / agent / last
// update, from the job's status sidecar. Omitted when there is no live run
// state (a job mg-jdi never drove, or a stale/unreadable sidecar).
type jdiStatusRow struct {
	State   string `json:"state"`
	Agent   string `json:"agent"`
	Updated string `json:"updated"` // RFC3339
}

// jobRow is one row in the jobs response — the TUI's info design: id / status
// / stage / type / date / title, one row per job, in job.Discover's sort
// order (newest first).
type jobRow struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Status string        `json:"status"`
	Stage  string        `json:"stage"`
	Type   string        `json:"type"`
	Date   string        `json:"date"`
	Title  string        `json:"title"`
	Branch string        `json:"branch,omitempty"`
	JDI    *jdiStatusRow `json:"jdi,omitempty"`
}

// jobsResponse is the /projects/{project}/jobs response shape.
type jobsResponse struct {
	Jobs []jobRow `json:"jobs"`
}

// handleProjectJobs lists a project's open jobs with the mg-jdi activity
// state per job.
func (s *Server) handleProjectJobs(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	jobs, err := job.Discover(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read jobs")
		return
	}
	rows := make([]jobRow, 0, len(jobs))
	for _, j := range jobs {
		row := jobRow{
			ID:     j.ID,
			Name:   j.Name,
			Status: j.Status,
			Stage:  string(j.Stage()),
			Type:   j.Type,
			Date:   j.Date,
			Title:  j.Title,
			Branch: j.Branch,
		}
		if st, ok := job.ReadJDIStatus(root, j.Name); ok {
			row.JDI = &jdiStatusRow{
				State:   string(st.State),
				Agent:   st.Agent,
				Updated: st.Updated.Format(time.RFC3339),
			}
		}
		rows = append(rows, row)
	}
	writeJSON(w, http.StatusOK, jobsResponse{Jobs: rows})
}

// --- /projects/{project}/jobs/{job}/files/{file} -----------------------------

// handleJobFile serves one of a job's four markdown files as raw markdown
// text. The file segment is whitelisted (jobFileNames); a missing file is a
// normal 404 (e.g. verdict.md before review).
func (s *Server) handleJobFile(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}
	file := r.PathValue("file")
	if !jobFileNames[file] {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	data, err := os.ReadFile(filepath.Join(j.Dir, file))
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// --- /projects/{project}/jobs/{job}/jdi -------------------------------------

// jobJDIResponse is the /projects/{project}/jobs/{job}/jdi response shape:
// the mg-jdi status (state/agent/updated — null when there is no live run
// state), the run.log tail (null when mg-jdi never ran the job), and the
// job's session.log tail (null when no run output was captured yet — a
// normal "no run yet", not an error).
type jobJDIResponse struct {
	Status     *jdiStatusRow `json:"status"`
	RunLog     *string       `json:"runLog"`
	SessionLog *string       `json:"sessionLog"`
}

// tailBytes bounds how much of a log file the jdi endpoint loads into memory
// — a tail, not "load everything", the same bounded-viewport approach
// job.ReadJDIRunLogTail already uses for run.log (256 KiB, generous enough
// for many invocations' worth of text while a small, fixed cost).
const tailBytes = 256 * 1024

// readFileTail returns the last tailBytes of the file at path — the
// session.log counterpart of job.ReadJDIRunLogTail, applied to a job-dir file
// (docs/jobs/<name>/session.log). A possibly-partial first line is dropped so
// the tail starts at a clean boundary, with a truncation marker. ok is false
// only when the file is missing; an existing-but-empty file is ("", true).
func readFileTail(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", false
	}
	size := info.Size()
	if size == 0 {
		return "", true
	}

	start := int64(0)
	truncated := false
	if size > tailBytes {
		start = size - tailBytes
		truncated = true
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", false
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", false
	}

	text := string(data)
	if truncated {
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
		text = "… (log truncated — showing the most recent portion)\n\n" + text
	}
	return text, true
}

// handleJobJDI serves a job's mg-jdi run state and its captured logs: the
// status sidecar (ReadJDIStatus), the run.log tail (ReadJDIRunLogTail), and
// the job's own session.log tail. All three are file reads scoped to the
// resolved root/job — never client-supplied paths.
func (s *Server) handleJobJDI(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}

	resp := jobJDIResponse{}
	if st, ok := job.ReadJDIStatus(root, j.Name); ok {
		resp.Status = &jdiStatusRow{
			State:   string(st.State),
			Agent:   st.Agent,
			Updated: st.Updated.Format(time.RFC3339),
		}
	}
	if text, ok := job.ReadJDIRunLogTail(root, j.Name); ok {
		resp.RunLog = &text
	}
	if text, ok := readFileTail(filepath.Join(j.Dir, "session.log")); ok {
		resp.SessionLog = &text
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- /projects/{project}/jobs/{job}/diff ------------------------------------

// jobDiffResponse is the /projects/{project}/jobs/{job}/diff response shape.
// Without ?full=1 it is the "quick eyeball" — the branch's commits
// (git.LogOneline) and its changed files (git.DiffStat), the same two calls
// `mg diff` makes. With ?full=1 it is the complete patch (git.Diff). The
// three-dot range <base>...<branch> is resolved exactly as mg diff resolves
// it.
type jobDiffResponse struct {
	Log   *string `json:"log,omitempty"`
	Stat  *string `json:"stat,omitempty"`
	Patch *string `json:"patch,omitempty"`
}

// errJobBranchNotFound / errJobBranchAmbiguous classify a failed job→branch
// resolution so the handler can map them to 404 / 409 with the CLI's wording.
var (
	errJobBranchNotFound  = errors.New("job branch not found")
	errJobBranchAmbiguous = errors.New("job branch ambiguous")
)

// resolveJobBranch mirrors cmd/mg/diff.go's resolveJobBranch — the same
// exact-then-prefix matching on the branch's id_slug tail segment, with the
// same error wording. Not-found and ambiguity are classified (errors.Is) so
// the API can map them onto 404 / 409.
func resolveJobBranch(branches []string, jobArg string) (string, error) {
	if branch := git.ExactBranchMatch(branches, jobArg); branch != "" {
		return branch, nil
	}
	prefixMatches := git.PrefixBranchMatches(branches, jobArg)
	switch len(prefixMatches) {
	case 0:
		msg := fmt.Sprintf("job '%s' not found among local branches.\nActive job branches:", jobArg)
		for _, b := range branches {
			msg += "\n  " + b
		}
		return "", fmt.Errorf("%w: %s", errJobBranchNotFound, msg)
	case 1:
		return prefixMatches[0], nil
	default:
		return "", fmt.Errorf("%w: job '%s' is ambiguous — matches branches: %s", errJobBranchAmbiguous, jobArg, strings.Join(prefixMatches, " "))
	}
}

// handleJobDiff serves a job's branch diff against the project's base branch
// — the `mg diff` quick eyeball (log + stat) or, with ?full=1, the complete
// patch. The base branch resolves exactly as mg diff does (the project's
// configured baseBranch, falling back to origin/HEAD → main); the job→branch
// resolution is the CLI's exact-then-prefix chain. All git access goes
// through internal/git — the package that owns every git shell-out. Read-only
// git commands only.
func (s *Server) handleJobDiff(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	jobArg := r.PathValue("job")
	if !validSegment(jobArg) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	branches, err := git.LocalBranches(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read branches")
		return
	}
	branch, err := resolveJobBranch(branches, jobArg)
	if err != nil {
		switch {
		case errors.Is(err, errJobBranchAmbiguous):
			writeError(w, http.StatusConflict, strings.TrimPrefix(err.Error(), "job branch ambiguous: "))
		default:
			writeError(w, http.StatusNotFound, strings.TrimPrefix(err.Error(), "job branch not found: "))
		}
		return
	}

	settings, err := project.Load(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read project settings")
		return
	}
	base := settings.BaseBranch
	if base == "" {
		base = git.SymbolicRefHead(root)
	}

	if r.URL.Query().Get("full") == "1" {
		patch, err := git.Diff(root, base, branch)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot compute diff")
			return
		}
		writeJSON(w, http.StatusOK, jobDiffResponse{Patch: &patch})
		return
	}

	logs, err := git.LogOneline(root, base, branch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read log")
		return
	}
	stat, err := git.DiffStat(root, base, branch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read diff stat")
		return
	}
	writeJSON(w, http.StatusOK, jobDiffResponse{Log: &logs, Stat: &stat})
}

// --- /projects/{project}/agents ---------------------------------------------

// agentRow is one entry in the agents response: name and one-line description
// only — never the agent files' contents beyond the description line.
type agentRow struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// agentsResponse is the /projects/{project}/agents response shape.
type agentsResponse struct {
	Agents []agentRow `json:"agents"`
}

// handleProjectAgents lists the agents available to a project — the `mg
// agents` list (agentlist.Discover), name + description only.
func (s *Server) handleProjectAgents(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	agents, err := agentlist.Discover(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list agents")
		return
	}
	rows := make([]agentRow, 0, len(agents))
	for _, a := range agents {
		rows = append(rows, agentRow{Name: a.Name, Description: a.Description})
	}
	writeJSON(w, http.StatusOK, agentsResponse{Agents: rows})
}
