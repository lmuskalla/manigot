# Tasks: md rendering

id: seven
status: open
analyst: claude (claude-pro)
date: 2026-08-29

<!-- Produced by @analyst from brief.md. -->

## Context / findings

The brief references "the screenshot in the workspace root" — no such
screenshot exists anywhere in this worktree (only `assets/manigot.png`, the
project logo). I could not view what the author saw. That materially affects
TASK-1's certainty; see its note.

The web UI referred to is `web-ui/` (the Svelte control-plane client for
`mg serve`). Investigation found:

- Markdown rendering already exists and looks fairly complete: `marked` +
  `DOMPurify` (`web-ui/src/lib/markdown.ts`), wired through
  `MarkdownView.svelte`, used for all four job files (brief/tasks/
  implementation/verdict) in `JobDetailView.svelte`, with its own test
  coverage (`markdown.test.ts`). So the brief's "md is rendered plainly, not
  parsed" claim does not match what's on this branch today — either it
  predates this feature (already merged before this job branched) or the
  screenshot showed something this audit didn't reproduce. TASK-1 asks the
  developer to actually render-check (they have `shot`; I don't) before
  concluding there's nothing to fix here.
- The run-log "lots of dots" complaint is real and reproducible from reading
  the code (not just a CSS nit): `web-ui/src/lib/runlog.ts`'s `parseRunLog`
  only recognizes `=== ... mg jdi started ===`, `=== ... <agent> invoked
  (attempt N) ===`, and `=== stopped: ... ===` as structured headers. The
  actual header `mg-jdi` writes when an invocation completes —
  `=== <ts> <agent> finished (attempt N) ===` (`src/cmd/mg/jdioutput.go`,
  `logInvocation`) — is NOT recognized, so it falls through to the generic
  `raw` case. Worse, `logInvocation` writes the agent's full final response
  text (`orchestrate.ResultText`, often multiple paragraphs) as the body
  right after that header; `parseRunLog` splits on newlines and treats every
  non-empty line as its own event, so a multi-line response becomes many
  separate `raw` events, each rendered by `RunConsole.svelte` as a timeline
  row whose "who" column is just a bare `·` — "a lot of unnecessary points
  where the row just renders a dot," exactly as described. The web UI's own
  mock fixtures (`web-ui/src/lib/api/mock.ts`) use an idealized single-line
  convention ("analyst finished — tasks.md written (3 tasks)") that doesn't
  match the real generator, which is presumably why this wasn't caught
  against `?mock=1` before.

## Task breakdown

TASK-1: Render-verify markdown coverage across the web UI and fix any
        genuinely plain/unparsed rendering of job markdown found.
     files: web-ui/src/lib/components/MarkdownView.svelte,
       web-ui/src/lib/markdown.ts, web-ui/src/lib/views/JobDetailView.svelte
       (read first — a working marked+DOMPurify pipeline already covers
       brief/tasks/implementation/verdict; only touch what's actually
       broken)
     depends: none
     risk: low — likely a no-op or small fix; the main risk is scope
       creep from guessing at a screenshot nobody can see, so keep any
       change narrowly tied to an actually-reproduced gap, and say clearly
       in implementation.md if none was found

TASK-2: Teach `parseRunLog` to recognize the `<agent> finished (attempt N)`
        run.log header and group the multi-line response text that follows
        it into that single event, instead of emitting one `raw` event per
        line of body text.
     files: web-ui/src/lib/runlog.ts, web-ui/src/lib/runlog.test.ts
     depends: none
     risk: medium — changes a parser other code (`RunConsole.svelte`)
       depends on; must preserve existing `start`/`invoke`/`stop`/`human`
       parsing and the `NEEDS-HUMAN-INPUT:` extraction exactly as today,
       and the new test fixture should mirror the real header format from
       `src/cmd/mg/jdioutput.go` (`logInvocation`/`logAgentInvoked`), not
       just the mock's idealized shape

TASK-3: Update `RunConsole.svelte`'s timeline rendering to display the new
        grouped "finished" event (from TASK-2) as one readable row/block —
        e.g. the response text as wrapped prose under a single event marker
        — instead of one bulleted row per line, and confirm the leftover
        bare-`·` fallback for genuinely unclassified lines is rare/expected
        rather than the common case.
     files: web-ui/src/lib/components/RunConsole.svelte
     depends: TASK-2
     risk: low-medium — markup/CSS only, but must keep existing
       invoke/human/stop styling and WCAG AA contrast intact (see
       `web-ui/screenshots/report-run-tab.md` for the current baseline)

TASK-4: Align the mock daemon's run.log fixtures with the real generator's
        shape (finished headers + multi-line response bodies) so `?mock=1`
        exercises the same parsing path production hits.
     files: web-ui/src/lib/api/mock.ts
     depends: TASK-2
     risk: low — fixture/test-data only; `mock.test.ts` only asserts
       substrings like `'mg jdi started'`, so reshaping the fixture text is
       safe, but re-run its suite to confirm
