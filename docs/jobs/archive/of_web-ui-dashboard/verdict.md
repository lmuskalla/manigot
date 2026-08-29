# Verdict: web ui: dashboard

id: of
status: open
reviewer: claude
date: 2026-08-29

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `web-ui/src/lib/views/DashboardView.svelte` + `web-ui/src/lib/dashboard.ts`
match the spec — projects grid with `href({name:'jobs', project})` links,
aggregate counts (`aggregateCounts`) and an attention-sorted cross-project
job list (`attentionJobs`, built on `stage.ts`'s `attention()`/
`sortByAttention()`), data fanned out via `Promise.allSettled` with
per-project failures degrading to an inline "couldn't load jobs for X" note
(not a page-level error) and failed loads correctly excluded from counts
rather than counted as zero (`dashboard.test.ts` covers this). No polling
beyond the initial `onMount` load, as the task called for. Styling follows
existing view conventions (`.page`, `--r-*`/`--ink-*`/`--bg-*` tokens,
`EmptyState` reuse). Verified against `web-ui/src/lib/api/types.ts` and
`client.ts` — `getProjects`/`getJobs` signatures and `JobRow`/`ProjectRow`
shapes all line up.

TASK-2: PASS
notes: `App.svelte` renders `DashboardView` for
`route.name === 'home' && connection.status === 'up' && !data.projectsError
&& data.projects.length > 0`, and the old `$effect` that force-navigated
`home` → `jobs/<active>` is removed. The existing down/no-projects/
connecting/error branches are untouched in the trailing `{:else}`. Minor
pre-existing (not introduced by this task) UX rough edge: during the brief
window after `connection.status` flips to `up` but before `loadProjects()`
resolves, the landing's "No projects registered" branch can flash before the
dashboard appears — this is the same gap the old code had before its
redirect fired, not a regression, and not something TASK-2 was scoped to
fix.

TASK-3: PASS
notes: `App.test.ts`'s rewritten test asserts the dashboard renders at `#/`
(hash stays `#/`, `.landing` gone, `h1` reads "Dashboard") and preserves the
original double-hash regression guard by driving a project switch via the
`<select>` and confirming the hash lands as `#/p/manigot`, not `##/p/manigot`.

TASK-4: PASS
notes: New `App.test.ts` case renders the dashboard with two projects/mixed
job states and asserts the attention list + projects grid content. New
`web-ui/src/lib/dashboard.test.ts` unit-tests `aggregateCounts()` and
`attentionJobs()` (counting, skipping failed loads, sort order, project
tagging), following `stage.test.ts`'s pattern.

TASK-5: PASS
notes: `web-ui/README.md`'s `Layout` section now lists `DashboardView`
alongside the other views.

## Verification performed

- `npm test` (vitest run): 93/93 tests pass, including the new
  `dashboard.test.ts` (4 tests) and the two new/rewritten `App.test.ts`
  cases.
- `npm run build`: clean production build, no errors.
- Reviewed the committed render report/screenshots
  (`screenshots/render-report.md`, 375px + 1280px PNGs). The dashboard
  renders correctly at both widths — stats, attention list, projects grid.
  The report's flagged findings are non-issues: the "overflow" entries are
  intentional `text-overflow: ellipsis` truncation on long titles/names (by
  design, matches the rest of the app), and the ".page × nav.tabbar" overlap
  entries are a known artifact of the render tool measuring full (taller
  than viewport) scrollable content against a viewport-fixed mobile tab bar
  — confirmed benign by inspecting the actual 375px screenshot, which shows
  no visible overlap.
- Diffed the full job branch against `main`: no changes outside the task
  list's stated files (`web-ui/src/App.svelte`, `App.test.ts`,
  `web-ui/src/lib/dashboard.ts`/`dashboard.test.ts`,
  `web-ui/src/lib/views/DashboardView.svelte`, `web-ui/README.md`) plus the
  job's own docs/screenshots/log — no scope creep, no unrelated refactors.

## Security

None — frontend-only change, no new endpoints, no new data exposure (the
dashboard only aggregates data the existing per-project `getJobs`/
`getProjects` calls already expose to the logged-in daemon client).

## Overall

APPROVED

No blockers. All five tasks are implemented as specified in `tasks.md`,
tests pass, the build is clean, and the diff stays within scope.
