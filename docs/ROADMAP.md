# Roadmap: what to build next

Prioritized recommendations from a product/user-perspective review of the
current state (2026-08-13, revised). This is the *only* forward-looking
planning document — `backlog.md` was deleted in this revision because every
entry in it had been dispositioned (promoted here or decided against). Promote
an item to a real job (`mg job`) when it's actually going to be worked on.

## Current state, in one paragraph

The product is coherent, feature-dense, and — for the first time — boring in
the right places. Single `mg` binary, three subscription profiles
(`claude-pro` / `zai` / `opencode-go`), real filesystem isolation, a full job
lifecycle on git worktrees, an autonomous mode (`mg jdi`), a TUI, host mode,
ntfy notifications, fourteen agents, a strong test suite, and the CODE_QUALITY
consolidation (one git-exec point, one branch-matching definition, one
error-framing place, probe timeouts, shared agent-name constants) all landed.
The first roadmap's core-loop work is done; the worry tax on `mg jdi` is paid
off. The abandoned-worktree signal that shaped the last roadmap is gone — the
tool now detects and removes orphans itself.

## Decisions recorded (2026-08-13)

- **`@owner` and `@security` are not part of `mg jdi` — by design, not by
  deferral.** The autonomous sequence is exactly three agents (`@analyst` →
  `@developer` → `@reviewer`) and was never meant to grow. The owner's
  SHIP/REVISIT/REJECT call and the security pass are human-triggered steps in
  the interactive flow and stay that way. The previous roadmap listed this as
  a future extension; that recommendation is withdrawn. Documentation
  (README, AGENTS.md) already states this correctly and must not be "fixed" to
  suggest a five-agent autonomous loop.
- **`feature/3ro17g_go-instructions` is irrelevant.** A lone remote branch
  with no worktree and no registered job — no action, no job.
- **`docs/backlog.md` is deleted.** Its entries: in-TUI terminal (now item 6),
  event-streaming (now item 5), `@owner`/`@security` in jdi (decided against,
  above), richer step-level logging (folded into item 5), headless/cron (now
  item 4). Nothing was lost; everything was dispositioned.

## The previous roadmap's items 1–4: done

1. **Prove `mg jdi` end-to-end on all three profiles** — done (`63quv2`):
   JSONL signal parsing, retry-budget state machine, real runs under `zai`
   and `opencode-go` with `run.log` and sidecars verified.
2. **Enforce read-only agents under OpenCode** — done, same job: the
   `permission:` frontmatter passes the `name:`/`tools:` strip and carries the
   read-only restriction into OpenCode's schema.
3. **Stage column in the TUI overview** — done (`3iqg8j`): the list shows
   id / status / stage / type / date / title.
4. **Orphaned-worktree detection and cleanup** — done (`nepbxu`): `mg jobs`
   surfaces orphans, `mg delete` removes them, with `mg delete`'s confirmation
   discipline.

Housekeeping from the old item 6 is likewise landed (CODE_QUALITY Phase 1,
probe timeouts, error-prefix consistency) — with the sole exception of the
`go-instructions` fate, which the decision above closes.

## What's next, in order

### 1. jdi-status sidecar cleanup — done

`mg delete` and `mg done` now both remove the job's `.manigot/jdi-status/<job>/`
sidecar (status + run.log), as does orphaned-worktree removal — the
keep-vs-remove decision for `mg done` is **remove**: the archive keeps the
job's docs, mg-jdi never runs against an archived job, and the sidecar would
otherwise be dead weight forever. Best-effort (a removal failure warns, never
aborts the already-succeeded delete/finish). This repo's own stale sidecar
dirs were cleaned up as part of the fix.

### 2. `mg doctor` health check (small, agreed)

One command that verifies the chain in one place: image present, docker daemon
up, profiles ready, git repo sane, worktree integrity. `mg setup --check`
covers credentials only. Given the project's history of "jdi does not work"
jobs, this pre-empts the silent-failure class of problem instead of fixing it
after the fact.

### 3. Configurable project toolchains (Docker images) — reframed, now in scope

The old rejection ("need isn't evidenced") was aimed at custom images per
project. The real need is narrower and real: **an agent on a Node/Python/
whatever project can't run that project's own build or test commands** because
the single image doesn't carry the toolchain — an agent that can't verify its
own work. Smallest useful version: keep the single base image as the default,
let a project declare additions (extra packages/toolchains layered at session
start) via a documented, `mg init`-era config. No from-scratch images, no
per-project fork of the isolation story.

### 4. Headless / cron execution — the VPS promise, now in scope

The autonomy story's completion: "set it on a task, it'll handle everything"
(the `vu33rn` Why). The hard 80% already exists — detached TUI-launched runs,
ntfy notifications, status sidecars, the `NEEDS-HUMAN-INPUT` marker. Missing:
a way to queue several jobs (`mg jdi --all`, or an ordered queue), a
non-terminal trigger, and an *away digest* — "here's what ran, here's what
finished, here's what needs a human" — surfaced through the notification
channel already built. The old "attended terminal" objection is mostly
obsoleted by the machinery built since.

### 5. Event-streaming subsystem — designed against a real consumer, now in scope

The polling model tells you the *stage*, not what the agent is *doing*. The
old fear ("wire-format commitment") is answered by building it against a
concrete need rather than as an abstraction: the first consumer in scope is
the richer step-level `run.log` (the old "richer logging" backlog item),
replacing the README's honest "final answer only" limitation. Design the
writer side in the agent invocation path and the reader side in the TUI for
*that* consumer; the format earns its keep before anything else attaches to
it.

### 6. In-TUI embedded terminal — biggest bet, last, smallest slice first, now in scope

The window-chaos fix, and the largest commitment on the list (PTY in Bubble
Tea). Ordered last deliberately: its first useful slice — a live "what is the
running agent doing right now" view in the detail view — can ride on the event
stream from item 5 instead of inventing visibility from scratch. Full
interactive embedding (typing to the agent inside the TUI) is a separate,
later slice. Items 5 and 6 must not collapse into one job; the event system
must not be designed by the terminal's requirements.

## Concerns

- **Four big bets in one roadmap is fine; four big bets in one quarter is
  not.** The ordering is deliberately value-per-effort: quick wins first
  (#1–2), then the correctness gap (#3 toolchains), then the autonomy
  completion (#4 headless), then the visibility foundation (#5 events), then
  the UI bet last (#6). They're genuinely independent — nothing downstream
  blocks if one stalls.
- **Toolchains (#3) has a real failure mode to scope out early:** the added
  layer is inside the container but it is new attack surface for a read-only
  agent's session. The hard boundary (read-only git mounts, deny-lists) must
  survive the change. That regression test belongs in the brief before
  `@analyst` sees it.
- **Headless (#4) must not become "cron, naively."** A nightly `mg jdi --all`
  firing at unread briefs, or two queues stepping on the same worktree, is how
  the multi-jdi-instances mess returns. The queue needs the same per-job guard
  the TUI's `j` key already has.

## Bottom line

The next jobs, in order: **(1) jdi-status sidecar cleanup — done**, **(2)
`mg doctor`**, **(3) configurable project toolchains**, **(4) headless/cron
execution**, **(5) event-streaming against a real consumer**, **(6) in-TUI
terminal, smallest slice first**. Item 2 is an immediate chore/feature job;
items 3–6 are sequenced as separate jobs, not an omnibus. The autonomy story
is now: three agents, on purpose, boring in the right places — and the docs
say so.
