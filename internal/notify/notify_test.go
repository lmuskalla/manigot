package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCheckout points $MANIGOT_HOME at a temp dir with an optional .env so
// config.EnvValue resolves there instead of at the real checkout (and falls
// back to the process environment for keys the fake .env does not set).
func fakeCheckout(t *testing.T, env string) {
	t.Helper()
	dir := t.TempDir()
	if env != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MANIGOT_HOME", dir)
}

func TestFromConfigDefaults(t *testing.T) {
	fakeCheckout(t, "")
	t.Setenv("NTFY_URL", "")
	t.Setenv("NTFY_TOPIC", "")
	t.Setenv("NTFY_TOKEN", "")

	c := FromConfig()
	if c.URL != DefaultURL {
		t.Errorf("URL = %q, want default %q", c.URL, DefaultURL)
	}
	if c.Enabled() {
		t.Error("Enabled = true with no NTFY_TOPIC, want false")
	}
}

func TestFromConfigReadsEnv(t *testing.T) {
	fakeCheckout(t, "NTFY_URL=https://ntfy.example.com/\nNTFY_TOPIC=my-topic\nNTFY_TOKEN=tok\n")
	// Clear the process-env fallbacks so only the fake .env can supply values.
	t.Setenv("NTFY_URL", "")
	t.Setenv("NTFY_TOPIC", "")
	t.Setenv("NTFY_TOKEN", "")

	c := FromConfig()
	// A trailing slash on NTFY_URL is tolerated.
	if c.URL != "https://ntfy.example.com" {
		t.Errorf("URL = %q, want %q", c.URL, "https://ntfy.example.com")
	}
	if c.Topic != "my-topic" || c.Token != "tok" {
		t.Errorf("Topic/Token = %q/%q, want my-topic/tok", c.Topic, c.Token)
	}
	if !c.Enabled() {
		t.Error("Enabled = false with NTFY_TOPIC set, want true")
	}
}

func TestPublishDisabledIsNoOp(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()

	c := Client{URL: srv.URL, HTTP: srv.Client()}
	if err := c.Publish(Message{Message: "should not be sent"}); err != nil {
		t.Fatalf("Publish on a disabled client returned an error: %v", err)
	}
	if requests != 0 {
		t.Errorf("server received %d requests, want 0 (disabled client must not send)", requests)
	}
}

func TestPublishSendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := Client{URL: srv.URL, Topic: "secret-topic", Token: "tok", HTTP: srv.Client()}
	err := c.Publish(Message{Title: "mg jdi: job finished", Message: "the reason", Priority: 4, Tags: []string{"warning"}})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/secret-topic" {
		t.Errorf("path = %q, want %q", gotPath, "/secret-topic")
	}
	if gotBody != "the reason" {
		t.Errorf("body = %q, want %q", gotBody, "the reason")
	}
	if gotHeaders.Get("Title") != "mg jdi: job finished" {
		t.Errorf("Title header = %q", gotHeaders.Get("Title"))
	}
	if gotHeaders.Get("Priority") != "4" {
		t.Errorf("Priority header = %q, want 4", gotHeaders.Get("Priority"))
	}
	if gotHeaders.Get("Tags") != "warning" {
		t.Errorf("Tags header = %q, want warning", gotHeaders.Get("Tags"))
	}
	if gotHeaders.Get("Authorization") != "Bearer tok" {
		t.Errorf("Authorization header = %q, want %q", gotHeaders.Get("Authorization"), "Bearer tok")
	}
}

func TestPublishOmitsOptionalHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := Client{URL: srv.URL, Topic: "t", HTTP: srv.Client()}
	if err := c.Publish(Message{Message: "body"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	for _, h := range []string{"Title", "Priority", "Tags", "Authorization"} {
		if gotHeaders.Get(h) != "" {
			t.Errorf("%s header = %q, want absent for an empty value", h, gotHeaders.Get(h))
		}
	}
}

func TestPublishServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := Client{URL: srv.URL, Topic: "t", HTTP: srv.Client()}
	err := c.Publish(Message{Message: "body"})
	if err == nil {
		t.Fatal("Publish with a 500 response: expected an error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to mention the status code", err)
	}
}

// TestPublishTransportErrorRedactsURL guards the "never log the URL+token
// combination" rule: an http.Client transport failure embeds the request URL
// in its error, which for ntfy contains the topic (effectively a password) —
// Publish must strip it before returning.
func TestPublishTransportErrorRedactsURL(t *testing.T) {
	// A server that accepts the connection and immediately closes it produces
	// a transport error whose *url.Error carries the full request URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	defer srv.Close()

	c := Client{URL: srv.URL, Topic: "super-secret-topic", HTTP: srv.Client()}
	err := c.Publish(Message{Message: "body"})
	if err == nil {
		t.Fatal("Publish against a closing server: expected an error")
	}
	if strings.Contains(err.Error(), "super-secret-topic") {
		t.Errorf("error leaks the topic (a password): %q", err)
	}
	if strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error leaks the server URL: %q", err)
	}
}

// TestPublishBuildErrorRedactsURL guards the same "never log the URL+token
// combination" rule on the request-build path: a malformed NTFY_URL makes
// http.NewRequest fail with an error whose string form embeds the full
// request URL — which for ntfy contains the topic (effectively a password).
// Publish must strip it before returning, exactly as it does for transport
// errors (TestPublishTransportErrorRedactsURL).
func TestPublishBuildErrorRedactsURL(t *testing.T) {
	// An interior space in the host survives FromConfig's TrimSpace and makes
	// the concatenated URL unparseable — a plausible misconfiguration.
	c := Client{URL: "http://my ntfy.sh", Topic: "super-secret-topic", HTTP: http.DefaultClient}
	err := c.Publish(Message{Message: "body"})
	if err == nil {
		t.Fatal("Publish with a malformed NTFY_URL: expected an error")
	}
	if strings.Contains(err.Error(), "super-secret-topic") {
		t.Errorf("error leaks the topic (a password): %q", err)
	}
	if strings.Contains(err.Error(), "http://my ntfy.sh") {
		t.Errorf("error leaks the server URL: %q", err)
	}
}
