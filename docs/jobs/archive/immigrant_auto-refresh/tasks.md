# Tasks: auto refresh

id: immigrant
status: open
analyst: deepseek-v4-flash
date: 2026-08-27

<!-- Produced by @analyst from brief.md. -->

## Context

`mg tui` (`src/internal/ui/app.go`) re-reads state from disk only on
*discrete events*: `ctrl+r` (the full `refresh()` — job list, project
settings, open detail view's four files + log tab + computed diff tab,
and the git-log strip), returning to the list (`esc`/`backspace` →
`refreshJobs()` only), and the lifecycle message handlers (`doneMsg`,
`deleteMsg`, `commitAllMsg`, `mergeMsg`, editor return). Nothing re-reads
on its own, so a TUI left sitting in a tmux pane shows a stale job list,
stale jdi badges, and a stale log/diff tab until the user presses
`ctrl+r`. The brief asks for an auto-refresh timer: every 1s, falling
back to ~5s if that is not performance-feasible.

Facts that shape the design:

1. The app currently has exactly two timer-driven messages —
   `spinnerTickMsg` (the activity indicator, which runs *only while an
   mg-jdi run is active* and self-terminates) and `statusExpireMsg`
   (status blink/expiry, *only while a status is set* and
   self-terminates). The code docs (spinnerTickMsg's doc in
   `app.go`, `pollJDIBell`'s doc, the README's "mg jdi status & log"
   section) explicitly call the spinner "the one narrow exception to no
   separate timer-driven tick". An auto-refresh is the app's first
   **permanent** timer — alive the whole time the TUI is open — so those
   "idle = no timer" claims become false and the docs/comments must be
   updated (TASK-5).
2. `refresh()` runs synchronously on the UI goroutine: `job.Discover`
   (an `os.ReadDir` + one `brief.md` read per job), `git.CurrentBranch`,
   `git.RecentCommits`, per-job `job.ReadJDIStatus` (pollJDIBell), plus —
   with a detail view open — `detail.reload()` (4 file reads, the run.log
   tail, and the computed diff tab, which spawns two git subprocesses)
   and `detail.refreshCommits` (another git subprocess). At a 1s cadence
   that is 3+ git subprocess spawns per second on the UI thread. The
   repo has a documented history of TUI-lag fixes (the archived
   `fg089d_tui-is-laggy` job; the deferred `loadTabs`/`syncViewerSize`
   eager-render fixes in `detail.go`), so the brief's "if that's
   performance-wise not feasible" clause is a real concern, not
   hand-waving — TASK-3 owns that decision.
3. `detail.reload()` is scroll-safe: `loadTab` → `Viewer.SetContent` →
   `rebuild` → `clamp()` preserves the scroll offset (it only clamps out
   of range, never resets). An auto-refresh will not yank the user's
   reading position.
4. State safety: `refresh()` is only ever invoked today from `stateList`
   and `stateDetail` (its doc notes "ctrl+r isn't routed here while the
   form is open"). The auto-refresh tick must skip the form/overlay
   states — `stateNewJob`, `stateSettings`, `stateAgents`,
   `stateConfirm`, `stateGitPanel` — where reloading the list or the
   detail view under an open form/picker/confirmation would be
   disruptive. The chain itself should keep ticking and just skip the
   work, so a single chain lives for the TUI's lifetime.
5. The auto-refresh must be **silent**: no `setStatus("refreshed")` — a
   footer status every second would blink constantly and drown out real
   feedback. `refresh()` itself sets no status (callers do), so the tick
   handler just calls it bare. The jdi stop-bell (`pollJDIBell`) rides
   along for free, which is exactly the "came back to the TUI and want to
   see what happened" outcome the brief wants.

Open question, deliberately not expanded into a task: the brief does not
ask for a user-facing interval setting, so the interval stays a named
constant (per the brief's "every 1s … or every 5s or so") rather than a
new field in `config/tui-settings.json` / the settings form. If a
reviewer wants it configurable later, that is a separate job.

Branch note: brief.md declares `branch: feature/immigrant_auto-refresh`;
this session has no shell to run `git branch --show-current`, but the
environment contract is that the mounted workspace is the job's own
worktree, always on the job branch — no checkout needed.

## Task breakdown

TASK-1: Add the auto-refresh timer infrastructure to `internal/ui/app.go`:
a new `autoRefreshMsg` type, an `autoRefreshTicking bool` guard field on
`App` (mirroring `spinnerTicking` — no duplicate concurrent chains), an
`autoRefreshCmd() tea.Cmd` returning `tea.Tick(autoRefreshInterval, …)`,
and wiring in `Init()` via `tea.Batch` alongside the existing
`startSpinnerIfRunning`/`armStatusExpiry` so the chain starts with the
program. Add the `autoRefreshInterval` constant (1s per the brief) to
`activity.go` next to the other timer constants (`activityInterval`,
`statusBlinkInterval`). Unlike the spinner/status chains, this one does
**not** self-terminate — the tick handler always re-arms.
  - files: `src/internal/ui/app.go`, `src/internal/ui/activity.go`
  - depends: none
  - risk: medium — the app's first permanent timer; the guard must
    prevent a second chain from starting (the same hazard `spinnerTicking`
    guards against), and `Init()`'s returned cmd set changes (check no
    test pins `Init()` returning nil in the idle state).

TASK-2: Handle `autoRefreshMsg` in `App.Update`: run `a.refresh()` (the
same full refresh `ctrl+r` uses — list + project settings + open detail
view + git-log strip) when `a.state` is `stateList` or `stateDetail`,
and do nothing (but still re-arm the timer) in the form/overlay states
`stateNewJob`, `stateSettings`, `stateAgents`, `stateConfirm`,
`stateGitPanel`. Never set a status line from this handler (silent
refresh — unlike the `ctrl+r` handler's "refreshed · N job(s)"). Always
return `a.autoRefreshCmd()` so the chain continues regardless of state.
  - files: `src/internal/ui/app.go`
  - depends: TASK-1
  - risk: medium — the handler is a new case in the root `Update` switch
    every other message routes through; the state gating must be exact so
    an open settings/new-job form or a confirm/git-panel overlay is never
    reloaded underneath, and the silent-refresh constraint must not leak
    a status (which would fight the status-expiry chain and blink).

TASK-3: Performance verification and the 1s-vs-5s decision the brief
explicitly leaves open. Run the TUI at the 1s cadence with a job open on
the diff/log tabs (the expensive paths: diff tab = 2 git subprocesses per
reload, plus `job.Discover` + 2 more git calls per tick) and confirm no
input lag / visible stutter while typing and scrolling; if 1s is not
feasible, flip `autoRefreshInterval` to 5s. Only if a measured problem
demands it, consider the smallest safe optimization (e.g. skip the
diff-tab recompute when the branch's commits are unchanged) — do not
refactor the refresh machinery generally; the fallback constant is the
brief-sanctioned lever.
  - files: `src/internal/ui/activity.go` (constant flip if needed),
    possibly `src/internal/ui/detail.go` (only if a measured problem
    demands it)
  - depends: TASK-1, TASK-2
  - risk: medium — every tick runs 3+ synchronous git subprocesses plus
    glamour markdown re-renders on the UI goroutine; this repo has a
    documented history of exactly this class of lag (archived
    `fg089d_tui-is-laggy`, the deferred-render fixes in `detail.go`), so
    the measurement must be honest rather than assumed.

TASK-4: Tests for the auto-refresh tick, in a new
`src/internal/ui/autorefresh_test.go` (patterns established by
`refresh_test.go`/`spinner_test.go` — construct an `App` and drive
messages through `Update`): the tick triggers a full refresh (job list
re-read, open detail view reloaded — e.g. a file edited out-of-band
appears without any keypress); the tick is silent (no status set, no
status-expiry arming from it); the guard prevents a second concurrent
chain; the chain re-arms from every state (tick in a form state returns
the next-tick cmd but performs no refresh); the interval constant's value
is pinned (1s, or 5s if TASK-3 flipped it).
  - files: `src/internal/ui/autorefresh_test.go` (new), plus any existing
    test that pins `Init()`'s exact cmd behavior
  - depends: TASK-1, TASK-2 (and the TASK-3 outcome for the pinned value)
  - risk: low — pure Go tests over the existing `App` model; no new
    subsystems.

TASK-5: Docs and comment sync for the now-false "no timer-driven tick"
claims, plus the refresh story. Update the in-code docs in `app.go`
(`spinnerTickMsg`'s "the app's only timer-driven message — the one narrow
exception", `pollJDIBell`'s "every 'poll tick' this app has (ctrl+r,
returning to list, a checkout, etc.)", `refresh()`'s "used by ctrl+r")
to include the auto-refresh tick; update the README's `### Keybindings`
`ctrl+r` rows ("refresh — re-read job files from disk" → mention the
automatic refresh), the "mg jdi status & log" section's "no separate
live-streaming subsystem … one exception" wording, and the diff-tab
"recomputed on every `ctrl+r`" wording; update `docs/AGENTS.md`'s TUI
architecture paragraph; and per the hard rule keep `agents/*.md` and
`project-template/docs/AGENTS.md` in sync with `docs/AGENTS.md`.
  - files: `src/internal/ui/app.go` (comments only), `README.md`,
    `docs/AGENTS.md`, `agents/*.md`, `project-template/docs/AGENTS.md`
  - depends: TASK-1, TASK-2, TASK-3 (docs must describe the shipped
    interval and behavior)
  - risk: low — prose/comment-only, but a deliberate multi-file sync pass
    so the "one exception" claims don't silently rot.