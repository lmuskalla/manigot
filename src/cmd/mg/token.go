// mg serve-token — generate a fresh secure bearer token for `mg serve` and
// write it into the manigot checkout's .env as MG_SERVE_TOKEN. The daemon
// reads the token once at startup (config.EnvValue: .env first, then the
// process environment), so the new token is picked up on the next start.
// This is the out-of-band token provisioning the listener design calls for —
// the API itself never issues, creates, or returns a token; the operator is
// the only mint, and this command is the ergonomic way to do it.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/lmuskalla/manigot/internal/config"
)

// serveTokenBytes is the token's entropy: 32 random bytes -> 64 hex chars
// (256 bits), the same ballpark as a strong API key.
const serveTokenBytes = 32

// runServeToken implements `mg serve-token`.
func runServeToken(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "Unknown argument: %s\n", args[0])
		return 1
	}

	buf := make([]byte, serveTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		cliError(stderr, fmt.Errorf("generate token: %v", err))
		return 1
	}
	token := hex.EncodeToString(buf)

	if err := config.UpsertEnv("MG_SERVE_TOKEN", token); err != nil {
		cliError(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "Generated a new secure token for mg serve (%d random bytes) and wrote it to %s as MG_SERVE_TOKEN.\n", serveTokenBytes, config.EnvFile())
	fmt.Fprintln(stdout, "It is read once at daemon startup - start mg serve to pick it up.")
	fmt.Fprintln(stdout, "Clients authenticate with: Authorization: Bearer <token>")
	return 0
}