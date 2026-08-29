// Connection state — where the daemon lives and how to authenticate.
// Persisted to localStorage; the same settings survive reloads. The token is
// only ever sent as a header (or SSE query param), never logged, never
// rendered back in full.

import * as api from '$lib/api/client'
import type { Health } from '$lib/api/types'

const LS_KEY = 'mg-control.connection'

export interface PersistedConnection {
  baseUrl: string
  token: string
}

function load(): PersistedConnection {
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (raw) {
      const v = JSON.parse(raw) as PersistedConnection
      return { baseUrl: v.baseUrl ?? '', token: v.token ?? '' }
    }
  } catch {
    /* fresh start */
  }
  return { baseUrl: '', token: '' }
}

function save(c: PersistedConnection) {
  localStorage.setItem(LS_KEY, JSON.stringify(c))
}

class ConnectionStore {
  baseUrl = $state('')
  token = $state('')
  health = $state<Health | null>(null)
  /** 'connecting' until the first /health answers; then 'up' | 'down'. */
  status = $state<'connecting' | 'up' | 'down'>('connecting')
  lastError = $state('')
  /** Bumped on every transition into 'up' — first boot, a settings change,
   *  the daemon coming back. The edge data loads key off: re-validations
   *  that stay up must not retrigger them (an empty project list reloaded
   *  on every check would spin forever). */
  established = $state(0)

  /** Bumped by set() — in-flight checks from a previous connection are stale. */
  #gen = 0
  #inFlight: { gen: number; url: string; token: string; p: Promise<boolean> } | null = null

  constructor() {
    const persisted = load()
    this.baseUrl = persisted.baseUrl
    this.token = persisted.token
    // ?api=<url> overrides the persisted base URL for this session — a
    // deep-link/testing convenience (tokens stay out of URLs by design).
    if (typeof location !== 'undefined') {
      const api = new URLSearchParams(location.search).get('api')
      if (api) this.baseUrl = api
    }
    this.apply()
  }

  private apply() {
    api.setConnection({ baseUrl: this.baseUrl, token: this.token })
  }

  set(baseUrl: string, token: string) {
    this.baseUrl = baseUrl
    this.token = token
    save({ baseUrl, token })
    // A deliberate reconfiguration re-establishes from scratch: drop the
    // previous daemon's state so the check against the new URL runs visibly
    // and cannot be answered by anything stale.
    this.health = null
    this.status = 'connecting'
    this.lastError = ''
    this.#gen++
    this.apply()
    void this.check()
  }

  async check(): Promise<boolean> {
    // Concurrent callers (the 30s re-check, a mounting view, the settings
    // modal) share one in-flight request per connection instead of racing.
    const gen = this.#gen
    const { baseUrl, token } = api.getConnection()
    if (
      this.#inFlight &&
      this.#inFlight.gen === gen &&
      this.#inFlight.url === baseUrl &&
      this.#inFlight.token === token
    ) {
      return this.#inFlight.p
    }
    const p = this.#probe(gen)
    this.#inFlight = { gen, url: baseUrl, token, p }
    try {
      return await p
    } finally {
      if (this.#inFlight?.p === p) this.#inFlight = null
    }
  }

  async #probe(gen: number): Promise<boolean> {
    // Only the first establishment (and a deliberate set()) shows
    // 'connecting'; re-checks of a known connection keep the current status
    // while in flight, so a live UI never flashes "connecting" on every
    // 30s re-validation.
    if (!this.health) this.status = 'connecting'
    try {
      const health = await api.getHealth()
      if (gen !== this.#gen) return false // superseded by a newer connection
      if (this.status !== 'up') this.established++
      this.health = health
      this.status = 'up'
      this.lastError = ''
      return true
    } catch (e) {
      if (gen !== this.#gen) return false
      this.health = null
      this.status = 'down'
      this.lastError = e instanceof Error ? e.message : String(e)
      return false
    }
  }
}

export const connection = new ConnectionStore()
