# Implementation: web-ui: connecting to the daemon..

id: balance
status: done
developer: balance
date: 2026-08-29

## Summary

Fixed the three connection-flow bugs in the web UI:

1. **Dropdown not populating after connection setup** — the project list now
   loads off an `established` edge in the connection store (bumped on every
   transition into `up`), reloaded once per establishment by an App-level
   effect. A settings change, a first boot, or the daemon coming back all
   repopulate the dropdown without a page reload.
2. **Pages stuck on "Connecting to the daemon…" despite a valid API** — root
   cause was `router.navigate()` double-prefixing the fragment:
   `href()` already returns `#/p/manigot`, and `navigate` wrapped it as
   `#${to}` → `##/p/manigot`, which `parseHash` cannot parse, so the
   home→jobs redirect never landed and the landing kept rendering. Fixed
   `navigate` to strip the leading `#` first. The landing also now tells the
   truth about the connection state instead of showing the message while
   connected.
3. **Errors not reaching the user** — `/projects` failures set
   `projectsError` (shown under the dropdown and on the landing); a 2xx that
   is not JSON (mis-pointed connection, HTML fallback) is named precisely
   instead of a JSON parse error.

## Changes

- `web-ui/src/lib/router.ts` — `navigate()` now strips a leading `#` before
  setting `location.hash` (single-fragment); `href()` falls home when no
  project is active yet.
- `web-ui/src/App.svelte` — bootstrap splits into `connection.check()` +
  effect-driven `loadProjects()` keyed off `established`; landing branches on
  down / projects error / no projects / connecting; dropdown surfaces the
  projects error.
- `web-ui/src/lib/state/connection.svelte.ts` — `established` counter,
  `#gen` supersession so a re-pointed connection is never answered by a stale
  in-flight response, `#inFlight` dedup across concurrent callers, and no
  "connecting" flash on 30s re-validations.
- `web-ui/src/lib/state/data.svelte.ts` — `projectsError` state; load failures
  reach the UI instead of a silent empty dropdown.
- `web-ui/src/lib/api/client.ts` — non-JSON 2xx bodies detected and named
  (HTML vs other); precise CORS/unreachable error text on fetch failure.
- `web-ui/src/lib/components/SettingsModal.svelte` — busy handling so the
  modal always closes/keeps open on the right result.
- Tests: `router.test.ts` (`navigate` single-hash round-trip, the regression),
  `App.test.ts` (dropdown populates after settings change without reload,
  redirect off the landing, empty-registry no-reload-loop, projects error
  surfaced, health panel renders), `connection.test.ts`, `data.test.ts`,
  `client.test.ts`.

Verified end to end against a live `mg serve` (real daemon, vite `/api`
proxy): fresh boot shows "unreachable" not "Connecting to the daemon";
persisted connection populates the dropdown and redirects to the jobs page
without a reload; settings save does the same; the health page renders the
real `/health` response. Full vitest suite: 88 tests passing.

## Review fix

`data.svelte.ts`'s `loadProjects()` catch was swallowing `ApiError` with
`status === 0` — copied from `refreshJobs`'s tight-2s-poll flicker guard,
but `loadProjects` only fires once per connection establishment, so there
was no flicker to guard against. That swallow silently ate exactly the
"2xx but non-JSON body" failure TASK-3 exists for: the dropdown would stay
empty with no `projectsError` shown. Removed the guard so every load
failure — unreachable, non-JSON 2xx included — reaches `projectsError`.
Added a regression test (`data.test.ts`: "surfaces a 2xx non-JSON
/projects response instead of a silent empty dropdown"). `refreshJobs`
keeps its own guard — that path is on a 2s poll and the flicker concern is
real there.

Also cleaned up job-loop debris unrelated to the fix: an accidental
overwrite of `docs/AGENTS.md` (truncated to a `# fixture` stub by a
runaway agent session that used this checkout to test something else) was
restored from `main`, and a stray `docs/jobs/sample_job/brief.md` fixture
from the same session was removed.

## Known issues / follow-ups

- None in scope. (Pre-existing, unrelated a11y warnings from the Svelte
  compiler in Modal/JobDetailView/RunConsole remain.)