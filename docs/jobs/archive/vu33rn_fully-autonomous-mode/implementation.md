# Implementation: fully autonomous mode

id: vu33rn
status: open
developer: Claude Sonnet 5
date: 2026-08-09

<!-- Produced by @developer after implementation. -->

## Summary

`mg-jdi` ("just do it") is a new host-side Go binary (`tui/cmd/jdi`, built
alongside the TUI via `make jdi`) that drives a job's `@analyst` →
`@developer` → `@reviewer` sequence end to end, without a human manually
triggering each stage. It resolves the job, ensures its own branch is
checked out, then loops: `tui/internal/orchestrate` decides what to do next
from the job's `Stage()` and git history, `mg-jdi` invokes that agent
non-interactively via a new `scripts/run.sh --print` flag, checks the
output for a `NEEDS-HUMAN-INPUT:` marker, and repeats — until the verdict is
APPROVED (`StopFinished`) or it needs a human: the one-bounce retry budget
is exhausted, an agent asks a question via the marker, or the same agent
makes no progress on two consecutive runs (a stall backstop). It never
auto-merges. Every invocation's output and a running/stopped status are
written to a gitignored sidecar (`docs/jobs/.jdi-status/<job>/`), which the
TUI polls for a list-row badge and a new detail-view log tab; the TUI can
also launch `mg-jdi` itself (`J`, detached, no spawned window) and rings a
notification bell — from `mg-jdi`'s own process for a direct CLI run, from
the TUI's own poll loop for a TUI-launched one.

Two design corrections were made during implementation, in conversation
with the author, before the affected tasks were built (both are recorded in
`tasks.md`'s Decision 1 / TASK-3 / TASK-6 with full reasoning, not just
noted here):

1. **The `NEEDS-HUMAN-INPUT:` marker is not a rule in the shared
   `agents/*.md` files.** The original task breakdown put it there, but
   those files are read identically by attended sessions (a human running
   `mg --agent ...` directly, or launching an agent from the TUI), where a
   human can just answer a question in conversation. A rule baked into the
   shared files would have made every launch path more trigger-happy about
   halting instead of asking. It's scoped instead to `scripts/run.sh`'s new
   `--print` flag, which is non-interactive by construction regardless of
   caller — `agents/*.md` is untouched by this job.
2. **`job.Stage()` alone can't tell "reviewer just rejected, developer
   hasn't responded" from "developer already fixed it, waiting on
   re-review"** — both look identical as `(StageImplement, 1 verdict
   commit)`. Resolved by adding `git.LatestCommitIsVerdict` (whether the
   branch tip is itself the verdict commit) alongside the verdict count.

A third issue was found (and fixed, not just noted) during TASK-14's manual
verification — see Known issues/TASK-14 below.

## Changes

**TASK-1 — `docs/AGENTS.md`.** Added a terse architecture bullet (matching
the file's existing one-bullet-per-script style) defining the
`NEEDS-HUMAN-INPUT:` marker as something `scripts/run.sh`'s `--print` flag
adds to the job prompt, not a rule in `agents/*.md`. Deliberately not added
to `project-template/docs/AGENTS.md` — that file is a blank scaffold for
*other* projects' own context, with no reason to carry manigot's internal
tooling details.

**TASK-2 — `scripts/run.sh`, `scripts/entrypoint.sh`.** New `--print` flag:
drops `docker run`'s `-it` for a plain foregrounded run, and
`entrypoint.sh` execs `claude --dangerously-skip-permissions --print
--output-format json` instead of the interactive `exec claude` (confirmed
`--output-format json` is supported by the pinned claude version, 2.1.226,
and returns `{"result": "...", "type": "result", ...}`). Rejects `--print`
combined with `--tool opencode`. When `--print` and `--job` are both given,
the job prompt gets the `NEEDS-HUMAN-INPUT:` sentence appended. `run.sh`'s
own diagnostic/banner output now goes through fd 3 (stderr in `--print`
mode, stdout otherwise — interactive behavior is byte-for-byte unchanged),
so a `--print` caller's stdout is exactly the agent's own output. Verified
by driving `run.sh` against a stub `docker` binary and inspecting the
resulting argv (confirms `-it` is dropped only under `--print`, `--job`'s
prompt gets the marker sentence, and interactive mode is unaffected).

**TASK-3 — `tui/internal/git/git.go`.** `CountVerdictCommits(root, branch,
jobID)` — counts commits matching `[<jobID>] verdict:` on a branch, tolerant
of zero commits, a non-repo, and messages that don't match the exact
convention. `LatestCommitIsVerdict(root, branch, jobID)` — whether the
branch tip is itself one of those commits (the ambiguity fix above).
`HeadCommit(root, branch)` (added while wiring TASK-6, not originally
scoped to TASK-3, but the same kind of git plumbing) — `git rev-parse
<branch>`, used by the stall backstop.

**TASK-4 — `tui/internal/orchestrate/orchestrate.go` (new).** `Next(stage,
verdictRounds, latestCommitIsVerdict) Decision` — the state machine:
`StageDefine` → needs-human (nothing to fix, none of the three driven
agents' job); `StagePlan` → analyst; `StageReview` → reviewer;
`StageFinished` → finished; `StageImplement` → developer if 0 verdicts,
needs-human if ≥2, otherwise developer if the tip *is* the verdict (rejected,
not yet addressed) or reviewer if something newer sits on top of it
(developer already responded). Table-tested against every
stage/round-count/tip-flag combination.

**TASK-5 — `tui/internal/orchestrate/signal.go` (new).** `DetectSignal(raw)`
— scans for `^NEEDS-HUMAN-INPUT:` (anchored, case-sensitive), preferring a
`--output-format json` payload's `"result"` field over raw bytes so a
marker-shaped string inside incidental tool-call output doesn't
false-positive. `ResultText(raw)` is the shared JSON-extraction helper,
reused by TASK-7's logging.

**TASK-6 — `tui/cmd/jdi/main.go`, `main_test.go` (new).** The orchestration
loop: resolves the job by ID/name (`resolveJob`, mirroring `run.sh`'s own
resolution — exact name, exact ID, then a unique prefix), ensures its branch
is checked out (`ensureOnBranch` — necessary because `run.sh` bind-mounts
whatever the host currently has checked out, not something tied to the job;
this wasn't explicit in the original task breakdown but is a hard
correctness requirement, documented in `tasks.md`'s TASK-6 entry), then
loops via `Run(root, j, runner, log, status)`: ask `orchestrate.Next`, run
the agent (`AgentRunner` interface — `commandAgentRunner` shells out to `mg
--print --agent <x> --job <name>` for real; tests fake it), check
`DetectSignal`, apply the stall backstop (two consecutive no-op invocations
of the same agent — Stage and branch HEAD both unchanged across the
invocation — stop; one no-op is tolerated as possibly transient), loop.
Tests exercise a real temp git repo with a fake `AgentRunner`: happy path,
one-bounce-then-approved, budget-exhausted (confirms no third developer
bounce), the `NEEDS-HUMAN-INPUT:` marker, the stall backstop, and a runner
error.

**TASK-7 — `tui/cmd/jdi/output.go` (new).** `logInvocation` writes one
`=== <timestamp> <agent> (attempt N) ===` section per invocation, with the
agent's extracted final-response text (not the raw JSON), to whatever
writer `main()` builds — an `io.MultiWriter` of `os.Stdout` and the
sidecar's `run.log` (`openRunLog`), written unconditionally regardless of
launch path (harmless if nothing reads stdout).

**TASK-8 — `tui/internal/job/jdistatus.go` (new), `tui/internal/ui/app.go`,
`.gitignore`.** `JDIState` (`running` / `stopped:finished` /
`stopped:needs-human`), `WriteJDIStatus`/`ReadJDIStatus` for the sidecar's
JSON `status` file. `ReadJDIStatus` degrades to "nothing to report" for a
missing/unparseable file or a `running` status stale beyond 30 minutes
(mg-jdi almost certainly killed mid-run) — a `stopped:*` status is terminal
and never considered stale. `Run` gained a `StatusFunc` callback reporting
`JDIRunning` before each invocation and the terminal state at exit.
`renderJobRow` gained a `[mg-jdi: ...]` badge reading `ReadJDIStatus` per
row. `docs/jobs/.jdi-status/` added to manigot's own `.gitignore` (see
TASK-14 finding below for why this alone wasn't sufficient).

**TASK-9 — `tui/internal/ui/detail.go`, `tui/internal/job/jdistatus.go`.**
A fifth "log" tab (key `5`), reading `job.ReadJDIRunLogTail` (last 256 KiB,
prefixed with a truncation note if larger — a tail, not load-everything)
instead of a job-directory file, since the sidecar isn't tracked in git at
all. Shows a distinct "no run yet" placeholder vs. an empty-but-existing
log. Never editable.

**TASK-10 — `scripts/jdi.sh` (new), `Makefile`.** Mirrors `scripts/tui.sh`
exactly: resolves the checkout, execs `bin/manigot-jdi`. `make jdi` builds
it; `mg-jdi:jdi.sh` added to `LINKS` so `make install` symlinks it.

**TASK-11 — `tui/cmd/jdi/main.go`, `tui/internal/ui/app.go`.** CLI path:
`mg-jdi` prints `\a` after `Run` returns, regardless of outcome. TUI path:
`App.pollJDIBell`, hooked into `refreshJobs` (the closest thing this app has
to a poll tick), rings on the first observation of a job's status
transitioning into a `stopped:*` state — dedup via `App.jdiSeen`
(in-memory, per job Name); the very first observation of any job only seeds
the map, never rings, so a restarted TUI doesn't re-alert on an
already-stopped job.

**TASK-12 — `tui/internal/resolve/{resolve,commands}.go`,
`tui/internal/launch/launch.go`, `tui/internal/ui/{app,agents,detail}.go`.**
`resolve.Jdi()` (new `MANIGOT_JDI_BIN` override) and `launch.Jdi` start
`mg-jdi --job <id>` via `cmd.Start()` — no terminal emulator, no window at
all. Detail view gains `J` (mirroring `D` mark-done as a composite action,
not a single-agent launch — so it's handled alongside `D`, not through
`agentForKey`), the same `branchGuard` every mutating action uses, an
action-bar `[J] mg-jdi` button, and a footer hint. Seeds `a.jdiSeen` as
`JDIRunning` immediately on a successful launch so TASK-11's dedup doesn't
miss a run that finishes before the next poll.

**TASK-13 — `README.md`, `docs/AGENTS.md`.** New "Autonomous mode
(`mg-jdi`)" and "mg-jdi status & log" sections in `README.md` (sequence,
retry bound, marker, honesty caveat, badge, log tab, two-path
notification); keybindings table, installed-commands table, and file-tree
updated. `docs/AGENTS.md`'s Stack/Architecture/Commands/Job workflow
sections updated the same way, at the level of detail matching its existing
one-bullet-per-script style. `project-template/docs/AGENTS.md` deliberately
untouched (see TASK-1).

**TASK-14 — verification.** `go build ./...`, `go vet ./...`, `go test
./...` clean across every package in `tui/` throughout implementation (not
just at the end). One real manual `mg-jdi` run: a throwaway git repo/branch
with a stub `mg` standing in for `scripts/run.sh`'s `--print` path (no
docker/Claude Code credentials available in this environment, so this
substitutes for the real container invocation while exercising every other
line of `mg-jdi`'s own code for real — resolution, branch checkout, the
loop, the real `run.log`/`status` files, the real bell) — confirmed the full
analyst→developer→reviewer→finished path, the `NEEDS-HUMAN-INPUT:`
immediate-stop path, and — critically — caught a real bug (see Known
issues). Also confirmed, via a throwaway test against the real sidecar files
that run produced, that the TUI's actual `renderJobRow`/detail-view log tab
code correctly show the real badge and log content (removed after
confirming; not part of the committed test suite, which already covers this
functionality against synthetic fixtures).

## Known issues / follow-ups

- **Found and fixed during TASK-14, not just noted:** `docs/jobs/.jdi-status/`
  was only gitignored in manigot's own repo — the *target project* `mg-jdi`
  drives has no such exclusion unless its own tracked `.gitignore` happens
  to include it, so a real `git add -A` (as any `@developer`/`@reviewer`
  invocation does) swept the sidecar straight into a real job-branch commit.
  Fixed with `ensureSidecarIgnored` (`tui/cmd/jdi/output.go`), called once
  at `mg-jdi` startup: appends the sidecar pattern to the project's own
  `.git/info/exclude` (local-only, per-checkout, never itself committed) if
  not already present. Re-verified against the corrected build.
- **OpenCode support** — deferred per the brief/tasks.md Decision 3; `--print`
  is Claude Code only for v1, tracked in `docs/backlog.md`.
- **No real Docker/Claude Code end-to-end run** was possible from this
  environment (no docker, no credentials) — TASK-14's manual verification
  used a stub `mg` in place of `scripts/run.sh`'s container invocation,
  which exercises everything in `mg-jdi`'s own code but not
  `scripts/run.sh --print`/`entrypoint.sh`'s actual container behavior
  (TASK-2's own verification already covered the `-it`/prompt/fd-3 argv
  behavior via a stub `docker`, and c4ouwc's prior work already confirmed
  `claude --print` itself runs headless). A first real `mg-jdi` run against
  an actual job, watched from both a direct CLI terminal and the TUI's log
  tab, is recommended as a pre-merge host check — the same recommendation
  c4ouwc's reviewer made for its own live-TTY behavior.
- **30-minute staleness threshold** for a `running` status
  (`jdiRunningStaleAfter` in `tui/internal/job/jdistatus.go`) is a judgment
  call, not something the brief specified — generous enough not to flag a
  single long agent invocation as dead, but arbitrary. Worth revisiting once
  real usage shows how long invocations typically take.
- **mg-jdi's own hard iteration cap** (`maxIterations = 20` in
  `tui/cmd/jdi/main.go`) is a defensive backstop against an unforeseen bug
  in the state machine, independent of and in addition to the one-bounce
  retry budget `orchestrate.Next` already enforces — should never actually
  be reached in normal operation.
