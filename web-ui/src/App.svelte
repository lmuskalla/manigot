<script lang="ts">
  import { onMount } from 'svelte'
  import Logo from '$lib/components/Logo.svelte'
  import Toasts from '$lib/components/Toasts.svelte'
  import Palette from '$lib/components/Palette.svelte'
  import SettingsModal from '$lib/components/SettingsModal.svelte'
  import JobsView from '$lib/views/JobsView.svelte'
  import JobDetailView from '$lib/views/JobDetailView.svelte'
  import AgentsView from '$lib/views/AgentsView.svelte'
  import HealthView from '$lib/views/HealthView.svelte'
  import DashboardView from '$lib/views/DashboardView.svelte'
  import { connection } from '$lib/state/connection.svelte'
  import { data } from '$lib/state/data.svelte'
  import { parseHash, href, navigate } from '$lib/router'

  let route = $state(parseHash(location.hash))
  let showSettings = $state(false)
  let showPalette = $state(false)

  $effect(() => {
    const onHash = () => (route = parseHash(location.hash))
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  })

  // Bootstrap: check the daemon, poll its liveness and the active project's
  // jobs. The project list itself loads from the connection effect below —
  // never in parallel with the health check against a half-configured URL.
  onMount(() => {
    void connection.check()
    data.startPolling()
    const t = setInterval(() => void connection.check(), 30_000)
    return () => {
      clearInterval(t)
      data.stopPolling()
    }
  })

  // A connection becoming established — first boot, a settings change, or the
  // daemon coming back — (re)loads the project list once per establishment,
  // so the dropdown populates without a page reload. Keying off the
  // establishment edge (not the list being empty) keeps an empty registry
  // from retriggering the load — and the fetch — forever.
  $effect(() => {
    if (connection.established > 0) {
      void data.loadProjects()
    }
  })

  function onKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      showPalette = !showPalette
    }
  }

  const connLabel = $derived(
    connection.status === 'up' ? 'connected' : connection.status === 'down' ? 'offline' : 'connecting…',
  )

  function switchProject(e: Event) {
    const name = (e.currentTarget as HTMLSelectElement).value
    data.setActive(name)
    navigate(href({ name: 'jobs', project: name }))
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="shell">
  <aside class="rail">
    <a class="brand" href="#/" aria-label="manigot home">
      <Logo size={24} />
      <span class="word">manigot</span>
      <span class="role">control plane</span>
    </a>

    <div class="project">
      <label class="eyebrow" for="project-select">project</label>
      <select id="project-select" class="select" value={data.active} onchange={switchProject} disabled={data.projects.length === 0}>
        {#each data.projects as p (p.name)}
          <option value={p.name}>{p.name}</option>
        {/each}
      </select>
      {#if data.projectsError}
        <p class="no-projects err">Couldn't load projects — {data.projectsError}</p>
      {:else if data.projects.length === 0 && connection.status === 'up'}
        <p class="no-projects">No projects registered.<br />Edit <code>config/serve.json</code> on the daemon.</p>
      {/if}
    </div>

    <nav aria-label="Primary">
      <a
        href={href({ name: 'jobs', project: route.project ?? data.active })}
        class:active={route.name === 'jobs' || route.name === 'job'}
      >
        <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><rect x="1.5" y="1.5" width="13" height="13" rx="3" fill="none" stroke="currentColor" stroke-width="1.4"/><path d="M4.5 5.5h7M4.5 8h7M4.5 10.5h4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        Jobs
      </a>
      <a
        href={href({ name: 'agents', project: route.project ?? data.active })}
        class:active={route.name === 'agents'}
      >
        <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><circle cx="8" cy="5" r="2.6" fill="none" stroke="currentColor" stroke-width="1.4"/><path d="M2.8 13.4c.7-2.5 2.8-3.8 5.2-3.8s4.5 1.3 5.2 3.8" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        Crew
      </a>
      <a href="#/health" class:active={route.name === 'health'}>
        <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M8 1.8l5.4 2.2v4c0 3.3-2.2 5.6-5.4 6.4-3.2-.8-5.4-3.1-5.4-6.4v-4z" fill="none" stroke="currentColor" stroke-width="1.4"/><path d="M5.6 7.9l1.8 1.8 3.2-3.4" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
        Daemon
      </a>
    </nav>

    <div class="rail-foot">
      <button class="conn" class:down={connection.status === 'down'} onclick={() => (showSettings = true)}>
        <span class="conn-dot" class:up={connection.status === 'up'} class:down={connection.status === 'down'}></span>
        {connLabel}
      </button>
      <button class="palette-hint" onclick={() => (showPalette = true)}>
        <span class="kbd">⌘K</span> commands
      </button>
    </div>
  </aside>

  <main class="main">
    {#if connection.status === 'down' && route.name !== 'health'}
      <div class="offline-banner" role="alert">
        <span>The daemon is unreachable — {connection.lastError}</span>
        <button class="btn btn-sm" onclick={() => (showSettings = true)}>Connection…</button>
      </div>
    {/if}

    {#if route.name === 'job' && route.project && route.job}
      <JobDetailView project={route.project} jobName={route.job} tab={route.tab ?? 'brief'} />
    {:else if route.name === 'agents' && route.project}
      <AgentsView project={route.project} />
    {:else if route.name === 'health'}
      <HealthView />
    {:else if route.name === 'jobs' && route.project}
      <JobsView project={route.project} />
    {:else if route.name === 'home' && connection.status === 'up' && !data.projectsError && data.projects.length > 0}
      <DashboardView />
    {:else}
      <div class="landing">
        <Logo size={40} />
        {#if connection.status === 'down'}
          <p>The daemon is unreachable.</p>
          <button class="btn" onclick={() => (showSettings = true)}>Connection…</button>
        {:else if data.projectsError}
          <p>Couldn't load projects.</p>
          <p class="landing-detail">{data.projectsError}</p>
        {:else if connection.status === 'up' && data.projects.length === 0}
          <p>No projects registered.</p>
          <p class="landing-detail">
            Edit <code>config/serve.json</code> on the daemon and restart <code>mg serve</code>.
          </p>
        {:else}
          <p>Connecting to the daemon…</p>
        {/if}
      </div>
    {/if}
  </main>

  <nav class="tabbar" aria-label="Primary mobile">
    <a href={href({ name: 'jobs', project: route.project ?? data.active })} class:active={route.name === 'jobs' || route.name === 'job'}>Jobs</a>
    <a href={href({ name: 'agents', project: route.project ?? data.active })} class:active={route.name === 'agents'}>Crew</a>
    <a href="#/health" class:active={route.name === 'health'}>Daemon</a>
  </nav>
</div>

<Toasts />
{#if showSettings}
  <SettingsModal onclose={() => (showSettings = false)} />
{/if}
{#if showPalette}
  <Palette
    onclose={() => (showPalette = false)}
    onnavigate={(h) => {
      showPalette = false
      navigate(h)
    }}
  />
{/if}

<style>
  .shell {
    display: grid;
    grid-template-columns: var(--rail-w) 1fr;
    height: 100dvh;
    min-height: 0;
  }

  /* ── rail ─────────────────────────────────────────────────────────────── */
  .rail {
    display: flex;
    flex-direction: column;
    gap: var(--r-5);
    padding: var(--r-5) var(--r-4);
    border-right: 1px solid var(--line);
    background: linear-gradient(180deg, rgba(139, 108, 246, 0.045), transparent 240px), var(--bg-1);
    overflow-y: auto;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 9px;
    color: var(--ink-0);
    text-decoration: none;
  }
  .brand:hover {
    text-decoration: none;
  }
  .word {
    font-weight: 680;
    font-size: 16.5px;
    letter-spacing: -0.02em;
  }
  .role {
    font-family: var(--font-mono);
    font-size: 9.5px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-2);
    margin-left: -2px;
    padding-top: 3px;
  }

  .project {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .no-projects {
    font-size: 12px;
    color: var(--ink-2);
    margin: 4px 0 0;
    line-height: 1.5;
  }
  .no-projects code {
    color: var(--ink-1);
  }
  .no-projects.err {
    color: #ffb3b6;
  }

  nav[aria-label='Primary'] {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  nav[aria-label='Primary'] a {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 7px 10px;
    border-radius: var(--radius-s);
    color: var(--ink-1);
    text-decoration: none;
    font-size: 14px;
    font-weight: 520;
    transition: background var(--t-fast), color var(--t-fast);
  }
  nav[aria-label='Primary'] a:hover {
    background: var(--bg-2);
    color: var(--ink-0);
    text-decoration: none;
  }
  nav[aria-label='Primary'] a.active {
    background: var(--accent-dim);
    color: var(--accent-bright);
  }

  .rail-foot {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: var(--r-2);
  }
  .conn {
    display: flex;
    align-items: center;
    gap: 8px;
    font: inherit;
    font-size: 12.5px;
    color: var(--ink-1);
    background: none;
    border: 1px solid var(--line);
    border-radius: var(--radius-s);
    padding: 6px 10px;
    cursor: pointer;
    transition: border-color var(--t-fast);
  }
  .conn:hover {
    border-color: var(--line-strong);
  }
  .conn.down {
    color: #ff8d91;
    border-color: rgba(242, 85, 90, 0.45);
  }
  .conn-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--ink-3);
    flex: none;
  }
  .conn-dot.up {
    background: var(--st-done);
    box-shadow: 0 0 6px rgba(76, 195, 138, 0.6);
  }
  .conn-dot.down {
    background: var(--st-human);
  }
  .palette-hint {
    display: flex;
    align-items: center;
    gap: 8px;
    font: inherit;
    font-size: 12px;
    color: var(--ink-2);
    background: none;
    border: none;
    padding: 4px 2px;
    cursor: pointer;
  }
  .palette-hint:hover {
    color: var(--ink-1);
  }

  /* ── main ─────────────────────────────────────────────────────────────── */
  .main {
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    scroll-behavior: smooth;
  }

  .offline-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--r-3);
    padding: 9px var(--r-5);
    background: var(--st-human-dim);
    border-bottom: 1px solid rgba(242, 85, 90, 0.35);
    color: #ffb3b6;
    font-size: 13px;
  }
  .offline-banner span {
    /* the message can be long (CORS guidance); wrap instead of truncating */
    overflow-wrap: anywhere;
  }

  .landing {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--r-3);
    color: var(--ink-2);
  }
  .landing-detail {
    font-size: 13px;
    max-width: 46ch;
    text-align: center;
    line-height: 1.6;
  }
  .landing-detail code {
    color: var(--ink-1);
  }

  /* mobile tab bar — hidden on desktop */
  .tabbar {
    display: none;
  }

  @media (max-width: 760px) {
    .shell {
      grid-template-columns: 1fr;
      grid-template-rows: 1fr auto;
    }
    .rail {
      display: none;
    }
    .tabbar {
      display: flex;
      border-top: 1px solid var(--line);
      background: var(--bg-1);
    }
    .tabbar a {
      flex: 1;
      text-align: center;
      padding: 12px 0;
      color: var(--ink-2);
      text-decoration: none;
      font-size: 13.5px;
      font-weight: 560;
    }
    .tabbar a.active {
      color: var(--accent-bright);
      box-shadow: inset 0 2px 0 var(--accent);
    }
  }
</style>
