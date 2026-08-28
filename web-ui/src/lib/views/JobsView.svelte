<script lang="ts">
  import { onMount } from 'svelte'
  import Pipeline from '$lib/components/Pipeline.svelte'
  import EmptyState from '$lib/components/EmptyState.svelte'
  import Modal from '$lib/components/Modal.svelte'
  import Confirm from '$lib/components/Confirm.svelte'
  import { data } from '$lib/state/data.svelte'
  import { toasts } from '$lib/state/toasts.svelte'
  import { attention } from '$lib/stage'
  import { relativeTime } from '$lib/time'
  import { href } from '$lib/router'
  import * as api from '$lib/api/client'
  import type { JobRow, OrphanRow } from '$lib/api/types'

  let { project }: { project: string } = $props()

  let filter = $state<'all' | 'human' | 'running'>('all')
  let typeFilter = $state('')
  let search = $state('')
  let showNew = $state(false)

  // new-job form
  let newTitle = $state('')
  let newType = $state('feature')
  let creating = $state(false)

  // orphans (Part 2 — silently absent on a read-only daemon)
  let orphans = $state<OrphanRow[]>([])
  let orphansAvailable = $state(true)
  let confirmOrphans = $state(false)
  let removingOrphans = $state(false)

  $effect(() => {
    // reload the list when the route's project changes
    project
    void data.refreshJobs()
    void loadOrphans()
  })

  onMount(() => {
    data.startPolling()
    return () => data.stopPolling()
  })

  async function loadOrphans() {
    try {
      const res = await api.getOrphans(project)
      orphans = res.orphans ?? []
      orphansAvailable = true
    } catch {
      orphans = []
      orphansAvailable = false
    }
  }

  const filtered = $derived.by(() => {
    let jobs = data.active === project ? data.jobs : []
    if (filter === 'human') jobs = jobs.filter((j) => j.jdi?.state === 'stopped:needs-human')
    if (filter === 'running') jobs = jobs.filter((j) => j.jdi?.state === 'running')
    if (typeFilter) jobs = jobs.filter((j) => j.type === typeFilter)
    const needle = search.trim().toLowerCase()
    if (needle) {
      jobs = jobs.filter(
        (j) =>
          j.title.toLowerCase().includes(needle) ||
          j.name.toLowerCase().includes(needle) ||
          j.id.toLowerCase().includes(needle),
      )
    }
    return [...jobs].sort((a, b) => attention(a).level - attention(b).level)
  })

  const counts = $derived.by(() => {
    const jobs = data.active === project ? data.jobs : []
    return {
      all: jobs.length,
      human: jobs.filter((j) => j.jdi?.state === 'stopped:needs-human').length,
      running: jobs.filter((j) => j.jdi?.state === 'running').length,
    }
  })

  async function createJob() {
    creating = true
    try {
      const res = await api.createJob(project, newTitle.trim(), newType)
      toasts.ok(`Job scaffolded — ${typeof res.job === 'string' ? res.job : res.job?.name ?? newTitle}`)
      showNew = false
      newTitle = ''
      newType = 'feature'
      await data.refreshJobs()
    } catch (e) {
      toasts.error(actionError(e, 'create jobs'))
    } finally {
      creating = false
    }
  }

  async function removeOrphans() {
    removingOrphans = true
    try {
      const res = await api.removeOrphans(project)
      toasts.ok(res.message ?? 'Orphaned worktrees removed')
      confirmOrphans = false
      await loadOrphans()
    } catch (e) {
      toasts.error(actionError(e, 'remove orphans'))
    } finally {
      removingOrphans = false
    }
  }

  /** Degrade path: Part 2 calls on a Part 1 daemon read as a clear miss. */
  function actionError(e: unknown, what: string): string {
    if (e instanceof api.ApiError && e.capabilityMiss) {
      return `This daemon does not expose ${what} yet — mutating endpoints land with job two of the control plane.`
    }
    return e instanceof Error ? e.message : String(e)
  }
</script>

<div class="page">
  <header class="head">
    <div>
      <p class="eyebrow">jobs</p>
      <h1>{project}</h1>
    </div>
    <div class="head-actions">
      <input
        class="input search"
        placeholder="Filter by title, name, id…"
        bind:value={search}
        spellcheck="false"
        aria-label="Filter jobs"
      />
      <button class="btn btn-primary" onclick={() => (showNew = true)}>＋ New job</button>
    </div>
  </header>

  <div class="filters">
    <div class="seg-group" role="group" aria-label="Filter by run state">
      <button aria-pressed={filter === 'all'} onclick={() => (filter = 'all')}>
        All {counts.all}
      </button>
      <button aria-pressed={filter === 'human'} onclick={() => (filter = 'human')}>
        Needs human {counts.human}
      </button>
      <button aria-pressed={filter === 'running'} onclick={() => (filter = 'running')}>
        Running {counts.running}
      </button>
    </div>
    <div class="seg-group" role="group" aria-label="Filter by type">
      <button aria-pressed={typeFilter === ''} onclick={() => (typeFilter = '')}>any type</button>
      {#each ['feature', 'fix', 'chore'] as t (t)}
        <button aria-pressed={typeFilter === t} onclick={() => (typeFilter = t)}>{t}</button>
      {/each}
    </div>
  </div>

  {#if data.jobsError}
    <div class="error" role="alert">{data.jobsError}</div>
  {/if}

  {#if filtered.length === 0 && !data.loadingJobs && !data.jobsError}
    <EmptyState
      title={data.jobs.length === 0 ? 'No open jobs' : 'Nothing matches'}
      hint={data.jobs.length === 0
        ? 'A job scaffolds brief.md, a branch and its own worktree — everything the crew needs to start.'
        : 'Loosen the filters or clear the search.'}
    >
      {#snippet children()}
        {#if data.jobs.length === 0}
          <button class="btn" onclick={() => (showNew = true)}>Scaffold the first job</button>
        {/if}
      {/snippet}
    </EmptyState>
  {:else}
    <ul class="jobs" role="list">
      {#each filtered as job (job.name)}
        <li>
          <a
            class="job"
            class:needs-human={job.jdi?.state === 'stopped:needs-human'}
            href={href({ name: 'job', project, job: job.name })}
          >
            <div class="left">
              <Pipeline stage={job.stage} jdi={job.jdi} variant="mini" />
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
              <span class="chip" class:chip-open={job.status === 'open'} class:chip-done={job.status === 'done'}>
                <span class="dot"></span> {job.status}
              </span>
              <span class="chip chip-neutral">{job.type}</span>
              <span class="when" title={job.jdi ? job.jdi.updated : job.date}>
                {job.jdi ? relativeTime(job.jdi.updated) : job.date}
              </span>
            </div>
          </a>
        </li>
      {/each}
    </ul>
  {/if}

  {#if orphans.length > 0}
    <section class="orphans" aria-label="Orphaned worktrees">
      <div class="orphans-head">
        <div>
          <p class="eyebrow">leftovers</p>
          <h2>Orphaned worktrees</h2>
          <p class="hint">
            Directories whose git registration is gone — safe to remove; their jobs are already abandoned.
          </p>
        </div>
        <button class="btn" onclick={() => (confirmOrphans = true)}>Clean up…</button>
      </div>
      <ul>
        {#each orphans as o (o.name)}
          <li><code>{o.name}</code></li>
        {/each}
      </ul>
    </section>
  {:else if !orphansAvailable}
    <!-- Part 2 endpoint absent on this daemon — nothing to show. -->
  {/if}
</div>

{#if showNew}
  <Modal title="New job" busy={creating} onclose={() => (showNew = false)} width="480px">
    <div class="field">
      <label for="nj-title">title</label>
      <input
        id="nj-title"
        class="input"
        bind:value={newTitle}
        placeholder="What is this job about?"
        onkeydown={(e) => e.key === 'Enter' && newTitle.trim() && createJob()}
      />
      <p class="hint">The title seeds the slug — keep it short and plain.</p>
    </div>
    <div class="field">
      <label for="nj-type">type</label>
      <select id="nj-type" class="select" bind:value={newType}>
        <option value="feature">feature — new capability</option>
        <option value="fix">fix — something is broken</option>
        <option value="chore">chore — cleanup, deps, tooling</option>
      </select>
    </div>
    {#snippet footer()}
      <button class="btn" onclick={() => (showNew = false)} disabled={creating}>Cancel</button>
      <button class="btn btn-primary" onclick={createJob} disabled={creating || !newTitle.trim()}>
        {#if creating}<span class="spin" aria-hidden="true"></span>{/if}
        Scaffold job
      </button>
    {/snippet}
  </Modal>
{/if}

{#if confirmOrphans}
  <Confirm
    title="Remove orphaned worktrees"
    body={'Remove ' +
      orphans.length +
      ' orphaned worktree' +
      (orphans.length === 1 ? '' : 's') +
      '?\nThis cannot be undone.'}
    confirmLabel="Remove"
    confirmKind="danger"
    busy={removingOrphans}
    onconfirm={removeOrphans}
    onclose={() => (confirmOrphans = false)}
  />
{/if}

<style>
  .page {
    padding: var(--r-6) var(--r-6) var(--r-7);
    max-width: 1060px;
    margin: 0 auto;
    width: 100%;
  }

  .head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: var(--r-4);
    flex-wrap: wrap;
    margin-bottom: var(--r-4);
  }
  h1 {
    font-size: 24px;
    letter-spacing: -0.02em;
  }
  .head .eyebrow {
    margin: 0 0 2px;
  }
  .head-actions {
    display: flex;
    gap: var(--r-2);
    align-items: center;
  }
  .search {
    width: min(280px, 60vw);
  }

  .filters {
    display: flex;
    gap: var(--r-3);
    flex-wrap: wrap;
    margin-bottom: var(--r-4);
  }

  .error {
    background: var(--st-human-dim);
    border: 1px solid rgba(242, 85, 90, 0.4);
    color: #ffb3b6;
    padding: var(--r-3) var(--r-4);
    border-radius: var(--radius-m);
    font-size: 13.5px;
    margin-bottom: var(--r-4);
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
    gap: var(--r-4);
    padding: 14px 18px;
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    text-decoration: none;
    color: var(--ink-0);
    transition:
      border-color var(--t-fast),
      background var(--t-fast),
      transform var(--t-fast);
  }
  .job:hover {
    border-color: var(--line-strong);
    background: var(--bg-2);
    text-decoration: none;
    transform: translateY(-1px);
  }
  .job.needs-human {
    border-color: rgba(242, 85, 90, 0.4);
    background: linear-gradient(90deg, rgba(242, 85, 90, 0.05), transparent 200px), var(--bg-1);
  }

  .left {
    display: flex;
    align-items: center;
    gap: var(--r-5);
    min-width: 0;
  }
  .ident {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .title {
    font-weight: 590;
    font-size: 15px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .name {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-2);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: none;
  }
  .when {
    font-size: 12px;
    color: var(--ink-3);
    min-width: 70px;
    text-align: right;
  }

  .orphans {
    margin-top: var(--r-6);
    border: 1px dashed var(--line-strong);
    border-radius: var(--radius-m);
    padding: var(--r-4) var(--r-5);
  }
  .orphans-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: var(--r-4);
  }
  .orphans h2 {
    font-size: 16px;
  }
  .hint {
    color: var(--ink-2);
    font-size: 13px;
    max-width: 52ch;
  }
  .orphans ul {
    list-style: none;
    margin: var(--r-3) 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .orphans code {
    color: var(--st-open);
    font-size: 12.5px;
  }

  @media (max-width: 760px) {
    .page {
      padding: var(--r-4) var(--r-4) var(--r-6);
    }
    .job {
      flex-direction: column;
      align-items: flex-start;
      gap: var(--r-3);
    }
    .meta {
      flex-wrap: wrap;
    }
    .when {
      text-align: left;
      min-width: 0;
    }
    .search {
      width: 100%;
    }
    .head-actions {
      width: 100%;
    }
  }
</style>
