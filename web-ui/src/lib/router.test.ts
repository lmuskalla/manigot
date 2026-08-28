import { describe, expect, it } from 'vitest'
import { href, parseHash } from '$lib/router'

describe('parseHash', () => {
  it('parses the jobs route', () => {
    expect(parseHash('#/p/manigot')).toEqual({ name: 'jobs', project: 'manigot' })
  })

  it('parses the job detail route with default tab', () => {
    expect(parseHash('#/p/manigot/j/farmer_part-2')).toEqual({
      name: 'job',
      project: 'manigot',
      job: 'farmer_part-2',
      tab: 'brief',
    })
  })

  it('parses an explicit tab', () => {
    expect(parseHash('#/p/manigot/j/farmer_part-2/run')).toMatchObject({ tab: 'run' })
  })

  it('parses agents and health', () => {
    expect(parseHash('#/p/manigot/agents')).toEqual({ name: 'agents', project: 'manigot' })
    expect(parseHash('#/health')).toEqual({ name: 'health' })
  })

  it('falls home on unknown paths', () => {
    expect(parseHash('#/')).toEqual({ name: 'home' })
    expect(parseHash('')).toEqual({ name: 'home' })
  })

  it('decodes URI components in names', () => {
    expect(parseHash('#/p/p%201/j/j_x')).toMatchObject({ project: 'p 1' })
  })
})

describe('href', () => {
  it('round-trips with parseHash', () => {
    for (const r of [
      { name: 'jobs', project: 'manigot' },
      { name: 'job', project: 'manigot', job: 'farmer_x', tab: 'diff' },
      { name: 'agents', project: 'manigot' },
      { name: 'health' },
    ] as const) {
      expect(parseHash(href(r as never))).toMatchObject(r)
    }
  })

  it('omits the default tab for clean job URLs', () => {
    expect(href({ name: 'job', project: 'p', job: 'j', tab: 'brief' })).toBe('#/p/p/j/j')
    expect(href({ name: 'job', project: 'p', job: 'j', tab: 'run' })).toBe('#/p/p/j/j/run')
  })
})
