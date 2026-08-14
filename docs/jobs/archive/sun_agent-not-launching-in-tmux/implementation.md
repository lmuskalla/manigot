# Implementation: agent not launching in tmux

## Summary

Fixed the reported bug where launching an agent from the TUI inside tmux opened
a new Kitty window instead of a tmux split pane. Root cause: the
`config.Settings.Terminal` override (`"kitty"`) bypassed the *entire* spawn
order unconditionally — including the tmux split-pane branch — so a TUI
running inside tmux with an override set spawned a separate Kitty window.

The fix reverses that precedence: inside tmux (with a tmux binary on PATH),
the tmux split-pane branch now always wins, regardless of the Terminal
setting. The override applies only when the TUI is NOT inside tmux. All other
spawn behavior (the tmux split-window construction, replace policy, macOS
Terminal.app / Linux emulator fallbacks, `launch.Jdi`) is unchanged.

## Changes

TASK-1: Recorded and confirmed the scope decisions in `tasks.md`: root cause
(the override bypassing the tmux branch is the only code path that can open a
Kitty window), the design (tmux wins inside tmux; the override applies outside
tmux only), and the open-question resolutions. Committed as the analyst's
produced tasks file.

TASK-2: Changed the launch precedence in `internal/launch/launch.go`:
- `launchDetached` now checks the tmux branch (`$TMUX` set + tmux binary on
  PATH) *before* consulting `terminal`, so inside tmux every launch
  (Agent/Quick/AgentQuick) routes to `launchTmuxPane` regardless of the
  override.
- `buildCmd` now checks the tmux branch first, then the override, then
  macOS Terminal.app, then the Linux emulator chain (renumbered 1–4).
- Updated the doc comments describing the old semantics: the package doc's
  spawn order, `Agent`'s override paragraph, `launchDetached`'s
  "takes over unconditionally, bypassing the tmux-detection branch" claim,
  `buildOverrideCmd`'s "takes over the entire spawn order" claim, and the
  now-stale "branch 3" reference in `terminalCandidates`.
- `internal/config/config.go`: `Settings.Terminal` doc now states the setting
  applies only when the TUI is not inside tmux.
- `internal/ui/settings.go`: the Terminal row's comment and rendered hint now
  note "inside tmux the split pane always wins".

TASK-3: Updated `internal/launch/launch_test.go`:
- Flipped `TestBuildCmdOverrideBypassesTmux` → `TestBuildCmdTmuxWinsOverOverrideInsideTmux`
  and `TestLaunchDetachedOverrideBypassesTmuxPane` →
  `TestLaunchDetachedTmuxWinsOverOverrideInsideTmux`: override + $TMUX + tmux
  on PATH now yields the tmux split-window command, desc "tmux pane", the
  kill/split/tag sequence runs, and the override is not invoked.
- Added `TestBuildCmdOverrideWinsOutsideTmux` and
  `TestLaunchDetachedOverrideWinsOutsideTmux` (override still wins with $TMUX
  unset — guards the r5x2a7 feature for non-tmux users) and
  `TestBuildCmdOverrideFallsThroughWhenTmuxMissing` /
  `TestLaunchDetachedOverrideFallsThroughWhenTmuxMissing` (override used when
  tmux is missing from PATH despite $TMUX being set).
- `go test ./...` passes (run with the real git on PATH — the container's
  git shim refuses `git init`, which the git-dependent UI tests need; this is
  an environment artifact, unrelated to these changes).

TASK-4: Updated documentation to match the new precedence:
- `README.md` "Supported platforms": the spawn order now lists the override
  as step 2, explicitly applying only when the TUI is NOT inside tmux, and the
  override paragraph no longer claims it wins inside tmux.
- `docs/AGENTS.md` `config/tui-settings.json` bullet: notes that inside tmux
  the launch is always a tmux split pane regardless of the setting.
- `project-template/docs/AGENTS.md`: no change needed (it is a fresh-project
  template with no tui-settings bullet).
- `docs/backlog.md` / `docs/ROADMAP.md`: no change needed — `docs/backlog.md`
  no longer exists (its in-TUI terminal entry moved to ROADMAP item 6, the
  separate, larger, still-deferred "In-TUI embedded terminal" feature, which
  is unaffected).

## Known issues / follow-ups

- Residual risk recorded in tasks.md (open question 4): if the user's real
  situation was instead "TUI not inside tmux, user wants launches to enter an
  existing tmux session from outside", the fix lands but the complaint could
  persist. That interpretation cannot produce a Kitty window in today's code,
  so it is disfavored but flagged revisitable.
- The git-dependent UI tests (`internal/ui`) require `git init` in temp
  repos, which the session git shim blocks inside a container session; they
  pass with the real git on PATH. Pre-existing environment artifact, not
  introduced here.
