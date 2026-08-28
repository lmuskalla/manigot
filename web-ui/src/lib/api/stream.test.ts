import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { watchSessionLog } from '$lib/api/stream'
import { installMock, mockDaemon } from '$lib/api/mock'
import * as client from '$lib/api/client'

describe('watchSessionLog', () => {
  beforeEach(() => {
    installMock()
    mockDaemon.reset()
    client.setConnection({ baseUrl: '', token: '' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('falls back to polling when SSE cannot connect', async () => {
    // Break EventSource entirely — the 404-on-Part-1 path, just faster.
    const original = globalThis.EventSource
    // @ts-expect-error — deliberately removing the global
    delete globalThis.EventSource

    const updates: { mode: string; text: string }[] = []
    const handle = watchSessionLog('manigot', 'farmer_part-2-of-web-ui-tui-path', (s) =>
      updates.push({ mode: s.mode, text: s.text }),
    )
    // The first poll fires immediately.
    await new Promise((r) => setTimeout(r, 50))
    expect(updates.length).toBeGreaterThan(0)
    expect(updates[0].mode).toBe('polling')
    expect(updates.some((u) => u.text.includes('developer session started'))).toBe(true)
    handle.stop()

    globalThis.EventSource = original
  })

  it('stop() halts updates', async () => {
    const original = globalThis.EventSource
    // @ts-expect-error — deliberately removing the global
    delete globalThis.EventSource
    const handle = watchSessionLog('manigot', 'farmer_part-2-of-web-ui-tui-path', () => {})
    handle.stop()
    await new Promise((r) => setTimeout(r, 30))
    globalThis.EventSource = original
  })
})
