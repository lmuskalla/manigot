// Endpoint paths — the single seam against the daemon.
//
// Part 1 paths are shipped (src/internal/serve/server.go routes). Part 2
// paths follow the job-two brief ("create a job", "edit brief.md", "launch
// agent detached via the --print path", "launch mg jdi detached", "done /
// delete / push", "orphan cleanup", "stream session.log"); when the pipeline
// job lands, re-pin anything that drifted HERE and nowhere else.
//
// Part 2 endpoints are expected to answer 404/405 on a Part 1-only daemon;
// the client surfaces that as a capability miss, not a crash (see client.ts).

export const ep = {
  health: '/health',
  projects: '/projects',
  jobs: (p: string) => `/projects/${p}/jobs`,
  job: (p: string, j: string) => `/projects/${p}/jobs/${j}`,
  file: (p: string, j: string, f: string) => `/projects/${p}/jobs/${j}/files/${f}`,
  jdi: (p: string, j: string) => `/projects/${p}/jobs/${j}/jdi`,
  diff: (p: string, j: string) => `/projects/${p}/jobs/${j}/diff`,
  agents: (p: string) => `/projects/${p}/agents`,

  // ── Part 2 (job two) ────────────────────────────────────────────────────
  createJob: (p: string) => `/projects/${p}/jobs`, // POST
  saveBrief: (p: string, j: string) => `/projects/${p}/jobs/${j}/files/brief`, // PUT
  launchAgent: (p: string, j: string, agent: string) =>
    `/projects/${p}/jobs/${j}/agents/${agent}/launch`, // POST
  startJdi: (p: string, j: string) => `/projects/${p}/jobs/${j}/jdi/start`, // POST
  doneJob: (p: string, j: string) => `/projects/${p}/jobs/${j}/done`, // POST
  deleteJob: (p: string, j: string) => `/projects/${p}/jobs/${j}/delete`, // POST
  pushJob: (p: string, j: string) => `/projects/${p}/jobs/${j}/push`, // POST
  orphans: (p: string) => `/projects/${p}/orphans`, // GET
  removeOrphans: (p: string) => `/projects/${p}/orphans/remove`, // POST
  pruneContainers: () => `/prune`, // POST — daemon-wide (docker), not per project
  sessionStream: (p: string, j: string) => `/projects/${p}/jobs/${j}/session-log/stream`, // GET SSE
}

/** Escape a URL segment conservatively (names are id_slug identifiers). */
export function seg(s: string): string {
  return encodeURIComponent(s)
}
