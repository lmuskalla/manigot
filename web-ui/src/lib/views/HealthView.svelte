<script lang="ts">
  import { onMount } from 'svelte'
  import { connection } from '$lib/state/connection.svelte'
  import { toasts } from '$lib/state/toasts.svelte'
  import SettingsModal from '$lib/components/SettingsModal.svelte'
  import Confirm from '$lib/components/Confirm.svelte'
  import { pruneContainers } from '$lib/api/client'

  let showSettings = $state(false)
  let confirmPrune = $state(false)
  let pruning = $state(false)

  onMount(() => {
    void connection.check()
  })

  async function doPrune() {
    pruning = true
    try {
      const res = await pruneContainers()
      toasts.ok(
        `Removed ${res.removed} exited container${res.removed === 1 ? '' : 's'} — ${res.running} manigot container${res.running === 1 ? '' : 's'} running`,
      )
      confirmPrune = false
    } catch (e) {
      const msg =
        e instanceof Error &&
        'capabilityMiss' in e &&
        (e as { capabilityMiss: boolean }).capabilityMiss
          ? 'Container pruning lands with job two of the control plane.'
          : e instanceof Error
            ? e.message
            : String(e)
      toasts.error(msg)
      confirmPrune = false
    } finally {
      pruning = false
    }
  }
</script>

<div class="page">
  <header class="head">
    <div>
      <p class="eyebrow">daemon</p>
      <h1>mg serve</h1>
    </div>
    <button class="btn" onclick={() => (showSettings = true)}>Connection…</button>
  </header>

  {#if connection.status === 'up' && connection.health}
    <section class="panel">
      <div class="row">
        <span class="k">version</span>
        <span class="v mono">{connection.health.version || '—'}</span>
      </div>
      <div class="row">
        <span class="k">docker image</span>
        <span class="v">
          {#if connection.health.imagePresent}
            <span class="chip chip-done"><span class="dot"></span> present</span>
          {:else}
            <span class="chip chip-human"><span class="dot"></span> missing — run make build</span>
          {/if}
        </span>
      </div>
      <div class="row">
        <span class="k">connection</span>
        <span class="v mono">{connection.baseUrl || '(same origin)'} · {connection.token ? 'bearer token' : 'tokenless'}</span>
      </div>
    </section>

    <section class="panel">
      <header class="panel-head">
        <h2>Profiles</h2>
        <p class="sub">Which subscription profiles have credentials ready — booleans only, never the credentials.</p>
      </header>
      <ul class="profiles">
        {#each connection.health.profiles ?? [] as p (p.id)}
          <li class="profile" class:ready={p.ready}>
            <span class="p-dot"></span>
            <span class="p-name mono">{p.id}</span>
            <span class="p-state">{p.ready ? 'ready' : 'not configured'}</span>
          </li>
        {/each}
      </ul>
    </section>
  {:else if connection.status === 'connecting'}
    <p class="loading">Checking the daemon…</p>
  {:else}
    <section class="panel down">
      <h2>Daemon unreachable</h2>
      <p class="sub">{connection.lastError}</p>
      <p class="sub">
        Start it with <code>mg serve</code> (localhost, tokenless) or point the connection at the
        right URL and token.
      </p>
    </section>
  {/if}

  <section class="panel">
    <header class="panel-head">
      <h2>Maintenance</h2>
      <p class="sub">The cleanup a headless daemon needs — no terminal required.</p>
    </header>
    <div class="maint">
      <div>
        <p class="m-k">Prune containers</p>
        <p class="m-d">Remove exited manigot containers — the residue of abnormal session ends.</p>
      </div>
      <button class="btn" onclick={() => (confirmPrune = true)}>Prune…</button>
    </div>
    <p class="fine">
      Orphaned-worktree cleanup lives in each project's jobs view.
    </p>
  </section>
</div>

{#if showSettings}
  <SettingsModal onclose={() => (showSettings = false)} />
{/if}

{#if confirmPrune}
  <Confirm
    title="Prune containers"
    body="Remove exited manigot containers?\nRunning containers are never touched."
    confirmLabel="Prune"
    onconfirm={doPrune}
    busy={pruning}
    onclose={() => (confirmPrune = false)}
  />
{/if}

<style>
  .page {
    max-width: 720px;
    margin: 0 auto;
    padding: var(--r-6) var(--r-6) var(--r-7);
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--r-4);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    gap: var(--r-4);
  }
  h1 {
    font-size: 24px;
    letter-spacing: -0.02em;
  }

  .panel {
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    padding: var(--r-4) var(--r-5);
  }
  .panel-head h2 {
    font-size: 15.5px;
    margin-bottom: 2px;
  }
  .panel-head {
    margin-bottom: var(--r-3);
  }
  .sub {
    color: var(--ink-2);
    font-size: 13px;
  }
  .sub code {
    color: var(--accent-bright);
  }

  .row {
    display: flex;
    justify-content: space-between;
    gap: var(--r-4);
    padding: 7px 0;
    border-bottom: 1px solid var(--line);
    font-size: 13.5px;
  }
  .row:last-child {
    border-bottom: none;
  }
  .k {
    color: var(--ink-2);
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .v {
    color: var(--ink-0);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: 12.5px;
  }

  .profiles {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
    gap: var(--r-2);
  }
  .profile {
    display: flex;
    align-items: center;
    gap: 9px;
    background: var(--bg-2);
    border: 1px solid var(--line);
    border-radius: var(--radius-s);
    padding: 8px 12px;
    font-size: 12.5px;
  }
  .p-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--ink-3);
    flex: none;
  }
  .profile.ready .p-dot {
    background: var(--st-done);
    box-shadow: 0 0 6px rgba(76, 195, 138, 0.5);
  }
  .p-name {
    color: var(--ink-0);
  }
  .p-state {
    margin-left: auto;
    color: var(--ink-3);
    font-size: 11.5px;
  }
  .profile.ready .p-state {
    color: var(--st-done);
  }

  .down h2 {
    color: #ffb3b6;
  }

  .maint {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--r-4);
  }
  .m-k {
    font-weight: 590;
    font-size: 14px;
    color: var(--ink-0);
  }
  .m-d {
    font-size: 12.5px;
    color: var(--ink-2);
    max-width: 52ch;
  }
  .fine {
    margin-top: var(--r-3);
    font-size: 12px;
    color: var(--ink-3);
    border-top: 1px solid var(--line);
    padding-top: var(--r-3);
  }
  .loading {
    color: var(--ink-2);
  }

  @media (max-width: 760px) {
    .page {
      padding: var(--r-4) var(--r-4) var(--r-6);
    }
    .maint {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>
