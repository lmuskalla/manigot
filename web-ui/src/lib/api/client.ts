// The API client — one fetch wrapper with the daemon's conventions:
//   · JSON everywhere except job files (raw markdown bodies)
//   · errors as `{ "error": "…" }` envelopes on every non-2xx
//   · optional bearer token (localhost daemon is tokenless; a remote bind
//     requires one — tokens are configured out-of-band, the API never issues)
//
// Part 2 capability probing: mutating endpoints answer 404/405 on a Part 1
// daemon. The client records that as a capability miss so the UI can render
// "not available on this daemon" instead of a raw error — the same frontend
// must work against both daemon generations while job two lands.

import { ep, seg } from './endpoints'
import type {
  AgentsResponse,
  Health,
  JobDiffResponse,
  JobJdiResponse,
  JobRow,
  JobsResponse,
  LaunchResponse,
  OrphansResponse,
  ProjectsResponse,
  PruneResponse,
} from './types'

export interface Connection {
  baseUrl: string
  token: string
}

export class ApiError extends Error {
  status: number
  /** True when the daemon answered "no such route" — a Part 2 call on a Part 1 daemon. */
  capabilityMiss: boolean

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.capabilityMiss = status === 404 || status === 405
  }
}

export function normalizeBaseUrl(url: string): string {
  const trimmed = url.trim().replace(/\/+$/, '')
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  // Same-origin (e.g. served by the daemon itself, or behind the vite proxy).
  return trimmed.startsWith('/') ? trimmed : `http://${trimmed}`
}

let conn: Connection = { baseUrl: '', token: '' }

export function setConnection(c: Connection) {
  conn = { baseUrl: normalizeBaseUrl(c.baseUrl), token: c.token }
}

export function getConnection(): Connection {
  return { ...conn }
}

// ── fetch plumbing ──────────────────────────────────────────────────────────

type BodyInitLike = string | undefined

interface ReqOpts {
  body?: BodyInitLike
  contentType?: string
  signal?: AbortSignal
}

async function request(method: string, path: string, opts: ReqOpts = {}): Promise<string> {
  const headers: Record<string, string> = {}
  if (conn.token) headers['Authorization'] = `Bearer ${conn.token}`
  if (opts.body !== undefined && opts.contentType) headers['Content-Type'] = opts.contentType

  let res: Response
  try {
    res = await fetch(conn.baseUrl + path, {
      method,
      headers,
      body: opts.body,
      signal: opts.signal,
    })
  } catch (e) {
    if (e instanceof Error && e.name === 'AbortError') throw e
    throw new ApiError(0, `cannot reach the daemon at ${conn.baseUrl || '(same origin)'} — ${e instanceof Error ? e.message : String(e)}`)
  }

  if (res.status === 401) {
    throw new ApiError(401, 'the daemon rejected the token (401) — check the connection settings')
  }
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = (await res.json()) as { error?: string }
      if (data && typeof data.error === 'string' && data.error) msg = data.error
    } catch {
      /* non-JSON error body — keep the status line */
    }
    throw new ApiError(res.status, msg)
  }
  return res.text()
}

async function json<T>(method: string, path: string, opts: ReqOpts = {}): Promise<T> {
  const text = await request(method, path, opts)
  return (text === '' ? undefined : (JSON.parse(text) as T)) as T
}

function post<T>(path: string, body?: unknown): Promise<T> {
  return json<T>('POST', path, {
    body: body === undefined ? undefined : JSON.stringify(body),
    contentType: 'application/json',
  })
}

// ── Part 1: read-only ───────────────────────────────────────────────────────

export function getHealth(signal?: AbortSignal): Promise<Health> {
  return json<Health>('GET', ep.health, { signal })
}

export function getProjects(signal?: AbortSignal): Promise<ProjectsResponse> {
  return json<ProjectsResponse>('GET', ep.projects, { signal })
}

export function getJobs(project: string, signal?: AbortSignal): Promise<JobsResponse> {
  return json<JobsResponse>('GET', `${ep.jobs(seg(project))}`, { signal })
}

export function getJobFile(project: string, job: string, file: string, signal?: AbortSignal): Promise<string> {
  return request('GET', ep.file(seg(project), seg(job), seg(file)), { signal })
}

export function getJobJdi(project: string, job: string, signal?: AbortSignal): Promise<JobJdiResponse> {
  return json<JobJdiResponse>('GET', ep.jdi(seg(project), seg(job)), { signal })
}

export function getJobDiff(project: string, job: string, full = false, signal?: AbortSignal): Promise<JobDiffResponse> {
  return json<JobDiffResponse>('GET', `${ep.diff(seg(project), seg(job))}${full ? '?full=1' : ''}`, { signal })
}

export function getAgents(project: string, signal?: AbortSignal): Promise<AgentsResponse> {
  return json<AgentsResponse>('GET', ep.agents(seg(project)), { signal })
}

// ── Part 2: mutating + supervision ──────────────────────────────────────────

export function createJob(project: string, title: string, type: string): Promise<{ job: JobRow | string }> {
  return post(ep.createJob(seg(project)), { title, type })
}

export function saveBrief(project: string, job: string, content: string): Promise<{ ok: boolean }> {
  return json<{ ok: boolean }>('PUT', ep.saveBrief(seg(project), seg(job)), {
    body: content,
    contentType: 'text/markdown; charset=utf-8',
  })
}

export function launchAgent(project: string, job: string, agent: string, profile?: string): Promise<LaunchResponse> {
  return post(ep.launchAgent(seg(project), seg(job), seg(agent)), profile ? { profile } : {})
}

export function startJdi(project: string, job: string, profile?: string): Promise<LaunchResponse> {
  return post(ep.startJdi(seg(project), seg(job)), profile ? { profile } : {})
}

export function doneJob(project: string, job: string): Promise<{ message?: string }> {
  return post(ep.doneJob(seg(project), seg(job)))
}

export function deleteJob(project: string, job: string): Promise<{ message?: string }> {
  return post(ep.deleteJob(seg(project), seg(job)))
}

export function pushJob(project: string, job: string): Promise<{ message?: string }> {
  return post(ep.pushJob(seg(project), seg(job)))
}

export function getOrphans(project: string, signal?: AbortSignal): Promise<OrphansResponse> {
  return json<OrphansResponse>('GET', ep.orphans(seg(project)), { signal })
}

export function removeOrphans(project: string): Promise<{ message?: string }> {
  return post(ep.removeOrphans(seg(project)))
}

export function pruneContainers(): Promise<PruneResponse> {
  return post(ep.pruneContainers())
}
