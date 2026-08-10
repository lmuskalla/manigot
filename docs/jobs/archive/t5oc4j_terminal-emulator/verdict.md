# Verdict: terminal emulator

id: t5oc4j
status: open
reviewer: opencode-go
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Re-review of the branch after the previous REJECTED verdict (split-window -T
broken on released tmux). All three required changes from that verdict were
addressed; I re-verified the tmux command construction against the actual
tmux source for the released versions in question (3.2a, 3.3a, 3.4), not just
against the stub.

TASK-1: PASS
notes: Scope decisions recorded in tasks.md ("TASK-1 scope decision
(confirmed)"): `tmux split-window -h -p 35` and the "replace" reuse policy,
plus the later "TASK-4 decision (confirmed)" for holdOnFailure. The brief
asked for deliberate answers; both are grounded in the brief's own text,
recorded with rationale, and flagged as assumptions made without a live
human — the same pattern the job's own tasks.md endorsed. Decision-only
task; nothing to verify in code.

TASK-2: PASS
notes: launch.go:396-397 — the tmux branch now constructs `tmux split-window
-h -p 35 -P -F '#{pane_id}' <inner>` and returns "tmux pane". The `-T`
flag is gone from split-window. Verified against released tmux source:
3.2a/3.3a/3.4 cmd-split-window.c args are `"bc:de:fF:hIl:p:Pt:vZ"` — `h`,
`p:` (percentage), `P`, and `F:` are all present, `T` is not (so the prior
verdict's failure mode is genuinely fixed). `-p 35` semantics confirmed from
source: without `-f`, size = 35% of the current pane's size, so the TUI's
pane keeps the majority share as TASK-1 decided. The `-P -F '#{pane_id}'`
output is the new pane's id, captured by launchTmuxPane's cmd.Output(). The
one open item is the task's own risk note — `split-window` with no explicit
`-t` targeting the TUI's own pane via the inherited `$TMUX_PANE` — which is
standard tmux client behavior and is recorded as needing runtime
verification; no code defect found.

TASK-3: PASS
notes: launchTmuxPane (launch.go:170-204) + killPreviousTmuxPane
(launch.go:228-249) implement the replace policy correctly: kill-before-split
ordering, identification via in-memory tmuxLastPaneID (guarded by tmuxMu)
plus the restart-surviving pane-title tag (tmuxPaneTag, set via select-pane
-T — verified present in released tmux 3.2a+ cmd-select-pane.c args
`"DdegLlMmP:RT:t:UZ"`), best-effort kills tolerating an already-closed pane,
`list-panes -s -F` scoped to the current session (flags verified in 3.2a
cmd-list-panes.c args `"asF:f:t:"`), and it can never kill a pane manigot
didn't open (only the recorded id and panes whose title is exactly the tag;
the tag itself is only ever applied by launchTmuxPane to a pane it just
created). Mutex serialization prevents concurrent Agent/Quick interleaving.
Empty split output is surfaced as an error rather than silently recorded.

TASK-4: PASS
notes: holdOnFailure is terminal-agnostic shell; the reasoning that the
`read -r _ignored` hold keeps a failed pane open exactly as it held a window
is sound (tmux runs the pane's shell-command in a pty and destroys the pane
when the command exits). The unconditional replace decision (a mid-hold pane
is killed by a new launch like a live session) is deliberate and recorded in
launch.go's holdOnFailure/killPreviousTmuxPane doc comments and tasks.md's
"TASK-4 decision (confirmed)". No code change needed, matching the task.

TASK-5: PASS
notes: The tmuxStub helper (invocation log, panes file, incrementing pane
ids starting at %100, failure injection via env) plus the 12 new tests cover:
kill-only-tracked/tagged, no-op when nothing to replace, tolerated
already-closed pane, list-panes failure surfaced, kill→split→select-pane
ordering, select-pane tags only the pane just created (never the previous
one), second launch replaces by recorded id, continues after list/select-pane
failure, split failure surfaced, launchDetached tmux routing, and Agent→Quick
sharing one tracked pane. All 29 tests in the package pass (`go test
./internal/launch/` green), `go vet` clean, full module suite green. The
tests assert the corrected command sequence (split without -T, then
select-pane -t <id> -T manigot) and the stub's documented limitation — it
verifies the commands manigot runs, not real tmux effect — is stated in the
test comments, per the task. I additionally source-verified the commands the
tests bless against real tmux, so the stub is not blessing an invalid
command this time.

TASK-6: PASS
notes: Doc comments updated consistently (package spawn-order list, Agent,
Quick, launchDetached, holdOnFailure, buildCmd's numbered list, tmuxPaneTag,
killPreviousTmuxPane, launchTmuxPane) to describe split-pane + replace and
the select-pane tagging mechanism. Historical cross-job TASK references are
preserved as the file's design record. No stale `split-window -T` or
`new-window` claims remain (the few remaining mentions are intentional
comment contexts explaining what is NOT used).

TASK-7: PASS
notes: README.md "Supported platforms" item 1 updated to the split-pane +
replace behavior. docs/AGENTS.md has no tmux mentions (no change needed);
docs/backlog.md's "In-TUI agent terminal (split pane / embedded terminal)"
entry still accurately describes the separate, deferred in-TUI PTY feature
and its claim that launch.go spawns a terminal per session remains broadly
accurate (the replace policy means at most one at a time, but the TUI still
doesn't render sessions — no change needed).

## Security

None — no security findings. All changes are confined to local
process/terminal spawning. killPreviousTmuxPane can only ever kill panes
manigot itself opened (tracked id or panes it tagged); list-panes is scoped
to the current session and kill failures are ignored, so a stale or
user-closed pane is a tolerated no-op, never an error path that could touch
unrelated panes.

## Overall

APPROVED

The prior REJECTED verdict's blockers are all fixed and verified against
released tmux source (3.2a/3.3a/3.4): the split no longer passes the
nonexistent `split-window -T` flag, the title tag is applied afterwards with
`select-pane -t <id> -T manigot` (a flag that exists in every released
version), the tests assert the corrected kill→split→select-pane sequence and
that only the new pane is tagged, and the doc comments describing `-T`
tagging were corrected. The remaining gaps are the ones the job itself
recorded as known issues rather than defects: no real tmux server in this
environment for runtime verification of pane lifecycle (documented in the
tests and implementation.md's Known issues), and the TASK-1 replace policy
adopted with recorded rationale because no live human was available to
confirm. Neither blocks merge.

Known non-blocking follow-ups already recorded by the developer: real-tmux
runtime verification (split targeting via $TMUX_PANE, actual
kill/replace/hold behavior), and the documented edge cases in
killPreviousTmuxPane's comment (agent-overwritten pane title combined with a
TUI restart; list-panes -s session scoping; two TUI instances in one session
sharing the tag namespace).
