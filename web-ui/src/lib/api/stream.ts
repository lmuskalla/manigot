// Live run supervision — the reader side of job two.
//
// The daemon streams `session.log` growth (the same file the TUI's `l` key
// tails) over SSE. EventSource cannot send an Authorization header, so the
// token rides as a query parameter when one is set (the daemon treats it as
// equivalent — a Part 2 decision documented in the endpoint map).
//
// Resilience contract: if the stream cannot be established (404 on a Part 1
// daemon, network drop, daemon restart), this client falls back to polling
// the /jdi endpoint and never throws upward — live output degrades to
// 2-second snapshots instead of breaking the view.

import { getConnection } from './client'
import { ep, seg } from './endpoints'
import { getJobJdi } from './client'
import type { JobJdiResponse } from './types'

export interface StreamState {
  /** 'streaming' = SSE attached; 'polling' = fallback; 'idle' = not started. */
  mode: 'streaming' | 'polling' | 'idle'
  /** Raw session.log text as received so far (the server sends tails, we keep the latest). */
  text: string
}

export interface StreamHandle {
  stop(): void
}

const POLL_MS = 2000

export function watchSessionLog(
  project: string,
  job: string,
  onUpdate: (state: StreamState) => void,
): StreamHandle {
  let stopped = false
  let es: EventSource | null = null
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let text = ''

  const { baseUrl, token } = getConnection()
  const url =
    (baseUrl || '') +
    ep.sessionStream(seg(project), seg(job)) +
    (token ? `${token.includes('=') ? '&' : '?'}token=${encodeURIComponent(token)}` : '')

  function emit(mode: StreamState['mode']) {
    if (!stopped) onUpdate({ mode, text })
  }

  function startPolling() {
    if (stopped || pollTimer) return
    emit('polling')
    const poll = async () => {
      if (stopped) return
      try {
        const resp: JobJdiResponse = await getJobJdi(project, job)
        if (resp.sessionLog !== null && resp.sessionLog !== text) {
          text = resp.sessionLog
          emit('polling')
        }
      } catch {
        /* transient — the next tick retries */
      }
    }
    void poll()
    pollTimer = setInterval(poll, POLL_MS)
  }

  function startStream() {
    try {
      es = new EventSource(url)
    } catch {
      startPolling()
      return
    }
    es.onmessage = (ev: MessageEvent<string>) => {
      if (stopped) return
      // Snapshot semantics: each event carries the current tail.
      text = ev.data ?? ''
      emit('streaming')
    }
    es.onerror = () => {
      // The stream dropped or never opened (Part 1 daemon → 404). Fall back
      // to polling; EventSource retries on its own, so close it first.
      es?.close()
      es = null
      startPolling()
    }
  }

  startStream()
  return {
    stop() {
      stopped = true
      es?.close()
      if (pollTimer) clearInterval(pollTimer)
    },
  }
}
