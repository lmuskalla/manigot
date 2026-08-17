# Tasks: cannot copy from agents

id: experiment
status: open
analyst: deepseek-v4-flash
date: 2026-08-17

<!-- Produced by @analyst from brief.md. -->

## Investigation summary

Copying text from inside an mg agent session (mostly OpenCode, occasionally
Claude Code) shows a "copied" indicator in the agent TUI but never reaches the
host clipboard.

Mechanism: both agent CLIs are full-screen TUIs running inside the Docker
container. The only way such an app can write the *host* OS clipboard is the
OSC 52 terminal escape sequence (`ESC ] 52 ; c ; <base64> BEL`), which flows
container TUI -> docker `-t` pty -> docker client -> host terminal (possibly
through tmux). Docker forwards the bytes unmodified; the failure happens
upstream of the terminal, at two points:

1. **The in-container TUI must decide to emit OSC 52.** Apps gate that on
   terminal identity/capability env vars (`TERM`, `COLORTERM`, `TERM_PROGRAM`,
   `TERM_PROGRAM_VERSION`, `VTE_VERSION`, `KITTY_WINDOW_ID`, `TMUX`,
   `TMUX_PANE`, `WT_SESSION`/`WEZTERM_*`). manigot forwards **none** of them:
   the Dockerfile sets no `TERM`, and `BuildDockerInvocation`'s `-e` list
   carries only credentials / model / GIT_* / MANIGOT_* vars. Inside the
   container `TERM` is empty (or `dumb`) and every capability var is absent,
   so the TUI sees an unrecognized terminal. OpenCode's select-to-copy (the
   thing that "says copied") cannot then engage the real clipboard path.
   Claude Code mostly works because its copy surface is often the terminal's
   native mouse selection, which never involves the app at all — consistent
   with "mostly opencode, claude code works most of the times".
2. **tmux swallows OSC 52 when `set-clipboard` is off.** The TUI launches
   every session as a tmux split pane (the always-wins branch in
   `internal/launch`), and bare `mg` inside a tmux pane works the same way.
   tmux intercepts all pane output; with `set-clipboard off` (the pre-tmux-3.2
   default and a common deliberate user setting) the sequence is stripped and
   the host clipboard is never touched — while the app still shows "copied"
   (it did write the bytes to its stdout).

`mg host` sessions are unaffected: the CLI runs on the host with the user's
full environment, so the capability vars are already present.

## Task breakdown

TASK-1: Forward the host's terminal environment into the container so the
     in-container TUIs see the real terminal: for each of `TERM`, `COLORTERM`,
     `TERM_PROGRAM`, `TERM_PROGRAM_VERSION`, `VTE_VERSION`, `KITTY_WINDOW_ID`,
     `TMUX`, `TMUX_PANE`, `WT_SESSION`, and any `WEZTERM_*` var, add a docker
     `-e` entry when the var is set and non-empty on the host (never forward an
     empty value; no var set -> byte-identical argv to today). Gated to the
     interactive docker path (`internal/session/docker.go`'s
     `BuildDockerInvocation`); `--print`/mg-jdi runs are non-interactive so the
     vars are harmless there; `mg host` needs nothing (full host env already).
     Update the pinned argv assertions in `internal/session/docker_test.go` and
     add a focused test for the conditional forwarding.
     files: internal/session/docker.go, internal/session/docker_test.go
     depends: none
     risk: medium — the docker argv is load-bearing and pinned by many tests;
     strict "only when set and non-empty" keeps every existing behavior intact.

TASK-2: Detect tmux clipboard interception at session start and warn: when the
     host `$TMUX` is set (and `tmux` is on PATH) and the session is
     interactive (not `--print`), run `tmux show-options -g set-clipboard`;
     if it reports `off`, print a clear one-line warning on stderr (the
     diagnostics stream) telling the user their tmux will swallow OSC 52
     clipboard writes and to run `tmux set -g set-clipboard on`. Strictly
     read-only: no tmux state mutation, and any tmux failure (not installed,
     no server) silently skips the check. Add the check where the session
     diagnostics are built and cover it with a test.
     files: internal/session/docker.go, internal/session/session_test.go (or
     docker_test.go)
     depends: none (independent of TASK-1)
     risk: low — a read-only tmux query plus a stderr warning; failures are
     silently skipped, so no launch path can break.

TASK-3: (Investigation-gated) Make OpenCode's copy engage the real clipboard:
     first verify against the installed `opencode-ai` version (the Dockerfile's
     unpinned `npm install -g opencode-ai`) what mechanism its select-copy
     uses and whether `~/.config/opencode/opencode.json` (already written by
     `scripts/entrypoint.sh` for the model substitution) supports a documented
     clipboard/copy key (e.g. an autocopy-style setting). If such a key
     exists, have the entrypoint write it into the per-session opencode config
     for interactive sessions without clobbering the existing `{env:OPENCODE_MODEL}`
     block. If no such knob exists, record the finding in implementation.md
     and mark this task N/A — the TASK-1 env forwarding is the fallback fix.
     files: scripts/entrypoint.sh (+ implementation.md note)
     depends: none
     risk: medium — the opencode config schema is unverified (unpinned npm
     package); the entrypoint is the only bash and must stay self-contained;
     must not regress the model substitution already written to the same file.

TASK-4: Document the clipboard path so users can self-diagnose: add a
     "Clipboard / copying from agent sessions" section to `docs/AGENTS.md`
     explaining that in-container TUIs copy via OSC 52, that mg now forwards
     the terminal env vars, and the two user-side prerequisites (the terminal
     emulator must support OSC 52, and tmux needs `set-clipboard on` when the
     session runs inside tmux). Mirror the same content into `README.md` and
     keep `project-template/docs/AGENTS.md` in sync (hard rule).
     files: docs/AGENTS.md, README.md, project-template/docs/AGENTS.md
     depends: TASK-1 (documents the concrete forwarded-var list)
     risk: low — documentation only, but the three doc sources must stay in
     sync per the hard rules.

TASK-5: Verify the whole change: `go build ./...` and `go test ./...` green
     (including the updated docker argv tests), and — if a docker daemon is
     available in the environment — a `docker run` env smoke check confirming
     the forwarded vars actually appear inside the container for an
     interactive build and are absent when unset on the host. Record the
     outcome in implementation.md, including any residual limitation (e.g.
     terminals without OSC 52 support can still not be fixed by mg).
     files: implementation.md (outcome note only)
     depends: TASK-1, TASK-2, TASK-3, TASK-4
     risk: low — test-suite run and an env smoke check; no new behavior.

## Notes for the developer

- The exact in-app copy mechanism of the installed opencode/Claude Code
  versions could not be inspected from this repo (the CLIs are installed by
  the Dockerfile's unpinned npm installs; nothing is vendored). TASK-1 is the
  general, low-risk fix that helps any TUI keying off terminal identity;
  TASK-3 is the targeted hedge and may end up N/A.
- Do not mutate tmux or the user's terminal config anywhere — TASK-2 is
  warn-only by design.
- The `-e` env list in `BuildDockerInvocation` is pinned by many tests;
  forward nothing when a var is unset or empty so non-interactive and CI
  behavior stays byte-identical.