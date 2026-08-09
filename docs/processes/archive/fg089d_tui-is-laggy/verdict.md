# Verdict: tui is laggy

id: fg089d
status: reviewed
reviewer: @reviewer
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: Investigation only, as scoped (files: none). The hypothesis (glamour's
`WithAutoStyle()` → `termenv.HasDarkBackground()` blocking-reads raw stdin,
racing Bubble Tea's own reader, triggered on nearly every keypress via
`setStatus` → `syncViewerSize`) is sound and matches the code at
`tui/internal/markdown/markdown.go` (pre-fix) and
`tui/internal/ui/detail.go`'s `setStatus`. Folded into the TASK-2 commit
(`ad927d2`) rather than getting its own commit — reasonable given it produced
no code change, but see commit-discipline note below.

TASK-2: PASS
notes: `tui/internal/markdown/markdown.go` — `rendererFor` now caches one
`*glamour.TermRenderer` per wrap width behind a mutex; `Render` no longer
rebuilds (and re-triggers the stdin probe) on every call. Cache is correctly
keyed by width so a resize doesn't get stale wrapping. Covered by
`TestRendererReusedPerWidth` and `TestRendererCacheKeyedByWidth` in
`markdown_test.go`. Minor unbounded-growth note below (not a blocker).

TASK-3: PASS
notes: `tui/internal/ui/detail.go` — `syncViewerSize` now resizes only the
active tab immediately and marks the other three `stale`;
`ensureCurrentSized` (called from `render()`) lazily resizes a tab the moment
it becomes active. Verified the call site: `render()` calls
`ensureCurrentSized()` before drawing the active viewer, and tab-switch keys
(`update()`) only change `d.cur`, so the next `render()` correctly catches up
the newly active tab. `reload()`/`loadTabs()` (ctrl+r, agent-edit refresh)
still eagerly re-renders all four tabs regardless of `stale` — that's fine,
it's a separate, much-less-frequent path and was explicitly out of scope here
(covered by TASK-4's reasoning). Covered by
`TestDetailDefersResizeForInactiveTabs`.

TASK-4: PASS
notes: Investigation only (files: none in tasks.md); developer added an
inline doc comment on `App.refresh()` explaining why it's not a lag source
and why an async `tea.Cmd` wasn't pursued. Matches the "only pursue if
TASK-1/3 don't fully resolve" condition in tasks.md — TASK-1/3 do resolve it,
so not pursuing is correct per the task's own conditional.

TASK-5: PASS
notes: Investigation only, as scoped. Correctly identifies that none of the
five spawn paths in `buildCmd` keep the window open on exit (tmux has no
`remain-on-exit`; the four Linux emulators close by default). Also correctly
notes Terminal.app isn't actually broken by this mechanism but applies the
fix there too since it's harmless — reasonable call given the platform was
never confirmed (brief's open question was never answered, and TASK-8
documents that this couldn't be resolved). Folded into the TASK-6 commit
(`8390ad4`); see commit-discipline note.

TASK-6: PASS
notes: `tui/internal/launch/launch.go` — `holdOnFailure` wraps the inner
shell command; on non-zero exit it prints a status line and blocks on `read`
before propagating the original exit code via `exit "$ec"`. Applied
uniformly across all five spawn paths via the shared `inner` string
(`shellCommand`), which is simpler and more robustly testable than the
per-terminal-mechanism approach tasks.md floated (tmux `remain-on-exit`,
`xterm -hold`) — a reasonable, documented deviation within scope, not a new
behavior beyond what was asked. Manually verified the wrapped script under
plain `bash`: non-zero exit prints the message and holds, zero exit runs
through silently with the correct exit code preserved in both cases. Covered
by `TestShellCommandFormat` and `TestHoldOnFailureExitsCleanlyOnSuccess` in
`launch_test.go`. The five `buildCmd` spawn paths themselves remain untested
beyond argv construction, as tasks.md's risk note anticipated (no GUI
terminal/tmux in CI).

TASK-7: PASS
notes: `tui/internal/launch/launch.go` — reviewed and documented inline why a
launcher-binary-itself failure (started successfully, then fails on its own,
e.g. no display server) is intentionally left unaddressed: safely surfacing
it would require either `cmd.Wait()` (unsafe for xterm, whose process *is*
the window and wouldn't return until the session ends — this would
reintroduce a UI freeze) or a `tea.Msg` back-channel (out of proportion for
this job). This is a legitimate "decide and document" outcome per the task's
own wording ("Decide whether to surface anything here...").

TASK-8: PASS
notes: `go test ./...` (run from `tui/`) passes, confirmed independently
during this review — all packages green, no regressions. `gofmt -l .` is
clean. The pty-based keystroke-timing check and the direct
`bash`-under-`holdOnFailure` check described in implementation.md are
reasonable substitutes for the real GUI-terminal/tmux paths, which are
correctly flagged as unexercisable in this environment (no display, no tmux
session, none of the five terminal binaries installed) — matches tasks.md's
own risk note. No dedicated commit for this task (folded into the
`implementation` commit); see commit-discipline note.

## Scope

`git diff main...HEAD` touches exactly the files named across
TASK-2/3/4/6/7 (`tui/internal/markdown/markdown.go`,
`tui/internal/markdown/markdown_test.go`, `tui/internal/ui/detail.go`,
`tui/internal/ui/detail_test.go`, `tui/internal/ui/app.go`,
`tui/internal/launch/launch.go`, `tui/internal/launch/launch_test.go`) plus
the four job docs. No unrelated refactors, no drive-by changes. Clean.

## Commit discipline (non-blocking notes)

- TASK-1 and TASK-5 are investigation-only per tasks.md (`files: none`) and
  were folded into the TASK-2 (`ad927d2`) and TASK-6 (`8390ad4`) commits
  respectively, rather than getting standalone commits. Given they produced
  no diff of their own, this is a defensible reading of "each task has its
  own commit," but it is a deviation from the literal rule — flagging for
  awareness, not blocking.
- TASK-8 (manual verification pass) has no commit of its own; its record
  lives only in `implementation.md`'s commit (`88156ca`). Same reasoning as
  above — no files changed, nothing to commit standalone.
- Not a blocker either way since the mapping from commit → task is still
  traceable (commit messages name both task numbers, e.g. "TASK-5/TASK-6:
  keep the launch window open on a failed agent launch"), and
  `implementation.md` explains the pairing. Worth a heads-up for future jobs
  to either give every task its own (possibly empty/doc-only) commit or state
  the pairing convention explicitly in AGENTS.md.

## Non-blocking follow-ups (not required before merge)

- `rendererCache` in `markdown.go` grows unbounded, keyed by every distinct
  wrap width seen (e.g. across window resizes). In practice this is a small,
  bounded set of terminal widths per session, not a real leak, but a future
  job could cap or evict it if it ever becomes relevant.
- The brief's "Why"/"Out of scope" and the platform the lag was observed on
  were never filled in, and the fix for TASK-6 could not be exercised against
  a real terminal/tmux session in this environment (correctly disclosed in
  both tasks.md and implementation.md). Real interactive confirmation on the
  user's actual setup is still worth doing, but this doesn't block merging a
  well-reasoned, uniformly-applied, low-risk fix.

## Security

None — no new external input handling, no new attacksurface. Shell-quoting
of `holdOnFailure`'s wrapper string is fixed/static (no user-controlled
interpolation added beyond what `shellQuote` already handled for
`safecodePath`/`agent`/`jobID`).

## Overall

APPROVED

Both root causes are correctly diagnosed and fixed with appropriately scoped,
low-risk changes; new regression tests exist for both fixes and the full
suite passes with no regressions; scope is clean (no unrelated changes). No
blockers. See "Commit discipline" and "Non-blocking follow-ups" above for
optional cleanup, none of which needs to happen before merge.
