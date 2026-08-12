# Tasks: ntfy notifications

id: xqrl9n
status: open
analyst: analyst
date: 2026-08-12

<!-- Produced by @analyst from brief.md. -->

## Scope

manigot's `mg jdi` runs unattended on a remote VPS. The user wants a push
notification (via their existing ntfy server) when a jdi run stops, so it
gets attention without watching the terminal. Scope decisions:

- Notifications fire only for `mg jdi` runs — both direct CLI runs and
  TUI-launched runs go through the same `mg jdi` subprocess (the TUI spawns
  it detached via `launch.Jdi`), so wiring the notification into `runJDI`'s
  stop path covers both. Bare interactive `mg` sessions and the TUI's own
  terminal bell are unchanged.
- Notifications are **opt-in**: when `NTFY_TOPIC` is unset in manigot's
  `.env`, nothing is sent and behavior is byte-for-byte identical to today.
- Config lives in manigot's `.env` (the existing credential store, read via
  `config.EnvValue`): `NTFY_URL` (default `https://ntfy.sh`), `NTFY_TOPIC`
  (required, activates the feature), `NTFY_TOKEN` (optional, sent as a
  `Bearer` Authorization header). ntfy topics are effectively a password —
  never log the URL+token combination.
- A notification-send failure must never abort the jdi loop: warn on stderr
  (`mg jdi: warning: ...`), matching the existing sidecar/run-log warnings.

## Task breakdown

TASK-1: Create the `internal/notify` package — an ntfy publish client that
POSTs a message to `{NTFY_URL}/{NTFY_TOPIC}` with `Title`/`Priority`/`Tags`
headers and an optional `Bearer NTFY_TOKEN` Authorization header, reads its
config from `config.EnvValue`, uses an `http.Client` with a short timeout
(≈10s), returns errors to the caller, and is a no-op when `NTFY_TOPIC` is
unset; include unit tests using an `httptest.Server`.
     files: internal/notify/notify.go, internal/notify/notify_test.go
     depends: none
     risk: low — a new isolated package; no existing code is touched.

TASK-2: Wire ntfy notifications into `mg jdi`'s stop path in `runJDI` — send
a success notification (default priority, e.g. tag `white_check_mark`) when
`result.Kind == StopFinished` and a high-priority attention notification
(e.g. tag `warning`, priority 4) when `StopNeedsHuman`, with a message
containing the job name and `result.Reason`; log send failures as stderr
warnings and never abort the run; keep the existing terminal bell; add tests
proving both stops notify and that an unconfigured env (no `NTFY_TOPIC`)
sends nothing.
     files: cmd/mg/jdi.go, cmd/mg/jdi_test.go
     depends: TASK-1
     risk: low — touches only the post-Run exit path; the unconfigured
     default must remain a strict no-op so existing runs and tests are
     unchanged.

TASK-3: Detect a previously crashed/killed `mg jdi` process at the next run's
start and notify (a raw read of the jdi-status sidecar whose state is still
`running` but stale past `jdiRunningStaleAfter` means the last run died
without ever reporting a stop). SCOPE DECISION — the brief says "when it
crashes": a SIGKILLed/OOM-killed process cannot notify from inside itself, so
this stale-sidecar check on next start is the self-contained approximation;
an external watchdog is a bigger design and is NOT included. If the user
prefers to defer crash detection entirely, cut this task.
     files: cmd/mg/jdi.go, internal/job/jdistatus.go (small exported read
     helper if needed), tests
     depends: TASK-1; order-independent of TASK-2
     risk: medium — must not false-positive on an actively running mg jdi
     for the same job (reuse the TUI's staleness semantics) or re-notify on
     every start about an old, already-known crash.

TASK-4: Add an optional ntfy block to the `mg setup` wizard so the three keys
are discoverable without hand-editing `.env` (prompt `NTFY_URL` default
`https://ntfy.sh`, `NTFY_TOPIC`, and `NTFY_TOKEN` as a secret, written via
`config.UpsertEnv`), following the existing `promptValue`/`promptSecret`
pattern; extend `--check` only if it stays profile-shaped (otherwise leave it
out).
     files: cmd/mg/setup.go, cmd/mg/setup_test.go
     depends: TASK-1
     risk: low — purely additive, mirrors the established wizard pattern.

TASK-5: Sync documentation — add `NTFY_URL`/`NTFY_TOPIC`/`NTFY_TOKEN` to the
Config-files section and the jdi stop-notification behavior to the jdi
section of `docs/AGENTS.md` (the canonical source), mirror the same wording
into `project-template/docs/AGENTS.md` (hard rule), add a short README
section, and verify `docs/CLAUDE.md` and `agents/*.md` need no changes
(ntfy is host-side only, not agent-facing).
     files: docs/AGENTS.md, project-template/docs/AGENTS.md, README.md,
     possibly docs/CLAUDE.md
     depends: TASK-2 (document the implemented behavior, not the plan)
     risk: low — doc-only; the three AGENTS.md copies must stay in sync.

## Open questions

- TASK-3's crash scope: confirm the stale-sidecar-on-next-start approach (or
  cut it) — a process-level crash notification while the process is dead is
  impossible without an external watchdog, which is out of scope here.
- Config key names (`NTFY_URL`/`NTFY_TOPIC`/`NTFY_TOKEN`) are a proposal —
  the user's ntfy server URL and topic should be confirmed before `mg setup`
  wording is finalized.
