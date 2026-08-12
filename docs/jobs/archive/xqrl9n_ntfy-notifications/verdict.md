# Verdict: ntfy notifications

id: xqrl9n
status: open
reviewer: reviewer
date: 2026-08-12

## Review

TASK-1: PASS
notes: `internal/notify/notify.go` implements the ntfy publish client exactly
as specified — config via `config.EnvValue` (`NTFY_URL` default
`https://ntfy.sh`, trailing slash tolerated, `NTFY_TOPIC` activation key,
optional `NTFY_TOKEN` sent as `Bearer`), `http.Client` with the ~10s
`defaultTimeout`, `Publish` returns errors to the caller, strict no-op when
`NTFY_TOPIC` is unset, and `Enabled()` for callers to skip message building.
Unit tests in `internal/notify/notify_test.go` use `httptest.Server` and cover
defaults, env parsing, request shape (method/path/body/Title/Priority/Tags/
Authorization headers), optional-header omission, server errors, and both
redaction rules. **The previous verdict's sole blocker is fixed**: the
request-build path in `Publish` now applies the same `*url.Error` stripping as
the transport path (notify.go:103-108), guarded by the new
`TestPublishBuildErrorRedactsURL` (malformed `NTFY_URL` with an interior space
— a plausible misconfiguration). I independently probed the error chain for
several malformed-URL shapes (space in host, bad `%` escape, missing `]` in
host, missing scheme): `errors.As` always matched the outer `*url.Error`
containing the URL, and `uerr.Err` was the plain inner error with no URL —
the strip is effective in every case, the topic never leaks. `go build`,
`go vet`, and all package tests pass.

TASK-2: PASS
notes: `notifyStop` (cmd/mg/jdi.go:192) is called in `runJDI`'s single stop
path after the terminal bell (cmd/mg/jdi.go:177), so both direct CLI runs and
TUI-launched detached runs notify (`launch.Jdi` execs the same `mg jdi`
subprocess — internal/launch/launch.go:189). `StopFinished` →
`white_check_mark` at default priority (no Priority header); `StopNeedsHuman`
→ `warning` at priority 4; both carry the job name in the Title and
`result.Reason` in the body. Send failures warn on stderr
(`mg jdi: warning: could not send ntfy notification: ...`) and never abort
the exit path; the terminal bell is kept. Tests
`TestNotifyStopFinished`, `TestNotifyStopNeedsHuman`,
`TestNotifyStopUnconfiguredSendsNothing`,
`TestNotifyStopSendFailureWarnsButDoesNotAbort` pass. Unconfigured behavior is
byte-for-byte unchanged (early return before any side effect).

TASK-3: PASS
notes: `job.StaleRunningJDI` (internal/job/jdistatus.go:147) shares the raw
sidecar parse (`readJDIStatusRaw`) and the same `jdiRunningStaleAfter`
staleness constant as `ReadJDIStatus`, whose degrade-away behavior is
preserved. No false positives: fresh `running`, `stopped:*` (any age),
missing, or unparseable sidecars all report ok=false. `notifyCrashedRun`
(cmd/mg/jdi.go:226) runs at the next run's start (after `resolveJob`, before
the loop), sends a priority-4 `warning` with the job name, the stale-since
timestamp, and the last agent, then dedups by overwriting the stale sidecar
with `stopped:needs-human` only after a successful send — a failed send
leaves it stale so the notification retries next start; a failed dedup write
is warn-only. Strict no-op when unconfigured: nothing is read-modify-written
(`TestNotifyCrashedRunUnconfiguredLeavesSidecarUntouched`). Tests cover
notify+dedup, fresh-running, stopped, and unconfigured; all pass. Accepted
scope items (documented in implementation.md): a Ctrl+C'd CLI run leaves a
stale `running` sidecar indistinguishable from a SIGKILL (reported as
"crashed" next start), and a process killed before its first sidecar write
leaves no trace — both consistent with the tasks' accepted
dead-process-cannot-notify approximation.

TASK-4: PASS
notes: `setupNtfy` (cmd/mg/setup.go:269) follows the established
`promptValue`/`promptSecret` wizard pattern with the shared `bufio.Reader`
contract: `NTFY_URL` default `https://ntfy.sh`, `NTFY_TOPIC`, `NTFY_TOKEN` as
a secret; "✓ Already configured" early return when `NTFY_TOPIC` is set
(masked via `cli.Mask`); empty topic/token are not written (opt-in stays
off); runs only in the no-profile walk; `--check` left unchanged per the
task's "otherwise leave it out" instruction (ntfy is not profile-shaped).
Help text updated. Tests `TestSetupNtfyWritesEnv`,
`TestSetupNtfySkipsWhenTopicEmpty`, `TestSetupNtfyAlreadyConfigured`,
`TestSetupWizardShowsNtfyBlock` pass.

TASK-5: PASS
notes: `docs/AGENTS.md` (ntfy keys added to the Config-files `.env` bullet;
stop/crash notification behavior added to the `mg jdi` section, including
the `NTFY_URL` default, optional `Bearer` token, and the never-abort warning
semantics) and `README.md` ("Getting notified" paragraph in the Autonomous
mode section) are synced with the implemented behavior. The literal "mirror
into `project-template/docs/AGENTS.md`" instruction was not followed — I
verified that file is a generic per-project placeholder (`# [Project Name]`,
no Config-files/jdi/.env sections) rather than a mirror of the manigot system
docs, so the deviation is correct and consistent with the approved rkj9qc
precedent; it is documented in implementation.md. `docs/CLAUDE.md` is empty
and no `agents/*.md` mentions ntfy/NTFY/jdi — verified, no changes needed
(ntfy is host-side only).

## Security

The "never log the URL+token combination" rule is now correctly enforced on
both error paths: `Publish` strips the request URL from transport errors
(`httpc.Do`, guarded by `TestPublishTransportErrorRedactsURL`) and from
request-build errors (`http.NewRequest`, guarded by the new
`TestPublishBuildErrorRedactsURL`). `NTFY_TOKEN` itself never appears in any
error (it only goes into the `Authorization` header), and `mg setup` masks
both `NTFY_TOPIC` and `NTFY_TOKEN` in its output. The previously committed
root-level `mg` build binary is untracked (`git ls-files` clean, confirmed)
and `/mg` is in `.gitignore`; no `.env` or credentials are tracked.

## Overall

APPROVED

All five tasks are implemented to spec. `go build ./...`, `go vet ./...` and
`go test ./...` pass. Commit discipline is clean: one commit per task in the
`[xqrl9n] TASK-N:` format, implementation.md has its own commits, and the two
follow-up `fix:` commits (build-path redaction, binary untrack) are precisely
scoped with their own implementation.md documentation commits. Both blockers
from the prior review cycle are resolved: the request-build URL leak is fixed
with a regression test, and the stray `mg` binary is removed from tracking
and ignored. The only deviations from the literal task text
(project-template/docs/AGENTS.md mirror; `--check` not covering ntfy) are
documented in implementation.md and consistent with the approved precedent.
Nothing must change before merge.
