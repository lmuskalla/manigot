# Tasks: fully autonomous mode

id: vu33rn
status: open
analyst: @analyst
date: 2026-08-09

<!-- Produced by @analyst from brief.md. -->

## Decisions this breakdown locks in

The brief flags one open question explicitly (the "blocked" signal) and
leaves several supporting mechanisms unspecified because the orchestration
logic "doesn't exist yet". Per "needs a defined signal... TBD in tasks.md"
and "ask, don't guess", the decisions below are made here, with reasoning,
rather than left for `@developer` to invent mid-implementation.

**1. "Needs human input" signal — a literal marker, not free-text heuristics,
injected into the prompt `mg-jdi` builds, not into the shared agent files.**
The instruction *if you cannot proceed without a human decision, stop
immediately and print a line starting with exactly `NEEDS-HUMAN-INPUT:`
followed by a one-sentence reason, and make no further changes* must NOT be
added to `agents/analyst.md`/`developer.md`/`reviewer.md` — those files are
global and read identically for a manual `mg --agent ... --job ...` session
or a TUI action-bar launch, where a human is actually watching and can just
answer a clarifying question in conversation. An agent has no way to tell
from inside a session whether it's attended or not, so a hard rule baked into
the shared files would make ordinary interactive sessions more trigger-happy
about halting instead of asking, for every launch path, not just `mg-jdi`'s.
(An earlier draft of this decision put the rule in the agent files directly —
corrected after review for exactly this reason.)
Instead, the marker instruction is appended to the *job prompt* only on the
`--print` path — `scripts/run.sh`'s own `JOB_PROMPT` ("Please work on the job
at `<path>` — start by reading brief.md"), extended with one sentence noting
the session is unattended and defining the marker, added when `--print` and
`--job` are both given (TASK-2). `--print` is non-interactive by
construction regardless of who invokes it, so scoping the sentence to that
flag rather than to `mg-jdi` specifically is both simpler (no prompt-building
code needed in `mg-jdi` itself — TASK-6 just invokes `mg --print --agent <x>
--job <id>` and gets the augmented prompt for free) and correct (a plain `mg
--agent ... --job ...` interactive session never sees this instruction and
keeps asking questions in plain prose exactly as it does today, since a human
is right there to answer it). `mg-jdi` scans that invocation's captured
output for a line matching `^NEEDS-HUMAN-INPUT:` (anchored, case-sensitive).
This is chosen over parsing free-form prose for "sounds like a question"
because it is deterministic and testable; the trade-off is that it only
works if the agent actually honours the instruction (a behavioral risk, not
a code one — flagged again on TASK-1).

**1a. Stall detection — a compliance-independent backstop.** TASK-1's marker
only works if the model remembers to use it. The realistic failure mode
isn't a hang (there's no interactive wait in `--print` mode — see the
conversation on this job for why) but a silent stall: an agent ends its turn
with something like a clarifying question in plain prose, writes nothing,
and `mg-jdi` just re-invokes it again indefinitely since `Stage()` never
moved. Backstop: all three agents in the sequence are expected to persist
something (`@analyst` → `tasks.md`, `@developer` → commits/
`implementation.md`, `@reviewer` → `verdict.md`), so if the same agent is
invoked twice in a row for the same job with no resulting change (`Stage()`
unchanged **and** no new commit on the branch), `mg-jdi` stops as
needs-human on the second occurrence — regardless of whether the marker was
present.

**2. Retry-budget tracking — count `verdict:` commits on the job's branch.**
`@reviewer`'s own convention (`agents/reviewer.md`) already commits verdicts
as `[ID] verdict: <summary>`. `mg-jdi` counts commits on the job branch
matching that pattern: 0 or 1 such commits → still within budget (a REJECTED/
NEEDS WORK first pass may bounce once to `@developer`); 2 or more → the
one-bounce budget (brief scope item 2) is exhausted, so this is a "needs
human" stop regardless of what the second verdict says. This needs no new
state file — it reads history already committed by the existing workflow.

**3. Non-interactive invocation — extend the existing `claude --print` path,
Claude Code only for v1.** `docs/jobs/archive/c4ouwc_auto-mode-for-claude-code`
already confirmed `claude --dangerously-skip-permissions --print "<prompt>"`
runs one-shot with no TTY and exits 0/non-0. `mg-jdi` needs `scripts/run.sh`
to expose that as a flag instead of always attaching `-it`. OpenCode's
non-interactive equivalent is unverified and out of the auto-mode
precedent's scope; rather than guess at an untested opencode invocation,
v1 restricts `mg-jdi` to `--tool claude-code` and errors clearly if asked to
run against opencode. Extending to OpenCode is a natural, isolated follow-up
once its headless behavior is verified the same way TASK-1 of c4ouwc did for
Claude Code.

**4. Status and logs — a per-job sidecar directory outside the job
directory.** The job directory's four files (`brief.md`/`tasks.md`/
`implementation.md`/`verdict.md`) are the only things `@developer`'s `git
add -A` should ever sweep into a commit. Neither a live "mg-jdi is running /
stopped why" status nor a run log can live inside the job dir without
risking exactly that. Instead `mg-jdi` gets one sidecar directory per job,
`docs/jobs/.jdi-status/<job-name>/` (gitignored), holding two files:
`status` (current state, for the list-row badge — see Decision 4a) and
`run.log` (the append-only transcript — see Decision 7). The TUI polls both
alongside `Stage()`, consistent with the brief's "derived from polling job
files... no new event-streaming subsystem".

**4a. Status content.** `status` records: state (`running` /
`stopped:finished` / `stopped:needs-human`), which agent is currently/last
active, and a timestamp. Read defensively — a missing or stale file (e.g.
`mg-jdi` was killed mid-run) must degrade to "no autonomous run in
progress", never be shown as if it were live.

**5. Notification — terminal bell, from whichever process actually has a
terminal (revised — see Decision 7a for why this isn't always `mg-jdi`
itself).** `\a` is portable (macOS + Linux), needs no new dependency, and is
heard in whatever terminal/pane is emitting it. Risk: a terminal/tmux config
with the bell muted won't notify; acceptable for v1, no mitigation planned.

**6. `mg-jdi` is a new Go binary, not a bash script.** The orchestration
decision logic needs `job.Stage()`, verdict-round counting, and a real loop
with typed state — all far more naturally expressed in Go than bash, and the
codebase already has the exact precedent to mirror: `mg-tui` is
`scripts/tui.sh` (thin wrapper) → `bin/manigot-tui` (Go binary). `mg-jdi`
follows the same shape: `scripts/jdi.sh` → `bin/manigot-jdi`.

**7. Run visibility (brief scope item 7) — live tee plus a persisted,
pollable log, honest about what it can actually show.** Every agent
invocation's captured output (from TASK-2) is fanned out three ways as it's
produced: (a) into the buffer TASK-5 scans for the marker, (b) straight to
`mg-jdi`'s own stdout, and (c) appended to the sidecar's `run.log`, one
section per invocation headed by a timestamp/agent/attempt line (e.g. `===
2026-08-09T… @developer (attempt 1) ===`). One honesty caveat this decision
can't paper over: `--print` in its plain form only returns the agent's
*final response text*, not a blow-by-blow of every tool call/file edit — so
unless TASK-2's `--output-format` investigation turns up a richer
streaming/step-level format on the pinned Claude Code version, "see what
happens" means "see each agent's final answer as it's produced," not a live
diff of its work. That gap should be surfaced to the author rather than
quietly shipped as if it were full transcript visibility.

**7a. `mg-jdi` gets no spawned terminal window when launched from the
TUI — corrected after review.** An earlier draft of this breakdown had the
TUI open `mg-jdi` in a new terminal window, mirroring how `launch.Agent`
opens agent sessions today. That was wrong: `launch.Agent`'s window exists
because a human needs to *interact* with a live, TTY-attached session
inside it — `mg-jdi` needs that from neither the human nor its own
subprocesses (Decision 3 already established none of its container
invocations use a TTY), and TASK-8/TASK-9 already give the TUI
window-independent visibility (status badge, polled log tab). Spawning a
window on top of that would be pure redundancy and would reintroduce the
exact per-agent-window overhead the backlog's "in-TUI agent terminal" idea
is about *removing*, not adding to. So: TUI-launched `mg-jdi` runs detached
in the background (`cmd.Start()`, no terminal emulator involved at all) —
visibility is TASK-9's log tab, not a window. Two consequences:
- Decision 7(b)'s "straight to `mg-jdi`'s own stdout" leg only has an
  audience when a human runs `mg-jdi <job>` directly from their own shell
  (the CLI entry point) — there, it's not a spawned window at all, just
  their existing terminal, so it was never actually in tension with
  Decision 3. For a TUI-launched run that stdout has no attached terminal
  and is simply discarded; (c)'s `run.log` (via TASK-9) is the only
  visibility path there.
- Decision 5's bell has the same split: a direct CLI run can ring it from
  `mg-jdi`'s own process (attached to the human's terminal, as before). A
  TUI-launched run cannot — there's no terminal for it to ring into. Instead
  the **TUI itself** rings the bell on its own next poll tick when it
  observes a job's status transition into a stopped state it hadn't already
  notified for (in-memory dedup, reset on TUI restart — acceptable, since a
  restarted TUI re-observing an already-stopped job isn't a new event worth
  re-alerting on). This is still poll-based, not a new subsystem, and it's
  the TUI process — which the human has open if they launched `mg-jdi` from
  it — that has a terminal to ring into.

## Task breakdown

TASK-1: Define the exact `NEEDS-HUMAN-INPUT:` marker convention (Decision
1) — the precise sentence appended to the job prompt on the `--print` path
and the precise line format (`^NEEDS-HUMAN-INPUT:` anchored, case-sensitive,
followed by a one-sentence reason) TASK-5's parser matches against — and add
one terse architecture bullet to `docs/AGENTS.md` (repo root only —
manigot's own project context, matching the existing one-bullet-per-script
style, e.g. the `entrypoint.sh` bullet), noting it as something `--print`
invocations add to the prompt, not a standing rule the shared `agents/*.md`
files carry (see Decision 1's correction). Not a change to
`agents/analyst.md`, `agents/developer.md`, or `agents/reviewer.md`, and
explicitly **not** `project-template/docs/AGENTS.md` either — that file is a
blank scaffold copied into *other* projects for *their* own context, and has
no reason to carry a behavioral detail of manigot's own internals.
files: docs/AGENTS.md (repo root; this task is documentation/definition
only — the sentence is actually appended to the prompt by TASK-2's
`scripts/run.sh` change, and matched by TASK-5's parser)
depends: none
risk: low — text/documentation only. Residual risk is behavioral, not code:
nothing guarantees a model actually emits the marker every time it should
even once it's told to in the prompt; TASK-4's retry-exhaustion check and
TASK-6's stall backstop (Decision 1a) are the structural fallbacks for when
it doesn't.

TASK-2: Add a non-interactive invocation mode to `scripts/run.sh` and
`scripts/entrypoint.sh` (Decision 3): a new flag (e.g. `--print`) that drops
`docker run`'s `-it` in favor of a plain foregrounded run and execs `claude
--dangerously-skip-permissions --print "$@"` instead of the interactive
`exec claude`, so the caller gets the agent's final output back on stdout
instead of an attached terminal. Reject `--print` combined with `--tool
opencode` with a clear error (Decision 3's Claude-Code-only scope for v1).
Investigate (don't assume) whether the pinned `claude` version also supports
`--output-format json` for `--print`; if it does, prefer it so TASK-5 parses
a clean final-response field instead of raw combined stdout (avoids a
false-positive marker match inside incidental tool-call output, e.g. a
`grep` result printed while an agent reads the codebase) — if it doesn't,
fall back to plain-text capture as described above. Also implements the
prompt-side half of Decision 1 (corrected after review): when `--print` and
`--job` are both given, `run.sh`'s existing `JOB_PROMPT` ("Please work on the
job at `<path>` — start by reading brief.md") gets one sentence appended
defining the `NEEDS-HUMAN-INPUT:` marker per TASK-1's convention — scoped to
`--print` specifically (a mode that is non-interactive by construction,
regardless of caller) rather than to the shared `agents/*.md` files (read
identically by attended sessions, where a human can just answer a question —
see Decision 1). `mg-jdi` (TASK-6) does not build this prompt text itself; it
just invokes `mg --print --agent <x> --job <id>` and gets it for free.
files: scripts/run.sh, scripts/entrypoint.sh
depends: TASK-1 (for the exact marker sentence/convention)
risk: medium — changes the shared container-invocation path every existing
interactive session also uses; must not regress the default `-it` behavior,
and needs to confirm `--print`'s (and, if used, `--output-format json`'s)
output/exit-code behavior matches what c4ouwc's TASK-5 observed
(buffering/no-ANSI differences from a real TTY).

TASK-3: New `tui/internal/git` helpers for the retry-budget state (Decision
2), corrected after review to add a second signal beyond a bare count:
`job.Stage()` alone cannot distinguish "`@reviewer` just rejected,
`@developer` hasn't responded yet" from "`@developer` has already committed
a fix since that verdict, waiting on re-review" — both look identical as
`(StageImplement, 1 verdict commit)`, since `Stage()` only changes again once
`verdict.md`'s own content changes, not on any commit `@developer` makes.
So, alongside `CountVerdictCommits` (counts commits matching `[ID] verdict:`,
tolerant of zero commits, a non-repo, and a message that doesn't match the
exact convention — treat as unparseable, don't count it, don't crash), add
`LatestCommitIsVerdict(root, branch, jobID) (bool, error)`: whether the
branch tip (`git log -1`) is itself a verdict commit for this job. TASK-4
uses both together to resolve the ambiguity without any new state file —
everything is still derived from git history already committed by the
existing workflow.
files: tui/internal/git/git.go, tui/internal/git/git_test.go
depends: none
risk: low-medium — read-only git plumbing following the existing package's
established patterns; the main edge cases are a malformed/human-edited
commit message and an empty/no-commit branch.

TASK-4: New `tui/internal/orchestrate` package implementing the state
machine: given a `Job`'s `Stage()`, TASK-3's verdict-round count, *and*
TASK-3's `LatestCommitIsVerdict` (corrected after review — see TASK-3),
return one of "run agent X next" (X ∈ analyst, developer, reviewer,
following the brief's fixed sequence — the *same* sequence for every
`Job.Type`, per the brief's own scope decision 6; `Type` is not branched
on), "stop: finished", or "stop: needs human — retry budget exhausted".
`StageImplement` resolves as: 0 verdict commits → developer (first pass); ≥2
→ stop (budget exhausted); exactly 1 → developer if the branch tip *is* that
verdict commit (rejected, not yet addressed — the one allowed bounce),
otherwise reviewer (developer already committed a fix since, time to
re-review). `StageDefine` (brief.md itself not written — none of the three
driven agents' job to fix) resolves as stop: needs human. Table-tested
against every `Stage()`/round-count/tip-is-verdict combination the brief's
scope items 1–2 imply, including the exact boundary (round count 1 with
developer-already-responded → reviewer, not another developer bounce; round
count ≥2 → stop regardless of tip).
files: tui/internal/orchestrate/orchestrate.go (new), tui/internal/orchestrate/orchestrate_test.go (new)
depends: TASK-3
risk: medium — the actual policy logic; an off-by-one on "exactly once", or
missing the Stage()-can't-tell-B-from-C ambiguity TASK-3's tip check exists
to resolve, are the likeliest mistakes and the ones the brief cares most
about getting right.

TASK-5: Blocked-signal parser (Decision 1): given an agent run's captured
stdout (TASK-2's output), detect a `^NEEDS-HUMAN-INPUT:` line and extract its
reason text.
files: tui/internal/orchestrate/signal.go (new), tui/internal/orchestrate/signal_test.go (new)
depends: TASK-1 (defines the exact string this must match)
risk: low — pure string parsing against a format this same job defines.

TASK-6: `mg-jdi` orchestration loop — `tui/cmd/jdi/main.go`, a new Go entry
point that: resolves the job by ID/slug, repeatedly (a) asks TASK-4 what to
do next, (b) if an agent should run, invokes TASK-2's non-interactive `mg
--print --agent <x> --job <id>` synchronously, handing its output stream to
TASK-7's fan-out, (c) runs TASK-5's signal check against the buffered copy of
that output (an immediate "needs human" stop, independent of TASK-4's own
budget check), (d) re-reads the job's `Stage()`, verdict-round count, and
`LatestCommitIsVerdict` (TASK-3) and loops, until `Stage() == StageFinished`
or either "needs human" trigger fires. Injects the agent-runner as an interface so TASK-14's tests can fake
container invocations rather than spawning real ones.

Two implementation necessities surfaced during TASK-6 that weren't spelled
out elsewhere in this breakdown, both resolved rather than left for
`@developer` to invent ad hoc:
- **Branch checkout.** `scripts/run.sh` bind-mounts the project root/`docs/`
  straight from whatever the host currently has checked out (see `run.sh`)
  — it does not know or care which branch a job "belongs to". So before
  looping, `mg-jdi` must itself `git checkout` the job's own branch if it
  isn't already checked out (mirroring the TUI's existing `git.Checkout` /
  "b" switch-branch action, just performed automatically instead of
  blocking on a human) — otherwise every agent invocation and commit would
  land against whatever branch happened to be checked out on the host,
  not the job's.
- **Stall backstop semantics (Decision 1a), precise timing.** "The same
  agent invoked twice in a row... stops on the second occurrence" means: a
  single no-op invocation is tolerated (transient), and only a *second
  consecutive* no-op for the same agent stops the loop — not the first.
  Implemented by comparing `Stage()`/branch-HEAD immediately before and
  after each invocation (did *this* call persist anything), carrying
  whether the previous call was also a no-op across loop iterations. This
  needed one more small `tui/internal/git` addition beyond TASK-3's two
  functions: `HeadCommit(root, branch) (string, error)` (`git rev-parse
  <branch>`), added here rather than back on TASK-3 since the need only
  became concrete while wiring up the loop itself.
files: tui/cmd/jdi/main.go (new), tui/cmd/jdi/main_test.go (new — exercises
the loop against a real temp git repo with a fake AgentRunner, covering the
happy path, one-bounce-then-approved, budget-exhausted, the
NEEDS-HUMAN-INPUT marker, the stall backstop, and a runner error), plus the
small `tui/internal/git` addition above (`git.go`/`git_test.go`)
depends: TASK-2, TASK-4, TASK-5
risk: high — this is the actual unattended loop a human is trusting to run
several real agent sessions without supervision; every prior task's
correctness is only as good as how they're wired together here, and a bug
could leave a job stuck mid-loop or (worse) looping past the one-bounce
bound.

TASK-7: Agent output logging (Decision 7) — fan out each invocation's output
stream (from TASK-6) three ways as it's produced: into TASK-5's buffer,
straight through to `mg-jdi`'s own stdout (live, not buffered-then-dumped —
this is what makes a direct CLI run show the agent working in real time; a
TUI-launched run has no terminal attached to this leg at all, per Decision
7a, so it's inert there and `run.log` is what matters), and appended to
`docs/jobs/.jdi-status/<job-name>/run.log` with a timestamp/agent/attempt
header per invocation (Decision 4). Honesty check against TASK-2's
`--output-format` finding: if only plain `--print` is available, document in
the log (and flag to the reviewer) that this is final-response-per-invocation
visibility, not a full tool-call transcript — do not present it as more than
it is.
files: tui/cmd/jdi/main.go, a small streaming/tee helper (new, e.g. tui/cmd/jdi/output.go)
depends: TASK-2, TASK-6
risk: medium — depends heavily on what TASK-2's `--output-format`
investigation finds; correctness here is "don't drop output, don't block the
loop on a slow writer, don't corrupt `run.log` if `mg-jdi` dies mid-write."

TASK-8: Status sidecar (Decision 4/4a) — `mg-jdi` writes/updates
`docs/jobs/.jdi-status/<job-name>/status` as it runs (state: running /
stopped:finished / stopped:needs-human, plus which agent is active, plus a
timestamp); a reader in `tui/internal/job` the TUI polls to render a
list-row badge next to the existing stage indicator. Gitignore the whole
`docs/jobs/.jdi-status/` directory (shared with TASK-7's `run.log`). Must
degrade gracefully when the file is absent or stale (e.g. `mg-jdi` was
killed mid-run).
files: tui/cmd/jdi/main.go (status writes), tui/internal/job/jdistatus.go (new), tui/internal/job/jdistatus_test.go (new), tui/internal/ui/app.go (list-row badge), .gitignore
depends: TASK-6
risk: medium — new persisted state outside the existing four-file job model;
must never be swept into an agent commit, and a stale file after a crash
must not be displayed as if it were live.

TASK-9: TUI log view (Decision 7/7a) — the primary way to see what `mg-jdi`
is doing from inside the TUI (a TUI-launched run has no spawned window at
all per Decision 7a, so this isn't a fallback for "not watching a window" —
it's the only in-TUI path): a new tab in the detail view (alongside
brief/tasks/implementation/verdict) reading TASK-7's `run.log` sidecar,
refreshed the same poll-based way (`ctrl+r` / the existing refresh path) as
every other tab — no live-streaming inside the TUI process itself,
consistent with "no new event-streaming subsystem". Must handle a
large/growing file (tail rather than load-everything) and the common case of
no `mg-jdi` run having happened yet for this job (no sidecar directory at
all).
files: tui/internal/ui/detail.go, tui/internal/ui/app.go, tui/internal/job/jdistatus.go (log reader alongside the status reader), tui/internal/ui/detail_test.go
depends: TASK-7
risk: low-medium — mostly presentational, reusing the existing tab/scroll
machinery already built for the four job files; the tail-not-load-everything
requirement is the one part that needs care on a long-running job.

TASK-10: `mg-jdi` host wrapper and install wiring, mirroring
`scripts/tui.sh` exactly (Decision 6): `scripts/jdi.sh` resolves the
checkout and execs `bin/manigot-jdi`; a new `make jdi` Makefile target
builds it; add `mg-jdi:jdi.sh` to the `LINKS` list so `make install`
symlinks it.
files: scripts/jdi.sh (new), Makefile
depends: TASK-6
risk: low — mechanical, follows the `tui.sh`/`make tui` pattern with no new
decisions.

TASK-11: Stop notification (Decision 5/7a) — two paths, not one:
(a) a direct CLI run emits `\a` to `mg-jdi`'s own stdout at both loop-exit
points (finished, needs human) — it's attached to the human's own terminal,
so this is unchanged from the original plan; (b) a TUI-launched run has no
terminal to ring into (Decision 7a), so instead the TUI's own existing
refresh/poll loop rings `\a` itself the first time it observes a job's
status (TASK-8) transition into a `stopped:*` state, tracked in-memory
per-job so it fires once, not on every subsequent poll tick.
files: tui/cmd/jdi/main.go (CLI-path bell), tui/internal/ui/app.go (TUI-path bell + transition tracking), tui/internal/ui/refresh_test.go
depends: TASK-6, TASK-8
risk: low-medium — two separate, small side effects, but the TUI-side one
needs correct dedup (don't ring on every poll while stopped; do ring exactly
once per fresh stop) and must not fire for a job that was already stopped
before the TUI started watching it.

TASK-12: TUI launch integration — `resolve.Jdi()` (new `MANIGOT_JDI_BIN`
env-override, mirroring `resolve.Manigot`/`Job`/`Done`/`Delete`), and a new
`launch` entry point that starts `mg-jdi --job <id>` **detached in the
background — no spawned terminal window** (Decision 7a; this is
deliberately *not* `launch.Agent`'s terminal-emulator-detection path, since
there's no interactive session for a human or subprocess to attach to), plus
a detail-view keybinding to fire it and an action-bar/help-text entry. Pick
an unused key — the detail view's bindings (`a p d r s e D x b tab
1-4`) are already dense; a capital `J` (mirroring `D` for mark-done as a
bigger, composite action) is the leading candidate but not mandated here.
files: tui/internal/resolve/commands.go, tui/internal/resolve/resolve.go, tui/internal/launch/launch.go (a new, simpler detached-start entry point — deliberately *not* extending `buildCmd`'s terminal-emulator selection, which this path skips entirely), tui/internal/ui/app.go, tui/internal/ui/agents.go, tui/internal/ui/detail.go
depends: TASK-10
risk: medium — new keybinding must not collide with the existing dense key
map; the launch command string is test-pinned elsewhere in this codebase, so
the new one should be too. Lower mechanical risk than a terminal-spawn path
would have been (no cross-platform emulator detection to get wrong), but the
detached process must still be correctly reaped (no zombies) without
blocking the TUI's `Update()` loop.

TASK-13: Documentation — `README.md` (Commands table, Usage, TUI
keybindings/status sections) and `docs/AGENTS.md` (repo root — Architecture,
Job workflow) updated to describe `mg-jdi`: the driven agent sequence, the
one-bounce retry bound, the `NEEDS-HUMAN-INPUT:` convention, the status
badge, the log tab (and that TUI-launched runs are detached with no spawned
window — TASK-9's log tab is the way to watch them, not a terminal), and the
two-path notification — including the honesty caveat from Decision 7/TASK-7
about what the log actually shows. `project-template/docs/AGENTS.md`
deliberately **not** touched (see TASK-1's reasoning: it's a blank scaffold
describing *other* projects' own context, not manigot's internals — `mg-jdi`
is manigot tooling, not something a project using manigot needs documented
in its own `AGENTS.md`).
files: README.md, docs/AGENTS.md
depends: TASK-1, TASK-6, TASK-7, TASK-8, TASK-9, TASK-10, TASK-11, TASK-12
risk: low — docs only.

TASK-14: Verification — `go build ./...` and `go test ./...` under `tui/`
(with TASK-6's agent-runner fake exercising the full loop, including a
forced NEEDS WORK verdict to confirm the one-bounce path stops correctly
rather than retrying twice), plus one real manual `mg-jdi` run against a
throwaway job end to end — including watching it live from a direct CLI run
*and* from the TUI's new log tab, to confirm TASK-7/TASK-9 actually satisfy
the brief's "TUI and CLI" visibility requirement rather than just building
each half in isolation.
files: none (verification) — except a real bug it found, fixed in
tui/cmd/jdi/output.go/main.go (see below)
depends: TASK-1…TASK-13
risk: low — verification only, but it is the one task that actually proves
the unattended loop, and its visibility, against real agent sessions;
nothing earlier in this breakdown does that.

**Finding (fixed, not just noted):** the manual run — a real git repo/branch
with a stub `mg` standing in for `scripts/run.sh`'s `--print` path, so no
docker/Claude Code credentials were needed to exercise the actual
`mg-jdi` binary end to end — caught that `docs/jobs/.jdi-status/` was only
gitignored in *manigot's own* repo (TASK-8's `.gitignore` edit), not in the
*target project* `mg-jdi` actually drives. A real `git add -A` inside that
project (simulating `@developer`/`@reviewer`) swept the sidecar straight into
a real job-branch commit — exactly the contamination TASK-7/8's "kept
outside every job's own directory" reasoning was supposed to prevent. Fixed
with `ensureSidecarIgnored` (`tui/cmd/jdi/output.go`, called once at startup
from `main.go`): appends the sidecar pattern to the project's own
`.git/info/exclude` if not already present — local-only, per-checkout, never
itself committed, so `mg-jdi` doesn't need to touch or assume anything about
the project's own tracked `.gitignore`. Re-verified against the corrected
build: the sidecar no longer appears anywhere in `git ls-tree` on the job
branch.

## Open questions for the author / @product-owner

- Decision 3 restricts `mg-jdi` v1 to `--tool claude-code`, deferring
  OpenCode. This isn't in the brief's own "Out of scope" list, so flagging it
  explicitly rather than treating it as settled: confirm this restriction is
  acceptable, or say if OpenCode support is actually required for v1 (in
  which case its non-interactive invocation needs its own investigation task
  before TASK-2 can be scoped for it).
- Decision 7's honesty caveat: if TASK-2's investigation finds the pinned
  Claude Code version has no richer streaming/step-level output format, "see
  what happens" will only mean each agent's final response text per
  invocation, not a live view of the actual file edits/tool calls it's
  making. Confirm that's an acceptable v1 bar for "visibility" — if not, this
  job may need to block on that investigation's outcome rather than proceed
  assuming plain `--print` is enough.

## Explicitly not covered by this breakdown

- Auto-merging the finished branch, retrying more than once per review
  cycle, an event-streaming subsystem, in-TUI embedded agent terminals,
  headless/cron execution, and `@product-owner`/`@security` in the automated
  sequence — all explicitly out of scope per the brief (the middle three
  tracked in `docs/backlog.md`). `mg-jdi` v1 drives `@analyst` → `@developer`
  → `@reviewer` only, uniformly for every job type; both excluded agents
  remain available as ordinary manual agents, untouched by this job.
- OpenCode support for `mg-jdi` — deferred per Decision 3; see the open
  questions above.
- A live-updating (non-refresh-triggered) log tab inside the TUI — TASK-9 is
  poll-based like every other tab, per the brief's own "no new
  event-streaming subsystem" constraint.
- Recovery if `mg-jdi`'s own process is killed mid-loop (TASK-8's status file
  going stale is handled defensively, but nothing resumes the loop
  automatically — the human re-runs `mg-jdi` on the same job, which is safe
  because every step re-derives state from `Stage()`/git rather than trusting
  in-memory state).
