# Web interface: running manigot on a server with a browser control plane

Exploration notes (2026-08-27). Verdict: **REVISIT** — the direction is right and
the product is already 70% server-shaped, but the scope as originally phrased
("command everything… create jobs, delete, merge, pull, whatever") is a vision,
not a buildable unit, and one fork decides everything. This document records the
findings, the fork, the scope, and the recommended path.

## Decision recorded (2026-08-27)

A sibling idea was evaluated the same day and rejected: **external ticketing
integration (Linear / Gitea)** — syncing manigot's job state into a third-party
issue tracker. It answers the *same* question this document answers (remote
visibility/control of jobs away from the terminal), and the web control plane
replaces it outright: it renders the real state (four files, diff, run logs)
natively and can act on it (launch, merge, delete), whereas a ticketing sync is
a lossy, view-only shadow that adds a second source of truth. It only re-enters
consideration as a later *consumer* of the event stream (post status/comments to
a team's board) if a team-need appears — never as a parallel control surface.
The one piece worth building independently of this decision is the git-forge
step (push job branch + open/merge PR on `mg done`) — the web UI needs it too,
and it is small.

## The idea

Run manigot on an always-on server instead of a personal workstation, and replace
the TUI with a secured web interface that talks to a server listener to control
it. A Svelte UI showing projects, their jobs, and job state — with the ability to
create jobs, delete, merge, pull, launch agents, and watch runs — replacing the
terminal-bound `mg tui`.

## What already works on a server (the good news)

The heavy machinery is mostly terminal-independent already:

- **`internal/job`** — `CreateJob`, `FinishJob`, `DeleteJob`, `Discover`,
  `ReadJDIStatus` are pure functions over a project root. The TUI calls them
  in-process; an HTTP handler can call the same functions in-process. No
  subprocess mg, no TTY.
- **`internal/git`** — worktree create/remove, squash merge, push, diff, commits.
  All host-side, all callable.
- **`mg jdi`** — already runs fully unattended via the `--print` path, writes a
  pollable status sidecar (`.manigot/jdi-status/`), handles `NEEDS-HUMAN-INPUT:`
  markers, and pushes ntfy notifications. This is exactly the state model a web
  UI would render — the TUI already polls it; the web UI would poll/stream the
  same files.
- **The sweep-commit** — every job-worktree session ends committed by the host.
  This is what makes headless server operation safe in the first place.
- **Docker isolation** — the agent containment story is the product's core
  safety property and works identically on a server. The daemon does not touch
  it.

The genuinely new pieces:

1. **A long-running daemon** — today everything is a one-shot CLI that discovers
   its project from `$PWD`. A server is a new process (`mg serve` or similar)
   that holds a registry of project roots. **Multi-project is a new concept for
   manigot** — nothing in the product today knows about more than one project
   per invocation.
2. **The HTTP API + auth surface** — today the trust boundary is the user's own
   terminal; nothing authenticates anything. A server daemon is a remote attack
   surface holding subscription credentials and docker access. This is the part
   with real teeth (see Security).
3. **Background run supervision with streamed output** — today's interactive
   agent launches spawn terminal windows/tmux panes, which is meaningless on a
   server. Web-launched agents need to be detached processes with captured logs
   the UI can watch live.
4. **The Svelte app itself.**
5. **"Pull"** — there is no host-side fetch/pull in manigot today (only
   push-to-origin and merge-default-branch). For a server, `mg pull` (fetch +
   fast-forward the base branch, refuse-or-report on divergence rather than
   merging blindly) is a small, genuinely useful addition — locally too.

## The fork that decides everything

**Is the web UI a control plane for background runs, or a remote terminal for
interactive sessions?**

- **Control plane (recommended for v1):** launch `mg jdi` per job, launch
  one-shot agent runs via the `--print` path, watch their logs live, review the
  four files, edit `brief.md`, merge (`done`), delete, push, answer
  `NEEDS-HUMAN-INPUT:` markers. This is the product's own trajectory — the
  autonomy machinery exists precisely so the human's job is *review and decide*,
  not drive. It reuses everything above.
- **Interactive browser terminals** (xterm.js + a PTY bridge so you can drop
  into a live Claude Code chat from the browser): much heavier, and arguably
  wrong for a server — the human usually isn't at the server, and an interactive
  session on a machine other people can reach raises "who is driving this"
  questions. The roadmap already has "in-TUI embedded terminal" as its biggest,
  last bet; building the same bet on a second surface at the same time is how
  scope explodes.

Decide that fork first, and you've decided the size of the whole thing.

## User perspective

For the person this actually serves — a developer who wants manigot's work
running on a box that's always on, reachable from anywhere — the control-plane
version is a genuine unlock: create a job from the browser, fire jdi at it, get
an ntfy ping when it needs a human, open the verdict from your phone, approve
and merge. The *interactive* version is a different product aimed at a different
need (remote pair-driving an agent), and it's the expensive one.

A caution on the "sleek, dynamic Svelte interface": the roadmap's current state
is praised as "boring in the right places," and that is a product property, not
an accident. The web UI should be a **port of the TUI's information design** —
id / status / stage / type / date / title, the four files, the diff tab, the
same actions — not a re-imagination of what a dashboard should be. "Dynamic"
should mean *live updates*, which comes from the event stream, not from adding
40 controls. If the interface exists at all, it must be as disciplined as the
TUI.

## Scope assessment

What's missing before this is a brief:

- **Single user or team?** One bearer token vs. real accounts. This changes the
  auth design completely. (Recommendation: single-user/small-team token auth is
  the right v1; user accounts are a different product.)
- **Internet-exposed or LAN/VPN?** Determines how much hardening is
  non-negotiable vs. nice-to-have.
- **Project registry:** where do the projects come from? A config file of roots
  is the obvious v1 (a scan-dir is a worse idea — you don't want a daemon
  auto-adopting repos it finds).
- **"Pull" semantics** need pinning down (fetch + fast-forward, report-don't-
  merge on divergence, refuse on dirty tree).
- **Sequencing against the roadmap.** This is the one that matters most: the web
  UI's core value depends on roadmap items **#4 (headless/cron — queue,
  non-terminal trigger, away digest)** and **#5 (event-streaming)**. The
  "sleek, dynamic" part *is* the event stream made visible; polling the coarse
  sidecars is exactly the non-dynamic experience to avoid. The roadmap's own
  caution applies here as much as it did for the TUI: don't let the web UI's
  requirements design the event format. So the honest path is: **#4 → #5 → web
  UI**, with the web UI as the event stream's real consumer — which is a *better*
  first consumer than the richer run.log the roadmap currently names.

## Security

The daemon is a new trust boundary with root-adjacent power: it drives docker
and git and holds subscription credentials. A compromised HTTP endpoint means an
attacker running agents on the paid subscription and committing to the repos.
Non-negotiable hardening:

- **TLS** — in-binary or, saner, behind a reverse proxy (Caddy/nginx).
- **Bearer-token auth** — over TLS only.
- **Zero path inputs from the client** — every operation resolved against the
  registered project roots; no job IDs or paths taken from the URL and trusted.
- **Credentials never returned** by any API or rendered in the UI. The `.env`
  holds subscription credentials; the daemon must never expose them.
- **Audit trail** — a remote/shared box changes the accountability picture; the
  local TUI never needed "who ran what," the server does.
- **Concurrency** — today every operation is naturally serialized (one CLI
  process, user-driven). A daemon gets simultaneous web requests plus
  long-running launches. `internal/git` has no locking; two concurrent
  `done`/`delete` on the same job would race. The daemon needs per-project (or
  per-job) serialization of mutating operations.
- **Cost** — a server makes firing runs easy, and unattended agent runs burn
  subscription quota. Surface run activity in the UI; keep ntfy as the "away
  digest" rather than building new notification machinery.

## Concerns

1. **The daemon's power** — see Security. This is the part with real teeth.
2. **Concurrency** — no locking in `internal/git`; the daemon must serialize
   mutating operations per job.
3. **Cost** — unattended runs on a server burn subscription quota.
4. **Scope creep** — "and pull, whatever" is how a control-plane job becomes a
   remote-desktop job. The interactive-terminal slice must be explicitly parked
   (it's the roadmap's item #6 bet, one surface at a time).

## Recommendation

Proceed, in this order:

1. **Decide the fork** — control plane for background runs, no interactive
   browser terminals in v1. Stake the product on that.
2. **Validate the cheap version first:** put `mg jdi` + ntfy on a VPS and run a
   real job from a phone. The away-digest experience may cover a large share of
   the "control from anywhere" need at near-zero cost, and it proves the server
   story before any web code exists.
3. **Sequence against the roadmap:** do #4 (headless — queue + away digest) and
   #5 (event-streaming) first, then:
   - **Job one:** the daemon — `mg serve`, project registry, bearer-token auth,
     read-only API first (projects, jobs, files, jdi status), embedded static UI
     later.
   - **Job two:** the mutating API + background run supervision with streamed
     logs + WebSocket/SSE.
   - **Job three:** the Svelte app — a web port of the TUI's design, embedded
     into the Go binary so the deployment story stays "one binary on a server"
     (very on-brand for a project that reduced itself to exactly one Go binary
     on purpose).
4. **Add `mg pull` as a small standalone job** — needed by the server story,
   useful locally, small enough to land on its own.

Right direction, right product fit, wrong size for one go. Narrow it to the
control plane, sequence it behind the roadmap's #4/#5, and it becomes three
clean jobs instead of one unbounded bet.