import { afterEach, describe, expect, it, vi } from 'vitest'
import * as client from '$lib/api/client'

function stubFetch(handler: (url: string, init?: RequestInit) => { status: number; body: unknown; contentType?: string }) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    const r = handler(url, init)
    return new Response(r.body === undefined ? '' : typeof r.body === 'string' ? r.body : JSON.stringify(r.body), {
      status: r.status,
      headers: { 'Content-Type': r.contentType ?? 'application/json' },
    })
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
  client.setConnection({ baseUrl: 'http://mg.test', token: '' })
})

describe('client', () => {
  it('sends the bearer token when configured', async () => {
    const f = stubFetch(() => ({ status: 200, body: { version: '1' } }))
    vi.stubGlobal('fetch', f)
    client.setConnection({ baseUrl: 'http://mg.test', token: 'sekret' })
    await client.getHealth()
    expect(f).toHaveBeenCalledWith(
      'http://mg.test/health',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer sekret' }),
      }),
    )
  })

  it('parses JSON responses', async () => {
    vi.stubGlobal(
      'fetch',
      stubFetch(() => ({ status: 200, body: { projects: [{ name: 'manigot', path: '/x' }] } })),
    )
    const res = await client.getProjects()
    expect(res.projects[0].name).toBe('manigot')
  })

  it('surfaces the error envelope', async () => {
    vi.stubGlobal('fetch', stubFetch(() => ({ status: 404, body: { error: 'project not found' } })))
    await expect(client.getProjects()).rejects.toMatchObject({ status: 404, message: 'project not found' })
  })

  it('maps 404 to a capability miss', async () => {
    vi.stubGlobal('fetch', stubFetch(() => ({ status: 404, body: { error: 'no route' } })))
    const err = await client.pruneContainers().catch((e) => e)
    expect(err.capabilityMiss).toBe(true)
  })

  it('explains 401 as a token problem', async () => {
    vi.stubGlobal('fetch', stubFetch(() => ({ status: 401, body: { error: 'unauthorized' } })))
    await expect(client.getHealth()).rejects.toThrow(/token/i)
  })

  it('wraps network failure with the base URL', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch')
      }),
    )
    await expect(client.getHealth()).rejects.toThrow(/cannot reach the daemon/)
  })

  it('returns raw text for job files', async () => {
    vi.stubGlobal(
      'fetch',
      stubFetch(() => ({ status: 200, body: '# Brief: x\n', contentType: 'text/markdown' })),
    )
    const text = await client.getJobFile('p', 'j', 'brief')
    expect(text).toBe('# Brief: x\n')
  })

  it('encodes URL segments', async () => {
    const f = stubFetch(() => ({ status: 200, body: { jobs: [] } }))
    vi.stubGlobal('fetch', f)
    await client.getJobs('my project')
    expect(f.mock.calls[0][0]).toBe('http://mg.test/projects/my%20project/jobs')
  })
})

describe('normalizeBaseUrl', () => {
  it('strips trailing slashes and adds http://', () => {
    expect(client.normalizeBaseUrl('127.0.0.1:8080///')).toBe('http://127.0.0.1:8080')
    expect(client.normalizeBaseUrl('https://vps.example.com')).toBe('https://vps.example.com')
    expect(client.normalizeBaseUrl('/api')).toBe('/api')
    expect(client.normalizeBaseUrl('  ')).toBe('')
  })
})
