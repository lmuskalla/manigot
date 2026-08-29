# Implementation: md rendering

id: seven
status: open
developer: claude (claude-pro)
date: 2026-08-29

<!-- Produced by @developer from tasks.md. -->

## Summary

Render-verified the web UI's existing markdown pipeline (found it already
correct — no code change) and fixed the actual, reproducible bug: mg-jdi's
real `run.log` "finished" header wasn't recognized by the front-end parser,
so a multi-paragraph agent response was split into one bare-`·` timeline row
per line. Taught the parser to recognize it, group the response into a
single event, updated the timeline UI to render it as one readable block,
and reshaped the mock daemon's fixtures to match the real generator's shape
so `?mock=1` exercises the same code path production does.

## Changes

TASK-1: Render-verified markdown coverage across the web UI. Read
`web-ui/src/lib/markdown.ts` and `MarkdownView.svelte`, confirmed
`MarkdownView` is the only `{@html}` renderer in the app and is used for all
four job files (brief/tasks/implementation/verdict) in `JobDetailView.svelte`.
Ran the existing `markdown.test.ts` suite (passes) and additionally
render-verified with `shot` against the mock daemon (`?mock=1`) across all
four tabs of two different jobs: headings, the frontmatter table, bold
`TASK-n:` markers, lists, and tables all render correctly — no plain/unparsed
markdown found anywhere. **No code change** — this task was a verification
pass, per its own risk note ("likely a no-op"). The one other place raw text
appears, the Diff tab, is intentionally plain (`git diff` output is not
markdown, per its own docs), so that's not a gap either.
Evidence: `screenshots/task1-md-render-brief.png`.

TASK-2: `web-ui/src/lib/runlog.ts` — taught `parseRunLog` to recognize the
real `=== <ts> <agent> finished (attempt N) ===` header
(`src/cmd/mg/jdioutput.go`'s `logInvocation`), which previously fell through
to the generic `raw` case entirely unrecognized. The header now opens a new
`'finished'` event (capturing `agent`/`attempt`); every following line is
appended to that event's body until the next header/stop line, at which
point the body is joined and trimmed into `.text` (blank lines preserved as
paragraph breaks). The event object is pushed to the output array immediately
at its header so ordering stays correct even when a `NEEDS-HUMAN-INPUT:`
line appears mid-body — it's still extracted as its own `'human'` event,
unchanged from before. Existing `start`/`invoke`/`stop`/`human` parsing and
the old single-line `note` convention are untouched.
`web-ui/src/lib/runlog.test.ts` — added a new fixture mirroring the real
header shape (multi-paragraph response, embedded `NEEDS-HUMAN-INPUT:` line)
alongside the pre-existing idealized-mock fixture and its tests, all of which
still pass.

TASK-3: `web-ui/src/lib/components/RunConsole.svelte` — added an
`ev-finished` timeline case: an `@agent finished · attempt N` marker row
(mirroring the existing `invoke` row's style) plus the grouped response text
rendered as a single wrapped-prose block (`<p class="response">`,
`white-space: pre-wrap`) underneath, instead of the old bare-`·` `raw`
fallback rows one per line. Existing `invoke`/`human`/`stop` styling is
unchanged; verified with `shot` against the reshaped mock fixtures (TASK-4)
that WCAG AA contrast (8.53:1) and layout (no overflow/clipping) hold.
Evidence: `screenshots/task3-runlog-grouped-farmer.png` (fully-finished run)
and (superseded by the post-review re-verification below, since the harbor
fixture's terminal line changed) `screenshots/task3-runlog-grouped-harbor-handoff-post-task2fix.png`
— a `finished` event whose body contains an embedded `NEEDS-HUMAN-INPUT:`
line, confirmed still extracted as its own handoff row.

TASK-4: `web-ui/src/lib/api/mock.ts` — reshaped both jobs' `runLog` fixtures
(`farmer_part-2-of-web-ui-tui-path`, `harbor_needs-human-on-conflict-handling`)
from the idealized single-line `"analyst finished — ..."` convention to the
real generator's shape: `invoked` header, `finished (attempt N)` header, two
blank lines, then a multi-line/multi-paragraph response — including one
response with an embedded `NEEDS-HUMAN-INPUT:` line for the harbor job's
handoff scenario. `mock.test.ts`'s substring-only assertions (`'mg jdi
started'`, `'quality invoked'`) still pass; full suite re-run confirmed
(92/92 passing).

## Post-review fixes (NEEDS WORK round)

The first review round found a real bug in TASK-2 (see verdict.md's first
round of notes, still visible in git history): the job-level terminal header
mg-jdi always appends at the end of every run —
`=== <ts> mg jdi finished: <kind> ===` (+ an optional reason line), or
`=== <ts> mg jdi stopped before running any agent ===` for the
stop-before-any-agent case (both from `src/cmd/mg/jdioutput.go`'s
`logJobFinished`/`logImmediateStop`) — was not recognized as a header at all.
Since it almost always immediately follows the last agent's `finished`
header/body, it was silently absorbed into that agent's response text
instead of becoming its own event — corrupting the most common real
run.log ending (a normally-completed run, approved or needing human input),
and doing so *silently* (worse than the pre-fix bare-`·` `raw` row it
replaced).

Fixed in `web-ui/src/lib/runlog.ts`: `parseRunLog` now recognizes both
job-level terminal headers, closing any still-open event first so they can
never be absorbed into it. They're emitted as their own `'stop'` event,
reworded to the existing `'finished'`/`'needs human'` labels (so the
pre-existing `needs human` styling in `RunConsole.svelte` keeps applying
unchanged) with the optional reason line appended (`"<label> — <reason>"`).
The `openFinished`/`flushFinished` machinery was generalized to
`openEvent`/`flushOpen` so both event kinds share the same body-accumulation
logic, keeping the diff small.
`web-ui/src/lib/runlog.test.ts` — added the regression test the verdict asked
for: a complete, realistic log (started → invoked → finished → the real
`mg jdi finished: stop-finished` trailer), plus a `stop-needs-human` variant
and a stop-before-any-agent variant (two consecutive job-level headers, the
`includeReason=false` case). All 13 tests pass (9 new).

`web-ui/src/lib/api/mock.ts` (TASK-4 follow-up) — the harbor job's `runLog`
fixture's terminal line was still the old idealized
`=== ... stopped: needs human ===` convention, the exact mismatch that let
the TASK-2 bug ship undetected against `?mock=1`. Replaced it with the real
`=== ... mg jdi finished: stop-needs-human ===` header + a realistic reason
line (`"developer asked for human input: ..."`, matching
`jdi.go`'s actual `finish()` call for the NEEDS-HUMAN-INPUT-in-response
path). `mock.test.ts` still passes (substring assertions only).

TASK-3 re-verified against the fixed harbor fixture: rendered
`?mock=1#/p/manigot/j/harbor_needs-human-on-conflict-handling/run` with
`shot --full-page`. The render report confirms the timeline now ends with
`@handoff` (`NEEDS-HUMAN-INPUT: should done ...`) immediately followed by its
own `stopped` row (`"needs human — developer asked ..."`) — a separate event,
not leaked into the preceding `finished` event's `.response` text, at
11.01:1 contrast (well past AA). Note: the app shell (`App.svelte`'s
`.shell { height: 100dvh }`) scrolls its content pane internally rather than
the document itself, so `shot --full-page`'s screenshot (driven by
`document.scrollHeight`, which stays at viewport height) doesn't visually
show content below the fold — this was already true of the pre-fix
screenshots in this same job. The render report's element inventory (exact
DOM text + position + contrast, independent of the PNG crop) is the
authoritative evidence here, same as it is for any measurement beyond what's
visually captured.
Evidence: `screenshots/task3-runlog-grouped-harbor-handoff-post-task2fix.png`
+ `screenshots/render-report.md`.

## Known issues / follow-ups

- No screenshot exists anywhere in this worktree matching the brief's "the
  screenshot in the workspace root" reference (confirmed by both the analyst
  and this pass) — TASK-1's conclusion (no plain-markdown bug found) rests on
  render-verification via `shot`, not on reproducing whatever the original
  screenshot showed.
- The web UI's content panes scroll internally rather than the document
  (see the TASK-3 re-verification note above) — `shot --full-page` can't
  capture content below an internal scroll fold. Not a bug introduced or
  found by this job, just a limitation worth knowing about for future
  render-verification passes on this app.
