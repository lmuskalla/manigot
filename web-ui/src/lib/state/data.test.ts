// Data store — the projects list must surface load errors to the UI instead
// of failing silently into an empty dropdown (the brief: "if the response is
// an error, we need to indicate that to the user").

import { afterEach, describe, expect, it, vi } from 'vitest'

function stubFetch(handler: (url: string) => { status: number; body: unknown }) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const r = handler(String(input))
      return new Response(typeof r.body === 'string' ? r.body : JSON.stringify(r.body), {
        status: r.status,
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
}

async function freshData() {
  localStorage.clear()
  vi.resetModules()
  const { data } = await import('$lib/state/data.svelte')
  return data
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('data store', () => {
  it('loads projects and activates the first one', async () => {
    stubFetch(() => ({
      status: 200,
      body: { projects: [{ name: 'manigot', path: '/a' }, { name: 'other', path: '/b' }] },
    }))
    const data = await freshData()
    await data.loadProjects()
    expect(data.projects.map((p) => p.name)).toEqual(['manigot', 'other'])
    expect(data.active).toBe('manigot')
    expect(data.projectsError).toBe('')
  })

  it('records the error when /projects fails — no unhandled rejection, no silent empty', async () => {
    stubFetch(() => ({ status: 500, body: { error: 'registry exploded' } }))
    const data = await freshData()
    await expect(data.loadProjects()).resolves.toBeUndefined()
    expect(data.projects).toEqual([])
    expect(data.projectsError).toMatch(/registry exploded/)
  })

  it('clears a previous error on the next successful load', async () => {
    let fail = true
    stubFetch(() =>
      fail
        ? { status: 500, body: { error: 'boom' } }
        : { status: 200, body: { projects: [{ name: 'manigot', path: '/a' }] } },
    )
    const data = await freshData()
    await data.loadProjects()
    expect(data.projectsError).toMatch(/boom/)
    fail = false
    await data.loadProjects()
    expect(data.projectsError).toBe('')
    expect(data.active).toBe('manigot')
  })

  it('surfaces a 2xx non-JSON /projects response instead of a silent empty dropdown', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response('<!doctype html><html></html>', { status: 200 })),
    )
    const data = await freshData()
    await expect(data.loadProjects()).resolves.toBeUndefined()
    expect(data.projects).toEqual([])
    expect(data.projectsError).toMatch(/HTML/)
  })

  it('keeps the remembered project active when it still exists', async () => {
    const data = await freshData()
    localStorage.setItem('mg-control.project', 'other')
    stubFetch(() => ({
      status: 200,
      body: { projects: [{ name: 'manigot', path: '/a' }, { name: 'other', path: '/b' }] },
    }))
    await data.loadProjects()
    expect(data.active).toBe('other')
  })
})
