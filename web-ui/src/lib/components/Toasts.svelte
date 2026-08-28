<script lang="ts">
  import { toasts } from '$lib/state/toasts.svelte'

  function icon(kind: 'ok' | 'error' | 'info') {
    return kind === 'ok' ? '✓' : kind === 'error' ? '✕' : 'ℹ'
  }
</script>

<div class="toasts" role="status" aria-live="polite">
  {#each toasts.items as t (t.id)}
    <button class="toast {t.kind}" onclick={() => toasts.dismiss(t.id)} aria-label="Dismiss notification">
      <span class="mark" aria-hidden="true">{icon(t.kind)}</span>
      <span class="text">{t.text}</span>
    </button>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    bottom: var(--r-5);
    right: var(--r-5);
    display: flex;
    flex-direction: column;
    gap: var(--r-2);
    z-index: 90;
    max-width: min(420px, calc(100vw - 32px));
  }
  .toast {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    background: linear-gradient(180deg, var(--bg-2), var(--bg-1));
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-m);
    box-shadow: var(--shadow-pop);
    padding: 10px 14px;
    font: inherit;
    font-size: 13.5px;
    cursor: pointer;
    animation: rise var(--t-med);
    text-align: left;
  }
  .mark {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    margin-top: 1px;
  }
  .ok .mark {
    color: var(--st-done);
  }
  .error {
    border-color: rgba(242, 85, 90, 0.5);
  }
  .error .mark {
    color: var(--st-human);
  }
  .info .mark {
    color: var(--st-info);
  }
  .text {
    color: var(--ink-1);
    overflow-wrap: anywhere;
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
  }
</style>
