# Implementation: tig in tui

id: term
status: open
developer: @developer
date: 2026-08-16

## Summary

Made tig reachable from a job's detail page in the TUI: the new `t` key opens
the job's branch diff in tig on the host, spawned exactly like an agent launch
(a tmux split pane when the TUI is inside tmux, falling back to the configured
terminal / Terminal.app / a Linux terminal emulator), by reusing the existing
CLI path `mg diff <job> --tig`. "if available" is implemented as an
availability gate: a new `launch.TigAvailable()` probe is cached on the detail
view at open time and gates both the footer hint (`· t tig`) and the key
itself, and `launch.Tig` re-checks availability as an authoritative backstop
so a stale cached gate surfaces a synchronous error instead of a doomed pane.

## Changes

TASK-1: added the tig availability query to `internal/launch/launch.go`: an
exported package-level `var TigLookPath = exec.LookPath` (the test seam,
mirroring `ExeOverride`/`JdiExe` and `cmd/mg`'s unexported `tigLookPath`) and
`func TigAvailable() bool` returning whether tig resolves on the host.

TASK-2: added `Tig(jobID, projectRoot, terminal string) (string, error)` and a
`tigShellCommand(manigotPath, jobID, projectRoot string) string` helper to
`internal/launch/launch.go`. The inner command is
`cd '<projectRoot>' && '<manigot>' diff '<jobID>' --tig` (cd-first,
shellQuote-everything, no `--profile`/`--agent`/`--job` — `mg diff` is a host
git command, not a session launch), wrapped in `holdOnFailure` and spawned via
the existing `launchDetached`, so the tmux split-pane / replace-policy /
terminal-override behavior comes for free. `Tig` checks `TigAvailable()` first
and returns the not-installed error synchronously, then resolves the mg binary
via `ExeOverride` exactly like the other launch paths, and returns the same
short "where it opened" description.

TASK-3: pinned the new launch path in `internal/launch/launch_test.go`:
`tigShellCommand` format (cd + `diff <name> --tig`, holdOnFailure wrap,
quote/space escaping, no session flags), `TigAvailable` true/false via a
stubbed `TigLookPath`, `Tig`'s tig-missing error path (proving the
availability check runs before `ExeOverride` is consulted), `Tig`'s
`ExeOverride` resolution-failure path, and a `Tig` success path through the
existing `tmuxStub` (desc "tmux pane", split-window invoked with the tig inner
command, pane tagged/recorded).

TASK-4: added a `tigAvailable bool` field to `detailView` in
`internal/ui/detail.go`, set in `newDetailView` via `launch.TigAvailable()`
(cached for the view's lifetime, re-checked on each job open), and extended
`renderFooter`'s hint with `· t tig` only when `tigAvailable` AND
`d.job.Branch != ""` — mirroring the conditional `e edit` hint.

TASK-5: added the `"t"` case to `updateDetail` in `internal/ui/app.go` (before
the agentForKey fallthrough): not-installed status when `!tigAvailable`, the
same "no branch known for this job" guard the `P` key uses for a branch-less
job, otherwise `launch.Tig(a.detail.job.Name, a.root, a.settings.Terminal)`
reporting `"→ tig in " + desc` or `cmdErrorText(err)`. Also updated the
`agentMeta` doc comment's list of non-colliding detail-view bindings to
include `t` (`internal/ui/agents.go`).

TASK-6: added `internal/ui/tig_test.go` covering the hint and the key: footer
shows `t tig` when available + branch set and omits it when unavailable or
branch-less (stubbing `launch.TigLookPath`), `t` when unavailable reports the
not-installed status without consulting `ExeOverride`, `t` on a branch-less
job reports "no branch known", `t` with an `ExeOverride` resolution failure
surfaces the error (proving routing to `launch.Tig`), and the success path
lands `→ tig in tmux pane` using a locally replicated tmux stub on PATH +
`$TMUX` (the codebase's self-contained test-helper convention), asserting the
tmux call log contains the `diff <name> --tig` inner command.

TASK-7: documentation — added the `t` row to the README's detail-view
Keybindings table, and matching sentences in the TUI section of
`docs/AGENTS.md` and `project-template/docs/AGENTS.md` (the hard-rule sync
pair; `/workspace/AGENTS.md` is the same inode as `docs/AGENTS.md`, so it
picks the change up automatically).

## Known issues / follow-ups

none
