import { describe, expect, it } from 'vitest'
import { attention, sortByAttention, stageIndex } from '$lib/stage'
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

describe('stageIndex', () => {
  it('orders the pipeline', () => {
    expect(stageIndex('define')).toBe(0)
    expect(stageIndex('finished')).toBe(4)
  })
  it('clamps unknown stages', () => {
    expect(stageIndex('nonsense')).toBe(0)
  })
})

describe('attention', () => {
  it('ranks needs-human above running above quiet', () => {
    expect(attention(job({ jdi: { state: 'stopped:needs-human', agent: 'a', updated: '' } })).level).toBe(0)
    expect(attention(job({ jdi: { state: 'running', agent: 'a', updated: '' } })).level).toBe(1)
    expect(attention(job({})).level).toBe(2)
    expect(attention(job({ jdi: { state: 'stopped:finished', agent: 'a', updated: '' } })).level).toBe(2)
  })
})

describe('sortByAttention', () => {
  it('puts needs-human first without losing jobs', () => {
    const quiet = job({ id: 'quiet', name: 'quiet_x' })
    const running = job({ id: 'running', name: 'running_x', jdi: { state: 'running', agent: 'a', updated: '' } })
    const human = job({ id: 'human', name: 'human_x', jdi: { state: 'stopped:needs-human', agent: 'a', updated: '' } })
    const sorted = sortByAttention([quiet, running, human])
    expect(sorted.map((j) => j.id)).toEqual(['human', 'running', 'quiet'])
  })
})
