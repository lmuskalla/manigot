# Implementation: web ui: dashboard

## Summary

Added a default dashboard view to the web UI, rendered at `#/` (root) and
reachable via the "manigot" brand link in the top-left, per the brief. The
dashboard shows an overview across every registered project: aggregate
counts (projects, open jobs, needs-human, running), an attention-sorted list
of jobs needing attention across all projects, and a projects grid linking
into each project's jobs view.

## Changes

TASK-1: Added `web-ui/src/lib/views/DashboardView.svelte` (new view) and
`web-ui/src/lib/dashboard.ts` (pure aggregation logic, factored out for
testability). The view loads `api.getProjects()` once on mount, then fans
out one `api.getJobs(project)` call per project via `Promise.allSettled` —
a per-project failure degrades to an inline "couldn't load jobs for X" note
in that project's card rather than blanking the page. `dashboard.ts`
exports `aggregateCounts()` and `attentionJobs()`, both built on `stage.ts`'s
existing `attention()`/`sortByAttention()`. No polling beyond the initial
load, to avoid a request storm as the project registry grows. Styled to
match the existing view conventions (`.page` layout, `--r-*`/`--ink-*`/
`--bg-*` tokens).

TASK-2: Wired the dashboard into `web-ui/src/App.svelte`: imported
`DashboardView` and added a route branch rendering it when
`route.name === 'home'` and the daemon is connected with a loaded, non-empty
project list. Removed the `$effect` that previously force-navigated `home`
→ `jobs/<active>` whenever a project was active — `#/` and the brand link
now land on the dashboard instead of auto-redirecting. The existing
down/no-projects/connecting/error landing states are untouched (same
trailing `{:else}` branch, now only reached when the dashboard's condition
doesn't hold).

TASK-3: Rewrote the `App.test.ts` regression test that asserted the old
auto-redirect behavior. It now asserts the dashboard renders at `#/` (hash
stays `#/`, `.landing` is gone, `h1` reads "Dashboard") and preserves the
original double-hash regression guard by driving the project-switch path
(`navigate(href(...))` via the project `<select>`) and confirming the hash
lands as a single `#/p/manigot`, not `##/p/manigot`.

TASK-4: Added test coverage for the new behavior: a focused `App.test.ts`
case rendering the dashboard with two projects and jobs in various
states, asserting the attention list and projects grid render correctly;
and a new `web-ui/src/lib/dashboard.test.ts` unit-testing `aggregateCounts()`
and `attentionJobs()` (counting, skipping failed per-project loads, sorting
by attention, tagging jobs with their project) — following `stage.test.ts`'s
pattern.

TASK-5: Updated `web-ui/README.md`'s `Layout` section to list
`DashboardView` alongside the other views.

## Verification

- `npm run build` — clean production build, no new warnings.
- `npm test` — full suite (93 tests across 13 files) passes.
- Rendered the dashboard with `shot` against the mock backend
  (`npm run dev:mock`) at 375px and 1280px — layout, contrast (WCAG AA),
  and attention-list/project-grid content all correct; screenshots and the
  render report are committed under `screenshots/`.

## Known issues / follow-ups

None.
