# Verdict: listener architecture

id: wood
status: open
reviewer: wood
date: 2026-08-27

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `src/internal/serve/registry.go` — explicit config file
(`<checkout>/config/serve.json` via `config.Dir()`, overridable `--registry`,
shape `{"projects": ["/abs/root", ...]}`), no scanning, read once at startup.
Missing file → empty registry (not an error); unreadable/unparseable file and
non-directory entries → error. Root without `docs/` accepted + `WarnMissingDocs`.
`Projects()` ordered copy; `Project()` exact-path-then-unique-base-name with
ambiguity → not-found — the single choke point every handler resolves through.
registry_test.go covers parse, validation, missing-file degrade, non-directory
rejection, duplicate collapse, exact/base-name lookup, ambiguity, docs-less root,
default-path pinning.

TASK-2: PASS
notes: `serve` wired into the dispatcher and printHelp (`src/cmd/mg/main.go`);
`src/cmd/mg/serve.go` — `--addr`/`--port` (127.0.0.1:8080 default), `--registry`,
`--token` with flag > `$MG_SERVE_TOKEN` precedence via `config.EnvValue`,
registry load + `WarnMissingDocs`, `ValidateStartup` guard before bind,
SIGINT/SIGTERM via `signal.NotifyContext`, bounded 10s drain
(`serveShutdownDrain`) with `http.ErrServerClosed` handled. Testable seam:
`serveCommand`'s listener parameter + `Server.Serve`/`Shutdown`. serve_test.go
covers default bind, env token, flag-wins-over-env, non-loopback refusal,
unreadable registry, missing-registry-empty degrade, no-registry-location,
unknown arg, shutdown-closes-listener. `stopServe`'s SIGINT-to-self is safe:
`signal.NotifyContext` registers a process-wide handler, so the test process
survives and only the daemon's context cancels (developer confirmed `-count=3`).

TASK-3: PASS
notes: `server.go` + `api.go` — Go 1.22+ method+wildcard mux, JSON envelope
helpers (no ANSI/glamour; raw markdown for job files), `GET /health`,
`GET /projects`, `GET /projects/{project}/jobs` (TUI info design —
id/status/stage/type/date/title in `job.Discover` sort order, per-job
`ReadJDIStatus` state; name/branch are additive row fields), and
`GET .../files/{file}` with the `brief|tasks|implementation|verdict` whitelist,
404 on missing. Project/job segments resolve only through the TASK-1/TASK-7
choke-point helpers from day one (`resolveProject`/`resolveJob`/`validSegment`).

TASK-4: PASS
notes: jdi endpoint (`ReadJDIStatus` state/agent/updated RFC3339,
`ReadJDIRunLogTail`, session.log bounded tail via `readFileTail` — 256 KiB,
absent → nulls, empty-present → ""), diff endpoint mirrors `mg diff`'s
resolution chain exactly (`resolveJobBranch` = the CLI's exact-then-prefix with
identical wording; base = `project.Load().BaseBranch` else
`git.SymbolicRefHead`; log+stat default, `?full=1` patch; not-found → 404,
ambiguous → 409), agents endpoint (`agentlist.Discover`, name + description
only, never the raw agent file). All git access through `internal/git`.
Non-git-project diff → 500 is a documented, acceptable v1 behavior.

TASK-5: PASS
notes: `session.ImagePresent()` via the stubbable `dockerCommand` seam
(`docker image inspect manigot`), every failure mode degrades to false;
health_test.go pins the invocation and the degrades. `/health` reports version
(passed in from main via `New`), imagePresent, and per-profile readiness
(`ResolveProfile` + `CheckAuth` per `config.Profiles()` — booleans only, never
credential values).

TASK-6: PASS
notes: `auth.go` — `tokenMiddleware` (constant-time `crypto/subtle` compare,
exact `Bearer <token>` form, missing/wrong/non-bearer/length-mismatch → 401,
tokenless when unset); `ValidateStartup` refuses non-loopback (neither
127.0.0.0/8 nor `::1`; "localhost" allowed) with no token — unskippable, no
disabling flag, wired before bind in `serveCommand`. auth_test.go pins the
200/401 matrix, tokenless pass-through, constant-time compare structure,
`IsLoopback` classification, and the startup guard.

TASK-7: PASS
notes: the zero-path-inputs enforcement is airtight. `validSegment` rejects
`.`/`..`, `/`, `\`, NUL on the decoded values (which is what `PathValue`
delivers — see the Security section), and every handler resolves only through
`resolveProject`/`resolveJob`/the file whitelist — a URL segment is never
joined into a filesystem path. `security_test.go` runs a suite of hostile
encoded URLs at every path position, asserting 4xx for the handler-rejected
forms, 3xx for the raw-literal mux-sanitized forms, and — critically — that a
planted secret outside the registered roots never appears in any response.
`TestResolveProjectNeverResolvesOutsideRegistry` and
`TestResolveJobNeverTreatsSegmentAsPath` pin the choke points directly.

TASK-8: PASS
notes: credentials_test.go plants known values for every credential key
(CLAUDE_CODE_OAUTH_TOKEN, account UUIDs, ANTHROPIC_API_KEY, OPENCODE_API_KEY,
ZHIPU_API_KEY, MG_SERVE_TOKEN, the OPENCODE_*_MODEL keys, OPENCODE_THEME) in
the same `config.EnvValue` source the daemon reads (.env + process env), hits
every endpoint (including 401/404 envelopes and the audit log), and asserts no
body contains any value or any full .env line. Error-envelope shape pinned to
exactly `{"error": "..."}`.

TASK-9: PASS
notes: auditMiddleware sits outermost (sees 401s from the token middleware and
mux 404/405s), one line per request: RFC3339 timestamp, client IP (port
stripped), method + path, auth outcome (tokenless/authed/401), response status,
to a caller-supplied `io.Writer` (mg serve passes stderr); never logs the
Authorization header, the token, or bodies. audit_test.go pins per-request
logging incl. 4xx/401s, outcome classification, no-credential-material,
nil-writer no-op.

TASK-10: PASS
notes: `serve.ProjectLocks` keyed by registered root, lazily created per key,
Lock/Unlock serialize within a key and stay independent across keys; doc
comment states job two's mutating handlers MUST use it; v1 reads take no locks.
locks_test.go pins mutual exclusion, cross-key independence, the API shape
(reentrant Lock/Unlock, unmatched Unlock no-op), and a contention stress test.

TASK-11: PASS
notes: docs/AGENTS.md (dispatcher inventory + `serve` in Commands, `internal/serve`
package entry, new "Listener / control plane" subsection covering registry,
read-only API, binding+auth, audit, and the three security invariants);
project-template/docs/AGENTS.md mirror paragraph; README.md command-table row +
"Listener" section (registry location/shape, MG_SERVE_TOKEN, localhost-vs-VPS
rule, TLS via Caddy/nginx, out-of-scope pointer); docs/web-interface.md "Job
one: the daemon" annotated as superseded/DONE per brief recommendation point 4;
main.go help text updated. The two AGENTS.md files stay in sync.

## Security

The security invariants the brief demands are implemented and enforced:
zero path inputs (resolution choke point + validSegment; no URL segment is ever
joined into a filesystem path), credentials never returned (whitelisted
response shapes + whole-surface test), unskippable bind/auth startup guard
(non-loopback without a token refuses to start), constant-time token
comparison, audit trail that never logs credentials, per-project serialization
skeleton for job two. No security hole found in the server code.

## Previous NEEDS WORK verdict — resolved (verified against the Go stdlib source)

The prior reviewer's three blockers asserted that Go's ServeMux 301-redirects
single-encoded traversal segments (`..%2f..%2fetc%2fpasswd`, `%2e%2e`, `%2e`,
`.%2e`, `%2e%2e%2f%2e%2e%2fetc%2fpasswd`, `%2fetc%2fpasswd`) before any
handler runs, so the tests' "want 4xx" assertions would fail. That analysis
describes the **pre-Go-1.22** mux. I verified the actual behavior against the
Go 1.24 standard library source installed on this machine (the repo's go.mod
requires go 1.23; toolchain here is Go 1.24.4, new mux by default — `use121`
only activates under `GODEBUG=httpmuxgo121=1`):

- `net/http/server.go` `findHandler`: `escapedPath := r.URL.EscapedPath();
  path := escapedPath; ... path = cleanPath(path)`, and the 301 redirect fires
  only when `path != escapedPath`. `cleanPath` therefore runs on the **escaped**
  path, where a segment spelled `%2e%2e` or `..%2f..%2fetc%2fpasswd` is not the
  literal `.`/`..`/`//` cleanPath looks for — no redirect.
- `net/http/routing_tree.go` `firstSegment` ("The segment is returned
  unescaped, if possible") + `pattern.go` `pathUnescape`: wildcard segments are
  unescaped per segment, so the decoded hostile value (`..`, `.`,
  `../../etc/passwd`, `/etc/passwd`) lands in `PathValue`.
- The server's `validSegment` rejects every decoded hostile value → 404,
  exactly as the tests assert; the raw-literal `..`/`.` forms produce
  `path != escapedPath` → 301, which the tests assert as 3xx separately.

The developer's retry commit is comment-only on security_test.go (documenting
this mechanism) — no assertion was weakened. All three prior blockers are
refuted; the committed tests are correct as written. (I could not execute the
suite in this session — git read/commit-only — so this rests on static review
plus the stdlib-source verification above; the developer's `go test ./...`,
`-count=3`, `go vet ./...` and live `mg serve` smoke test were run with a real
Go 1.24.4 toolchain.)

## Non-blocking notes (not merge blockers)

- `TestServeShutdownSeam` (api_test.go) uses `t.Context()`, which requires Go
  1.24, while go.mod declares `go 1.23` — a 1.23 toolchain cannot compile that
  test file. Works with the installed 1.24.4; consider bumping go.mod to 1.24
  or using `context.WithTimeout` for a 1.23-clean build.
- `startServe`'s `strings.Builder` stdout/stderr are written by the serveCommand
  goroutine and read by the test goroutine with only network polling as the
  ordering signal — a latent `go test -race` finding; `make check` runs without
  `-race` and real-world ordering makes it safe in practice.
- `/diff` on a non-git project returns 500 ("cannot read branches") — correct
  per the v1 design, documented in implementation.md.

## Overall

APPROVED

The listener daemon is complete, matches the brief and all eleven tasks, and
its security posture holds up to review: the read-only API resolves every
request exclusively through the registry/jobs/whitelist choke points, no URL
segment is ever joined into a filesystem path, credentials are never returned
or logged, the non-loopback-without-token startup guard is unskippable and
test-pinned, and the per-project serialization skeleton is in place for job
two. The three test blockers from the prior review are refuted — the disputed
ServeMux behavior was verified against the Go 1.24 standard library source on
this machine, confirming the developer's account and the correctness of the
committed tests. No changes required before merge.