# Brief: part 2 of web ui tui path

status: done
type: feature
id: farmer
branch: feature/farmer_part-2-of-web-ui-tui-path
date: 2026-08-28
author: Leander Muskalla

## What

Job two of the control-plane sequence (`docs/listener.md`). Job one shipped
`mg serve` as a read-only daemon; this job adds mutating endpoints and live
run supervision on top of it, so the daemon becomes able to actually drive a
job's lifecycle, not just report on it.

- **Mutating endpoints**, each serialized per project via the existing
  `serve.ProjectLocks` skeleton (`internal/serve/locks.go` — `Lock(root)` /
  `defer Unlock(root)` around the critical section):
  - Create a job (`internal/job.CreateJob`).
  - Edit a job's `brief.md` (raw content replace — no `$EDITOR`, this is an
    HTTP body write).
  - Launch an agent detached, one-shot, via the `--print` path (never an
    attached/interactive session).
  - Launch `mg jdi` detached for a job.
  - `done` (`FinishJob`), `delete` (`DeleteJob`), push.
  - Orphan cleanup: `mg prune` (containers) and orphaned-worktree removal
    (`job.RemoveOrphans`) — a headless daemon with nobody running the CLI by
    hand will accumulate exactly this cruft, so this needs an endpoint (or a
    background sweep), not just CLI parity.
- **Live run supervision**: stream `session.log` growth over WebSocket/SSE
  instead of polling — the reader side of roadmap #5, riding the writer-side
  capture that already exists. This is the same file the TUI's `l` key tails
  (`tail -f` on `docs/jobs/<id>_<slug>/session.log`); job two exposes that
  same live tail over the network instead of a local pane. It is not a
  separate "container log" concept — `session.log` already is the captured
  stream for both containerized and host-mode runs.
- **`NEEDS-HUMAN-INPUT:` visibility**: job one's `/jdi` endpoint already
  surfaces the marker via the status sidecar. What job two needs to decide
  (see Notes) is whether "answering" one is its own endpoint or just the
  composition of the edit-brief + relaunch endpoints above.

## Why

The read-only daemon (job one) can only show what happened; nobody can act on
it without going back to the terminal. That defeats the point of a remote/VPS
control plane — the whole "review from a phone, get a ping when a job needs
you, act on it" story requires write access and live output, not just a
status snapshot. This job is what turns `mg serve` from a dashboard into an
actual control plane, and it's the direct prerequisite for job three (the web
UI) — building a frontend against polling instead of this job's streaming
would just be "a worse TUI with more pixels" (per `docs/listener.md`).

## Out of scope

- **Interactive agent sessions** (`mg`, `mg host`) — no attach-and-chat over
  the API in any form. Roadmap item 6 (in-TUI embedded terminal) is the one
  place this bet gets built, on whichever surface the product commits to, not
  duplicated here.
- **Host-machine config commands** (`mg profiles`, `mg theme`, `mg setup`,
  `mg serve-token`) — credential/environment configuration stays CLI-only.
- **Any frontend** (web UI or native GUI) — job three.
- **User accounts / teams** — bearer-token auth stays the v1 model.
- **`mg pull`** — independently useful, its own job.
- Anything not listed under "What" above.

## Notes

Open questions to resolve before this goes to @analyst:

- **`mg done` conflict handling.** Today a squash-merge conflict is an
  interactive prompt: offer to hand off to `@git-solver` via `mg host`, or
  roll back on decline. There's no human at a terminal to answer that prompt
  over HTTP. Pin what the `done` endpoint does instead — most likely: report
  the conflict as a structured error and leave the job untouched (no
  automatic rollback, no automatic git-solver handoff), requiring an explicit
  follow-up decision through some other call. Do not let this fall back to
  silently picking one of the two existing CLI behaviors.
- **What "answer `NEEDS-HUMAN-INPUT:`" actually means.** There is no existing
  "answer" mechanism even in the CLI/TUI today — a human currently just edits
  the relevant file or launches an ordinary interactive agent, then re-runs
  `mg jdi`. Decide whether job two needs a dedicated endpoint at all, or
  whether "answering" is just: edit-brief (or another job file) + relaunch
  `mg jdi`, using the endpoints already in scope. Favor the latter unless a
  concrete need for something more shows up.
- **Streaming transport**: WebSocket vs. SSE — either is fine technically for
  a one-way tail; pick whichever is less code against the existing
  `net/http` server, and confirm it survives the daemon's graceful-shutdown
  drain (`serveShutdownDrain`) cleanly (client disconnect on shutdown, not a
  hang).
- **`ProjectLocks` scope**: confirm which operations actually need the lock.
  Per `internal/serve/locks.go`'s own doc comment, it's for git-mutating
  operations (create/done/delete/push) contending on the same project's git
  metadata — launching an agent or streaming logs doesn't touch git state and
  arguably shouldn't block behind the same lock. Don't serialize more than
  necessary; a job-launch that has to wait behind an unrelated `done` defeats
  the point of a responsive control plane.
- Reuse job one's zero-path-input discipline exactly: every mutating handler
  resolves project/job through the same `resolveProject`/`resolveJob` helpers
  in `internal/serve/api.go` — never a new ad hoc path-join.


