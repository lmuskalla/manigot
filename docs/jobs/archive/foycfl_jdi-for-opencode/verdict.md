# Verdict: jdi for opencode

id: foycfl
status: open
reviewer: '@reviewer'
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: Investigation-only, no code change (as scoped). Independently
re-verified live against the real `opencode` binary in this environment
(same `opencode-ai@1.18.16` the Dockerfile installs unpinned): `opencode run
[message..] --agent <agent> --format json` exists, runs headless with no TTY,
honors `--agent`, auto-executes tool calls with no `--auto` flag needed, and
emits JSONL with `"text"`-typed events carrying `part.text`. Matches
implementation.md's account exactly.

TASK-2: PASS
notes: `scripts/entrypoint.sh`'s new `MANIGOT_PRINT` branch under `opencode`
correctly parses `--agent`/`--prompt` out of `"$@"` and re-emits
`opencode run <prompt> --agent <agent> --format json`. Reproduced the exact
translation live and got a correct agent response. `bash -n` passes on both
changed scripts. Interactive path (no `MANIGOT_PRINT`) untouched.

TASK-3: PASS
notes: `scripts/run.sh`'s guard now only rejects `--print` for the legacy
profile-less `--tool opencode` path (`-z "$PROFILE"`), which is the only way
`PROFILE` is left empty by the resolution logic above it — verified by
reading the full resolution block. `zai`/`opencode-go` profiles pass through.
Comment and error message updated accurately.

TASK-4: PASS
notes: `opencodeResultText` in `signal.go` correctly requires every non-blank
line to parse as JSON with a `type`, concatenates only `"text"`-event
`part.text`, and falls back to the untouched Claude-JSON/plain-text paths
otherwise. New tests cover match, non-text-event exclusion, and
malformed-line fallback. All existing signal tests still pass (`go test
./tui/...` green). Verified test fixtures match the real event shape opencode
actually emits (checked live: `tool_use` events carry `"type":"tool_use"`,
satisfying the non-empty-`type` requirement, and are correctly ignored since
`part.type != "text"`).

TASK-5: PASS
notes: `--profile` flag added to `mg-jdi`, defaults to `claude-pro`,
validated against `config.ProfileByID`, threaded through
`newCommandAgentRunner`/`commandAgentRunner.Run` replacing the hardcoded pin.
`TestCommandAgentRunnerUsesGivenProfile` confirms a non-default profile is
forwarded as `--profile` in the exec'd argv. Minor: no test exercises the
flag-validation branch in `main()` itself (invalid `--profile` value, or the
default-when-unset path) — only the runner level is covered. Not a blocker;
the logic is short and reviewed by inspection, but worth a follow-up test.

TASK-6: PASS
notes: `launch.Jdi` gained a `profile` parameter (empty → `claude-pro`,
matching `Agent`/`Quick`), forwarded as `mg-jdi --profile <profile>`. Its one
call site in `app.go` passes `a.settings.ProfileValue()`. Existing
`launch_test.go` calls updated for the new signature plus a new default vs.
explicit-profile test. `jdilaunch_test.go` (listed in tasks.md's files)
wasn't touched, correctly — it only exercises `Jdi` indirectly through
`updateDetail`, so the signature change doesn't affect it; all its tests
still pass.

TASK-7: PASS
notes: `docs/AGENTS.md`, `README.md`, `scripts/mg.sh`, and `docs/backlog.md`
all updated consistently; grepped the whole tree for stale "Claude Code
only"/"v1 is Claude Code only" jdi references and found none remaining.
Read-only mounts not touched (nothing under `.claude/` in this repo anyway).

TASK-8: PASS
notes: Verification-only, no code change (as scoped). Reasonable given no
`docker` in this sandbox either — the substitute verification (real
`opencode` CLI + real `orchestrate` package, live) is a fair proxy and was
independently reproduced here with the same results. `go build ./...`, `go
vet ./...`, `go test ./...`, `make jdi`, `make tui` all pass. Known-issue
about the missing full Docker E2E run is disclosed honestly in
implementation.md rather than glossed over.

## Security

None. No new secrets/credentials handling introduced; the OpenCode headless
path reuses the same provider-key forwarding `scripts/run.sh` already had.
No injection concern beyond what already existed — `OC_PROMPT`/`OC_AGENT` are
passed as discrete argv elements (arrays), not string-interpolated into a
shell command, so no quoting/injection risk from job-prompt content.

## Overall

APPROVED

All 8 tasks fulfil what tasks.md specified, commit discipline is followed
(`[foycfl] TASK-N: ...` per task, plus a separate implementation-summary
commit), `go build`/`go vet`/`go test`/`make jdi`/`make tui` all pass, and no
out-of-scope files were touched. The one non-blocking note: TASK-5's
`main()`-level `--profile` validation/default logic has no direct unit test
(only the runner-level forwarding is tested) — worth adding in a follow-up,
not required before merge.
