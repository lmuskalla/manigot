# Verdict: cannot copy from agents

id: experiment
status: open
reviewer: deepseek-v4-flash
date: 2026-08-17

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed via `git diff main...HEAD` (base `main` per `.manigot/manigot.json`),
cross-referenced against tasks.md. Only the files listed in tasks.md were
touched — no scope creep.

TASK-1: PASS
notes: internal/session/docker.go — `terminalEnvVars` covers exactly the ten
vars named in the task (TERM, COLORTERM, TERM_PROGRAM, TERM_PROGRAM_VERSION,
VTE_VERSION, KITTY_WINDOW_ID, TMUX, TMUX_PANE, WT_SESSION) plus every
WEZTERM_* var via os.Environ(); each forwarded as a docker `-e KEY=VALUE`
entry only when set and non-empty on the host, appended in
BuildDockerInvocation (line 345) alongside the other env flags. No var set →
empty slice → argv byte-identical to before (the Dockerfile sets no TERM, so
the container previously saw none of these). Test
TestBuildTerminalEnvForwarding covers both the nothing-set (no forwarding)
and all-set (each var forwarded, WEZTERM_* included) cases. Existing pinned
argv tests use containsAll presence checks, so the conditional forwarding
cannot break them even when the test environment has TERM set. Forwarding
applies to --print runs too, which the task explicitly declares harmless.
`mg host` correctly untouched (full host env already present).

TASK-2: PASS
notes: internal/session/docker.go — `warnTmuxClipboard` (lines 94-112) is
called at the top of BuildDockerInvocation (line 131) with
`interactive`/`opts.Print`; all guards match the spec: skipped when not
interactive or --print, when $TMUX is unset, when tmux is not on PATH, and on
any tmux query failure — strictly read-only, no state mutation. The warning
goes to diag (stderr, the diagnostics stream), so the --print stdout contract
cannot be corrupted. Exact-match on trimmed "set-clipboard off" output is
correct (`on`/`external`/unset → no warning). Interactive flag flows
correctly from the callers: cli.IsTerminal(stdin) on the bare-mg path
(cmd/mg/session.go:36), false on the mg-jdi --print path (cmd/mg/jdi.go:359).
Seven test cases in docker_test.go cover off/on/external/no-binary/outside-
tmux/print/failure. Warning text is accurate and actionable.

TASK-3: PASS (N/A per investigation)
notes: Investigation-gated task whose explicitly permitted outcome is N/A
when no config knob exists. The writeup in implementation.md is detailed and
internally consistent: opencode-ai 1.18.18's clipboard.ts emits OSC 52
unconditionally on a TTY (no terminal-identity gating), wraps it in tmux DCS
passthrough when TMUX is set, and falls back to native clipboard tools absent
from the container; no clipboard/copy/autocopy key exists in either the
opencode.json or tui.json schemas, so there was nothing for the entrypoint to
write, and the model-substitution block in entrypoint.sh is correctly left
untouched. Note: opencode-ai is baked into the Docker image and not present
in this workspace, so the claim could not be independently re-verified from
this review session; the finding is plausible and the N/A path is sanctioned
by the task, so this is a note, not a blocker.

TASK-4: PASS
notes: The "Clipboard / copying from agent sessions" section was added to
docs/AGENTS.md, mirrored into README.md, and the project-template/docs/AGENTS.md
comment updated — the three doc sources are consistent with each other and
with the implementation (the exact forwarded-var list, the set-and-non-empty
condition, the two user-side prerequisites, and the TASK-2 warning). The
hard-rule sync requirement is met.

TASK-5: PASS
notes: Verification recorded in implementation.md: go build/vet/test claimed
green including the new and updated docker tests; the docker env smoke check
was legitimately skipped (no docker daemon/binary in the agent environment —
the task's "if available" condition), with the env-forwarding behavior pinned
by the unit tests instead. Residual limitation (terminals without OSC 52
support cannot be fixed by mg) is documented. Caveat: this review session's
bash is restricted to git, so I could not independently re-run the build/test
suite; the code was verified by inspection and the tests are sound.

## Security

No security findings. Forwarded variables (TERM, COLORTERM, TERM_PROGRAM*,
VTE_VERSION, KITTY_WINDOW_ID, TMUX, TMUX_PANE, WT_SESSION, WEZTERM_*) carry
no credentials. The tmux check is strictly read-only and failure-silent.

## Overall

APPROVED

Nothing must change before merge. Non-blocking observations for the record:
(1) tasks.md (the analyst's file) was filled in and committed inside the
TASK-1 commit rather than as its own commit, and implementation.md was built
up across the TASK-3 and TASK-5 commits rather than a single dedicated
commit — all commits still follow the `[experiment] TASK-N:` format, so this
is hygiene only; (2) the opencode-ai install is unpinned, so the TASK-3 N/A
verdict should be re-checked on the next image rebuild (already noted as a
follow-up in implementation.md).