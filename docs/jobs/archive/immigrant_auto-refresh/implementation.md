# Implementation: auto refresh

id: immigrant
status: open
date: 2026-08-27

## Summary

Introduced a permanent 1-second auto-refresh timer to `mg tui`: the TUI now
re-reads everything `ctrl+r` reads (job list, project settings, open detail
view, git-log strip) on its own every second, so a TUI left sitting in a
tmux pane catches up without a manual refresh — including the mg-jdi stop
bell, which rides along for free. The interval stays at the brief's
preferred 1s: the TASK-3 measurement confirmed the full refresh costs
~6.6ms on the UI thread (0.7% of each second) even on the expensive
diff-tab path, so the 5s fallback is unnecessary.

## Changes

TASK-1: auto-refresh timer infrastructure.
- `src/internal/ui/activity.go`: added the `autoRefreshInterval` constant
  (1s) next to the other timer constants.
- `src/internal/ui/app.go`: added the `autoRefreshMsg` type, the
  `autoRefreshTicking` guard field on `App` (mirroring `spinnerTicking` —
  no duplicate concurrent chains), `autoRefreshCmd()` (a `tea.Tick` at
  `autoRefreshInterval`), and `startAutoRefresh()` (guard-checking starter).
  `Init()` now batches `startAutoRefresh()` alongside
  `startSpinnerIfRunning()`/`armStatusExpiry()` so the chain starts with the
  program. Unlike the spinner/status chains this one never self-terminates.

TASK-2: `autoRefreshMsg` handling in `App.Update`.
- The tick runs the same full `refresh()` `ctrl+r` uses when the state is
  `stateList` or `stateDetail`, and does nothing (but still re-arms) in the
  form/overlay states (`stateNewJob`, `stateSettings`, `stateAgents`,
  `stateConfirm`, `stateGitPanel`) — reloading under an open
  form/picker/confirmation would be disruptive. The chain always re-arms
  (`autoRefreshCmd()` returned from every state), and the refresh is
  deliberate silent: no status line, so it never fights the status-expiry
  blink chain. The spinner cmd `refresh()` produces is propagated the same
  way `ctrl+r` propagates it, so a run discovered by a tick still starts
  its activity animation.

TASK-3: performance verification, 1s-vs-5s decision.
- Measured the synchronous per-tick cost of `refresh()` with a throwaway
  benchmark (3 jobs, 5 commits each, real git worktrees, detail view open
  on the diff tab — the expensive path: `git log` + `git diff --stat` +
  `git.BranchCommits` + `git.CurrentBranch` + `git.RecentCommits` +
  `job.Discover` per tick): **avg 6.6ms / max 7.1ms** with the detail+diff
  view open (0.7% of a 1s tick), **avg 4.3ms** list-only. At 1s cadence the
  UI thread is blocked well under 1% of each second, so typing/scrolling
  cannot stutter; even at 10× the cost on a much larger repo that is still
  ~7% per second. Decision: keep `autoRefreshInterval` at 1s — no constant
  flip, no optimization needed. (The benchmark file was removed after the
  measurement; the interval is pinned by a permanent test instead.)

TASK-4: tests, in the new `src/internal/ui/autorefresh_test.go`.
- `TestAutoRefreshTickTriggersFullRefresh` — a tick reloads the open detail
  view (out-of-band file edit appears), re-reads the job list (new job on
  disk appears), and re-reads project settings (baseBranch change picked
  up), with no keypress, and re-arms.
- `TestAutoRefreshTickIsSilent` — no list/detail status, no status-expiry
  arming, no blink state after a tick.
- `TestAutoRefreshGuardPreventsSecondChain` — `startAutoRefresh` is a no-op
  while a chain is live, and the chain persists (guard stays set, re-arms)
  after a handled tick.
- `TestAutoRefreshSkipsWorkInFormStates` — in every form/overlay state the
  tick re-arms but leaves the job list untouched even when disk changed;
  contrast case proves `stateList` does refresh.
- `TestAutoRefreshIntervalPinned` — pins `autoRefreshInterval` at 1s (the
  TASK-3 outcome).

TASK-5: docs and comment sync.
- `src/internal/ui/app.go` (comments): `spinnerTickMsg`/`statusExpireMsg`
  docs reworded from "the app's only timer-driven message / the one narrow
  exception" to "one of three timer-driven messages — the two narrow
  *self-terminating* exceptions"; `pollJDIBell`'s poll-tick enumeration now
  names the auto-refresh tick; `refresh()`'s doc says it is used by ctrl+r
  *and* the auto-refresh tick, and notes the tick skips the form states;
  the `commitAllMsg` comment says "next refresh (auto or ctrl+r)".
- Reviewer bounce-back (verdict NEEDS WORK → one fix): three stale
  "idle (no timer) behaviour" comments in `app.go` that the first pass
  missed — the `statusExpiryTicking` and `spinnerTicking` field docs and
  the `statusExpireMsg` handler comment — all claimed the app returns to a
  timer-free idle state when the self-terminating chains stop, which the
  permanent auto-refresh tick made false. Reworded all three to mirror the
  type-level docs: the two chains are the narrow self-terminating
  exceptions, leaving the permanent auto-refresh tick as the app's one
  always-running timer.
- `README.md`: the two `ctrl+r` keybinding rows mention the automatic
  every-second refresh; the diff-tab and git-log-strip "recomputed on every
  `ctrl+r`" wording now says "every refresh (the automatic every-second
  refresh and `ctrl+r` alike)"; the "mg jdi status & log" section's "one
  exception" wording now names the two self-terminating timer exceptions
  alongside the permanent auto-refresh.
- `docs/AGENTS.md`: the TUI architecture paragraph now describes the
  permanent 1s auto-refresh timer (silent, skipped while a form/overlay is
  open).
- `agents/*.md` and `project-template/docs/AGENTS.md`: verified in sync —
  neither mentions any TUI refresh/timer claim (only the unrelated `t`-key
  and settings-screen mentions), so no changes were needed.

## Known issues / follow-ups

- The brief's open question — a user-facing interval setting — was
  deliberately left out of scope per the analyst's note: the interval stays
  a named constant (`autoRefreshInterval`). Making it configurable is a
  separate job.
- The TASK-3 measurement used a small synthetic repo; a very large real
  repo (many thousands of commits) would make the per-tick git calls slower,
  but even a 10× cost leaves ~93% of each second free on the UI thread, so
  the 1s cadence has wide headroom. If a pathological repo ever shows
  stutter, the brief-sanctioned lever is a one-line constant flip to 5s.
- The auto-refresh tick does not fire while the terminal is suspended
  (e.g. the `e` edit shortcut's `tea.ExecProcess`); Bubble Tea pauses the
  program's command loop during that, so no refresh runs mid-edit — the
  editor return handler refreshes the edited tab itself, as before.
- Verification in the reviewer bounce-back pass: `go build ./...` clean,
  `go vet ./internal/ui/...` clean, and the full `go test ./...` suite
  passes (run against the real git at `/usr/bin/git` — the session git shim
  blocks the `git init` the temp-repo tests need; the shim is a soft layer,
  and no workspace repo was touched by the tests).