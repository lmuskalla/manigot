package serve

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestAuditLogsEveryRequest: one line per request — including 4xx/401s —
// carrying a timestamp, the client, the operation (method + path), the auth
// outcome, and the response status.
func TestAuditLogsEveryRequest(t *testing.T) {
	var audit strings.Builder
	srv, _ := testServer(t, "", &audit)

	// 200, 404, and another 200.
	get(t, srv, "/projects", "")
	get(t, srv, "/projects/no-such-project/jobs", "")
	get(t, srv, "/projects", "")

	lines := strings.Split(strings.TrimRight(audit.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("audit lines = %d, want 3:\n%s", len(lines), audit.String())
	}
	for i, want := range []struct {
		path   string
		status string
	}{
		{"/projects", "200"},
		{"/projects/no-such-project/jobs", "404"},
		{"/projects", "200"},
	} {
		line := lines[i]
		for _, part := range []string{
			"client=",
			"method=GET",
			"path=" + want.path,
			"auth=" + string(authTokenless),
			"status=" + want.status,
		} {
			if !strings.Contains(line, part) {
				t.Errorf("line %d = %q, missing %q", i, line, part)
			}
		}
		// A parseable RFC3339 timestamp leads the line.
		ts := line
		if idx := strings.Index(line, " "); idx >= 0 {
			ts = line[:idx]
		}
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			t.Errorf("line %d = %q, timestamp %q not RFC3339: %v", i, line, ts, err)
		}
		// The client is an IP (the remote address's host portion).
		if ip := net.ParseIP(strings.TrimPrefix(line[strings.Index(line, "client=")+7:], "")); ip == nil {
			// The client= value ends at the next space; check it parses.
			rest := line[strings.Index(line, "client=")+7:]
			host := rest
			if idx := strings.Index(rest, " "); idx >= 0 {
				host = rest[:idx]
			}
			if net.ParseIP(host) == nil {
				t.Errorf("line %d = %q, client %q not an IP", i, line, host)
			}
		}
	}
}

// TestAuditLogs401sAndOutcomes: with a token configured, the audit classifies
// authed vs 401, and both are logged.
func TestAuditLogs401sAndOutcomes(t *testing.T) {
	var audit strings.Builder
	srv, _ := testServer(t, "sekrit-token", &audit)

	get(t, srv, "/projects", "")                    // 401 — no token
	get(t, srv, "/projects", "Bearer sekrit-token") // 200 — authed
	get(t, srv, "/projects", "Bearer wrong")        // 401 — wrong token

	lines := strings.Split(strings.TrimRight(audit.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("audit lines = %d, want 3:\n%s", len(lines), audit.String())
	}
	if !strings.Contains(lines[0], "auth=401") || !strings.Contains(lines[0], "status=401") {
		t.Errorf("no-token line = %q, want auth=401 status=401", lines[0])
	}
	if !strings.Contains(lines[1], "auth=authed") || !strings.Contains(lines[1], "status=200") {
		t.Errorf("authed line = %q, want auth=authed status=200", lines[1])
	}
	if !strings.Contains(lines[2], "auth=401") || !strings.Contains(lines[2], "status=401") {
		t.Errorf("wrong-token line = %q, want auth=401 status=401", lines[2])
	}
}

// TestAuditNeverLogsCredentials: a request carrying a known token value must
// not echo it — or the Authorization header — in any audit line.
func TestAuditNeverLogsCredentials(t *testing.T) {
	var audit strings.Builder
	srv, _ := testServer(t, "sekrit-token", &audit)

	// A token-carrying request (and a rejected one with a wrong token).
	get(t, srv, "/projects", "Bearer sekrit-token")
	get(t, srv, "/projects", "Bearer wrong-token")

	if strings.Contains(audit.String(), "sekrit-token") {
		t.Errorf("audit log echoes the token:\n%s", audit.String())
	}
	if strings.Contains(audit.String(), "wrong-token") {
		t.Errorf("audit log echoes a rejected token value:\n%s", audit.String())
	}
	if strings.Contains(audit.String(), "Authorization") || strings.Contains(audit.String(), "Bearer ") {
		t.Errorf("audit log echoes the Authorization header:\n%s", audit.String())
	}
}

// TestAuditNilWriterIsNoop: a nil audit writer (audit logging disabled) is a
// strict no-op — every request still works.
func TestAuditNilWriterIsNoop(t *testing.T) {
	srv, _ := testServer(t, "", nil)
	if rec := get(t, srv, "/projects", ""); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
