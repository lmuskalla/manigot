<!-- Command palette (⌘K) — jump to jobs/projects and fire maintenance actions. -->
<!-- It reads live data, so everything it offers is real. -->

<script lang="ts">
  import { data } from '$lib/state/data.svelte'
  import { toasts } from '$lib/state/toasts.svelte'
  import { pruneContainers } from '$lib/api/client'
  import { href } from '$lib/router'

  let {
    onclose,
    onnavigate,
  }: {
    onclose: () => void
    onnavigate: (h: string) => void
  } = $props()

  let q = $state('')
  let sel = $state(0)

  interface Item {
    label: string
    hint: string
    group: string
    run: () => void | Promise<void>
  }

  const items = $derived.by(() => {
    const list: Item[] = []
    for (const p of data.projects) {
      list.push({
        label: p.name,
        hint: p.path,
        group: 'projects',
        run: () => onnavigate(href({ name: 'jobs', project: p.name })),
      })
    }
    for (const j of data.jobs) {
      const at = j.jdi?.state === 'stopped:needs-human' ? ' · needs human' : j.jdi?.state === 'running' ? ' · running' : ''
      list.push({
        label: j.title,
        hint: `${j.name}${at}`,
        group: 'jobs',
        run: () => onnavigate(href({ name: 'job', project: data.active, job: j.name })),
      })
    }
    list.push({
      label: 'New job…',
      hint: 'scaffold a brief, branch and worktree',
      group: 'actions',
      run: () => onnavigate(href({ name: 'jobs', project: data.active })),
    })
    list.push({
      label: 'Daemon health',
      hint: 'version, image, profiles',
      group: 'actions',
      run: () => onnavigate('#/health'),
    })
    list.push({
      label: 'Prune containers',
      hint: 'remove exited manigot containers',
      group: 'actions',
      run: async () => {
        try {
          const res = await pruneContainers()
          toasts.ok(`Pruned ${res.removed} container${res.removed === 1 ? '' : 's'} — ${res.running} running`)
        } catch (e) {
          toasts.error(e instanceof Error ? e.message : String(e))
        }
      },
    })
    const needle = q.trim().toLowerCase()
    if (!needle) return list
    return list.filter(
      (i) => i.label.toLowerCase().includes(needle) || i.hint.toLowerCase().includes(needle),
    )
  })

  const grouped = $derived.by(() => {
    const groups: { name: string; items: Item[] }[] = []
    for (const item of items) {
      const g = groups.find((x) => x.name === item.group)
      if (g) g.items.push(item)
      else groups.push({ name: item.group, items: [item] })
    }
    return groups
  })

  let listEl: HTMLElement | undefined = $state()

  $effect(() => {
    // clamp selection when the filtered list shrinks
    if (sel >= items.length) sel = Math.max(0, items.length - 1)
  })

  async function choose(item: Item) {
    // Navigation closes the palette through onnavigate; a raw action (prune)
    // does not navigate, so close here.
    const result = item.run()
    if (result instanceof Promise) {
      await result.catch(() => {})
      onclose()
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || (e.key === 'n' && e.ctrlKey)) {
      e.preventDefault()
      sel = Math.min(items.length - 1, sel + 1)
    } else if (e.key === 'ArrowUp' || (e.key === 'p' && e.ctrlKey)) {
      e.preventDefault()
      sel = Math.max(0, sel - 1)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const item = items[sel]
      if (item) choose(item)
    }
    // scroll the selected row into view
    listEl?.querySelector('[data-sel="true"]')?.scrollIntoView({ block: 'nearest' })
  }

  let inputEl: HTMLInputElement | undefined = $state()
  $effect(() => {
    inputEl?.focus()
  })
</script>

<svelte:window onkeydown={onKeydown} />

<div
  class="scrim"
  role="presentation"
  onmousedown={(e) => e.target === e.currentTarget && onclose()}
>
  <div class="palette" role="dialog" aria-label="Commands">
    <input
      bind:this={inputEl}
      bind:value={q}
      class="q"
      placeholder="Jump to a job, project, or action…"
      spellcheck="false"
    />
    <div class="list" bind:this={listEl}>
      {#each grouped as g (g.name)}
        <div class="group">{g.name}</div>
        {#each g.items as item, gi (item.label)}
          {@const flat = items.indexOf(item)}
          <button
            class="item"
            data-sel={flat === sel}
            onclick={() => choose(item)}
            onpointerenter={() => (sel = flat)}
          >
            <span class="label">{item.label}</span>
            <span class="hint">{item.hint}</span>
          </button>
        {/each}
      {/each}
      {#if items.length === 0}
        <div class="none">Nothing matches “{q}”.</div>
      {/if}
    </div>
    <div class="foot">
      <span><span class="kbd">↑↓</span> navigate</span>
      <span><span class="kbd">↵</span> run</span>
      <span><span class="kbd">esc</span> close</span>
    </div>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(10, 7, 16, 0.6);
    backdrop-filter: blur(2px);
    z-index: 70;
    display: flex;
    justify-content: center;
    padding-top: 12vh;
  }
  .palette {
    width: min(560px, calc(100vw - 32px));
    max-height: 62vh;
    display: flex;
    flex-direction: column;
    background: linear-gradient(180deg, var(--bg-2), var(--bg-1));
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-l);
    box-shadow: var(--shadow-pop);
    overflow: hidden;
    animation: rise var(--t-med);
    align-self: flex-start;
  }
  .q {
    font: inherit;
    font-size: 15px;
    color: var(--ink-0);
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--line);
    padding: 15px 18px;
    outline: none;
  }
  .q::placeholder {
    color: var(--ink-2);
  }
  .list {
    overflow-y: auto;
    padding: var(--r-2);
  }
  .group {
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-2);
    padding: 8px 10px 4px;
  }
  .item {
    display: flex;
    flex-direction: column;
    gap: 1px;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    border-radius: var(--radius-s);
    padding: 7px 10px;
    cursor: pointer;
    font: inherit;
  }
  .item[data-sel='true'] {
    background: var(--accent-dim);
  }
  .label {
    color: var(--ink-0);
    font-size: 14px;
    font-weight: 530;
  }
  .hint {
    color: var(--ink-2);
    font-size: 12px;
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .none {
    padding: var(--r-5);
    text-align: center;
    color: var(--ink-2);
    font-size: 13.5px;
  }
  .foot {
    display: flex;
    gap: var(--r-4);
    padding: 9px 16px;
    border-top: 1px solid var(--line);
    color: var(--ink-2);
    font-size: 11.5px;
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
  }
</style>
