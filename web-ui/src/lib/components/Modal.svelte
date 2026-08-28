<!-- Modal — focused dialog over a scrim. Escape closes (unless busy), click -->
<!-- on the scrim closes, focus moves to the panel. Used by every action that -->
<!-- needs a decision (confirmations, new job, agent picker, settings). -->

<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    title,
    children,
    footer,
    busy = false,
    width = '480px',
    onclose,
  }: {
    title: string
    children: Snippet
    footer?: Snippet
    busy?: boolean
    width?: string
    onclose: () => void
  } = $props()

  let panel: HTMLElement | undefined = $state()

  $effect(() => {
    panel?.focus()
  })

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && !busy) {
      e.stopPropagation()
      onclose()
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div
  class="scrim"
  onclick={(e) => {
    if (e.target === e.currentTarget && !busy) onclose()
  }}
>
  <div
    class="panel"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    tabindex="-1"
    bind:this={panel}
    style="--panel-w: {width}"
  >
    <header>
      <h2>{title}</h2>
    </header>
    <div class="body">
      {@render children()}
    </div>
    {#if footer}
      <footer>
        {@render footer()}
      </footer>
    {/if}
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(10, 7, 16, 0.66);
    backdrop-filter: blur(3px);
    display: grid;
    place-items: center;
    z-index: 60;
    padding: var(--r-4);
    animation: fade var(--t-fast);
  }

  .panel {
    width: min(var(--panel-w), 100%);
    max-height: min(78vh, 720px);
    display: flex;
    flex-direction: column;
    background: linear-gradient(180deg, var(--bg-2), var(--bg-1));
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-l);
    box-shadow: var(--shadow-pop);
    animation: rise var(--t-med);
    outline: none;
  }

  header {
    padding: var(--r-4) var(--r-5) var(--r-3);
  }

  h2 {
    font-size: 17px;
    letter-spacing: -0.01em;
  }

  .body {
    padding: 0 var(--r-5);
    overflow-y: auto;
    color: var(--ink-1);
    font-size: 14px;
  }

  footer {
    padding: var(--r-4) var(--r-5);
    display: flex;
    justify-content: flex-end;
    gap: var(--r-2);
    border-top: 1px solid var(--line);
    margin-top: var(--r-4);
  }

  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(10px) scale(0.985);
    }
  }
</style>
