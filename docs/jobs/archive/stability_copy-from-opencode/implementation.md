# Implementation: copy from opencode

id: stability
status: open
developer: deepseek-v4-flash
date: 2026-08-25

<!-- Produced by @developer after implementation. -->

## Summary

Fixed copying from OpenCode sessions (the brief: "run opencode via mg via
tmux, cannot copy anything"). The previous job's fix (`experiment_cannot-
copy-from-agents`) was the regression: it forwarded `TMUX`/`TMUX_PANE` into
the container, which made OpenCode emit its tmux DCS-passthrough OSC 52 form
(`ESC P tmux; ...`). Default tmux configuration discards that passthrough
entirely (`allow-passthrough` defaults to off) — before any `set-clipboard`
handling — so the host clipboard was never touched even for users who set
`set-clipboard on`. The fix: OpenCode sessions must not see `TMUX`/`TMUX_PANE`,
so OpenCode emits plain OSC 52, which tmux's `set-clipboard on` (the already-
documented user prerequisite) handles correctly. Claude Code's env is
untouched. The same strip applies to the `mg host` OpenCode child env, which
shares the root cause (host env has `TMUX` set).

## Changes

TASK-1: Stop forwarding `TMUX`/`TMUX_PANE` into the container for OpenCode
     sessions. `internal/session/docker.go`: `terminalEnvArgs` now takes the
     resolved tool and skips `TMUX`/`TMUX_PANE` when the tool is OpenCode
     (new `tmuxEnvVars` gate); Claude Code's argv stays byte-identical. The
     comment block above `terminalEnvVars` was rewritten — the old claim that
     OpenCode "additionally switches to its tmux DCS-passthrough OSC 52 form
     when TMUX is set" was the bug; it now documents the exception and why.
     Call site passes `info.Tool`.

TASK-2: `internal/session/docker_test.go`: kept `TestBuildTerminalEnvForwarding`
     as-is (default claude-pro profile, TMUX assertions still valid for Claude
     Code) and added `TestBuildTerminalEnvForwardingOpenCodeStripsTmux`: a zai
     (OpenCode) session forwards TERM/COLORTERM/.../WEZTERM_* but NOT
     TMUX/TMUX_PANE.

TASK-3: Docs — `docs/AGENTS.md`, `README.md`, `project-template/docs/AGENTS.md`
     (kept in sync per the hard rule). The "OpenCode additionally switches to
     tmux's DCS-passthrough OSC 52 form when TMUX is set" claim is replaced
     with the corrected statement: TMUX/TMUX_PANE are forwarded for Claude Code
     but deliberately NOT for OpenCode, because OpenCode's TMUX-gated DCS
     passthrough is discarded by default tmux config (`allow-passthrough` off)
     and the supported path is plain OSC 52 + tmux `set-clipboard on`. The
     `mg host` exception is documented too.

TASK-4: `internal/session/host.go`: `hostEnv` now filters `TMUX`/`TMUX_PANE`
     out of the OpenCode child env (Claude Code keeps the full host env).
     `internal/session/host_test.go`: added `TestBuildHostOpenCodeStripsTmux`
     (OpenCode child env has no TMUX/TMUX_PANE, rest passes through) and
     `TestBuildHostClaudeKeepsTmux` (Claude host env keeps TMUX/TMUX_PANE).

TASK-5: Verification — `go build ./...`, `go vet ./...`, `go test ./...`
     (`-count=1`) all green. Re-verified the opencode clipboard mechanism
     against the installed opencode-ai 1.18.18 (the exact version the archived
     job verified): the compiled binary contains both the plain OSC 52 start
     (`ESC ] 52`) and the tmux DCS passthrough wrapper (`ESC P tmux;`), so the
     TMUX-gated passthrough behavior is unchanged at the installed version and
     the fix's mechanism holds. Outcome recorded here.

## Known issues / follow-ups

- The `warnTmuxClipboard` stderr warning is unchanged (read-only, as the
  tasks mandated) — its "set-clipboard on" advice is now fully correct again,
  since OpenCode emits plain OSC 52.
- Residual user-side limitations are unchanged and unfixable from mg: the host
  terminal emulator must support OSC 52, and tmux `set-clipboard` must be
  `on`/`external` inside tmux.
- The Dockerfile's `npm install -g opencode-ai` is unpinned; if a future
  opencode version drops the TMUX-gated passthrough (or changes its clipboard
  emission), the strip becomes harmless either way — plain OSC 52 is the
  universally-supported form — but the mechanism note above should be
  re-checked against the then-installed version.
- One flaky pre-existing failure observed in `internal/launch`
  (`TestJdiStartsResolvedCommandDetached`, a timing-sensitive poll of a
  detached stub's marker file) — passes reliably with `-count=1`; unrelated to
  this job (launch does not depend on session).