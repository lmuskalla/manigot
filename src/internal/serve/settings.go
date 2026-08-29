package serve

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/project"
)

// --- GET/PUT /settings — the global default profile (TASK-1) ------------------
//
// The one global setting this API exposes is the default subscription profile
// (the brief's "set the profile"). Its storage and its write primitive are
// exactly `mg profiles <name>`'s: MANIGOT_PROFILE in manigot's .env, written
// via config.UpsertEnv — so CLI, TUI, and this API share one default, and a
// profile switch made over HTTP changes what the next bare `mg`, TUI, and
// mg-jdi launch use, with no daemon-side caching to drift.

// settingsResponse is the GET /settings response: the effective default
// profile id — config.EnvValue("MANIGOT_PROFILE") with the ProfileClaudePro
// fallback, the same "Active default" chain `mg profiles` displays. The
// profile id is a plain identifier, never credential material (readiness
// lives in /health as a boolean).
type settingsResponse struct {
	Profile string `json:"profile"`
}

// settingsRequest is the PUT /settings request body. Profile is required: an
// empty body or an absent field is a 400, not a silent reset to claude-pro —
// the endpoint exists to set the default, and a defaulting write would let a
// caller believe it changed something it did not.
type settingsRequest struct {
	Profile string `json:"profile"`
}

// handleGetSettings serves the effective default profile. No lock: a pure
// .env read, no project-scoped state.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsResponse{Profile: activeProfile()})
}

// activeProfile returns the effective default profile the way `mg profiles`
// reports it: MANIGOT_PROFILE via config.EnvValue (the .env file first, then
// the process environment), falling back to ProfileClaudePro when unset.
func activeProfile() string {
	if p := strings.TrimSpace(config.EnvValue("MANIGOT_PROFILE")); p != "" {
		return p
	}
	return config.ProfileClaudePro
}

// handlePutSettings sets the default profile (config.UpsertEnv — the same
// write `mg profiles <name>` makes, so every launch path picks it up). The id
// is validated against config.ProfileByID first: an unknown profile is a 400
// listing nothing about credentials, not a launch failure surfacing later. No
// project lock: this setting is global (manigot's .env), scoped to no
// registered root.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	profile := strings.TrimSpace(req.Profile)
	if profile == "" {
		writeError(w, http.StatusBadRequest, "profile is required")
		return
	}
	if _, known := config.ProfileByID(profile); !known {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown profile %q", profile))
		return
	}
	if err := config.UpsertEnv("MANIGOT_PROFILE", profile); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settingsResponse{Profile: profile})
}

// --- GET/PUT /projects/{project}/settings (TASK-2) ---------------------------
//
// The per-project settings this API exposes are the brief's baseBranch and
// prefix (jobBranchPrefix). Their storage and writer are exactly the TUI
// settings screen's: .manigot/manigot.json in the target project, read via
// project.Load and written via project.Save — the sanctioned host-side writer
// for the one directory outside docs/ manigot tooling owns.

// projectSettingsResponse is the project settings response shape. BaseBranch
// is the effective value (project.Settings.BaseBranchValue — "main" when
// unset), so a client always gets a usable ref; JobBranchPrefix is the stored
// value, empty meaning no prefix (it has no non-empty default).
type projectSettingsResponse struct {
	BaseBranch      string `json:"baseBranch"`
	JobBranchPrefix string `json:"jobBranchPrefix"`
}

// projectSettingsResponseFrom builds the response from stored settings.
func projectSettingsResponseFrom(s project.Settings) projectSettingsResponse {
	return projectSettingsResponse{
		BaseBranch:      s.BaseBranchValue(),
		JobBranchPrefix: s.JobBranchPrefix,
	}
}

// projectSettingsRequest is the PUT request body — a wholesale replacement
// (the PUT files/brief precedent: the whole settings object, not a patch), so
// an absent field clears that setting to its default. Both fields are
// optional; empty means "unset" (baseBranch → main fallback, prefix → none).
type projectSettingsRequest struct {
	BaseBranch      string `json:"baseBranch"`
	JobBranchPrefix string `json:"jobBranchPrefix"`
}

// handleGetProjectSettings serves a project's baseBranch + jobBranchPrefix.
// No lock: a pure read, like every other read endpoint.
func (s *Server) handleGetProjectSettings(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	settings, err := project.Load(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read project settings")
		return
	}
	writeJSON(w, http.StatusOK, projectSettingsResponseFrom(settings))
}

// handlePutProjectSettings replaces a project's baseBranch + jobBranchPrefix
// (project.Save — the same writer the TUI settings screen uses) under the
// project lock: the write itself is a plain file write, but create/done read
// these very settings inside their own locked sections (job.CreateJob's base
// branch, FinishJob's merge target), so serializing keeps a settings change
// from interleaving with a job lifecycle decision mid-flight.
//
// Validation is deliberately this layer's: the TUI form free-accepts these
// fields, but an HTTP caller cannot be shown a form — garbage would surface
// later as opaque git failures (mg job's "base branch '...' does not exist",
// or worse, a git argv a ref-shaped string was never meant for). A non-empty
// value must therefore be a sane ref component (refComponentProblem); an empty
// value clears the setting to its default.
func (s *Server) handlePutProjectSettings(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}

	var req projectSettingsRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	baseBranch := strings.TrimSpace(req.BaseBranch)
	jobBranchPrefix := strings.TrimSpace(req.JobBranchPrefix)
	for _, field := range []struct{ name, value string }{
		{"baseBranch", baseBranch},
		{"jobBranchPrefix", jobBranchPrefix},
	} {
		if reason, ok := refComponentProblem(field.value); !ok {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s %q: %s", field.name, field.value, reason))
			return
		}
	}

	s.locks.Lock(root)
	defer s.locks.Unlock(root)

	settings := project.Settings{
		BaseBranch:      baseBranch,
		JobBranchPrefix: jobBranchPrefix,
	}
	if err := project.Save(root, settings); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot write project settings")
		return
	}
	writeJSON(w, http.StatusOK, projectSettingsResponseFrom(settings))
}

// refComponentMaxLen bounds a single settings value — far above any real
// ref name (git's own documented ceiling is 255 bytes for a full ref; these
// are components) while refusing an unbounded write.
const refComponentMaxLen = 200

// refComponentProblem checks value as a git ref *component chain* — the shape
// baseBranch and jobBranchPrefix actually hold: namespaced branch names like
// "release/1.2" or prefix chains like "team/feature", where '/' separates
// legal components. It enforces git's own ref rules where they matter for
// correctness and safety (a value that reaches internal/git as an argv
// element must never be an option — hence the leading '-' rejection — nor
// contain shell-irrelevant but git-forbidden characters that make every later
// operation fail), without shelling out: no repo is needed (a non-git project
// may still store settings for later), and the check stays deterministic and
// testable. ok=false with a human-readable reason means reject.
//
// Empty is valid (it means "clear the setting") — callers decide whether an
// empty value is allowed; this check is about shape.
func refComponentProblem(value string) (reason string, ok bool) {
	if value == "" {
		return "", true
	}
	if len(value) > refComponentMaxLen {
		return "longer than the 200-character limit", false
	}
	if strings.HasPrefix(value, "-") {
		return "must not start with '-' (git would parse it as an option)", false
	}
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return "must not start or end with '/'", false
	}
	if strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return "must not start or end with '.'", false
	}
	if strings.HasSuffix(value, ".lock") {
		return "must not end with '.lock' (reserved by git)", false
	}
	if strings.Contains(value, "..") {
		return "must not contain '..'", false
	}
	if strings.Contains(value, "@{") || value == "@" {
		return "must not contain '@{' or be a lone '@'", false
	}
	for _, bad := range "~^:?*[\\" {
		if strings.ContainsRune(value, bad) {
			return fmt.Sprintf("must not contain %q", bad), false
		}
	}
	for i := 0; i < len(value); i++ {
		if c := value[i]; c <= 0x20 || c == 0x7f {
			return "must not contain control characters or spaces", false
		}
	}
	return "", true
}
