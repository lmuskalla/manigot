// Cross-project aggregation for the dashboard (DashboardView.svelte) — pure
// functions over the per-project job lists the view fans out for with
// Promise.allSettled, kept separate from the component so the counting and
// attention-sorting logic is unit-testable without rendering anything
// (mirrors stage.ts's own attention()/sortByAttention()).

import { attention, sortByAttention } from '$lib/stage'
import type { JobRow } from '$lib/api/types'

/** One project's job list — `jobs: null` marks a failed per-project fetch. */
export interface ProjectLoad {
  project: string
  jobs: JobRow[] | null
}

/** A job flattened out of its project, for the cross-project attention list. */
export interface DashboardJob extends JobRow {
  project: string
}

export interface DashboardCounts {
  /** Projects the dashboard has (attempted to) load jobs for. */
  projects: number
  openJobs: number
  needsHuman: number
  running: number
}

/** Aggregate counts across every successfully-loaded project. Failed loads
 * (`jobs: null`) are skipped, not counted as zero — a project that couldn't
 * load must not silently look "all clear". */
export function aggregateCounts(loads: ProjectLoad[]): DashboardCounts {
  let openJobs = 0
  let needsHuman = 0
  let running = 0
  for (const load of loads) {
    if (!load.jobs) continue
    for (const job of load.jobs) {
      if (job.status === 'open') openJobs++
      const level = attention(job).level
      if (level === 0) needsHuman++
      else if (level === 1) running++
    }
  }
  return { projects: loads.length, openJobs, needsHuman, running }
}

/** Every job across every successfully-loaded project, tagged with its
 * project and attention-sorted (needs-human, then running, then quiet). */
export function attentionJobs(loads: ProjectLoad[]): DashboardJob[] {
  const flat: DashboardJob[] = []
  for (const load of loads) {
    if (!load.jobs) continue
    for (const job of load.jobs) flat.push({ ...job, project: load.project })
  }
  return sortByAttention(flat)
}
