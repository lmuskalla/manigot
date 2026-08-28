# Implementation: listener architecture

id: wood
status: open
developer: wood
date: 2026-08-27

<!-- Produced by @developer after implementation. -->

## Summary

Implemented `mg serve`, the listener daemon — job one of the control-plane
sequence: a long-running process exposing a **read-only** control API over an
explicit registry of project roots, so any surface (web UI, native GUI,
future CLI) can attach to it as a client from localhost or from a VPS. The
daemon is additive (the TUI stays in-process) and is a new trust boundary, so
the v1 surface is read-only by design with the security invariants the brief
demands enforced by tests: zero path inputs (URL segments are never joined
into filesystem paths), credentials never returned by any endpoint and never
logged, bearer-token auth for any non-loopback bind (unskippable startup
guard), a per-request audit trail, and a per-project serialization skeleton
for job two's mutating API.

## Changes

TASK-1: Added `src/internal/serve/registry.go` + `registry_test.go` — the
`Registry` type: loads an explicit config file (`<checkout>/config/serve.json`,
overridable via `--registry`, shape `{"projects": ["/abs/root", ...]}`), no
scanning, read once at startup. Each entry validated as an existing directory;
a missing config file is an empty registry (not an error); a root without
`docs/` is accepted and only warned about (`WarnMissingDocs`). `Projects()`
returns an ordered copy; `Project(name)` resolves exact path then unique base
name — the single choke point every handler resolves through.

TASK-2: Added `src/cmd/mg/serve.go` + `serve_test.go` and wired `serve` into
the dispatcher and help text in `src/cmd/mg/main.go`. `runServe` parses
`--addr`/`--port` (localhost `127.0.0.1:8080` default), `--registry` and
`--token` (flag, then `$MG_SERVE_TOKEN` via `config.EnvValue`), loads the
registry, enforces the bind/auth startup guard, then serves until
SIGINT/SIGTERM with a bounded graceful drain (`http.Server.Shutdown` with a
10s cap). The serve loop is httptest-able through `serveCommand`'s listener
seam. Note: the original test for an unreadable registry used a *missing*
file, which contradicts TASK-1's missing-file-is-empty-registry degrade and
hung the whole `cmd/mg` package — fixed to use a genuinely unreadable path
(a directory) and added `TestServeMissingRegistryIsEmptyRegistry` pinning the
degrade end-to-end.

TASK-3: Added `src/internal/serve/server.go` + `api.go` + `api_test.go` — the
`net/http` mux (Go 1.22+ method+wildcard patterns), JSON envelope helpers
(never ANSI; raw markdown for job files), and the first read endpoints:
`GET /health`, `GET /projects`, `GET /projects/{project}/jobs` (the TUI's
info design — id/status/stage/type/date/title in `job.Discover` sort order
plus per-job mg-jdi state from `ReadJDIStatus`), and
`GET /projects/{project}/jobs/{job}/files/{file}` with the file segment
whitelisted to `brief|tasks|implementation|verdict` (404 on missing files).
Project and job segments resolve only through the resolution helpers built in
this task.

TASK-4: Extended `api.go`/`api_test.go` — `GET .../jdi` (status + `run.log`
tail via `ReadJDIRunLogTail` + `session.log` bounded tail; absent logs are
nulls, not errors), `GET .../diff` (base branch resolved exactly as `mg diff`
does — `project.Load(root).BaseBranch` falling back to `git.SymbolicRefHead`;
job→branch via the CLI's exact-then-prefix chain; quick eyeball log+stat by
default, `?full=1` for the full patch via `git.Diff`; not-found → 404,
ambiguous → 409 with the CLI's wording), and `GET .../agents`
(`agentlist.Discover`, name + description only). All git access goes through
`internal/git`.

TASK-5: Added `src/internal/session/health.go` + `health_test.go` —
`session.ImagePresent()` (bool) via `docker image inspect manigot` through the
existing stubbable `dockerCommand` seam; every failure mode degrades to
false. `GET /health` reports version (passed in from main), image presence,
and per-profile readiness (`session.ResolveProfile` + `CheckAuth` per
`config.Profiles()` — booleans only, never credential values).

TASK-6: Added `src/internal/serve/auth.go` + `auth_test.go` — bearer-token
auth when a token is configured (constant-time comparison via
`crypto/subtle`, exact `Authorization: Bearer <token>` form, missing/wrong →
401), tokenless when unset. `ValidateStartup` is the unskippable guard: a
non-loopback bind (neither 127.0.0.0/8 nor `::1`) with no token refuses to
start; there is no flag that turns it off. TLS is explicitly the reverse
proxy's job — the daemon always serves plain HTTP.

TASK-7: The zero-path-inputs enforcement, built on the TASK-1/TASK-3 choke
points (`validSegment` + `resolveProject`/`resolveJob`): URL segments are
validated as plain identifiers (rejecting `.`/`..`, `/`, `\`, NUL — decoded
and double-encoded forms) and matched against the registry / discovered jobs
(ID, id_slug name, unique prefix) / the file whitelist — never joined into
paths. `security_test.go` runs a suite of hostile encoded URLs at every path
position, asserting 4xx and — critically — that no response leaks content
from outside the registered roots.

TASK-8: `credentials_test.go` — the "no .env content, no keys, no tokens in
any response, ever" guarantee enforced over the whole surface: known
credential values are planted via the same `config.EnvValue` source the
daemon reads (fixture `.env` + process env), every endpoint is hit (including
401/404 error envelopes), and every body is grepped for the values and for
full `.env` lines. The audit log is asserted to never carry the token or the
Authorization header.

TASK-9: `audit.go` + `audit_test.go` — request-log middleware (outermost in
the chain, so it sees 401s too): one line per request with timestamp, client
IP, method + path, auth outcome (authed/tokenless/401), and response status,
written to a caller-supplied `io.Writer` (mg serve passes stderr). Never logs
the Authorization header, the token, or request bodies.

TASK-10: `locks.go` + `locks_test.go` — `serve.ProjectLocks`, the per-project
serialization skeleton keyed by registered project root (mutual exclusion
within a key, independence across keys), with a doc comment stating job two's
mutating handlers MUST use it. v1 read endpoints deliberately take no locks.

TASK-11: Docs + help sync. `docs/AGENTS.md`: `serve` added to the dispatcher
inventory and Commands section, `internal/serve` added to the package
inventory, and a new "Listener / control plane" architecture subsection
(daemon, project registry, read-only API surface, binding + bearer-token auth
model, audit trail, security invariants). `project-template/docs/AGENTS.md`:
a condensed mirror paragraph in the context comment (the two AGENTS.md files
describe the same system). `README.md`: an `mg serve` command-table row plus
a "Listener" section (registry config location/shape, `MG_SERVE_TOKEN`,
localhost-vs-VPS binding rule, TLS via Caddy/nginx, pointer to the
out-of-scope list). `docs/web-interface.md`: its "Job one: the daemon"
recommendation annotated as superseded by the listener decision (short
annotation per the brief, not a rewrite).

## Known issues / follow-ups

- **Environmental test failures in this session:** the session git shim
  refuses `git init`, so the pre-existing lifecycle/diff/ui/job/git tests that
  build throwaway git repos fail under an agent session (`manigot: git 'init'
  is not allowed in agent sessions`). These are unrelated to this job and pass
  in a normal environment; the serve package's own tests that need real git
  (diff endpoint) bypass the shim via `pathWithRealGitOnly`.
- **`/diff` on a non-git project** returns a 500 ("cannot read branches") —
  correct per the v1 design (diff needs git), surfaced as a 500 rather than a
  404; a later job may want to distinguish "not a git repo" more gracefully.
- The registry file is read once at startup (restart to change) and tokens are
  configured out-of-band — both deliberate v1 constraints, documented in
  docs/AGENTS.md and the README.

## Reviewer blockers — resolution (developer retry)

The reviewer's NEEDS WORK verdict listed three blockers, all in the committed
test suite, all based on static analysis of Go's ServeMux without a Go
toolchain ("no Go toolchain is available in this session"). Re-verified with a
real toolchain (Go 1.24.4): **all three are refuted — the tests pass exactly
as committed, and no code or test change was required.** Full suite:
`go test ./...` (plus `-count=3` on the flagged tests and `go vet ./...`) all
green; a live `mg serve` smoke test (health/projects/jobs/agents/404 + graceful
SIGINT shutdown + audit lines) confirmed the daemon end-to-end.

- **Blocker 1** (`security_test.go` `TestHostileURLsRejectedAtEveryPathPosition`,
  ~30 URL cases): the reviewer asserted the six single-encoded segments
  (`..%2f..%2fetc%2fpasswd`, `%2e%2e`, `%2e`, `.%2e`,
  `%2e%2e%2f%2e%2e%2fetc%2fpasswd`, `%2fetc%2fpasswd`) get a 301 sanitization
  redirect, so the "want 4xx" assertion fails. Wrong on the mechanism:
  ServeMux's cleanPath runs on the ESCAPED path (`EscapedPath`), where a
  segment spelled `%2e%2e` is not the literal `.`/`..` cleanPath looks for.
  Verified with a probe program (httptest-direct AND over a real socket): all
  six encoded forms reach the handler with the decoded hostile value in
  PathValue (bare mux answers 200), and only the raw literal forms
  (`/projects/..`) are redirected. The server's validSegment rejects the
  decoded values → 404, exactly as the tests assert; the raw-literal forms are
  separately asserted as 3xx in the same test. The leak-assertion (a planted
  secret in an unregistered dir never appears) passes.
- **Blocker 2** (`api_test.go` `TestHandleProjectJobsUnknownProjectIs404`:
  `%2e%2e`, `%2e`, `%2fetc`, `..%2f..%2fetc%2fpasswd` return 301, not 404) and
  **Blocker 3** (`api_test.go` `TestHandleJobFileNonWhitelistedIs404`:
  `..%2fbrief.md`, `..%2f..%2fetc%2fpasswd`, `%2e%2e%2fbrief.md`): same false
  premise, same empirical refutation — these encoded segments reach the
  handler and are rejected by validSegment/whitelist with 404. The tests
  already assert the raw-`..`/`.` 3xx redirect separately.
- **Caveat** (`serve_test.go` `stopServe` sends SIGINT to the test process):
  confirmed clean. `serveCommand`'s `signal.NotifyContext` registers a
  process-wide SIGINT handler, so the test process survives and only the
  daemon's context cancels; the serve tests pass repeatedly (`-count=3`).
- **Change made:** one comment-only edit to `security_test.go` documenting the
  escaped-path cleanPath behavior and why the encoded forms reach the handler
  — so a future reader/reviewer cannot repeat the static-analysis mistake. No
  assertion was weakened; the strict 4xx-for-handler-rejected / 3xx-for-mux-
  sanitized split is preserved.