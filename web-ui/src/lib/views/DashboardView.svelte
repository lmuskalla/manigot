<script lang="ts">
  // The default landing view at `#/` — an overview across every registered
  // project, not just the active one. The daemon has no aggregate endpoint
  // (job two of the control plane), so this is the one place in the app that
  // fans out N HTTP calls client-side: one getJobs(project) per registered
  // project, via Promise.allSettled so a single project's failure degrades
  // to an inline "couldn't load" note instead of blanking the page. Loads
  // once on mount — no polling, to avoid turning this into a request storm
  // as the registry grows.
  import { onMount } from 'svelte'
  import * as api from '$lib/api/client'
  import { href } from '$lib/router'
  import { relativeTime } from '$lib/time'
  import { aggregateCounts, attentionJobs, type ProjectLoad } from '$lib/dashboard'
  import EmptyState from '$lib/components/EmptyState.svelte'
  import type { ProjectRow } from '$lib/api/types'

  let projects = $state<ProjectRow[]>([])
  let loads = $state<ProjectLoad[]>([])
  let errors = $state<Record<string, string>>({})
  let loading = $state(true)
  let error = $state('')

  onMount(async () => {
    try {
      const res = await api.getProjects()
      projects = res.projects ?? []
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
      loading = false
      return
    }

    const settled = await Promise.allSettled(projects.map((p) => api.getJobs(p.name)))
    const nextLoads: ProjectLoad[] = []
    const nextErrors: Record<string, string> = {}
    settled.forEach((res, i) => {
      const project = projects[i].name
      if (res.status === 'fulfilled') {
        nextLoads.push({ project, jobs: res.value.jobs ?? [] })
      } else {
        nextLoads.push({ project, jobs: null })
        nextErrors[project] = res.reason instanceof Error ? res.reason.message : String(res.reason)
      }
    })
    loads = nextLoads
    errors = nextErrors
    loading = false
  })

  const counts = $derived(aggregateCounts(loads))
  const attention = $derived(attentionJobs(loads).slice(0, 10))
  const jobCount = (project: string) => loads.find((l) => l.project === project)?.jobs?.length ?? null
</script>

<div class="page">
  <header class="head">
    <div>
      <p class="eyebrow">overview</p>
      <h1>Dashboard</h1>
    </div>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else}
    <section class="stats" aria-label="Overview">
      <div class="stat">
        <span class="stat-v">{projects.length}</span>
        <span class="stat-k">projects</span>
      </div>
      <div class="stat">
        <span class="stat-v">{counts.openJobs}</span>
        <span class="stat-k">open jobs</span>
      </div>
      <div class="stat" class:warn={counts.needsHuman > 0}>
        <span class="stat-v">{counts.needsHuman}</span>
        <span class="stat-k">needs human</span>
      </div>
      <div class="stat">
        <span class="stat-v">{counts.running}</span>
        <span class="stat-k">running</span>
      </div>
    </section>

    <section class="block">
      <h2>Needs attention</h2>
      {#if loading}
        <p class="loading">Loading jobs across projects…</p>
      {:else if attention.length === 0}
        <EmptyState title="Nothing needs attention" hint="No jobs are running or waiting on a human right now." />
      {:else}
        <ul class="jobs" role="list">
          {#each attention as job (job.project + '/' + job.name)}
            <li>
              <a class="job" class:needs-human={job.jdi?.state === 'stopped:needs-human'} href={href({ name: 'job', project: job.project, job: job.name })}>
                <div class="left">
                  <span class="proj">{job.project}</span>
                  <div class="ident">
                    <span class="title">{job.title}</span>
                    <span class="name">{job.name}</span>
                  </div>
                </div>
                <div class="meta">
                  {#if job.jdi?.state === 'stopped:needs-human'}
                    <span class="chip chip-human"><span class="dot"></span> needs human</span>
                  {:else if job.jdi?.state === 'running'}
                    <span class="chip chip-running"><span class="dot"></span> {job.jdi.agent} running</span>
                  {/if}
                  <span class="when" title={job.jdi ? job.jdi.updated : job.date}>
                    {job.jdi ? relativeTime(job.jdi.updated) : job.date}
                  </span>
                </div>
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section class="block">
      <h2>Projects</h2>
      {#if projects.length === 0}
        <EmptyState title="No projects registered" hint="Edit config/serve.json on the daemon and restart mg serve." />
      {:else}
        <ul class="grid" role="list">
          {#each projects as p (p.name)}
            <li>
              <a class="proj-card" href={href({ name: 'jobs', project: p.name })}>
                <span class="proj-name">{p.name}</span>
                {#if errors[p.name]}
                  <span class="proj-err">Couldn't load jobs — {errors[p.name]}</span>
                {:else if jobCount(p.name) === null}
                  <span class="proj-count">…</span>
                {:else}
                  <span class="proj-count">{jobCount(p.name)} job{jobCount(p.name) === 1 ? '' : 's'}</span>
                {/if}
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}
</div>

<style>
  .page {
    padding: var(--r-6) var(--r-6) var(--r-7);
    max-width: 1060px;
    margin: 0 auto;
    width: 100%;
  }

  .head {
    margin-bottom: var(--r-4);
  }
  h1 {
    font-size: 24px;
    letter-spacing: -0.02em;
  }
  .head .eyebrow {
    margin: 0 0 2px;
  }

  .error {
    background: var(--st-human-dim);
    border: 1px solid rgba(242, 85, 90, 0.4);
    color: #ffb3b6;
    padding: var(--r-3) var(--r-4);
    border-radius: var(--radius-m);
    font-size: 13.5px;
  }

  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: var(--r-3);
    margin-bottom: var(--r-6);
  }
  .stat {
    display: flex;
    flex-direction: column;
    gap: 2px;
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    padding: var(--r-4) var(--r-5);
  }
  .stat-v {
    font-size: 26px;
    font-weight: 660;
    letter-spacing: -0.02em;
    color: var(--ink-0);
  }
  .stat.warn .stat-v {
    color: var(--st-human);
  }
  .stat-k {
    font-family: var(--font-mono);
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-2);
  }

  .block {
    margin-bottom: var(--r-6);
  }
  .block h2 {
    font-size: 15.5px;
    margin-bottom: var(--r-3);
  }
  .loading {
    color: var(--ink-2);
    font-size: 13.5px;
  }

  .jobs {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .job {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--r-3) var(--r-4);
    flex-wrap: wrap;
    padding: 12px 16px;
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    text-decoration: none;
    color: var(--ink-0);
    transition:
      border-color var(--t-fast),
      background var(--t-fast);
  }
  .job:hover {
    border-color: var(--line-strong);
    background: var(--bg-2);
    text-decoration: none;
  }
  .job.needs-human {
    border-color: rgba(242, 85, 90, 0.4);
    background: linear-gradient(90deg, rgba(242, 85, 90, 0.05), transparent 200px), var(--bg-1);
  }
  .left {
    display: flex;
    align-items: center;
    gap: var(--r-4);
    min-width: 0;
    flex: 1 1 240px;
  }
  .proj {
    font-family: var(--font-mono);
    font-size: 11px;
    letter-spacing: 0.04em;
    color: var(--accent-bright);
    background: var(--accent-dim);
    border-radius: var(--radius-s);
    padding: 3px 7px;
    flex: none;
  }
  .ident {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .title {
    font-weight: 590;
    font-size: 14.5px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .name {
    font-family: var(--font-mono);
    font-size: 11.5px;
    color: var(--ink-2);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 0 1 auto;
    flex-wrap: wrap;
  }
  .when {
    font-size: 12px;
    color: var(--ink-2);
  }

  .grid {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: var(--r-3);
  }
  .proj-card {
    display: flex;
    flex-direction: column;
    gap: 6px;
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    padding: var(--r-4);
    text-decoration: none;
    color: var(--ink-0);
    transition: border-color var(--t-fast), background var(--t-fast);
  }
  .proj-card:hover {
    border-color: var(--line-strong);
    background: var(--bg-2);
    text-decoration: none;
  }
  .proj-name {
    font-weight: 600;
    font-size: 15px;
  }
  .proj-count {
    font-size: 12.5px;
    color: var(--ink-2);
  }
  .proj-err {
    font-size: 12px;
    color: #ffb3b6;
  }

  @media (max-width: 760px) {
    .page {
      padding: var(--r-4) var(--r-4) var(--r-6);
    }
  }
</style>
