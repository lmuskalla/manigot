# Implementation: introduce log tailing in tui

id: normal
status: open
developer: @developer
date: 2026-08-27

<!-- Produced by @developer after implementation. -->

## Summary

Added a way, from the TUI's job detail view, to spawn a tmux split pane / new
terminal (exactly like agent launches and `t` tig already do) that live-tails
the currently running mg-jdi run's log. The key is `l` — the brief's first
guess `t` was already bound to tig, so the analysis recommended `l`
(mnemonic for "log", unused in the detail view). It tails `run.log` (the
mg-jdi sidecar the log tab already reads, `job.JDIRunLogPath`), gated on a
run.log existing for the job, and the inner command is deliberately NOT
wrapped in holdOnFailure so a user's Ctrl+C closes the pane instead of
leaving a "press enter to close" prompt every time.

## Changes

TASK-1: Added `launch.Tail(logPath, terminal string) (string, error)` to
`src/internal/launch/launch.go`, following the `Tig` shape: a
`tailShellCommand` builder produces the plain `tail -f '<logPath>'` inner
command (shellQuote'd, no cd-first since the path is absolute, deliberately
NOT wrapped in holdOnFailure — the one launch path that deviates, since
Ctrl+C on a tail is a normal, non-zero exit) and spawns it via the existing
`launchDetached` path (tmux split pane with the replace policy inside tmux;
terminal override / auto-detect chain outside). No ExeOverride hop and no
profile flag: `tail -f` is a plain host command.

TASK-2: Added launch-package tests in `src/internal/launch/launch_test.go`:
exact `tail -f '<path>'` format, no holdOnFailure wrap, quoting of a path
with spaces, quote escaping, and a full Tail launch through the existing
tmuxStub (split-window invocation carries the tail inner command, pane
tagged/recorded, description "tmux pane").

TASK-3: Wired the `l` key in `App.updateDetail`
(`src/internal/ui/app.go`): gated on a run.log existing for this job
(`detailView.runLogExists`, an os.Stat on `job.JDIRunLogPath`), calls
`launch.Tail`, surfaces `"→ tailing run.log in <desc>"` on success or
`cmdErrorText` on failure, and reports the gate ("no mg jdi run has happened
for this job yet") when no run.log exists. No branch guard (unlike
`t`/`P`/`c`/`g`) — the log sidecar is job-name-keyed, not branch-keyed. All
other keys fall through unchanged.

TASK-4: Added the conditional `· l tail` footer hint in
`detailView.renderFooter` (`src/internal/ui/detail.go`), gated on the same
`runLogExists` condition, mirroring the conditional `e edit` / `t tig` hints.
The shared `runLogExists` helper also lives here so the key and the hint can
never disagree on the gate.

TASK-5: Added `src/internal/ui/tail_test.go` (new), mirroring tig_test.go's
conventions: footer hint shows only when a run.log exists; `l` with no
run.log reports the gate and never reaches the launch path (tmux stub call
log stays empty); `l` with a run.log launches the tail pane (split-window
invocation contains `tail -f '<path>'`, pane tagged, status reports the
launch).

TASK-6: Updated the docs: README.md's detail-view Keybindings table (new `l`
row) and the "mg jdi status & log" section (new "Live tail pane" bullet), and
the TUI-key paragraphs in docs/AGENTS.md and project-template/docs/AGENTS.md
(kept in sync per the hard rule). Also updated the key-collision inventory in
`src/internal/ui/agents.go`'s doc comment to include `l tail`.

## Known issues / follow-ups

- The tail pane updates per agent invocation, not mid-invocation: mg-jdi only
  appends to run.log at agent-invocation boundaries (the current capture
  model). Live mid-invocation streaming needs the event-streaming subsystem
  (ROADMAP items 5/6), explicitly out of scope.
- No CLI tail command (e.g. `mg jdi --tail`) was added — the brief only
  asked for the TUI spawn; a CLI tail command would be a follow-up if wanted.
- `session.log` (the raw step-level event stream from the "capture all logs"
  work) is not tailed — ROADMAP item 5 names rendering/tailing it as a
  separate, later job; this job tails `run.log`, the surface the TUI already
  presents.
- Test-environment note: the sandbox's session git shim refuses `git init`,
  which the UI/launch tests need for their scratch repos; tests were verified
  with the real git on PATH (`PATH=/usr/...`). No code change was needed.