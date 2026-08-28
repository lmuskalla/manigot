# Brief: listener architecture

status: done
type: feature
id: wood
branch: feature/wood_listener-architecture
date: 2026-08-27
author: Leander Muskalla

## What

A previous agent has summarized the job target. Read further below.
This is the direction of manigot.
However, this can ONLY EVER BE IMPLEMENTED if it doesn't introduce security risks.
If we are listening on a port (be it locally or on a VPS), it needs to implement maximum security. Only manigot calls can ever go through. And they need to be secured tightly.
So I'm thinking if we add listening functionality, 1. we make sure the port cannot be used for anything else 2. we're authing against an API key, token or whatever.
So if we implement adding clients that are authed to listen to, they need to get a key.

------

listener.md.

# Listener: one control plane on a port — any surface attaches

Exploration notes (2026-08-27). Verdict: **SHIP** — recorded as the basis for
the listener job (job one of the control-plane sequence). This resolves the
fork that `docs/gui.md` and `docs/web-interface.md` both hit the same day:
instead of choosing between a web UI and a native GUI as *the* surface, the
daemon is built once, and every surface — web UI, native GUI, future CLI —
becomes an interchangeable client of the same API. No third surface; one
foundation, two frontends, built one at a time.

## Decision recorded (2026-08-27)

- **The listener is the product; the GUI is a client of it.** "Make mg listen
  on a port" is the buildable unit. The GUI is the last slice, not the first.
- **The web-vs-GUI fork is dissolved.** Both `gui.md` and `web-interface.md`
  concluded REVISIT because a surface had to be chosen before the foundations
  existed. A shared control API removes the choice: local or VPS, browser or
  desktop — same daemon, same API, different binding and auth.
- **Control plane only, on every surface.** No interactive agent terminals in
  any frontend until roadmap item 6 (in-TUI embedded terminal) lands. That bet
  is built once, on the surface the product commits to — not twice in two
  frontends.
- **The TUI stays in-process.** The listener is additive. Forcing the TUI
  through the API in v1 buys nothing and risks the working 70%.
- **One frontend at a time.** The listener makes the second surface cheap (one
  shared API client, one shared info-design port) — not free. Build one first;
  the second is a smaller job only after the first proves the need.
- **The daemon is a state server, not a command socket.** Its value is live
  state — jobs, runs, files, diffs — with commands attached. A pure
  "command-execution over HTTP" listener is a worse TUI with more pixels.

## The idea

A long-running `mg serve` process that listens on a port and exposes a control
API over the existing in-process machinery — `internal/job`, `internal/git`,
the `mg jdi` state machine, the session launcher — so anything can connect to
it: a web UI, a native GUI, a future CLI, from localhost or from a VPS, and
control everything.

The same daemon serves both deployment shapes the user asked for:

- **Local:** bound to `127.0.0.1`, no auth (the machine's own user is the
  trust boundary, as it is for the CLI today).
- **Remote (VPS):** bound to an address behind a TLS reverse proxy, bearer
  token required.

The process model changes: today every `mg` invocation is a one-shot CLI that
discovers a single project from `$PWD`. The daemon is long-lived and holds a
**project registry** — multi-project is a new concept for manigot.

## What already works

The heavy machinery is terminal-independent and in-process-callable today:

- **`internal/job`** — `CreateJob`, `FinishJob`, `DeleteJob`, `Discover`,
  `ReadJDIStatus` are pure functions over a project root. An HTTP handler can
  call the same functions in-process. No subprocess mg, no TTY.
- **`internal/git`** — worktree create/remove, squash merge, push, diff,
  commits, branch resolution (the job id → branch → worktree matching the CLI
  already uses: exact then prefix).
- **`mg jdi`** — fully unattended via the `--print` path, pollable status
  sidecars (`.manigot/jdi-status/`), `NEEDS-HUMAN-INPUT:` markers, ntfy
  notifications. This is the state model the API renders.
- **`session.log`** — roadmap #5's writer side has landed: every
  non-interactive invocation's step-level output is persisted to the job.
  The reader side (streaming it live) is part of job two below.
- **The sweep-commit** — every job-worktree session ends committed by the host.
  What makes headless server operation safe in the first place.
- **Docker isolation** — the containment story is the product's core safety
  property and works identically on a server. The daemon does not touch it.
- **Credentials + profiles** — resolved via the existing `internal/config`
  layer; the daemon reads them exactly as the session launcher does.

## The listener job — scope

The basis for `mg job`. Scope is the daemon and a **read-only** API.

### In scope

1. **`mg serve` subcommand** — a new subcommand in the single binary's
   dispatcher (like `tui` and `jdi`), long-running, listening on a port.
   Bind address/port via flags with a sensible localhost default.
2. **Project registry** — an explicit config file of project roots. No
   scanning, no auto-adopting directories the daemon finds. Registrations are
   read at startup; changing them means editing the config and restarting
   (v1). The daemon resolves nothing outside the registered roots.
3. **Read-only API** — the state the TUI renders today, exposed over HTTP:
   - projects list
   - jobs per project — the TUI's info design: id / status / stage / type /
     date / title, one row per job
   - job files — brief / tasks / implementation / verdict (read)
   - jdi status + `run.log` + `session.log` (read)
   - job diff — the `mg diff` quick eyeball (log + stat; full patch behind a
     flag), resolved against the job's base branch
   - agents available to a project (the `mg agents` list)
   - a health/status endpoint (version, image present, profiles ready)
4. **Binding and auth** — localhost default, no auth; remote exposure requires
   a bearer token. TLS is the reverse proxy's job (Caddy/nginx), not the
   daemon's. Token configured out-of-band (config/env), never issued by the
   API.
5. **Zero path inputs** — every operation resolved against the registry's
   roots; job IDs resolved via the same branch-matching logic the CLI uses.
   Nothing from the URL is trusted as a filesystem path.
6. **Credentials never returned** — no `.env` content, no keys, no tokens in
   any response, ever.
7. **Audit trail** — a request log (timestamp, client, operation). A remote or
   shared box changes the accountability picture; the local TUI never needed
   "who ran what", the daemon does.
8. **Serialization skeleton** — the concurrency pattern for mutating
   operations (per-project) is established in the daemon's structure now, even
   though v1 exposes no mutating endpoints — `internal/git` has no locking
   today, and job two's mutating API inherits the pattern.

### Out of scope

- **Mutating endpoints** — create/edit/launch/`done`/`delete`/push. That is
  job two.
- **Detached run supervision with streamed logs** — watching a live agent run
  with step-level output (the #5 reader side). Job two.
- **WebSocket/SSE streaming.** Job two.
- **Any frontend** — web UI or native GUI. Job three.
- **Interactive agent terminals** in any surface. Roadmap item 6, built once,
  last.
- **Migrating the TUI onto the API** — the TUI stays in-process.
- **`mg pull`** — small, independently useful, its own job.
- **User accounts / teams** — single user + bearer token is the v1 auth model.
- **Anything not in the list above.** The API is read-only; if an endpoint
  mutates state, it is out of scope.

## Security

The daemon is a new trust boundary with root-adjacent power: it drives docker
and git and holds subscription credentials. A compromised endpoint means an
attacker running agents on the paid subscription and committing to the repos.
Non-negotiable:

- **Bearer-token auth over TLS only.** Localhost binds may be tokenless; a
  token without TLS is no protection.
- **Zero path inputs from the client** — see scope item 5.
- **Credentials never returned** by any API or rendered anywhere — see scope
  item 6.
- **Audit trail** — see scope item 7.
- **Concurrency** — mutating operations serialized per project (job two's
  implementation; the skeleton lands in this job).
- **Cost** — a remote daemon makes firing runs easy, and unattended agent runs
  burn subscription quota. The API surfaces run activity; ntfy stays the away
  digest. No new notification machinery.

## Sequencing after this job

- **Now, zero code (validation):** put `mg jdi` + ntfy on a VPS and drive a
  real job from a phone. The away-digest experience may cover a large share of
  the "control from anywhere" need at near-zero cost, and it proves the server
  story before more code exists. It also tells us which frontend to build
  first: remote need → web UI; local glanceable need → native GUI.
- **Job two — mutating API + run supervision:** create/edit brief, launch
  agents and `mg jdi` detached, watch runs live with streamed logs (the #5
  reader side, riding the `session.log` capture already landed), answer
  `NEEDS-HUMAN-INPUT:` markers, `done`/`delete`/push. Per-project serialization
  enforced here.
- **Job three — one frontend:** an embedded web UI port of the TUI's
  information design (on-brand one-binary, works from a phone, matches the VPS
  story). The native GUI is then a port of the same API client + same info
  design — a much smaller job, only if the local, glanceable need survives
  real use.
- **Independent:** `mg pull`; roadmap #4 (headless/queue) feeds the daemon's
  value but blocks nothing here.

## Concerns

1. **The daemon's power** — see Security. This is the part with real teeth,
   and the read-only-first scope is what makes the first job safe to ship.
2. **Concurrency** — no locking in `internal/git`; the mutating API must
   serialize per project. The skeleton lands in this job so job two inherits
   it rather than inventing it under pressure.
3. **Cost** — unattended runs on a server burn subscription quota; run
   activity must be visible in the API from day one.
4. **Scope creep** — "and control everything" is how a read-only daemon becomes
   a remote-desktop product. The out-of-scope list is explicit; if an endpoint
   mutates state, it is a later job.

## Recommendation

1. Cut **job one (this job)** from the scope above: `mg serve` + project
   registry + read-only API + localhost default + audit log + serialization
   skeleton. Read-only first is what keeps the new trust boundary safe.
2. Validate the cheap version (jdi + ntfy on a VPS) in parallel — zero code,
   and it decides which frontend job three should be.
3. Do **not** start the GUI or the mutating API until the read-only listener
   is real and the event-stream reader side (job two's supervision) exists —
   a surface built on polling is a worse TUI with more pixels.
4. After the listener lands, `docs/web-interface.md`'s "job one: the daemon"
   is done — this document supersedes that recommendation; no need to re-litigate.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

