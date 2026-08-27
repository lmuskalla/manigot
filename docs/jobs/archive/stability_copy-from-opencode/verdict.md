# Verdict: copy from opencode

id: stability
status: open
reviewer: deepseek-v4-flash
date: 2026-08-25

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: internal/session/docker.go — `terminalEnvArgs(tool string)` now skips
TMUX/TMUX_PANE when `tool == config.ToolOpenCode` (via the new `tmuxEnvVars`
map gate, lines 75-78, 86-95); the single call site passes `info.Tool`
(line 367), which is `config.ToolOpenCode` for zai / opencode-go /
opencode-zen / legacy `--tool opencode` (session.go ResolveProfile), so every
OpenCode docker session is covered. The Claude Code path is byte-identical:
default claude-pro resolves `ToolClaudeCode` and the existing
`TestBuildTerminalEnvForwarding` (claude-pro) still asserts TMUX/TMUX_PANE
are forwarded. The comment block above `terminalEnvVars` was rewritten and
now documents the exception and its reason; the old "OpenCode additionally
switches to its tmux DCS-passthrough OSC 52 form when TMUX is set" claim (the
bug) is gone. Mechanism cross-checked against the archived job's ground truth
(experiment_cannot-copy-from-agents: opencode-ai 1.18.18 clipboard.ts wraps
OSC 52 in `ESC P tmux;` passthrough when `process.env.TMUX` is set) and tmux's
documented `allow-passthrough` default (off, tmux >= 3.1) — the passthrough is
discarded before any set-clipboard handling, so stripping TMUX so OpenCode
emits plain OSC 52 is the correct fix. `warnTmuxClipboard` correctly left
untouched (read-only, host-side, its set-clipboard-on advice is now accurate
again).

TASK-2: PASS
notes: internal/session/docker_test.go — `TestBuildTerminalEnvForwarding`
kept as-is; new `TestBuildTerminalEnvForwardingOpenCodeStripsTmux` resolves
the zai profile (via MANIGOT_PROFILE in the fake .env checkout), asserts
TERM/COLORTERM/TERM_PROGRAM/TERM_PROGRAM_VERSION/VTE_VERSION/KITTY_WINDOW_ID/
WT_SESSION/WEZTERM_* are forwarded and that no `-e\nTMUX=` /
`-e\nTMUX_PANE=` pair appears in the joined argv. Assertions match the argv
construction (`-e`, `KEY=VALUE` as separate entries; WEZTERM_* as single
`-e KEY=VALUE` entries), so the check is exact. Test logic is sound.

TASK-3: PASS
notes: docs/AGENTS.md, README.md, project-template/docs/AGENTS.md all updated
with the same corrected statement (TMUX/TMUX_PANE forwarded for Claude Code,
deliberately NOT for OpenCode, because the TMUX-gated DCS passthrough is
discarded by default tmux config; supported path is plain OSC 52 +
set-clipboard on), including the mg host exception. The three sources are
mutually consistent; the root AGENTS.md overlay (mounted docs/AGENTS.md) shows
the updated text. Archived job files correctly left as historical record —
no sync requirement there.

TASK-4: PASS
notes: internal/session/host.go — `hostEnv` filters `TMUX=`/`TMUX_PANE=`
entries out of the child env when `info.Tool == config.ToolOpenCode` (lines
195-204); the prefix match is exact (no false positives on other TMUX_* vars)
and runs after the KeyEnv append, which never contains TMUX. Claude Code host
env untouched. `strings` and `config` were already imported, so the file
compiles. host_test.go adds `TestBuildHostOpenCodeStripsTmux` (envMap
assertions: TMUX/TMUX_PANE absent, TERM/ZHIPU_API_KEY pass through) and
`TestBuildHostClaudeKeepsTmux` (TMUX/TMUX_PANE retained) — both correct.

TASK-5: PASS
notes: implementation.md records the outcome: go build/vet/test (-count=1)
green, and re-verification that the installed opencode-ai 1.18.18 compiled
binary contains both the plain OSC 52 start and the `ESC P tmux;` passthrough
wrapper, so the TMUX-gated mechanism the fix relies on still holds at the
installed version. Residual user-side limitations (terminal OSC 52 support,
tmux set-clipboard on/external) and the unpinned opencode-ai install are
documented. Note: the Go toolchain could not be re-run during this review —
the session git shim allows only git read/commit commands — but static review
of every changed file confirms compilation (imports present, signatures
consistent across the single call sites) and the tests are logically correct.

## Security

none — no security review run. The change is env-forwarding-only: it removes
two env vars from OpenCode sessions (container and mg host) and never mutates
tmux/terminal state. No new privileges, no new mounts, no config writes.

## Overall

APPROVED

The regression is correctly identified (the previous job's TMUX forwarding
made OpenCode emit tmux DCS passthrough, which default tmux config discards
entirely) and the fix is minimal and correct: OpenCode sessions no longer see
TMUX/TMUX_PANE, so they emit plain OSC 52, which tmux's set-clipboard on (the
already-documented, warned-about user prerequisite) handles. Claude Code is
untouched on both the docker and host paths, all five tasks are implemented as
specified with tests, the three doc sources stay in sync, and every commit is
in the `[ID] TASK-N:` format with implementation.md committed. Nothing must
change before merge.