# Tasks: Fix tailing

id: guest
status: open
analyst: analyst
date: 2026-08-28

<!-- Produced by @analyst from brief.md. -->

## Analysis

The "tail" feature (`l` in the detail view) spawns `tail -f` on the mg-jdi
`run.log` sidecar (`.manigot/jdi-status/<job>/run.log`) — the exact same file
the log tab (key 5) renders. That is the whole problem, confirmed by reading
the code:

- **No stream / buffered:** `run.log` only grows at invocation boundaries —
  `logAgentInvoked` immediately before an agent starts, `logInvocation` after
  it finishes (cmd/mg/jdi.go + jdioutput.go). During a long-running invocation
  nothing is appended, and the invocation's raw output is captured whole into
  a `bytes.Buffer` (commandAgentRunner.Run → DockerInvocation.Run's
  `cmd.Stdout`) and only flushed afterwards. So the tail shows the same
  sectioned summary as the log tab, nothing live.
- **Output IS being grabbed:** the raw output (opencode `--format json` JSONL
  stream, claude stream-json) is fully captured and already persisted verbatim
  to a different, more verbose log — `docs/jobs/<id>_<slug>/session.log`
  (appendSessionLog, post-hoc per invocation). The existing opencode JSONL
  tests pin that the capture + prose extraction works. What's missing: the
  tail doesn't follow it, and it isn't written live.
- **Log vs tail distinction:** the log tab (events: invoked / result / next
  invoked) = `run.log`; the tail should be the verbose raw stream =
  `session.log`. The fix is to make `session.log` grow live during each
  invocation and point the `l` key at it.

Design decision (recommended, low scope): keep `session.log` in the job's own
folder (`docs/jobs/<id>_<slug>/session.log`) exactly where it is today — the
prior capture job deliberately put it there so the sweep commits it, `mg done`
carries it into the archive, `mg delete` removes it with the job, and the
serve `/jdi` endpoint already serves it. Only the *write timing* changes
(post-hoc → live). Moving it to the sidecar dir is rejected: `mg done`'s
RemoveJDIStatus would delete it on archive, and finish/delete/serve would all
need changes — scope creep the brief doesn't ask for.

Open question for the developer to verify (TASK-1): whether the CLIs inside
the container flush their stdout per event when writing to a pipe (the
"seems buffered" suspicion). The host-side tee will stream whatever chunks
docker forwards; per-event flushing by opencode/claude is likely but should be
confirmed. If a CLI block-buffers, the tail still shows the verbose stream —
just in bursts — which is strictly better than today's post-hoc summary.

## Task breakdown

TASK-1: Verify the invocation-output capture end to end (opencode in
     particular, since the brief names it) and pin it with a test — the
     concrete answer to "check if we're actually grabbing the output or not":
     confirm `cmd.Stdout` wiring captures the full `--print` stream (no bytes
     dropped), note the observed streaming granularity (per-event vs
     buffered-burst), and add/extend a test proving the raw output lands in
     the job's session.log for an opencode-shaped JSONL stream.
     files: src/cmd/mg/jdi.go, src/cmd/mg/jdi_test.go, src/internal/session/docker.go
     depends: none
     risk: low — investigation plus a pin; the capture path is already
     exercised by the existing opencode JSONL tests, so this confirms and
     documents rather than introducing new mechanism.

TASK-2: Make mg-jdi stream each invocation's raw output live into the job's
     session.log — write the section header (agent, attempt, timestamp)
     before the invocation and tee the captured bytes into the file as they
     arrive during the run — replacing the post-hoc appendSessionLog call.
     Keep run.log exactly as the event summary (log tab unchanged). Update the
     loop-exit sweep's comment, which currently exists to commit the final
     post-hoc session.log section (the per-invocation sweep now covers it).
     Keep the file in the job dir so the archive-carry, mg delete, and the
     serve /jdi endpoint stay untouched.
     files: src/cmd/mg/jdi.go, src/cmd/mg/jdioutput.go, src/cmd/mg/jdi_test.go, src/cmd/mg/jdioutput_test.go
     depends: TASK-1
     risk: medium — touches the AgentRunner capture path (runner interface or
     internals) and the session.log write mechanism; pinned tests
     (TestRunWritesSessionLogPerInvocation, the appendSessionLog unit tests)
     and the fake runner must be adapted, but the change is contained to
     cmd/mg.

TASK-3: Point the detail view's "l" tail at the job's session.log (the
     verbose log) instead of run.log: pass filepath.Join(job.Dir,
     "session.log") to launch.Tail, gate the key + footer hint on that file
     existing (create it at mg-jdi run start so the gate is stable from the
     moment a run begins, mirroring run.log's timing), and update the status
     text and comments. The log tab (key 5) stays on run.log — this is what
     makes the log-vs-tail difference real.
     files: src/internal/ui/app.go, src/internal/ui/detail.go, src/internal/ui/tail_test.go
     depends: TASK-2
     risk: low-medium — small surface, but pinned test expectations change
     (tail_test.go asserts the run.log path and the "→ tailing run.log" status)
     and the gate's file-existence target moves.

TASK-4: Update the documentation describing the tail feature to the new
     log-vs-tail distinction (log tab = run.log event summary; "l" tail =
     live verbose session.log stream): docs/AGENTS.md, README.md,
     project-template/docs/AGENTS.md, and the ROADMAP's "reader side remains
     open" note for the session.log tail.
     files: docs/AGENTS.md, README.md, project-template/docs/AGENTS.md, docs/ROADMAP.md
     depends: TASK-2, TASK-3
     risk: low — docs only, but keep agents/*.md and
     project-template/docs/AGENTS.md in sync per the hard rules.