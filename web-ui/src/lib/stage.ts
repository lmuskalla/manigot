// The stage model — the pipeline every job travels: define → plan →
// implement → review → finished (internal/job/stage.go). The same vocabulary
// drives the metro-line signature element.

import type { JobRow } from '$lib/api/types'

export const STAGES = ['define', 'plan', 'implement', 'review', 'finished'] as const
export type Stage = (typeof STAGES)[number]

/** The agents that own each stage — informational, never a launch gate. */
export const STAGE_AGENTS: Record<Stage, string[]> = {
  define: ['owner'],
  plan: ['analyst'],
  implement: ['developer'],
  review: ['reviewer', 'security'],
  finished: [],
}

export function stageIndex(stage: string): number {
  const i = STAGES.indexOf(stage as Stage)
  return i < 0 ? 0 : i
}

export interface JobAttention {
  /** Jobs needing a human outrank everything; running next; rest keep order. */
  level: 0 | 1 | 2
  label: '' | 'needs human' | 'running'
}

export function attention(job: JobRow): JobAttention {
  if (job.jdi?.state === 'stopped:needs-human') return { level: 0, label: 'needs human' }
  if (job.jdi?.state === 'running') return { level: 1, label: 'running' }
  return { level: 2, label: '' }
}

/** Sort: needs-human first, then running, then discovery order (newest first). */
export function sortByAttention<T extends JobRow>(jobs: T[]): T[] {
  return [...jobs].sort((a, b) => attention(a).level - attention(b).level)
}
