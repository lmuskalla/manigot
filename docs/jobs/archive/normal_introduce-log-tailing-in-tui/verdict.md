# Verdict: introduce log tailing in tui

id: normal
status: open
reviewer: @reviewer
date: 2026-08-27

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `launch.Tail(logPath, terminal)` added to src/internal/launch/launch.go
(plus the `tailShellCommand` builder). Follows the established `Tig` shape:
inner command is `tail -f '<logPath>'` (absolute path, shellQuote'd, no
cd-first), spawned via the existing `launchDetached` path — tmux split pane
with the replace policy inside tmux, terminal override / auto-detect chain
outside — returning the short "where it opened" description. The deliberate
no-holdOnFailure deviation is correct and documented: a user ends `tail -f`
with Ctrl+C (exit 130, non-zero), so wrapping would leave a "press enter to
close" prompt every time. No ExeOverride hop, no profile flag — correct for a
plain host command.

TASK-2: PASS
notes: src/internal/launch/launch_test.go adds the four Tail tests: exact
`tail -f '<path>'` format, explicit no-holdOnFailure assertion (`ec=$?`
absent), spaces-quoting, embedded-quote escaping (`'\''`), and a full Tail
launch through the existing tmuxStub asserting the split-window invocation
carries the tail inner command, the pane is tagged (`select-pane -t %100 -T
manigot`), and the description is "tmux pane". Mirrors the Tig test
conventions; statically correct against the stub.

TASK-3: PASS
notes: `case "l"` wired in `App.updateDetail` (src/internal/ui/app.go),
placed between the "t" and "c" cases. Gated on `detailView.runLogExists()`
(os.Stat on `job.JDIRunLogPath`), calls `launch.Tail` with the absolute
run.log path derived from the same `a.detail.job.Root` the gate checks
(commit 20799b5), surfaces `"→ tailing run.log in <desc>"` on success or
`cmdErrorText` on failure, and reports the gate ("no mg jdi run has happened
for this job yet") otherwise. No branch guard — correct, the sidecar is
job-name-keyed. Key-collision verified: `l` is used by no other detail-view
binding (detail.go's update handles 1-6/tab/scroll only; agentMeta uses
a/o/d/r/s; the list view's `l` is a separate state).

TASK-4: PASS
notes: conditional `· l tail` hint in `detailView.renderFooter`
(src/internal/ui/detail.go), gated on the same shared `runLogExists` helper
as the key, so key and hint can never disagree. Existing footer tests assert
substrings only and none of their jobs has a run.log, so the addition is
safe.

TASK-5: PASS
notes: src/internal/ui/tail_test.go (new) mirrors tig_test.go conventions:
footer hint shown only when a run.log exists; `l` with no run.log reports the
gate and never reaches the launch path (stub call log absent/empty); `l` with
a run.log launches the tail pane (split-window invocation contains
`tail -f '<path>'`, pane tagged, status reports "→ tailing run.log in tmux
pane"). Statically verified against `writeTmuxStub`'s `%100` pane id and the
`NewApp`/`newDetailView`/`discoverOneJob`/`keyMsg` helpers; the non-repo
TempDir jobs and stub-only PATH exercise the same graceful-degradation paths
the tig tests already rely on.

TASK-6: PASS
notes: README.md updated (detail-view Keybindings `l` row + a "Live tail
pane" bullet in the "mg jdi status & log" section, including the accurate
per-invocation-update and idle-after-run notes); docs/AGENTS.md and
project-template/docs/AGENTS.md TUI-key paragraphs updated and kept in sync;
the key-collision inventory comment in src/internal/ui/agents.go updated to
include `l tail` — a directly-related doc-comment change, not scope creep.
draft-brief-capture-agent-sessions.md confirms session.log tailing is a
separate, later job, so tailing run.log is the right target.

Open decisions from tasks.md all resolved per the analysis recommendations
and documented in implementation.md: key `l` (t is taken by tig), log file
run.log, file-exists gate, no holdOnFailure wrap.

Non-blocking nits (no change required): README's "Two places ... plus a live
tail" reads slightly off now that the section lists three items; tail_test.go
is missing a trailing newline (gofmt cosmetic).

## Security

none run — the change spawns `tail -f` on a path constructed from
job.JDIRunLogPath (job-name-keyed, shellQuote'd against injection), reads no
new secrets into the TUI, and adds no network surface. The tail pane is
subject to the same tmux replace policy as agent/tig panes.

## Overall

APPROVED

All six tasks are implemented as specified, the four open decisions match the
analysis's recommendations, tests follow established conventions, and no
out-of-scope changes were found. No blockers.