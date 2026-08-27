# Tasks: introduce log tailing in tui

id: normal
status: open
analyst: @analyst
date: 2026-08-27

<!-- Produced by @analyst from brief.md. -->

## Problem as understood

Add a way, from the TUI's job detail view, to spawn a tmux split pane (like
agent launches / tig already do, via `internal/launch`'s `launchDetached` /
`launchTmuxPane`) that tails the logs of the currently running mg-jdi run.

**Shortcut check (the brief's explicit ask):** `t` is NOT available in the
detail view — it is already bound to tig (`updateDetail`'s `"t"` case, gated on
`detailView.tigAvailable` + the job having a branch, with a "· t tig" footer
hint). The free single-letter candidates in the detail view are `l` (recommended
— mnemonic for "log", unused in this state; the list view's `l` opens a job but
that is a separate view state with per-state key routing) and `T` (shift+t, less
discoverable). Every other detail-view binding: tab/shift+tab/left/right, 1-6
(brief/tasks/implementation/verdict/log/diff), `e`, `D`, `j`, `x`/`del`, `P`,
`c`, `g`, `ctrl+r`, `esc`/`backspace`, `q`, the agent keys `o`/`a`/`d`/`r`/`s`,
and the viewer scroll keys.

**What to tail — two candidate log files exist for a job's mg-jdi run:**
1. `.manigot/jdi-status/<job-name>/run.log` (`job.JDIRunLogPath`) — mg-jdi's
   formatted, human-readable per-invocation transcript; this is exactly what the
   detail view's existing "log" tab (key `5`, `job.ReadJDIRunLogTail`) reads.
   Stable path from the project root, tied to the job name, removed with the job.
2. `<job-worktree>/docs/jobs/<id>_<slug>/session.log` — the raw step-level event
   stream captured by the recent "capture all logs from non-interactive agent
   sessions" work. ROADMAP.md's item 5 explicitly names "rendering/tailing
   `session.log`" as a separate, later job — which this job may or may not be.

Both files are appended only at agent-invocation boundaries
(`logInvocation`/`appendSessionLog` run after each `runner.Run` returns in
`cmd/mg/jdi.go`), so a live `tail -f` updates per invocation, not mid-invocation
— inherent to the current capture model, not something this job can change
without the event-streaming subsystem (ROADMAP items 5/6, out of scope).

**Recommended scope decision:** tail `run.log` (option 1) — it is the log
surface the TUI already presents, and its path is stable and job-name-keyed.
`session.log` is the flagged alternative; the developer should confirm with the
brief author only if they read the brief as specifically wanting the raw
capture.

**Mechanics:** a new `launch.Tail(logPath, terminal)` following the `launch.Tig`
shape but with no mg-binary hop — the inner command is a plain
`tail -f '<absolute log path>'` (shellQuote'd), spawned through the existing
detached/tmux paths. One deliberate deviation to confirm: the agent/tig launch
paths wrap their inner command in `holdOnFailure`, but a user ends `tail -f`
with Ctrl+C (exit 130, non-zero) — the hold would then leave a "press enter to
close" prompt every single time. Recommend NOT wrapping the tail command; flag
for the developer.

**Gating:** gate the key and footer hint on "a run.log exists for this job"
(the same condition under which the log tab shows real content — os.Stat on
`job.JDIRunLogPath`). `tail -f` idles harmlessly once a run ends. The stricter
"only while a run is live" gate (`job.ReadJDIStatus` == `JDIRunning`) matches
the brief's "currently running agent" wording more literally — decision point
for the developer, recommendation is the file-exists gate.

## Task breakdown

TASK-1: Add `launch.Tail(logPath, terminal string) (string, error)` to
`internal/launch` — builds the `tail -f '<logPath>'` inner command (absolute
path, shellQuote'd, deliberately NOT wrapped in holdOnFailure so a user's
Ctrl+C closes the pane instead of holding it open) and spawns it via the
existing `launchDetached` path (tmux split pane with the replace policy inside
tmux; terminal override / auto-detect chain outside), returning the short
"where it opened" description like `Tig`.
     files: src/internal/launch/launch.go
     depends: none
     risk: low — a pure addition following the established Tig/Agent launch
     pattern; no existing behavior touched.

TASK-2: Add launch-package tests for the Tail inner command (exact
`tail -f '<path>'` format, quoting of a path containing spaces) and the spawn
path (tmux stub: split-window invocation + select-pane tagging), mirroring the
existing launch_test.go conventions for the other shell-command builders.
     files: src/internal/launch/launch_test.go
     depends: TASK-1
     risk: low — mirrors established test patterns for command builders.

TASK-3: Wire the `l` key in `App.updateDetail` (`internal/ui/app.go`): gate on
a run.log existing for this job (os.Stat on `job.JDIRunLogPath(root,
job.Name)`), call `launch.Tail`, surface `"→ tailing run.log in <desc>"` on
success or `cmdErrorText` on failure; all other keys fall through to the
existing default handling unchanged. No branch guard (unlike `t`/`P`/`c`/`g`)
— the log sidecar is job-name-keyed, not branch-keyed.
     files: src/internal/ui/app.go
     depends: TASK-1
     risk: low-medium — a new key case in the detail dispatcher; `l` collides
     with no existing detail-view binding (verified against updateDetail,
     detail.go's update/scroll, and agentMeta), and the gate + status handling
     follow the existing `t`-key pattern.

TASK-4: Add the `· l tail` footer hint in `detailView.renderFooter`
(`internal/ui/detail.go`), gated on the same run.log-exists condition, mirroring
how the `e edit` and `t tig` hints are conditional.
     files: src/internal/ui/detail.go
     depends: TASK-3
     risk: low — a conditional hint string in the existing footer builder;
     existing footer tests assert substrings only, so an addition is safe
     (re-verify against detail_test.go's footer assertions when landing).

TASK-5: Add TUI tests (new `internal/ui/tail_test.go` mirroring tig_test.go's
tmux-stub pattern): footer hint shows only when a run.log exists; `l` with no
run.log reports the gate and never reaches the launch path (tmux stub call log
stays empty); `l` with a run.log launches the tail pane (split-window
invocation contains `tail -f '<path>'`, pane tagged, status reports the
launch).
     files: src/internal/ui/tail_test.go (new)
     depends: TASK-3, TASK-4
     risk: low — follows the established tig_test.go conventions (writeTmuxStub,
     addJobWorktree/gitInitRepo job setup).

TASK-6: Update the docs: README.md's detail-view Keybindings table (new `l`
row) and the "mg jdi status & log" section (mention the live tail pane), plus
the TUI-key paragraphs in docs/AGENTS.md and project-template/docs/AGENTS.md
(which document the `t` tig key and must stay in sync per the hard rule).
     files: README.md, docs/AGENTS.md, project-template/docs/AGENTS.md
     depends: TASK-3 (final key choice)
     risk: low — doc-only; keep phrasing consistent with the existing `t`-tig
     entries.

## Explicitly out of scope

- No new CLI command (e.g. `mg jdi --tail` or `mg tail`); the brief only asks
  for the TUI spawn. A CLI tail command would be a follow-up if wanted.
- No list-view binding; the brief scopes this to the job detail view.
- No mid-invocation live streaming of agent output — that needs the
  event-streaming subsystem (ROADMAP items 5/6).
- No in-TUI rendering/tailing of `session.log` (the ROADMAP pending item)
  unless the developer reads the brief as specifically wanting the raw capture
  rather than `run.log` — then it becomes in scope and this plan's TASK-1/3/4/5
  should target `session.log`'s path instead.

## Suggested order

TASK-1 → TASK-2; TASK-3 (needs TASK-1) → TASK-4 (needs TASK-3); TASK-5 after
TASK-3/TASK-4; TASK-6 last (needs the final key from TASK-3).

## Note for the developer

At analysis time the job worktree's git registration could not be verified from
inside the sandbox (no git binary available in this read-only session), but the
worktree gitdir name matches the job exactly, so `git status`/`git log` from
`/workspace` should work normally — re-check before relying on per-task
commits.

Open decisions to confirm while implementing:
1. Key: `l` (recommended) vs `T`. `t` is taken by tig — the brief's first
   guess is unavailable.
2. Log file: `run.log` (recommended, matches the log tab) vs `session.log`
   (the ROADMAP-pending raw capture). If the brief author intended
   `session.log`, the path is `<job-worktree>/docs/jobs/<id>_<slug>/session.log`
   and the gate becomes a stat on that file instead.
3. Gate: file-exists (recommended) vs live-run-only (JDIRunning sidecar).
4. holdOnFailure: recommended NOT to wrap the tail command (Ctrl+C is the
   normal exit for a tail pane and is non-zero); the agent/tig convention wraps
   — confirm the desired UX either way.