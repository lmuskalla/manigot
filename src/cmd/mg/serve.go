// mg serve — the listener: a long-running daemon exposing a read-only control
// API over a registry of project roots, so any surface (a web UI, a native
// GUI, a future CLI) can attach to it as a client — from localhost or from a
// VPS. This is job one of the control-plane sequence described in
// docs/web-interface.md and the listener brief: `mg serve` + project registry
// + read-only API + localhost default + audit log + serialization skeleton.
//
// The daemon is additive: the TUI stays in-process, and every existing
// command keeps working untouched. It binds to 127.0.0.1:8080 by default
// (tokenless — the machine's own user is the trust boundary, as it is for the
// CLI today); a non-loopback bind REQUIRES a bearer token (--token or
// $MG_SERVE_TOKEN) or the daemon refuses to start. TLS is the reverse proxy's
// job (Caddy/nginx), never the daemon's.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/serve"
)

// serveShutdownDrain bounds how long mg serve waits for in-flight requests to
// finish on SIGINT/SIGTERM before giving up — a hung request must never wedge
// the exit.
const serveShutdownDrain = 10 * time.Second

// runServe implements `mg serve` — the listener daemon. It parses the flags,
// loads the project registry, enforces the bind/auth startup guard, then
// blocks serving until SIGINT/SIGTERM, shutting down gracefully with a
// bounded drain. Returns the process exit code.
func runServe(args []string, stdout, stderr io.Writer) int {
	return serveCommand(args, stdout, stderr, nil)
}

// serveCommand is runServe's testable core: it differs only in the listener
// source. When ln is non-nil it serves on that already-bound listener (tests
// hand over a bound 127.0.0.1:0 listener); when nil it binds one itself from
// the --addr/--port flags. Signal handling and the graceful shutdown path are
// identical in both shapes.
func serveCommand(args []string, stdout, stderr io.Writer, ln net.Listener) int {
	fs := flag.NewFlagSet("mg serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1", "bind host (loopback default; a non-loopback bind requires a token)")
	port := fs.Int("port", 8080, "bind port")
	registry := fs.String("registry", "", "path to the project registry config (default <checkout>/config/serve.json)")
	tokenFlag := fs.String("token", "", "bearer token required on every request (default $MG_SERVE_TOKEN)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "mg serve %s\n\n", version)
		fmt.Fprintf(stderr, "The listener: a long-running daemon exposing a read-only control API\n")
		fmt.Fprintf(stderr, "over a registry of project roots (the basis for the web UI / native GUI\n")
		fmt.Fprintf(stderr, "control-plane sequence).\n\n")
		fmt.Fprintf(stderr, "Usage:\n")
		fmt.Fprintf(stderr, "  mg serve [flags]\n\n")
		fmt.Fprintf(stderr, "The registry config file holds the named project roots the daemon serves:\n")
		fmt.Fprintf(stderr, "  {\"projects\": [{\"name\": \"my-project\", \"path\": \"/abs/project/root\"}, ...]}\n")
		fmt.Fprintf(stderr, "\"name\" is the URL segment the project is served under (a single URL-safe\n")
		fmt.Fprintf(stderr, "path segment, unique across the registry); it is required and chosen by you.\n")
		fmt.Fprintf(stderr, "Default location: <manigot checkout>/config/serve.json (override with --registry).\n")
		fmt.Fprintf(stderr, "Changing the registered roots means editing the file and restarting.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2 // the flag package already printed the error + usage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "Unknown argument: %s\n", fs.Arg(0))
		return 1
	}

	// Token precedence: --token flag, then $MG_SERVE_TOKEN (config.EnvValue —
	// the .env file first, then the process environment, exactly like every
	// other credential the daemon reads). A token is never issued, created, or
	// returned by the API.
	token := *tokenFlag
	if token == "" {
		token = config.EnvValue("MG_SERVE_TOKEN")
	}

	// The project registry: an explicit config file of project roots, read
	// once at startup. No scanning, no auto-adopting.
	registryPath := *registry
	if registryPath == "" {
		registryPath = serve.DefaultRegistryPath()
		if registryPath == "" {
			cliError(stderr, serve.ErrNoRegistryPath)
			return 1
		}
	}
	reg, err := serve.LoadRegistry(registryPath)
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	reg.WarnMissingDocs(stderr)

	// The startup guard — the hard invariant: a non-loopback bind with no
	// token refuses to start. Unskippable: there is no flag that turns it off.
	if err := serve.ValidateStartup(*addr, token); err != nil {
		cliError(stderr, err)
		return 1
	}

	srv := serve.New(reg, version, token, stderr)

	// Bind (or accept the test-supplied listener) and announce.
	if ln == nil {
		bindAddr := net.JoinHostPort(*addr, strconv.Itoa(*port))
		ln, err = net.Listen("tcp", bindAddr)
		if err != nil {
			cliError(stderr, fmt.Errorf("mg serve: cannot listen on %s: %v", bindAddr, err))
			return 1
		}
	}
	fmt.Fprintf(stdout, "mg serve %s — listening on %s\n", version, ln.Addr())
	if token != "" {
		fmt.Fprintln(stdout, "Bearer-token auth enabled — every request must carry Authorization: Bearer <token>.")
		fmt.Fprintln(stdout, "TLS is the reverse proxy's job (Caddy/nginx); the daemon always serves plain HTTP.")
	} else {
		fmt.Fprintln(stdout, "No token configured — tokenless mode (safe only on a loopback bind).")
	}
	if len(reg.Projects()) == 0 {
		fmt.Fprintln(stdout, "Warning: no projects registered — edit the registry config and restart to serve any.")
	}

	// Serve until SIGINT/SIGTERM, then drain in-flight requests within a
	// bounded window and exit cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			cliError(stderr, fmt.Errorf("mg serve: %v", err))
			return 1
		}
		return 0
	case <-ctx.Done():
		drainCtx, cancel := context.WithTimeout(context.Background(), serveShutdownDrain)
		defer cancel()
		if err := srv.Shutdown(drainCtx); err != nil {
			cliError(stderr, fmt.Errorf("mg serve: graceful shutdown: %v", err))
			return 1
		}
		<-errCh // the serve loop returns http.ErrServerClosed
		fmt.Fprintln(stdout, "mg serve: shutting down.")
		return 0
	}
}
