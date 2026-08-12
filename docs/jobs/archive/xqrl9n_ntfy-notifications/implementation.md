# Implementation: ntfy notifications

id: xqrl9n
status: open
developer: opencode-go
date: 2026-08-12

## Summary

Implemented opt-in ntfy push notifications for `mg jdi` so unattended runs
on a VPS get attention when they stop or crash. A new `internal/notify`
package is an ntfy publish client (config from `config.EnvValue`, ~10s
timeout, errors returned to the caller, strict no-op when `NTFY_TOPIC` is
unset). `runJDI`'s stop path now sends a success notification
(`white_check_mark`) on `StopFinished` and a high-priority attention one
(`warning`, priority 4) on `StopNeedsHuman`, and the next run's start
detects a previously killed `mg jdi` process (a `running` sidecar stale past
`jdiRunningStaleAfter`) and notifies about it, deduplicating once notified.
`mg setup` gained an optional ntfy block prompting `NTFY_URL` (default
`https://ntfy.sh`), `NTFY_TOPIC` and `NTFY_TOKEN`. Docs synced in
`docs/AGENTS.md` and `README.md`. Unconfigured behavior (no `NTFY_TOPIC`) is
byte-for-byte identical to before; send failures warn on stderr and never
abort the run.

## Changes

TASK-1: Created `internal/notify` — an ntfy publish client (`Client`,
`Message`, `FromConfig`, `Enabled`, `Publish`). `Publish` POSTs to
`{URL}/{Topic}` with `Title`/`Priority`/`Tags` headers and an optional
`Bearer` `Authorization` header; errors never embed the request URL (the
topic is effectively a password — see
`TestPublishTransportErrorRedactsURL`). Unit tests in
`internal/notify/notify_test.go` use `httptest.Server`.

TASK-2: Wired stop notifications into `runJDI` (`cmd/mg/jdi.go`) via a new
`notifyStop` helper called after the existing terminal bell — both direct
CLI runs and TUI-launched runs go through this same stop path. Success →
`white_check_mark` at default priority; needs-human → `warning` at priority
4; both carry the job name and `result.Reason`. Tests in `cmd/mg/jdi_test.go`
prove both stops notify and that an unconfigured env sends nothing.

TASK-3: Added crash detection. `internal/job/jdistatus.go` gained a shared
raw-sidecar parse (`readJDIStatusRaw`) plus the exported `StaleRunningJDI`
helper (same `jdiRunningStaleAfter` staleness semantics as `ReadJDIStatus`,
which keeps its degrade-away behavior). `cmd/mg/jdi.go`'s new
`notifyCrashedRun` runs at the next run's start: on a stale-`running`
sidecar it sends a priority-4 `warning` notification and overwrites the
sidecar with `stopped:needs-human` so a later start doesn't re-notify about
the same already-known crash (a failed send leaves the sidecar stale so the
notification is retried next start). Tests cover both the notify+dedup path
and the no-false-positive cases (fresh running, stopped, unconfigured leaves
the sidecar untouched).

TASK-4: Added the optional ntfy block to the `mg setup` wizard
(`cmd/mg/setup.go`, `setupNtfy`), following the existing
`promptValue`/`promptSecret` pattern and the wizard's `bufio.Reader`
contract; `NTFY_TOPIC` already set → "✓ Already configured" early return.
Runs only in the no-profile walk (ntfy is not a subscription profile), and
`--check` is left unchanged since its per-profile shape does not fit ntfy.
Help text updated; tests in `cmd/mg/setup_test.go`.

TASK-5: Documented the feature — `docs/AGENTS.md` (ntfy keys in the
Config-files `.env` bullet, stop/crash notification behavior in the `mg jdi`
section) and a "Getting notified" paragraph in `README.md`'s Autonomous mode
section. Verified `docs/CLAUDE.md` (empty), `agents/*.md`, and
`project-template/docs/*` contain no jdi/notification references and need no
changes (ntfy is host-side only, not agent-facing).

## Known issues / follow-ups

- Verdict fix (NEEDS WORK, 2026-08-12): `Publish`'s request-build error path
  leaked the URL (and thus the topic, a password) when `http.NewRequest`
  failed on a malformed `NTFY_URL` — the `errors.As` stripping was only
  applied to the transport (`httpc.Do`) path. Fixed by applying the same
  `*url.Error` stripping to the build path, with a new
  `TestPublishBuildErrorRedactsURL` guarding it (malformed-URL case alongside
  `TestPublishTransportErrorRedactsURL`).
- Blocker fix (verdict NEEDS WORK): the compiled root-level `mg` binary
  (20 MB, accidentally committed in TASK-2, re-committed in TASK-4) is
  untracked (`git rm --cached mg`) and the root `mg` path is now in
  `.gitignore` to prevent recurrence — build output remains `bin/mg`.
- `project-template/docs/AGENTS.md` was NOT changed, deviating from TASK-5's
  literal "mirror the same wording" instruction: that file is a generic
  per-project agent-context template (placeholders, no Config-files/jdi
  sections), not a mirror of the manigot system docs — the same conclusion
  the precedent doc-sync task (rkj9qc TASK-6) reached by touching only
  README.md + docs/AGENTS.md. Putting manigot host-side `.env` config into a
  template copied into arbitrary new projects would be wrong.
- Crash detection limitation (accepted scope): a process killed before it
  ever writes its first status sidecar leaves no on-disk trace for the next
  start to find — the TUI's `jdiSeenFallbackTTL` covers the TUI's own launch
  window, and a direct CLI run that dies that early goes unnoticed. An
  external watchdog would be needed to close this; out of scope per the
  tasks.
- Config key names (`NTFY_URL`/`NTFY_TOPIC`/`NTFY_TOKEN`) were implemented
  as proposed in the tasks' open questions — no instruction to change them
  was given, and TASK-3's crash scope was kept (not cut).
