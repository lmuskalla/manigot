# Tasks: Add new global setting

id: r5x2a7
status: open
analyst: leomuck@posteo.de
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Scope note

The brief asks for a new global TUI setting: which terminal (emulator) to
call when launching an agent session, mirroring the existing editor setting
(`config.Settings.Editor`, the settings screen's "Editor" field, resolved by
`tui/internal/editor`). "Terminal" here means the mechanism
`tui/internal/launch/launch.go`'s `buildCmd` already implements — the
spawn-order chain tried when the TUI opens `launch.Agent`/`Quick`/`AgentQuick`
(tmux split pane, macOS Terminal.app, then a Linux emulator list). "Keep what
we have now as default" = an unset setting must reproduce today's `buildCmd`
behavior exactly. This job only touches the TUI (`tui/internal/...`) — no
container/`scripts/*.sh` changes, since the terminal choice only affects
where the host spawns a session, not anything that runs inside the container.
`launch.Jdi` (headless, no terminal at all) is unaffected and out of scope,
same as it was for `t5oc4j_terminal-emulator`.

## Open questions from the brief — proposed resolutions

The brief is intentionally short ("which terminal to call... keep what we
have now as default, but let me define a terminal of my choice") and leaves
several implementation details unstated. No interactive human was available
to confirm them (this is an unattended run), so — following the precedent in
`docs/jobs/archive/t5oc4j_terminal-emulator/tasks.md` — proposed resolutions
are recorded here, each grounded in the brief's own wording or an existing
in-repo pattern, and adopted as the working assumption below rather than
guessed silently. They should be treated as revisitable, not final, and are
flagged again in TASK-1.

1. **Where the setting is stored.** Proposed: `config.Settings.Terminal`
   (new field, `json:"terminal,omitempty"`), persisted to
   `config/tui-settings.json` — the same gitignored, personal-preference file
   `Editor` and `RecentActivityCount` already live in — not `manigot/.env`
   (that's for the shared `MANIGOT_PROFILE` default) and not
   `.manigot/manigot.json` (that's committable, project-shared convention;
   which terminal a person's machine has installed is inherently personal).
   This is the direct analog of `Editor`'s own storage, which the brief
   explicitly points to ("similarly to how we can select vim for editing").

2. **Override semantics — does it apply inside tmux too?** Proposed: **yes,
   unconditionally**. When set, the override replaces the *entire*
   `buildCmd` spawn-order chain (tmux split pane, macOS Terminal.app, Linux
   candidate list) — mirroring `editor.Resolve`, where `Settings.Editor`
   (when set) wins over even an explicitly-set `$VISUAL`/`$EDITOR`, not just
   over the no-env-var fallback. This is the simplest, most consistent
   reading of "let me define a terminal of my choice if I want to" as an
   unconditional request. Flagged explicitly: this means a user who both runs
   the TUI inside tmux *and* sets a terminal override loses the tmux
   split-pane/replace behavior (`t5oc4j_terminal-emulator`) in favor of a
   plain new-window spawn of their chosen terminal — a real behavioral
   trade-off the brief doesn't call out either way. Worth a quick explicit
   confirmation before TASK-3 if a human becomes available; adopted as-is
   otherwise since it's the reading most consistent with `Editor`'s existing
   precedent.

3. **Invocation / flag convention for an arbitrary override value.** The
   existing Linux candidate list in `buildCmd` uses two different flag
   conventions to hand off the inner shell command: `--` (gnome-terminal,
   ptyxis) vs `-e` (x-terminal-emulator, konsole, xterm). An arbitrary
   user-supplied terminal name can't be known to need one or the other in
   advance. Proposed: treat the override's first whitespace-separated token
   as the binary name (matching how `Editor`/`$VISUAL`/`$EDITOR` already
   allow trailing args, e.g. `"code --wait"`); if that token case-insensitively
   matches one of the four known `--`-style/`-e`-style names already in
   `buildCmd`'s candidate table, reuse that name's existing convention;
   otherwise default to `-e` (the more common convention among terminals not
   already in the list — kitty, alacritty, wezterm, rxvt, terminator all
   accept it). Look the binary up via `exec.LookPath` exactly like the
   existing candidates, so a typo/missing binary surfaces a clear "launch
   <name>: exec: ... file not found" error instead of silently falling back
   to auto-detect. This is a best-effort convention, not a guarantee for
   every possible terminal emulator — documented as a known limitation
   (TASK-7) rather than solved generally, since a fully general solution
   would need a per-terminal flag field, which the brief didn't ask for.

4. **Settings screen field placement.** Proposed: append "Terminal" as a
   *5th* field, after Profile (tab order: Editor → Base branch → Recent
   activity → Profile → Terminal → wraps to Editor), rather than inserting it
   next to Editor. This is lower-risk than inserting: the four existing focus
   constants (`stFocusEditor`=0 ... `stFocusProfile`=3) keep their values
   unchanged, so existing tests need no renumbering — only `stFieldCount` and
   the wrap targets change, plus the new field's own assertions are additive.
   Grouping it visually next to Editor would read better but isn't worth the
   larger, easier-to-botch diff for a small settings form.

## TASK-1 scope decision (confirmed)

No live human confirmation was available before implementation starts, so
the four proposed resolutions above are adopted as the working design,
since each is directly grounded in the brief's own text or an existing,
already-reviewed in-repo pattern (`Editor`'s override semantics; the
existing candidate table's two flag conventions) rather than an unguided
guess. Point 2 in particular (tmux interaction) and point 3 (the `-e`
fallback for an unrecognized terminal) are flagged again in
`implementation.md`'s "Known issues / follow-ups" as assumptions made
without an explicit human yes/no, per the brief's own spirit of "define a
terminal of my choice" not being fully unambiguous about edge cases.

## Task breakdown

TASK-1: Record and confirm the scope decisions above (storage field,
override-vs-tmux semantics, flag-convention fallback, settings-form field
placement) before other tasks start.
     files: docs/jobs/r5x2a7_add-new-global-setting/tasks.md (this file)
     depends: none
     risk: low — decision/documentation only, but every other task below
            depends on getting this right, especially point 2 (tmux
            interaction), which is a real behavioral trade-off the brief
            doesn't resolve explicitly.
     STATUS: done — see "TASK-1 scope decision (confirmed)" above.

TASK-2: Add a `Terminal` field to `config.Settings` (`json:"terminal,omitempty"`)
with a doc comment matching `Editor`'s, persisted to and loaded from
`config/tui-settings.json` only — no `.env`/`project.Settings` involvement.
Empty (unset) must mean "use today's `buildCmd` auto-detect", matching how
an empty `Editor` means "use `$VISUAL`/`$EDITOR`/fallback".
     files: tui/internal/config/config.go
     depends: TASK-1
     risk: low — additive struct field, directly mirrors the existing
            `Editor` field; `Load`/`Save` already round-trip the struct via
            `encoding/json`, so no bespoke (de)serialization code is needed.

TASK-3: Thread a new `terminal` parameter through
`tui/internal/launch/launch.go`'s `Agent`, `Quick`, `AgentQuick`,
`launchDetached` and `buildCmd`, implementing TASK-1's decisions: when set,
skip the tmux-detection branch and the macOS/Linux auto-detect chain
entirely and invoke the override instead (first token via `exec.LookPath`,
remaining tokens as leading args, then the known-name-vs-`-e`-fallback flag
convention from TASK-1 point 3, then `bash -lc <inner>`); when unset, produce
byte-for-byte the same command construction `buildCmd` produces today. Update
the package/function doc comments (which currently describe only the fixed
spawn order) to describe the override.
     files: tui/internal/launch/launch.go
     depends: TASK-1, TASK-2
     risk: medium — touches the core spawn-order logic shared by every
            launch path; the biggest risk is a regression in the *unset*
            case (today's five spawn paths must be byte-for-byte unchanged),
            closely followed by the `-e`-vs-`--` guess for an override that
            isn't one of the four already-known names not fitting every
            terminal a user might pick.

TASK-4: Update the three call sites in `tui/internal/ui/app.go`
(`launch.Agent`/`launch.Quick`/`launch.AgentQuick`) to pass
`a.settings.Terminal`, matching how `a.settings.ProfileValue()` is already
threaded through today.
     files: tui/internal/ui/app.go
     depends: TASK-2, TASK-3
     risk: low — mechanical parameter threading, same shape as the existing
            profile parameter at each of the three call sites.

TASK-5: Add a "Terminal" field to the settings form
(`tui/internal/ui/settings.go`): a new `textinput.Model` mirroring `editor`'s
construction/width handling, appended as a 5th field per TASK-1 point 4
(`stFocusTerminal` = 4, `stFieldCount` 4→5, wrap targets in `update`/`setFocus`
updated), wired into `newSettingsView`, `update`, `setFocus`, `settingsValue`,
`render` (new row + hint text, e.g. "blank = auto-detect (tmux / Terminal.app
/ gnome-terminal / ...)"), and `hint()`'s focus-aware footer text.
     files: tui/internal/ui/settings.go
     depends: TASK-1, TASK-2
     risk: medium — touches focus-cycling logic shared by every other field;
            an error in the new wrap-around target (profile → terminal →
            editor) would break tab/shift+tab for the whole form, not just
            the new field.

TASK-6: Add/update unit tests:
`tui/internal/config/config_test.go` (Terminal round-trips through
Save/Load, defaults to "" / auto-detect); `tui/internal/launch/launch_test.go`
(buildCmd/launch* honor a set override and bypass tmux/macOS/Linux
auto-detect, an unset override reproduces today's five existing spawn-path
tests unchanged, the known-name-vs-`-e` flag-convention selection, a missing
override binary surfaces a clear error); `tui/internal/ui/settings_test.go`
(new field's seeding, editing, focus cycling including the new wrap targets,
`settingsValue`, `render`).
     files: tui/internal/config/config_test.go,
            tui/internal/launch/launch_test.go,
            tui/internal/ui/settings_test.go
     depends: TASK-2, TASK-3, TASK-5
     risk: medium — the focus-cycle tests in settings_test.go are order-
            sensitive; easy to update the new field's own assertions but
            miss updating an existing wrap-around assertion (e.g. shift+tab
            from Editor now needs to land on Terminal, not Profile).

TASK-7: Update documentation: `docs/AGENTS.md`'s `config/tui-settings.json`
bullet (mention the new terminal override, its storage, and its
default-is-auto-detect fallback wording, matching the existing editor/
recent-activity phrasing) and `README.md`'s "Supported platforms" section
(describe the override and, per TASK-1 point 2, that it also overrides the
tmux split-pane behavior when set); verify `docs/backlog.md`'s "In-TUI agent
terminal" entry doesn't need a change (it describes a different, larger,
still-deferred embedded-PTY feature, not this override).
     files: docs/AGENTS.md, README.md; docs/backlog.md (verify only)
     depends: TASK-3, TASK-5
     risk: low — targeted documentation updates only.

## Suggested sequencing

TASK-1 first (hard gate — in particular, flag point 2's tmux interaction and
point 3's flag-convention fallback for confirmation if a human becomes
available before implementation starts). Then TASK-2, followed by TASK-3 and
TASK-5 in parallel (both depend only on TASK-1/TASK-2 and touch different
files), then TASK-4, with TASK-6 written alongside TASK-3/TASK-5 as they land
rather than after. TASK-7 last, once behavior is final.
