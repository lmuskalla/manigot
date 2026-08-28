<script lang="ts">
  import { onMount } from 'svelte'
  import EmptyState from '$lib/components/EmptyState.svelte'
  import { getAgents } from '$lib/api/client'
  import { href } from '$lib/router'
  import type { AgentRow } from '$lib/api/types'

  let { project }: { project: string } = $props()

  let agents = $state<AgentRow[]>([])
  let loading = $state(true)
  let error = $state('')
  let q = $state('')

  onMount(async () => {
    try {
      const res = await getAgents(project)
      agents = res.agents ?? []
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  })

  const filtered = $derived.by(() => {
    const needle = q.trim().toLowerCase()
    if (!needle) return agents
    return agents.filter(
      (a) => a.name.toLowerCase().includes(needle) || a.description.toLowerCase().includes(needle),
    )
  })

  const JDI_CREW = ['analyst', 'developer', 'reviewer']

  function crewOf(name: string): 'jdi' | 'launchable' {
    return JDI_CREW.includes(name) ? 'jdi' : 'launchable'
  }
</script>

<div class="page">
  <header class="head">
    <div>
      <p class="eyebrow">crew</p>
      <h1>Agents on {project}</h1>
    </div>
    <input class="input search" placeholder="Filter the crew…" bind:value={q} spellcheck="false" aria-label="Filter agents" />
  </header>

  <p class="lead">
    The autonomous sequence is fixed — <code>@analyst → @developer → @reviewer</code> — that is what
    <code>mg jdi</code> runs. Every agent can also be fired once, detached, from a job's
    <a href={href({ name: 'jobs', project })}>detail view</a>.
  </p>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if loading}
    <p class="loading">Listing the crew…</p>
  {:else if filtered.length === 0}
    <EmptyState title="No agents match" hint="Clear the filter to see the whole crew." />
  {:else}
    <ul class="grid">
      {#each filtered as a (a.name)}
        <li class="agent" data-crew={crewOf(a.name)}>
          <div class="agent-head">
            <span class="agent-name">{"@" + a.name}</span>
            {#if crewOf(a.name) === 'jdi'}
              <span class="chip chip-running"><span class="dot"></span> jdi crew</span>
            {:else}
              <span class="chip chip-neutral">manual</span>
            {/if}
          </div>
          <p class="agent-desc">{a.description}</p>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .page {
    max-width: 900px;
    margin: 0 auto;
    padding: var(--r-6) var(--r-6) var(--r-7);
    width: 100%;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    gap: var(--r-4);
    flex-wrap: wrap;
    margin-bottom: var(--r-3);
  }
  h1 {
    font-size: 24px;
    letter-spacing: -0.02em;
  }
  .search {
    width: min(260px, 60vw);
  }
  .lead {
    color: var(--ink-2);
    font-size: 13.5px;
    margin-bottom: var(--r-5);
    max-width: 70ch;
  }
  .lead code {
    color: var(--accent-bright);
    font-size: 0.9em;
  }

  .grid {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: var(--r-3);
  }
  .agent {
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    padding: var(--r-4);
    transition: border-color var(--t-fast);
  }
  .agent:hover {
    border-color: var(--line-strong);
  }
  .agent[data-crew='jdi'] {
    border-left: 2px solid var(--accent);
  }
  .agent-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--r-2);
    margin-bottom: 8px;
  }
  .agent-name {
    font-family: var(--font-mono);
    font-size: 13.5px;
    color: var(--ink-0);
    font-weight: 500;
  }
  .agent[data-crew='jdi'] .agent-name {
    color: var(--accent-bright);
  }
  .agent-desc {
    font-size: 13px;
    color: var(--ink-2);
    line-height: 1.55;
  }
  .error {
    color: #ffb3b6;
  }
  .loading {
    color: var(--ink-2);
  }

  @media (max-width: 760px) {
    .page {
      padding: var(--r-4) var(--r-4) var(--r-6);
    }
    .search {
      width: 100%;
    }
  }
</style>
