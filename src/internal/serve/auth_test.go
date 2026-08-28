package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testServer builds a Server over a registry holding one temp project root,
// returning the server and the registered root. audit is the audit writer
// (nil for tests that don't assert audit lines).
func testServer(t *testing.T, token string, audit io.Writer) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	reg := &Registry{entries: []Entry{entryFor(root)}}
	return New(reg, "test-version", token, audit), root
}

// get performs a GET against the handler with the given Authorization header.
func get(t *testing.T, srv *Server, path, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// request performs an arbitrary-method request against the handler with an
// optional body and Authorization header — the mutating-endpoint twin of get
// (post/put/patch/delete). body is sent verbatim as the request body; "" sends
// no body at all.
func request(t *testing.T, srv *Server, method, path, auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// post performs a POST against the handler with an optional JSON body and
// Authorization header.
func post(t *testing.T, srv *Server, path, auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, srv, http.MethodPost, path, auth, body)
}

// put performs a PUT against the handler with an optional raw body and
// Authorization header.
func put(t *testing.T, srv *Server, path, auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, srv, http.MethodPut, path, auth, body)
}

// TestTokenRequiredPins the auth contract: with a token configured, the
// correct bearer header passes, and a missing or wrong token is a 401 before
// any handler logic runs.
func TestTokenRequiredPins(t *testing.T) {
	srv, _ := testServer(t, "sekrit-token", nil)

	if rec := get(t, srv, "/projects", "Bearer sekrit-token"); rec.Code != http.StatusOK {
		t.Errorf("correct token: status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if rec := get(t, srv, "/projects", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("missing token: status = %d, want 401", rec.Code)
	}
	if rec := get(t, srv, "/projects", "Bearer wrong-token"); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: status = %d, want 401", rec.Code)
	}
	if rec := get(t, srv, "/projects", "Basic sekrit-token"); rec.Code != http.StatusUnauthorized {
		t.Errorf("non-bearer scheme: status = %d, want 401", rec.Code)
	}
	if rec := get(t, srv, "/projects", "Bearer sekrit-token-extra"); rec.Code != http.StatusUnauthorized {
		t.Errorf("token with wrong length: status = %d, want 401", rec.Code)
	}
}

// TestTokenlessLocalhostPins the default: no token configured means every
// request passes (the machine's own user is the trust boundary, as it is for
// the CLI today).
func TestTokenlessLocalhostPins(t *testing.T) {
	srv, _ := testServer(t, "", nil)
	if rec := get(t, srv, "/projects", ""); rec.Code != http.StatusOK {
		t.Errorf("tokenless: status = %d, want 200", rec.Code)
	}
	// An Authorization header on a tokenless daemon is ignored, not rejected.
	if rec := get(t, srv, "/projects", "Bearer anything"); rec.Code != http.StatusOK {
		t.Errorf("tokenless with stray header: status = %d, want 200", rec.Code)
	}
}

// TestUnauthorizedIsJSONError: the 401 body is the JSON error envelope, never
// a hint about the expected token.
func TestUnauthorizedIsJSONError(t *testing.T) {
	srv, _ := testServer(t, "sekrit-token", nil)
	rec := get(t, srv, "/projects", "")
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("401 body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body["error"] == "" {
		t.Errorf("401 body = %v, want an error envelope", body)
	}
	if strings.Contains(rec.Body.String(), "sekrit-token") {
		t.Errorf("401 body echoes the token: %s", rec.Body.String())
	}
}

// TestTokenMatchesConstantTimeCompare: the comparison goes through
// crypto/subtle.ConstantTimeCompare (the value path) once the length
// matches — asserted structurally by pinning the accept/reject behavior at
// the tokenMatches level, the function the middleware uses.
func TestTokenMatchesConstantTimeCompare(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer correct-horse")
	if !tokenMatches(req, "correct-horse") {
		t.Error("tokenMatches with the right token = false, want true")
	}
	req.Header.Set("Authorization", "Bearer correct-horsf") // same length, wrong value
	if tokenMatches(req, "correct-horse") {
		t.Error("tokenMatches with a same-length wrong token = true, want false")
	}
	req.Header.Set("Authorization", "Bearer short")
	if tokenMatches(req, "correct-horse") {
		t.Error("tokenMatches with a different-length token = true, want false")
	}
	req.Header.Set("Authorization", "Basic correct-horse")
	if tokenMatches(req, "correct-horse") {
		t.Error("tokenMatches with a non-Bearer scheme = true, want false")
	}
}

// TestIsLoopback pins the binding classification: 127.0.0.0/8, ::1 and
// localhost are loopback; everything else — including the empty host (binds
// all interfaces) — is not.
func TestIsLoopback(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "127.0.0.0", "127.99.99.99", "::1", "localhost"} {
		if !IsLoopback(host) {
			t.Errorf("IsLoopback(%q) = false, want true", host)
		}
	}
	for _, host := range []string{"", "0.0.0.0", "::", "192.168.1.10", "10.0.0.5", "example.com", "2001:db8::1"} {
		if IsLoopback(host) {
			t.Errorf("IsLoopback(%q) = true, want false", host)
		}
	}
}

// TestValidateStartupPins the hard invariant: a non-loopback bind without a
// token refuses to start; a token (or a loopback bind) permits it. There is
// no flag that turns the refusal off — only a token or a loopback bind.
func TestValidateStartupPins(t *testing.T) {
	if err := ValidateStartup("0.0.0.0", ""); err == nil {
		t.Error("ValidateStartup(0.0.0.0, no token) = nil, want refusal")
	}
	if err := ValidateStartup("192.168.1.10", ""); err == nil {
		t.Error("ValidateStartup(non-loopback, no token) = nil, want refusal")
	}
	if err := ValidateStartup("2001:db8::1", ""); err == nil {
		t.Error("ValidateStartup(non-loopback IPv6, no token) = nil, want refusal")
	}
	if err := ValidateStartup("0.0.0.0", "some-token"); err != nil {
		t.Errorf("ValidateStartup(0.0.0.0, token) = %v, want nil", err)
	}
	if err := ValidateStartup("127.0.0.1", ""); err != nil {
		t.Errorf("ValidateStartup(127.0.0.1, no token) = %v, want nil", err)
	}
	if err := ValidateStartup("::1", ""); err != nil {
		t.Errorf("ValidateStartup(::1, no token) = %v, want nil", err)
	}
	if err := ValidateStartup("localhost", ""); err != nil {
		t.Errorf("ValidateStartup(localhost, no token) = %v, want nil", err)
	}
}
