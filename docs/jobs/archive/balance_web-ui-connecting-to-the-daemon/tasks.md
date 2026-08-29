# Tasks: web-ui: connecting to the daemon..

id: balance
status: open
analyst: balance
date: 2026-08-29

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Dropdown not populating after connection setup without a reload.
  files: web-ui/src/App.svelte, web-ui/src/lib/state/connection.svelte.ts,
  web-ui/src/lib/state/data.svelte.ts
  depends: none
  risk: medium — the fix must reload the project list exactly once per
  connection establishment (first boot, a settings change, the daemon
  coming back), never on every 30s re-validation and never in a loop when
  the registry is legitimately empty.

TASK-2: Pages stuck on "Connecting to the daemon.." despite a valid API
  response (e.g. the health page).
  files: web-ui/src/lib/router.ts, web-ui/src/App.svelte
  depends: none
  risk: high — root cause is in the hash router (`navigate`/`href`), a
  shared primitive; a wrong fix here breaks every route, not just the
  landing page.

TASK-3: Errors from the daemon (or a misconfigured connection) must reach
  the user instead of being silently swallowed into an empty state.
  files: web-ui/src/lib/api/client.ts, web-ui/src/lib/state/data.svelte.ts,
  web-ui/src/App.svelte
  depends: none
  risk: medium — includes the case where the endpoint answers 2xx but with
  a non-JSON body (e.g. a dev server or reverse proxy serving `index.html`
  for an unmatched API path), which reads as a confusing JSON-parse error
  today.

TASK-4: Add regression tests for all of the above and verify end-to-end
  against a live `mg serve`.
  files: web-ui/src/lib/router.test.ts, web-ui/src/App.test.ts,
  web-ui/src/lib/state/connection.test.ts, web-ui/src/lib/state/data.test.ts,
  web-ui/src/lib/api/client.test.ts
  depends: TASK-1, TASK-2, TASK-3
  risk: low
