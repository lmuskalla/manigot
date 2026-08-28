// The mock daemon is also a test fixture — these tests pin its routes so the
// dev/verification environment can't silently drift from the real API shapes.

import { beforeEach, describe, expect, it } from 'vitest'
import { fixtureState, installMock, mockDaemon } from '$lib/api/mock'
import * as client from '$lib/api/client'

beforeEach(() => {
  installMock()
  mockDaemon.reset()
  client.setConnection({ baseUrl: '', token: '' })
})

describe('mock daemon reads (Part 1 shapes)', () => {
  it('serves health', async () => {
    const h = await client.getHealth()
    expect(h.version).toBeTruthy()
    expect(h.profiles?.length).toBeGreaterThan(2)
  })

  it('serves projects and jobs with the row shape', async () => {
    const { projects } = await client.getProjects()
    expect(projects.length).toBe(2)
    const { jobs } = await client.getJobs('manigot')
    expect(jobs.length).toBeGreaterThan(3)
    expect(jobs[0]).toHaveProperty('name')
    expect(jobs[0]).toHaveProperty('stage')
    expect(jobs[0]).toHaveProperty('jdi')
  })

  it('serves job files as raw markdown', async () => {
    const brief = await client.getJobFile('manigot', 'farmer_part-2-of-web-ui-tui-path', 'brief')
    expect(brief).toMatch(/^# Brief:/)
  })

  it('404s a missing file', async () => {
    await expect(client.getJobFile('manigot', 'farmer_part-2-of-web-ui-tui-path', 'verdict')).rejects.toMatchObject({
      status: 404,
    })
  })

  it('serves jdi status with both log tails', async () => {
    const res = await client.getJobJdi('manigot', 'farmer_part-2-of-web-ui-tui-path')
    expect(res.status?.state).toBe('running')
    expect(res.runLog).toContain('mg jdi started')
    expect(res.sessionLog).toBeTruthy()
  })

  it('serves the quick diff and the full patch', async () => {
    const quick = await client.getJobDiff('manigot', 'farmer')
    expect(quick.log).toContain('TASK-7')
    expect(quick.stat).toContain('files changed')
    const full = await client.getJobDiff('manigot', 'farmer', true)
    expect(full.patch).toContain('diff --git')
  })

  it('serves agents for a registered project only', async () => {
    const { agents } = await client.getAgents('manigot')
    expect(agents.length).toBeGreaterThanOrEqual(14)
    await expect(client.getAgents('nope')).rejects.toMatchObject({ status: 404 })
  })
})

describe('mock daemon mutations (Part 2 shapes)', () => {
  it('creates a job that then appears in the list', async () => {
    await client.createJob('manigot', 'Docs sweep', 'chore')
    const { jobs } = await client.getJobs('manigot')
    expect(jobs.some((j) => j.title === 'Docs sweep')).toBe(true)
  })

  it('replaces the brief on PUT', async () => {
    await client.saveBrief('manigot', 'farmer_part-2-of-web-ui-tui-path', '# Brief: rewritten\n')
    const brief = await client.getJobFile('manigot', 'farmer_part-2-of-web-ui-tui-path', 'brief')
    expect(brief).toBe('# Brief: rewritten\n')
  })

  it('starts jdi and flips the run state', async () => {
    await client.startJdi('manigot', 'quartz_prune-on-a-timer')
    const { jobs } = await client.getJobs('manigot')
    const quartz = jobs.find((j) => j.id === 'quartz')!
    expect(quartz.jdi?.state).toBe('running')
  })

  it('launches a one-shot agent', async () => {
    const res = await client.launchAgent('manigot', 'farmer', 'quality')
    expect(res.started).toBe(true)
    const log = await client.getJobJdi('manigot', 'farmer_part-2-of-web-ui-tui-path')
    expect(log.runLog).toContain('quality invoked')
  })

  it('done and delete remove the job', async () => {
    await client.doneJob('manigot', 'lumen_doctor-health-check')
    let { jobs } = await client.getJobs('manigot')
    expect(jobs.some((j) => j.id === 'lumen')).toBe(false)
    await client.deleteJob('manigot', 'ember_tui-bell-volume')
    ;({ jobs } = await client.getJobs('manigot'))
    expect(jobs.some((j) => j.id === 'ember')).toBe(false)
  })

  it('lists and removes orphans', async () => {
    const before = await client.getOrphans('manigot')
    expect(before.orphans?.length).toBeGreaterThan(0)
    await client.removeOrphans('manigot')
    const after = await client.getOrphans('manigot')
    expect(after.orphans).toHaveLength(0)
  })

  it('prunes containers', async () => {
    const res = await client.pruneContainers()
    expect(res).toHaveProperty('removed')
    expect(res).toHaveProperty('running')
  })
})

describe('fixture state', () => {
  it('covers every stage and jdi state', () => {
    const jobs = fixtureState().jobs['manigot']!
    const stages = new Set(jobs.map((j) => j.stage))
    for (const s of ['define', 'plan', 'implement', 'review', 'finished']) {
      expect(stages).toContain(s)
    }
    const states = new Set(jobs.map((j) => j.jdi?.state))
    expect(states).toContain('running')
    expect(states).toContain('stopped:needs-human')
    expect(states).toContain('stopped:finished')
  })
})
