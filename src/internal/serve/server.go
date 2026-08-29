package serve

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

// Server is the manigot listener daemon: a long-running HTTP server exposing
// the control API (projects, jobs, job files, jdi status + logs, diff,
// agents, health — read-only; plus job two's mutating endpoints and the
// session-log SSE stream) over the registered projects. Mutating handlers
// serialize per project via locks (see locks.go's own doc for which
// operations take it and why).
type Server struct {
	reg     *Registry
	version string
	token   string
	audit   io.Writer
	http    *http.Server
	locks   *ProjectLocks

	// shutdownCtx/shutdownCancel let a long-lived handler (the session-log SSE
	// stream, see stream.go) notice a graceful Shutdown even though
	// http.Server.Shutdown itself only stops accepting new connections and
	// waits for in-flight handlers to return — it does not interrupt a
	// handler stuck in its own poll loop. Shutdown cancels shutdownCtx before
	// calling s.http.Shutdown, so every streaming handler selecting on it
	// unblocks and returns promptly instead of forcing the drain to wait out
	// its full timeout.
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

// New builds the server: the route table, the middleware chain (auth, then
// audit — audit outermost so it sees every request including 401s), and the
// http.Server wrapper. version is the mg version passed in from main (the
// health endpoint reports it; internal/serve cannot import package main).
// token is the bearer token ("" = tokenless localhost mode). audit is the
// request-log destination (mg serve passes stderr).
func New(reg *Registry, version, token string, audit io.Writer) *Server {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	s := &Server{
		reg:            reg,
		version:        version,
		token:          token,
		audit:          audit,
		locks:          NewProjectLocks(),
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}
	mux := http.NewServeMux()
	s.routes(mux)
	var h http.Handler = mux
	h = s.tokenMiddleware(h)
	h = s.auditMiddleware(h)
	s.http = &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// routes registers every v1+v2 endpoint. The resolution helpers (see api.go)
// are the only way any handler obtains a root, job or file path — the
// zero-path-inputs choke point. Mutating routes and their ProjectLocks
// boundary are documented at each handler; see locks.go for the summary.
func (s *Server) routes(mux *http.ServeMux) {
	// Read-only (job one).
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /projects", s.handleProjects)
	mux.HandleFunc("GET /projects/{project}/jobs", s.handleProjectJobs)
	mux.HandleFunc("GET /projects/{project}/jobs/{job}/files/{file}", s.handleJobFile)
	mux.HandleFunc("GET /projects/{project}/jobs/{job}/jdi", s.handleJobJDI)
	mux.HandleFunc("GET /projects/{project}/jobs/{job}/diff", s.handleJobDiff)
	mux.HandleFunc("GET /projects/{project}/agents", s.handleProjectAgents)

	// Mutating (job two).
	mux.HandleFunc("POST /projects/{project}/jobs", s.handleCreateJob)
	mux.HandleFunc("PUT /projects/{project}/jobs/{job}/files/brief", s.handleEditBrief)
	mux.HandleFunc("POST /projects/{project}/jobs/{job}/agents/{agent}", s.handleLaunchAgent)
	mux.HandleFunc("POST /projects/{project}/jobs/{job}/jdi", s.handleLaunchJDI)
	mux.HandleFunc("POST /projects/{project}/jobs/{job}/done", s.handleDoneJob)
	mux.HandleFunc("POST /projects/{project}/jobs/{job}/delete", s.handleDeleteJob)
	mux.HandleFunc("POST /projects/{project}/jobs/{job}/push", s.handlePushJob)
	mux.HandleFunc("POST /prune", s.handlePrune)
	mux.HandleFunc("GET /projects/{project}/orphans", s.handleProjectOrphans)
	mux.HandleFunc("POST /projects/{project}/orphans/{name}/delete", s.handleDeleteOrphan)

	// Settings (job brother): the global default profile, and the
	// per-project baseBranch + jobBranchPrefix — see settings.go.
	mux.HandleFunc("GET /settings", s.handleGetSettings)
	mux.HandleFunc("PUT /settings", s.handlePutSettings)
	mux.HandleFunc("GET /projects/{project}/settings", s.handleGetProjectSettings)
	mux.HandleFunc("PUT /projects/{project}/settings", s.handlePutProjectSettings)

	// Live run supervision (job two).
	mux.HandleFunc("GET /projects/{project}/jobs/{job}/session-log/stream", s.handleSessionLogStream)
}

// Handler returns the fully-wrapped http.Handler — the httptest-able seam for
// endpoint tests (httptest.NewServer(srv.Handler())).
func (s *Server) Handler() http.Handler { return s.http.Handler }

// Serve serves on ln until Shutdown or a fatal error — the httptest-able
// seam for the serve loop (mg serve calls this on its bound listener; tests
// hand over a bound 127.0.0.1:0 listener).
func (s *Server) Serve(ln net.Listener) error { return s.http.Serve(ln) }

// Shutdown gracefully drains in-flight requests and closes the listener. The
// caller bounds the wait with the context (mg serve uses a bounded drain so a
// hung request cannot wedge the exit). shutdownCtx is cancelled first, so any
// long-lived streaming handler (see stream.go) unblocks its own poll loop and
// returns immediately rather than forcing the drain to wait out its full
// timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownCancel()
	return s.http.Shutdown(ctx)
}
