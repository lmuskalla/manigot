# Backlog

Ideas that came up during job scoping but were deliberately deferred out of
that job's scope. Not commitments, not prioritized — just written down so
they aren't lost or re-litigated. Promote an item to a real job (`mg-job`)
when it's actually going to be worked on.

## In-TUI agent terminal (split pane / embedded terminal)

Raised during scoping of `vu33rn_fully-autonomous-mode`. Instead of spawning
a new terminal window/pane for each agent session (current `launch.go`
behavior), render the agent's session inside the TUI itself — e.g. a split
window. Would remove the window-management overhead of the current
new-terminal-per-agent model, but is a substantial UI change (embedding or
proxying a PTY inside the Bubble Tea app) and orthogonal to autonomous
orchestration itself. Needs its own scoping.

## Event-streaming subsystem for agent progress

Raised during scoping of `vu33rn_fully-autonomous-mode`. Instead of the TUI
polling job files (`brief.md`/`tasks.md`/`implementation.md`/`verdict.md`)
and deriving `Stage()` from what's written, have a running agent stream
structured events (started, working, blocked, done, etc.) that the TUI reads
live. Would give a more granular, real-time picture of what an agent is
doing mid-session rather than only what stage a job has reached. Deferred
because the v1 autonomous mode is delivering real value with polling alone,
and this is a bigger architectural commitment (a wire format, a writer side
in every agent invocation, a reader side in the TUI) that deserves its own
design pass.

## `@owner` / `@security` in the autonomous sequence

Raised during scoping of `vu33rn_fully-autonomous-mode`. `mg-jdi` v1 drives
a fixed `@analyst → @developer → @reviewer` sequence only, uniformly for
every job type — deliberately starting simple. Folding `@owner` in
(before `@analyst`, gating whether the sequence proceeds at all) and/or
`@security` (alongside or after `@reviewer`, sharing `verdict.md`) is a
natural extension once the core loop is proven, but each adds its own
wrinkle worth scoping separately rather than guessing now — `@owner`
in particular never writes to disk today, so routing on its verdict needs a
signal convention of its own (a "PO-VERDICT:"-style marker was drafted and
then dropped when this was cut from v1 — worth revisiting if this is
picked up).

## Richer step-level agent logging for `mg-jdi`

Raised during scoping of `vu33rn_fully-autonomous-mode`. `mg-jdi`'s `run.log`
(and live output) is only as rich as `claude --print` returns — in its plain
form, each agent's *final response text* per invocation, not a blow-by-blow
of tool calls/file edits. Accepted as the v1 bar for "visibility" rather than
blocking the job on it. If the pinned Claude Code version's `--output-format`
investigation (TASK-2) turns up a richer streaming/step-level format,
upgrading `run.log` to show actual per-tool-call progress instead of just
final answers is a natural follow-up worth its own look, once real usage
shows whether the v1 bar is actually good enough in practice.

## Headless / cron execution

Raised during scoping of `vu33rn_fully-autonomous-mode`. The idea of running
`mg-jdi` unattended on a schedule rather than from an interactive session.
The entire current architecture — the session launcher's `docker run -it`,
terminal spawning in `launch.go`, the TUI polling loop — assumes an attended,
visible terminal a human can look at and interact with. Cron execution has
none of that: it needs detached logging, a way to surface "needs human
input" without a person watching the TUI at that moment (the ping-sound
notification agreed for `vu33rn` obviously doesn't help if no one's in the
room), and a separate review path for whatever it produces. Real enough to
be worth doing eventually, but shouldn't be folded into the first
autonomous-mode job — needs its own brief.
