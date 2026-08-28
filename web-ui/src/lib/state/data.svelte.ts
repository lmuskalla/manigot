// Data store — projects + the active project's jobs, polled while visible.
// The TUI re-reads state on a 1s timer; the web client polls every 2s while
// the tab is visible and pauses when hidden (an SSE future replaces the
// job-level polling only — the list stays a snapshot API by design).

import * as api from '$lib/api/client'
import type { JobRow, ProjectRow } from '$lib/api/types'

const POLL_MS = 2000

class DataStore {
  projects = $state<ProjectRow[]>([])
  active = $state<string>('')
  jobs = $state<JobRow[]>([])
  loadingProjects = $state(false)
  loadingJobs = $state(false)
  jobsError = $state('')

  #jobsTimer: ReturnType<typeof setInterval> | null = null

  async loadProjects(): Promise<void> {
    this.loadingProjects = true
    try {
      const res = await api.getProjects()
      this.projects = res.projects ?? []
      if (this.active && this.projects.some((p) => p.name === this.active)) return
      const remembered = localStorage.getItem('mg-control.project')
      this.active =
        this.projects.find((p) => p.name === remembered)?.name ?? this.projects[0]?.name ?? ''
      if (this.active) localStorage.setItem('mg-control.project', this.active)
    } finally {
      this.loadingProjects = false
    }
  }

  setActive(project: string) {
    if (!project || project === this.active) return
    this.active = project
    localStorage.setItem('mg-control.project', project)
    void this.refreshJobs()
  }

  async refreshJobs(): Promise<void> {
    if (!this.active) return
    this.loadingJobs = true
    try {
      const res = await api.getJobs(this.active)
      this.jobs = res.jobs ?? []
      this.jobsError = ''
    } catch (e) {
      if (e instanceof api.ApiError && e.status === 0) return // aborted / offline — keep last
      this.jobsError = e instanceof Error ? e.message : String(e)
    } finally {
      this.loadingJobs = false
    }
  }

  startPolling() {
    this.stopPolling()
    const tick = () => {
      if (document.visibilityState === 'visible') void this.refreshJobs()
    }
    this.#jobsTimer = setInterval(tick, POLL_MS)
  }

  stopPolling() {
    if (this.#jobsTimer) clearInterval(this.#jobsTimer)
    this.#jobsTimer = null
  }
}

export const data = new DataStore()
