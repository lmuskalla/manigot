# Tasks: web ui: dashboard

id: of
status: open
analyst: claude
date: 2026-08-29

<!-- Produced by @analyst from brief.md. -->

## Context

The web UI (`web-ui/`, a Svelte 5 SPA against the `mg serve` control-plane
API) currently has no dashboard. `#/` (the `home` route) exists in
`router.ts` but `App.svelte` immediately auto-redirects it to the active
project's jobs view (`$effect` in `App.svelte`) whenever a project is
active, so `home` is only ever seen transiently on first boot. The
`manigot` brand link in the rail already points at `#/` (`href="#/"`), so
once `home` renders a real dashboard instead of redirecting, "click manigot
top left to get to the dashboard" falls out for free — no separate nav
change is needed for that part of the brief.

The daemon has no aggregate "jobs across all projects" endpoint — only
`GET /projects` (name + path) and `GET /projects/<p>/jobs` (one project at a
time). Building an "all projects" overview therefore means the dashboard
fans out one `getJobs(project)` call per registered project client-side and
aggregates the results in the browser. This is a frontend-only design
choice (no backend/API change) and is small in practice — the number of
registered projects is expected to be small (a handful, not hundreds).

The brief's "maybe have some useful widgets" is intentionally vague; the
breakdown below proposes a concrete, small set (aggregate counts, an
attention-sorted cross-project job list, a projects grid) but leaves room
for the developer to trim scope if something proves awkward — this is a
judgment call, not a blocking ambiguity, since the brief's core, unambiguous
ask (a default dashboard at root reachable via the logo, showing projects +
an overview) is clear.

## Task breakdown

TASK-1: Add `DashboardView.svelte` (new view) rendering the overview: a
projects grid (name + a link into each project's jobs view, `href({name:
'jobs', project})`), aggregate counts across all registered projects (open
jobs, `stopped:needs-human`, `running`, via `stage.ts`'s existing
`attention()`), and an attention-sorted list of jobs needing attention
across all projects (reusing `sortByAttention`). Data comes from
`api.getProjects()` plus one `api.getJobs(project)` call per project,
fanned out with `Promise.allSettled` so one project's failure doesn't blank
the whole dashboard — a per-project failure should degrade to "couldn't
load jobs for X" inline, not a page-level error. Follow the existing
view conventions (`JobsView.svelte`/`HealthView.svelte`: `.page` layout,
`--r-*`/`--ink-*`/`--bg-*` style tokens, WCAG AA contrast).
files: web-ui/src/lib/views/DashboardView.svelte (new); reads from
  web-ui/src/lib/api/client.ts, web-ui/src/lib/api/types.ts,
  web-ui/src/lib/stage.ts, web-ui/src/lib/router.ts, web-ui/src/lib/time.ts
depends: none
risk: medium — new UI surface and the only place in the app that fans out
  N HTTP calls client-side; needs deliberate partial-failure handling and
  care to avoid an accidental request storm if it's ever wired into the
  existing 2s poll cycle (it should poll on its own, more slowly, or not at
  all beyond initial load — this is a call for the developer, not a
  reason to stop).

TASK-2: Wire the dashboard into the app shell: render `DashboardView` for
`route.name === 'home'` in `App.svelte`'s route block, and remove (or
narrow) the `$effect` that currently force-navigates `home` →
`jobs/<active>` whenever a project is active, so `#/` and the `manigot`
brand link land on the dashboard instead of always bouncing to a project's
jobs list. The existing down/no-projects/connecting/error landing states
(currently the `home` route's only content, in the trailing `{:else}`
branch) must still show correctly when the daemon is unreachable or the
registry is empty/erroring — the dashboard only renders once there's
something to show.
files: web-ui/src/App.svelte
depends: TASK-1
risk: medium — touches the app shell's core routing/state logic; the
  several existing connection-state branches (down, connecting, projects
  error, empty registry) must keep working unchanged for a not-yet-usable
  daemon.

TASK-3: Update the existing regression test in `App.test.ts` — "leaves the
landing once a project is active — the redirect hash must be single" —
which currently asserts the old behavior (home auto-redirects to
`#/p/manigot` once connected, landing disappears, `h1` reads the project
name). This is a deliberate, expected behavior change from TASK-2, not a
regression to avoid: after this job, a connected app on `#/` should show
the dashboard, not the jobs view, and the hash should stay `#/`. Rewrite
the test's assertions to match (still asserting `.landing` is gone once
connected, still guarding against the original double-hash bug it was
written for — `navigate(href(...))` producing `##/p/manigot` — just via a
different expected end-state).
files: web-ui/src/App.test.ts
depends: TASK-2
risk: low — test-only, but the double-hash regression the test protects
  against must not be lost in the rewrite.

TASK-4: Add test coverage for the new behavior: a home-route dashboard test
in `App.test.ts` (dashboard renders at `#/`, connected + projects present)
and, if TASK-1 factors the cross-project aggregation (counts, attention
sort) into a plain function rather than inline component logic, a focused
unit test for that function alongside `stage.test.ts`'s pattern.
files: web-ui/src/App.test.ts; possibly a new
  web-ui/src/lib/dashboard.ts + web-ui/src/lib/dashboard.test.ts if TASK-1
  extracts pure aggregation logic (developer's call, based on how TASK-1
  turns out)
depends: TASK-1, TASK-2
risk: low

TASK-5: Update `web-ui/README.md`'s `Layout` section to list
`DashboardView` alongside the other views it already enumerates
(`JobsView`, `JobDetailView`, `AgentsView`, `HealthView`).
files: web-ui/README.md
depends: TASK-1
risk: low — documentation only.

## Out of scope (per brief)

- No daemon/API changes — no new aggregate endpoint. The dashboard's
  cross-project view is assembled client-side from existing endpoints.
- No new persistent dashboard state/settings (e.g. pinning/reordering
  widgets) — the brief asks for a default overview, not a customizable one.
