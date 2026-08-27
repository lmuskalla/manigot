# Verdict: auto refresh

id: immigrant
status: open
reviewer: deepseek-v4-flash
date: 2026-08-27

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Re-review after the NEEDS WORK bounce-back. The three stale comments flagged
in the previous verdict were fixed in commit `a5185d7`; this verdict covers
the full branch state at HEAD (`0c01150`).

TASK-1: PASS
notes: `autoRefreshMsg` type, `autoRefreshTicking` guard field, `autoRefreshCmd()`, `startAutoRefresh()`, and the `autoRefreshInterval` (1s) constant in `activity.go` are all present and wired as specified. `Init()` (src/internal/ui/app.go:275-277) batches `startAutoRefresh()` alongside `startSpinnerIfRunning()`/`armStatusExpiry()`; `tea.Batch` drops the nils, so the idle-state cmd set grows by exactly the one permanent chain. The guard is set once and never cleared — correct, since the chain never self-terminates and every tick handler path re-arms. No test pins `Init()`'s cmd set (verified: no `.Init(` call in any ui test).

TASK-2: PASS
notes: Handler at src/internal/ui/app.go:530-554. State gating is exact — full `refresh()` in `stateList`/`stateDetail`, skip-but-re-arm in the other five states via the default branch; the `appState` enum has exactly those seven values, so every state is covered and no form/picker/confirmation/git-panel overlay is ever reloaded underneath. The tick is silent (no `setStatus`, so it never fights the status-expiry blink chain), always returns `autoRefreshCmd()`, and propagates the spinner cmd `refresh()` produced through `tea.Batch` (nil-safe — Batch drops nils) — matching ctrl+r's discovery behavior. `stateDetail` with a nil `detail` is unreachable (state transitions always pair the two), and `refresh()` guards `a.detail != nil` anyway.

TASK-3: PASS
notes: The 1s cadence is kept, per the brief's preference. The measurement is documented (avg 6.6ms / max 7.1ms with a detail view open on the diff tab, 4.3ms list-only) and is consistent with the per-tick code path (`job.Discover`, `git.CurrentBranch`, `git.RecentCommits`, `detail.reload` → diff tab's two git subprocesses, `detail.refreshCommits`). The benchmark was removed and the interval is permanently pinned by `TestAutoRefreshIntervalPinned`. Caveat: this session's git shim permits only git read/commit commands, so I could not execute the benchmark or test suite — verification is static; nothing in the code contradicts the documented figures, and the 5s fallback remains a one-line constant flip as the brief-sanctioned lever.

TASK-4: PASS
notes: `src/internal/ui/autorefresh_test.go` covers all five required behaviors: tick triggers the full refresh (out-of-band brief edit appears in the open detail view, a new job on disk appears, a `baseBranch` change is picked up, chain re-arms), tick is silent (no list/detail status, no status-expiry arming, no blink state), the guard prevents a second chain and survives a handled tick, form states re-arm without refreshing (with a stateList contrast case proving the refresh does run there), and the interval is pinned at 1s. Statically sound: `newDetailView(j, w, h)` matches the signature at detail.go:128; `tabs[0].viewer`/`status`/`statusUntil`/`statusBlinkOn`/`statusExpiryTicking`/`detail.status` all exist; `job.Discover` falls back to `discoverWorkingTree` for the non-repo temp-dir roots (discover.go:101-105), so the setups are valid; branchless jobs short-circuit `loadDiff`/`refreshCommits` before any git call. Could not execute them (shim), but they follow the established `refresh_test.go`/`spinner_test.go` patterns.

TASK-5: PASS
notes: All previously flagged "idle (no timer)" comments are fixed — the `statusExpiryTicking` field doc (app.go:102-109), the `spinnerTicking` field doc (app.go:143-151), and the `statusExpireMsg` handler comment (app.go:499-503) now all describe the two chains as the narrow self-terminating exceptions leaving the permanent auto-refresh tick as the app's one always-running timer (commit `a5185d7`). Type-level docs (`spinnerTickMsg`, `statusExpireMsg`, `autoRefreshMsg`), `pollJDIBell`'s poll-tick enumeration, `refresh()`'s doc, the `commitAllMsg` comment, the README keybinding rows / "mg jdi status & log" / diff-tab / git-log-strip wording, and `docs/AGENTS.md`'s TUI paragraph are all updated and accurate. A repo-wide grep finds no remaining "only timer-driven / no separate timer / idle (no timer)" claims outside the intentional "two narrow exceptions" phrasings. `agents/*.md` and `project-template/docs/AGENTS.md` verified to contain no refresh/timer claims — no changes needed there, as documented.

## Security

Not run (no security-sensitive surface in this change: a Bubble Tea timer, a refresh call, tests, and prose).

## Overall

APPROVED

No changes required. TASK-1 through TASK-5 are complete and correct; the previous review's blockers are resolved.