<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    title,
    hint = '',
    children,
    action,
  }: {
    title: string
    hint?: string
    children?: Snippet
    action?: Snippet
  } = $props()
</script>

<div class="empty">
  <div class="mark" aria-hidden="true">
    <svg width="40" height="16" viewBox="0 0 40 16" fill="none">
      <rect x="1" y="4" width="8" height="8" rx="2" stroke="var(--line-strong)" stroke-width="1.5" />
      <rect x="16" y="4" width="8" height="8" rx="2" stroke="var(--line-strong)" stroke-width="1.5" />
      <rect x="31" y="4" width="8" height="8" rx="2" stroke="var(--line-strong)" stroke-width="1.5" stroke-dasharray="2.5 2.5" />
    </svg>
  </div>
  <h3>{title}</h3>
  {#if hint}<p class="hint">{hint}</p>{/if}
  {#if children}{@render children()}{/if}
  {#if action}
    <div class="action">
      {@render action()}
    </div>
  {/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: var(--r-7) var(--r-5);
    gap: var(--r-2);
  }
  h3 {
    font-size: 16px;
    color: var(--ink-1);
  }
  .hint {
    color: var(--ink-2);
    font-size: 13.5px;
    max-width: 40ch;
  }
  .action {
    margin-top: var(--r-3);
  }
</style>
