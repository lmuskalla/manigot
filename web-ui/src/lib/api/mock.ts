// Mock daemon — an in-memory stand-in for `mg serve` that implements the
// whole surface: Part 1 reads exactly, Part 2 mutations for real (create /
// save brief / launch / jdi / done / delete mutate the fixture state), plus
// a live session.log ticker so the streaming console can be verified
// without a daemon at all.
//
// Enabled by `--mode mock` (npm run dev:mock) or `?mock=1`. Inert otherwise.
// Tests import `mockDaemon` directly and drive its fixture state.

import type {
  AgentRow,
  Health,
  JobRow,
  OrphanRow,
  ProjectRow,
} from './types'

declare const __MG_MOCK__: boolean

const now = () => new Date().toISOString()
const minsAgo = (m: number) => new Date(Date.now() - m * 60_000).toISOString()

export interface MockState {
  health: Health
  projects: ProjectRow[]
  jobs: Record<string, JobRow[]> // project name → jobs
  files: Record<string, string> // `${project}/${job}/${file}` → content
  logs: Record<string, { runLog: string; sessionLog: string }> // project/job
  orphans: Record<string, OrphanRow[]>
}

const briefScaffold = (title: string, type: string, id: string) => `# Brief: ${title}

status: open
type: ${type}
id: ${id}
branch: feature/${id}_slug
date: 2026-08-28
author: Leander Muskalla

## What

Add orphaned-worktree cleanup and container pruning to the mutating API, so a
headless daemon with nobody running the CLI by hand does not accumulate
exactly this cruft.

## Why

The read-only daemon can only show what happened; nobody can act on it without
going back to the terminal. That defeats the point of a remote control plane —
the whole "review from a phone, get a ping when a job needs you, act on it"
story requires write access and live output.

## Out of scope

- Interactive agent sessions in any form
- Host-machine config commands
- Anything not listed under "What" above.
`

const tasksMd = `# Tasks: listener mutating API + run supervision

TASK-1: Create-job endpoint — internal/job.CreateJob behind POST /projects/{p}/jobs, serialized per project via serve.ProjectLocks.
TASK-2: Brief edit endpoint — raw content replace, no $EDITOR; HTTP body write pinned to text/markdown.
TASK-3: Detached agent launch — one-shot via the --print path, never an attached session.
TASK-4: mg jdi launch — detached, profile selectable, status sidecar as before.
TASK-5: done / delete / push endpoints — FinishJob / DeleteJob / git push with the CLI's confirmation semantics surfaced as structured errors.
TASK-6: Orphan cleanup — container prune + worktree orphan removal endpoints.
TASK-7: Live session.log streaming over SSE; confirm it survives the graceful-shutdown drain.
`

const implMd = `# Implementation: listener mutating API + run supervision

## Summary

Mutating endpoints and live run supervision landed on the listener: create
job, edit brief, detached agent + jdi launches, done/delete/push, orphan
cleanup, and SSE streaming of session.log growth.

## Changes

TASK-1: POST /projects/{p}/jobs wired to CreateJob; per-project lock taken
around scaffold + worktree creation.
TASK-7: GET /projects/{p}/jobs/{j}/session-log/stream — SSE with tail
semantics; client disconnect on shutdown drain verified.
`

const verdictApproved = `# Verdict: listener mutating API + run supervision

## Overall

APPROVED — clean endpoint discipline, zero path inputs everywhere, locks held
for exactly the git-mutating operations and not a line more.

## Findings

- The SSE tail survives the shutdown drain (client disconnect, no hang).
- Conflict on done is reported as a structured error; the job is left
  untouched, exactly as pinned.
`

const verdictRejected = `# Verdict: listener mutating API + run supervision

## Overall

NEEDS WORK — the launch endpoint does not validate the profile before
detaching, so a typo silently burns the claude-pro default.

## Findings

- TASK-3: validate the profile id against config.Profiles() before launch.
`

export function fixtureState(): MockState {
  return {
    health: {
      version: '0.14.2',
      imagePresent: true,
      profiles: [
        { id: 'claude-pro', ready: true },
        { id: 'zai', ready: true },
        { id: 'opencode-go', ready: false },
        { id: 'opencode-zen', ready: true },
        { id: 'opencode-zen-free', ready: true },
      ],
    },
    projects: [
      { name: 'manigot', path: '/srv/code/manigot' },
      { name: 'solyto-api', path: '/srv/code/solyto/api' },
    ],
    jobs: {
      manigot: [
        {
          id: 'farmer',
          name: 'farmer_part-2-of-web-ui-tui-path',
          status: 'open',
          stage: 'implement',
          type: 'feature',
          date: '2026-08-28',
          title: 'listener mutating API + run supervision',
          branch: 'feature/farmer_part-2-of-web-ui-tui-path',
          jdi: { state: 'running', agent: 'developer', updated: minsAgo(2) },
        },
        {
          id: 'harbor',
          name: 'harbor_needs-human-on-conflict-handling',
          status: 'open',
          stage: 'review',
          type: 'feature',
          date: '2026-08-27',
          title: 'conflict handling over HTTP',
          branch: 'feature/harbor_needs-human-on-conflict-handling',
          jdi: { state: 'stopped:needs-human', agent: 'reviewer', updated: minsAgo(14) },
        },
        {
          id: 'quartz',
          name: 'quartz_prune-on-a-timer',
          status: 'open',
          stage: 'plan',
          type: 'chore',
          date: '2026-08-26',
          title: 'prune orphaned containers on a timer',
          branch: 'chore/quartz_prune-on-a-timer',
          jdi: null,
        },
        {
          id: 'ember',
          name: 'ember_tui-bell-volume',
          status: 'open',
          stage: 'define',
          type: 'fix',
          date: '2026-08-26',
          title: 'TUI bell rings at full volume',
          branch: 'fix/ember_tui-bell-volume',
          jdi: { state: 'stopped:finished', agent: 'reviewer', updated: minsAgo(200) },
        },
        {
          id: 'lumen',
          name: 'lumen_doctor-health-check',
          status: 'open',
          stage: 'finished',
          type: 'feature',
          date: '2026-08-24',
          title: 'mg doctor health check',
          branch: 'feature/lumen_doctor-health-check',
          jdi: { state: 'stopped:finished', agent: 'reviewer', updated: minsAgo(1500) },
        },
      ],
      'solyto-api': [
        {
          id: 'cedar',
          name: 'cedar_rate-limit-headers',
          status: 'open',
          stage: 'implement',
          type: 'fix',
          date: '2026-08-27',
          title: 'rate limit headers missing on 429',
          branch: 'fix/cedar_rate-limit-headers',
          jdi: { state: 'running', agent: 'analyst', updated: minsAgo(0) },
        },
      ],
    },
    files: {
      'manigot/farmer_part-2-of-web-ui-tui-path/brief.md':
        briefScaffold('listener mutating API + run supervision', 'feature', 'farmer'),
      'manigot/farmer_part-2-of-web-ui-tui-path/tasks.md': tasksMd,
      'manigot/farmer_part-2-of-web-ui-tui-path/implementation.md': implMd,
      'manigot/harbor_needs-human-on-conflict-handling/brief.md':
        briefScaffold('conflict handling over HTTP', 'feature', 'harbor') +
        '\n## Notes\n\nA squash-merge conflict is an interactive prompt today — there is no human\nat a terminal to answer it over HTTP.\n',
      'manigot/harbor_needs-human-on-conflict-handling/tasks.md':
        '# Tasks: conflict handling over HTTP\n\nTASK-1: Pin the done-conflict behavior — structured error, job untouched.\n',
      'manigot/harbor_needs-human-on-conflict-handling/implementation.md':
        '# Implementation: conflict handling over HTTP\n\n## Summary\nLanded the structured conflict error on done.\n',
      'manigot/harbor_needs-human-on-conflict-handling/verdict.md': verdictRejected,
      'manigot/lumen_doctor-health-check/verdict.md': verdictApproved,
      'manigot/quartz_prune-on-a-timer/brief.md':
        briefScaffold('prune orphaned containers on a timer', 'chore', 'quartz'),
    },
    logs: {
      'manigot/farmer_part-2-of-web-ui-tui-path': {
        runLog: [
          '=== 2026-08-28T16:02:11+02:00 mg jdi started: job farmer_part-2-of-web-ui-tui-path, profile claude-pro ===',
          '',
          '=== 2026-08-28T16:02:11+02:00 analyst invoked (attempt 1) ===',
          'analyst finished — tasks.md written (3 tasks)',
          '=== 2026-08-28T16:11:40+02:00 developer invoked (attempt 1) ===',
          'developer finished — 5 commits, implementation.md written',
          '=== 2026-08-28T18:04:02+02:00 reviewer invoked (attempt 1) ===',
          'reviewer verdict: NEEDS WORK — profile validation missing',
          '=== 2026-08-28T18:06:19+02:00 developer invoked (attempt 2) ===',
        ].join('\n'),
        sessionLog: [
          '18:06:19 developer session started (claude-pro, print mode)',
          '18:06:21 reading tasks.md — 7 tasks listed',
          '18:06:24 TASK-1 in progress: create-job endpoint',
          '18:07:58 wrote internal/serve/create.go (h: 84)',
          '18:08:03 tests green: TestCreateJobLocks',
        ].join('\n'),
      },
      'manigot/harbor_needs-human-on-conflict-handling': {
        runLog: [
          '=== 2026-08-27T09:14:00+02:00 mg jdi started: job harbor_needs-human-on-conflict-handling, profile zai ===',
          'analyst finished — tasks.md written (4 tasks)',
          'developer finished — 2 commits',
          'reviewer verdict: NEEDS WORK',
          '=== 2026-08-27T13:41:02+02:00 developer invoked (attempt 2) ===',
          '=== 2026-08-27T14:02:55+02:00 stopped: needs human ===',
        ].join('\n'),
        sessionLog: [
          '14:02:51 NEEDS-HUMAN-INPUT: should done auto-hand-off to @git-solver when a merge conflicts over HTTP, or always require an explicit human call?',
        ].join('\n'),
      },
    },
    orphans: {
      manigot: [{ name: 'abandoned_probe-matrix', dir: '/srv/code/.manigot-worktrees/manigot/abandoned_probe-matrix' }],
      'solyto-api': [],
    },
  }
}

export const mockDaemon = {
  state: fixtureState(),
  reset() {
    this.state = fixtureState()
  },
}

const agents: AgentRow[] = [
  { name: 'analyst', description: 'Reads the brief and drafts tasks.md — one ordered task list, nothing more' },
  { name: 'architect', description: 'Plans how to build it — framework, components, deployment' },
  { name: 'chat', description: 'General conversational assistant' },
  { name: 'designer', description: 'Directs UI/UX — typography, colour, spacing, hierarchy' },
  { name: 'developer', description: 'Implements tasks one at a time, committing as it goes' },
  { name: 'devops', description: 'Pipelines, builds, deployment, getting services running' },
  { name: 'git-solver', description: 'Untangles broken worktrees, conflicted merges, detached HEADs' },
  { name: 'mentor', description: 'Grounded tech mentor — imposter syndrome, skill growth' },
  { name: 'owner', description: 'Reviews features from the product and user perspective' },
  { name: 'prompter', description: 'Crafts high-quality prompts and system instructions' },
  { name: 'quality', description: 'Reviews code quality — readability, DRY, modularity' },
  { name: 'reviewer', description: 'Reviews changes against the task requirements — correctness only' },
  { name: 'security', description: 'Reviews code for vulnerabilities and exposure risks' },
  { name: 'sysadmin', description: 'Manages and administers servers' },
]

const diffLog = [
  '77f2ab1 (HEAD -> feature/farmer_part-2-of-web-ui-tui-path) TASK-7: SSE stream of session.log growth',
  '5b31c9e TASK-5: done/delete/push endpoints',
  'a9c4f82 TASK-3: detached agent launch via --print',
  'e10d7aa TASK-1: create-job endpoint behind ProjectLocks',
  '3fd88c0 TASK-2: brief edit — raw markdown body write',
].join('\n')

const diffStat = [
  ' src/cmd/mg/serve.go                |  38 ++++++++--',
  ' src/internal/serve/api.go          | 212 +++++++++++++++++++++++++++++++++++++++++++--',
  ' src/internal/serve/create.go       |  84 +++++++++++++++++++++',
  ' src/internal/serve/launch.go       | 156 ++++++++++++++++++++++++++++++++++++++',
  ' src/internal/serve/server.go       |  18 +++-',
  ' src/internal/serve/stream.go       | 197 +++++++++++++++++++++++++++++++++++++++++++++',
  ' 6 files changed, 668 insertions(+), 15 deletions(-)',
].join('\n')

const diffPatch = [
  'diff --git a/src/internal/serve/create.go b/src/internal/serve/create.go',
  'new file mode 100644',
  'index 0000000..8c1f2ab',
  '--- /dev/null',
  '+++ b/src/internal/serve/create.go',
  '@@ -0,0 +1,84 @@',
  '+package serve',
  '+',
  '+import (',
  '+\t"net/http"',
  '+)',
  '+',
  '+// handleCreateJob scaffolds a job via internal/job.CreateJob under the',
  '+// project lock — the first mutating endpoint.',
  '+func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {',
  '+\troot, ok := s.resolveProject(w, r.PathValue("project"))',
  '+\tif !ok {',
  '+\t\treturn',
  '+\t}',
  '+\ts.locks.Lock(root)',
  '+\tdefer s.locks.Unlock(root)',
  '+\tname, err := job.CreateJob(root, title, jobType)',
  '+\tif err != nil {',
  '+\t\twriteError(w, http.StatusInternalServerError, err.Error())',
  '+\t\treturn',
  '+\t}',
  '+\twriteJSON(w, http.StatusCreated, map[string]string{"job": name})',
  '+}',
  '',
  'diff --git a/src/internal/serve/server.go b/src/internal/serve/server.go',
  'index 2f9d1c1..a01b3ee 100644',
  '--- a/src/internal/serve/server.go',
  '+++ b/src/internal/serve/server.go',
  '@@ -47,6 +47,12 @@ func (s *Server) routes(mux *http.ServeMux) {',
  ' \tmux.HandleFunc("GET /projects/{project}/agents", s.handleProjectAgents)',
  '+\tmux.HandleFunc("POST /projects/{project}/jobs", s.handleCreateJob)',
  '+\tmux.HandleFunc("PUT /projects/{project}/jobs/{job}/files/brief", s.handleSaveBrief)',
  '+\tmux.HandleFunc("POST /projects/{project}/jobs/{job}/jdi/start", s.handleStartJdi)',
  ' }',
].join('\n')

// ── fetch interception ──────────────────────────────────────────────────────

type Route = {
  method: string
  pattern: RegExp
  handle: (m: RegExpMatchArray, req: { body?: unknown; url: string }) => Response | Promise<Response>
}

const jsonRes = (v: unknown, status = 200) =>
  new Response(JSON.stringify(v === undefined ? null : v), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
const errRes = (status: number, error: string) => jsonRes({ error }, status)
const textRes = (t: string, type = 'text/markdown; charset=utf-8') =>
  new Response(t, { status: 200, headers: { 'Content-Type': type } })

let ticker: ReturnType<typeof setInterval> | null = null
const tickerLines = [
  '18:09:12 TASK-2 in progress: brief edit endpoint',
  '18:10:47 wrote internal/serve/brief.go (h: 41)',
  '18:10:52 tests green: TestSaveBriefReplaces',
  '18:11:03 commit: [farmer] TASK-2: brief edit — raw markdown body write',
  '18:11:20 TASK-3 in progress: detached agent launch',
]
let tickerIdx = 0

function ensureTicker() {
  if (ticker) return
  ticker = setInterval(() => {
    const jobs = mockDaemon.state.jobs['manigot'] ?? []
    const farmer = jobs.find((j) => j.id === 'farmer')
    if (!farmer || farmer.jdi?.state !== 'running') return
    const log = mockDaemon.state.logs['manigot/farmer_part-2-of-web-ui-tui-path']
    if (!log) return
    if (tickerIdx >= tickerLines.length) {
      tickerIdx = 0
      log.sessionLog += '\n…'
    }
    log.sessionLog += '\n' + tickerLines[tickerIdx++]
    farmer.jdi.updated = now()
  }, 4000)
}

function routes(): Route[] {
  const S = mockDaemon.state
  return [
    {
      method: 'GET',
      pattern: /^\/health$/,
      handle: () => jsonRes(S.health),
    },
    {
      method: 'GET',
      pattern: /^\/projects$/,
      handle: () => jsonRes({ projects: S.projects }),
    },
    {
      method: 'GET',
      pattern: /^\/projects\/([^/]+)\/jobs$/,
      handle: (m) => {
        const jobs = S.jobs[m[1]]
        return jobs ? jsonRes({ jobs }) : errRes(404, 'project not found')
      },
    },
    {
      method: 'POST',
      pattern: /^\/projects\/([^/]+)\/jobs$/,
      handle: (m, req) => {
        const jobs = S.jobs[m[1]]
        if (!jobs) return errRes(404, 'project not found')
        const body = JSON.parse(typeof req.body === 'string' ? req.body : '{}') as {
          title?: string
          type?: string
        }
        const id = body.title?.toLowerCase().replace(/[^a-z]+/g, '').slice(0, 6) || 'newjob'
        const name = `${id}_${(body.title ?? 'untitled').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 40)}`
        const job: JobRow = {
          id,
          name,
          status: 'open',
          stage: 'define',
          type: body.type ?? 'feature',
          date: new Date().toISOString().slice(0, 10),
          title: body.title ?? 'untitled',
          branch: `${body.type ?? 'feature'}/${name}`,
          jdi: null,
        }
        jobs.unshift(job)
        S.files[`${m[1]}/${name}/brief.md`] = briefScaffold(job.title, job.type, id)
        return jsonRes({ job }, 201)
      },
    },
    {
      method: 'GET',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/files\/(brief|tasks|implementation|verdict)\.md$/,
      handle: (m) => {
        const content = S.files[`${m[1]}/${m[2]}/${m[3]}.md`]
        return content !== undefined ? textRes(content) : errRes(404, 'file not found')
      },
    },
    {
      method: 'PUT',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/files\/brief$/,
      handle: async (m, req) => {
        S.files[`${m[1]}/${m[2]}/brief.md`] = String(typeof req.body === 'string' ? req.body : '')
        return jsonRes({ ok: true })
      },
    },
    {
      method: 'GET',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/jdi$/,
      handle: (m) => {
        const jobs = S.jobs[m[1]] ?? []
        const job = jobs.find((j) => j.id === m[2] || j.name === m[2])
        if (!job) return errRes(404, 'job not found')
        const log = S.logs[`${m[1]}/${job.name}`]
        return jsonRes({
          status: job.jdi ?? null,
          runLog: log?.runLog ?? null,
          sessionLog: log?.sessionLog ?? null,
        })
      },
    },
    {
      method: 'POST',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/jdi\/start$/,
      handle: (m) => {
        const jobs = S.jobs[m[1]] ?? []
        const job = jobs.find((j) => j.id === m[2] || j.name === m[2])
        if (!job) return errRes(404, 'job not found')
        job.jdi = { state: 'running', agent: 'analyst', updated: now() }
        S.logs[`${m[1]}/${job.name}`] ??= { runLog: `=== ${now()} mg jdi started ===`, sessionLog: '' }
        ensureTicker()
        return jsonRes({ started: true, message: `mg jdi detached for ${job.name}` })
      },
    },
    {
      method: 'POST',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/agents\/([^/]+)\/launch$/,
      handle: (m) => {
        const jobs = S.jobs[m[1]] ?? []
        const job = jobs.find((j) => j.id === m[2] || j.name === m[2])
        if (!job) return errRes(404, 'job not found')
        const log = S.logs[`${m[1]}/${job.name}`] ??= { runLog: '', sessionLog: '' }
        log.runLog += `\n=== ${now()} ${m[3]} invoked (detached, one-shot) ===`
        job.jdi = { state: 'running', agent: m[3], updated: now() }
        return jsonRes({ started: true, message: `${m[3]} launched detached on ${job.name}` })
      },
    },
    {
      method: 'GET',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/diff$/,
      handle: (m, req) => {
        const query = req.url.split('?')[1] ?? ''
        if (new URLSearchParams(query).get('full') === '1') return jsonRes({ patch: diffPatch })
        return jsonRes({ log: diffLog, stat: diffStat })
      },
    },
    {
      method: 'GET',
      pattern: /^\/projects\/([^/]+)\/agents$/,
      handle: (m) => (S.jobs[m[1]] ? jsonRes({ agents }) : errRes(404, 'project not found')),
    },
    {
      method: 'POST',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/done$/,
      handle: (m) => {
        const jobs = S.jobs[m[1]] ?? []
        const idx = jobs.findIndex((j) => j.id === m[2] || j.name === m[2])
        if (idx < 0) return errRes(404, 'job not found')
        const [job] = jobs.splice(idx, 1)
        return jsonRes({ message: `archived ${job.name} — squash-merged into main` })
      },
    },
    {
      method: 'POST',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/delete$/,
      handle: (m) => {
        const jobs = S.jobs[m[1]] ?? []
        const idx = jobs.findIndex((j) => j.id === m[2] || j.name === m[2])
        if (idx < 0) return errRes(404, 'job not found')
        const [job] = jobs.splice(idx, 1)
        return jsonRes({ message: `deleted ${job.name} — worktree and branch removed` })
      },
    },
    {
      method: 'POST',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/push$/,
      handle: (m) => jsonRes({ message: `pushed ${m[2]} to origin` }),
    },
    {
      method: 'GET',
      pattern: /^\/projects\/([^/]+)\/orphans$/,
      handle: (m) => jsonRes({ orphans: S.orphans[m[1]] ?? [] }),
    },
    {
      method: 'POST',
      pattern: /^\/projects\/([^/]+)\/orphans\/remove$/,
      handle: (m) => {
        const count = S.orphans[m[1]]?.length ?? 0
        S.orphans[m[1]] = []
        return jsonRes({ message: `removed ${count} orphaned worktree${count === 1 ? '' : 's'}` })
      },
    },
    {
      method: 'POST',
      pattern: /^\/prune$/,
      handle: () => jsonRes({ removed: 2, running: 1 }),
    },
    // SSE stream — sends the current tail, then keeps sending as the ticker
    // grows it. Falls through to 404 semantics if the job has no log yet.
    {
      method: 'GET',
      pattern: /^\/projects\/([^/]+)\/jobs\/([^/]+)\/session-log\/stream$/,
      handle: (m) => {
        const jobs = S.jobs[m[1]] ?? []
        const job = jobs.find((j) => j.id === m[2] || j.name === m[2])
        const log = job && S.logs[`${m[1]}/${job.name}`]
        if (!log) return errRes(404, 'no session log for this job')
        const stream = new ReadableStream({
          start(controller) {
            const enc = new TextEncoder()
            let lastLen = -1
            const send = () => {
              if (log.sessionLog.length !== lastLen) {
                lastLen = log.sessionLog.length
                controller.enqueue(enc.encode(`data: ${JSON.stringify(log.sessionLog).slice(1, -1)}\n\n`))
              }
            }
            send()
            const iv = setInterval(send, 1500)
            // @ts-expect-error — signal cleanup when the client disconnects
            this._iv = iv
          },
          cancel(reason) {
            // @ts-expect-error — mirror of start's handle
            clearInterval(this._iv)
            void reason
          },
        })
        return new Response(stream, { headers: { 'Content-Type': 'text/event-stream' } })
      },
    },
  ]
}

let installed = false

export function installMock() {
  if (installed) return
  installed = true
  const realFetch = globalThis.fetch
  mockFetch.realFetch = realFetch
  globalThis.fetch = mockFetch as typeof fetch
  ensureTicker()
}

export const mockFetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
  const method = (
    init?.method ?? (typeof input !== 'string' && !(input instanceof URL) ? input.method : 'GET')
  ).toUpperCase()
  const path = url.replace(/^https?:\/\/[^/]+/, '')
  for (const r of routes()) {
    if (r.method !== method) continue
    const m = path.split('?')[0].match(r.pattern)
    if (m) return await r.handle(m, { body: init?.body, url })
  }
  return errRes(404, `mock daemon: no route for ${method} ${path}`)
}
mockFetch.realFetch = undefined as unknown as typeof fetch

/** Wire the mock into the module loader when enabled (mock mode or ?mock=1). */
export function maybeInstallMockFromEnv() {
  const forced = typeof __MG_MOCK__ !== 'undefined' && __MG_MOCK__
  const qs = typeof location !== 'undefined' && new URLSearchParams(location.search).has('mock')
  if (forced || qs) installMock()
}
