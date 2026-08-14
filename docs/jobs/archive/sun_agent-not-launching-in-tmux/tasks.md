# Tasks: agent not launching in tmux

id: sun
status: open
analyst: deepseek-v4-flash
date: 2026-08-14

<!-- Produced by @analyst from brief.md. -->

## Scope note

Root-cause analysis: the user reports that after switching to a more elaborate
tmux setup and to Kitty, launching an agent from the TUI "opens in a new
window" instead of launching in tmux. The only code path that can spawn a
*Kitty* window is `buildOverrideCmd` in `internal/launch/launch.go` — kitty is
not in `terminalCandidates` (gnome-terminal, ptyxis, x-terminal-emulator,
konsole, xterm), so the auto-detect chain can never open Kitty. That means the
user has `config.Settings.Terminal` set to "kitty" (the setting added by
`r5x2a7_add-new-global-setting`), and per that job's documented point-2
decision a set override bypasses the *entire* spawn chain unconditionally —
including the tmux split-pane path added by `t5oc4j_terminal-emulator`.
Inside tmux with the override set, `launchDetached` (guarded by
`terminal == "" && $TMUX != ""`) and `buildCmd` (override branch checked
first) therefore open a new Kitty window instead of a tmux pane — exactly the
reported symptom.

The fix reverses r5x2a7's point-2 decision, which was explicitly flagged there
as "worth a quick explicit confirmation ... adopted as-is otherwise": this
brief is that confirmation. When the TUI is inside tmux ($TMUX set + a tmux
binary on PATH), the tmux split-pane path wins; the Terminal override applies
only when the TUI is NOT inside tmux. This matches the title "agent not
launching in tmux" — the agent should launch in tmux.

Out of scope, deliberately unchanged: the tmux split-window construction
itself (argv, `-h -l 35%`, the replace policy, pane tagging), `launch.Jdi`
(no terminal at all), the macOS Terminal.app / Linux emulator fallback paths,
and the container/entrypoint. Only the precedence between the tmux branch and
the override branch changes.

## Open questions — proposed resolutions

1. **Is the user's TUI actually running inside tmux?** Assumed yes: the brief
   says the user "switched to a more elaborate tmux setup", and the complaint
   is specifically that the agent does not launch *in tmux*. Adopted as the
   working assumption; flagged revisitable.
2. **Does the user actually have a Terminal override set to kitty?** The
   code-level evidence says yes — it is the only way a Kitty window can open
   (see Scope note). Even if the user's exact config differs, the fix (tmux
   wins inside tmux) is correct for the stated complaint either way.
3. **Should the override still be honored inside tmux in any scenario?**
   Proposed: no. Inside tmux the tmux pane is always used; the override
   applies only outside tmux. This reverses r5x2a7's point 2, which was the
   deliberate-but-unconfirmed decision this brief now corrects. Keeping the
   override winning inside tmux is the behavior the user already has and is
   the reported bug.
4. **Could the real situation instead be "TUI not inside tmux, user wants
   launches to enter an existing tmux session from outside"?** That was
   explicitly out of scope for t5oc4j, and it cannot produce a Kitty window in
   today's code (kitty is not a candidate), so it is disfavored. Recorded as a
   residual risk: if the fix lands and the user still reports a new window,
   revisit with that interpretation in mind.

## Task breakdown

TASK-1: Record and confirm the scope decisions above (root cause: the Terminal
override bypassing the tmux branch; fix: tmux wins inside tmux, the override
applies outside tmux only) before other tasks start.
     files: docs/jobs/sun_agent-not-launching-in-tmux/tasks.md (this file)
     depends: none
     risk: low — decision/documentation only, but every other task depends on
            the precedence reversal being the agreed design; it reverses the
            documented r5x2a7 point-2 behavior.
     STATUS: done — see "Scope note" and "Open questions" above.

TASK-2: Change the launch precedence in `internal/launch/launch.go`: in
`launchDetached` and `buildCmd`, check the tmux branch (inside tmux + tmux
binary on PATH) BEFORE the `terminal != ""` override branch, so inside tmux
every launch (Agent/Quick/AgentQuick) goes to the tmux split pane regardless
of `config.Settings.Terminal`, and the override still applies when the TUI is
not inside tmux (the outside-tmux + override and inside-tmux + no-override
cases must be byte-for-byte unchanged). Update the affected doc comments —
the package doc's spawn order, `Agent`/`Quick`/`AgentQuick`,
`launchDetached`, `buildOverrideCmd`'s "takes over the entire spawn order"
claim, `config.Settings.Terminal` in `internal/config/config.go`, and the
settings form's Terminal hint in `internal/ui/settings.go` — to describe the
new precedence (e.g. "when inside tmux, always a tmux split pane; otherwise
this terminal, else auto-detect").
     files: internal/launch/launch.go, internal/config/config.go,
            internal/ui/settings.go
     depends: TASK-1
     risk: medium — reverses a deliberately documented decision; the riskiest
            edges are a regression in the outside-tmux + override case (the
            override must still win there) and the inside-tmux +
            no-override case (unchanged behavior).

TASK-3: Update the two tests that assert the old override-bypasses-tmux
semantics — `TestBuildCmdOverrideBypassesTmux` and
`TestLaunchDetachedOverrideBypassesTmuxPane` in
`internal/launch/launch_test.go` — to assert the new tmux-wins-inside-tmux
behavior (override set + $TMUX set + tmux on PATH → `tmux split-window ...`,
desc `"tmux pane"`, the kill/split/tag sequence runs, and the override is not
invoked). Add new tests: buildCmd/launchDetached with an override set but NOT
inside tmux still invokes the override (guards the outside-tmux precedence
from TASK-2); override set + $TMUX set but tmux binary missing falls through
to the override. Run `go test ./...` to confirm nothing else regresses.
     files: internal/launch/launch_test.go
     depends: TASK-2
     risk: medium — the two flipped tests are easy to mis-update, and the new
            outside-tmux override tests are the guard against silently
            breaking the r5x2a7 feature for non-tmux users.

TASK-4: Update documentation to match the new precedence: `README.md`'s
"Supported platforms" section (currently: the override "overrides that whole
spawn order unconditionally — including the tmux split-pane behavior ... even
when the TUI itself is running inside tmux") and `docs/AGENTS.md`'s
`config/tui-settings.json` bullet (note that inside tmux the split pane wins
regardless of the setting); sync `project-template/docs/AGENTS.md` if the
AGENTS.md wording changes. Verify `docs/backlog.md` / `docs/ROADMAP.md`'s
"In-TUI embedded terminal" entry needs no change (it is the separate, larger,
still-deferred feature).
     files: README.md, docs/AGENTS.md, project-template/docs/AGENTS.md
     depends: TASK-2
     risk: low — targeted wording updates; the only trap is leaving the README
            describing the old unconditional-override semantics.

## Suggested sequencing

TASK-1 first (hard gate — the Scope note is the design the rest implements).
Then TASK-2 (precedence + comment updates), with TASK-3 written alongside it
as the flipped tests land, and TASK-4 last once behavior is final.
`go test ./...` must stay green throughout.
