// API types — pinned to the daemon's actual JSON shapes.
//
// Part 1 (shipped, read-only — src/internal/serve/api.go) shapes are exact.
// Part 2 (mutating + run supervision, per the job-two brief) shapes are the
// agreed contract; the endpoint paths live in endpoints.ts so they can be
// re-pinned in one place when the pipeline job lands.

// ── Part 1: read-only (exact) ────────────────────────────────────────────────

export interface HealthProfile {
  id: string
  ready: boolean
}

export interface Health {
  version: string
  imagePresent: boolean
  profiles: HealthProfile[] | null // Go emits null for an empty slice
}

export interface ProjectRow {
  name: string
  path: string
}

export interface ProjectsResponse {
  projects: ProjectRow[]
}

/** mg-jdi activity state for a job, from the status sidecar. */
export interface JdiStatus {
  state: 'running' | 'stopped:finished' | 'stopped:needs-human'
  agent: string
  updated: string // RFC3339
}

/** One row in the jobs response — the info design: id/status/stage/type/date/title. */
export interface JobRow {
  id: string
  name: string
  status: string // "open" | "done"
  stage: 'define' | 'plan' | 'implement' | 'review' | 'finished' | string
  type: string // "feature" | "fix" | "chore"
  date: string // YYYY-MM-DD
  title: string
  branch?: string
  jdi?: JdiStatus | null
}

export interface JobsResponse {
  jobs: JobRow[]
}

export type JobFileKind = 'brief' | 'tasks' | 'implementation' | 'verdict'

/** GET /projects/{p}/jobs/{j}/jdi — status + captured log tails (null = absent). */
export interface JobJdiResponse {
  status: JdiStatus | null
  runLog: string | null
  sessionLog: string | null
}

/** GET /projects/{p}/jobs/{j}/diff — quick eyeball (log+stat) or ?full=1 patch. */
export interface JobDiffResponse {
  log?: string | null
  stat?: string | null
  patch?: string | null
}

export interface AgentRow {
  name: string
  description: string
}

export interface AgentsResponse {
  agents: AgentRow[]
}

// ── Part 2: mutating + run supervision (per the job-two brief) ───────────────

/** An orphaned worktree surfaced for cleanup (name is what mg delete resolves). */
export interface OrphanRow {
  name: string
  dir?: string
}

export interface OrphansResponse {
  orphans: OrphanRow[] | null
}

export interface PruneResponse {
  removed: number
  running: number
}

/** Result of a detached launch (agent one-shot or mg jdi). */
export interface LaunchResponse {
  started: boolean
  message?: string
}
