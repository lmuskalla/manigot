package serve

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hostileSegments are the URL-path segment forms that must never resolve to a
// filesystem path: encoded traversal (single and double), absolute-path
// segments, Windows separators, and NUL bytes. These are the ENCODED forms as
// they appear in a URL — net/http's single decoding pass hands the handler
// the decoded value, which validSegment then rejects. Raw `..` / `.` segments
// in the URL path never reach a handler (ServeMux's own path sanitization
// redirects them first), so they are not in this list; the endpoint tests
// assert that redirect separately.
//
// Why the ENCODED forms reach the handler at all (pinned empirically on
// Go 1.22+): ServeMux's cleanPath sanitization runs on the ESCAPED request
// path (r.URL.EscapedPath), where a segment spelled `%2e%2e` is not the
// literal `.`/`..` cleanPath looks for — so it is not redirected. Only the
// raw literal forms (`/projects/../jobs`) trigger the redirect. The encoded
// forms are therefore matched as wildcard segments with the DECODED hostile
// value in PathValue, and it is validSegment's rejection of that decoded
// value that answers them with 4xx. A future ServeMux that cleaned the
// decoded path instead would answer these with a 3xx redirect — equally
// nothing served, equally nothing read — which is why the endpoint tests
// assert "4xx" for the handler-rejected forms and "3xx" for the
// mux-sanitized raw forms separately.
var hostileSegments = []string{
	"..%2f..%2fetc%2fpasswd",         // decodes to ../../
	"%2e%2e",                         // decodes to ".."
	"%2e",                            // decodes to "."
	".%2e",                           // decodes to ".."
	"%2e%2e%2f%2e%2e%2fetc%2fpasswd", // fully-encoded traversal
	"%2fetc%2fpasswd",                // decodes to an absolute path
	"a%2fb",                          // decodes to a forward slash
	"a%5cb",                          // decodes to a Windows backslash
	"a%00b",                          // decodes to a NUL byte
	"..%252f..%252fetc%252fpasswd",   // double-encoded traversal
	"%252e%252e%252f%252e%252e%252fetc%252fpasswd",
}

// decodedHostileSegments are the DECODED segment values — exactly what
// r.PathValue hands the handlers — that validSegment must reject outright.
var decodedHostileSegments = []string{
	"..",
	".",
	"../../etc/passwd",
	"/etc/passwd",
	"a/b",
	"a\\b",
	"a\x00b",
	"..\\..\\etc\\passwd",
	"sub/../../x",
	"..\x00",
}

// TestValidSegmentRejectsHostileInput pins the choke-point primitive: every
// hostile decoded segment is rejected outright, while legitimate identifiers
// (job ids, id_slug names, file names) pass. A rejected segment can never
// even reach the matching logic, so it can never be joined into a path.
func TestValidSegmentRejectsHostileInput(t *testing.T) {
	for _, seg := range decodedHostileSegments {
		if validSegment(seg) {
			t.Errorf("validSegment(%q) = true, want false", seg)
		}
	}
	for _, seg := range []string{"wood", "wood_oak", "brief.md", "tasks.md", "some-project"} {
		if !validSegment(seg) {
			t.Errorf("validSegment(%q) = false, want true", seg)
		}
	}
}

// TestValidSegmentRejectsTraversalSubstrings: beyond the exact-segment forms,
// any decoded segment containing a path separator or NUL is rejected — the
// belt-and-braces layer under the whitelists.
func TestValidSegmentRejectsTraversalSubstrings(t *testing.T) {
	for _, seg := range []string{
		"../../etc/passwd",
		"/etc/passwd",
		"..\\..\\etc\\passwd",
		"sub/../../x",
		"a\x00b",
		"..\x00",
	} {
		if validSegment(seg) {
			t.Errorf("validSegment(%q) = true, want false", seg)
		}
	}
}

// TestResolveProjectNeverResolvesOutsideRegistry: the project choke point
// returns not-found for every hostile segment and for unregistered names —
// never a path the handler could open.
func TestResolveProjectNeverResolvesOutsideRegistry(t *testing.T) {
	srv, root := testServer(t, "", nil)
	base := filepath.Base(root)

	// The registered base name resolves; hostile and unknown segments do not.
	for _, seg := range append(hostileSegments, "no-such-project", "etc") {
		if _, ok := srv.reg.Project(seg); ok {
			t.Errorf("Project(%q) resolved, want not-found", seg)
		}
	}
	if got, ok := srv.reg.Project(base); !ok || got != filepath.Clean(root) {
		t.Errorf("Project(%q) = %q, %v; want the registered root", base, got, ok)
	}
}

// TestHostileURLsRejectedAtEveryPathPosition: the full endpoint surface — a
// suite of hostile URLs against the project, job, and file path positions,
// asserting 4xx and — critically — that no response carries content from
// outside the registered roots (a secret planted in an unregistered
// directory must never appear).
func TestHostileURLsRejectedAtEveryPathPosition(t *testing.T) {
	// A secret in an UNREGISTERED directory (a sibling of the registered
	// project) that hostile URLs must never reach.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("TOP-SECRET-OUTSIDE"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A secret INSIDE the registered project but outside any whitelisted job
	// file (session.log — never served by the file endpoint).
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"), "session.log")
	if err := os.WriteFile(filepath.Join(root, "docs", "jobs", "wood_oak", "session.log"), []byte("TOP-SECRET-SESSION"), 0o644); err != nil {
		t.Fatal(err)
	}
	// And a secret directly in the project root (docs/../secret.txt position).
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("TOP-SECRET-ROOT"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := &Registry{entries: []Entry{entryFor(root)}}
	srv := New(reg, "test-version", "", nil)
	base := filepath.Base(root)

	type hostileURL struct {
		path string
		desc string
	}
	var urls []hostileURL
	// Project position: every hostile segment as the project.
	for _, seg := range hostileSegments {
		urls = append(urls, hostileURL{"/projects/" + seg + "/jobs", "project segment " + seg})
		urls = append(urls, hostileURL{"/projects/" + seg + "/agents", "project segment " + seg})
	}
	// Job position: hostile segments as the job (project resolves by base name).
	for _, seg := range hostileSegments {
		urls = append(urls, hostileURL{"/projects/" + base + "/jobs/" + seg + "/files/brief.md", "job segment " + seg})
		urls = append(urls, hostileURL{"/projects/" + base + "/jobs/" + seg + "/jdi", "job segment " + seg})
	}
	// File position: hostile segments as the file (project + job resolve).
	for _, seg := range hostileSegments {
		urls = append(urls, hostileURL{"/projects/" + base + "/jobs/wood_oak/files/" + seg, "file segment " + seg})
	}
	// Attempts to reach the secrets by name.
	urls = append(urls,
		hostileURL{"/projects/" + base + "/jobs/wood_oak/files/secret.txt", "secret.txt inside the job"},
		hostileURL{"/projects/" + base + "/jobs/wood_oak/files/session.log", "session.log (not whitelisted)"},
		hostileURL{"/projects/" + filepath.Base(outside) + "/jobs", "unregistered project (secret dir)"},
	)

	for _, u := range urls {
		rec := get(t, srv, u.path, "")
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s (%s): status = %d, want 4xx", u.desc, u.path, rec.Code)
		}
		if body := rec.Body.String(); strings.Contains(body, "TOP-SECRET") {
			t.Errorf("%s (%s): response leaks secret content: %s", u.desc, u.path, body)
		}
	}

	// Raw `..` / `.` path segments are redirected by ServeMux's own
	// sanitization before routing — still never served, never read.
	for _, seg := range []string{"..", "."} {
		rec := get(t, srv, "/projects/"+seg+"/jobs", "")
		if rec.Code < 300 || rec.Code >= 400 {
			t.Errorf("raw %q segment: status = %d, want a 3xx sanitization redirect", seg, rec.Code)
		}
	}
}

// TestResolveJobNeverTreatsSegmentAsPath: the job choke point matches against
// discovered jobs (ID, name, unique prefix) — a segment that is a valid
// identifier but matches nothing is a 404, and no job path is ever derived
// from the segment itself.
func TestResolveJobNeverTreatsSegmentAsPath(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)

	// A valid-looking identifier that matches nothing → 404.
	rec := get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/wood_birch/files/brief.md", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unmatched job segment: status = %d, want 404", rec.Code)
	}
	// The resolved job's files are read from the discovered job's Dir — never
	// from a path built from the URL.
	rec = get(t, srv, "/projects/"+filepath.Base(root)+"/jobs/wood_oak/files/brief.md", "")
	if rec.Code != http.StatusOK {
		t.Errorf("matched job segment: status = %d, want 200", rec.Code)
	}
}

// TestHostileURLsRejectedOnMutatingSurface extends the hostile-segment suite
// to job two's mutating endpoints: every new URL position (the {agent} and
// {name} segments, and the mutating methods on the existing {project}/{job}
// positions) must reject encoded traversal with a 4xx and never leak content
// from outside the registered roots. Mutating requests are never allowed to
// reach a handler's critical section with a hostile segment — validation
// happens in the same resolveProject/resolveJob/validSegment choke points the
// read-only surface uses.
func TestHostileURLsRejectedOnMutatingSurface(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("TOP-SECRET-MUTATE"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A checkout so agentlist.Discover succeeds (the launch-agent handler
	// lists agents before validating the {agent} segment) — the hostile
	// segment then correctly 404s as an unknown agent rather than hitting the
	// "cannot list agents" 500 path.
	fakeCheckout(t, map[string]string{
		"analyst": "name: analyst\ndescription: Breaks requests into tasks.\n",
	})
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	reg := &Registry{entries: []Entry{entryFor(root)}}
	srv := New(reg, "test-version", "", nil)
	base := filepath.Base(root)

	type mutURL struct {
		method, path string
		desc         string
	}
	var urls []mutURL

	// Project position on the create route (the one mutating route with a
	// {project} segment and no {job}).
	for _, seg := range hostileSegments {
		urls = append(urls, mutURL{http.MethodPost, "/projects/" + seg + "/jobs", "create project segment " + seg})
	}
	// Job position on every mutating route.
	for _, seg := range hostileSegments {
		urls = append(urls,
			mutURL{http.MethodPut, "/projects/" + base + "/jobs/" + seg + "/files/brief", "edit-brief job segment " + seg},
			mutURL{http.MethodPost, "/projects/" + base + "/jobs/" + seg + "/agents/analyst", "launch-agent job segment " + seg},
			mutURL{http.MethodPost, "/projects/" + base + "/jobs/" + seg + "/jdi", "launch-jdi job segment " + seg},
			mutURL{http.MethodPost, "/projects/" + base + "/jobs/" + seg + "/done", "done job segment " + seg},
			mutURL{http.MethodPost, "/projects/" + base + "/jobs/" + seg + "/delete", "delete job segment " + seg},
			mutURL{http.MethodPost, "/projects/" + base + "/jobs/" + seg + "/push", "push job segment " + seg},
		)
	}
	// Agent position: hostile segments as the {agent}.
	for _, seg := range hostileSegments {
		urls = append(urls, mutURL{http.MethodPost, "/projects/" + base + "/jobs/wood_oak/agents/" + seg, "agent segment " + seg})
	}
	// Orphan-name position: hostile segments as the {name}.
	for _, seg := range hostileSegments {
		urls = append(urls, mutURL{http.MethodPost, "/projects/" + base + "/orphans/" + seg + "/delete", "orphan name segment " + seg})
	}
	// Settings routes (job brother): the project position on GET/PUT
	// /projects/{project}/settings — the same resolveProject choke point.
	for _, seg := range hostileSegments {
		urls = append(urls,
			mutURL{http.MethodGet, "/projects/" + seg + "/settings", "settings project segment " + seg},
			mutURL{http.MethodPut, "/projects/" + seg + "/settings", "settings project segment " + seg},
		)
	}

	for _, u := range urls {
		rec := request(t, srv, u.method, u.path, "", "")
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s (%s %s): status = %d, want 4xx", u.desc, u.method, u.path, rec.Code)
		}
		if body := rec.Body.String(); strings.Contains(body, "TOP-SECRET") {
			t.Errorf("%s (%s %s): response leaks secret content: %s", u.desc, u.method, u.path, body)
		}
	}
}
