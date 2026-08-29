// A tiny hash router — the app is embedded into the Go binary eventually
// (same-origin, no server routing), so hash paths keep it proxy-agnostic.
//
//   #/                          → redirect to the active project
//   #/p/:project                → jobs
//   #/p/:project/j/:job/:tab?   → job detail (brief|tasks|implementation|verdict|diff|run)
//   #/p/:project/agents         → the crew
//   #/health                    → daemon health + connection + maintenance

export interface Route {
  name: 'jobs' | 'job' | 'agents' | 'health' | 'home'
  project?: string
  job?: string
  tab?: string
}

export function parseHash(hash: string): Route {
  const h = hash.replace(/^#/, '')
  const parts = h.split('/').filter(Boolean).map(decodeURIComponent)

  if (parts[0] === 'health') return { name: 'health' }
  if (parts[0] === 'p' && parts[1]) {
    const project = parts[1]
    if (parts[2] === 'j' && parts[3]) {
      return { name: 'job', project, job: parts[3], tab: parts[4] ?? 'brief' }
    }
    if (parts[2] === 'agents') return { name: 'agents', project }
    return { name: 'jobs', project }
  }
  return { name: 'home' }
}

export function href(route: Route): string {
  switch (route.name) {
    case 'health':
      return '#/health'
    // No project yet (fresh boot, daemon down, nothing registered): link
    // home — '#/p/' and '#/p//agents' parse to garbage, not to home.
    case 'jobs':
      return route.project ? `#/p/${encodeURIComponent(route.project)}` : '#/'
    case 'agents':
      return route.project ? `#/p/${encodeURIComponent(route.project)}/agents` : '#/'
    case 'job':
      return `#/p/${encodeURIComponent(route.project ?? '')}/j/${encodeURIComponent(route.job ?? '')}${route.tab && route.tab !== 'brief' ? `/${route.tab}` : ''}`
    default:
      return '#/'
  }
}

export function navigate(to: string) {
  // href() already returns '#/…' — never double-prefix the fragment:
  // '#/p/manigot' fed through '#${to}' becomes '##/p/manigot', which
  // parseHash cannot parse, so the app never leaves the landing.
  const path = to.replace(/^#/, '')
  location.hash = path.startsWith('/') ? `#${path}` : to
}
