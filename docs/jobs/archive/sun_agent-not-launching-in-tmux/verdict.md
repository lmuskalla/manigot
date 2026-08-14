# Verdict: agent not launching in tmux

id: sun
status: open
reviewer: deepseek-v4-flash
date: 2026-08-14

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed the full `main...HEAD` diff (commit list: e689b4f TASK-1, 71471ca
TASK-2, 891e614 TASK-3, cfa7dac TASK-4, c76ea91 implementation) against
`tasks.md`, plus the surrounding unchanged code and doc files.

TASK-1: PASS
notes: Scope note + open questions with adopted resolutions recorded in
`tasks.md` before any code changed; commit `[sun] TASK-1` touches tasks.md
only.

TASK-2: PASS
notes: `internal/launch/launch.go` — `launchDetached` (lines 343-350) now
checks the tmux branch ($TMUX set + tmux on PATH) before consulting
`terminal`, and `buildCmd` (lines 552-597) is reordered tmux → override →
macOS Terminal.app → Linux emulators (renumbered 1-4). Traced all four
behavioral quadrants against the pre-change code: outside-tmux + override →
override wins (unchanged); inside-tmux + no-override → tmux pane (unchanged);
inside-tmux + override + tmux on PATH → tmux pane (the fix); inside-tmux +
override + tmux missing → override (unchanged, just one redundant LookPath).
The two "byte-for-byte unchanged" cases required by TASK-2 hold. All doc
comments the task lists were updated: package spawn order, `Agent`,
`launchDetached`, `buildOverrideCmd` ("takes over the auto-detect spawn
order"), the stale "branch 3" reference in `terminalCandidates` (now 4),
`internal/config/config.go` `Settings.Terminal`, and the Terminal hint in
`internal/ui/settings.go` (line 392; `TestSettingsRender` only asserts the
"auto-detect" substring, still present). Callers (`internal/ui/app.go` lines
682/805/920) just thread `settings.Terminal` through, so the precedence
change is fully contained in the launch package.

TASK-3: PASS
notes: `internal/launch/launch_test.go` — the two old tests are flipped to
the new semantics and 4 new tests added (buildCmd/launchDetached ×
override-wins-outside-tmux, falls-through-when-tmux-missing). Manually traced
each against the implementation: `newTmuxStub` sets both PATH and $TMUX, so
`TestLaunchDetachedTmuxWinsOverOverrideInsideTmux` exercises the full
kill/split/tag sequence and asserts kitty is never invoked;
`TestLaunchDetachedOverrideWinsOutsideTmux` and
`TestLaunchDetachedOverrideFallsThroughWhenTmuxMissing` correctly expect
desc "kitty" with `tmuxLastPaneID` empty. Caveat: `go test` could not be
executed in this review session (bash is restricted to git read/commit
commands here), so test/build verification is by manual trace only — no
inconsistency found. The implementation.md note that the git-dependent
`internal/ui` tests need a real `git init` (the container git shim refuses
it) is consistent with this environment and pre-existing.

TASK-4: PASS
notes: `README.md` "Supported platforms" now lists the Terminal override as
step 2 applying only outside tmux, with the tmux step 1 documented as always
winning inside tmux. `docs/AGENTS.md` `config/tui-settings.json` bullet
updated. Verified `project-template/docs/AGENTS.md` has no tui-settings
bullet (no change needed, as claimed), `docs/backlog.md` no longer exists,
and `docs/ROADMAP.md` item 6 ("In-TUI embedded terminal") contains no
tmux/override precedence content (no change needed). Also verified the root
`docs/AGENTS.md`/`AGENTS.md` context files reflect the new wording.

Commit discipline: PASS — one commit per task in `[sun] TASK-N: description`
format with the files each task specifies; `[sun] implementation: add
summary` committed separately; working tree clean; branch
`feature/sun_agent-not-launching-in-tmux` matches the brief.

Scope: PASS — the diff touches only the files named in tasks.md (launch.go,
launch_test.go, config.go, settings.go, README.md, docs/AGENTS.md) plus the
job's own scaffold files. No unrelated refactors.

## Security

No security findings. The change only reorders launch-path precedence between
the tmux split-pane branch and the user's explicit terminal override; no new
command construction, no new env/exec surface. The shell-command quoting and
`holdOnFailure` wrap are untouched.

## Overall

APPROVED

All four tasks are implemented as specified and the precedence reversal
matches the design recorded in tasks.md. No blockers. Non-blocking caveats:
(1) test/build execution was verified by manual trace only, since this
review session's shell is restricted to git commands; (2) `launch.go:327`
"branch 3/4 below" is terse but accurate.
