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
    this.apply()
    void this.check()
  }

  async check(): Promise<boolean> {
    this.status = 'connecting'
    try {
      this.health = await api.getHealth()
      this.status = 'up'
      this.lastError = ''
      return true
    } catch (e) {
      this.status = 'down'
      this.lastError = e instanceof Error ? e.message : String(e)
      return false
    }
  }
}

export const connection = new ConnectionStore()
