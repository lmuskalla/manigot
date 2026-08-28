package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/config"
)

// TestServeTokenWritesEnv: mg serve-token writes MG_SERVE_TOKEN into the
// checkout's .env as a 64-hex-char token and preserves existing lines.
func TestServeTokenWritesEnv(t *testing.T) {
	dir := profileCheckout(t, "# header\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret\n")
	var out, errOut strings.Builder
	code := runServeToken(nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, errOut.String())
	}

	token := config.GetEnv("MG_SERVE_TOKEN")
	if len(token) != 64 {
		t.Errorf("MG_SERVE_TOKEN length = %d, want 64 (32 random bytes hex)", len(token))
	}
	for _, c := range token {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("token contains a non-hex character %q", c)
			break
		}
	}

	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-secret") {
		t.Errorf(".env lost an existing credential:\n%s", env)
	}
}

// TestServeTokenRegenerates: a second run replaces the previous token.
func TestServeTokenRegenerates(t *testing.T) {
	profileCheckout(t, "# header\n")
	var out, errOut strings.Builder
	if code := runServeToken(nil, &out, &errOut); code != 0 {
		t.Fatalf("first exit code = %d, stderr: %s", code, errOut.String())
	}
	first := config.GetEnv("MG_SERVE_TOKEN")
	errOut.Reset()
	if code := runServeToken(nil, &out, &errOut); code != 0 {
		t.Fatalf("second exit code = %d, stderr: %s", code, errOut.String())
	}
	second := config.GetEnv("MG_SERVE_TOKEN")
	if first == "" || first == second {
		t.Errorf("token did not change on regeneration (first %q, second %q)", first, second)
	}
}

// TestServeTokenUnknownArg: an unexpected argument is rejected.
func TestServeTokenUnknownArg(t *testing.T) {
	var out, errOut strings.Builder
	code := runServeToken([]string{"bogus"}, &out, &errOut)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Unknown argument: bogus") {
		t.Errorf("stderr missing error:\n%s", errOut.String())
	}
}