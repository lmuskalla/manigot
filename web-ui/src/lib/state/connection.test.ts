// Connection store — the status machine behind every "connecting" surface.
// The bug report's symptoms pin three behaviors: concurrent checks must not
// race (the settings modal, a mounting view, and the 30s re-check all call
// check()), a live connection must not flash 'connecting' on every
// re-validation, and a reconfiguration must never be answered by an older
// connection's in-flight response.

import { afterEach, describe, expect, it, vi } from 'vitest'

const HEALTH = JSON.stringify({ version: '1.2.3', imagePresent: true, profiles: [] })
const OTHER_HEALTH = JSON.stringify({ version: '9.9.9', imagePresent: false, profiles: [] })

function ok(body = HEALTH) {
  return new Response(body, { status: 200, headers: { 'Content-Type': 'application/json' } })
}

function deferredFetch() {
  const pending = new Map<string, { resolve: (r: Response) => void; reject: (e: unknown) => void }>()
  const fetches: string[] = []
  const flush = (url: string, r: Response) => pending.get(url)?.resolve(r)
  const fail = (url: string, e: unknown) => pending.get(url)?.reject(e)
  const stub = vi.fn(
    (input: RequestInfo | URL) =>
      new Promise<Response>((resolve, reject) => {
        const url = String(input)
        fetches.push(url)
        pending.set(url, { resolve, reject })
      }),
  )
  vi.stubGlobal('fetch', stub)
  return { stub, fetches, flush, fail }
}

/** Fresh singletons — the store builds itself from localStorage on import. */
async function fresh(persisted?: { baseUrl?: string; token?: string }) {
  localStorage.clear()
  if (persisted) localStorage.setItem('mg-control.connection', JSON.stringify(persisted))
  vi.resetModules()
  return await import('$lib/state/connection.svelte')
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('connection store', () => {
  it('goes connecting → up and captures health', async () => {
    const { connection } = await fresh()
    const { flush } = deferredFetch()
    const p = connection.check()
    expect(connection.status).toBe('connecting')
    flush('/health', ok())
    expect(await p).toBe(true)
    expect(connection.status).toBe('up')
    expect(connection.health?.version).toBe('1.2.3')
    expect(connection.lastError).toBe('')
  })

  it('marks down with the error when the daemon cannot be reached', async () => {
    const { connection } = await fresh()
    const { fail } = deferredFetch()
    const p = connection.check()
    fail('/health', new TypeError('Failed to fetch'))
    expect(await p).toBe(false)
    expect(connection.status).toBe('down')
    expect(connection.lastError).toMatch(/cannot reach the daemon/)
  })

  it('keeps a live connection "up" while re-validating — no connecting flash', async () => {
    const { connection } = await fresh()
    const { flush } = deferredFetch()
    const first = connection.check()
    flush('/health', ok())
    await first
    expect(connection.status).toBe('up')

    // the 30s re-check starts; while it is in flight the UI must stay 'up'
    const p = connection.check()
    expect(connection.status).toBe('up')
    flush('/health', ok())
    expect(await p).toBe(true)
    expect(connection.status).toBe('up')
  })

  it('shares one in-flight check between concurrent callers', async () => {
    const { connection } = await fresh()
    const { stub, flush } = deferredFetch()
    const a = connection.check()
    const b = connection.check()
    expect(stub).toHaveBeenCalledTimes(1)
    flush('/health', ok())
    expect(await a).toBe(true)
    expect(await b).toBe(true)
  })

  it('never applies a stale response after set() re-points the connection', async () => {
    const { connection } = await fresh({ baseUrl: '/old' })
    const { flush } = deferredFetch()

    const old = connection.check() // against /old/health — pending
    connection.set('/api', '') // re-point while the old check is in flight

    // the old daemon answers late — it must not establish anything
    flush('/old/health', ok(OTHER_HEALTH))
    expect(await old).toBe(false)
    expect(connection.status).not.toBe('up')
    expect(connection.health).toBe(null)

    // the new daemon answers — that one counts
    flush('/api/health', ok())
    await waitForUp(connection)
    expect(connection.health?.version).toBe('1.2.3')
  })

  it('re-establishes visibly after set(): status resets to connecting', async () => {
    const { connection } = await fresh()
    const { flush } = deferredFetch()
    const first = connection.check()
    flush('/health', ok())
    await first
    expect(connection.status).toBe('up')

    connection.set('/api', '')
    expect(connection.status).toBe('connecting')
    expect(connection.health).toBe(null)
    flush('/api/health', ok())
    await waitForUp(connection)
    expect(connection.status).toBe('up')
  })

  it('bumps established once per transition into up — not on re-validations', async () => {
    const { connection } = await fresh()
    const { stub, flush } = deferredFetch()

    const first = connection.check()
    flush('/health', ok())
    await first
    expect(connection.established).toBe(1)

    // the 30s re-check succeeds — still up, no new establishment
    const revalidate = connection.check()
    flush('/health', ok())
    await revalidate
    expect(connection.established).toBe(1)

    // the daemon dies and comes back — a real transition, a new bump
    const died = connection.check()
    flush('/health', new Response(null, { status: 503 }))
    await died
    expect(connection.status).toBe('down')
    expect(connection.established).toBe(1)

    const back = connection.check()
    flush('/health', ok())
    await back
    expect(connection.status).toBe('up')
    expect(connection.established).toBe(2)
    expect(stub).toHaveBeenCalledTimes(4)
  })
})

async function waitForUp(connection: { status: string }) {
  for (let i = 0; i < 50 && connection.status !== 'up'; i++) {
    await new Promise((r) => setTimeout(r, 10))
  }
  expect(connection.status).toBe('up')
}
