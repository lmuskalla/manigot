# Implementation: Add new global setting

id: r5x2a7
developer: unattended run (no human available)
date: 2026-08-11

## Summary

Added a new global TUI setting, "Terminal" — the command used to spawn an
agent session's terminal window/pane — mirroring the existing "Editor"
setting. It lives in `config.Settings.Terminal`, persisted to the personal,
gitignored `config/tui-settings.json` alongside `Editor`/`RecentActivityCount`.
When unset (the default), every launch path reproduces today's fixed
auto-detect spawn order unchanged (tmux split pane → macOS Terminal.app →
Linux emulator list). When set, it overrides that entire spawn order
unconditionally, including the tmux branch, and invokes the chosen terminal
directly instead.

## Changes

TASK-1: No code change — the analyst's scope-decision write-up in `tasks.md`
(storage field, tmux-override semantics, flag-convention fallback,
settings-form field placement) is recorded in its own `[r5x2a7] TASK-1: ...`
commit, adopted as-is; see that file's "TASK-1 scope decision (confirmed)"
section. (Correction from an earlier revision of this note: the write-up was
originally committed together with TASK-2's code change rather than in its
own commit; per the reviewer's first-pass feedback, the branch's history was
rebuilt — via cherry-picks onto a temp branch, verified byte-for-byte
tree-identical to the pre-split tip at every step, with the original tip kept
under a local `backup-r5x2a7-presplit` branch/reflog until the split was
confirmed safe — so tasks.md's TASK-1 content and config.go's TASK-2 change
now each have their own commit, matching every other task in this job.)

TASK-2: Added `Settings.Terminal string \`json:"terminal,omitempty"\`` to
`tui/internal/config/config.go`, with a doc comment matching `Editor`'s.
`Load`/`Save` already round-trip the whole struct via `encoding/json`, so no
new (de)serialization code was needed — the field just rides along.

TASK-3: Threaded a new `terminal` parameter through
`tui/internal/launch/launch.go`'s `Agent`, `Quick`, `AgentQuick`,
`launchDetached` and `buildCmd` (each gained a trailing `terminal string`
parameter). Added `buildOverrideCmd`, which implements the override: the
override value's first whitespace token is the binary (`exec.LookPath`'d
exactly like the existing auto-detect candidates, so a missing/typo'd binary
surfaces a clear "launch \<name\>: exec: ... file not found" error instead of
silently falling back to auto-detect), remaining tokens are passed as leading
args (mirroring `Editor`'s own "trailing args" allowance, e.g. "code --wait"),
and the flag used to hand off the inner shell command reuses the known `--`/
`-e` convention when the binary case-insensitively matches one of the four
already-known emulator names, otherwise defaults to `-e`. Extracted the
Linux-candidate table (name → flag convention) into a package-level
`terminalCandidates` var so both the auto-detect chain and the override's
known-name lookup share one table instead of duplicating it.
`launchDetached` now skips the tmux-detection branch entirely when `terminal`
is non-empty, per TASK-1 point 2 (the override wins even inside tmux). The
unset (`terminal == ""`) path is unchanged byte-for-byte — verified by the
pre-existing spawn-path tests continuing to pass unmodified except for the
added trailing `""` argument at each call site.

TASK-4: Updated the three `launch.Agent`/`launch.Quick`/`launch.AgentQuick`
call sites in `tui/internal/ui/app.go` to pass `a.settings.Terminal`,
matching how `a.settings.ProfileValue()` is already threaded through.

TASK-5: Added a "Terminal" field to the settings form
(`tui/internal/ui/settings.go`): a new `textinput.Model` mirroring `editor`'s
construction/width handling, appended as a 5th field (`stFocusTerminal = 4`,
`stFieldCount` 4→5) after Profile in tab order (Editor → Base branch →
Recent activity → Profile → Terminal → wraps to Editor), per TASK-1 point 4.
Wired into `newSettingsView`, `update`, `settingsValue` (value trimmed like
`Editor`), `render` (new row + "blank = auto-detect (tmux / Terminal.app /
gnome-terminal / ...)" hint), and `hint()`'s focus-aware footer text.
`setFocus` was refactored to unconditionally blur all four text inputs before
focusing the target one, rather than repeating a per-case blur list — this
removed the risk of a missed blur when adding the new field and keeps future
field additions lower-risk.

TASK-6: Added/updated unit tests:
- `tui/internal/config/config_test.go`: `Terminal` round-trips through
  Save/Load, and defaults to `""` (auto-detect) when unset.
- `tui/internal/launch/launch_test.go`: updated existing call sites for the
  new trailing parameters (`buildCmd`, `launchDetached`, `Agent`, `Quick`);
  added tests that an override bypasses tmux even when `$TMUX` is set and a
  `tmux` binary is present (`buildCmd` and `launchDetached` levels), that a
  known-name override reuses its existing flag convention (case-insensitive),
  that an unknown-name override defaults to `-e`, that leading args in the
  override value are passed through, and that a missing override binary
  returns a clear error naming it.
- `tui/internal/ui/settings_test.go`: updated the tab/shift-tab cycle tests
  for the new 5-field wrap order; added tests for the Terminal field's
  seeding, editing (with focus routed there via 4 tabs), value trimming, and
  its presence in `render()`.

All packages build (`go build ./...`), vet cleanly (`go vet ./...`), and the
full test suite passes (`go test ./...`).

TASK-7: Updated documentation:
- `docs/AGENTS.md`'s `config/tui-settings.json` bullet now mentions the
  `terminal` field, its default (today's auto-detect spawn order), and that
  setting it overrides the whole spawn order unconditionally including tmux.
- `README.md`'s "Supported platforms" section gained a paragraph describing
  the Terminal setting and its tmux-override behavior, right after the
  existing three-step auto-detect list.
- `docs/backlog.md`'s "In-TUI agent terminal" entry was verified, not
  changed: it describes a different, much larger, still-deferred
  embedded-PTY feature (rendering the agent session inside the TUI itself),
  not this override.

## Known issues / follow-ups

- **Tmux interaction (TASK-1 point 2).** Setting a terminal override while
  also running the TUI inside tmux replaces the split-pane behavior entirely
  with a plain spawn of the chosen terminal — the user loses the "stays in
  the TUI's own window" convenience `t5oc4j_terminal-emulator` added. This
  was adopted as the simplest, most consistent reading of "let me define a
  terminal of my choice... if I want to" (an unconditional override,
  mirroring `Editor`'s own precedent over `$VISUAL`/`$EDITOR`), but it's a
  real trade-off the brief didn't explicitly resolve. No live human was
  available to confirm before implementation started; worth a quick
  confirmation if this surfaces as a real complaint.
- **`-e` fallback for an unrecognized terminal (TASK-1 point 3).** An
  override whose binary isn't one of the four known names (gnome-terminal,
  ptyxis, x-terminal-emulator, konsole, xterm — plus the reused xterm/konsole
  `-e` convention) is assumed to accept `-e` for "run this shell command".
  This is a best-effort convention, not a guarantee for every possible
  terminal emulator a user might have installed — a terminal that needs a
  different flag (or none at all) will fail with a `exec:` error from the
  spawned process, not a clean "unsupported terminal" message. A fully
  general solution would need a per-terminal flag field, which the brief
  didn't ask for; documented as a known limitation rather than solved.
- No other scope changes or surprises came up; all seven tasks from
  `tasks.md` were completed as scoped.

## Re-review fixes (post first verdict)

`verdict.md`'s first pass ("NEEDS WORK") flagged two small issues, both
addressed:

1. **TASK-1 commit split.** TASK-1's `tasks.md` write-up had landed in the
   same commit as TASK-2's `config.go` change (`faec77a`). The branch history
   was rebuilt so each now has its own commit (see the corrected TASK-1 note
   above) — content is byte-for-byte identical to before the split, only the
   commit boundaries changed.
2. **gofmt.** `gofmt -w tui/internal/ui/settings.go` fixed the struct-field
   alignment `gofmt -l` flagged after the `terminal` field was added;
   `gofmt -l` now reports nothing. No functional change; `go build`/`go
   vet`/`go test ./...` all still pass.
