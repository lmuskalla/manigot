// App-level connection flows — the scenarios from the "connecting to the
// daemon" bug report: the dropdown must populate after a settings change
// (no reload), the landing must tell the truth about the connection state,
// and API errors must reach the user instead of an eternal "connecting".
//
// Each test builds fresh singletons (vi.resetModules) so the stores
// reconstruct from a controlled localStorage, exactly like a page load.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const html = (body = '') => `<!doctype html><html><head></head><body>${body}</body></html>`
const json = (v: unknown) => JSON.stringify(v)

const HEALTH = { version: '1.2.3', imagePresent: true, profiles: [] }
const PROJECTS = { projects: [{ name: 'manigot', path: '/srv/manigot' }] }

interface Stubbed {
  status: number
  body: string
  type?: string
}

/** A fetch stub dispatching on the URL — HTML for anything unconfigured. */
function stubFetch(handler: (url: string) => Stubbed) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const r = handler(String(input))
      return new Response(r.body, {
        status: r.status,
        headers: { 'Content-Type': r.type ?? 'application/json' },
      })
    }),
  )
}

/** Everything unproxied (same-origin) is HTML — the vite dev server fallback. */
const devServer: (urls: Record<string, Stubbed>) => (url: string) => Stubbed =
  (urls) =>
  (url) =>
    urls[url] ?? { status: 200, body: html(), type: 'text/html' }

/** Fresh singletons AND a matching render() from the same module registry —
 * mixing a statically-imported testing library with a reset-registry App
 * gives two Svelte runtimes and orphaned effects. The registry's own
 * cleanup() is stashed so afterEach can unmount what it rendered (the
 * setup file's static cleanup cannot see these components).
 */
async function freshApp() {
  vi.resetModules()
  const { render, screen, waitFor, fireEvent, cleanup } = await import('@testing-library/svelte')
  const { connection } = await import('$lib/state/connection.svelte')
  const { data } = await import('$lib/state/data.svelte')
  const { default: App } = await import('./App.svelte')
  cleanupFn = cleanup
  return { render, screen, waitFor, fireEvent, connection, data, App }
}

let cleanupFn: (() => void) | null = null

beforeEach(() => {
  localStorage.clear()
  location.hash = '#/'
})

afterEach(() => {
  cleanupFn?.()
  cleanupFn = null
  vi.unstubAllGlobals()
})

describe('App connection flows', () => {
  it('populates the project dropdown after connecting via settings, without a reload', async () => {
    stubFetch(
      devServer({
        '/api/health': { status: 200, body: json(HEALTH) },
        '/api/projects': { status: 200, body: json(PROJECTS) },
      }),
    )
    const { render, screen, waitFor, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('down'))

    // the user opens the settings modal and points the connection at /api
    connection.set('/api', '')
    await waitFor(() => expect(connection.status).toBe('up'))

    const select = await waitFor(() => screen.getByLabelText('project') as HTMLSelectElement)
    await waitFor(() => {
      expect(select.options.length).toBe(1)
      expect(select.options[0]?.textContent).toBe('manigot')
    })
  })

  it('does not claim "Connecting to the daemon" once the check has failed', async () => {
    stubFetch(devServer({}))
    const { render, screen, waitFor, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('down'))
    await waitFor(() => {
      const landing = document.querySelector('.landing')
      expect(landing).toBeTruthy()
      expect(landing!.textContent).not.toMatch(/Connecting to the daemon/)
    })
  })

  it('says no projects are registered when a reachable daemon has none', async () => {
    stubFetch(
      devServer({
        '/health': { status: 200, body: json(HEALTH) },
        '/projects': { status: 200, body: json({ projects: [] }) },
      }),
    )
    const { render, screen, waitFor, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('up'))
    await waitFor(() => {
      const landing = document.querySelector('.landing')
      expect(landing?.textContent).toMatch(/No projects registered/)
    })
  })

  it('stops fetching /projects once an empty registry has loaded — no reload loop', async () => {
    let projectsCalls = 0
    stubFetch((url) => {
      if (url.includes('/projects')) {
        projectsCalls++
        return { status: 200, body: json({ projects: [] }) }
      }
      if (url.endsWith('/health')) return { status: 200, body: json(HEALTH) }
      return { status: 200, body: html(), type: 'text/html' }
    })
    const { render, waitFor, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('up'))
    await waitFor(() => expect(projectsCalls).toBeGreaterThan(0))
    await new Promise((r) => setTimeout(r, 150))
    const settled = projectsCalls
    await new Promise((r) => setTimeout(r, 150))
    // Regression: keying the reload on "list is empty" refetched forever
    // (and OOMed the tab) whenever the daemon had zero projects.
    expect(projectsCalls).toBe(settled)
  })

  it('shows the projects error instead of an empty dropdown when /projects fails', async () => {
    stubFetch(
      devServer({
        '/health': { status: 200, body: json(HEALTH) },
        '/projects': { status: 500, body: json({ error: 'registry exploded' }) },
      }),
    )
    const { render, screen, waitFor, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('up'))
    await waitFor(() => {
      const landing = document.querySelector('.landing')
      expect(landing?.textContent).toMatch(/registry exploded/)
    })
  })

  it('leaves the landing once a project is active — home shows the dashboard, not a redirect', async () => {
    // Home used to auto-redirect to the active project's jobs view; it now
    // renders the dashboard in place, so the hash must stay '#/' once
    // connected instead of jumping to '#/p/manigot'.
    stubFetch(
      devServer({
        '/api/health': { status: 200, body: json(HEALTH) },
        '/api/projects': { status: 200, body: json(PROJECTS) },
        '/api/projects/manigot/jobs': { status: 200, body: json({ jobs: [] }) },
      }),
    )
    const { render, screen, waitFor, fireEvent, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('down'))

    connection.set('/api', '')
    await waitFor(() => expect(connection.status).toBe('up'))
    await waitFor(() => expect(screen.getByLabelText('project')).toBeTruthy())

    // The landing must be gone, and the hash must stay '#/' — no redirect.
    await waitFor(() => {
      expect(location.hash).toBe('#/')
      expect(document.querySelector('.landing')).toBeNull()
      expect(document.querySelector('h1')?.textContent).toBe('Dashboard')
    })

    // Regression guard: switching projects still goes through
    // navigate(href(...)) — it must not double-prefix the fragment
    // ('##/p/manigot'), which parseHash cannot parse.
    const select = screen.getByLabelText('project') as HTMLSelectElement
    await fireEvent.change(select, { target: { value: 'manigot' } })
    await waitFor(() => {
      expect(location.hash).toBe('#/p/manigot')
      expect(document.querySelector('h1')?.textContent).toBe('manigot')
    })
  })

  it('renders the dashboard at #/ with cross-project counts and an attention list', async () => {
    const jobs = {
      jobs: [
        { id: 'a', name: 'a_x', status: 'open', stage: 'implement', type: 'feature', date: '2026-08-28', title: 'quiet job', jdi: null },
        {
          id: 'b',
          name: 'b_y',
          status: 'open',
          stage: 'review',
          type: 'fix',
          date: '2026-08-27',
          title: 'stuck job',
          jdi: { state: 'stopped:needs-human', agent: 'reviewer', updated: '2026-08-29T00:00:00Z' },
        },
      ],
    }
    stubFetch(
      devServer({
        '/api/health': { status: 200, body: json(HEALTH) },
        '/api/projects': { status: 200, body: json(PROJECTS) },
        '/api/projects/manigot/jobs': { status: 200, body: json(jobs) },
      }),
    )
    const { render, screen, waitFor, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('down'))

    connection.set('/api', '')
    await waitFor(() => expect(connection.status).toBe('up'))

    await waitFor(() => {
      expect(document.querySelector('h1')?.textContent).toBe('Dashboard')
      expect(screen.getByText('stuck job')).toBeTruthy()
      expect(screen.getByText('manigot', { selector: '.proj-name' })).toBeTruthy()
    })
  })

  it('renders the daemon health panel on #/health when /health answers', async () => {
    localStorage.setItem('mg-control.connection', JSON.stringify({ baseUrl: '/api', token: '' }))
    location.hash = '#/health'
    stubFetch(
      devServer({
        '/api/health': { status: 200, body: json(HEALTH) },
      }),
    )
    const { render, screen, waitFor, connection, App } = await freshApp()
    render(App)
    await waitFor(() => expect(connection.status).toBe('up'))
    await waitFor(() => expect(screen.getByText('1.2.3')).toBeTruthy())
  })
})
