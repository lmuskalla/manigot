package serve

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// IsLoopback reports whether host is a loopback bind address: any address in
// 127.0.0.0/8, ::1, or the literal hostname "localhost". It is the daemon's
// binding classification for the startup guard — a remote (non-loopback) bind
// is only ever allowed with a token configured.
func IsLoopback(host string) bool {
	if host == "" {
		return false // "" binds all interfaces — not loopback
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// ValidateStartup enforces the bind/auth invariant that makes the daemon safe
// to run at all: a non-loopback bind address (neither 127.0.0.0/8 nor ::1)
// with NO token configured is refused outright — a remote daemon must be
// authed (the brief: "a token without TLS is no protection"; TLS is the
// reverse proxy's job, the daemon always serves plain HTTP). It is
// deliberately unskippable: there is no flag that turns it off, only a
// non-loopback bind, a token, or both.
func ValidateStartup(host, token string) error {
	if IsLoopback(host) {
		return nil
	}
	if token != "" {
		return nil
	}
	return fmt.Errorf("refusing to start: bind address %q is not loopback (neither 127.0.0.0/8 nor ::1) and no token is configured.\nA remotely-reachable daemon must require a bearer token (--token or MG_SERVE_TOKEN).\nBind to 127.0.0.1 or ::1 for a tokenless localhost daemon, or configure a token for remote exposure behind a TLS reverse proxy (Caddy/nginx).", host)
}

// authOutcome classifies a request for the audit log: tokenless (no token
// configured), authed (valid bearer token), or rejected (401 — missing or
// wrong token). The raw token itself is never logged.
type authOutcome string

const (
	authTokenless authOutcome = "tokenless"
	authAuthed    authOutcome = "authed"
	authRejected  authOutcome = "401"
)

// tokenMiddleware wraps next with bearer-token auth. When the server has no
// token configured (the tokenless localhost default), every request passes
// and is classified tokenless. When a token IS configured, every request must
// carry `Authorization: Bearer <token>` — compared in constant time via
// crypto/subtle — and a missing or wrong token is a 401 before the handler
// runs. The outcome is recorded on the wrapped writer (a *statusRecorder,
// since the audit middleware is always the outer layer) for the audit line.
func (s *Server) tokenMiddleware(next http.Handler) http.Handler {
	if s.token == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rec, ok := w.(*statusRecorder); ok {
				rec.setAuthOutcome(authTokenless)
			}
			next.ServeHTTP(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !tokenMatches(r, s.token) {
			if rec, ok := w.(*statusRecorder); ok {
				rec.setAuthOutcome(authRejected)
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if rec, ok := w.(*statusRecorder); ok {
			rec.setAuthOutcome(authAuthed)
		}
		next.ServeHTTP(w, r)
	})
}

// tokenMatches extracts the bearer token from r's Authorization header and
// compares it to want in constant time (crypto/subtle) once the length
// matches — the length pre-check is a structural early exit, not a value
// leak. Only the exact "Bearer <token>" form is accepted.
func tokenMatches(r *http.Request, want string) bool {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := strings.TrimPrefix(h, prefix)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
