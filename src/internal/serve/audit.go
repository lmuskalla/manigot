package serve

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// auditMiddleware logs one line per request — the daemon's audit trail (the
// brief's scope item 7). Each line carries: a timestamp, the client (the
// remote address — the IP; the port is stripped), the operation (method +
// path), the auth outcome (authed / tokenless / 401), and the response
// status. It never logs the Authorization header, the token, or request
// bodies. It sits outermost in the middleware chain so it sees every request
// — including the 401s the token middleware rejects before any handler runs —
// and it hands the wrapped writer (a *statusRecorder) to the inner chain, so
// the token middleware can record the auth outcome on it.
func (s *Server) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logAuditLine(r, rec.status, rec.auth)
	})
}

// statusRecorder wraps the ResponseWriter to capture the response status code
// and the auth outcome for the audit line. It implements the minimum surface
// the handlers use (WriteHeader is the only interception that matters — a
// handler that never calls WriteHeader means 200, the zero-value default).
type statusRecorder struct {
	http.ResponseWriter
	status int
	auth   authOutcome
}

// WriteHeader captures the status before delegating.
func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// setAuthOutcome records the request's auth classification for the audit
// line. The token middleware calls it on the wrapped writer (the audit
// middleware is always the outer layer, so the writer it receives is a
// *statusRecorder).
func (rec *statusRecorder) setAuthOutcome(o authOutcome) {
	rec.auth = o
}

// logAuditLine writes one audit line to the server's audit writer (nil means
// no audit logging — the tokenless localhost daemon is free to run with
// logging disabled; mg serve passes stderr by default). The client is the
// remote address's host portion. The path is the raw URL path — identifiers,
// never credentials.
func (s *Server) logAuditLine(r *http.Request, status int, outcome authOutcome) {
	if s.audit == nil {
		return
	}
	client := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		client = host
	}
	fmt.Fprintf(s.audit, "%s client=%s method=%s path=%s auth=%s status=%d\n",
		time.Now().UTC().Format(time.RFC3339), client, r.Method, r.URL.Path, outcome, status)
}
