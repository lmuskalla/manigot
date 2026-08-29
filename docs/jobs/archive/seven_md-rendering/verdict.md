# Verdict: md rendering

id: seven
status: open
reviewer: claude (claude-pro)
date: 2026-08-29

## Review

This is a re-review after the first round's NEEDS WORK (previous verdict
content is preserved in git history at commit 5d0f901). All three items the
first verdict required were addressed; verified independently below rather
than taken on faith.

TASK-1: PASS
notes: Correctly a no-op, unchanged from the first round. `markdown.ts` +
`MarkdownView.svelte` already run `marked` + `DOMPurify`, wired through all
four job-file tabs. No diff touches these files. Render evidence
(`screenshots/task1-md-render-brief.png`) and `markdown.test.ts` (9 tests)
confirm parsed rendering, not plain text.

TASK-2: PASS
notes: `web-ui/src/lib/runlog.ts` — `parseRunLog` now recognizes both
job-level terminal headers (`mg jdi finished: <kind>` and `mg jdi stopped
before running any agent`) as their own header match, calling `flushOpen()`
first so an in-progress `finished`/`stop` event can never absorb them. I
reproduced the fix directly: ran the full `runlog.test.ts` suite (13/13
pass, including the new `FULL_APPROVED_LOG` regression case: started →
invoked → finished → `mg jdi finished: stop-finished` trailer) and confirmed
by inspection that the previously-reported repro
(`reviewer finished (attempt 1)` body immediately followed by the trailer)
now yields four distinct events (`start`, `invoke`, `finished`, `stop`) with
the trailer's text isolated to its own `stop` event
(`"finished — verdict.md's Overall verdict is APPROVED"`), never leaking
into the preceding `finished` event's `.text`. The stop-before-any-agent
case (no reason line, `includeReason=false`) and the `stop-needs-human`
relabeling (kept as `"needs human"` for existing CSS styling) are also
covered and pass. `start`/`invoke`/`human` parsing is untouched; the old
idealized `note`/single-line-`stopped:` conventions still parse for
backward compat. Full web-ui suite: 97/97 passing, `npm run build` succeeds.

TASK-3: PASS
notes: Re-verified against the now-fixed harbor mock fixture
(`?mock=1#/p/manigot/j/harbor_needs-human-on-conflict-handling/run`).
The render report's element inventory (`screenshots/render-report.md`,
lines ~140-147) confirms the exact sequence the fix is meant to produce:
`@developer finished` → its own `p.response` row ("The done-conflict fix is
strai...") → a separate `handoff` row (`NEEDS-HUMAN-INPUT: should done...`)
→ a separate `stopped` row (`"needs human — developer asked..."`, 11.01:1
contrast, well past AA) — three distinct rows, not one bloated/leaked
`finished` event. `RunConsole.svelte`'s `.response` block (`flex-basis:
100%`, `pre-wrap`) and `ev-finished` styling are markup/CSS only, and no AA
contrast regression is reported (the AAA-only warnings in the render report
predate this change and apply to unrelated chrome, e.g. nav labels).

TASK-4: PASS
notes: `web-ui/src/lib/api/mock.ts` — the harbor fixture's terminal line is
now the real `=== ... mg jdi finished: stop-needs-human ===` header + a
realistic reason line, replacing the old idealized `stopped: needs human`
convention that let the TASK-2 bug ship undetected last round. Both
fixtures' `invoked`/`finished (attempt N)` headers and multi-paragraph
bodies match the real generator's shape. `mock.test.ts` (15 tests, substring
assertions only) still passes.

## Security

none — front-end parsing/rendering only; the grouped response text is
rendered via Svelte text interpolation (`{ev.text}`), not `{@html}`, so no
new XSS surface. Confirmed unchanged in `RunConsole.svelte`'s diff.

## Verification performed this round

- `npm test` (vitest): 97/97 passing, including all 13 `runlog.test.ts`
  cases (9 new).
- `npm run build`: succeeds, no type/compile errors.
- Read the full `runlog.ts` diff line-by-line against the reported bug's
  minimal repro; confirmed `flushOpen()` is called before the job-level
  header branches and that `openEvent`/`openBody` are reset per new event.
- Cross-checked `screenshots/render-report.md`'s element inventory (DOM
  text + position + contrast, not just the PNG) against the specific claim
  in implementation.md about the harbor job's timeline ordering.
- Confirmed the stale-filename cleanup (`task3-runlog-grouped-harbor-
  handoff.png` → `...-post-task2fix.png`) is a clean rename across commits
  04ab08d/8d7f11a with implementation.md's reference updated to match.

## Overall

APPROVED

No further changes required. All four tasks pass; the TASK-2 bug from the
first review round is fixed and covered by a regression test built from the
exact realistic log shape (start → invoke → finished → real job-finished
trailer) the first verdict asked for, and TASK-3/TASK-4 were correctly
re-verified against the fixed path rather than left on stale evidence.
