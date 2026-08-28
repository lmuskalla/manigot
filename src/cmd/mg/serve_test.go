package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// emptyRegistryFile writes an empty registry config ({"projects": []}) to a
// temp path and returns it — the --registry flag value for tests that don't
// need any registered projects.
func emptyRegistryFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "serve.json")
	if err := os.WriteFile(path, []byte(`{"projects": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// startServe launches serveCommand in a goroutine with a pre-bound loopback
// listener (the testable seam) and waits until the server answers /health,
// returning the base URL and the exit-code channel. The caller must drain
// done — typically by sending SIGINT and reading the exit code, or with
// t.Cleanup.
func startServe(t *testing.T, args []string) (base string, done chan int, stdout, stderr *strings.Builder) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base = "http://" + ln.Addr().String()
	done = make(chan int, 1)
	stdout = &strings.Builder{}
	stderr = &strings.Builder{}
	go func() { done <- serveCommand(args, stdout, stderr, ln) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(base + "/health")
		if err == nil {
			resp.Body.Close()
			// 200 (tokenless) or 401 (token configured) both mean the server
			// is up and the middleware chain is live.
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not come up in time (last err %v)", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return base, done, stdout, stderr
}

// stopServe sends SIGINT (the daemon's shutdown signal) and waits for the
// exit code.
func stopServe(t *testing.T, done chan int) int {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not shut down within 10s of SIGINT")
		return -1
	}
}

// TestServeDefaultBindAndServes: the default flags bind a loopback listener
// and the daemon answers /health; SIGINT shuts it down gracefully with exit
// code 0.
func TestServeDefaultBindAndServes(t *testing.T) {
	base, done, stdout, _ := startServe(t, []string{"--registry", emptyRegistryFile(t)})
	defer func() {
		if code := stopServe(t, done); code != 0 {
			t.Errorf("exit code after SIGINT = %d, want 0", code)
		}
	}()

	resp, err := http.Get(base + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (tokenless default)", resp.StatusCode)
	}
	if !strings.Contains(stdout.String(), "listening on") {
		t.Errorf("stdout = %q, want the listening announcement", stdout.String())
	}
}

// TestServeTokenFromEnv: with no --token flag, $MG_SERVE_TOKEN (read via
// config.EnvValue, the same path the daemon uses for every credential)
// configures auth — a request without it is a 401, with it a 200.
func TestServeTokenFromEnv(t *testing.T) {
	// Isolate the checkout so config.EnvValue falls through to the process
	// environment (no .env in a fresh fake checkout).
	t.Setenv("MANIGOT_HOME", t.TempDir())
	t.Setenv("MG_SERVE_TOKEN", "env-token")
	base, done, _, _ := startServe(t, []string{"--registry", emptyRegistryFile(t)})
	defer func() {
		if code := stopServe(t, done); code != 0 {
			t.Errorf("exit code after SIGINT = %d, want 0", code)
		}
	}()

	resp, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401 (env token must be enforced)", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, base+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer env-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("with env token: status = %d, want 200", resp.StatusCode)
	}
}

// TestServeTokenFlagWinsOverEnv: an explicit --token flag takes precedence
// over $MG_SERVE_TOKEN — the env token must no longer authenticate.
func TestServeTokenFlagWinsOverEnv(t *testing.T) {
	t.Setenv("MANIGOT_HOME", t.TempDir())
	t.Setenv("MG_SERVE_TOKEN", "env-token")
	base, done, _, _ := startServe(t, []string{"--registry", emptyRegistryFile(t), "--token", "flag-token"})
	defer func() {
		if code := stopServe(t, done); code != 0 {
			t.Errorf("exit code after SIGINT = %d, want 0", code)
		}
	}()

	req, _ := http.NewRequest(http.MethodGet, base+"/health", nil)
	req.Header.Set("Authorization", "Bearer env-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("env token after --token flag: status = %d, want 401 (flag must win)", resp.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodGet, base+"/health", nil)
	req2.Header.Set("Authorization", "Bearer flag-token")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("flag token: status = %d, want 200", resp2.StatusCode)
	}
}

// TestServeRefusesNonLoopbackWithoutToken pins the startup guard wiring: a
// non-loopback bind with no token exits non-zero before ever listening.
func TestServeRefusesNonLoopbackWithoutToken(t *testing.T) {
	var stdout, stderr strings.Builder
	code := serveCommand([]string{"--registry", emptyRegistryFile(t), "--addr", "0.0.0.0"}, &stdout, &stderr, nil)
	if code == 0 {
		t.Fatal("serveCommand with 0.0.0.0 and no token = exit 0, want refusal")
	}
	if !strings.Contains(stderr.String(), "refusing to start") {
		t.Errorf("stderr = %q, want the refusal message", stderr.String())
	}
}

// TestServeRefusesUnreadableRegistryPath: a registry path that exists but
// cannot be read as a config file is a clear error and non-zero exit. (A
// MISSING file is deliberately the opposite — an empty registry, not an
// error; see TestServeMissingRegistryIsEmptyRegistry. The registry degrade
// is pinned in internal/serve's registry_test.go.)
func TestServeRefusesUnreadableRegistryPath(t *testing.T) {
	// A directory is present but unreadable as a file — os.ReadFile fails
	// with a non-IsNotExist error, which LoadRegistry surfaces.
	dir := t.TempDir()
	var stdout, stderr strings.Builder
	code := serveCommand([]string{"--registry", dir}, &stdout, &stderr, nil)
	if code == 0 {
		t.Fatal("serveCommand with an unreadable registry = exit 0, want an error")
	}
	if !strings.Contains(stderr.String(), "registry") {
		t.Errorf("stderr = %q, want a registry error", stderr.String())
	}
}

// TestServeMissingRegistryIsEmptyRegistry pins the missing-file degrade
// end-to-end through serveCommand: a registry config that does not exist yet
// is an empty registry (the daemon starts, serving nothing), not a startup
// error.
func TestServeMissingRegistryIsEmptyRegistry(t *testing.T) {
	base, done, stdout, _ := startServe(t, []string{"--registry", filepath.Join(t.TempDir(), "missing.json")})
	defer func() {
		if code := stopServe(t, done); code != 0 {
			t.Errorf("exit code after SIGINT = %d, want 0", code)
		}
	}()

	resp, err := http.Get(base + "/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /projects with empty registry: status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(stdout.String(), "no projects registered") {
		t.Errorf("stdout = %q, want the no-projects warning", stdout.String())
	}
}

// TestServeRefusesNoRegistryLocation: with no --registry and no resolvable
// checkout (the default registry path cannot be determined), mg serve exits
// with the clear ErrNoRegistryPath error.
func TestServeRefusesNoRegistryLocation(t *testing.T) {
	// Hide any real checkout from both the env and the executable/cwd
	// fallbacks.
	t.Setenv("MANIGOT_HOME", "")
	t.Setenv("PATH", t.TempDir())
	var stdout, stderr strings.Builder
	code := serveCommand(nil, &stdout, &stderr, nil)
	if code == 0 {
		t.Fatal("serveCommand with no registry location = exit 0, want an error")
	}
	if !strings.Contains(stderr.String(), "registry") {
		t.Errorf("stderr = %q, want the no-registry-location error", stderr.String())
	}
}

// TestServeUnknownArgument: an unexpected positional argument is a clear
// error.
func TestServeUnknownArgument(t *testing.T) {
	var stdout, stderr strings.Builder
	code := serveCommand([]string{"--registry", emptyRegistryFile(t), "stray"}, &stdout, &stderr, nil)
	if code == 0 {
		t.Fatal("serveCommand with a stray argument = exit 0, want an error")
	}
	if !strings.Contains(stderr.String(), "Unknown argument: stray") {
		t.Errorf("stderr = %q, want the unknown-argument error", stderr.String())
	}
}

// TestServeShutdownClosesListener: after SIGINT and graceful shutdown, the
// listener is closed — a request fails instead of hanging or being served.
func TestServeShutdownClosesListener(t *testing.T) {
	base, done, _, _ := startServe(t, []string{"--registry", emptyRegistryFile(t)})
	if code := stopServe(t, done); code != 0 {
		t.Fatalf("exit code after SIGINT = %d, want 0", code)
	}
	// The listener is gone — the request must fail (connection refused), not
	// hang.
	client := &http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Get(base + "/health"); err == nil {
		resp.Body.Close()
		t.Error("server still answering after shutdown")
	}
}
