# Verdict: Launch agents without workflow

id: 8g06st
status: reviewed
reviewer: @reviewer
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `tui/internal/ui/app.go` `agentForKey` now iterates the fixed
`agentOrder` list instead of `job.Stage().Agents()`; any of the five agent
keys resolves regardless of stage. Verified with `TestAgentForKeyIgnoresStage`
(all three stages × all five keys) and confirmed by reading the code — no
`job.Stage()` call remains in the dispatch path.

TASK-2: PASS
notes: `tui/internal/ui/detail.go` `renderActionBar` always renders all five
buttons from the new `agentOrder` (`tui/internal/ui/agents.go`), in a fixed
order, and keeps `stage: <name>` as a label only. Matches the "informational
hint, no longer a gate" spec from Q3.

TASK-3: PASS
notes: `Stage.Agents()` and `TestStageAgents` were removed
(`tui/internal/job/stage.go` / `stage_test.go`); `Stage()` itself and the
stage constants are kept for the label. This is a documented, deliberate
decision (per Q3), not a silent removal — the doc comment on `Stage` explains
why and warns against reintroducing it as a gate. `grep` confirms no other
caller of `.Agents()` was left dangling.

TASK-4: PASS
notes: `tui/internal/ui/agents_test.go` (new) covers
`TestAgentForKeyIgnoresStage` (5 keys × 3 stages), `TestAgentForKeyUnknownKey`,
`TestAgentForKeyNoDetail`, and `TestRenderActionBarAlwaysShowsAllAgents`
(stage label present + all five labels present, across all three stages).

TASK-5: PASS
notes: `hostcmd.DoneCommand` in `tui/internal/hostcmd/hostcmd.go` mirrors
`NewJob`'s `resolve.Resolve` + `cmd.Dir`/`cmd.Env["PWD"]` pattern exactly, and
passes the job's exact directory name (`job.Name`), matching
`finish-job.sh`'s "exact match first" resolution (verified by reading
`scripts/finish-job.sh` lines 47-50). Unlike `NewJob` it correctly returns the
unstarted `*exec.Cmd` rather than running it, since the caller needs
`tea.ExecProcess`.

TASK-6: PASS
notes: `updateDetail` in `tui/internal/ui/app.go` adds a `"D"` case (capital,
confirmed no collision with existing `d`/lowercase agent key or any other
detail-view binding — checked `agentMeta`, `detailView.update`, and
`fileTab.scroll`) that calls `doneCmd`, which builds the command via
`hostcmd.DoneCommand` and wraps it in `tea.ExecProcess`, same
suspend/resume mechanism as `editCmd`. Resolution failures surface through
`cmdErrorText` before any process is started.

TASK-7: PASS
notes: the `doneMsg` case in `Update` always calls `refreshJobs()`, clears
`a.detail`, and returns to `stateList` regardless of `msg.err`, exactly per
Q2's reasoning (re-reading from disk shows true state; exit code is not
trustworthy given `finish-job.sh`'s decline-paths also exit 0). A non-zero
error is still surfaced via `cmdErrorText` into `a.status` first. Covered by
`TestDoneMsgSuccessReturnsToList`, `TestDoneMsgDeclinedStillReturnsToList`,
and `TestDoneMsgErrorSurfacesAndReturnsToList` in the new
`donemsg_test.go`.

TASK-8: PASS
notes: `renderActionBar` appends a `│`-separated `[D] Done` button styled with
`statusDoneStyle` (pre-existing style, reused not added) instead of
`accentStyle`, visually distinct from the agent buttons. `renderFooter`'s hint
now includes "D mark done". README's keybindings table was updated to match.

TASK-9: PASS
notes: `hostcmd_test.go` adds `TestDoneCommandUnresolvable` (surfaces a "not
found" error, mirroring `TestNewJobUnresolvable`) and
`TestDoneCommandBuildsResolvedCommand` (stub binary via `SAFECODE_DONE_BIN`,
verifies absolute-path invocation, cwd, `$PWD`, and the exact job-name arg —
no real repo touched). `donemsg_test.go` adds the `App`-level clean/declined/
error cases plus `TestDoneCmdResolutionFailureSurfacesNotFoundError`. All new
and existing tests pass (`go test ./...`, `go vet ./...`, `gofmt -l .` clean).

## Security

No security-relevant findings beyond what's already flagged as an accepted,
documented risk in `tasks.md` (TASK-6/TASK-9's own risk notes): the `D` key
triggers a real squash-merge + branch-delete + directory-move git flow. That
risk is mitigated as designed — `finish-job.sh`'s own interactive
confirmations (unmodified, out of scope per the brief) still gate the
irreversible steps, and the TUI only ever invokes the resolved host script
(`resolve.Done()`), never re-implements the git operations itself. No new
attack surface (no shell interpolation of user input — `exec.Command` is used
with argv, not a shell string; `jobName` passed as `job.Name`, not free user
text). `SAFECODE_DONE_BIN` test isolation correctly prevents any test run
from touching a real repo.

## Overall

APPROVED

All 9 tasks are implemented as specified in `tasks.md`, with no scope creep:
the git diff is limited to the two features described in the brief
(stage-gating removal, mark-done wiring) plus directly-related test and doc
updates. `Stage.Agents()`'s removal (TASK-3) was an explicit, documented
decision rather than an unflagged deletion, as tasks.md required. Commit
history is one commit per task (TASK-1/2 and TASK-6/7 combined, which
tasks.md itself calls for — "land together" / same dependent pair), plus
separate `implementation.md` and scaffold commits, in the correct
`[8g06st] TASK-N: ...` format. `go build`, `go vet`, `go test ./...`, and
`gofmt -l .` are all clean on the branch. No blockers.
