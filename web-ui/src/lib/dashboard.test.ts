import { describe, expect, it } from 'vitest'
import { aggregateCounts, attentionJobs, type ProjectLoad } from '$lib/dashboard'
import type { JobRow } from '$lib/api/types'

const job = (over: Partial<JobRow>): JobRow => ({
  id: 'x',
  name: 'x_job',
  status: 'open',
  stage: 'implement',
  type: 'feature',
  date: '2026-08-28',
  title: 't',
  jdi: null,
  ...over,
})

describe('aggregateCounts', () => {
  it('counts open jobs, needs-human and running across every loaded project', () => {
    const loads: ProjectLoad[] = [
      {
        project: 'a',
        jobs: [
          job({ id: '1', name: '1_x', status: 'open' }),
          job({
            id: '2',
            name: '2_x',
            status: 'open',
            jdi: { state: 'stopped:needs-human', agent: 'reviewer', updated: '' },
          }),
        ],
      },
      {
        project: 'b',
        jobs: [
          job({ id: '3', name: '3_x', status: 'done' }),
          job({ id: '4', name: '4_x', status: 'open', jdi: { state: 'running', agent: 'developer', updated: '' } }),
        ],
      },
    ]
    expect(aggregateCounts(loads)).toEqual({ projects: 2, openJobs: 3, needsHuman: 1, running: 1 })
  })

  it('skips failed loads instead of counting them as zero', () => {
    const loads: ProjectLoad[] = [
      { project: 'a', jobs: null },
      { project: 'b', jobs: [job({ status: 'open' })] },
    ]
    const counts = aggregateCounts(loads)
    expect(counts.projects).toBe(2)
    expect(counts.openJobs).toBe(1)
  })
})

describe('attentionJobs', () => {
  it('flattens every project, tags each job with its project, and sorts by attention', () => {
    const loads: ProjectLoad[] = [
      {
        project: 'a',
        jobs: [job({ id: 'quiet', name: 'quiet_x' })],
      },
      {
        project: 'b',
        jobs: [
          job({ id: 'running', name: 'running_x', jdi: { state: 'running', agent: 'a', updated: '' } }),
          job({ id: 'human', name: 'human_x', jdi: { state: 'stopped:needs-human', agent: 'a', updated: '' } }),
        ],
      },
    ]
    const sorted = attentionJobs(loads)
    expect(sorted.map((j) => [j.project, j.id])).toEqual([
      ['b', 'human'],
      ['b', 'running'],
      ['a', 'quiet'],
    ])
  })

  it('skips failed loads', () => {
    const loads: ProjectLoad[] = [
      { project: 'a', jobs: null },
      { project: 'b', jobs: [job({ id: 'x', name: 'x_x' })] },
    ]
    expect(attentionJobs(loads).map((j) => j.project)).toEqual(['b'])
  })
})
