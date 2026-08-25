# Tasks: copy from opencode

id: stability
status: open
analyst: deepseek-v4-flash
date: 2026-08-25

<!-- Produced by @analyst from brief.md. -->

## Investigation summary

The brief: running opencode via mg inside tmux, nothing can be copied from the
session. A previous job (`docs/jobs/archive/experiment_cannot-copy-from-agents`,
merged) already attempted a fix; it did not work. This job re-investigates the
same failure with the previous fix's code in place and finds that the previous
fix is the regression.

### Mechanism (unchanged from the archived job)

Both agent CLIs are full-screen TUIs inside the Docker container. The only way
such an app can write the *host* OS clipboard is the OSC 52 escape sequence
(`ESC ] 52 ; c ; <base64> BEL`), which flows container TUI -> docker pty ->
docker client -> host terminal -> tmux. mg forwards the bytes unmodified; the
failure happens upstream.

### Root cause: the previous fix's `TMUX` forwarding

OpenCode's clipboard write (`packages/tui/src/clipboard.ts`, verified against
the installed opencode-ai 1.18.18 in the archived job) emits:

- **plain OSC 52** (`ESC ] 52 ; c ; <base64> ST`) when `process.env.TMUX` is
  NOT set, and
- **tmux DCS passthrough** (`ESC P tmux; ESC ESC ] 52 ; c ; <base64> ESC BEL
  ESC \`) when `process.env.TMUX` IS set.

The archived fix added `TMUX`/`TMUX_PANE` to the forwarded terminal-identity
vars (`terminalEnvVars` in `internal/session/docker.go`) so that — per its
analysis — OpenCode would use the "tmux-aware" passthrough form.

**That passthrough form is dropped by default tmux configuration.** tmux only
honors the DCS passthrough escape (`ESC P tmux; ... ESC \`) when the tmux
option `allow-passthrough` is `on`; the option was added in tmux 3.1 and
**defaults to `off`**. With it off, tmux discards the entire sequence — before
any `set-clipboard` handling, since passthrough is designed to bypass tmux's
own processing entirely. The TUI still shows "copied" (the toast fires on the
promise resolving), but the host clipboard is never touched.

So the two states:

- Before the archived fix: `TMUX` not forwarded -> OpenCode emits plain OSC 52
  -> swallowed when tmux has `set-clipboard off` (default). Broken.
- After the archived fix: `TMUX` forwarded -> OpenCode emits the DCS
  passthrough form -> discarded by tmux (`allow-passthrough off`, the
  default) — **even for a user who followed the archived fix's advice and set
  `set-clipboard on`**, because the passthrough never reaches set-clipboard
  handling. Still broken. This is "we tried, it did not work."

### The fix

OpenCode must emit **plain OSC 52**, which tmux's `set-clipboard on`
(the already-documented user prerequisite, and the thing the archived fix's
warning tells users to set) handles correctly. Concretely: do not let
OpenCode see `TMUX`/`TMUX_PANE` inside the container. Claude Code's container
env stays unchanged (TMUX still forwarded — its copy path is mostly native
mouse selection and must not be regressed).

`mg host` sessions share the root cause: opencode runs on the host with the
full host env (`TMUX` set), so it also emits the passthrough form there. Same
fix applies (strip `TMUX`/`TMUX_PANE` from the OpenCode child env).

### Residual limitations (unchanged, user-side)

- The host terminal emulator must support OSC 52 — unfixable from mg.
- tmux `set-clipboard` must be `on` (or `external`); the existing
  `warnTmuxClipboard` stderr warning already tells the user this, and becomes
  fully correct once OpenCode emits plain OSC 52 again.

## Task breakdown

TASK-1: Stop forwarding `TMUX`/`TMUX_PANE` into the container for OpenCode
     sessions. In `internal/session/docker.go`, make `terminalEnvArgs` take the
     resolved tool and skip `TMUX`/`TMUX_PANE` when the tool is OpenCode
     (keep forwarding them for Claude Code — its container env must stay
     byte-identical). Update the comment block above `terminalEnvVars`
     (currently claims OpenCode "additionally switches to its tmux
     DCS-passthrough OSC 52 form when TMUX is set" — that claim is the bug).
     files: internal/session/docker.go
     depends: none
     risk: medium — the docker argv is load-bearing and pinned by tests; the
     change must keep the Claude Code (default) argv unchanged and only alter
     the OpenCode path.

TASK-2: Cover the OpenCode TMUX-strip in `internal/session/docker_test.go`:
     keep `TestBuildTerminalEnvForwarding` as-is (it uses the default
     claude-pro profile, so its TMUX assertions stay valid for Claude Code)
     and add a test that an OpenCode session (e.g. via `ResolveProfile` with
     the opencode tool) forwards TERM/COLORTERM/... but NOT TMUX/TMUX_PANE.
     files: internal/session/docker_test.go
     depends: TASK-1
     risk: low — test-only.

TASK-3: Mirror the OpenCode TMUX exception into the "Clipboard / copying from
     agent sessions" sections of `docs/AGENTS.md`, `README.md`, and
     `project-template/docs/AGENTS.md` (hard rule: the three sources stay in
     sync). Replace the current "OpenCode additionally switches to tmux's
     DCS-passthrough OSC 52 form when TMUX is set" claim with the corrected
     statement: TMUX/TMUX_PANE are forwarded for Claude Code but deliberately
     NOT for OpenCode, because OpenCode's TMUX-gated DCS passthrough is
     discarded by default tmux config (`allow-passthrough` off) and the
     supported path is plain OSC 52 + tmux `set-clipboard on`.
     files: docs/AGENTS.md, README.md, project-template/docs/AGENTS.md
     depends: TASK-1
     risk: low — documentation only, but the three sources must stay in sync.

TASK-4: Apply the same TMUX strip to the `mg host` OpenCode path: in
     `internal/session/host.go`, filter `TMUX`/`TMUX_PANE` out of the OpenCode
     child env in `hostEnv` (Claude Code untouched). Add/adjust a test in
     `internal/session/host_test.go`. If the developer judges host mode out of
     scope for this brief, record the residual limitation in implementation.md
     instead of changing code.
     files: internal/session/host.go, internal/session/host_test.go
     depends: none (independent of TASK-1; same root cause, different launch path)
     risk: low-medium — host mode is niche; the change alters the OpenCode
     child env inside tmux but keeps the Claude Code host env byte-identical.

TASK-5: Verify the whole change: `go build ./...`, `go vet ./...`, `go test
     ./...` green; confirm the archived job's opencode clipboard mechanism
     (TMUX-gated DCS passthrough) still matches the installed opencode-ai
     version if the image is available in the environment (the Dockerfile's
     `npm install -g opencode-ai` is unpinned — re-check if reachable); record
     the outcome and residual limitations in implementation.md. A docker-run
     smoke check is likely impossible in this agent environment (no daemon) —
     the argv pinning tests are the fallback.
     files: implementation.md (outcome note only)
     depends: TASK-1, TASK-2, TASK-3, TASK-4
     risk: low — test-suite run + outcome note.

## Notes for the developer

- The root cause rests on two external facts, both from the archived job's
  ground truth + tmux's documented defaults: (1) OpenCode wraps OSC 52 in
  tmux DCS passthrough when `process.env.TMUX` is set (verified against
  opencode-ai 1.18.18 in the image), and (2) tmux's `allow-passthrough`
  option defaults to `off` (tmux >= 3.1), so the passthrough is discarded by
  default configuration. Re-verify (1) against the currently installed
  opencode version if the image is reachable; the fix (TASK-1) is correct and
  harmless either way, since plain OSC 52 is the universally-supported form.
- Do NOT change tmux or terminal state anywhere — mg never mutates tmux
  config; `warnTmuxClipboard` stays read-only and stays as-is (its
  "set-clipboard on" advice becomes correct again once OpenCode emits plain
  OSC 52).
- Do NOT remove TMUX/TMUX_PANE from the forwarded list for Claude Code — the
  previous job's env forwarding for Claude Code is considered working
  ("mostly works" per the archived analysis) and must not be regressed.
- The docker argv is pinned by many tests; forward nothing extra and change
  nothing outside the OpenCode branch of TASK-1.