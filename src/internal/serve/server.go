package serve

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

// Server is the manigot listener daemon: a long-running HTTP server exposing
// the read-only control API (projects, jobs, job files, jdi status + logs,
// diff, agents, health) over the registered projects. v1 is read-only by
// design (see the brief's out-of-scope list); mutating endpoints are a later
// job and inherit the per-project serialization skeleton (see locks.go).
type Server struct {
	reg     *Registry
	version string
	token   string
	audit   io.Writer
	http    *http.Server
}

// New builds the server: the route table, the middleware chain (auth, then
// audit — audit outermost so it sees every request including 401s), and the
// http.Server wrapper. version is the mg version passed in from main (the
// health endpoint reports it; internal/serve cannot import package main).
// token is the bearer token ("" = tokenless localhost mode). audit is the
// request-log destination (mg serve passes stderr).
func New(reg *Registry, version, token string, audit io.Writer) *Server {
	s := &Server{reg: reg, version: version, token: token, audit: audit}
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

// routes registers every v1 endpoint. All routes are GETs; the resolution
// helpers (see api.go) are the only way any handler obtains a root, job or
// file path — the zero-path-inputs choke point.
func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /projects", s.handleProjects)
	mux.HandleFunc("GET /projects/{project}/jobs", s.handleProjectJobs)
	mux.HandleFunc("GET /projects/{project}/jobs/{job}/files/{file}", s.handleJobFile)
	mux.HandleFunc("GET /projects/{project}/jobs/{job}/jdi", s.handleJobJDI)
	mux.HandleFunc("GET /projects/{project}/jobs/{job}/diff", s.handleJobDiff)
	mux.HandleFunc("GET /projects/{project}/agents", s.handleProjectAgents)
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
// hung request cannot wedge the exit).
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }
