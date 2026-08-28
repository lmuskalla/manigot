## Summary

Built the Svelte 5 web UI (`web-ui/`) for the manigot control plane — the
browser client for the `mg serve` daemon that the TUI already provides on
the terminal. The app renders the daemon's read-only API today (projects,
jobs with the stage pipeline, job markdown files, jdi status + run/session
logs, quick diff, agents, health) and is structured for the mutating
surface as job two's endpoints land, probing for the capability and
rendering "not available on this daemon" instead of crashing.

The previous developer session was cut off mid-way through the visual
verification pass; this session finished that verification against both the
mock backend and a real `mg serve` daemon, fixed the issues it surfaced
(most notably a job-file URL mismatch that broke brief/tasks rendering
against the real daemon), pruned accidentally-committed `node_modules`/
`dist`, added a README, and committed everything.

## Changes

- **Scaffold + stack** (`web-ui/`, committed in the prior session as
  `149d178d`): Vite + Svelte 5 (runes) + TypeScript + Vitest, fonts
  (Familjen Grotesk Variable + IBM Plex Mono), design tokens in
  `src/app.css` and shared styles in `src/styles/ui.css`. Views: Jobs,
  Job detail (tabs: brief/tasks/implementation/verdict/diff/run), Crew
  (agents), Daemon (health); command palette (⌘K); connection settings
  modal; toasts; hash router; polling data store; in-browser mock backend
  for daemon-less development.
- **API client** (`web-ui/src/lib/api/client.ts`): fetch wrapper with the
  daemon's error-envelope conventions, optional bearer token, and Part 2
  capability probing (404/405 → `capabilityMiss`, so mutating actions
  degrade gracefully on a read-only daemon). Endpoint paths live in
  `src/lib/api/endpoints.ts` as the single seam against the daemon.
- **Job-file URL fix** (this session): the client requested
  `/files/brief` but the shipped daemon (job one) serves the on-disk name
  `/files/brief.md` — brief/tasks/implementation/verdict all rendered
  "no file yet" against a real daemon. Fixed the client to map tab ids to
  filenames and mirrored that in the mock backend; all mock tests pin the
  corrected shape.
- **`?api=` URL param** (`connection.svelte.ts`): deep-link/testing
  convenience to point the UI at a daemon for a session without touching
  the settings modal (tokens still only live in settings, never URLs).
- **Visual verification + fixes** (this session, continuing the prior
  session's pass): rendered every view with the `shot` tool against the
  mock backend at 375/768/1280 and against a real `mg serve` daemon (via
  the vite `/api` proxy) with a fixture project + two real jobs. Fixed:
  stray `//` comment text rendering as template content in components,
  WCAG AA contrast (ink tokens, `.when` timestamps, primary button),
  job-row flex collapse at mid widths, mobile stacking, pipeline station
  label overflow, and a solid primary-button background so the render
  report's contrast measurement is meaningful. All views now report zero
  error findings (contrast/overflow) at all three widths; the only
  remaining flags are by-design horizontal scroll containers (tab strip
  and stage pipeline on phones), which have a scrollability fade hint.
- **Hygiene**: `node_modules/` (4372 files) and `dist/` were accidentally
  committed in the prior session's scaffold commit — untracked them
  (index-only) and added `web-ui/.gitignore`. Committed the analyst's
  `docs/listener.md` gap annotation and dropped two stray draft-brief
  files left at the repo root.
- **Tests**: 61 vitest tests across the client, endpoint paths, parsers
  (diff/runlog/markdown/stage/router), the mock backend, and the Pipeline
  component — all passing; production build clean.
- **Docs**: `web-ui/README.md` with run/dev/test instructions, the
  mock-mode workflow, and design notes.

## Known issues / follow-ups

- **Part 2 endpoints are not shipped yet** — the UI degrades to the
  read-only surface against the current daemon (job two's mutating
  endpoints + SSE stream are the direct prerequisite; the client already
  probes for them).
- The `shot --describe` vision layer is unavailable in this session's
  profile (no `ZHIPU_API_KEY`), so visual review was driven by the
  measured render reports (contrast/overflow/alignment) rather than
  vision prose.
- The vite dev proxy targets `127.0.0.1:8080` by default; a daemon on a
  different port needs the `?api=` param or a settings change.
- Cross-origin direct connections are blocked by the daemon's no-CORS
  posture by design; production serves the UI same-origin (or behind a
  reverse proxy).