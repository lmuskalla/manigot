# Verdict: web-ui: connecting to the daemon..

id: balance
status: done
reviewer: balance
date: 2026-08-29

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
  notes: `established` counter on the connection store + an App-level
  `$effect` keyed on it reload the project list exactly once per
  connection establishment. Covered by App.test.ts ("populates the project
  dropdown after connecting via settings, without a reload" and "stops
  fetching /projects once an empty registry has loaded — no reload loop").

TASK-2: PASS
  notes: root-caused to `router.navigate()` double-prefixing an
  already-hashed path from `href()` (`##/p/manigot`, unparseable by
  `parseHash`), which stranded the home→jobs redirect on the landing
  screen forever. Fixed in `router.ts`; `router.test.ts` has a direct
  regression test for the single-hash round trip.

TASK-3: PASS (one issue found and fixed during review)
  notes: `client.ts` now detects and names non-JSON 2xx bodies (HTML vs.
  other) instead of surfacing a raw JSON.parse error, and `data.svelte.ts`
  exposes `projectsError` to the UI. Review found the `loadProjects()`
  catch was swallowing exactly that case: an `ApiError` with `status === 0`
  (which non-JSON 2xx bodies are tagged with) was silently discarded,
  copied over from `refreshJobs`'s flicker-guard on its 2s poll — a guard
  that doesn't apply to `loadProjects`, which only runs once per
  connection establishment. Fixed by removing the guard there; added a
  regression test in `data.test.ts` for a 2xx HTML `/projects` response.
  `refreshJobs` correctly keeps its own guard.

TASK-4: PASS
  notes: 88 tests passing (`npx vitest run`), including the new regression
  test from the TASK-3 fix. `npx tsc --noEmit` clean. `npm run build`
  succeeds. Developer's implementation.md documents a live end-to-end
  verification against `mg serve`.

## Security

None. No new attack surface: this is client-side error-message plumbing
and hash-routing logic; no new endpoints, no credential handling changes,
no new trust boundary.

## Overall

APPROVED

All four tasks pass. One real bug was caught in review (TASK-3's
status-0 swallow silently defeating its own purpose) and fixed with a
regression test before approval. Also swept up incidental job-loop debris
unrelated to the fix itself — a runaway session in this same worktree had
truncated `docs/AGENTS.md` to a stub and left a stray
`docs/jobs/sample_job/` fixture; both restored/removed, see
implementation.md's "Review fix" section.
